// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows
// +build windows

package osnetconfig

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"v.io/x/lib/netconfig/route"
)

var (
	modws2_32             = windows.NewLazySystemDLL("ws2_32.dll")
	modiphlpapi           = windows.NewLazySystemDLL("iphlpapi.dll")
	procNotifyRouteChange *windows.LazyProc
	overlap               = &windows.Overlapped{}
)

func init() {
	modws2_32.System, modiphlpapi.System = true, true
	procNotifyRouteChange = modiphlpapi.NewProc("NotifyRouteChange")
}

func (n *Notifier) initLocked() error {
	var err error
	overlap.HEvent, err = windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return err
	}
	go func() {
		n.watcher(overlap, 1000*5)
		windows.Close(overlap.HEvent)
	}()
	return nil
}

func waitForRouteTableChange(overlap *windows.Overlapped, delayMillisecond uint32) bool {
	notifyHandle := windows.Handle(0)
	syscall.Syscall(uintptr(procNotifyRouteChange.Addr()), 2, uintptr(notifyHandle), uintptr(unsafe.Pointer(overlap)), 0)
	event, err := windows.WaitForSingleObject(overlap.HEvent, delayMillisecond)
	return err == nil && event == windows.WAIT_OBJECT_0
}

func (n *Notifier) watcher(overlap *windows.Overlapped, delayMillisecond uint32) {
	for {
		if waitForRouteTableChange(overlap, delayMillisecond) {
			if n.ding() {
				return
			}
		}
		if n.stopped() {
			return
		}
	}
}

// GetIPRoutes implements netconfig.Notifier.
func (n *Notifier) GetIPRoutes(defaultOnly bool) []route.IPRoute {
	cmd := exec.Command("route", "print")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("%s failed: %s: %v\n", strings.Join(cmd.Args, " "), out, err)
		return nil
	}
	routes, err := ParseWindowsRouteCommandOutput(string(out))
	if err != nil {
		log.Printf("%s failed to parse output: %s: %v\n", strings.Join(cmd.Args, " "), out, err)
		return nil
	}
	return routes
}

// ParseWindowsRouteCommandOutput parses the output of the windows
// 'route print' command's output and is used by GetIPRoutes.
func ParseWindowsRouteCommandOutput(output string) ([]route.IPRoute, error) {
	lines, err := readLines(output)
	if err != nil {
		return nil, err
	}
	ifcs, err := getInterfaceInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to read network interface config")
	}
	routes, err := ifcs.parseIPv4(lines)
	if err != nil {
		return nil, err
	}
	v6, err := ifcs.parseIPv6(lines)
	if err != nil {
		return nil, err
	}
	return append(routes, v6...), nil
}

func readLines(output string) ([]string, error) {
	sc := bufio.NewScanner(bytes.NewBufferString(output))
	lines := []string{}
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err()
}

func scanTo(lines []string, text string) int {
	for i, l := range lines {
		if strings.HasPrefix(l, text) {
			return i
		}
	}
	return -1
}

type netIfc struct {
	idx   int
	addrs []net.IP
}

type netIfcs []netIfc

func getInterfaceInfo() (netIfcs, error) {
	ifcs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	nifcs := make([]netIfc, len(ifcs))
	for i, ifc := range ifcs {
		nifcs[i].idx = ifc.Index
		addrs, err := ifc.Addrs()
		if err != nil {
			return nil, err
		}
		nifcs[i].addrs = make([]net.IP, 0, len(addrs))
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok {
				nifcs[i].addrs = append(nifcs[i].addrs, ipn.IP)
			}
		}
	}
	return nifcs, nil
}

func (ni netIfcs) ifcIndexForAddr(addr net.IP) int {
	for _, ifc := range ni {
		for _, a := range ifc.addrs {
			if bytes.Equal(a, addr) {
				return ifc.idx
			}
		}
	}
	return -1
}

func (ni netIfcs) ipv6AddrForIndex(idx int) []net.IP {
	for _, ifc := range ni {
		if ifc.idx != idx {
			continue
		}
		var v6addrs []net.IP
		for _, a := range ifc.addrs {
			if len(a) == net.IPv6len {
				v6addrs = append(v6addrs, a)
			}
		}
		return v6addrs
	}
	return nil
}

func findRoutingTable(lines []string, title string) (int, int, error) {
	table := scanTo(lines, title)
	if table == -1 {
		return -1, -1, fmt.Errorf("no %v found", title)
	}
	start := scanTo(lines[table:], "Active Routes:")
	if start == -1 {
		return -1, -1, fmt.Errorf("no %v found", title)
	}
	if len(lines) < start+2 {
		return -1, -1, fmt.Errorf("no entries found for %v", title)
	}
	start += 2 // skip past header 'Network Destination.....
	stop := scanTo(lines[table+start:], "================================")
	if stop == -1 {
		return -1, -1, fmt.Errorf("failed to find end of %v", title)
	}
	return table + start, table + start + stop, nil
}

func (ni netIfcs) parseIPv4(lines []string) ([]route.IPRoute, error) {
	start, stop, err := findRoutingTable(lines, "IPv4 Route Table")
	if err != nil {
		return nil, err
	}
	return ni.parseIP4Routes(lines[start:stop])
}

func (ni netIfcs) parseIP4Routes(lines []string) ([]route.IPRoute, error) {
	const (
		netdst  = 0
		netmask = 1
		gateway = 2
		ifc     = 3
	)
	var routes []route.IPRoute
	for _, l := range lines {
		parts := strings.Fields(l)
		if got, want := len(parts), 5; got != want {
			return nil, fmt.Errorf("IP4 route has incorrect # of fields: got %v, want %v, from %v", got, want, l)
		}
		dstIP := net.ParseIP(parts[netdst])
		if dstIP == nil {
			return nil, fmt.Errorf("invalid destination address: %v", parts[netdst])
		}
		mask := net.ParseIP(parts[netmask])
		if dstIP == nil {
			return nil, fmt.Errorf("invalid netmask: %v", parts[netmask])
		}
		ifcIP := net.ParseIP(parts[ifc])
		if dstIP == nil {
			return nil, fmt.Errorf("invalid interface address: %v", parts[ifc])
		}
		var gw net.IP
		if gws := parts[gateway]; gws == "On-link" {
			gw = ifcIP
		} else {
			gw = net.ParseIP(gws)
			if gw == nil {
				return nil, fmt.Errorf("invalid gateway: %v", gw)
			}
		}
		idx := ni.ifcIndexForAddr(ifcIP)
		if idx < 0 {
			return nil, fmt.Errorf("failed to determine interface index for route %v", l)
		}
		routes = append(routes, route.IPRoute{
			Net: net.IPNet{
				IP:   dstIP,
				Mask: net.IPMask(mask.To4()),
			},
			Gateway:  gw,
			IfcIndex: idx,
		})
	}
	return routes, nil
}

func (ni netIfcs) parseIPv6(lines []string) ([]route.IPRoute, error) {
	start, stop, err := findRoutingTable(lines, "IPv6 Route Table")
	if err != nil {
		return nil, err
	}
	return ni.parseIP6Routes(lines[start:stop])
}

func (ni netIfcs) parseIP6Routes(lines []string) ([]route.IPRoute, error) {
	const (
		ifc     = 0
		netdst  = 2
		gateway = 3
	)
	var routes []route.IPRoute

	// Annoyingly, for long ipv6 addresses the gateway column may printed
	// on the following line:
	//   3    281 fe80::4986:e542:6726:73ed/128
	//                                     On-link
	merged := make([][]string, 0, len(lines))
	nl := len(lines)
	for i := 0; i < nl; i++ {
		fields := strings.Fields(lines[i])
		nf := len(fields)
		if nf < 3 || nf > 4 {
			return nil, fmt.Errorf("IP6 route has incorrect # of fields: got %v, want 3 or 4, from %v", nf, lines[i])
		}
		if len(fields) == 4 {
			merged = append(merged, fields)
			continue
		}
		if i+1 >= nl {
			return nil, fmt.Errorf("IP6 route has is missing the gateway field on a subsrquent line: %v", lines[i])
		}
		nfields := strings.Fields(lines[i+1])
		if len(nfields) != 1 {
			return nil, fmt.Errorf("IP6 route has more fields than the gateway on a following line: %v", lines[i+1])
		}
		merged = append(merged, append(fields, nfields[0]))
		i++ // skip next line.
	}

	for _, parts := range merged {
		_, dstIP, err := net.ParseCIDR(parts[netdst])
		if err != nil {
			return nil, fmt.Errorf("invalid destination address: %v: %v", parts[netdst], err)
		}
		ifcIdx, err := strconv.Atoi(parts[ifc])
		if err != nil {
			return nil, fmt.Errorf("failed to parse interface index: %v: %v", parts[ifc], err)
		}

		var gw net.IP
		if gws := parts[gateway]; gws == "On-link" {
			addrs := ni.ipv6AddrForIndex(ifcIdx)
			if len(addrs) == 0 {
				return nil, fmt.Errorf("no addresses found for interface %v", ifcIdx)
			}
			gw = addrs[0]
		} else {
			gw = net.ParseIP(gws)
			if gw == nil {
				return nil, fmt.Errorf("invalid gateway: %v", gw)
			}
		}
		routes = append(routes, route.IPRoute{
			Net:      *dstIP,
			Gateway:  gw,
			IfcIndex: ifcIdx,
		})
	}
	return routes, nil
}

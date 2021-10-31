// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows
// +build windows

package osnetconfig

import (
	"net"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"v.io/x/lib/netconfig/route"
)

const testData = `===========================================================================
Interface List
  3...00 1c 42 74 78 6d ......Intel(R) 82574L Gigabit Network Connection
  1...........................Software Loopback Interface 1
===========================================================================

IPv4 Route Table
===========================================================================
Active Routes:
Network Destination        Netmask          Gateway       Interface  Metric
          0.0.0.0          0.0.0.0       172.16.1.1     172.16.1.235     25
        127.0.0.0        255.0.0.0         On-link         127.0.0.1    331
        127.0.0.1  255.255.255.255         On-link         127.0.0.1    331
  127.255.255.255  255.255.255.255         On-link         127.0.0.1    331
       172.16.1.0    255.255.255.0         On-link      172.16.1.235    281
     172.16.1.235  255.255.255.255         On-link      172.16.1.235    281
     172.16.1.255  255.255.255.255         On-link      172.16.1.235    281
        224.0.0.0        240.0.0.0         On-link         127.0.0.1    331
        224.0.0.0        240.0.0.0         On-link      172.16.1.235    281
  255.255.255.255  255.255.255.255         On-link         127.0.0.1    331
  255.255.255.255  255.255.255.255         On-link      172.16.1.235    281
===========================================================================
Persistent Routes:
  None

IPv6 Route Table
===========================================================================
Active Routes:
 If Metric Network Destination      Gateway
  1    331 ::1/128                  On-link
  3    281 fe80::/64                On-link
  3    281 fe80::4986:e542:6726:73ed/128
                                    On-link
  1    331 ff00::/8                 On-link
  3    281 ff00::/8                 On-link
===========================================================================
`

func TestGetRoutes(t *testing.T) {
	notifier := NewNotifier(time.Second)
	routes := notifier.GetIPRoutes(false)
	if routes == nil {
		t.Errorf("failed to get any system routes")
	}
	if len(routes) < 2 {
		t.Errorf("too few routes")
	}
}

func fakeInterfaceInfo() netIfcs {
	a31, _, _ := net.ParseCIDR("fe80::4986:e542:6726:73ed/64")
	a32, _, _ := net.ParseCIDR("172.16.1.235/24")
	a11, _, _ := net.ParseCIDR("::1/128")
	a12, _, _ := net.ParseCIDR("127.0.0.1/8")
	return []netIfc{
		{3, []net.IP{a31, a32}},
		{1, []net.IP{a11, a12}},
	}
}

func TestRouteParsing(t *testing.T) {
	nifcs := fakeInterfaceInfo()
	routes, err := parseWindowsRouteCommandOutput(nifcs, testData, false)
	if err != nil {
		t.Errorf("failed to parse route command's output: %v", err)
	}
	if got, want := len(routes), 16; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	defaultRoute := route.IPRoute{
		Net: net.IPNet{
			IP:   net.ParseIP("0.0.0.0"),
			Mask: net.CIDRMask(0, 32),
		},
		Gateway:  net.ParseIP("172.16.1.1"),
		IfcIndex: 3,
	}
	if got, want := routes[0], defaultRoute; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	ip, ipn, _ := net.ParseCIDR("fe80::4986:e542:6726:73ed/128")
	rt := route.IPRoute{
		Net:      *ipn,
		Gateway:  ip,
		IfcIndex: 3,
	}
	if got, want := routes[13], rt; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	routes, err = parseWindowsRouteCommandOutput(nifcs, testData, true)
	if err != nil {
		t.Errorf("failed to parse route command's output: %v", err)
	}
	if got, want := len(routes), 1; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got, want := routes[0], defaultRoute; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRouteParsingErrors(t *testing.T) {
	var err error
	expect := func(msg string) {
		_, _, line, _ := runtime.Caller(1)
		if err == nil {
			t.Errorf("line %v: expected an error", line)
			return
		}
		if !strings.Contains(err.Error(), msg) {
			t.Errorf("line %v: error %q does not contain: %q", line, err, msg)
		}
	}
	ifcInfo := fakeInterfaceInfo()
	_, err = parseWindowsRouteCommandOutput(ifcInfo, "", false)
	expect("no IPv4 Route Table found")
	_, err = parseWindowsRouteCommandOutput(ifcInfo, `IPv4 Route Table
===========================================================================
`, false)
	expect("no IPv4 Route Table found")
	_, err = parseWindowsRouteCommandOutput(ifcInfo, `IPv4 Route Table
===========================================================================
===========================================================================
`, false)
	expect("no IPv4 Route Table found")
	_, err = parseWindowsRouteCommandOutput(ifcInfo, `IPv4 Route Table
  ===========================================================================
  Active Routes:
  Network Destination        Netmask          Gateway       Interface  Metric
  ===========================================================================
  `, false)
	expect("no IPv4 Route Table found")

	_, err = parseWindowsRouteCommandOutput(ifcInfo, `IPv4 Route Table
===========================================================================
Active Routes:
Network Destination        Netmask          Gateway       Interface  Metric
          0.0.0.0          0.0.0.0       172.16.1.1     172.16.1.235     25
`, false)
	expect("failed to find end")

	_, err = parseWindowsRouteCommandOutput(ifcInfo, `IPv4 Route Table
===========================================================================
Active Routes:
Network Destination        Netmask          Gateway       Interface  Metric
          0.0.0.0          0.0.0.0       172.16.1.1     172.16.1.235     25
===========================================================================
`, false)
	expect("no IPv6 Route Table found")

	_, err = parseWindowsRouteCommandOutput(ifcInfo, `IPv4 Route Table
===========================================================================
Active Routes:
Network Destination        Netmask          Gateway       Interface  Metric
          0.0.0.0          0.0.0.0       172.16.1.1     172.16.1.235     25
===========================================================================
IPv6 Route Table
===========================================================================
Active Routes:
 If Metric Network Destination      Gateway
  3    281 fe80::4986:e542:6726:73ed/128
`, false)
	expect("failed to find end")
}

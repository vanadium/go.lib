// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin dragonfly freebsd netbsd openbsd

package osnetconfig

// We connect to the Route socket and parse messages to
// look for network configuration changes.  This is generic
// to all BSD based systems (including MacOS).  The net
// library already has code to parse the messages so all
// we need to do is look for message types.

import (
	"errors"
	"fmt"
	"net"
	"syscall"

	xroute "golang.org/x/net/route"
	"v.io/x/lib/netconfig/route"
	"v.io/x/lib/vlog"
)

/*
#include <sys/socket.h>
*/
import "C"

func (n *Notifier) initLocked() error {
	s, err := syscall.Socket(C.PF_ROUTE, syscall.SOCK_RAW, syscall.AF_UNSPEC)
	if err != nil {
		return fmt.Errorf("socket(PF_ROUTE, SOCK_RAW, AF_UNSPEC) failed: %v", err)
	}
	go watcher(n, s)
	return nil
}

func watcher(n *Notifier, sock int) {
	defer syscall.Close(sock)
	buf := make([]byte, 4096)
	for {
		nr, err := syscall.Read(sock, buf)
		if err != nil {
			vlog.Infof("read(%d) on an PF_ROUTE socket failed: %v", sock, err)
			return
		}
		msgs, err := xroute.ParseRIB(xroute.RIBTypeRoute, buf[:nr])
		if err != nil {
			vlog.Infof("ParseRoutingMessage failed: %s", err)
			continue
		}
		for _, m := range msgs {
			switch m.(type) {
			case *xroute.InterfaceMessage:
			case *xroute.InterfaceAddrMessage:
			case *xroute.RouteMessage:
			default:
				continue
			}
			if n.ding() {
				return
			}
			break
		}
	}
}

func toIP(sa xroute.Addr) (net.IP, error) {
	switch v := sa.(type) {
	case *xroute.Inet4Addr:
		return net.IPv4(v.IP[0], v.IP[1], v.IP[2], v.IP[3]), nil
	case *xroute.Inet6Addr:
		return net.IP(v.IP[:]), nil
	}
	return net.IPv6zero, errors.New("unknown sockaddr ip")
}

func toIPNet(sa xroute.Addr, msa xroute.Addr) (net.IPNet, error) {
	var x net.IPNet
	var err error
	x.IP, err = toIP(sa)
	if err != nil {
		return x, err
	}
	switch v := msa.(type) {
	case *xroute.Inet4Addr:
		x.Mask = net.IPv4Mask(v.IP[0], v.IP[1], v.IP[2], v.IP[3])
		return x, nil
	case *xroute.Inet6Addr:
		x.Mask = net.IPMask(v.IP[:])
		return x, nil
	}
	return x, errors.New("unknown sockaddr ipnet")
}

func (n *Notifier) shutdown() {}

// GetIPRoutes implements netconfig.Notifier.
func (n *Notifier) GetIPRoutes(defaultOnly bool) []route.IPRoute {
	var x []route.IPRoute
	rib, err := xroute.FetchRIB(0, xroute.RIBTypeRoute, 0)
	if err != nil {
		vlog.Infof("Couldn't read: %s", err)
		return x
	}
	msgs, err := xroute.ParseRIB(xroute.RIBTypeRoute, rib)
	if err != nil {
		vlog.Infof("Couldn't parse: %s", err)
		return x
	}
	for _, m := range msgs {
		if v, ok := m.(*xroute.RouteMessage); ok {
			addrs := v.Addrs
			if len(addrs) < 3 {
				continue
			}
			if addrs[0] == nil || addrs[1] == nil || addrs[2] == nil {
				continue
			}
			r := route.IPRoute{}
			if r.Gateway, err = toIP(addrs[1]); err != nil {
				continue
			}
			if r.Net, err = toIPNet(addrs[0], addrs[2]); err != nil {
				continue
			}
			r.IfcIndex = v.Index
			if !defaultOnly || route.IsDefaultIPRoute(&r) {
				x = append(x, r)
			}
		}
	}
	return x
}

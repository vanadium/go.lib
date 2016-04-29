// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin dragonfly freebsd netbsd openbsd

package netconfig

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

	"v.io/x/lib/vlog"
)

/*
#include <sys/socket.h>
*/
import "C"

func (n *notifier) initLocked() error {
	s, err := syscall.Socket(C.PF_ROUTE, syscall.SOCK_RAW, syscall.AF_UNSPEC)
	if err != nil {
		return fmt.Errorf("socket(PF_ROUTE, SOCK_RAW, AF_UNSPEC) failed: %v", err)
	}
	go watcher(n, s)
	return nil
}

func watcher(n *notifier, sock int) {
	defer syscall.Close(sock)
	buf := make([]byte, 4096)
	for {
		nr, err := syscall.Read(sock, buf)
		if err != nil {
			vlog.Infof("read(%d) on an PF_ROUTE socket failed: %v", sock, err)
			return
		}
		msgs, err := syscall.ParseRoutingMessage(buf[:nr])
		if err != nil {
			vlog.Infof("ParseRoutingMessage failed: %s", err)
			continue
		}
		for _, m := range msgs {
			switch m.(type) {
			case *syscall.InterfaceMessage:
			case *syscall.InterfaceAddrMessage:
			case *syscall.RouteMessage:
			default:
				continue
			}
			n.ding()
			break
		}
	}
}

func toIP(sa syscall.Sockaddr) (net.IP, error) {
	switch v := sa.(type) {
	case *syscall.SockaddrInet4:
		return net.IPv4(v.Addr[0], v.Addr[1], v.Addr[2], v.Addr[3]), nil
	case *syscall.SockaddrInet6:
		return net.IP(v.Addr[:]), nil
	}
	return net.IPv6zero, errors.New("unknown sockaddr ip")
}

func toIPNet(sa syscall.Sockaddr, msa syscall.Sockaddr) (net.IPNet, error) {
	var x net.IPNet
	var err error
	x.IP, err = toIP(sa)
	if err != nil {
		return x, err
	}
	switch v := msa.(type) {
	case *syscall.SockaddrInet4:
		x.Mask = net.IPv4Mask(v.Addr[0], v.Addr[1], v.Addr[2], v.Addr[3])
		return x, nil
	case *syscall.SockaddrInet6:
		x.Mask = net.IPMask(v.Addr[:])
		return x, nil
	}
	return x, errors.New("unknown sockaddr ipnet")
}

// IPRoutes returns all kernel known routes.  If defaultOnly is set, only default routes
// are returned.
func GetIPRoutes(defaultOnly bool) []*IPRoute {
	var x []*IPRoute
	rib, err := syscall.RouteRIB(syscall.NET_RT_DUMP, 0)
	if err != nil {
		vlog.Infof("Couldn't read: %s", err)
		return x
	}
	msgs, err := syscall.ParseRoutingMessage(rib)
	if err != nil {
		vlog.Infof("Couldn't parse: %s", err)
		return x
	}
	for _, m := range msgs {
		switch v := m.(type) {
		case *syscall.RouteMessage:
			addrs, err := syscall.ParseRoutingSockaddr(m)
			if err != nil {
				return x
			}
			if addrs[0] == nil || addrs[1] == nil || addrs[2] == nil {
				continue
			}
			r := new(IPRoute)
			if r.Gateway, err = toIP(addrs[1]); err != nil {
				continue
			}
			if r.Net, err = toIPNet(addrs[0], addrs[2]); err != nil {
				continue
			}
			r.IfcIndex = int(v.Header.Index)
			if !defaultOnly || IsDefaultIPRoute(r) {
				x = append(x, r)
			}
		}
	}
	return x
}

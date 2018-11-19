// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netstate

import (
	"net"

	"v.io/x/lib/netconfig/route"
)

type ma struct {
	n, a string
}

func (a *ma) Network() string {
	return a.n
}

func (a *ma) String() string {
	return a.a
}

func MkAddr(n, a string) net.Addr {
	ip := net.ParseIP(a)
	if ip == nil {
		ip, _, _ = net.ParseCIDR(a)
	}
	return &ma{n: n, a: ip.String()}
}

func NewIPAddr(n, a string) Address {
	ip := net.ParseIP(a)
	return NewAddr(n, ip.String())
}

func NewAddr(n, a string) Address {
	return &address{
		addr: &ma{n: n, a: a},
	}
}

func NewInterface(name string, index int, addrs []net.Addr, routes []route.IPRoute) NetworkInterface {
	return &ipifc{
		name:     name,
		index:    index,
		addrs:    addrs,
		ipRoutes: routes,
	}
}

func AddRoute(ifc NetworkInterface, rt route.IPRoute) {
	ii := ifc.(*ipifc)
	ii.ipRoutes = append(ii.ipRoutes, rt)
}

func RemoveLastRoute(ifc NetworkInterface) {
	ii := ifc.(*ipifc)
	ii.ipRoutes = ii.ipRoutes[:len(ii.ipRoutes)-1]
}

func SetRoutes(ifc NetworkInterface, rt ...route.IPRoute) {
	ii := ifc.(*ipifc)
	ii.ipRoutes = rt
}

func CreateAndUseMockCache(ifcs []NetworkInterface, routetable RouteTable) func() {
	prev := internalCache
	internalCache = &netstateCache{
		current:    true,
		interfaces: ifcs,
		routes:     routetable,
		valid:      make(chan struct{}),
	}
	return func() {
		internalCache = prev
		InvalidateCache()
	}
}

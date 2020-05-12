// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netstate_test

import (
	"net"

	"v.io/x/lib/netconfig/route"
	"v.io/x/lib/netstate"
)

func mockInterfacesAndRouteTable() ([]net.IP, []netstate.NetworkInterface, netstate.RouteTable) {
	// CIDR blocks as returned by net.Interfaces
	net1A0 := "192.168.1.10/24"
	net1A1 := "192.168.1.20/24"
	net2A := "172.16.1.11/24"
	net3A := "172.16.2.12/24"
	net4A := "172.19.39.142/23"
	net5A := "2620::1000:5e01:56e4:3aff:fef1:1383/64"
	lbA := "127.0.0.1/0"

	// Destination gateways
	defGW0 := net.ParseIP("172.16.2.12")
	defGW1 := net.ParseIP("172.16.2.1")
	net1GW := net.ParseIP("192.168.1.11")
	net2GW := net.ParseIP("172.16.1.12")
	net3GW := net.ParseIP("172.16.2.12")
	net4GW := net.ParseIP("fe80::5:73ff:fea0:fb2")
	net5GW := net.ParseIP("fe80::5:73ff:fea0:fb")

	net1IP, net1, _ := net.ParseCIDR(net1A0)
	net1aIP, _, _ := net.ParseCIDR(net1A1)
	net2IP, net2, _ := net.ParseCIDR(net2A)
	net3IP, net3, _ := net.ParseCIDR(net3A)
	net4IP, net4, _ := net.ParseCIDR(net4A)
	net5IP, net5, _ := net.ParseCIDR(net5A)
	lbIP, lbNet, _ := net.ParseCIDR(lbA)

	_, defaultDest, _ := net.ParseCIDR("0.0.0.0/0")
	def := route.IPRoute{
		Net:             *defaultDest,
		Gateway:         defGW0,
		PreferredSource: defGW1,
		IfcIndex:        3,
	}
	rt1 := []route.IPRoute{{
		Net:             *net1,
		Gateway:         net1GW,
		PreferredSource: nil,
		IfcIndex:        1,
	}}
	rt2 := []route.IPRoute{{
		Net:             *net2,
		Gateway:         net2GW,
		PreferredSource: nil,
		IfcIndex:        2,
	}}
	rt3 := []route.IPRoute{{
		Net:             *net3,
		Gateway:         net3GW,
		PreferredSource: nil,
		IfcIndex:        3,
	}, def}
	// Nets 4 and 5 are on the same interface
	rt4_0 := route.IPRoute{
		Net:             *net4,
		Gateway:         net4GW,
		PreferredSource: nil,
		IfcIndex:        6,
	}
	rt4_1 := route.IPRoute{
		Net:             *net5,
		Gateway:         net5GW,
		PreferredSource: nil,
		IfcIndex:        6,
	}
	rt4 := []route.IPRoute{rt4_0, rt4_1}
	lb := []route.IPRoute{{
		Net:             *lbNet,
		Gateway:         lbIP,
		PreferredSource: nil,
		IfcIndex:        7,
	}, def}

	rt := make(netstate.RouteTable)
	for _, r := range [][]route.IPRoute{rt1, rt2, rt3, rt4} {
		rt[r[0].IfcIndex] = r
	}

	ips := []net.IP{net1IP, net1aIP, net2IP, net3IP, net4IP, net5IP}

	ipa := netstate.MkAddr

	ifcdata := []struct {
		name   string
		index  int
		addrs  []net.Addr
		routes []route.IPRoute
	}{
		{"eth0", 1, []net.Addr{ipa("ip", net1A0), ipa("ip6", net1A1)}, rt1},
		{"eth1", 2, []net.Addr{ipa("ip", net2A)}, rt2},
		{"eth2", 3, []net.Addr{ipa("ip", net3A)}, rt3},
		{"wn0", 6, []net.Addr{ipa("ip", net4A), ipa("ip6", net5A)}, rt4},
		{"ln0", 7, []net.Addr{ipa("ip", lbA)}, lb},
	}

	ifcs := []netstate.NetworkInterface{}
	for _, ifc := range ifcdata {
		ifcs = append(ifcs, netstate.NewInterface(ifc.name, ifc.index, ifc.addrs, ifc.routes))
	}

	return ips, ifcs, rt
}

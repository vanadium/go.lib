// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netstate_test

import (
	"net"

	"v.io/x/lib/netconfig"
	"v.io/x/lib/netstate"
)

func mockInterfacesAndRouteTable() ([]net.IP, []netstate.NetworkInterface, netstate.RouteTable) {
	// CIDR blocks as returned by net.Interfaces
	net1_a0 := "192.168.1.10/24"
	net1_a1 := "192.168.1.20/24"
	net2_a := "172.16.1.11/24"
	net3_a := "172.16.2.12/24"
	net4_a := "172.19.39.142/23"
	net5_a := "2620::1000:5e01:56e4:3aff:fef1:1383/64"
	lb_a := "127.0.0.1/0"

	// Destination gateways
	def_gw0 := net.ParseIP("172.16.2.12")
	def_gw1 := net.ParseIP("172.16.2.1")
	net1_gw := net.ParseIP("192.168.1.11")
	net2_gw := net.ParseIP("172.16.1.12")
	net3_gw := net.ParseIP("172.16.2.12")
	net4_gw := net.ParseIP("fe80::5:73ff:fea0:fb2")
	net5_gw := net.ParseIP("fe80::5:73ff:fea0:fb")

	net1_ip, net1, _ := net.ParseCIDR(net1_a0)
	net1a_ip, _, _ := net.ParseCIDR(net1_a1)
	net2_ip, net2, _ := net.ParseCIDR(net2_a)
	net3_ip, net3, _ := net.ParseCIDR(net3_a)
	net4_ip, net4, _ := net.ParseCIDR(net4_a)
	net5_ip, net5, _ := net.ParseCIDR(net5_a)
	lb_ip, lb_net, _ := net.ParseCIDR(lb_a)

	_, defaultDest, _ := net.ParseCIDR("0.0.0.0/0")
	def := netconfig.IPRoute{
		Net:             *defaultDest,
		Gateway:         def_gw0,
		PreferredSource: def_gw1,
		IfcIndex:        3,
	}
	rt1 := []*netconfig.IPRoute{{
		Net:             *net1,
		Gateway:         net1_gw,
		PreferredSource: nil,
		IfcIndex:        1,
	}}
	rt2 := []*netconfig.IPRoute{{
		Net:             *net2,
		Gateway:         net2_gw,
		PreferredSource: nil,
		IfcIndex:        2,
	}}
	rt3 := []*netconfig.IPRoute{{
		Net:             *net3,
		Gateway:         net3_gw,
		PreferredSource: nil,
		IfcIndex:        3,
	}, &def}
	// Nets 4 and 5 are on the same interface
	rt4_0 := &netconfig.IPRoute{
		Net:             *net4,
		Gateway:         net4_gw,
		PreferredSource: nil,
		IfcIndex:        6,
	}
	rt4_1 := &netconfig.IPRoute{
		Net:             *net5,
		Gateway:         net5_gw,
		PreferredSource: nil,
		IfcIndex:        6,
	}
	rt4 := []*netconfig.IPRoute{rt4_0, rt4_1}
	lb := []*netconfig.IPRoute{{
		Net:             *lb_net,
		Gateway:         lb_ip,
		PreferredSource: nil,
		IfcIndex:        7,
	}, &def}

	rt := make(netstate.RouteTable)
	for _, r := range [][]*netconfig.IPRoute{rt1, rt2, rt3, rt4} {
		rt[r[0].IfcIndex] = r
	}

	ips := []net.IP{net1_ip, net1a_ip, net2_ip, net3_ip, net4_ip, net5_ip}

	ipa := func(n string, a string) net.Addr {
		return netstate.MkAddr(n, a)
	}

	ifcdata := []struct {
		name   string
		index  int
		addrs  []net.Addr
		routes []*netconfig.IPRoute
	}{
		{"eth0", 1, []net.Addr{ipa("ip", net1_a0), ipa("ip6", net1_a1)}, rt1},
		{"eth1", 2, []net.Addr{ipa("ip", net2_a)}, rt2},
		{"eth2", 3, []net.Addr{ipa("ip", net3_a)}, rt3},
		{"wn0", 6, []net.Addr{ipa("ip", net4_a), ipa("ip6", net5_a)}, rt4},
		{"ln0", 7, []net.Addr{ipa("ip", lb_a)}, lb},
	}

	ifcs := []netstate.NetworkInterface{}
	for _, ifc := range ifcdata {
		ifcs = append(ifcs, netstate.NewInterface(ifc.name, ifc.index, ifc.addrs, ifc.routes))
	}

	return ips, ifcs, rt
}

// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netstate_test

import (
	"net"
	"reflect"
	"strings"
	"testing"

	"v.io/x/lib/netconfig"
	"v.io/x/lib/netconfig/osnetconfig"
	"v.io/x/lib/netconfig/route"
	"v.io/x/lib/netstate"
)

func init() {
	netconfig.SetOSNotifier(osnetconfig.NewNotifier(0))
}
func TestInterfaces(t *testing.T) {
	ifcs, err := netstate.GetAllInterfaces()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(ifcs), 1; got < want {
		t.Fatalf("got %v, want at least %v", got, want+1)
	}

	str := ifcs.String()
	if got, want := strings.Count(str, "("), len(ifcs); got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestRoutes(t *testing.T) {
	accessible, err := netstate.GetAccessibleIPs()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	interfaces, err := netstate.GetAllInterfaces()
	if err != nil {
		t.Fatal(err)
	}

	if len(interfaces) == 0 || len(accessible) == 0 {
		t.Errorf("expected non zero lengths, not %d and %d", len(interfaces), len(accessible))
	}

	testedRoutes := false
	for _, ifc := range interfaces {
		ipifc := ifc.(netstate.IPNetworkInterface)
		routes := ipifc.IPRoutes()

		str := routes.String()
		if got, want := strings.Count(str, "("), len(routes); got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		// Make sure that the routes refer to valid interfaces
		for _, r := range routes {
			found := false
			for _, ifc := range interfaces {
				if r.IfcIndex == ifc.Index() {
					found = true
					break
				}
			}
			testedRoutes = true
			if !found {
				t.Errorf("failed to find ifc index %d", r.IfcIndex)
			}
		}
	}

	// Any usable test host should have at least one interface with at least
	// one route.
	if !testedRoutes {
		t.Fatalf("failed to test any routes on this host")
	}

}

func TestDefaultRoutes(t *testing.T) {
	_, ifcs, rt := mockInterfacesAndRouteTable()
	cleanup := netstate.CreateAndUseMockCache(ifcs, rt)
	defer cleanup()

	interfaces, err := netstate.GetAllInterfaces()
	if err != nil {
		t.Fatal(err)
	}

	defaultRoute := interfaces[2].(netstate.IPNetworkInterface).IPRoutes().Filter(netstate.IsDefaultRoute)
	if got, want := len(defaultRoute), 1; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	for _, i := range []int{0, 1, 3} {
		defaultRoute := interfaces[i].(netstate.IPNetworkInterface).IPRoutes().Filter(netstate.IsDefaultRoute)
		if got, want := len(defaultRoute), 0; got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	address, err := netstate.AddressFromIP(net.ParseIP("172.16.2.12"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := netstate.IsOnDefaultRoute(address), true; got != want {
		t.Fatalf("got %v, want %v for %s", got, want, address)
	}
}

func cmpRoutes(a, b []route.IPRoute) bool {
	if len(a) != len(b) {
		return false
	}
	for i, r := range b {
		if !reflect.DeepEqual(a[i], r) {
			return false
		}
	}
	return true
}

func cmpAddr(a, b net.Addr) bool {
	return a.Network() == b.Network() && a.String() == b.String()
}

func cmpIPAddrs(a, b []net.Addr) bool {
	if len(a) != len(b) {
		return false
	}
	for i, r := range b {
		if !netstate.IsIPProtocol(a[i].Network()) || !netstate.IsIPProtocol(r.Network()) {
			return false
		}
		if a[i].String() != r.String() {
			return false
		}
	}
	return true
}
func TestRoutePredicate(t *testing.T) {
	ips, ifcs, rt := mockInterfacesAndRouteTable()
	cleanup := netstate.CreateAndUseMockCache(ifcs, rt)

	fromip := func(ip net.IP) netstate.Address {
		a, err := netstate.AddressFromIP(ip)
		if err != nil {
			t.Fatalf("failed to get address from net.IP %s: %v", ip, err)
		}
		return a
	}

	net1Addr := fromip(ips[0])
	net1aAddr := fromip(ips[1])
	net2Addr := fromip(ips[2])
	net3Addr := fromip(ips[3])
	net4Addr := fromip(ips[4])
	net5Addr := fromip(ips[5])

	if got, want := net1Addr.Interface().Name(), "eth0"; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	if got, want := ifcs[0].Addrs(), []net.Addr{net1Addr, net1aAddr}; !cmpIPAddrs(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	if got, want := net4Addr.Interface().Name(), "wn0"; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	if got, want := net4Addr.Interface().Index(), 6; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	if got, want := net3Addr.Interface().(netstate.IPNetworkInterface).IPRoutes(), rt[3]; !cmpRoutes(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	al := netstate.AddrList{}
	if got, want := al.Filter(netstate.IsOnDefaultRoute), (netstate.AddrList{}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	al = netstate.AddrList{net1Addr, net2Addr, net3Addr, net4Addr, net5Addr}
	if got, want := al.Filter(netstate.IsOnDefaultRoute), (netstate.AddrList{net3Addr}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	refresh := func() {
		cleanup()
		cleanup = netstate.CreateAndUseMockCache(ifcs, rt)
		al[0] = fromip(ips[0])
		al[1] = fromip(ips[2])
		al[2] = fromip(ips[3])
		al[3] = fromip(ips[4])
		al[4] = fromip(ips[5])
		net1Addr = fromip(ips[0])
		net2Addr = fromip(ips[2])
		net3Addr = fromip(ips[3])
		net4Addr = fromip(ips[4])
		net5Addr = fromip(ips[5])
	}

	defaultRoute := net.IPNet{
		IP:   net.IPv4zero,
		Mask: make([]byte, net.IPv4len),
	}

	// Make eth1 a default route.
	net2Net := rt[2][0].Net
	rt[2][0].Net = defaultRoute
	refresh()
	if got, want := al.Filter(netstate.IsOnDefaultRoute), (netstate.AddrList{net2Addr, net3Addr}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// Make wn0 a default route also, but it won't show up because it has
	// an IPv6 address.
	rt[3][0].Net = defaultRoute
	refresh()
	if got, want := al.Filter(netstate.IsOnDefaultRoute), (netstate.AddrList{net2Addr, net3Addr}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	net6Net0 := rt[6][0].Net
	net6GW0 := rt[6][0].Gateway
	rt[2][0].Net = net2Net // Restore the original route.
	rt[6][0].Net = defaultRoute
	rt[6][0].Gateway = net.IPv4zero // Need an ipv4 gateway
	refresh()
	if got, want := al.Filter(netstate.IsOnDefaultRoute), (netstate.AddrList{net3Addr, net4Addr}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// Shouldn't return the IPv6 default route so long as al doesn't
	// contain any IPv6 default routes.
	rt[6][0].Net = net6Net0
	rt[6][0].Gateway = net6GW0
	rt[6][1].Net = defaultRoute
	refresh()
	if got, want := al.Filter(netstate.IsOnDefaultRoute), (netstate.AddrList{net3Addr}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// Now that we have an IPv6 default route that matches an IPv6 gateway
	// we can expect to find the IPv6 host address
	rt[6][1].Net = net.IPNet{
		IP:   net.IPv6zero,
		Mask: make([]byte, net.IPv6len),
	}
	refresh()

	if got, want := al.Filter(netstate.IsOnDefaultRoute), (netstate.AddrList{net3Addr, net4Addr, net5Addr}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	cleanup()

}

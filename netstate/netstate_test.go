// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netstate_test

import (
	"fmt"
	"net"
	"reflect"
	"testing"

	"v.io/x/lib/netstate"
)

func TestGet(t *testing.T) {
	// We assume that this machine running this test has at least
	// one non-loopback interface.
	all, _, err := netstate.GetAllAddresses()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	all = all.Map(netstate.WithIPHost)
	accessible, err := netstate.GetAccessibleIPs()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(all) == 0 || len(accessible) == 0 {
		t.Errorf("expected non zero lengths, not %d and %d", len(all), len(accessible))
	}
	if len(accessible) > len(all) {
		t.Errorf("should never be more accessible addresses than 'all' addresses")
	}
	loopback := netstate.FindAdded(accessible, all)
	if got, want := loopback.Filter(netstate.IsLoopbackIP), loopback; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

type ma struct {
	n, a string
}

func (a *ma) Network() string {
	return a.n
}

func (a *ma) String() string {
	return a.a
}

func TestAsIP(t *testing.T) {
	lh := net.ParseIP("127.0.0.1")
	if got, want := netstate.AsIP(&net.IPAddr{IP: lh}), "127.0.0.1"; got == nil || got.String() != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := netstate.AsIP(&net.IPNet{IP: lh}), "127.0.0.1"; got == nil || got.String() != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := netstate.AsIP(&ma{"tcp", lh.String()}), "127.0.0.1"; got == nil || got.String() != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := netstate.AsIP(&ma{"tcp", "192.168.10.1/24"}), "192.168.10.1"; got == nil || got.String() != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := netstate.AsIP(&ma{"tcp", net.JoinHostPort(lh.String(), "100")}), "127.0.0.1"; got == nil || got.String() != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestConversions(t *testing.T) {
	// No underlying network interface.
	al := netstate.ConvertToAddresses([]net.Addr{netstate.NewNetAddr("tcp", "1.1.1.1")})
	if got, want := len(al), 1; got != want {
		t.Fatalf("got %v, want %v, al: %v", got, want, al)
	}
	if got, want := al[0].Interface(), netstate.NetworkInterface(nil); got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	// Not a netstate.Address, but has an underlying network interface.
	al = netstate.ConvertToAddresses([]net.Addr{netstate.NewNetAddr("tcp", "127.0.0.1")})
	if got, want := len(al), 1; got != want {
		t.Fatalf("got %v, want %v, al: %v", got, want, al)
	}
	if got, want := fmt.Sprintf("%T", al[0].Interface()), "netstate.ipifc"; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	// We should get the same instance, i.e. pointer, returned by
	// ConvertToAddresses as we pass in.
	all, _, _ := netstate.GetAllAddresses()
	lb := all.Filter(netstate.IsLoopbackIP).Filter(netstate.IsUnicastIPv4)
	for i, a := range lb {
		if !netstate.IsUnicastIPv4(a) {
			t.Fatalf("%v: %v is not a unicast IPv4 address", i, a)
		}
	}
	al = netstate.ConvertToAddresses(lb.AsNetAddrs())
	if got, want := len(al), len(lb); got != want {
		t.Fatalf("got %v, want %v, al: %v", got, want, al)
	}
	found := false
	for _, a := range all {
		if a == al[0] {
			found = true
		}
	}
	if !found {
		t.Fatalf("%v isn't one of in %v", al[0], all)
	}
}

type hw struct{}

func (*hw) Network() string { return "mac" }
func (*hw) String() string  { return "01:23:45:67:89:ab:cd:ef" }

func TestPredicates(t *testing.T) {
	hwa := &hw{}
	hwifc := netstate.NewAddr(hwa.Network(), hwa.String())
	if got, want := netstate.IsUnicastIP(hwifc), false; got != want {
		t.Errorf("got %t, want %t", got, want)

	}
	cases := []struct {
		f func(a netstate.Address) bool
		a string
		r bool
	}{
		{netstate.IsUnspecifiedIP, "0.0.0.0", true},
		{netstate.IsUnspecifiedIP, "::", true},
		{netstate.IsUnspecifiedIP, "127.0.0.1", false},
		{netstate.IsUnspecifiedIP, "::1", false},

		{netstate.IsLoopbackIP, "0.0.0.0", false},
		{netstate.IsLoopbackIP, "::", false},
		{netstate.IsLoopbackIP, "127.0.0.1", true},
		{netstate.IsLoopbackIP, "::1", true},

		{netstate.IsAccessibleIP, "0.0.0.0", false},
		{netstate.IsAccessibleIP, "::", false},
		{netstate.IsAccessibleIP, "127.0.0.1", false},
		{netstate.IsAccessibleIP, "::1", false},
		{netstate.IsAccessibleIP, "224.0.0.2", true},
		{netstate.IsAccessibleIP, "fc00:1234::", true},
		{netstate.IsAccessibleIP, "192.168.1.1", true},
		{netstate.IsAccessibleIP, "2001:4860:0:2001::68", true},

		{netstate.IsUnicastIP, "0.0.0.0", false},
		{netstate.IsUnicastIP, "::", false},
		{netstate.IsUnicastIP, "127.0.0.1", true},
		{netstate.IsUnicastIP, "::1", true},
		{netstate.IsUnicastIP, "192.168.1.2", true},
		{netstate.IsUnicastIP, "74.125.239.36", true},
		{netstate.IsUnicastIP, "224.0.0.2", false},
		{netstate.IsUnicastIP, "fc00:1235::", true},
		{netstate.IsUnicastIP, "ff01::01", false},
		{netstate.IsUnicastIP, "2001:4860:0:2001::69", true},

		{netstate.IsUnicastIPv4, "0.0.0.0", false},
		{netstate.IsUnicastIPv4, "::", false},
		{netstate.IsUnicastIPv4, "127.0.0.1", true},
		{netstate.IsUnicastIPv4, "::1", false},
		{netstate.IsUnicastIPv4, "192.168.1.3", true},
		{netstate.IsUnicastIPv6, "74.125.239.37", false},
		{netstate.IsUnicastIPv4, "224.0.0.2", false},
		{netstate.IsUnicastIPv4, "fc00:1236::", false},
		{netstate.IsUnicastIPv4, "ff01::02", false},
		{netstate.IsUnicastIPv4, "2001:4860:0:2001::6a", false},

		{netstate.IsUnicastIPv6, "0.0.0.0", false},
		{netstate.IsUnicastIPv6, "::", false},
		{netstate.IsUnicastIPv6, "127.0.0.1", false},
		{netstate.IsUnicastIPv6, "::1", true},
		{netstate.IsUnicastIPv6, "192.168.1.4", false},
		{netstate.IsUnicastIPv6, "74.125.239.38", false},
		{netstate.IsUnicastIPv6, "224.0.0.2", false},
		{netstate.IsUnicastIPv6, "fc00:1237::", true},
		{netstate.IsUnicastIPv6, "ff01::03", false},
		{netstate.IsUnicastIPv6, "2607:f8b0:4003:c00::6b", true},

		{netstate.IsPublicUnicastIP, "0.0.0.0", false},
		{netstate.IsPublicUnicastIP, "::", false},
		{netstate.IsPublicUnicastIP, "127.0.0.1", false},
		{netstate.IsPublicUnicastIP, "::1", false},
		{netstate.IsPublicUnicastIP, "192.168.1.2", false},
		{netstate.IsPublicUnicastIP, "74.125.239.39", true},
		{netstate.IsPublicUnicastIP, "224.0.0.2", false},
		// Arguably this is buggy, the fc00:/7 prefix is supposed to be
		// non-routable.
		{netstate.IsPublicUnicastIP, "fc00:1238::", true},
		{netstate.IsPublicUnicastIP, "ff01::01", false},
		{netstate.IsPublicUnicastIP, "2001:4860:0:2001::69", true},

		{netstate.IsPublicUnicastIPv4, "0.0.0.0", false},
		{netstate.IsPublicUnicastIPv4, "::", false},
		{netstate.IsPublicUnicastIPv4, "127.0.0.1", false},
		{netstate.IsPublicUnicastIPv4, "::1", false},
		{netstate.IsPublicUnicastIPv4, "192.168.1.3", false},
		{netstate.IsPublicUnicastIPv4, "74.125.239.40", true},
		{netstate.IsPublicUnicastIPv4, "224.0.0.2", false},
		{netstate.IsPublicUnicastIPv4, "fc00:1239::", false},
		{netstate.IsPublicUnicastIPv4, "ff01::02", false},
		{netstate.IsPublicUnicastIPv4, "2001:4860:0:2001::6a", false},

		{netstate.IsPublicUnicastIPv6, "0.0.0.0", false},
		{netstate.IsPublicUnicastIPv6, "::", false},
		{netstate.IsPublicUnicastIPv6, "127.0.0.1", false},
		{netstate.IsPublicUnicastIPv6, "::1", false},
		{netstate.IsPublicUnicastIPv6, "192.168.1.4", false},
		{netstate.IsPublicUnicastIPv6, "74.125.239.41", false},
		{netstate.IsPublicUnicastIPv6, "224.0.0.2", false},
		// Arguably this is buggy, the fc00:/7 prefix is supposed to be
		// non-routable.
		{netstate.IsPublicUnicastIPv6, "fc00:123a::", true},
		{netstate.IsPublicUnicastIPv6, "ff01::03", false},
		{netstate.IsPublicUnicastIPv6, "2607:f8b0:4003:c00::6b", true},
	}
	for i, c := range cases {
		net := "tcp"
		if got, want := c.f(netstate.NewIPAddr(net, c.a)), c.r; got != want {
			t.Errorf("#%d: %s %s: got %t, want %t", i+1, net, c.a, got, want)
		}
	}
}

var (
	a  = netstate.NewIPAddr("tcp4", "1.2.3.4")
	b  = netstate.NewIPAddr("tcp4", "1.2.3.5")
	c  = netstate.NewIPAddr("tcp4", "1.2.3.6")
	d  = netstate.NewIPAddr("tcp4", "1.2.3.7")
	a6 = netstate.NewIPAddr("tcp6", "2001:4860:0:2001::68")
	b6 = netstate.NewIPAddr("tcp6", "2001:4860:0:2001::69")
	c6 = netstate.NewIPAddr("tcp6", "2001:4860:0:2001::70")
	d6 = netstate.NewIPAddr("tcp6", "2001:4860:0:2001::71")
)

func TestRemoved(t *testing.T) {
	al := netstate.AddrList{a, b, c, a6, b6, c6}
	bl := netstate.AddrList{}

	// no changes.
	got, want := netstate.FindRemoved(al, al), netstate.AddrList{}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	// missing everything
	if got, want := netstate.FindRemoved(al, bl), al; !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}

	// missing nothing
	if got, want := netstate.FindRemoved(bl, al), (netstate.AddrList{}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}

	// remove some addresses
	bl = netstate.AddrList{a, b, a6, b6}
	if got, want := netstate.FindRemoved(al, bl), (netstate.AddrList{c, c6}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}

	// add some addresses
	bl = netstate.AddrList{a, b, c, a6, b6, c6, d6}
	if got, want := netstate.FindRemoved(al, bl), (netstate.AddrList{}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}

	// change some addresses
	bl = netstate.AddrList{a, b, d, a6, d6, c6}
	if got, want := netstate.FindRemoved(al, bl), (netstate.AddrList{c, b6}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestAdded(t *testing.T) {
	al := netstate.AddrList{a, b, c, a6, b6, c6}
	bl := netstate.AddrList{}

	// no changes.
	if got, want := netstate.FindAdded(al, al), (netstate.AddrList{}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}

	// add nothing
	if got, want := netstate.FindAdded(al, bl), bl; !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}

	// add everything
	if got, want := netstate.FindAdded(bl, al), al; !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}

	// add some addresses
	bl = netstate.AddrList{a, b, c, d, a6, b6, c6, d6}
	if got, want := netstate.FindAdded(al, bl), (netstate.AddrList{d, d6}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}

	// remove some addresses
	bl = netstate.AddrList{a, b, c, b6}
	if got, want := netstate.FindAdded(al, bl), (netstate.AddrList{}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}

	// change some addresses
	bl = netstate.AddrList{a, d, c, a6, d6, c6}
	if got, want := netstate.FindAdded(al, bl), (netstate.AddrList{d, d6}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}
}

// buildNonLocalhostTestAddress constructs a selection of test addresses
// that are local.
func buildNonLocalhostTestAddress(t *testing.T) []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Errorf("InterfaceAddrs() failed: %v\n", err)
	}

	ips := make([]string, 0, len(addrs))
	for _, a := range addrs {
		ip, _, err := net.ParseCIDR(a.String())
		if err != nil {
			t.Errorf("ParseCIDR() failed: %v\n", err)
		}
		ips = append(ips, net.JoinHostPort(ip.String(), "111"))
	}
	return ips
}

func TestSameMachine(t *testing.T) {
	cases := []struct {
		Addr *ma
		Same bool
		Err  error
	}{
		{
			Addr: &ma{
				n: "tcp",
				a: "batman.com:4444",
			},
			Same: false,
			Err:  nil,
		},
		{
			Addr: &ma{
				n: "tcp",
				a: "127.0.0.1:1000",
			},
			Same: true,
			Err:  nil,
		},
		{
			Addr: &ma{
				n: "tcp",
				a: "::1/128",
			},
			Same: false,
			Err:  &net.AddrError{Err: "too many colons in address", Addr: "::1/128"},
		},
	}

	for _, a := range buildNonLocalhostTestAddress(t) {
		cases = append(cases, struct {
			Addr *ma
			Same bool
			Err  error
		}{
			Addr: &ma{
				n: "tcp",
				a: a,
			},
			Same: true,
			Err:  nil,
		})
	}

	for _, v := range cases {
		issame, err := netstate.SameMachine(v.Addr)
		if !reflect.DeepEqual(err, v.Err) {
			t.Errorf("Bad error: got %#v, expected %#v\n", err, v.Err)
		}
		if issame != v.Same {
			t.Errorf("for Endpoint address %v: got %v, expected %v\n", v.Addr.a, issame, v.Same)
		}
	}
}

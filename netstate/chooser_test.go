// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netstate_test

import (
	"fmt"
	"net"
	"testing"

	"v.io/x/lib/netstate"
)

func TestChooser(t *testing.T) {
	_, ifcs, rt := mockInterfacesAndRouteTable()
	cleanup := netstate.CreateAndUseMockCache(ifcs, rt)
	defer cleanup()

	chooser := netstate.AddressChooserFunc(func(protocol string, addrs []net.Addr) ([]net.Addr, error) {
		r := []net.Addr{}
		for _, a := range netstate.ConvertToAddresses(addrs) {
			if netstate.IsIPProtocol(a.Network()) {
				r = append(r, a)
			}
		}
		return r, nil
	})

	addrs, unspecified, err := netstate.PossibleAddresses("tcp", "0.0.0.0", chooser)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := unspecified, true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := len(addrs), 7; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := addrs[0], netstate.MkAddr("ip", "192.168.1.10"); !cmpAddr(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
	for _, a := range addrs {
		ip := net.ParseIP(a.String())
		if ip == nil {
			t.Fatalf("failed to parse %q", addrs[0].String())
		}
	}

	addrs, unspecified, err = netstate.PossibleAddresses("tcp", "172.16.2.12", chooser)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := unspecified, false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := len(addrs), 1; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	ip := net.ParseIP(addrs[0].String())
	if ip == nil {
		t.Fatalf("failed to parse %q", addrs[0].String())
	}

	naddr, err := netstate.AddressFromIP(ip)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := naddr.Interface().Index(), 3; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestChooserWithPorts(t *testing.T) {
	_, ifcs, rt := mockInterfacesAndRouteTable()
	cleanup := netstate.CreateAndUseMockCache(ifcs, rt)
	defer cleanup()

	chooser := netstate.AddressChooserFunc(func(protocol string, addrs []net.Addr) ([]net.Addr, error) {
		return addrs, nil
	})

	addrs, unspecified, err := netstate.PossibleAddresses("tcp", "0.0.0.0:30", chooser)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := unspecified, true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := len(addrs), 7; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	for _, a := range addrs {
		_, port, _ := net.SplitHostPort(a.String())
		if got, want := port, "30"; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}

	addrs, unspecified, err = netstate.PossibleAddresses("tcp", "192.168.1.10:31", chooser)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := unspecified, false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := len(addrs), 1; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	for _, a := range addrs {
		_, port, _ := net.SplitHostPort(a.String())
		if got, want := port, "31"; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}

}

func TestChooserNoMatches(t *testing.T) {
	_, ifcs, rt := mockInterfacesAndRouteTable()
	cleanup := netstate.CreateAndUseMockCache(ifcs, rt)
	defer cleanup()

	chooser := netstate.AddressChooserFunc(func(protocol string, addrs []net.Addr) ([]net.Addr, error) {
		return nil, nil
	})

	addrs, unspecified, err := netstate.PossibleAddresses("tcp", "0.0.0.0", chooser)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(addrs), 1; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	if got, want := addrs[0].String(), "0.0.0.0"; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	if got, want := unspecified, true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	addrs, unspecified, err = netstate.PossibleAddresses("tcp", "172.16.1.11", chooser)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(addrs), 1; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	address := addrs[0]
	if _, ok := address.(netstate.Address); !ok {
		t.Fatalf("%v has wront type %T", address, address)
	}
	if got, want := unspecified, false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestChooserIsNilAndErrors(t *testing.T) {
	_, ifcs, rt := mockInterfacesAndRouteTable()
	cleanup := netstate.CreateAndUseMockCache(ifcs, rt)
	defer cleanup()

	addrs, unspecified, err := netstate.PossibleAddresses("tcp", "0.0.0.0", nil)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(addrs), 7; got != want {
		// mockInterfacesAndRouteTable sets up with 7 IP addresses.
		t.Fatalf("got %v, want %v", got, want)
	}
	if got, want := unspecified, true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	errorChooser := netstate.AddressChooserFunc(func(protocol string, addrs []net.Addr) ([]net.Addr, error) {
		return nil, fmt.Errorf("a test error")
	})
	_, _, err = netstate.PossibleAddresses("tcp", "0.0.0.0", errorChooser)
	if err == nil || err.Error() != "a test error" {
		t.Fatal(err)
	}

	_, _, err = netstate.PossibleAddresses("xxx", "0.0.0.0", nil)
	if got, want := err, netstate.ErrNotAnIPProtocol; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	_, _, err = netstate.PossibleAddresses("tcp", "0.x.0.0", nil)
	if got, want := err, netstate.ErrFailedToParseIPAddr; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

}

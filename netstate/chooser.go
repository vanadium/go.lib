// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netstate

import (
	"errors"
	"net"
)

var (
	ErrNotAnIPProtocol = errors.New("requested protocol is not from the IP family")
)

// AddressChooser returns the address it prefers out of the set passed to it
// for the specified protocol. It should return an error if the requested protocol
// is not supported, but an empty slice of net.Addrs in the case for it fails to
// find any appropriate addresses rather than an error.
type AddressChooser func(protocol string, candidates []net.Addr) ([]net.Addr, error)

// PossibleAddresses returns the set of addresses that can be used to reach the
// specified host and that satisfy whatever policy is implemented by the supplied
// AddressChooser. It also returns an indication of whether the supplied host is
// unspecified or not. An unspecified host can be used over any network interface
// on the host. If the supplied address contains a port in then all of the
// returned addresses will also contain that port.
// The returned net.Addr's need have the exact same protocol as that passed
// in as a parameter, rather, the chooser should return net.Addr's that can
// be used for that protocol. Using tcp as a parameter for example will generally
// result in net.Addr's whose Network method returns "ip" or "ip6".
// If a nil chooser is supplied then it is assumed then LoopbackIPv4AddressChooser
// will be used.
// If the chooser fails to find any appropriate addresses then the protocol, addr
// parameters will be returned as net.Addr (and if possible as a netstate.Address).
//
// PossibleAddress currently only supports IP addresses.
func PossibleAddresses(protocol, addr string, chooser AddressChooser) ([]net.Addr, bool, error) {
	if !IsIPProtocol(protocol) {
		return nil, false, ErrNotAnIPProtocol
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
		port = ""
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return nil, false, ErrFailedToParseIPAddr
	}

	var candidates []net.Addr
	unspecified := ip.IsUnspecified()
	if unspecified {
		all, err := GetAllAddresses()
		if err != nil {
			return nil, unspecified, err
		}
		all = all.Map(WithIPHost)
		candidates = all.AsNetAddrs()
	} else {
		ipaddr, err := AddressFromIP(ip)
		if err != nil {
			return nil, unspecified, err
		}
		return []net.Addr{WithIPHostAndPort(ipaddr, port)}, unspecified, nil
	}
	if chooser == nil {
		addrs, err := LoopbackIPv4AddressChooser(protocol, candidates)
		return addrs, unspecified, err
	}
	chosen, err := chooser(protocol, candidates)
	if err != nil {
		return nil, unspecified, err
	}
	if len(chosen) == 0 {
		netaddr := NewNetAddr(protocol, addr)
		if address, err := AddressFromAddr(netaddr); err != nil {
			return []net.Addr{netaddr}, unspecified, nil
		} else {
			return []net.Addr{address}, unspecified, nil
		}
	}
	if len(port) > 0 {
		addPort := func(a Address) Address {
			return WithIPHostAndPort(a, port)
		}
		withPort := ConvertToAddresses(chosen).Map(addPort)
		chosen = withPort.AsNetAddrs()
	}
	return chosen, unspecified, nil
}

// LoopbackIPv4AddressChooser will return the loopback IPv4 address if it is found
// in the supplied candidates.
func LoopbackIPv4AddressChooser(protocol string, candidates []net.Addr) ([]net.Addr, error) {
	return ConvertToAddresses(candidates).Filter(IsLoopbackIP).Filter(IsUnicastIPv4).AsNetAddrs(), nil
}

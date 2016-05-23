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

// AddressChooser determines the preferred addresses to publish with the mount
// table when one is not otherwise specified.
type AddressChooser interface {
	ChooseAddresses(protocol string, candidates []net.Addr) ([]net.Addr, error)
}

// AddressChooserFunc is a convenience for implementations that wish to supply
// a function literal implementation of AddressChooser.
type AddressChooserFunc func(protocol string, candidates []net.Addr) ([]net.Addr, error)

func (f AddressChooserFunc) ChooseAddresses(protocol string, candidates []net.Addr) ([]net.Addr, error) {
	return f(protocol, candidates)
}

// PossibleAddresses returns the set of addresses that can be used to reach the
// specified host and that satisfy whatever policy is implemented by the supplied
// AddressChooser. It also returns an indication of whether the supplied host is
// unspecified or not. An unspecified host can be used over any network interface
// on the host. If the supplied address contains a port in then all of the
// returned addresses will also contain that port.
//
// The returned net.Addr's need have the exact same protocol as that passed
// in as a parameter, rather, the chooser should return net.Addr's that can
// be used for that protocol. Using tcp as a parameter for example will generally
// result in net.Addr's whose Network method returns "ip" or "ip6".
//
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
		all, _, err := GetAllAddresses()
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
	chosen := candidates
	if chooser != nil {
		if chosen, err = chooser.ChooseAddresses(protocol, candidates); err != nil {
			return nil, unspecified, err
		}
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

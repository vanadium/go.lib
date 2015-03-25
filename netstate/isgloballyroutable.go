// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netstate

import (
	"net"
)

var privateCIDRs = []net.IPNet{
	net.IPNet{IP: net.IPv4(10, 0, 0, 0), Mask: net.IPv4Mask(0xff, 0, 0, 0)},
	net.IPNet{IP: net.IPv4(172, 16, 0, 0), Mask: net.IPv4Mask(0xff, 0xf0, 0, 0)},
	net.IPNet{IP: net.IPv4(192, 168, 0, 0), Mask: net.IPv4Mask(0xff, 0xff, 0, 0)},
}

// IsGloballyRoutable returns true if the argument is a globally routable IP address.
func IsGloballyRoutableIP(ip net.IP) bool {
	if !ip.IsGlobalUnicast() {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil {
		for _, cidr := range privateCIDRs {
			if cidr.Contains(ip4) {
				return false
			}
		}
		if ip4.Equal(net.IPv4bcast) {
			return false
		}
	}
	return true
}

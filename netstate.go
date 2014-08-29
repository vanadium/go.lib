// Package netstate provides routines to obtain the available set of
// of network addresess, for determining changes to those addresses and for
// selecting from amongst them according to some set of policies that are
// implemented by applying simple predicates (functions with names of the form
// Is<condition> to filter or find the first matching address from a list
// of addresses. The intent is to make it easy to create policies that do
// things like 'find the first IPv4 unicast address that is globally routable,
// failing that use a private IPv4 address, and failing that, an IPv6 address'.
//
// A typical usage would be:
//
//   state, _ := netstate.GetAccessibleIPs()
//   first := netstate.First(netstate.IsPublicIPv4)
//   // first will contain the first public IPv4 address or be nil.
//
// The example policy described above would be implemented using a
// series of calls to First with appropriate predicates.
//
// Although most commercial networking hardware supports IPv6, some consumer
// devices and more importantly many ISPs do not, so routines are provided
// to allow developers to easily distinguish between the two and to use
// whichever is appropriate for their product/situation.
//
// The term 'accessible' is used to refer to any non-loopback IP address.
// The term 'public' is used to refer to any globally routable IP address.
//
// All IPv6 addresses are intended to be 'public', but any starting with
// fc00::/7 are (RFC4193) are reserved for private use, but the go
// net libraries do not appear to recognise this. Similarly fe80::/10
// (RFC 4291) are reserved for 'site-local' usage, but again this is not
// implemented in the go libraries. Any developer who needs to distinguish
// these cases will need to write their own routines to test for them.
//
// When using the go net package it is important to remember that IPv6
// addresses subsume IPv4 and hence in many cases the same internal
// representation is used for both, thus testing for the length of the IP
// address is unreliable. The reliable test is to use the net.To4() which
// will return a non-nil result if can be used as an IPv4 one. Any address
// can be used as an IPv6 and hence the only reliable way to test for an IPv6
// address that is not an IPv4 one also is for the To4 call to return nil for
// it.
package netstate

import (
	"fmt"
	"net"
	"strings"
)

type AddrList []net.Addr

func (al AddrList) String() string {
	r := ""
	for _, v := range al {
		r += fmt.Sprintf("(%s) ", v)
	}
	return strings.TrimRight(r, " ")
}

// GetAll gets all of the available addresses on the device, including
// loopback addresses.
func GetAll() (AddrList, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var all AddrList
	for _, ifc := range interfaces {
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		all = append(all, addrs...)
	}
	return all, nil
}

// GetAccessibleIPs returns all of the IP addresses on the device that are
// accessible to other devices - i.e. excluding loopback etc.
func GetAccessibleIPs() (AddrList, error) {
	all, err := GetAll()
	if err != nil {
		return nil, err
	}
	return all.Filter(IsAccessibleIP), nil
}

type Predicate func(a net.Addr) bool

// Filter returns all of the addresses for which the predicate
// function is true.
func (al AddrList) Filter(predicate Predicate) AddrList {
	r := AddrList{}
	for _, a := range al {
		if predicate(a) {
			r = append(r, a)
		}
	}
	return r
}

// Filter returns the first address for which the predicate function is true.
func (al AddrList) First(predicate Predicate) net.Addr {
	for _, a := range al {
		if predicate(a) {
			return a
		}
	}
	return nil
}

func IsIPNetwork(n string) bool {
	switch n {
	case "ip+net", "tcp", "tcp4", "tcp6", "udp":
		return true
	default:
		return false

	}
}

// AsIP returns its argument as a net.IP if that's possible.
func AsIP(a net.Addr) net.IP {
	ipn, ok := a.(*net.IPNet)
	if ok {
		return ipn.IP
	}
	ipa, ok := a.(*net.IPAddr)
	if ok {
		return ipa.IP
	}
	switch a.Network() {
	default:
		return nil
	case "ip+net", "tcp", "tcp4", "tcp6", "udp":
	}
	return net.ParseIP(a.String())
}

// IsUnspecified returns true if its argument is an unspecified IP address
func IsUnspecifiedIP(a net.Addr) bool {
	if ip := AsIP(a); ip != nil {
		return ip.IsUnspecified()
	}
	return false
}

// IsLoopback returns true if its argument is a loopback IP address
func IsLoopbackIP(a net.Addr) bool {
	if ip := AsIP(a); ip != nil && !ip.IsUnspecified() {
		return ip.IsLoopback()
	}
	return false
}

// IsAccessible returns true if its argument is an accessible (non-loopback)
// IP address.
func IsAccessibleIP(a net.Addr) bool {
	if ip := AsIP(a); ip != nil && !ip.IsUnspecified() {
		return !ip.IsLoopback()
	}
	return false
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

// AsAddr returns its argument as a net.Addr.
func AsAddr(network string, a net.IP) net.Addr {
	return &ma{n: network, a: a.String()}
}

// IsUnicastIP returns true if its argument is a unicast IP address.
func IsUnicastIP(a net.Addr) bool {
	if ip := AsIP(a); ip != nil && !ip.IsUnspecified() {
		// ipv4 or v6
		return !(ip.IsMulticast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast())
	}
	return false
}

// IsUnicastIPv4 returns true if its argument is a unicast IP4 address
func IsUnicastIPv4(a net.Addr) bool {
	if ip := AsIP(a); ip != nil && ip.To4() != nil && !ip.IsMulticast() {
		return !ip.IsUnspecified() && !ip.IsMulticast()
	}
	return false
}

// IsPublicUnicastIPv4 returns true if its argument is a globally routable,
// public IPv4 unicast address.
func IsPublicUnicastIPv4(a net.Addr) bool {
	if ip := AsIP(a); ip != nil && !ip.IsUnspecified() {
		if t := ip.To4(); t != nil && IsGloballyRoutableIP(t) {
			return !ip.IsMulticast()
		}
	}
	return false
}

// IsUnicastIPv6 returns true if its argument is a unicast IPv6 address
func IsUnicastIPv6(a net.Addr) bool {
	if ip := AsIP(a); ip != nil && ip.To4() == nil {
		return !ip.IsUnspecified() && !(ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast())
	}
	return false
}

// IsUnicastIPv6 returns true if its argument is a globally routable IP6 address
func IsPublicUnicastIPv6(a net.Addr) bool {
	if ip := AsIP(a); ip != nil && ip.To4() == nil {
		if t := ip.To16(); t != nil && IsGloballyRoutableIP(t) {
			return true
		}
	}
	return false
}

// IsPublicUnicastIP returns true if its argument is a global routable IPv4 or 6
// address.
func IsPublicUnicastIP(a net.Addr) bool {
	if ip := AsIP(a); ip != nil {
		if t := ip.To4(); t != nil && IsGloballyRoutableIP(t) {
			return true
		}
		if t := ip.To16(); t != nil && IsGloballyRoutableIP(t) {
			return true
		}
	}
	return false
}

func diffAB(a, b AddrList) AddrList {
	diff := AddrList{}
	for _, av := range a {
		found := false
		for _, bv := range b {
			if av.Network() == bv.Network() &&
				av.String() == bv.String() {
				found = true
				break
			}
		}
		if !found {
			diff = append(diff, av)
		}
	}
	return diff
}

// FindAdded returns the set of interfaces and/or addresses that are
// present in b, but not in a - i.e. have been added.
func FindAdded(a, b AddrList) AddrList {
	return diffAB(b, a)
}

// FindRemoved returns the set of interfaces and/or addresses that
// are present in a, but not in b - i.e. have been removed.
func FindRemoved(a, b AddrList) AddrList {
	return diffAB(a, b)
}

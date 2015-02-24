// Package netstate provides routines to obtain the available set of
// of network addresess, for determining changes to those addresses and for
// selecting from amongst them according to some set of policies that are
// implemented by applying simple predicates (functions with names of the form
// Is<condition>) to filter or find the first matching address from a list
// of addresses. The intent is to make it easy to create policies that do
// things like 'find the first IPv4 unicast address that is globally routable,
// failing that use a private IPv4 address, and failing that, an IPv6 address'.
//
// A simple usage would be:
//
//   state, _ := netstate.GetAccessibleIPs()
//   ipv4 := state.Filter(netstate.IsPublicUnicastIPv4)
//   // ipv4 will contain all of the public IPv4 addresses, if any.
//
// The example policy described above would be implemented using a
// series of calls to Filter with appropriate predicates.
//
// In some cases, it may be necessary to take IP routing information
// into account and hence interface hosting the address. The interface
// hosting each address is provided in the AddrIfc structure used to represent
// addresses and the IP routing information is provided by the GetAccessibleIPs
// function which will typically be used to obtain the available IP addresses.
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
// fc00::/7 (RFC4193) are reserved for private use, but the go
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

	"v.io/v23/ipc"

	"v.io/core/veyron/lib/netconfig"
)

// AddrIfc represents a network address and the network interface that
// hosts it.
type AddrIfc struct {
	// Network address
	Addr net.Addr

	// The name of the network interface this address is hosted on, empty
	// if this information is not available.
	Name string

	// The IPRoutes of the network interface this address is hosted on,
	// nil if this information is not available.
	IPRoutes []*netconfig.IPRoute
}

func (a *AddrIfc) String() string {
	if a.IPRoutes != nil {
		r := fmt.Sprintf("%s: %s[", a.Addr, a.Name)
		for _, rt := range a.IPRoutes {
			src := ""
			if rt.PreferredSource != nil {
				src = ", src: " + rt.PreferredSource.String()
			}
			r += fmt.Sprintf("{%d: net: %s, gw: %s%s}, ", rt.IfcIndex, rt.Net, rt.Gateway, src)
		}
		r = strings.TrimSuffix(r, ", ")
		r += "]"
		return r
	}
	return a.Addr.String()
}

func (a *AddrIfc) Address() net.Addr {
	return a.Addr
}

func (a *AddrIfc) InterfaceIndex() int {
	if len(a.IPRoutes) == 0 {
		return -1
	}
	return a.IPRoutes[0].IfcIndex
}

func (a *AddrIfc) InterfaceName() string {
	return a.Name
}

func (a *AddrIfc) Networks() []net.Addr {
	nets := []net.Addr{}
	for _, r := range a.IPRoutes {
		nets = append(nets, &r.Net)
	}
	return nets
}

type AddrList []ipc.Address

func (al AddrList) String() string {
	r := ""
	for _, v := range al {
		r += fmt.Sprintf("(%s) ", v)
	}
	return strings.TrimRight(r, " ")
}

// GetAll gets all of the available addresses on the device, including
// loopback addresses, non-IP protocols etc.
func GetAll() (AddrList, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	routes := netconfig.GetIPRoutes(false)
	routeTable := make(map[int][]*netconfig.IPRoute)
	for _, r := range routes {
		routeTable[r.IfcIndex] = append(routeTable[r.IfcIndex], r)
	}
	var all AddrList
	for _, ifc := range interfaces {
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			all = append(all, &AddrIfc{a, ifc.Name, routeTable[ifc.Index]})
		}
	}
	return all, nil
}

// GetAccessibleIPs returns all of the accessible IP addresses on the device
// - i.e. excluding loopback and unspecified addresses.
// The IP addresses returned will be host addresses.
func GetAccessibleIPs() (AddrList, error) {
	all, err := GetAll()
	if err != nil {
		return nil, err
	}
	return all.Map(ConvertAccessibleIPHost), nil
}

// AddressPredicate defines the function signature for predicate functions
// to be used with AddrList
type AddressPredicate func(a ipc.Address) bool

// Filter returns all of the addresses for which the predicate
// function is true.
func (al AddrList) Filter(predicate AddressPredicate) AddrList {
	r := AddrList{}
	for _, a := range al {
		if predicate(a) {
			r = append(r, a)
		}
	}
	return r
}

type Mapper func(a ipc.Address) ipc.Address

// Map will apply the Mapper function to all of the items in its receiver
// and return a new AddrList containing all of the non-nil results from
// said calls.
func (al AddrList) Map(mapper Mapper) AddrList {
	var ral AddrList
	for _, a := range al {
		if na := mapper(a); na != nil {
			ral = append(ral, na)
		}
	}
	return ral
}

// ConvertToIPHost converts the network address component of an ipc.Address into
// an instance with a net.Addr that contains an IP host address (as opposed to a
// network CIDR for example).
func ConvertToIPHost(a ipc.Address) ipc.Address {
	aifc, ok := a.(*AddrIfc)
	if !ok {
		return nil
	}
	aifc.Addr = AsIPAddr(aifc.Addr)
	return aifc
}

// ConvertAccessibleIPHost converts the network address component of an ipc.Address
// into an instance with a net.Addr that contains an IP host address (as opposed to a
// network CIDR for example) with filtering out a loopback or non-accessible IPs.
func ConvertAccessibleIPHost(a ipc.Address) ipc.Address {
	if !IsAccessibleIP(a) {
		return nil
	}
	aifc, ok := a.(*AddrIfc)
	if !ok {
		return nil
	}
	if ip := AsIPAddr(aifc.Addr); ip != nil {
		aifc.Addr = ip
	}
	return aifc
}

// IsIPProtocol returns true if its parameter is one of the allowed
// network/protocol values for IP.
func IsIPProtocol(n string) bool {
	// Removed the training IP version number.
	n = strings.TrimRightFunc(n, func(r rune) bool { return r == '4' || r == '6' })
	switch n {
	case "ip+net", "ip", "tcp", "udp", "ws", "wsh":
		return true
	default:
		return false
	}
}

// AsIPAddr returns its argument as a net.IPAddr if that's possible.
func AsIPAddr(a net.Addr) *net.IPAddr {
	if v, ok := a.(*net.IPAddr); ok {
		return v
	}
	if ipn, ok := a.(*net.IPNet); ok {
		return &net.IPAddr{IP: ipn.IP}
	}
	if IsIPProtocol(a.Network()) {
		if r := net.ParseIP(a.String()); r != nil {
			return &net.IPAddr{IP: r}
		}
	}
	return nil
}

// AsIP returns its argument as a net.IP if that's possible.
func AsIP(a net.Addr) net.IP {
	ipAddr := AsIPAddr(a)
	if ipAddr == nil {
		return nil
	}
	return ipAddr.IP
}

// IsUnspecified returns true if its argument is an unspecified IP address
func IsUnspecifiedIP(a ipc.Address) bool {
	if ip := AsIP(a.Address()); ip != nil {
		return ip.IsUnspecified()
	}
	return false
}

// IsLoopback returns true if its argument is a loopback IP address
func IsLoopbackIP(a ipc.Address) bool {
	if ip := AsIP(a.Address()); ip != nil && !ip.IsUnspecified() {
		return ip.IsLoopback()
	}
	return false
}

// IsAccessible returns true if its argument is an accessible (non-loopback)
// IP address.
func IsAccessibleIP(a ipc.Address) bool {
	if ip := AsIP(a.Address()); ip != nil && !ip.IsUnspecified() {
		return !ip.IsLoopback()
	}
	return false
}

// IsUnicastIP returns true if its argument is a unicast IP address.
func IsUnicastIP(a ipc.Address) bool {
	if ip := AsIP(a.Address()); ip != nil && !ip.IsUnspecified() {
		// ipv4 or v6
		return !(ip.IsMulticast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast())
	}
	return false
}

// IsUnicastIPv4 returns true if its argument is a unicast IP4 address
func IsUnicastIPv4(a ipc.Address) bool {
	if ip := AsIP(a.Address()); ip != nil && ip.To4() != nil {
		return !ip.IsUnspecified() && !ip.IsMulticast()
	}
	return false
}

// IsPublicUnicastIPv4 returns true if its argument is a globally routable,
// public IPv4 unicast address.
func IsPublicUnicastIPv4(a ipc.Address) bool {
	if ip := AsIP(a.Address()); ip != nil && !ip.IsUnspecified() {
		if t := ip.To4(); t != nil && IsGloballyRoutableIP(t) {
			return !ip.IsMulticast()
		}
	}
	return false
}

// IsUnicastIPv6 returns true if its argument is a unicast IPv6 address
func IsUnicastIPv6(a ipc.Address) bool {
	if ip := AsIP(a.Address()); ip != nil && ip.To4() == nil {
		return !ip.IsUnspecified() && !(ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast())
	}
	return false
}

// IsUnicastIPv6 returns true if its argument is a globally routable IP6
// address
func IsPublicUnicastIPv6(a ipc.Address) bool {
	if ip := AsIP(a.Address()); ip != nil && ip.To4() == nil {
		if t := ip.To16(); t != nil && IsGloballyRoutableIP(t) {
			return true
		}
	}
	return false
}

// IsPublicUnicastIP returns true if its argument is a global routable IPv4
// or 6 address.
func IsPublicUnicastIP(a ipc.Address) bool {
	if ip := AsIP(a.Address()); ip != nil {
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
			if av.Address().Network() == bv.Address().Network() &&
				av.Address().String() == bv.Address().String() {
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

// FindAdded returns the set addresses that are present in b, but not
// in a - i.e. have been added.
func FindAdded(a, b AddrList) AddrList {
	return diffAB(b, a)
}

// FindRemoved returns the set of addresses that are present in a, but not
// in b - i.e. have been removed.
func FindRemoved(a, b AddrList) AddrList {
	return diffAB(a, b)
}

// SameMachine returns true if the provided addr is on the device executing this
// function.
func SameMachine(addr net.Addr) (bool, error) {
	// The available interfaces may change between calls.
	addrs, err := GetAll()
	if err != nil {
		return false, err
	}
	ips := make(map[string]struct{})
	for _, a := range addrs {
		ip, _, err := net.ParseCIDR(a.Address().String())
		if err != nil {
			return false, err
		}
		ips[ip.String()] = struct{}{}
	}

	client, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return false, err
	}
	_, islocal := ips[client]
	return islocal, nil
}

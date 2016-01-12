// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package netstate implements utilities for retrieving and filtering network
// interface state.
//
// There are routines to obtain the available set of of network addresess, for
// determining changes to those addresses, and for selecting from amongst them
// according to some set of policies.  Polices are implemented by applying
// simple predicates (functions with names of the form Is<condition>) to filter
// or find the first matching address from a list of addresses.  The intent is
// to make it easy to create policies that do things like 'find the first IPv4
// unicast address that is globally routable, failing that use a private IPv4
// address, and failing that, an IPv6 address'.
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
// into account and hence the interface hosting the address. The interface
// hosting each address is provided via NetworkInterface. GetAllAddresses and
// GetAccessibleIPs return instances of Address that provide access to the
// network address and the network interface that hosts it.
//
// GetAll and GetAccessibleIPs cache the state of the network interfaces
// and routing information. This cache is invalidated by the Invalidate
// function which must be called whenever the network changes state (e.g.
// in response to dhcp changes).
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
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"v.io/x/lib/netconfig"
)

var (
	ErrUnsupportedProtocol   = errors.New("unsupported protocol")
	ErrFailedToParseIPAddr   = errors.New("failed to parse IP address")
	ErrUnspecifiedIPAddr     = errors.New("unspecified (i.e. zero) IP address")
	ErrFailedToFindInterface = errors.New("failed to find a network interface")
)

type netAddr struct {
	network string
	addr    string
}

func (hp *netAddr) Network() string {
	return hp.network
}

func (hp *netAddr) String() string {
	return hp.addr
}

func NewNetAddr(network, protocol string) net.Addr {
	return &netAddr{network, protocol}
}

// address represents a network address and the network interface that
// hosts it. It implements the Address interface.
type address struct {
	addr net.Addr
	ifc  NetworkInterface
}

// Implements Address
func (a *address) Network() string {
	return a.addr.Network()
}

// Implements Address
func (a *address) String() string {
	return a.addr.String()
}

func (a *address) DebugString() string {
	return fmt.Sprintf("[%s:%s %s:%d]", a.addr.Network(), a.addr.String(), a.ifc.Name(), a.ifc.Index())
}

// Implements Address
func (a *address) Interface() NetworkInterface {
	return a.ifc
}

// ipifc represents a network interface and associated routing information for
// IP networks.
type ipifc struct {
	// local copies of net.Interface data
	name         string
	index, mtu   int
	hardwareAddr net.HardwareAddr
	flags        net.Flags

	// copy of addresses so that we don't have to call into
	// the net package to get them again
	addrs []net.Addr
	// The IPRoutes of the network interface this address is hosted on,
	// nil if this information is not available.
	ipRoutes IPRouteList
}

// return a comma separated string of network addresses
func addrsToStr(addrs []net.Addr) string {
	r := ""
	for _, a := range addrs {
		r += a.String() + " "
	}
	return strings.TrimRight(r, " ")
}

// Implements NetworkInterface
func (ifc ipifc) String() string {
	r := fmt.Sprintf("(%s:%d %s flags[%s], mtu[%d], hw:[%s])", ifc.name, ifc.index, addrsToStr(ifc.addrs), ifc.flags, ifc.mtu, ifc.hardwareAddr)
	if len(ifc.ipRoutes) > 0 {
		r += " ["
		for _, rt := range ifc.ipRoutes {
			src := ""
			if rt.PreferredSource != nil {
				src = ", src: " + rt.PreferredSource.String()
			}
			r += fmt.Sprintf("{%d: net: %s, gw: %s%s}, ", rt.IfcIndex, rt.Net, rt.Gateway, src)
		}
		r = strings.TrimSuffix(r, ", ")
		r += "]"
	}
	return r
}

// Implements NetworkInterface
func (a ipifc) Addrs() []net.Addr {
	return a.addrs
}

// Implements NetworkInterface
func (a ipifc) Index() int {
	return a.index
}

// Implements NetworkInterface
func (a ipifc) Name() string {
	return a.name
}

// Implements NetworkInterface
func (a ipifc) MTU() int {
	return a.mtu
}

// Implements NetworkInterface
func (a ipifc) HardwareAddr() net.HardwareAddr {
	return a.hardwareAddr
}

// Implements NetworkInterface
func (a ipifc) Flags() net.Flags {
	return a.flags
}

// Implements NetworkInterface
func (a ipifc) Networks() []net.Addr {
	nets := []net.Addr{}
	for _, r := range a.ipRoutes {
		nets = append(nets, &r.Net)
	}
	return nets
}

// Implements IPNetworkInterface
func (a ipifc) IPRoutes() IPRouteList {
	routes := make(IPRouteList, len(a.ipRoutes))
	copy(routes, a.ipRoutes)
	return routes
}

// Network interface represents a network interface.
type NetworkInterface interface {
	// Addrs returns the addresses hosted by this interface.
	Addrs() []net.Addr
	// Index returns the index of this interface.
	Index() int
	// Name returns the name of this interface, e.g., "en0", "lo0", "eth0.100"
	Name() string
	// MTU returns the maximum transmission unit
	MTU() int
	// HardwareAddr returns the hardware address in IEEE MAC-48, EUI-48 and EUI-64 form
	HardwareAddr() net.HardwareAddr
	// Flags returns the flags for the interface e.g., FlagUp, FlagLoopback, FlagMulticast
	Flags() net.Flags
	// Networks returns the set of networks accessible over this interface.
	Networks() []net.Addr
	// String returns a string representation of the interface.
	String() string
}

// IPNetworkInterface represents a network interface supporting IP protocols.
type IPNetworkInterface interface {
	NetworkInterface
	IPRoutes() IPRouteList
}

// Address represents a network address and the interface that hosts it.
type Address interface {
	// Address returns the network address this instance represents.
	net.Addr
	Interface() NetworkInterface
	DebugString() string
}

// AddrList is a slice of Addresses.
type AddrList []Address

func (al AddrList) String() string {
	r := ""
	for _, v := range al {
		r += fmt.Sprintf("(%s) ", v)
	}
	return strings.TrimRight(r, " ")
}

// RouteTable represents the set of currently available network interfaces
// and the routes on each such interface. It is index by the index number
// of each interface.
type RouteTable map[int]IPRouteList

type netstateCache struct {
	mu         sync.RWMutex
	current    bool
	interfaces []NetworkInterface
	routes     RouteTable
	valid      chan struct{}
}

func (cache *netstateCache) invalidate() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if !cache.current {
		return
	}
	cache.current = false
	close(cache.valid)
}

func (cache *netstateCache) refresh() error {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cache.current {
		return nil

	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return err
	}
	routes := netconfig.GetIPRoutes(false)

	cache.interfaces = make([]NetworkInterface, len(interfaces))
	for i, ifc := range interfaces {
		addrs, err := ifc.Addrs()
		if err != nil {
			return err
		}
		cache.interfaces[i] = &ipifc{
			name:         ifc.Name,
			index:        ifc.Index,
			mtu:          ifc.MTU,
			flags:        ifc.Flags,
			hardwareAddr: ifc.HardwareAddr,
			addrs:        addrs,
		}
	}

	cache.routes = make(RouteTable)
	for _, r := range routes {
		cache.routes[r.IfcIndex] = append(cache.routes[r.IfcIndex], r)
	}
	cache.current = true
	cache.valid = make(chan struct{})
	return nil
}

func (cache *netstateCache) getNetState() ([]NetworkInterface, RouteTable, <-chan struct{}, error) {
	if err := cache.refresh(); err != nil {
		return nil, nil, nil, err
	}
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	ifcs := make([]NetworkInterface, len(cache.interfaces))
	copy(ifcs, cache.interfaces)

	rt := make(RouteTable)
	for k, v := range cache.routes {
		rt[k] = make(IPRouteList, len(v))
		copy(rt[k], v)
	}
	return ifcs, rt, cache.valid, nil
}

// Allow this to be overwritten by tests.
var internalCache *netstateCache

func init() {
	internalCache = &netstateCache{valid: make(chan struct{})}
}

// InvalidateCache invalidates any cached network state.
func InvalidateCache() {
	internalCache.invalidate()
}

// GetAllAddresses gets all of the available addresses on the device, including
// loopback addresses, non-IP protocols etc. The IP interface addresses
// returned are in CIDR form.
// GetAllAddressses caches the state of network interfaces and route tables to avoid
// expensive system calls (for routing information), the cache is invalidated
// by the Invalidate function which should be called whenever the network state
// may have changed (e.g. following a dhcp change).
// The returned chan is closed when the returned AddrList has become stale.
func GetAllAddresses() (AddrList, <-chan struct{}, error) {
	interfaces, routeTable, valid, err := internalCache.getNetState()
	if err != nil {
		return nil, nil, err
	}
	var all AddrList
	for _, ifc := range interfaces {
		for _, a := range ifc.Addrs() {
			na := &address{
				addr: a,
				ifc:  fillInterfaceInfo(ifc, routeTable[ifc.Index()]),
			}
			all = append(all, na)
		}

	}
	return all, valid, nil
}

// InterfaceList represents a list of network interfaces.
type InterfaceList []NetworkInterface

func fillInterfaceInfo(ifc NetworkInterface, rl IPRouteList) ipifc {
	n := ipifc{
		name:         ifc.Name(),
		index:        ifc.Index(),
		mtu:          ifc.MTU(),
		flags:        ifc.Flags(),
		hardwareAddr: ifc.HardwareAddr(),
		addrs:        ifc.Addrs(),
	}
	if rl != nil {
		n.ipRoutes = rl
	}
	return n
}

// GetAllInterfaces returns a list of all of the network interfaces on this
// device. It uses the same cache as GetAllAddresses.
func GetAllInterfaces() (InterfaceList, error) {
	interfaces, routeTable, _, err := internalCache.getNetState()
	r := []NetworkInterface{}
	for _, ifc := range interfaces {
		ipifc := fillInterfaceInfo(ifc, routeTable[ifc.Index()])
		r = append(r, &ipifc)
	}
	return r, err
}

func (ifcl InterfaceList) String() string {
	r := ""
	for _, ifc := range ifcl {
		r += fmt.Sprintf("%s, ", ifc)
	}
	return strings.TrimRight(r, ", ")
}

// GetAccessibleIPs returns all of the accessible IP addresses on the device
// - i.e. excluding loopback and unspecified addresses.
// The IP addresses returned will be host addresses.
func GetAccessibleIPs() (AddrList, error) {
	all, _, err := GetAllAddresses()
	if err != nil {
		return nil, err
	}

	convertAccessible := func(a Address) Address {
		ah := WithIPHost(a)
		if !IsAccessibleIP(ah) {
			return nil
		}
		return WithIPHost(ah)
	}

	return all.Map(convertAccessible), nil
}

// AsNetAddrs returns al as a slice of net.Addrs by changing the type
// of the slice that contains them and not by copying them.
func (al AddrList) AsNetAddrs() []net.Addr {
	r := make([]net.Addr, len(al))
	for i, a := range al {
		r[i] = a
	}
	return r
}

// ConvertToAddresses attempts to convert a slice of net.Addr's into
// an AddrList. It does so as follows:
// - using type assertion if the net.Addr instance is also an instance
//   of Address.
// - using AddressFromAddr.
// - filling in just the address portion of Address without any interface
//   information.
func ConvertToAddresses(addrs []net.Addr) AddrList {
	r := []Address{}
	for _, addr := range addrs {
		if a, ok := addr.(Address); ok {
			r = append(r, a)
			continue
		}
		if a, err := AddressFromAddr(addr); err == nil {
			r = append(r, a)
			continue
		} else {
			r = append(r, &address{addr: addr})
		}
	}
	return r
}

// AddressPredicate defines the function signature for predicate functions
// to be used with AddrList
type AddressPredicate func(a Address) bool

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

type Mapper func(a Address) Address

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

// WithIPHost returns an instance of Address with the network
// address component being an instance with a net.Addr that contains an
// IP host address (as opposed to a network CIDR for example).
func WithIPHost(a Address) Address {
	aifc := &address{}
	aifc.addr = AsIPAddr(a)
	aifc.ifc = ipifcFromNetIfc(a.Interface().(IPNetworkInterface))
	return aifc
}

// WithIPHostAndPort returns an instance of Address with the network
// address component being an instance of net.Addr that contains an
// IP host and port in : notation.
func WithIPHostAndPort(a Address, port string) Address {
	aifc, ok := a.(*address)
	if !ok {
		return nil
	}
	aifc.addr = AsIPAddr(aifc.addr)
	hostAndPort := a.String()
	if len(port) > 0 {
		hostAndPort = net.JoinHostPort(hostAndPort, port)
	}
	aifc.addr = &netAddr{a.Network(), hostAndPort}
	return aifc
}

func ipifcFromNetIfc(ifc IPNetworkInterface) ipifc {
	return ipifc{
		name:         ifc.Name(),
		index:        ifc.Index(),
		mtu:          ifc.MTU(),
		flags:        ifc.Flags(),
		hardwareAddr: ifc.HardwareAddr(),
		addrs:        ifc.Addrs(),
		ipRoutes:     ifc.IPRoutes(),
	}
}

// AddressFromAddr creates an instance of Address given the suppied
// net.Addr.  It will search through the available network interfaces
// to find the interface that hosts this address.
// It currently supports only IP protocols.
func AddressFromAddr(addr net.Addr) (Address, error) {
	if !IsIPProtocol(addr.Network()) {
		return nil, ErrUnsupportedProtocol
	}
	ip := net.ParseIP(addr.String())
	if ip == nil {
		return nil, ErrFailedToParseIPAddr
	}
	return AddressFromIP(ip)
}

// AddressFromIP creates an instance of Address given the supplied
// IP address. It will search through the available network interfaces
// to find the interface that hosts this IP address.
func AddressFromIP(ip net.IP) (Address, error) {
	if ip.IsUnspecified() {
		return nil, ErrUnspecifiedIPAddr
	}
	ifcs, _, _, err := internalCache.getNetState()
	if err != nil {
		return nil, err
	}
	for _, ifc := range ifcs {
		for _, ifaddr := range ifc.Addrs() {
			if !IsIPProtocol(ifaddr.Network()) {
				continue
			}
			cip := AsIP(ifaddr)
			if ip.Equal(cip) {
				addr := &address{addr: &net.IPAddr{IP: ip}}
				addr.ifc = ipifcFromNetIfc(ifc.(IPNetworkInterface))
				return addr, nil
			}
		}
	}
	return nil, ErrFailedToFindInterface
}

// IsIPProtocol returns true if its parameter is one of the allowed
// network/protocol values for IP. It considers the vanadium specific
// websockect protocols (wsh, wsh4, wsh6) as being IP.
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

// AsIPAddr returns its argument as a net.IPAddr if that's possible. If
// the address is an IP in host:port notation it will use the host portion
// only.
func AsIPAddr(a net.Addr) *net.IPAddr {
	switch v := a.(type) {
	case *net.IPAddr:
		return v
	case *net.IPNet:
		return &net.IPAddr{IP: v.IP}
	case *address:
		return AsIPAddr(v.addr)
	}
	if IsIPProtocol(a.Network()) {
		if r := net.ParseIP(a.String()); r != nil {
			return &net.IPAddr{IP: r}
		}
		if r, _, _ := net.ParseCIDR(a.String()); r != nil {
			return &net.IPAddr{IP: r}
		}
		host, _, _ := net.SplitHostPort(a.String())
		if r := net.ParseIP(host); r != nil {
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
func IsUnspecifiedIP(a Address) bool {
	if ip := AsIP(a); ip != nil {
		return ip.IsUnspecified()
	}
	return false
}

// IsLoopback returns true if its argument is a loopback IP address
func IsLoopbackIP(a Address) bool {
	if ip := AsIP(a); ip != nil && !ip.IsUnspecified() {
		return ip.IsLoopback()
	}
	return false
}

// IsAccessible returns true if its argument is an accessible (non-loopback)
// IP address.
func IsAccessibleIP(a Address) bool {
	if ip := AsIP(a); ip != nil && !ip.IsUnspecified() {
		return !ip.IsLoopback()
	}
	return false
}

// IsUnicastIP returns true if its argument is a unicast IP address.
func IsUnicastIP(a Address) bool {
	if ip := AsIP(a); ip != nil && !ip.IsUnspecified() {
		// ipv4 or v6
		return !(ip.IsMulticast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast())
	}
	return false
}

// IsUnicastIPv4 returns true if its argument is a unicast IP4 address
func IsUnicastIPv4(a Address) bool {
	if ip := AsIP(a); ip != nil && ip.To4() != nil {
		return !ip.IsUnspecified() && !ip.IsMulticast()
	}
	return false
}

// IsPublicUnicastIPv4 returns true if its argument is a globally routable,
// public IPv4 unicast address.
func IsPublicUnicastIPv4(a Address) bool {
	if ip := AsIP(a); ip != nil && !ip.IsUnspecified() {
		if t := ip.To4(); t != nil && IsGloballyRoutableIP(t) {
			return !ip.IsMulticast()
		}
	}
	return false
}

// IsUnicastIPv6 returns true if its argument is a unicast IPv6 address
func IsUnicastIPv6(a Address) bool {
	if ip := AsIP(a); ip != nil && ip.To4() == nil {
		return !ip.IsUnspecified() && !(ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast())
	}
	return false
}

// IsUnicastIPv6 returns true if its argument is a globally routable IP6
// address
func IsPublicUnicastIPv6(a Address) bool {
	if ip := AsIP(a); ip != nil && ip.To4() == nil {
		if t := ip.To16(); t != nil && IsGloballyRoutableIP(t) {
			return true
		}
	}
	return false
}

// IsPublicUnicastIP returns true if its argument is a global routable IPv4
// or 6 address.
func IsPublicUnicastIP(a Address) bool {
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

// SameMachine returns true if the provided addr is on the host
// executing this function.
func SameMachine(addr net.Addr) (bool, error) {
	addrs, _, err := GetAllAddresses()
	if err != nil {
		return false, err
	}
	ips := make(map[string]struct{})
	for _, a := range addrs {
		ip, _, err := net.ParseCIDR(a.String())
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

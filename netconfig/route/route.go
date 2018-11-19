// Package route defines the types of the values returned by
// netconfig.GetIPRoutes. It's kept in a separate package to avoid dependency
// cycles.
package route

import "net"

// IPRoute represents a route in the kernel's routing table.
// Any route with a nil Gateway is a directly connected network.
type IPRoute struct {
	Net             net.IPNet
	Gateway         net.IP
	PreferredSource net.IP
	IfcIndex        int
}

func isZeroSlice(a []byte) bool {
	for _, i := range a {
		if i != 0 {
			return false
		}
	}
	return true
}

// IsDefaultRoute returns true if r is a default route, i.e., that it matches any destination address.
func IsDefaultIPRoute(r *IPRoute) bool {
	if !r.Net.IP.Equal(net.IPv4zero) && !r.Net.IP.Equal(net.IPv6zero) {
		return false
	}
	return isZeroSlice(r.Net.Mask[:])
}

// IsDefaultIPv4Route returns true if r is a default IPv4 route.
func IsDefaultIPv4Route(r *IPRoute) bool {
	if !r.Net.IP.Equal(net.IPv4zero) && !r.Net.IP.Equal(net.IPv6zero) {
		return false
	}
	return len(r.Net.Mask) == 4 && isZeroSlice(r.Net.Mask[:])
}

// IsDefaultIPv6Route returns true if r is a default IPv6 route.
func IsDefaultIPv6Route(r *IPRoute) bool {
	if !r.Net.IP.Equal(net.IPv4zero) && !r.Net.IP.Equal(net.IPv6zero) {
		return false
	}
	return len(r.Net.Mask) == 16 && isZeroSlice(r.Net.Mask[:])
}

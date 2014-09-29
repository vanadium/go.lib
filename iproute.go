// package netconfig implements a network configuration watcher.
// NOTE(p): This is also where we should put any code that changes
//          network configuration.

package netconfig

import (
	"net"
)

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

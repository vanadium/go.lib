// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netstate

import (
	"fmt"
	"strings"

	"v.io/x/lib/netconfig/route"
)

// IPRouteList is a slice of IPRoutes as returned by the netconfig package.
type IPRouteList []route.IPRoute

func (rl IPRouteList) String() string {
	r := ""
	for _, rt := range rl {
		src := ""
		if len(rt.PreferredSource) > 0 {
			src = ", src: " + rt.PreferredSource.String()
		}
		r += fmt.Sprintf("(%d: net: %s, gw: %s%s) ", rt.IfcIndex, rt.Net, rt.Gateway, src)
	}
	return strings.TrimRight(r, " ")
}

// RoutePredicate defines the function signature for predicate functions
// to be used with RouteList
type RoutePredicate func(r *route.IPRoute) bool

// Filter returns all of the routes for which the predicate
// function is true.
func (rl IPRouteList) Filter(predicate RoutePredicate) IPRouteList {
	r := IPRouteList{}
	for _, rt := range rl {
		tmp := rt
		if predicate(&tmp) {
			r = append(r, rt)
		}
	}
	return r
}

// IsDefaultRoute returns true if the supplied IPRoute is a default route.
func IsDefaultRoute(r *route.IPRoute) bool {
	return route.IsDefaultIPRoute(r)
}

// IsOnDefaultRoute returns true for addresses that are on an interface that
// has a default route set for the supplied address.
func IsOnDefaultRoute(a Address) bool {
	ipifc, ok := a.Interface().(IPNetworkInterface)
	if !ok || len(ipifc.IPRoutes()) == 0 {
		return false
	}

	ipv4 := IsUnicastIPv4(a)
	for _, r := range ipifc.IPRoutes() {
		// Ignore entries with a nil gateway.
		if r.Gateway == nil {
			continue
		}
		tmp := r
		// We have a default route, so we check the gateway to make sure
		// it matches the format of the default route.
		if ipv4 && route.IsDefaultIPv4Route(&tmp) && r.Gateway.To4() != nil {
			return true
		}
		if route.IsDefaultIPv6Route(&tmp) {
			return true
		}
	}
	return false
}

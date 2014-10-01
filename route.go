package netstate

import (
	"fmt"
	"net"
	"strings"

	"veyron.io/veyron/veyron2/ipc"

	"veyron.io/veyron/veyron/lib/netconfig"
)

// Interface represents a network interface.
type Interface struct {
	Index int
	Name  string
}
type InterfaceList []*Interface

// GetInterfaces returns a list of all of the network interfaces on this
// device.
func GetInterfaces() (InterfaceList, error) {
	ifcl := InterfaceList{}
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, ifc := range interfaces {
		ifcl = append(ifcl, &Interface{ifc.Index, ifc.Name})
	}
	return ifcl, nil
}

func (ifcl InterfaceList) String() string {
	r := ""
	for _, ifc := range ifcl {
		r += fmt.Sprintf("(%d: %s) ", ifc.Index, ifc.Name)
	}
	return strings.TrimRight(r, " ")
}

// IPRouteList is a slice of IPRoutes as returned by the netconfig package.
type IPRouteList []*netconfig.IPRoute

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

func GetRoutes() IPRouteList {
	return netconfig.GetIPRoutes(false)
}

// RoutePredicate defines the function signature for predicate functions
// to be used with RouteList
type RoutePredicate func(r *netconfig.IPRoute) bool

// Filter returns all of the routes for which the predicate
// function is true.
func (rl IPRouteList) Filter(predicate RoutePredicate) IPRouteList {
	r := IPRouteList{}
	for _, rt := range rl {
		if predicate(rt) {
			r = append(r, rt)
		}
	}
	return r
}

// IsDefaultRoute returns true if the supplied IPRoute is a default route.
func IsDefaultRoute(r *netconfig.IPRoute) bool {
	return netconfig.IsDefaultIPRoute(r)
}

// IsOnDefaultRoute returns true for addresses that are on an interface that
// has a default route set for the supplied address.
func IsOnDefaultRoute(a ipc.Address) bool {
	aifc, ok := a.(*AddrIfc)
	if !ok || len(aifc.IPRoutes) == 0 {
		return false
	}
	ipv4 := IsUnicastIPv4(a)
	for _, r := range aifc.IPRoutes {
		// Ignore entries with a nil gateway.
		if r.Gateway == nil {
			continue
		}
		// We have a default route, so we check the gateway to make sure
		// it matches the format of the default route.
		if ipv4 {
			return netconfig.IsDefaultIPv4Route(r) && r.Gateway.To4() != nil
		}
		if netconfig.IsDefaultIPv6Route(r) {
			return true
		}
	}
	return false
}

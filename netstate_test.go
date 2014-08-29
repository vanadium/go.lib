package netstate_test

import (
	"fmt"
	"net"
	"os"
	"reflect"
	"testing"

	"veyron/lib/netstate"
)

func TestGet(t *testing.T) {
	all, err := netstate.GetAll()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	accessible, err := netstate.GetAccessibleIPs()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(all) == 0 || len(accessible) == 0 {
		t.Errorf("expected non zero lengths, not %d and %d", len(all), len(accessible))
	}
	if len(accessible) > len(all) {
		t.Errorf("should never be more accessible addresses than 'all' addresses")
	}
	loopback := netstate.FindAdded(accessible, all)
	if got, want := loopback.Filter(netstate.IsLoopbackIP), loopback; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func mkAddr(n, a string) net.Addr {
	ip := net.ParseIP(a)
	fmt.Fprintf(os.Stderr, "%s -> %d\n", a, len(ip))
	return netstate.AsAddr(n, ip)
}

func TestAsIP(t *testing.T) {
	lh := net.ParseIP("127.0.0.1")
	if got, want := netstate.AsIP(&net.IPAddr{IP: lh}), "127.0.0.1"; got == nil || got.String() != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := netstate.AsIP(&net.IPNet{IP: lh}), "127.0.0.1"; got == nil || got.String() != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := netstate.AsIP(netstate.AsAddr("tcp", lh)), "127.0.0.1"; got == nil || got.String() != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

type hw struct{}

func (*hw) Network() string { return "mac" }
func (*hw) String() string  { return "01:23:45:67:89:ab:cd:ef" }

func TestPredicates(t *testing.T) {
	if got, want := netstate.IsUnicastIP(&hw{}), false; got != want {
		t.Errorf("got %t, want %t", got, want)

	}

	cases := []struct {
		f func(a net.Addr) bool
		a string
		r bool
	}{
		{netstate.IsUnspecifiedIP, "0.0.0.0", true},
		{netstate.IsUnspecifiedIP, "::", true},
		{netstate.IsUnspecifiedIP, "127.0.0.1", false},
		{netstate.IsUnspecifiedIP, "::1", false},

		{netstate.IsLoopbackIP, "0.0.0.0", false},
		{netstate.IsLoopbackIP, "::", false},
		{netstate.IsLoopbackIP, "127.0.0.1", true},
		{netstate.IsLoopbackIP, "::1", true},

		{netstate.IsAccessibleIP, "0.0.0.0", false},
		{netstate.IsAccessibleIP, "::", false},
		{netstate.IsAccessibleIP, "127.0.0.1", false},
		{netstate.IsAccessibleIP, "::1", false},
		{netstate.IsAccessibleIP, "224.0.0.2", true},
		{netstate.IsAccessibleIP, "fc00:1234::", true},
		{netstate.IsAccessibleIP, "192.168.1.1", true},
		{netstate.IsAccessibleIP, "2001:4860:0:2001::68", true},

		{netstate.IsUnicastIP, "0.0.0.0", false},
		{netstate.IsUnicastIP, "::", false},
		{netstate.IsUnicastIP, "127.0.0.1", true},
		{netstate.IsUnicastIP, "::1", true},
		{netstate.IsUnicastIP, "192.168.1.2", true},
		{netstate.IsUnicastIP, "74.125.239.36", true},
		{netstate.IsUnicastIP, "224.0.0.2", false},
		{netstate.IsUnicastIP, "fc00:1235::", true},
		{netstate.IsUnicastIP, "ff01::01", false},
		{netstate.IsUnicastIP, "2001:4860:0:2001::69", true},

		{netstate.IsUnicastIPv4, "0.0.0.0", false},
		{netstate.IsUnicastIPv4, "::", false},
		{netstate.IsUnicastIPv4, "127.0.0.1", true},
		{netstate.IsUnicastIPv4, "::1", false},
		{netstate.IsUnicastIPv4, "192.168.1.3", true},
		{netstate.IsUnicastIPv6, "74.125.239.37", false},
		{netstate.IsUnicastIPv4, "224.0.0.2", false},
		{netstate.IsUnicastIPv4, "fc00:1236::", false},
		{netstate.IsUnicastIPv4, "ff01::02", false},
		{netstate.IsUnicastIPv4, "2001:4860:0:2001::6a", false},

		{netstate.IsUnicastIPv6, "0.0.0.0", false},
		{netstate.IsUnicastIPv6, "::", false},
		{netstate.IsUnicastIPv6, "127.0.0.1", false},
		{netstate.IsUnicastIPv6, "::1", true},
		{netstate.IsUnicastIPv6, "192.168.1.4", false},
		{netstate.IsUnicastIPv6, "74.125.239.38", false},
		{netstate.IsUnicastIPv6, "224.0.0.2", false},
		{netstate.IsUnicastIPv6, "fc00:1237::", true},
		{netstate.IsUnicastIPv6, "ff01::03", false},
		{netstate.IsUnicastIPv6, "2607:f8b0:4003:c00::6b", true},

		{netstate.IsPublicUnicastIP, "0.0.0.0", false},
		{netstate.IsPublicUnicastIP, "::", false},
		{netstate.IsPublicUnicastIP, "127.0.0.1", false},
		{netstate.IsPublicUnicastIP, "::1", false},
		{netstate.IsPublicUnicastIP, "192.168.1.2", false},
		{netstate.IsPublicUnicastIP, "74.125.239.39", true},
		{netstate.IsPublicUnicastIP, "224.0.0.2", false},
		// Arguably this is buggy, the fc00:/7 prefix is supposed to be
		// non-routable.
		{netstate.IsPublicUnicastIP, "fc00:1238::", true},
		{netstate.IsPublicUnicastIP, "ff01::01", false},
		{netstate.IsPublicUnicastIP, "2001:4860:0:2001::69", true},

		{netstate.IsPublicUnicastIPv4, "0.0.0.0", false},
		{netstate.IsPublicUnicastIPv4, "::", false},
		{netstate.IsPublicUnicastIPv4, "127.0.0.1", false},
		{netstate.IsPublicUnicastIPv4, "::1", false},
		{netstate.IsPublicUnicastIPv4, "192.168.1.3", false},
		{netstate.IsPublicUnicastIPv4, "74.125.239.40", true},
		{netstate.IsPublicUnicastIPv4, "224.0.0.2", false},
		{netstate.IsPublicUnicastIPv4, "fc00:1239::", false},
		{netstate.IsPublicUnicastIPv4, "ff01::02", false},
		{netstate.IsPublicUnicastIPv4, "2001:4860:0:2001::6a", false},

		{netstate.IsPublicUnicastIPv6, "0.0.0.0", false},
		{netstate.IsPublicUnicastIPv6, "::", false},
		{netstate.IsPublicUnicastIPv6, "127.0.0.1", false},
		{netstate.IsPublicUnicastIPv6, "::1", false},
		{netstate.IsPublicUnicastIPv6, "192.168.1.4", false},
		{netstate.IsPublicUnicastIPv6, "74.125.239.41", false},
		{netstate.IsPublicUnicastIPv6, "224.0.0.2", false},
		// Arguably this is buggy, the fc00:/7 prefix is supposed to be
		// non-routable.
		{netstate.IsPublicUnicastIPv6, "fc00:123a::", true},
		{netstate.IsPublicUnicastIPv6, "ff01::03", false},
		{netstate.IsPublicUnicastIPv6, "2607:f8b0:4003:c00::6b", true},
	}
	for i, c := range cases {
		net := "tcp"
		if got, want := c.f(mkAddr(net, c.a)), c.r; got != want {
			t.Errorf("#%d: %s %s: got %t, want %t", i+1, net, c.a, got, want)
		}
	}
}

var (
	a  = mkAddr("tcp4", "1.2.3.4")
	b  = mkAddr("tcp4", "1.2.3.5")
	c  = mkAddr("tcp4", "1.2.3.6")
	d  = mkAddr("tcp4", "1.2.3.7")
	a6 = mkAddr("tcp6", "2001:4860:0:2001::68")
	b6 = mkAddr("tcp6", "2001:4860:0:2001::69")
	c6 = mkAddr("tcp6", "2001:4860:0:2001::70")
	d6 = mkAddr("tcp6", "2001:4860:0:2001::71")
)

func TestRemoved(t *testing.T) {
	al := netstate.AddrList{a, b, c, a6, b6, c6}
	bl := netstate.AddrList{}

	// no changes.
	got, want := netstate.FindRemoved(al, al), netstate.AddrList{}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	// missing everything
	if got, want := netstate.FindRemoved(al, bl), al; !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}

	// missing nothing
	if got, want := netstate.FindRemoved(bl, al), (netstate.AddrList{}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}

	// remove some addresses
	bl = netstate.AddrList{a, b, a6, b6}
	if got, want := netstate.FindRemoved(al, bl), (netstate.AddrList{c, c6}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}

	// add some addresses
	bl = netstate.AddrList{a, b, c, a6, b6, c6, d6}
	if got, want := netstate.FindRemoved(al, bl), (netstate.AddrList{}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}

	// change some addresses
	bl = netstate.AddrList{a, b, d, a6, d6, c6}
	if got, want := netstate.FindRemoved(al, bl), (netstate.AddrList{c, b6}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestAdded(t *testing.T) {
	al := netstate.AddrList{a, b, c, a6, b6, c6}
	bl := netstate.AddrList{}

	// no changes.
	if got, want := netstate.FindAdded(al, al), (netstate.AddrList{}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}

	// add nothing
	if got, want := netstate.FindAdded(al, bl), bl; !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}

	// add everything
	if got, want := netstate.FindAdded(bl, al), al; !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}

	// add some addresses
	bl = netstate.AddrList{a, b, c, d, a6, b6, c6, d6}
	if got, want := netstate.FindAdded(al, bl), (netstate.AddrList{d, d6}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}

	// remove some addresses
	bl = netstate.AddrList{a, b, c, b6}
	if got, want := netstate.FindAdded(al, bl), (netstate.AddrList{}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}

	// change some addresses
	bl = netstate.AddrList{a, d, c, a6, d6, c6}
	if got, want := netstate.FindAdded(al, bl), (netstate.AddrList{d, d6}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}
}

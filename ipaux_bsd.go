// +build darwin dragonfly freebsd netbsd openbsd

package netconfig

// We connect to the Route socket and parse messages to
// look for network configuration changes.  This is generic
// to all BSD based systems (including MacOS).  The net
// library already has code to parse the messages so all
// we need to do is look for message types.

import (
	"errors"
	"net"
	"sync"
	"syscall"
	"time"
	"v.io/veyron/veyron2/vlog"
)

/*
#include <sys/socket.h>
*/
import "C"

type bsdNetConfigWatcher struct {
	sync.Mutex
	t       *time.Timer
	c       chan struct{}
	s       int
	stopped bool
}

func (w *bsdNetConfigWatcher) Stop() {
	w.Lock()
	defer w.Unlock()
	if w.stopped {
		return
	}
	w.stopped = true
	syscall.Close(w.s)
}

func (w *bsdNetConfigWatcher) Channel() chan struct{} {
	return w.c
}

// NewNetConfigWatcher returns a watcher that sends a message
// on the Channel() whenever the config changes.
func NewNetConfigWatcher() (NetConfigWatcher, error) {
	s, err := syscall.Socket(C.PF_ROUTE, syscall.SOCK_RAW, syscall.AF_UNSPEC)
	if err != nil {
		vlog.Infof("socket failed: %s", err)
		return nil, err
	}
	w := &bsdNetConfigWatcher{c: make(chan struct{}, 1), s: s}
	go w.watcher()
	return w, nil
}

func (w *bsdNetConfigWatcher) ding() {
	w.Lock()
	defer w.Unlock()
	w.t = nil
	if w.stopped {
		return
	}
	// Don't let us hang in the lock.  The default is safe because the requirement
	// is that the client get a message after the last config change.  Since this is
	// a queued chan, we really don't have to stuff anything in it if there's already
	// something there.
	select {
	case w.c <- struct{}{}:
	default:
	}
}

func (w *bsdNetConfigWatcher) watcher() {
	defer w.Stop()

	// Loop waiting for messages.
	for {
		b := make([]byte, 4096)
		nr, err := syscall.Read(w.s, b)
		if err != nil {
			return
		}
		msgs, err := syscall.ParseRoutingMessage(b[:nr])
		if err != nil {
			vlog.Infof("Couldn't parse: %s", err)
			continue
		}

		// Decode the addresses.
		for _, m := range msgs {
			switch m.(type) {
			case *syscall.InterfaceMessage:
			case *syscall.InterfaceAddrMessage:
			default:
				continue
			}
			// Changing networks usually spans many seconds and involves
			// multiple network config changes.  We add histeresis by
			// setting an alarm when the first change is detected and
			// not informing the client till the alarm goes off.
			// NOTE(p): I chose 3 seconds because that covers all the
			// events involved in moving from one wifi network to another.
			w.Lock()
			if w.t == nil {
				w.t = time.AfterFunc(3*time.Second, w.ding)
			}
			w.Unlock()
		}
	}

	w.Stop()
	w.Lock()
	close(w.c)
	if w.t != nil {
		w.t.Stop()
	}
	w.Unlock()
}

func toIP(sa syscall.Sockaddr) (net.IP, error) {
	switch v := sa.(type) {
	case *syscall.SockaddrInet4:
		return net.IPv4(v.Addr[0], v.Addr[1], v.Addr[2], v.Addr[3]), nil
	case *syscall.SockaddrInet6:
		return net.IP(v.Addr[:]), nil
	}
	return net.IPv6zero, errors.New("unknown sockaddr ip")
}

func toIPNet(sa syscall.Sockaddr, msa syscall.Sockaddr) (net.IPNet, error) {
	var x net.IPNet
	var err error
	x.IP, err = toIP(sa)
	if err != nil {
		return x, err
	}
	switch v := msa.(type) {
	case *syscall.SockaddrInet4:
		x.Mask = net.IPv4Mask(v.Addr[0], v.Addr[1], v.Addr[2], v.Addr[3])
		return x, nil
	case *syscall.SockaddrInet6:
		x.Mask = net.IPMask(v.Addr[:])
		return x, nil
	}
	return x, errors.New("unknown sockaddr ipnet")
}

// IPRoutes returns all kernel known routes.  If defaultOnly is set, only default routes
// are returned.
func GetIPRoutes(defaultOnly bool) []*IPRoute {
	var x []*IPRoute
	rib, err := syscall.RouteRIB(syscall.NET_RT_DUMP, 0)
	if err != nil {
		vlog.Infof("Couldn't read: %s", err)
		return x
	}
	msgs, err := syscall.ParseRoutingMessage(rib)
	if err != nil {
		vlog.Infof("Couldn't parse: %s", err)
		return x
	}
	for _, m := range msgs {
		switch v := m.(type) {
		case *syscall.RouteMessage:
			addrs, err := syscall.ParseRoutingSockaddr(m)
			if err != nil {
				return x
			}
			if addrs[0] == nil || addrs[1] == nil || addrs[2] == nil {
				continue
			}
			r := new(IPRoute)
			if r.Gateway, err = toIP(addrs[1]); err != nil {
				continue
			}
			if r.Net, err = toIPNet(addrs[0], addrs[2]); err != nil {
				continue
			}
			r.IfcIndex = int(v.Header.Index)
			if !defaultOnly || IsDefaultIPRoute(r) {
				x = append(x, r)
			}
		}
	}
	return x
}

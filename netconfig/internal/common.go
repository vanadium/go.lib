package internal

// Force this file to compile as cgo, to work around bazel/rules_go
// limitations. See also https://github.com/bazelbuild/rules_go/issues/255

import "C"
import (
	"net"
	"sync"
	"time"
)

// IPRoute represents a route in the kernel's routing table.
// Any route with a nil Gateway is a directly connected network.
type IPRoute struct {
	Net             net.IPNet
	Gateway         net.IP
	PreferredSource net.IP
	IfcIndex        int
}

// NewNotifier returns a new network change Notifier.
func NewNotifier(delay time.Duration) *Notifier {
	if delay == 0 {
		// See ding method below.
		// NOTE(p): I chose 3 seconds because that covers all the
		// events involved in moving from one wifi network to another.
		delay = 3 * time.Second
	}
	return &Notifier{delay: delay}
}

// Notifier represents a new network change Notifier.
type Notifier struct {
	sync.Mutex
	ch    chan struct{}
	timer *time.Timer
	delay time.Duration

	initErr error
	inited  bool
}

func (n *Notifier) Add() (<-chan struct{}, error) {
	n.Lock()
	defer n.Unlock()
	if !n.inited {
		n.ch = make(chan struct{})
		n.initErr = n.initLocked()
		n.inited = true
	}
	if n.initErr != nil {
		return nil, n.initErr
	}
	return n.ch, nil
}

func (n *Notifier) ding() {
	// Changing networks usually spans many seconds and involves
	// multiple network config changes.  We add histeresis by
	// setting an alarm when the first change is detected and
	// not informing the client till the alarm goes off.
	n.Lock()
	if n.timer == nil {
		n.timer = time.AfterFunc(n.delay, n.resetChan)
	}
	n.Unlock()
}

func (n *Notifier) resetChan() {
	n.Lock()
	close(n.ch)
	n.ch = make(chan struct{})
	n.timer = nil
	n.Unlock()
}

func isZeroSlice(a []byte) bool {
	for _, i := range a {
		if i != 0 {
			return false
		}
	}
	return true
}

func isDefaultIPRoute(r *IPRoute) bool {
	if !r.Net.IP.Equal(net.IPv4zero) && !r.Net.IP.Equal(net.IPv6zero) {
		return false
	}
	return isZeroSlice(r.Net.Mask[:])
}

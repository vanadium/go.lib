// +build darwin dragonfly freebsd netbsd openbsd

package netconfig

// We connect to the Route socket and parse messages to
// look for network configuration changes.  This is generic
// to all BSD based systems (including MacOS).  The net
// library already has code to parse the messages so all
// we need to do is look for message types.

import (
	"sync"
	"syscall"
	"time"
	"veyron2/vlog"
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

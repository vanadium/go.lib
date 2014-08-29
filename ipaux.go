// +build plan9 windows

package netconfig

// Code to signal a network change every 2 minutes.   We use
// this for systems where we don't yet have a good way to
// watch for network changes.

import (
	"time"
)

type timerNetConfigWatcher struct {
	c    chan struct{} // channel to signal confg changes
	stop chan struct{} // channel to tell the watcher to stop
}

// Stop any waiter
func (w *timerNetConfigWatcher) Stop() {
	w.stop <- struct{}{}
}

func (w *timerNetConfigWatcher) Channel() chan struct{} {
	return w.c
}

func (w *timerNetConfigWatcher) watcher() {
	for {
		select {
		case <-w.stop:
			close(w.c)
			return
		case <-time.NewTimer(2 * time.Minute).C:
			select {
			case w.c <- struct{}{}:
			default:
			}
		}
	}
}

func NewNetConfigWatcher() (NetConfigWatcher, error) {
	w.c = make(chan struct{})
	w.stop = make(chan struct{}, 1)
	go w.watcher()
	return w, nil
}

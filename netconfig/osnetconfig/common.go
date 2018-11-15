package osnetconfig

// Force this file to compile as cgo, to work around bazel/rules_go
// limitations. See also https://github.com/bazelbuild/rules_go/issues/255

import "C"
import (
	"sync"
	"time"
)

type notifier struct {
	sync.Mutex
	ch    chan struct{}
	timer *time.Timer

	initErr error
	inited  bool
}

func (n *notifier) add() (<-chan struct{}, error) {
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

func (n *notifier) ding() {
	// Changing networks usually spans many seconds and involves
	// multiple network config changes.  We add histeresis by
	// setting an alarm when the first change is detected and
	// not informing the client till the alarm goes off.
	// NOTE(p): I chose 3 seconds because that covers all the
	// events involved in moving from one wifi network to another.
	n.Lock()
	if n.timer == nil {
		n.timer = time.AfterFunc(3*time.Second, n.resetChan)
	}
	n.Unlock()
}

func (n *notifier) resetChan() {
	n.Lock()
	close(n.ch)
	n.ch = make(chan struct{})
	n.timer = nil
	n.Unlock()
}

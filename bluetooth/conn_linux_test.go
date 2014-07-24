// +build veyronbluetooth,!android

package bluetooth

import (
	"runtime"
	"syscall"
	"testing"
)

// TestConnConcurrency attempts to tests that methods on the *conn type be
// friendly to concurrent invocation. Unable to figure out a clean way to do
// this, the author has resorted to just firing up a bunch of goroutines and
// hoping that failures will manifest often.
func TestConnConcurrency(t *testing.T) {
	const (
		// These numbers were tuned to make the test fail "often"
		// without the accompanying change to conn.go in the commit
		// that added this test on the machine that the author was
		// using at the time.
		nConcurrentReaders = 30
		nConcurrentClosers = 10
	)
	mp := runtime.GOMAXPROCS(nConcurrentReaders)
	defer runtime.GOMAXPROCS(mp)

	pipe := func() (rfd, wfd int) {
		var fds [2]int
		if err := syscall.Pipe(fds[:]); err != nil {
			t.Fatal(err)
		}
		return fds[0], fds[1]
	}
	rfd, wfd := pipe()
	rConn, _ := newConn(rfd, nil, nil)
	wConn, _ := newConn(wfd, nil, nil)
	const (
		bugs  = "bugs bunny"
		daffy = "daffy duck"
	)
	rchan := make(chan string)
	// Write a bunch of times
	for i := 0; i < nConcurrentReaders; i++ {
		go wConn.Write([]byte(bugs))
	}
	read := func() {
		buf := make([]byte, len(bugs))
		if n, err := rConn.Read(buf); err == nil {
			rchan <- string(buf[:n])
			return
		}
		rchan <- ""
	}
	// Fire up half the readers before Close
	for i := 0; i < nConcurrentReaders; i += 2 {
		go read()
	}
	// Fire up the closers (and attempt to reassign the file descriptors to
	// something new).
	for i := 0; i < nConcurrentClosers; i++ {
		go func() {
			rConn.Close()
			// Create new FDs, which may re-use the closed file descriptors
			// and write something else to them.
			rfd, wfd := pipe()
			syscall.Write(wfd, []byte(daffy))
			syscall.Close(wfd)
			syscall.Close(rfd)
		}()
	}
	// And then the remaining readers
	for i := 1; i < nConcurrentReaders; i += 2 {
		go read()
	}
	// Now read from the channel, should either see full bugs bunnies or empty strings.
	nEmpty := 0
	for i := 0; i < nConcurrentReaders; i++ {
		got := <-rchan
		switch {
		case len(got) == 0:
			nEmpty++
		case got != bugs:
			t.Errorf("Read %q, wanted %q or empty string", got, bugs)
		}
	}
	t.Logf("Read returned non-empty %d/%d times", (nConcurrentReaders - nEmpty), nConcurrentReaders)
}

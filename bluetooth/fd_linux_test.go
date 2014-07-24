// +build veyronbluetooth,!android

package bluetooth

import (
	"fmt"
	"io"
	"sort"
	"syscall"
	"testing"
	"time"
)

// mkfds returns two *fds, one on which Read can be called and one on which
// Write can be called by using the pipe system call. This pipe is a cheap
// approximation of a file descriptor backed by a network socket that the fd type
// is really intended for.
func mkfds() (readfd, writefd *fd, err error) {
	var fds [2]int
	if err = syscall.Pipe(fds[:]); err != nil {
		err = fmt.Errorf("syscall.Pipe failed: %v", err)
		return
	}
	if readfd, err = newFD(fds[0]); err != nil {
		err = fmt.Errorf("newFD failed for readfd: %v", err)
		return
	}
	if writefd, err = newFD(fds[1]); err != nil {
		err = fmt.Errorf("newFD failed for writefd: %v", err)
		return
	}
	return
}

// canClose calls fd.Close and returns true if fd.Close returns.
// It returns false if fd.Close blocks.
// This function uses time to guess whether fd.Close is blocked or
// not, and is thus not the most accurate implementation. The author
// welcomes advice on restructuring this function or tests involving
// it to make the testing deterministically accurate.
func canClose(fd *fd) bool {
	c := make(chan error)
	go func() {
		c <- fd.Close()
	}()
	select {
	case <-c:
		return true
	case <-time.After(time.Millisecond):
		return false
	}
}

func TestFDBasic(t *testing.T) {
	rfd, wfd, err := mkfds()
	if err != nil {
		t.Fatal(err)
	}
	const batman = "batman"
	if n, err := wfd.Write([]byte(batman)); n != 6 || err != nil {
		t.Errorf("Got (%d, %v) want (6, nil)", n, err)
	}
	var read [1024]byte
	if n, err := rfd.Read(read[:]); n != 6 || err != nil || string(read[:n]) != string(batman) {
		t.Errorf("Got (%d, %v) = %q, want (6, nil) = %q", n, err, read[:n], batman)
	}
	if err := rfd.Close(); err != nil {
		t.Error(err)
	}
	if err := wfd.Close(); err != nil {
		t.Error(err)
	}
}

func TestFDReference(t *testing.T) {
	fd, _, err := mkfds()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fd.Reference(); err != nil {
		t.Fatal(err)
	}
	if canClose(fd) {
		t.Errorf("Should not be able to close fd since there is an outstanding reference")
	}
	fd.ReleaseReference()
	if !canClose(fd) {
		t.Errorf("Should be able to close fd since there are no outstanding references")
	}
}

func TestFDReadEOF(t *testing.T) {
	rfd, wfd, err := mkfds()
	if err != nil {
		t.Fatal(err)
	}
	const (
		bugs  = "bugs"
		bunny = "bunny"
	)
	if n, err := wfd.Write([]byte(bugs)); n != len(bugs) || err != nil {
		t.Fatalf("Got (%d, %v) want (%d, nil)", n, err, len(bugs))
	}
	if n, err := wfd.Write([]byte(bunny)); n != len(bunny) || err != nil {
		t.Fatalf("Got (%d, %v) want (%d, nil)", n, err, len(bunny))
	}
	if err := wfd.Close(); err != nil {
		t.Fatal(err)
	}
	var read [1024]byte
	if n, err := rfd.Read(read[:]); n != len(bugs)+len(bunny) || err != nil {
		t.Errorf("Got (%d, %v) = %q, want (%d, nil) = %q", n, err, read[:n], len(bugs)+len(bunny), "bugsbunny")
	}
	if n, err := rfd.Read(read[:]); n != 0 || err != io.EOF {
		t.Errorf("Got (%d, %v) = %q, want (0, EOF)", n, err, read[:n])
	}
}

func TestFDReadLessThanReady(t *testing.T) {
	rfd, wfd, err := mkfds()
	if err != nil {
		t.Fatal(err)
	}
	const nbytes = 20
	rchan := make(chan int, nbytes)
	written := make([]byte, nbytes)
	for i := 1; i <= nbytes; i++ {
		written[i-1] = byte(i)
		go func() {
			var buf [1]byte
			rfd.Read(buf[:])
			rchan <- int(buf[0])
		}()
	}
	if n, err := wfd.Write(written); n != nbytes || err != nil {
		t.Fatal("Got (%d, %v), want (%d, nil)", n, err, nbytes)
	}
	if err := wfd.Close(); err != nil {
		t.Fatal(err)
	}
	read := make([]int, nbytes)
	for i := 0; i < nbytes; i++ {
		read[i] = <-rchan
	}
	sort.Ints(read)
	for i, v := range read {
		if i != v-1 {
			t.Fatalf("Got %v, wanted it to be sorted", read)
		}
	}
}

// +build veyronbluetooth,!android

package bluetooth

// #include <stddef.h>
// #include <sys/eventfd.h>
// #include <sys/select.h>
//
// int add_to_eventfd(int fd) {
//	uint64_t val = 1;
//	return write(fd, &val, 8);
// }
//
// int wait(int eventfd, int readfd, int writefd) {
//	fd_set readfds, writefds;
//	FD_ZERO(&readfds);
//	FD_ZERO(&writefds);
//	fd_set* writefdsp = NULL;
//
//	FD_SET(eventfd, &readfds);
//	int nfds = eventfd + 1;
//
//	if (readfd >= 0) {
//		FD_SET(readfd, &readfds);
//		if (readfd >= nfds) {
//			nfds = readfd + 1;
//		}
//	}
//	if (writefd >= 0) {
//		FD_SET(writefd, &writefds);
//		if (writefd >= nfds) {
//			nfds = writefd + 1;
//		}
//		writefdsp = &writefds;
//	}
//	// TODO(ashankar): Should EINTR be handled by a retry?
//	// See "Select Law" section of "man 2 select_tut".
//	int nready = select(nfds, &readfds, writefdsp, NULL, NULL);
//	if (nready < 0) {
//	  return nready;
//	}
//	return (readfd >= 0 && FD_ISSET(readfd, &readfds)) ||
//	       (writefd >= 0 && FD_ISSET(writefd, &writefds));
// }
import "C"

import (
	"fmt"
	"io"
	"sync"
	"syscall"
)

// An fd enables concurrent invocations of Read, Write and Close on a file
// descriptor.
//
// It ensures that read, write and close operations do not conflict and thereby
// avoids races between file descriptors being closed and re-used while a
// read/write is being initiated.
//
// This is achieved by using an eventfd to signal the intention to close a
// descriptor and a select over the eventfd and the file descriptor being
// protected.
type fd struct {
	mu              sync.Mutex
	datafd, eventfd C.int
	closing         bool       // Whether Close has been or is being invoked.
	done            *sync.Cond // Signaled when no Read or Writes are pending.
	refcnt          int
}

// newFD creates an fd object providing read, write and close operations
// over datafd that are not hostile to concurrent invocations.
func newFD(datafd int) (*fd, error) {
	eventfd, err := C.eventfd(0, C.EFD_CLOEXEC)
	if err != nil {
		return nil, fmt.Errorf("failed to create eventfd: %v", err)
	}
	ret := &fd{datafd: C.int(datafd), eventfd: eventfd}
	ret.done = sync.NewCond(&ret.mu)
	return ret, nil
}

func (fd *fd) Read(p []byte) (int, error) {
	e, d, err := fd.prepare()
	if err != nil {
		return 0, err
	}
	defer fd.finish()
	if err := wait(e, d, -1); err != nil {
		return 0, err
	}
	return fd.rw(syscall.Read(int(fd.datafd), p))
}

func (fd *fd) Write(p []byte) (int, error) {
	e, d, err := fd.prepare()
	if err != nil {
		return 0, err
	}
	defer fd.finish()
	if err := wait(e, -1, d); err != nil {
		return 0, err
	}
	return fd.rw(syscall.Write(int(fd.datafd), p))
}

// RunWhenReadable invokes f(file descriptor) when the file descriptor is ready
// to be read. It returns an error if the file descriptor has been closed
// either before or while this method is being invoked.
//
// f must NOT close readfd.
func (fd *fd) RunWhenReadable(f func(readfd int)) error {
	e, d, err := fd.prepare()
	if err != nil {
		return err
	}
	defer fd.finish()
	if err := wait(e, d, -1); err != nil {
		return err
	}
	f(int(d))
	return nil
}

// Reference returns the underlying file descriptor and ensures that calls to
// Close will block until ReleaseReference has been called.
//
// Clients must NOT close the returned file descriptor.
func (fd *fd) Reference() (int, error) {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	if fd.closing {
		return -1, fmt.Errorf("closing")
	}
	if fd.datafd < 0 {
		return -1, fmt.Errorf("closed")
	}
	fd.refcnt++
	return int(fd.datafd), nil
}

// ReleaseReference returns a reference to the file descriptor grabbed by a
// call to Reference, thereby unblocking any Close operations.
func (fd *fd) ReleaseReference() { fd.finish() }

// helper method for Read and Write that ensures:
// - the returned 'n' is always >= 0, as per guidelines for the io.Reader and
//   io.Writer interfaces.
func (fd *fd) rw(n int, err error) (int, error) {
	if n == 0 && err == nil {
		err = io.EOF
	}
	if n < 0 {
		n = 0
	}
	return n, err
}

func (fd *fd) prepare() (eventfd, datafd C.int, err error) {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	if fd.closing {
		return 0, 0, fmt.Errorf("closing")
	}
	fd.refcnt++
	// returned file descriptors are guaranteed to be
	// valid till refcnt is reduced by at least 1, since
	// Close waits for the refcnt to go down to zero before
	// closing these file descriptors.
	return fd.eventfd, fd.datafd, nil
}

func wait(eventfd, readfd, writefd C.int) error {
	ok, err := C.wait(eventfd, readfd, writefd)
	if err != nil {
		return err
	}
	if ok <= 0 {
		return fmt.Errorf("closing")
	}
	return nil
}

func (fd *fd) finish() {
	fd.mu.Lock()
	fd.refcnt--
	if fd.closing && fd.refcnt == 0 {
		fd.done.Broadcast()
	}
	fd.mu.Unlock()
}

func (fd *fd) Close() error {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	if !fd.closing {
		// Send an "event" to notify of closures.
		if _, err := C.add_to_eventfd(fd.eventfd); err != nil {
			return fmt.Errorf("failed to notify closure on eventfd: %v", err)
		}
		// Prevent any new Read/Write/RunWhenReadable calls from starting.
		fd.closing = true
	}
	for fd.refcnt > 0 {
		fd.done.Wait()
	}
	// At this point, there are no concurrent Read/Write/RunWhenReadable
	// calls that are using the file descriptors.
	if fd.eventfd > 0 {
		if err := syscall.Close(int(fd.eventfd)); err != nil {
			return fmt.Errorf("failed to close eventfd: %v", err)
		}
		fd.eventfd = -1
	}
	if fd.datafd > 0 {
		if err := syscall.Close(int(fd.datafd)); err != nil {
			return fmt.Errorf("failed to close underlying socket/filedescriptor: %v", err)
		}
		fd.datafd = -1
	}
	return nil
}

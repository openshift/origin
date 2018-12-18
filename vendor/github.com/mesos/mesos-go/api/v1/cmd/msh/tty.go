package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/mesos/mesos-go/api/v1/lib"
)

// #include <stdio.h>
// #include <stdlib.h>
// #include <termios.h>
// #include <unistd.h>
// #include <fcntl.h>
// #include <sys/ioctl.h>
//
// /* because golang doesn't like the ... param of ioctl */
// int ioctl_winsize(int d, unsigned long request, void *buf) {
//   return ioctl(d, request, buf);
// }
//
import "C"
import "unsafe"

type cleanups struct {
	ops  []func()
	once sync.Once
}

func (c *cleanups) unwind() {
	c.once.Do(func() {
		for _, f := range c.ops {
			defer f()
		}
	})
}

func (c *cleanups) push(f func()) {
	if f != nil {
		c.ops = append(c.ops, f)
	}
}

type ttyDevice struct {
	fd               int
	cancel           chan struct{}
	winch            chan mesos.TTYInfo_WindowSize
	cleanups         *cleanups
	original_winsize C.struct_winsize
}

func (t *ttyDevice) Done() <-chan struct{} { return t.cancel }
func (t *ttyDevice) Close()                { t.cleanups.unwind() }

func initTTY() (_ *ttyDevice, err error) {
	return newTTY(
		ttyConsoleAttach(&os.Stdin, &os.Stdout, &os.Stderr),
		ttyWinch,
		ttyTermReset,
	)
}

func newTTY(opts ...ttyOption) (_ *ttyDevice, err error) {
	tty := ttyDevice{
		cancel:   make(chan struct{}),
		cleanups: new(cleanups),
	}
	tty.cleanups.push(func() { close(tty.cancel) })
	defer func() {
		if err != nil {
			tty.Close()
		}
	}()

	ttyname := C.ctermid((*C.char)(unsafe.Pointer(nil)))
	if p := (*C.char)(unsafe.Pointer(ttyname)); p == nil {
		err = fmt.Errorf("failed to get tty name")
		return
	}

	tty.fd, _ = syscall.Open(C.GoString(ttyname), syscall.O_RDWR, 0)
	if tty.fd < 0 {
		err = fmt.Errorf("failed to open tty device: %d", tty.fd)
		return
	}
	tty.cleanups.push(func() { syscall.Close(tty.fd) })

	var original_termios C.struct_termios
	result := C.tcgetattr(C.int(tty.fd), &original_termios)
	if result < 0 {
		err = fmt.Errorf("failed getting termios: %d", result)
		return
	}

	new_termios := original_termios
	C.cfmakeraw(&new_termios)
	result = C.tcsetattr(C.int(tty.fd), C.TCSANOW, &new_termios)
	if result < 0 {
		err = fmt.Errorf("failed setting termios: %d", result)
		return
	}
	tty.cleanups.push(func() {
		r := C.tcsetattr(C.int(tty.fd), C.TCSANOW, &original_termios)
		if r < 0 {
			log.Printf("failed to set original termios: %d", r)
		}
	})

	// use this local var instead of tty.original_winsize to avoid cgo complaints about double-pointers
	var original_winsize C.struct_winsize
	result = C.ioctl_winsize(0, C.TIOCGWINSZ, unsafe.Pointer(&original_winsize))
	if result < 0 {
		err = fmt.Errorf("failed to get winsize: %d", result)
		return
	}
	tty.original_winsize = original_winsize
	tty.cleanups.push(func() {
		r := C.ioctl_winsize(0, C.TIOCSWINSZ, unsafe.Pointer(&original_winsize))
		if r < 0 {
			log.Printf("failed to set winsize: %d", r)
		}
	})

	log.Printf("original window size is %d x %d\n", tty.original_winsize.ws_col, tty.original_winsize.ws_row)

	for _, f := range opts {
		if f != nil {
			f(&tty)
		}
	}

	return &tty, nil
}

type ttyOption func(*ttyDevice)

func ttyConsoleAttach(stdin, stdout, stderr **os.File) ttyOption {
	swapfd := func(newfd uintptr, name string, target **os.File) func() {
		f := os.NewFile(newfd, name)
		if f == nil {
			panic(fmt.Sprintf("failed to swap fd for %q", name))
		}
		old := *target
		*target = f
		return func() {
			*target = old
		}
	}
	return func(tty *ttyDevice) {
		tty.cleanups.push(swapfd(uintptr(tty.fd), "tty", stdout))
		tty.cleanups.push(swapfd(uintptr(tty.fd), "tty", stderr))
		tty.cleanups.push(swapfd(uintptr(tty.fd), "tty", stdin))
	}
}

func ttyWinch(tty *ttyDevice) {
	// translate window-size signals into chan events
	c := make(chan os.Signal, 1)
	tty.winch = make(chan mesos.TTYInfo_WindowSize, 1)
	tty.winch <- mesos.TTYInfo_WindowSize{
		Rows:    uint32(tty.original_winsize.ws_row),
		Columns: uint32(tty.original_winsize.ws_col),
	}
	go func() {
		defer signal.Reset(os.Signal(syscall.SIGWINCH))
		for {
			select {
			case <-c:
				signal.Ignore(os.Signal(syscall.SIGWINCH))
				var temp_winsize C.struct_winsize
				r := C.ioctl_winsize(0, C.TIOCGWINSZ, unsafe.Pointer(&temp_winsize))
				if r < 0 {
					panic(fmt.Sprintf("failed to get winsize: %d", r))
				}
				ws := mesos.TTYInfo_WindowSize{
					Rows:    uint32(temp_winsize.ws_row),
					Columns: uint32(temp_winsize.ws_col),
				}
				select {
				case <-tty.Done():
					return
				case tty.winch <- ws:
					signal.Notify(c, os.Signal(syscall.SIGWINCH))
				}
			case <-tty.Done():
				return
			}
		}
	}()
	signal.Notify(c, os.Signal(syscall.SIGWINCH))
}

func ttyTermReset(tty *ttyDevice) {
	var (
		// cleanup properly upon SIGTERM
		term = make(chan os.Signal, 1)
		done = make(chan struct{})
	)
	go func() {
		select {
		case <-term:
			tty.cleanups.unwind()
			os.Exit(0)
		case <-done:
			//println("stop waiting for SIGTERM")
		}
	}()
	tty.cleanups.push(func() {
		signal.Reset(os.Signal(syscall.SIGTERM))
		close(done) // stop waiting for a signal
	})
	signal.Notify(term, os.Signal(syscall.SIGTERM))
}

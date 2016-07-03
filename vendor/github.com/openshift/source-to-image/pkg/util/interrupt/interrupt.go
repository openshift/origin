/*
Derived from:

https://github.com/kubernetes/kubernetes

Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package interrupt

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// terminationSignals are signals that cause the program to exit in the
// supported platforms (linux, darwin, windows).
var terminationSignals = []os.Signal{syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT}

// A Handler executes notification functions after a critical section (the
// function passed to its Run method), even in the presence of process
// termination via a termination signal. A final handler is executed if a
// termination signal is caught. The handler guarantees that the provided notify
// functions will be called at most once. A Handler is not reusable in the sense
// that its Run method should be called only once: calling it more times will
// not execute any of notify nor final functions.
type Handler struct {
	notify []func()
	final  func(os.Signal)
	once   sync.Once
}

// New creates a new Handler. The final function will be called only if a
// termination signal is caught, after all notify functions are run. If final is
// nil, it defaults to os.Exit(2). The final function may call os.Exit if
// exiting is desired. The notify functions will be called when a termination
// signal is caught, or after the argument to the Run method completes.
func New(final func(os.Signal), notify ...func()) *Handler {
	return &Handler{
		final:  final,
		notify: notify,
	}
}

// Run calls fn in the current goroutine, while waiting for a termination signal
// in a separate goroutine. If a signal is caught while this method is running,
// the notify functions will be called sequentially in order, and then the final
// function is called. Otherwise, only the notify functions are called after fn
// returns. The fn function may not complete, for example in case of a call to
// os.Exit.
func (h *Handler) Run(fn func() error) error {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, terminationSignals...)
	defer close(ch)
	defer signal.Stop(ch)
	go func() {
		sig, ok := <-ch
		if !ok {
			return
		}
		h.signal(sig)
	}()
	defer h.close()
	return fn()
}

// close calls the notify functions, used when no signal was caught and the Run
// method returned.
func (h *Handler) close() {
	h.once.Do(func() {
		for _, fn := range h.notify {
			fn()
		}
	})
}

// signal calls the notify functions and final, used when a signal was caught
// while the Run method was running. If final is nil, os.Exit will be called as
// a default.
func (h *Handler) signal(s os.Signal) {
	h.once.Do(func() {
		for _, fn := range h.notify {
			fn()
		}
		if h.final == nil {
			os.Exit(2)
		}
		h.final(s)
	})
}

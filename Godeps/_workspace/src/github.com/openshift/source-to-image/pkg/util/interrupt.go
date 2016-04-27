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

package util

import (
	"os"
	"os/signal"
	"sync"
)

type InterruptHandler struct {
	notify []func()
	final  func(os.Signal)
	once   sync.Once
}

// New creates a new Handler.
// The first argument will be called only if a terminating signal is received, it will be the last
// function called.  It is this function's responsibility to call os.Exit() if exiting is desired.
// If the first argument is nil, this Handler will call os.Exit when the signal is received.
// The second+ arguments are functions that will be called when the signal is received, or after
// the argument to Run() completes.
func NewInterruptHandler(final func(os.Signal), notify ...func()) *InterruptHandler {
	return &InterruptHandler{
		final:  final,
		notify: notify,
	}
}

// Close invokes the notify functions of the Handler
// when no signal was received and Run() completed.
func (h *InterruptHandler) Close() {
	h.once.Do(func() {
		for _, fn := range h.notify {
			fn()
		}
	})
}

// Signal invokes the notify functions of the Handler
// when a signal is received.  It also invokes final,
// if any, otherwise it invokes os.Exit.
func (h *InterruptHandler) Signal(s os.Signal) {
	h.once.Do(func() {
		for _, fn := range h.notify {
			fn()
		}
		if h.final == nil {
			os.Exit(1)
		}
		h.final(s)
	})
}

// Run takes a single function argument which will be immediately invoked.
// If a signal arrives while the function is running, it will be handled,
// interrupting the function execution and executing any final and notify
// functions immediately.  The original function may or may not complete
// depending on whether the final argument invoking os.Exit.
func (h *InterruptHandler) Run(fn func() error) error {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, childSignals...)
	defer close(ch)
	defer signal.Stop(ch)
	go func() {
		sig, ok := <-ch
		if !ok {
			return
		}
		h.Signal(sig)
	}()
	defer h.Close()
	return fn()
}

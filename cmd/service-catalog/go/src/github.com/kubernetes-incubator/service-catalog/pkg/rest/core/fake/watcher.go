/*
Copyright 2017 The Kubernetes Authors.

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

package fake

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

// Watcher is a completely in-memory watcher mechanism for use in the fake REST client. This
// construct sends and receives on the same stream regardless of the type of object, its
// namespace, or anything else.
//
// Also note that this is not the same thing as a watch.Interface.
type Watcher struct {
	ch chan watch.Event
}

// NewWatcher creates a new Watcher with no events in it
func NewWatcher() *Watcher {
	return &Watcher{
		ch: make(chan watch.Event),
	}
}

// SendObject sends a watch event of evtType with object obj. It returns an error if the send
// couldn't be completed within timeout
func (w *Watcher) SendObject(evtType watch.EventType, obj runtime.Object, timeout time.Duration) error {
	evt := watch.Event{
		Type:   evtType,
		Object: obj,
	}
	select {
	case w.ch <- evt:
	case <-time.After(timeout):
		return fmt.Errorf("couldn't send after %s", timeout)
	}
	return nil
}

// ReceiveChan returns a read-only channel that can be used to receive events sent via this watcher
func (w *Watcher) ReceiveChan() <-chan watch.Event {
	return w.ch
}

// Close closes this watcher. All calls to SendObject after this func is called will cause a panic,
// and all channels returned by ReceiveChan, before or after this function is called, will be
// closed
func (w *Watcher) Close() {
	close(w.ch)
}

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

package record

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

type Event struct {
	Eventtype string
	Reason    string
	Message   string
}

type Recorder struct {
	Events chan Event
}

func (e *Recorder) Event(object runtime.Object, eventtype, reason, message string) {
	if e.Events != nil {
		e.Events <- Event{eventtype, reason, message}
	}
}

func (e *Recorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	if e.Events != nil {
		e.Events <- Event{eventtype, reason, fmt.Sprintf(messageFmt, args...)}
	}
}

func (e *Recorder) PastEventf(object runtime.Object, timestamp metav1.Time, eventtype, reason, messageFmt string, args ...interface{}) {

}

// NewFakeRecorder creates new fake event recorder with event channel with
// buffer of given size.
func NewRecorder(bufferSize int) *Recorder {
	return &Recorder{
		Events: make(chan Event, bufferSize),
	}
}

var _ record.EventRecorder = &Recorder{}

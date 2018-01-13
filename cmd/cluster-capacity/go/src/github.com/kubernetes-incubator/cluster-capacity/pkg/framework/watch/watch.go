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

package watch

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	ccapi "github.com/kubernetes-incubator/cluster-capacity/pkg/api"
)

// Every watcher expects infinite byte stream
// E.g. http.Body can provide continuous byte stream,
// each chunk sent asynchronously (invocation of Read is blocked until new chunk)

// Implementation of io.ReadCloser
type WatchBuffer struct {
	buf      *bytes.Buffer
	read     chan []byte
	write    chan []byte
	retc     chan retc
	Resource ccapi.ResourceType

	closed   bool
	closeMux sync.RWMutex
}

type retc struct {
	n int
	e error
}

var _ io.ReadCloser = &WatchBuffer{}

// Read watch events as byte stream
func (w *WatchBuffer) Read(p []byte) (n int, err error) {
	if w.closed {
		return 0, io.EOF
	}
	w.read <- p
	ret := <-w.retc
	return ret.n, ret.e
}

// Close all channels
func (w *WatchBuffer) Close() error {
	w.closeMux.Lock()
	defer w.closeMux.Unlock()

	if !w.closed {
		w.closed = true
		close(w.read)
		close(w.write)
		close(w.retc)
	}
	return nil
}

// Write
func (w *WatchBuffer) Write(data []byte) (nr int, err error) {
	if w.closed {
		return 0, io.EOF
	}
	w.write <- data
	return len(data), nil
}

func (c *WatchBuffer) EmitWatchEvent(eType watch.EventType, object runtime.Object) error {
	//event := watch.Event{
	//	Type: eType,
	//	Object: object,
	//}

	info, ok := runtime.SerializerInfoForMediaType(legacyscheme.Codecs.SupportedMediaTypes(), runtime.ContentTypeJSON)
	if !ok {
		return fmt.Errorf("serializer for %s not registered", runtime.ContentTypeJSON)
	}

	gv := v1.SchemeGroupVersion
	encoder := legacyscheme.Codecs.EncoderForVersion(info.Serializer, gv)

	obj_str := runtime.EncodeOrDie(encoder, object)
	obj_str = strings.Replace(obj_str, "\n", "", -1)

	var buffer bytes.Buffer
	buffer.WriteString("{\"type\":\"")
	buffer.WriteString(string(eType))
	buffer.WriteString("\",\"object\":")
	buffer.WriteString(obj_str)
	buffer.WriteString("}")

	_, err := c.Write(buffer.Bytes())
	return err
}

func (w *WatchBuffer) loop() {
	var dataIn, dataOut []byte
	var ok bool
	for {
		select {
		case dataIn = <-w.write:
			// channel closed
			if len(dataIn) == 0 {
				if w.closed {
					return
				}
			}
			_, err := w.buf.Write(dataIn)
			if err != nil {
				// TODO(jchaloup): add log message
				fmt.Println("Write error")
				break
			}
		case dataOut = <-w.read:
			if w.buf.Len() == 0 {
				dataIn, ok = <-w.write
				if !ok {
					break
				}
				_, err := w.buf.Write(dataIn)
				if err != nil {
					// TODO(jchaloup): add log message
					fmt.Println("Write error")
					break
				}
			}
			nr, err := w.buf.Read(dataOut)
			if w.closed {
				break
			}
			w.retc <- retc{nr, err}
		}
	}
}

func NewWatchBuffer(resource ccapi.ResourceType) *WatchBuffer {
	wb := &WatchBuffer{
		buf:      bytes.NewBuffer(nil),
		read:     make(chan []byte),
		write:    make(chan []byte),
		retc:     make(chan retc),
		Resource: resource,
		closed:   false,
	}

	go wb.loop()
	return wb
}

func main() {
	buffer := NewWatchBuffer("pods")

	go func() {
		buffer.Write([]byte("Ahoj"))
		time.Sleep(5 * time.Second)
		buffer.Write([]byte(" Svete"))
		//time.Sleep(time.Second)
	}()

	data := []byte{0, 0, 0, 0, 0, 0}
	buffer.Read(data)
	fmt.Printf("\tdata: %s\n", data)
	buffer.Read(data)
	fmt.Printf("\tdata: %s\n", data)

	time.Sleep(10 * time.Second)

	buffer.Close()
	fmt.Println("Ahoj")
}

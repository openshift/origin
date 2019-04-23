/*
Copyright 2014 The Kubernetes Authors.

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

package flushwriter

import (
	"bytes"
	"io"
	"net/http"
	"runtime/debug"

	"github.com/davecgh/go-spew/spew"
	"k8s.io/klog"
)

// Wrap wraps an io.Writer into a writer that flushes after every write if
// the writer implements the Flusher interface.
func Wrap(w io.Writer) io.Writer {
	fw := &flushWriter{
		writer: w,
	}
	if flusher, ok := w.(http.Flusher); ok {
		fw.flusher = flusher
	}
	return fw
}

// flushWriter provides wrapper for responseWriter with HTTP streaming capabilities
type flushWriter struct {
	flusher http.Flusher
	writer  io.Writer
}

// Write is a FlushWriter implementation of the io.Writer that sends any buffered
// data to the client.
func (fw *flushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.writer.Write(p)
	if err != nil {
		return
	}
	if bytes.Contains(p, unauthorizedMsg1) || bytes.Contains(p, unauthorizedMsg2) {
		log(string(p))
	}
	if fw.flusher != nil {
		fw.flusher.Flush()
	}
	return
}

func log(args ...interface{}) {
	klog.ErrorDepth(1, "ENJ:\n", config.Sdump(args...))
	debug.PrintStack()
}

var config = spew.ConfigState{Indent: "\t", MaxDepth: 5, DisableMethods: true}

var unauthorizedMsg1 = []byte(`{\"kind\":\"Status\",\"apiVersion\":\"v1\",\"metadata\":{},\"status\":\"Failure\",\"message\":\"Unauthorized\",\"reason\":\"Unauthorized\",\"code\":401}`)
var unauthorizedMsg2 = []byte(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Failure","message":"Unauthorized","reason":"Unauthorized","code":401}`)

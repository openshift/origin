// Copyright 2017 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !windows

package docker

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"
)

func TestClientDoConcurrentStress(t *testing.T) {
	t.Parallel()
	var reqs []*http.Request
	var mu sync.Mutex
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		reqs = append(reqs, r)
		mu.Unlock()
	})
	var nativeSrvs []*httptest.Server
	for i := 0; i < 3; i++ {
		srv, cleanup, err := newNativeServer(handler)
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		nativeSrvs = append(nativeSrvs, srv)
	}
	var tests = []struct {
		testCase      string
		srv           *httptest.Server
		scheme        string
		withTimeout   bool
		withTLSServer bool
		withTLSClient bool
	}{
		{testCase: "http server", srv: httptest.NewUnstartedServer(handler), scheme: "http"},
		{testCase: "native server", srv: nativeSrvs[0], scheme: nativeProtocol},
		{testCase: "http with timeout", srv: httptest.NewUnstartedServer(handler), scheme: "http", withTimeout: true},
		{testCase: "native with timeout", srv: nativeSrvs[1], scheme: nativeProtocol, withTimeout: true},
		{testCase: "http with tls", srv: httptest.NewUnstartedServer(handler), scheme: "https", withTLSServer: true, withTLSClient: true},
		{testCase: "native with client-only tls", srv: nativeSrvs[2], scheme: nativeProtocol, withTLSServer: false, withTLSClient: nativeProtocol == unixProtocol}, // TLS client only works with unix protocol
	}
	for _, tt := range tests {
		t.Run(tt.testCase, func(t *testing.T) {
			reqs = nil
			var client *Client
			var err error
			endpoint := tt.scheme + "://" + tt.srv.Listener.Addr().String()
			if tt.withTLSServer {
				tt.srv.StartTLS()
			} else {
				tt.srv.Start()
			}
			defer tt.srv.Close()
			if tt.withTLSClient {
				certPEMBlock, certErr := ioutil.ReadFile("testing/data/cert.pem")
				if certErr != nil {
					t.Fatal(certErr)
				}
				keyPEMBlock, certErr := ioutil.ReadFile("testing/data/key.pem")
				if certErr != nil {
					t.Fatal(certErr)
				}
				client, err = NewTLSClientFromBytes(endpoint, certPEMBlock, keyPEMBlock, nil)
			} else {
				client, err = NewClient(endpoint)
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.withTimeout {
				client.SetTimeout(time.Minute)
			}
			n := 50
			wg := sync.WaitGroup{}
			var paths []string
			errsCh := make(chan error, 3*n)
			waiters := make(chan CloseWaiter, n)
			for i := 0; i < n; i++ {
				path := fmt.Sprintf("/%05d", i)
				paths = append(paths, "GET"+path)
				paths = append(paths, "POST"+path)
				paths = append(paths, "HEAD"+path)
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, clientErr := client.do("GET", path, doOptions{})
					if clientErr != nil {
						errsCh <- clientErr
					}
					clientErr = client.stream("POST", path, streamOptions{})
					if clientErr != nil {
						errsCh <- clientErr
					}
					cw, clientErr := client.hijack("HEAD", path, hijackOptions{})
					if clientErr != nil {
						errsCh <- clientErr
					} else {
						waiters <- cw
					}
				}()
			}
			wg.Wait()
			close(errsCh)
			close(waiters)
			for cw := range waiters {
				cw.Wait()
				cw.Close()
			}
			for err = range errsCh {
				t.Error(err)
			}
			var reqPaths []string
			for _, r := range reqs {
				reqPaths = append(reqPaths, r.Method+r.URL.Path)
			}
			sort.Strings(paths)
			sort.Strings(reqPaths)
			if !reflect.DeepEqual(reqPaths, paths) {
				t.Fatalf("expected server request paths to equal %v, got: %v", paths, reqPaths)
			}
		})
	}
}

// Copyright 2014 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestExecCreate(t *testing.T) {
	t.Parallel()
	jsonContainer := `{"Id": "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"}`
	var expected struct{ ID string }
	err := json.Unmarshal([]byte(jsonContainer), &expected)
	if err != nil {
		t.Fatal(err)
	}
	fakeRT := &FakeRoundTripper{message: jsonContainer, status: http.StatusOK}
	client := newTestClient(fakeRT)
	config := CreateExecOptions{
		Container:    "test",
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: false,
		Tty:          false,
		Cmd:          []string{"touch", "/tmp/file"},
		User:         "a-user",
	}
	execObj, err := client.CreateExec(config)
	if err != nil {
		t.Fatal(err)
	}
	expectedID := "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"
	if execObj.ID != expectedID {
		t.Errorf("ExecCreate: wrong ID. Want %q. Got %q.", expectedID, execObj.ID)
	}
	req := fakeRT.requests[0]
	if req.Method != "POST" {
		t.Errorf("ExecCreate: wrong HTTP method. Want %q. Got %q.", "POST", req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/containers/test/exec"))
	if gotPath := req.URL.Path; gotPath != expectedURL.Path {
		t.Errorf("ExecCreate: Wrong path in request. Want %q. Got %q.", expectedURL.Path, gotPath)
	}
	var gotBody struct{ ID string }
	err = json.NewDecoder(req.Body).Decode(&gotBody)
	if err != nil {
		t.Fatal(err)
	}
}

func TestExecCreateWithEnvErr(t *testing.T) {
	t.Parallel()
	jsonContainer := `{"Id": "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"}`
	var expected struct{ ID string }
	err := json.Unmarshal([]byte(jsonContainer), &expected)
	if err != nil {
		t.Fatal(err)
	}
	fakeRT := &FakeRoundTripper{message: jsonContainer, status: http.StatusOK}
	client := newTestClient(fakeRT)
	config := CreateExecOptions{
		Container:    "test",
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: false,
		Tty:          false,
		Env:          []string{"foo=bar"},
		Cmd:          []string{"touch", "/tmp/file"},
		User:         "a-user",
	}
	_, err = client.CreateExec(config)
	if err == nil || err.Error() != "exec configuration Env is only supported in API#1.25 and above" {
		t.Error("CreateExec: options contain Env for unsupported api version")
	}
}

func TestExecCreateWithEnv(t *testing.T) {
	t.Parallel()
	jsonContainer := `{"Id": "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"}`
	var expected struct{ ID string }
	err := json.Unmarshal([]byte(jsonContainer), &expected)
	if err != nil {
		t.Fatal(err)
	}
	fakeRT := &FakeRoundTripper{message: jsonContainer, status: http.StatusOK}
	endpoint := "http://localhost:4243"
	u, _ := parseEndpoint("http://localhost:4243", false)
	testAPIVersion, _ := NewAPIVersion("1.25")
	client := Client{
		HTTPClient:             &http.Client{Transport: fakeRT},
		Dialer:                 &net.Dialer{},
		endpoint:               endpoint,
		endpointURL:            u,
		SkipServerVersionCheck: true,
		serverAPIVersion:       testAPIVersion,
	}
	config := CreateExecOptions{
		Container:    "test",
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: false,
		Tty:          false,
		Env:          []string{"foo=bar"},
		Cmd:          []string{"touch", "/tmp/file"},
		User:         "a-user",
	}
	_, err = client.CreateExec(config)
	if err != nil {
		t.Error(err)
	}
}

func TestExecStartDetached(t *testing.T) {
	t.Parallel()
	execID := "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"
	fakeRT := &FakeRoundTripper{status: http.StatusOK}
	client := newTestClient(fakeRT)
	config := StartExecOptions{
		Detach: true,
	}
	err := client.StartExec(execID, config)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	if req.Method != "POST" {
		t.Errorf("ExecStart: wrong HTTP method. Want %q. Got %q.", "POST", req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/exec/" + execID + "/start"))
	if gotPath := req.URL.Path; gotPath != expectedURL.Path {
		t.Errorf("ExecCreate: Wrong path in request. Want %q. Got %q.", expectedURL.Path, gotPath)
	}
	t.Log(req.Body)
	var gotBody struct{ Detach bool }
	err = json.NewDecoder(req.Body).Decode(&gotBody)
	if err != nil {
		t.Fatal(err)
	}
	if !gotBody.Detach {
		t.Fatal("Expected Detach in StartExecOptions to be true")
	}
}

func TestExecStartAndAttach(t *testing.T) {
	var reader = strings.NewReader("send value")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte{1, 0, 0, 0, 0, 0, 0, 5})
		w.Write([]byte("hello"))
	}))
	defer server.Close()
	client, _ := NewClient(server.URL)
	client.SkipServerVersionCheck = true
	var stdout, stderr bytes.Buffer
	success := make(chan struct{})
	execID := "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"
	opts := StartExecOptions{
		OutputStream: &stdout,
		ErrorStream:  &stderr,
		InputStream:  reader,
		RawTerminal:  true,
		Success:      success,
	}
	go func() {
		if err := client.StartExec(execID, opts); err != nil {
			t.Error(err)
		}
	}()
	<-success
}

func TestExecResize(t *testing.T) {
	t.Parallel()
	execID := "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"
	fakeRT := &FakeRoundTripper{status: http.StatusOK}
	client := newTestClient(fakeRT)
	err := client.ResizeExecTTY(execID, 10, 20)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	if req.Method != "POST" {
		t.Errorf("ExecStart: wrong HTTP method. Want %q. Got %q.", "POST", req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/exec/" + execID + "/resize?h=10&w=20"))
	if gotPath := req.URL.RequestURI(); gotPath != expectedURL.RequestURI() {
		t.Errorf("ExecCreate: Wrong path in request. Want %q. Got %q.", expectedURL.Path, gotPath)
	}
}

func TestExecInspect(t *testing.T) {
	t.Parallel()
	jsonExec := `{
	  "CanRemove": false,
	  "ContainerID": "b53ee82b53a40c7dca428523e34f741f3abc51d9f297a14ff874bf761b995126",
	  "DetachKeys": "",
	  "ExitCode": 2,
	  "ID": "f33bbfb39f5b142420f4759b2348913bd4a8d1a6d7fd56499cb41a1bb91d7b3b",
	  "OpenStderr": true,
	  "OpenStdin": true,
	  "OpenStdout": true,
	  "ProcessConfig": {
	    "arguments": [
	      "-c",
	      "exit 2"
	    ],
	    "entrypoint": "sh",
	    "privileged": false,
	    "tty": true,
	    "user": "1000"
	  },
	  "Running": false
	}`
	var expected ExecInspect
	err := json.Unmarshal([]byte(jsonExec), &expected)
	if err != nil {
		t.Fatal(err)
	}
	fakeRT := &FakeRoundTripper{message: jsonExec, status: http.StatusOK}
	client := newTestClient(fakeRT)
	expectedID := "b53ee82b53a40c7dca428523e34f741f3abc51d9f297a14ff874bf761b995126"
	execObj, err := client.InspectExec(expectedID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*execObj, expected) {
		t.Errorf("ExecInspect: Expected %#v. Got %#v.", expected, *execObj)
	}
	req := fakeRT.requests[0]
	if req.Method != "GET" {
		t.Errorf("ExecInspect: wrong HTTP method. Want %q. Got %q.", "GET", req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/exec/" + expectedID + "/json"))
	if gotPath := fakeRT.requests[0].URL.Path; gotPath != expectedURL.Path {
		t.Errorf("ExecInspect: Wrong path in request. Want %q. Got %q.", expectedURL.Path, gotPath)
	}
}

// Copyright 2013 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storageos

import (
	"bytes"
	"context"
	"fmt"
	"github.com/storageos/go-api/serror"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func newTestClient(rt http.RoundTripper) *Client {
	testAPIVersion, _ := NewAPIVersion("1")
	return &Client{
		HTTPClient:             &http.Client{Transport: rt},
		SkipServerVersionCheck: true,
		serverAPIVersion:       testAPIVersion,
	}
}

func TestNewAPIClient(t *testing.T) {
	endpoint := "http://localhost:4243"
	client, err := NewClient(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if !client.SkipServerVersionCheck {
		t.Error("Expected SkipServerVersionCheck to be true, got false")
	}
	if client.requestedAPIVersion != 0 {
		t.Errorf("Expected requestedAPIVersion to be nil, got %#v.", client.requestedAPIVersion)
	}
}

func TestNewVersionedClient(t *testing.T) {
	endpoint := "http://localhost:4243"
	client, err := NewVersionedClient(endpoint, "1")
	if err != nil {
		t.Fatal(err)
	}
	if reqVersion := client.requestedAPIVersion; reqVersion != 1 {
		t.Errorf("Wrong requestAPIVersion. Want %d. Got %d.", 1, reqVersion)
	}
	if client.SkipServerVersionCheck {
		t.Error("Expected SkipServerVersionCheck to be false, got true")
	}
}

func TestNewClientInvalidEndpoint(t *testing.T) {
	cases := []string{
		"htp://localhost:3243", "http://localhost:a",
		"", "http://localhost:8080:8383", "http://localhost:65536",
		"https://localhost:-20",
	}
	for _, c := range cases {
		client, err := NewClient(c)
		if client != nil {
			t.Errorf("Want <nil> client for invalid endpoint (%v), got %#v.", c, client)
		}
		if serror.IsStorageOSError(err) {
			if serror.ErrorKind(err) != serror.InvalidHostConfig {
				t.Errorf("NewClient(%q): Got invalid error for invalid endpoint. Want serror.InvalidHostConfig. Got %#v.", c, err)
			}
		}
	}
}

func TestNewClientNoSchemeEndpoint(t *testing.T) {
	cases := []string{"localhost", "localhost:8080"}
	for _, c := range cases {
		client, err := NewClient(c)
		if client == nil {
			t.Errorf("Want client for scheme-less endpoint, got <nil>")
		}
		if err != nil {
			t.Errorf("Got unexpected error scheme-less endpoint: %q", err)
		}
	}
}

func TestGetURLVersioned(t *testing.T) {
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}

	var tests = []struct {
		endpoint string
		path     string
		expected string
	}{
		{"http://localhost:4243/", "/", "http://storageos-cluster/v0/"},
		{"http://localhost:4243", "/", "http://storageos-cluster/v0/"},
		{"http://localhost:4243", "/containers/ps", "http://storageos-cluster/v0/containers/ps"},
		{"tcp://localhost:4243", "/containers/ps", "http://storageos-cluster/v0/containers/ps"},
		{"http://localhost:4243/////", "/", "http://storageos-cluster/v0/"},
	}
	for i, tt := range tests {
		client, _ := NewClient(tt.endpoint)

		// replace the client with a fake
		client.HTTPClient = &http.Client{Transport: fakeRT}
		client.SkipServerVersionCheck = true

		// drive a request to capture the url
		client.do("GET", tt.path, doOptions{})

		got := fakeRT.requests[i].URL.String()
		if got != tt.expected {
			t.Errorf("getURL(%q): Got %s. Want %s.", tt.path, got, tt.expected)
		}
	}
}

func TestError(t *testing.T) {
	fakeBody := ioutil.NopCloser(bytes.NewBufferString("bad parameter"))
	resp := &http.Response{
		StatusCode: 400,
		Body:       fakeBody,
	}
	err := newError(resp)
	expected := Error{Status: 400, Message: "bad parameter"}
	if !reflect.DeepEqual(expected, *err) {
		t.Errorf("Wrong error type. Want %#v. Got %#v.", expected, *err)
	}
	message := "API error (Server failed to process your request. Was the data correct?): bad parameter"
	if err.Error() != message {
		t.Errorf("Wrong error message. Want %q. Got %q.", message, err.Error())
	}
}

func TestQueryString(t *testing.T) {
	v := float32(2.4)
	f32QueryString := fmt.Sprintf("w=%s&x=10&y=10.35", strconv.FormatFloat(float64(v), 'f', -1, 64))
	jsonPerson := url.QueryEscape(`{"Name":"gopher","age":4}`)
	var tests = []struct {
		input interface{}
		want  string
	}{
		// {&types.VolumeListOptions{All: true}, "all=1"},
		// {types.VolumeListOptions{All: true}, "all=1"},
		// {VolumeListOptions{Filters: map[string][]string{"status": {"paused", "running"}}}, "filters=%7B%22status%22%3A%5B%22paused%22%2C%22running%22%5D%7D"},
		{dumb{X: 10, Y: 10.35000}, "x=10&y=10.35"},
		{dumb{W: v, X: 10, Y: 10.35000}, f32QueryString},
		{dumb{X: 10, Y: 10.35000, Z: 10}, "x=10&y=10.35&zee=10"},
		{dumb{v: 4, X: 10, Y: 10.35000}, "x=10&y=10.35"},
		{dumb{T: 10, Y: 10.35000}, "y=10.35"},
		{dumb{Person: &person{Name: "gopher", Age: 4}}, "p=" + jsonPerson},
		{nil, ""},
		{10, ""},
		{"not_a_struct", ""},
	}
	for _, tt := range tests {
		got := queryString(tt.input)
		if got != tt.want {
			t.Errorf("queryString(%v). Want %q. Got %q.", tt.input, tt.want, got)
		}
	}
}

func TestAPIVersions(t *testing.T) {
	var tests = []struct {
		a string
		b APIVersion
	}{
		{"1", 1},
		{"2", 2},
	}

	for _, tt := range tests {
		a, err := NewAPIVersion(tt.a)
		if err != nil {
			t.Fatal(err)
		}

		if a != tt.b {
			t.Errorf("Expected %q == %d", a, tt.b)
		}
	}
}

func TestSetAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ := r.BasicAuth()
		if user != "user" || pass != "secret" {
			http.Error(w, "Unauthorized.", 401)
			return
		}
	}))
	t.Log(srv.URL)
	client, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	client.SetAuth("user", "secret")
	_, err = client.do("POST", "/xxx", doOptions{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPing(t *testing.T) {
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	err := client.Ping()
	if err != nil {
		t.Fatal(err)
	}
}

func TestPingFailing(t *testing.T) {
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusInternalServerError}
	client := newTestClient(fakeRT)
	err := client.Ping()
	if err == nil {
		t.Fatal("Expected non nil error, got nil")
	}
	expectedErrMsg := "API error (Server failed to process your request. Was the data correct?): "
	if err.Error() != expectedErrMsg {
		t.Fatalf("Expected error to be %q, got: %q", expectedErrMsg, err.Error())
	}
}

func TestPingFailingWrongStatus(t *testing.T) {
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusAccepted}
	client := newTestClient(fakeRT)
	err := client.Ping()
	if err == nil {
		t.Fatal("Expected non nil error, got nil")
	}
	expectedErrMsg := "API error (Accepted): "
	if err.Error() != expectedErrMsg {
		t.Fatalf("Expected error to be %q, got: %q", expectedErrMsg, err.Error())
	}
}

func TestClientDoContextDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	client, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = client.do("POST", VolumeAPIPrefix, doOptions{
		namespace: "testns",
		context:   ctx,
	})
	if err != context.DeadlineExceeded {
		t.Fatalf("expected %s, got: %s", context.DeadlineExceeded, err)
	}
}

func TestClientDoContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	client, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	_, err = client.do("POST", VolumeAPIPrefix, doOptions{
		namespace: "testns",
		context:   ctx,
	})
	if err != context.Canceled {
		t.Fatalf("expected %s, got: %s", context.Canceled, err)
	}
}

type FakeRoundTripper struct {
	message  string
	status   int
	header   map[string]string
	requests []*http.Request
}

func (rt *FakeRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	body := strings.NewReader(rt.message)
	rt.requests = append(rt.requests, r)
	res := &http.Response{
		StatusCode: rt.status,
		Body:       ioutil.NopCloser(body),
		Header:     make(http.Header),
	}
	for k, v := range rt.header {
		res.Header.Set(k, v)
	}
	return res, nil
}

func (rt *FakeRoundTripper) Reset() {
	rt.requests = nil
}

type person struct {
	Name string
	Age  int `json:"age"`
}

type dumb struct {
	T      int `qs:"-"`
	v      int
	W      float32
	X      int
	Y      float64
	Z      int     `qs:"zee"`
	Person *person `qs:"p"`
}

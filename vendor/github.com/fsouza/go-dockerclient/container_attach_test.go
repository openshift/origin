package docker

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAttachToContainerLogs(t *testing.T) {
	t.Parallel()
	var req http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte{1, 0, 0, 0, 0, 0, 0, 19})
		w.Write([]byte("something happened!"))
		req = *r
	}))
	defer server.Close()
	client, _ := NewClient(server.URL)
	client.SkipServerVersionCheck = true
	var buf bytes.Buffer
	opts := AttachToContainerOptions{
		Container:    "a123456",
		OutputStream: &buf,
		Stdout:       true,
		Stderr:       true,
		Logs:         true,
	}
	err := client.AttachToContainer(opts)
	if err != nil {
		t.Fatal(err)
	}
	expected := "something happened!"
	if buf.String() != expected {
		t.Errorf("AttachToContainer for logs: wrong output. Want %q. Got %q.", expected, buf.String())
	}
	if req.Method != http.MethodPost {
		t.Errorf("AttachToContainer: wrong HTTP method. Want POST. Got %s.", req.Method)
	}
	u, _ := url.Parse(client.getURL("/containers/a123456/attach"))
	if req.URL.Path != u.Path {
		t.Errorf("AttachToContainer for logs: wrong HTTP path. Want %q. Got %q.", u.Path, req.URL.Path)
	}
	expectedQs := map[string][]string{
		"logs":   {"1"},
		"stdout": {"1"},
		"stderr": {"1"},
	}
	got := map[string][]string(req.URL.Query())
	if !reflect.DeepEqual(got, expectedQs) {
		t.Errorf("AttachToContainer: wrong query string. Want %#v. Got %#v.", expectedQs, got)
	}
}

func TestAttachToContainer(t *testing.T) {
	t.Parallel()
	reader := strings.NewReader("send value")
	var req http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte{1, 0, 0, 0, 0, 0, 0, 5})
		w.Write([]byte("hello"))
		req = *r
	}))
	defer server.Close()
	client, _ := NewClient(server.URL)
	client.SkipServerVersionCheck = true
	var stdout, stderr bytes.Buffer
	opts := AttachToContainerOptions{
		Container:    "a123456",
		OutputStream: &stdout,
		ErrorStream:  &stderr,
		InputStream:  reader,
		Stdin:        true,
		Stdout:       true,
		Stderr:       true,
		Stream:       true,
		RawTerminal:  true,
	}
	err := client.AttachToContainer(opts)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string][]string{
		"stdin":  {"1"},
		"stdout": {"1"},
		"stderr": {"1"},
		"stream": {"1"},
	}
	got := map[string][]string(req.URL.Query())
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("AttachToContainer: wrong query string. Want %#v. Got %#v.", expected, got)
	}
}

func TestAttachToContainerSentinel(t *testing.T) {
	t.Parallel()
	reader := strings.NewReader("send value")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte{1, 0, 0, 0, 0, 0, 0, 5})
		w.Write([]byte("hello"))
	}))
	defer server.Close()
	client, _ := NewClient(server.URL)
	client.SkipServerVersionCheck = true
	var stdout, stderr bytes.Buffer
	success := make(chan struct{})
	opts := AttachToContainerOptions{
		Container:    "a123456",
		OutputStream: &stdout,
		ErrorStream:  &stderr,
		InputStream:  reader,
		Stdin:        true,
		Stdout:       true,
		Stderr:       true,
		Stream:       true,
		RawTerminal:  true,
		Success:      success,
	}
	errCh := make(chan error)
	go func() {
		errCh <- client.AttachToContainer(opts)
	}()
	success <- <-success
	if err := <-errCh; err != nil {
		t.Error(err)
	}
}

func TestAttachToContainerNilStdout(t *testing.T) {
	t.Parallel()
	reader := strings.NewReader("send value")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte{1, 0, 0, 0, 0, 0, 0, 5})
		w.Write([]byte("hello"))
	}))
	defer server.Close()
	client, _ := NewClient(server.URL)
	client.SkipServerVersionCheck = true
	var stderr bytes.Buffer
	opts := AttachToContainerOptions{
		Container:    "a123456",
		OutputStream: nil,
		ErrorStream:  &stderr,
		InputStream:  reader,
		Stdin:        true,
		Stdout:       true,
		Stderr:       true,
		Stream:       true,
		RawTerminal:  true,
	}
	err := client.AttachToContainer(opts)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAttachToContainerNilStderr(t *testing.T) {
	t.Parallel()
	reader := strings.NewReader("send value")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte{1, 0, 0, 0, 0, 0, 0, 5})
		w.Write([]byte("hello"))
	}))
	defer server.Close()
	client, _ := NewClient(server.URL)
	client.SkipServerVersionCheck = true
	var stdout bytes.Buffer
	opts := AttachToContainerOptions{
		Container:    "a123456",
		OutputStream: &stdout,
		InputStream:  reader,
		Stdin:        true,
		Stdout:       true,
		Stderr:       true,
		Stream:       true,
		RawTerminal:  true,
	}
	err := client.AttachToContainer(opts)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAttachToContainerStdinOnly(t *testing.T) {
	t.Parallel()
	reader := strings.NewReader("send value")
	serverFinished := make(chan struct{})
	clientFinished := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("cannot hijack server connection")
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Fatal(err)
		}
		// wait for client to indicate it's finished
		<-clientFinished
		// inform test that the server has finished
		close(serverFinished)
		conn.Close()
	}))
	defer server.Close()
	client, _ := NewClient(server.URL)
	client.SkipServerVersionCheck = true
	success := make(chan struct{})
	opts := AttachToContainerOptions{
		Container:   "a123456",
		InputStream: reader,
		Stdin:       true,
		Stdout:      false,
		Stderr:      false,
		Stream:      true,
		RawTerminal: false,
		Success:     success,
	}
	go func() {
		if err := client.AttachToContainer(opts); err != nil {
			t.Error(err)
		}
		// client's attach session is over
		close(clientFinished)
	}()
	success <- <-success
	// wait for server to finish handling attach
	<-serverFinished
}

func TestAttachToContainerRawTerminalFalse(t *testing.T) {
	t.Parallel()
	input := strings.NewReader("send value")
	var req http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req = *r
		w.WriteHeader(http.StatusOK)
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("cannot hijack server connection")
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Fatal(err)
		}
		conn.Write([]byte{1, 0, 0, 0, 0, 0, 0, 5})
		conn.Write([]byte("hello"))
		conn.Write([]byte{2, 0, 0, 0, 0, 0, 0, 6})
		conn.Write([]byte("hello!"))
		time.Sleep(10 * time.Millisecond)
		conn.Close()
	}))
	client, _ := NewClient(server.URL)
	client.SkipServerVersionCheck = true
	var stdout, stderr bytes.Buffer
	opts := AttachToContainerOptions{
		Container:    "a123456",
		OutputStream: &stdout,
		ErrorStream:  &stderr,
		InputStream:  input,
		Stdin:        true,
		Stdout:       true,
		Stderr:       true,
		Stream:       true,
		RawTerminal:  false,
	}
	client.AttachToContainer(opts)
	expected := map[string][]string{
		"stdin":  {"1"},
		"stdout": {"1"},
		"stderr": {"1"},
		"stream": {"1"},
	}
	got := map[string][]string(req.URL.Query())
	server.Close()
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("AttachToContainer: wrong query string. Want %#v. Got %#v.", expected, got)
	}
	if stdout.String() != "hello" {
		t.Errorf("AttachToContainer: wrong content written to stdout. Want %q. Got %q.", "hello", stdout.String())
	}
	if stderr.String() != "hello!" {
		t.Errorf("AttachToContainer: wrong content written to stderr. Want %q. Got %q.", "hello!", stderr.String())
	}
}

func TestAttachToContainerWithoutContainer(t *testing.T) {
	t.Parallel()
	var client Client
	err := client.AttachToContainer(AttachToContainerOptions{})
	expectNoSuchContainer(t, "", err)
}

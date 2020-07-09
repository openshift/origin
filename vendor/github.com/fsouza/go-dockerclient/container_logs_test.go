package docker

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
)

func TestLogs(t *testing.T) {
	t.Parallel()
	var req http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prefix := []byte{1, 0, 0, 0, 0, 0, 0, 19}
		w.Write(prefix)
		w.Write([]byte("something happened!"))
		req = *r
	}))
	defer server.Close()
	client, _ := NewClient(server.URL)
	client.SkipServerVersionCheck = true
	var buf bytes.Buffer
	opts := LogsOptions{
		Container:    "a123456",
		OutputStream: &buf,
		Follow:       true,
		Stdout:       true,
		Stderr:       true,
		Timestamps:   true,
	}
	err := client.Logs(opts)
	if err != nil {
		t.Fatal(err)
	}
	expected := "something happened!"
	if buf.String() != expected {
		t.Errorf("Logs: wrong output. Want %q. Got %q.", expected, buf.String())
	}
	if req.Method != http.MethodGet {
		t.Errorf("Logs: wrong HTTP method. Want GET. Got %s.", req.Method)
	}
	u, _ := url.Parse(client.getURL("/containers/a123456/logs"))
	if req.URL.Path != u.Path {
		t.Errorf("AttachToContainer for logs: wrong HTTP path. Want %q. Got %q.", u.Path, req.URL.Path)
	}
	expectedQs := map[string][]string{
		"follow":     {"1"},
		"stdout":     {"1"},
		"stderr":     {"1"},
		"timestamps": {"1"},
		"tail":       {"all"},
	}
	got := map[string][]string(req.URL.Query())
	if !reflect.DeepEqual(got, expectedQs) {
		t.Errorf("Logs: wrong query string. Want %#v. Got %#v.", expectedQs, got)
	}
}

func TestLogsNilStdoutDoesntFail(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		prefix := []byte{1, 0, 0, 0, 0, 0, 0, 19}
		w.Write(prefix)
		w.Write([]byte("something happened!"))
	}))
	defer server.Close()
	client, _ := NewClient(server.URL)
	client.SkipServerVersionCheck = true
	opts := LogsOptions{
		Container:  "a123456",
		Follow:     true,
		Stdout:     true,
		Stderr:     true,
		Timestamps: true,
	}
	err := client.Logs(opts)
	if err != nil {
		t.Fatal(err)
	}
}

func TestLogsNilStderrDoesntFail(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		prefix := []byte{2, 0, 0, 0, 0, 0, 0, 19}
		w.Write(prefix)
		w.Write([]byte("something happened!"))
	}))
	defer server.Close()
	client, _ := NewClient(server.URL)
	client.SkipServerVersionCheck = true
	opts := LogsOptions{
		Container:  "a123456",
		Follow:     true,
		Stdout:     true,
		Stderr:     true,
		Timestamps: true,
	}
	err := client.Logs(opts)
	if err != nil {
		t.Fatal(err)
	}
}

func TestLogsSpecifyingTail(t *testing.T) {
	t.Parallel()
	var req http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prefix := []byte{1, 0, 0, 0, 0, 0, 0, 19}
		w.Write(prefix)
		w.Write([]byte("something happened!"))
		req = *r
	}))
	defer server.Close()
	client, _ := NewClient(server.URL)
	client.SkipServerVersionCheck = true
	var buf bytes.Buffer
	opts := LogsOptions{
		Container:    "a123456",
		OutputStream: &buf,
		Follow:       true,
		Stdout:       true,
		Stderr:       true,
		Timestamps:   true,
		Tail:         "100",
	}
	err := client.Logs(opts)
	if err != nil {
		t.Fatal(err)
	}
	expected := "something happened!"
	if buf.String() != expected {
		t.Errorf("Logs: wrong output. Want %q. Got %q.", expected, buf.String())
	}
	if req.Method != http.MethodGet {
		t.Errorf("Logs: wrong HTTP method. Want GET. Got %s.", req.Method)
	}
	u, _ := url.Parse(client.getURL("/containers/a123456/logs"))
	if req.URL.Path != u.Path {
		t.Errorf("AttachToContainer for logs: wrong HTTP path. Want %q. Got %q.", u.Path, req.URL.Path)
	}
	expectedQs := map[string][]string{
		"follow":     {"1"},
		"stdout":     {"1"},
		"stderr":     {"1"},
		"timestamps": {"1"},
		"tail":       {"100"},
	}
	got := map[string][]string(req.URL.Query())
	if !reflect.DeepEqual(got, expectedQs) {
		t.Errorf("Logs: wrong query string. Want %#v. Got %#v.", expectedQs, got)
	}
}

func TestLogsRawTerminal(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("something happened!"))
	}))
	defer server.Close()
	client, _ := NewClient(server.URL)
	client.SkipServerVersionCheck = true
	var buf bytes.Buffer
	opts := LogsOptions{
		Container:    "a123456",
		OutputStream: &buf,
		Follow:       true,
		RawTerminal:  true,
		Stdout:       true,
		Stderr:       true,
		Timestamps:   true,
		Tail:         "100",
	}
	err := client.Logs(opts)
	if err != nil {
		t.Fatal(err)
	}
	expected := "something happened!"
	if buf.String() != expected {
		t.Errorf("Logs: wrong output. Want %q. Got %q.", expected, buf.String())
	}
}

func TestLogsNoContainer(t *testing.T) {
	t.Parallel()
	var client Client
	err := client.Logs(LogsOptions{})
	expectNoSuchContainer(t, "", err)
}

package docker

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestTopContainer(t *testing.T) {
	t.Parallel()
	jsonTop := `{
  "Processes": [
    [
      "ubuntu",
      "3087",
      "815",
      "0",
      "01:44",
      "?",
      "00:00:00",
      "cmd1"
    ],
    [
      "root",
      "3158",
      "3087",
      "0",
      "01:44",
      "?",
      "00:00:01",
      "cmd2"
    ]
  ],
  "Titles": [
    "UID",
    "PID",
    "PPID",
    "C",
    "STIME",
    "TTY",
    "TIME",
    "CMD"
  ]
}`
	var expected TopResult
	err := json.Unmarshal([]byte(jsonTop), &expected)
	if err != nil {
		t.Fatal(err)
	}
	id := "4fa6e0f0"
	fakeRT := &FakeRoundTripper{message: jsonTop, status: http.StatusOK}
	client := newTestClient(fakeRT)
	processes, err := client.TopContainer(id, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(processes, expected) {
		t.Errorf("TopContainer: Expected %#v. Got %#v.", expected, processes)
	}
	if len(processes.Processes) != 2 || len(processes.Processes[0]) != 8 ||
		processes.Processes[0][7] != "cmd1" {
		t.Errorf("TopContainer: Process list to include cmd1. Got %#v.", processes)
	}
	expectedURI := "/containers/" + id + "/top"
	if !strings.HasSuffix(fakeRT.requests[0].URL.String(), expectedURI) {
		t.Errorf("TopContainer: Expected URI to have %q. Got %q.", expectedURI, fakeRT.requests[0].URL.String())
	}
}

func TestTopContainerNotFound(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "no such container", status: http.StatusNotFound})
	_, err := client.TopContainer("abef348", "")
	expectNoSuchContainer(t, "abef348", err)
}

func TestTopContainerWithPsArgs(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "no such container", status: http.StatusNotFound}
	client := newTestClient(fakeRT)
	_, err := client.TopContainer("abef348", "aux")
	expectNoSuchContainer(t, "abef348", err)

	expectedURI := "/containers/abef348/top?ps_args=aux"
	if !strings.HasSuffix(fakeRT.requests[0].URL.String(), expectedURI) {
		t.Errorf("TopContainer: Expected URI to have %q. Got %q.", expectedURI, fakeRT.requests[0].URL.String())
	}
}

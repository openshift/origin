package docker

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"testing"
)

func TestListContainers(t *testing.T) {
	t.Parallel()
	jsonContainers := `[
     {
             "Id": "8dfafdbc3a40",
             "Image": "base:latest",
             "Command": "echo 1",
             "Created": 1367854155,
             "Ports":[{"PrivatePort": 2222, "PublicPort": 3333, "Type": "tcp"}],
             "Status": "Exit 0"
     },
     {
             "Id": "9cd87474be90",
             "Image": "base:latest",
             "Command": "echo 222222",
             "Created": 1367854155,
             "Ports":[{"PrivatePort": 2222, "PublicPort": 3333, "Type": "tcp"}],
             "Status": "Exit 0"
     },
     {
             "Id": "3176a2479c92",
             "Image": "base:latest",
             "Command": "echo 3333333333333333",
             "Created": 1367854154,
             "Ports":[{"PrivatePort": 2221, "PublicPort": 3331, "Type": "tcp"}],
             "Status": "Exit 0"
     },
     {
             "Id": "4cb07b47f9fb",
             "Image": "base:latest",
             "Command": "echo 444444444444444444444444444444444",
             "Ports":[{"PrivatePort": 2223, "PublicPort": 3332, "Type": "tcp"}],
             "Created": 1367854152,
             "Status": "Exit 0"
     }
]`
	var expected []APIContainers
	err := json.Unmarshal([]byte(jsonContainers), &expected)
	if err != nil {
		t.Fatal(err)
	}
	client := newTestClient(&FakeRoundTripper{message: jsonContainers, status: http.StatusOK})
	containers, err := client.ListContainers(ListContainersOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(containers, expected) {
		t.Errorf("ListContainers: Expected %#v. Got %#v.", expected, containers)
	}
}

func TestListContainersParams(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input  ListContainersOptions
		params map[string][]string
	}{
		{ListContainersOptions{}, map[string][]string{}},
		{ListContainersOptions{All: true}, map[string][]string{"all": {"1"}}},
		{ListContainersOptions{All: true, Limit: 10}, map[string][]string{"all": {"1"}, "limit": {"10"}}},
		{
			ListContainersOptions{All: true, Limit: 10, Since: "adf9983", Before: "abdeef"},
			map[string][]string{"all": {"1"}, "limit": {"10"}, "since": {"adf9983"}, "before": {"abdeef"}},
		},
		{
			ListContainersOptions{Filters: map[string][]string{"status": {"paused", "running"}}},
			map[string][]string{"filters": {"{\"status\":[\"paused\",\"running\"]}"}},
		},
		{
			ListContainersOptions{All: true, Filters: map[string][]string{"exited": {"0"}, "status": {"exited"}}},
			map[string][]string{"all": {"1"}, "filters": {"{\"exited\":[\"0\"],\"status\":[\"exited\"]}"}},
		},
	}
	const expectedPath = "/containers/json"
	for _, tt := range tests {
		test := tt
		t.Run("", func(t *testing.T) {
			t.Parallel()
			fakeRT := &FakeRoundTripper{message: "[]", status: http.StatusOK}
			client := newTestClient(fakeRT)
			if _, err := client.ListContainers(test.input); err != nil {
				t.Error(err)
			}
			got := map[string][]string(fakeRT.requests[0].URL.Query())
			if !reflect.DeepEqual(got, test.params) {
				t.Errorf("Expected %#v, got %#v.", test.params, got)
			}
			if path := fakeRT.requests[0].URL.Path; path != expectedPath {
				t.Errorf("Wrong path on request. Want %q. Got %q.", expectedPath, path)
			}
			if meth := fakeRT.requests[0].Method; meth != http.MethodGet {
				t.Errorf("Wrong HTTP method. Want GET. Got %s.", meth)
			}
		})
	}
}

func TestListContainersFailure(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status  int
		message string
	}{
		{400, "bad parameter"},
		{500, "internal server error"},
	}
	for _, tt := range tests {
		test := tt
		t.Run(strconv.Itoa(test.status), func(t *testing.T) {
			t.Parallel()
			client := newTestClient(&FakeRoundTripper{message: test.message, status: test.status})
			expected := Error{Status: test.status, Message: test.message}
			containers, err := client.ListContainers(ListContainersOptions{})
			if !reflect.DeepEqual(expected, *err.(*Error)) {
				t.Errorf("Wrong error in ListContainers. Want %#v. Got %#v.", expected, err)
			}
			if len(containers) > 0 {
				t.Errorf("ListContainers failure. Expected empty list. Got %#v.", containers)
			}
		})
	}
}

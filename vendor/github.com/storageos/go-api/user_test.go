package storageos

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/storageos/go-api/types"
)

func TestUserList(t *testing.T) {
	userData := `[
	{
		"id": "0d01ce98-c0d7-0538-3f98-9cb86a0c6ff0",
		"username": "alice",
		"groups": "foo,bar",
		"email": "alice@very-alice.io",
		"role": "admin"
	},
	{
		"id": "7087ebc8-aa47-3f54-fbcd-74db102a1253",
		"username": "bob",
		"groups": "",
		"email": "bob@much-bob.sh",
		"role": "user"
	}
]`

	var expected []*types.User
	if err := json.Unmarshal([]byte(userData), &expected); err != nil {
		t.Fatal(err)
	}

	client := newTestClient(&FakeRoundTripper{message: userData, status: http.StatusOK})
	users, err := client.UserList(types.ListOptions{})
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(users, expected) {
		t.Errorf("Users: Wrong return value. Want %#v. Got %#v.", expected, users)
	}
}

func TestUserCreate(t *testing.T) {
	fakeRT := &FakeRoundTripper{status: http.StatusOK}
	client := newTestClient(fakeRT)
	err := client.UserCreate(
		types.UserCreateOptions{
			Username: "muchUser",
			Groups:   []string{"foo", "bar"},
			Password: "ultraSecret",
			Role:     "admin",
			Context:  context.Background(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	expectedMethod := "POST"
	if req.Method != expectedMethod {
		t.Errorf("UserCreate(): Wrong HTTP method. Want %s. Got %s.", expectedMethod, req.Method)
	}
	u, _ := url.Parse(client.getAPIPath(UserAPIPrefix, url.Values{}, false))
	if req.URL.Path != u.Path {
		t.Errorf("UserCreate(): Wrong request path. Want %q. Got %q.", u.Path, req.URL.Path)
	}
}

func TestUser(t *testing.T) {
	body := `{
	"id": "0d01ce98-c0d7-0538-3f98-9cb86a0c6ff0",
	"username": "alice",
	"groups": "foo,bar",
	"email": "alice@very-alice.io",
	"role": "admin"
}`

	var expected types.User
	if err := json.Unmarshal([]byte(body), &expected); err != nil {
		t.Fatal(err)
	}
	fakeRT := &FakeRoundTripper{message: body, status: http.StatusOK}
	client := newTestClient(fakeRT)
	name := "0d01ce98-c0d7-0538-3f98-9cb86a0c6ff0"
	user, err := client.User(name)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(user, &expected) {
		t.Errorf("Wrong return value. Want %#v. Got %#v.", expected, user)
	}
	req := fakeRT.requests[0]
	expectedMethod := "GET"
	if req.Method != expectedMethod {
		t.Errorf("Wrong HTTP method. Want %s. Got %s.", expectedMethod, req.Method)
	}
	path := fmt.Sprintf("%s/%s", UserAPIPrefix, name)
	u, _ := url.Parse(client.getAPIPath(path, url.Values{}, false))
	if req.URL.Path != u.Path {
		t.Errorf("Wrong request path. Want %q. Got %q.", u.Path, req.URL.Path)
	}
}

func TestUserDelete(t *testing.T) {
	name := "someID"
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusNoContent}
	client := newTestClient(fakeRT)
	err := client.UserDelete(
		types.DeleteOptions{
			Name: name,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	expectedMethod := "DELETE"
	if req.Method != expectedMethod {
		t.Errorf("Wrong HTTP method. Want %s. Got %s.", expectedMethod, req.Method)
	}
	path := fmt.Sprintf("%s/%s", UserAPIPrefix, name)
	u, _ := url.Parse(client.getAPIPath(path, url.Values{}, false))
	if req.URL.Path != u.Path {
		t.Errorf("Wrong request path. Want %q. Got %q.", u.Path, req.URL.Path)
	}
}

func TestUserDeleteNotFound(t *testing.T) {
	client := newTestClient(&FakeRoundTripper{message: "no such user", status: http.StatusNotFound})
	err := client.UserDelete(
		types.DeleteOptions{
			Name: "badname",
		},
	)
	if err != ErrNoSuchUser {
		t.Errorf("wrong error. Want %#v. Got %#v.", ErrNoSuchUser, err)
	}
}

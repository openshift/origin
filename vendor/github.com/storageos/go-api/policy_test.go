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

func TestPolicyList(t *testing.T) {
	data := `{
	"ce560594-4318-1806-45f0-7649c61800b6": {
		"Spec": {
			"User": "",
			"Group": "foo",
			"Readonly": false,
			"APIGroup": "",
			"Resource": "",
			"Namespace": "",
			"NonResourcePath": "*"
		}
	}
}`

	var expected types.PolicySet
	if err := json.Unmarshal([]byte(data), &expected); err != nil {
		t.Fatal(err)
	}

	client := newTestClient(&FakeRoundTripper{message: data, status: http.StatusOK})
	policies, err := client.PolicyList(types.ListOptions{})
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(policies, expected) {
		t.Errorf("Users: Wrong return value. Want %#v. Got %#v.", expected, policies)
	}
}

func TestPolicyCreate(t *testing.T) {
	fakeRT := &FakeRoundTripper{status: http.StatusOK}
	client := newTestClient(fakeRT)
	err := client.PolicyCreate([]byte(`{"Spec": {"Group": "foo","NonResourcePath": "*"}}`), context.Background())
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	expectedMethod := "POST"
	if req.Method != expectedMethod {
		t.Errorf("Wrong HTTP method. Want %s. Got %s.", expectedMethod, req.Method)
	}
	u, _ := url.Parse(client.getAPIPath(PolicyAPIPrefix, url.Values{}, false))
	if req.URL.Path != u.Path {
		t.Errorf("UserCreate(): Wrong request path. Want %q. Got %q.", u.Path, req.URL.Path)
	}
}

func TestPolicy(t *testing.T) {
	body := `{"Spec": {"Group": "foo","NonResourcePath": "*"}}`

	var expected types.Policy
	if err := json.Unmarshal([]byte(body), &expected); err != nil {
		t.Fatal(err)
	}
	fakeRT := &FakeRoundTripper{message: body, status: http.StatusOK}
	client := newTestClient(fakeRT)
	name := "0d01ce98-c0d7-0538-3f98-9cb86a0c6ff0"
	policy, err := client.Policy(name)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(policy, &expected) {
		t.Errorf("Wrong return value. Want %#v. Got %#v.", expected, policy)
	}
	req := fakeRT.requests[0]
	expectedMethod := "GET"
	if req.Method != expectedMethod {
		t.Errorf("Wrong HTTP method. Want %s. Got %s.", expectedMethod, req.Method)
	}
	path := fmt.Sprintf("%s/%s", PolicyAPIPrefix, name)
	u, _ := url.Parse(client.getAPIPath(path, url.Values{}, false))
	if req.URL.Path != u.Path {
		t.Errorf("Wrong request path. Want %q. Got %q.", u.Path, req.URL.Path)
	}
}

func TestPolicyDelete(t *testing.T) {
	name := "someID"
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusNoContent}
	client := newTestClient(fakeRT)
	err := client.PolicyDelete(
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
	path := fmt.Sprintf("%s/%s", PolicyAPIPrefix, name)
	u, _ := url.Parse(client.getAPIPath(path, url.Values{}, false))
	if req.URL.Path != u.Path {
		t.Errorf("Wrong request path. Want %q. Got %q.", u.Path, req.URL.Path)
	}
}

func TestPolicyDeleteNotFound(t *testing.T) {
	client := newTestClient(&FakeRoundTripper{message: "no such policy", status: http.StatusNotFound})
	err := client.PolicyDelete(
		types.DeleteOptions{
			Name: "badname",
		},
	)
	if err != ErrNoSuchPolicy {
		t.Errorf("wrong error. Want %#v. Got %#v.", ErrNoSuchPolicy, err)
	}
}

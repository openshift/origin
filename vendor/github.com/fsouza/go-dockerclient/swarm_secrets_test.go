// Copyright 2017 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/docker/docker/api/types/swarm"
)

func TestCreateSecret(t *testing.T) {
	t.Parallel()
	result := `{
  "Id": "13417726f7654bc286201f7c9accc98ccbd190efcc4753bf8ecfc0b61ef3dde8"
}`
	var expected swarm.Secret
	err := json.Unmarshal([]byte(result), &expected)
	if err != nil {
		t.Fatal(err)
	}
	fakeRT := &FakeRoundTripper{message: result, status: http.StatusOK}
	client := newTestClient(fakeRT)
	opts := CreateSecretOptions{}
	secret, err := client.CreateSecret(opts)
	if err != nil {
		t.Fatal(err)
	}
	id := "13417726f7654bc286201f7c9accc98ccbd190efcc4753bf8ecfc0b61ef3dde8"
	if secret.ID != id {
		t.Errorf("CreateSecret: wrong ID. Want %q. Got %q.", id, secret.ID)
	}
	req := fakeRT.requests[0]
	if req.Method != http.MethodPost {
		t.Errorf("CreateSecret: wrong HTTP method. Want %q. Got %q.", http.MethodPost, req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/secrets/create"))
	if gotPath := req.URL.Path; gotPath != expectedURL.Path {
		t.Errorf("CreateSecret: Wrong path in request. Want %q. Got %q.", expectedURL.Path, gotPath)
	}
	var gotBody Config
	err = json.NewDecoder(req.Body).Decode(&gotBody)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRemoveSecret(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	id := "13417726f7654bc286201f7c9accc98ccbd190efcc4753bf8ecfc0b61ef3dde8"
	opts := RemoveSecretOptions{ID: id}
	err := client.RemoveSecret(opts)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	if req.Method != http.MethodDelete {
		t.Errorf("RemoveSecret(%q): wrong HTTP method. Want %q. Got %q.", id, http.MethodDelete, req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/secrets/" + id))
	if gotPath := req.URL.Path; gotPath != expectedURL.Path {
		t.Errorf("RemoveSecret(%q): Wrong path in request. Want %q. Got %q.", id, expectedURL.Path, gotPath)
	}
}

func TestRemoveSecretNotFound(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "no such secret", status: http.StatusNotFound})
	err := client.RemoveSecret(RemoveSecretOptions{ID: "a2334"})
	expected := &NoSuchSecret{ID: "a2334"}
	if !reflect.DeepEqual(err, expected) {
		t.Errorf("RemoveSecret: Wrong error returned. Want %#v. Got %#v.", expected, err)
	}
}

func TestUpdateSecret(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	id := "13417726f7654bc286201f7c9accc98ccbd190efcc4753bf8ecfc0b61ef3dde8"
	update := UpdateSecretOptions{Version: 23}
	err := client.UpdateSecret(id, update)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	if req.Method != http.MethodPost {
		t.Errorf("UpdateSecret: wrong HTTP method. Want %q. Got %q.", http.MethodPost, req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/secrets/" + id + "/update?version=23"))
	if gotURI := req.URL.RequestURI(); gotURI != expectedURL.RequestURI() {
		t.Errorf("UpdateSecret: Wrong path in request. Want %q. Got %q.", expectedURL.RequestURI(), gotURI)
	}
	expectedContentType := "application/json"
	if contentType := req.Header.Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("UpdateSecret: Wrong content-type in request. Want %q. Got %q.", expectedContentType, contentType)
	}
	var out UpdateSecretOptions
	if err := json.NewDecoder(req.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	update.Version = 0
	if !reflect.DeepEqual(out, update) {
		t.Errorf("UpdateSecret: wrong body\ngot  %#v\nwant %#v", out, update)
	}
}

func TestUpdateSecretWithAuthentication(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	id := "13417726f7654bc286201f7c9accc98ccbd190efcc4753bf8ecfc0b61ef3dde8"
	update := UpdateSecretOptions{Version: 23}
	update.Auth = AuthConfiguration{
		Username: "gopher",
		Password: "gopher123",
		Email:    "gopher@tsuru.io",
	}

	err := client.UpdateSecret(id, update)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	if req.Method != http.MethodPost {
		t.Errorf("UpdateSecret: wrong HTTP method. Want %q. Got %q.", http.MethodPost, req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/secrets/" + id + "/update?version=23"))
	if gotURI := req.URL.RequestURI(); gotURI != expectedURL.RequestURI() {
		t.Errorf("UpdateSecret: Wrong path in request. Want %q. Got %q.", expectedURL.RequestURI(), gotURI)
	}
	expectedContentType := "application/json"
	if contentType := req.Header.Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("UpdateSecret: Wrong content-type in request. Want %q. Got %q.", expectedContentType, contentType)
	}
	var out UpdateSecretOptions
	if err := json.NewDecoder(req.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	var updateAuth AuthConfiguration

	auth, err := base64.URLEncoding.DecodeString(req.Header.Get("X-Registry-Auth"))
	if err != nil {
		t.Errorf("UpdateSecret: caught error decoding auth. %#v", err.Error())
	}

	err = json.Unmarshal(auth, &updateAuth)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(updateAuth, update.Auth) {
		t.Errorf("UpdateSecret: wrong auth configuration. Want %#v. Got %#v", update.Auth, updateAuth)
	}
}

func TestUpdateSecretNotFound(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "no such Secret", status: http.StatusNotFound})
	update := UpdateSecretOptions{}
	err := client.UpdateSecret("notfound", update)
	expected := &NoSuchSecret{ID: "notfound"}
	if !reflect.DeepEqual(err, expected) {
		t.Errorf("UpdateSecret: Wrong error returned. Want %#v. Got %#v.", expected, err)
	}
}

func TestInspectSecretNotFound(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "no such secret", status: http.StatusNotFound})
	secret, err := client.InspectSecret("notfound")
	if secret != nil {
		t.Errorf("InspectSecret: Expected <nil> Secret, got %#v", secret)
	}
	expected := &NoSuchSecret{ID: "notfound"}
	if !reflect.DeepEqual(err, expected) {
		t.Errorf("InspectSecret: Wrong error returned. Want %#v. Got %#v.", expected, err)
	}
}

func TestInspectSecret(t *testing.T) {
	t.Parallel()
	jsonSecret := `{
		"ID": "ak7w3gjqoa3kuz8xcpnyy0pvl",
		"Version": {
			"Index": 11
		},
		"CreatedAt": "2016-11-05T01:20:17.327670065Z",
		"UpdatedAt": "2016-11-05T01:20:17.327670065Z",
		"Spec": {
			"Name": "app-dev.crt",
			"Labels": {
				"foo": "bar"
			},
			"Driver": {
				"Name": "secret-bucket",
				"Options": {
					"OptionA": "value for driver option A",
					"OptionB": "value for driver option B"
				}
			}
		}
}`
	var expected swarm.Secret
	err := json.Unmarshal([]byte(jsonSecret), &expected)
	if err != nil {
		t.Fatal(err)
	}
	fakeRT := &FakeRoundTripper{message: jsonSecret, status: http.StatusOK}
	client := newTestClient(fakeRT)
	id := "ak7w3gjqoa3kuz8xcpnyy0pvl"
	secret, err := client.InspectSecret(id)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*secret, expected) {
		t.Errorf("InspectSecret(%q): Expected %#v. Got %#v.", id, expected, secret)
	}
	expectedURL, _ := url.Parse(client.getURL("/secrets/ak7w3gjqoa3kuz8xcpnyy0pvl"))
	if gotPath := fakeRT.requests[0].URL.Path; gotPath != expectedURL.Path {
		t.Errorf("InspectSecret(%q): Wrong path in request. Want %q. Got %q.", id, expectedURL.Path, gotPath)
	}
}

func TestListSecrets(t *testing.T) {
	t.Parallel()
	jsonSecrets := `[
			{
				"ID": "blt1owaxmitz71s9v5zh81zun",
				"Version": {
					"Index": 85
				},
				"CreatedAt": "2017-07-20T13:55:28.678958722Z",
				"UpdatedAt": "2017-07-20T13:55:28.678958722Z",
				"Spec": {
					"Name": "mysql-passwd",
					"Labels": {
						"some.label": "some.value"
					},
					"Driver": {
						"Name": "secret-bucket",
						"Options": {
							"OptionA": "value for driver option A",
							"OptionB": "value for driver option B"
						}
					}
				}
			},
			{
				"ID": "ktnbjxoalbkvbvedmg1urrz8h",
				"Version": {
					"Index": 11
				},
				"CreatedAt": "2016-11-05T01:20:17.327670065Z",
				"UpdatedAt": "2016-11-05T01:20:17.327670065Z",
				"Spec": {
					"Name": "app-dev.crt",
					"Labels": {
						"foo": "bar"
					}
				}
			}
]`
	var expected []swarm.Secret
	err := json.Unmarshal([]byte(jsonSecrets), &expected)
	if err != nil {
		t.Fatal(err)
	}
	client := newTestClient(&FakeRoundTripper{message: jsonSecrets, status: http.StatusOK})
	secrets, err := client.ListSecrets(ListSecretsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(secrets, expected) {
		t.Errorf("ListSecrets: Expected %#v. Got %#v.", expected, secrets)
	}
}

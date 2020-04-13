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

func TestCreateConfig(t *testing.T) {
	t.Parallel()
	result := `{
  "Id": "d1c00f91353ab0fe368363fab76d124cc764f2db8e11832f89f5ce21c2ece675"
}`
	var expected swarm.Config
	err := json.Unmarshal([]byte(result), &expected)
	if err != nil {
		t.Fatal(err)
	}
	fakeRT := &FakeRoundTripper{message: result, status: http.StatusOK}
	client := newTestClient(fakeRT)
	opts := CreateConfigOptions{}
	config, err := client.CreateConfig(opts)
	if err != nil {
		t.Fatal(err)
	}
	id := "d1c00f91353ab0fe368363fab76d124cc764f2db8e11832f89f5ce21c2ece675"
	if config.ID != id {
		t.Errorf("CreateConfig: wrong ID. Want %q. Got %q.", id, config.ID)
	}
	req := fakeRT.requests[0]
	if req.Method != http.MethodPost {
		t.Errorf("CreateConfig: wrong HTTP method. Want %q. Got %q.", http.MethodPost, req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/configs/create"))
	if gotPath := req.URL.Path; gotPath != expectedURL.Path {
		t.Errorf("CreateConfig: Wrong path in request. Want %q. Got %q.", expectedURL.Path, gotPath)
	}
	var gotBody Config
	err = json.NewDecoder(req.Body).Decode(&gotBody)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRemoveConfig(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	id := "d1c00f91353ab0fe368363fab76d124cc764f2db8e11832f89f5ce21c2ece675"
	opts := RemoveConfigOptions{ID: id}
	err := client.RemoveConfig(opts)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	if req.Method != http.MethodDelete {
		t.Errorf("RemoveConfig(%q): wrong HTTP method. Want %q. Got %q.", id, http.MethodDelete, req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/configs/" + id))
	if gotPath := req.URL.Path; gotPath != expectedURL.Path {
		t.Errorf("RemoveConfig(%q): Wrong path in request. Want %q. Got %q.", id, expectedURL.Path, gotPath)
	}
}

func TestRemoveConfigNotFound(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "no such config", status: http.StatusNotFound})
	err := client.RemoveConfig(RemoveConfigOptions{ID: "a2334"})
	expectNoSuchConfig(t, "a2334", err)
}

func TestUpdateConfig(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	id := "d1c00f91353ab0fe368363fab76d124cc764f2db8e11832f89f5ce21c2ece675"
	update := UpdateConfigOptions{Version: 23}
	err := client.UpdateConfig(id, update)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	if req.Method != http.MethodPost {
		t.Errorf("UpdateConfig: wrong HTTP method. Want %q. Got %q.", http.MethodPost, req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/configs/" + id + "/update?version=23"))
	if gotURI := req.URL.RequestURI(); gotURI != expectedURL.RequestURI() {
		t.Errorf("UpdateConfig: Wrong path in request. Want %q. Got %q.", expectedURL.RequestURI(), gotURI)
	}
	expectedContentType := "application/json"
	if contentType := req.Header.Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("UpdateConfig: Wrong content-type in request. Want %q. Got %q.", expectedContentType, contentType)
	}
	var out UpdateConfigOptions
	if err := json.NewDecoder(req.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	update.Version = 0
	if !reflect.DeepEqual(out, update) {
		t.Errorf("UpdateConfig: wrong body\ngot  %#v\nwant %#v", out, update)
	}
}

func TestUpdateConfigWithAuthentication(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	id := "d1c00f91353ab0fe368363fab76d124cc764f2db8e11832f89f5ce21c2ece675"
	update := UpdateConfigOptions{Version: 23}
	update.Auth = AuthConfiguration{
		Username: "gopher",
		Password: "gopher123",
		Email:    "gopher@tsuru.io",
	}

	err := client.UpdateConfig(id, update)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	if req.Method != http.MethodPost {
		t.Errorf("UpdateConfig: wrong HTTP method. Want %q. Got %q.", http.MethodPost, req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/configs/" + id + "/update?version=23"))
	if gotURI := req.URL.RequestURI(); gotURI != expectedURL.RequestURI() {
		t.Errorf("UpdateConfig: Wrong path in request. Want %q. Got %q.", expectedURL.RequestURI(), gotURI)
	}
	expectedContentType := "application/json"
	if contentType := req.Header.Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("UpdateConfig: Wrong content-type in request. Want %q. Got %q.", expectedContentType, contentType)
	}
	var out UpdateConfigOptions
	if err := json.NewDecoder(req.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	var updateAuth AuthConfiguration

	auth, err := base64.URLEncoding.DecodeString(req.Header.Get("X-Registry-Auth"))
	if err != nil {
		t.Errorf("UpdateConfig: caught error decoding auth. %#v", err.Error())
	}

	err = json.Unmarshal(auth, &updateAuth)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(updateAuth, update.Auth) {
		t.Errorf("UpdateConfig: wrong auth configuration. Want %#v. Got %#v", update.Auth, updateAuth)
	}
}

func TestUpdateConfigNotFound(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "no such Config", status: http.StatusNotFound})
	update := UpdateConfigOptions{}
	err := client.UpdateConfig("notfound", update)
	expectNoSuchConfig(t, "notfound", err)
}

func TestInspectConfigNotFound(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "no such config", status: http.StatusNotFound})
	config, err := client.InspectConfig("notfound")
	if config != nil {
		t.Errorf("InspectConfig: Expected <nil> Config, got %#v", config)
	}
	expectNoSuchConfig(t, "notfound", err)
}

func TestInspectConfig(t *testing.T) {
	t.Parallel()
	jsonConfig := `{
		"ID": "ktnbjxoalbkvbvedmg1urrz8h",
		"Version": {
			"Index": 11
		},
		"CreatedAt": "2016-11-05T01:20:17.327670065Z",
		"UpdatedAt": "2016-11-05T01:20:17.327670065Z",
		"Spec": {
			"Name": "app-dev.crt"
		}
}`
	var expected swarm.Config
	err := json.Unmarshal([]byte(jsonConfig), &expected)
	if err != nil {
		t.Fatal(err)
	}
	fakeRT := &FakeRoundTripper{message: jsonConfig, status: http.StatusOK}
	client := newTestClient(fakeRT)
	id := "ktnbjxoalbkvbvedmg1urrz8h"
	config, err := client.InspectConfig(id)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*config, expected) {
		t.Errorf("InspectConfig(%q): Expected %#v. Got %#v.", id, expected, config)
	}
	expectedURL, _ := url.Parse(client.getURL("/configs/ktnbjxoalbkvbvedmg1urrz8h"))
	if gotPath := fakeRT.requests[0].URL.Path; gotPath != expectedURL.Path {
		t.Errorf("InspectConfig(%q): Wrong path in request. Want %q. Got %q.", id, expectedURL.Path, gotPath)
	}
}

func TestListConfigs(t *testing.T) {
	t.Parallel()
	jsonConfigs := `[
		{
			"ID": "ktnbjxoalbkvbvedmg1urrz8h",
			"Version": {
				"Index": 11
			},
			"CreatedAt": "2016-11-05T01:20:17.327670065Z",
			"UpdatedAt": "2016-11-05T01:20:17.327670065Z",
			"Spec": {
				"Name": "server.conf"
			}
		}
]`
	var expected []swarm.Config
	err := json.Unmarshal([]byte(jsonConfigs), &expected)
	if err != nil {
		t.Fatal(err)
	}
	client := newTestClient(&FakeRoundTripper{message: jsonConfigs, status: http.StatusOK})
	configs, err := client.ListConfigs(ListConfigsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(configs, expected) {
		t.Errorf("ListConfigs: Expected %#v. Got %#v.", expected, configs)
	}
}

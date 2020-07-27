// Copyright 2018 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"
)

var expectPluginDetail = PluginDetail{
	ID:     "5724e2c8652da337ab2eedd19fc6fc0ec908e4bd907c7421bf6a8dfc70c4c078",
	Name:   "tiborvass/sample-volume-plugin",
	Tag:    "latest",
	Active: true,
	Settings: PluginSettings{
		Env:     []string{"DEBUG=0"},
		Args:    nil,
		Devices: nil,
	},
	Config: PluginConfig{
		Description:   "A sample volume plugin for Docker",
		Documentation: "https://docs.docker.com/engine/extend/plugins/",
		Interface: PluginInterface{
			Types:  []string{"docker.volumedriver/1.0"},
			Socket: "plugins.sock",
		},
		Entrypoint: []string{
			"/usr/bin/sample-volume-plugin",
			"/data",
		},
		WorkDir:         "",
		User:            PluginUser{},
		Network:         PluginNetwork{Type: ""},
		Linux:           PluginLinux{Capabilities: nil, AllowAllDevices: false, Devices: nil},
		Mounts:          nil,
		PropagatedMount: "/data",
		Env: []PluginEnv{
			{
				Name:        "DEBUG",
				Description: "If set, prints debug messages",
				Settable:    nil,
				Value:       "0",
			},
		},
		Args: PluginArgs{
			Name:        "args",
			Description: "command line arguments",
			Settable:    nil,
			Value:       []string{},
		},
	},
}

const jsonPluginDetail = `{
    "Id": "5724e2c8652da337ab2eedd19fc6fc0ec908e4bd907c7421bf6a8dfc70c4c078",
    "Name": "tiborvass/sample-volume-plugin",
    "Tag": "latest",
    "Enabled": true,
    "Settings": {
      "Env": [
        "DEBUG=0"
      ],
      "Args": null,
      "Devices": null
    },
    "Config": {
      "Description": "A sample volume plugin for Docker",
      "Documentation": "https://docs.docker.com/engine/extend/plugins/",
      "Interface": {
        "Types": [
          "docker.volumedriver/1.0"
        ],
        "Socket": "plugins.sock"
      },
      "Entrypoint": [
        "/usr/bin/sample-volume-plugin",
        "/data"
      ],
      "WorkDir": "",
      "User": {},
      "Network": {
        "Type": ""
      },
      "Linux": {
        "Capabilities": null,
        "AllowAllDevices": false,
        "Devices": null
      },
      "Mounts": null,
      "PropagatedMount": "/data",
      "Env": [
        {
          "Name": "DEBUG",
          "Description": "If set, prints debug messages",
          "Settable": null,
          "Value": "0"
        }
      ],
      "Args": {
        "Name": "args",
        "Description": "command line arguments",
        "Settable": null,
        "Value": []
      }
    }
  }`

func TestListPlugins(t *testing.T) {
	t.Parallel()
	jsonPlugins := fmt.Sprintf("[%s]", jsonPluginDetail)
	var expected []PluginDetail
	err := json.Unmarshal([]byte(jsonPlugins), &expected)
	if err != nil {
		t.Fatal(err)
	}
	client := newTestClient(&FakeRoundTripper{message: jsonPlugins, status: http.StatusOK})
	pluginDetails, err := client.ListPlugins(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pluginDetails, expected) {
		t.Errorf("ListPlugins: Expected %#v. Got %#v.", expected, pluginDetails)
	}
}

func TestListFilteredPlugins(t *testing.T) {
	t.Parallel()
	jsonPlugins := fmt.Sprintf("[%s]", jsonPluginDetail)
	var expected []PluginDetail
	err := json.Unmarshal([]byte(jsonPlugins), &expected)
	if err != nil {
		t.Fatal(err)
	}
	client := newTestClient(&FakeRoundTripper{message: jsonPlugins, status: http.StatusOK})

	pluginDetails, err := client.ListFilteredPlugins(
		ListFilteredPluginsOptions{
			Filters: map[string][]string{
				"capability": {"volumedriver"},
				"enabled":    {"true"},
			},
			Context: context.Background(),
		})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pluginDetails, expected) {
		t.Errorf("ListPlugins: Expected %#v. Got %#v.", expected, pluginDetails)
	}
}

func TestListFilteredPluginsFailure(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status  int
		message string
	}{
		{400, "bad parameter"},
		{500, "internal server error"},
	}
	for _, tt := range tests {
		client := newTestClient(&FakeRoundTripper{message: tt.message, status: tt.status})
		expected := Error{Status: tt.status, Message: tt.message}
		pluginDetails, err := client.ListFilteredPlugins(ListFilteredPluginsOptions{})
		if !reflect.DeepEqual(expected, *err.(*Error)) {
			t.Errorf("Wrong error in ListFilteredPlugins. Want %#v. Got %#v.", expected, err)
		}
		if len(pluginDetails) > 0 {
			t.Errorf("ListFilteredPlugins failure. Expected empty list. Got %#v.", pluginDetails)
		}
	}
}

func TestGetPluginPrivileges(t *testing.T) {
	t.Parallel()
	name := "test_plugin"
	jsonPluginPrivileges := `[ { "Name": "network", "Description": "", "Value": [ "host" ] }]`
	fakeRT := &FakeRoundTripper{message: jsonPluginPrivileges, status: http.StatusNoContent}
	client := newTestClient(fakeRT)
	expected := []PluginPrivilege{
		{
			Name:        "network",
			Description: "",
			Value:       []string{"host"},
		},
	}
	pluginPrivileges, err := client.GetPluginPrivileges(name, context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pluginPrivileges, expected) {
		t.Errorf("PluginPrivileges: Expected %#v. Got %#v.", expected, pluginPrivileges)
	}
}

func TestGetPluginPrivilegesWithOptions(t *testing.T) {
	t.Parallel()
	remote := "test_plugin"
	jsonPluginPrivileges := `[ { "Name": "network", "Description": "", "Value": [ "host" ] }]`
	fakeRT := &FakeRoundTripper{message: jsonPluginPrivileges, status: http.StatusNoContent}
	client := newTestClient(fakeRT)
	expected := []PluginPrivilege{
		{
			Name:        "network",
			Description: "",
			Value:       []string{"host"},
		},
	}
	pluginPrivileges, err := client.GetPluginPrivilegesWithOptions(GetPluginPrivilegesOptions{
		Remote:  remote,
		Context: context.Background(),
		Auth:    AuthConfiguration{Username: "XY"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pluginPrivileges, expected) {
		t.Errorf("PluginPrivileges: Expected %#v. Got %#v.", expected, pluginPrivileges)
	}
	req := fakeRT.requests[0]
	authHeader := req.Header.Get("X-Registry-Auth")
	if authHeader == "" {
		t.Errorf("InstallImage: unexpected empty X-Registry-Auth header: %v", authHeader)
	}
}

func TestInstallPlugins(t *testing.T) {
	opts := InstallPluginOptions{
		Remote: "", Name: "test",
		Plugins: []PluginPrivilege{
			{
				Name:        "network",
				Description: "",
				Value:       []string{"host"},
			},
		},
		Context: context.Background(),
		Auth:    AuthConfiguration{Username: "XY"},
	}

	fakeRT := &FakeRoundTripper{message: "", status: http.StatusOK}
	client := newTestClient(fakeRT)
	err := client.InstallPlugins(opts)
	if err != nil {
		t.Fatal(err)
	}

	req := fakeRT.requests[0]
	authHeader := req.Header.Get("X-Registry-Auth")
	if authHeader == "" {
		t.Errorf("InstallImage: unexpected empty X-Registry-Auth header: %v", authHeader)
	}
}

func TestInspectPlugin(t *testing.T) {
	name := "test_plugin"
	fakeRT := &FakeRoundTripper{message: jsonPluginDetail, status: http.StatusNoContent}
	client := newTestClient(fakeRT)
	pluginPrivileges, err := client.InspectPlugins(name, context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pluginPrivileges, &expectPluginDetail) {
		t.Errorf("InspectPlugins: Expected %#v. Got %#v.", &expectPluginDetail, pluginPrivileges)
	}
}

func TestRemovePlugin(t *testing.T) {
	opts := RemovePluginOptions{
		Name:    "test_plugin",
		Force:   false,
		Context: context.Background(),
	}
	fakeRT := &FakeRoundTripper{message: jsonPluginDetail, status: http.StatusNoContent}
	client := newTestClient(fakeRT)
	pluginPrivileges, err := client.RemovePlugin(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pluginPrivileges, &expectPluginDetail) {
		t.Errorf("RemovePlugin: Expected %#v. Got %#v.", &expectPluginDetail, pluginPrivileges)
	}
}

func TestRemovePluginNoResponse(t *testing.T) {
	opts := RemovePluginOptions{
		Name:    "test_plugin",
		Force:   false,
		Context: context.Background(),
	}
	fakeRT := &FakeRoundTripper{message: "", status: http.StatusNoContent}
	client := newTestClient(fakeRT)
	plugindetails, err := client.RemovePlugin(opts)
	if err != nil {
		t.Fatal(err)
	}

	if plugindetails != nil {
		t.Errorf("RemovePlugin: Expected %#v. Got %#v.", nil, plugindetails)
	}
}

func TestEnablePlugin(t *testing.T) {
	opts := EnablePluginOptions{
		Name:    "test",
		Timeout: 5,
		Context: context.Background(),
	}
	client := newTestClient(&FakeRoundTripper{message: "", status: http.StatusOK})
	err := client.EnablePlugin(opts)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDisablePlugin(t *testing.T) {
	opts := DisablePluginOptions{
		Name:    "test",
		Context: context.Background(),
	}
	client := newTestClient(&FakeRoundTripper{message: "", status: http.StatusOK})
	err := client.DisablePlugin(opts)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreatePlugin(t *testing.T) {
	opts := CreatePluginOptions{
		Name:    "test",
		Path:    "",
		Context: context.Background(),
	}
	client := newTestClient(&FakeRoundTripper{message: "", status: http.StatusOK})
	_, err := client.CreatePlugin(opts)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPushPlugin(t *testing.T) {
	opts := PushPluginOptions{
		Name:    "test",
		Context: context.Background(),
	}
	client := newTestClient(&FakeRoundTripper{message: "", status: http.StatusOK})
	err := client.PushPlugin(opts)
	if err != nil {
		t.Fatal(err)
	}
}

func TestConfigurePlugin(t *testing.T) {
	opts := ConfigurePluginOptions{
		Name:    "test",
		Envs:    []string{},
		Context: context.Background(),
	}
	client := newTestClient(&FakeRoundTripper{message: "", status: http.StatusOK})
	err := client.ConfigurePlugin(opts)
	if err != nil {
		t.Fatal(err)
	}
}

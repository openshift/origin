// Copyright 2015 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"
)

func TestAuthConfigurationSearchPath(t *testing.T) {
	t.Parallel()
	testData := []struct {
		dockerConfigEnv string
		homeEnv         string
		expectedPaths   []string
	}{
		{"", "", []string{}},
		{"", "home", []string{path.Join("home", ".docker", "plaintext-passwords.json"), path.Join("home", ".docker", "config.json"), path.Join("home", ".dockercfg")}},
		{"docker_config", "", []string{path.Join("docker_config", "plaintext-passwords.json"), path.Join("docker_config", "config.json")}},
		{"a", "b", []string{path.Join("a", "plaintext-passwords.json"), path.Join("a", "config.json")}},
	}
	for _, tt := range testData {
		tt := tt
		t.Run(tt.dockerConfigEnv+tt.homeEnv, func(t *testing.T) {
			t.Parallel()
			paths := cfgPaths(tt.dockerConfigEnv, tt.homeEnv)
			if got, want := strings.Join(paths, ","), strings.Join(tt.expectedPaths, ","); got != want {
				t.Errorf("cfgPaths: wrong result. Want: %s. Got: %s", want, got)
			}
		})
	}
}

func TestAuthConfigurationsFromFile(t *testing.T) {
	t.Parallel()
	tmpDir, err := ioutil.TempDir("", "go-dockerclient-auth-test")
	if err != nil {
		t.Fatalf("Unable to create temporary directory for TestAuthConfigurationsFromFile: %s", err)
	}
	defer os.RemoveAll(tmpDir)
	authString := base64.StdEncoding.EncodeToString([]byte("user:pass"))
	content := fmt.Sprintf(`{"auths":{"foo": {"auth": "%s"}}}`, authString)
	configFile := path.Join(tmpDir, "docker_config")
	if err = ioutil.WriteFile(configFile, []byte(content), 0o600); err != nil {
		t.Errorf("Error writing auth config for TestAuthConfigurationsFromFile: %s", err)
	}
	auths, err := NewAuthConfigurationsFromFile(configFile)
	if err != nil {
		t.Errorf("Error calling NewAuthConfigurationsFromFile: %s", err)
	}
	if _, hasKey := auths.Configs["foo"]; !hasKey {
		t.Errorf("Returned auths did not include expected auth key foo")
	}
}

func TestAuthConfigurationsFromDockerCfg(t *testing.T) {
	t.Parallel()
	tmpDir, err := ioutil.TempDir("", "go-dockerclient-auth-dockercfg-test")
	if err != nil {
		t.Fatalf("Unable to create temporary directory for TestAuthConfigurationsFromDockerCfg: %s", err)
	}
	defer os.RemoveAll(tmpDir)

	keys := []string{
		"docker.io",
		"us.gcr.io",
	}
	pathsToTry := []string{"some/unknown/path"}
	for i, key := range keys {
		authString := base64.StdEncoding.EncodeToString([]byte("user:pass"))
		content := fmt.Sprintf(`{"auths":{"%s": {"auth": "%s"}}}`, key, authString)
		configFile := path.Join(tmpDir, fmt.Sprintf("docker_config_%d.json", i))
		if err = ioutil.WriteFile(configFile, []byte(content), 0o600); err != nil {
			t.Errorf("Error writing auth config for TestAuthConfigurationsFromFile: %s", err)
		}
		pathsToTry = append(pathsToTry, configFile)
	}
	auths, err := newAuthConfigurationsFromDockerCfg(pathsToTry)
	if err != nil {
		t.Errorf("Error calling NewAuthConfigurationsFromFile: %s", err)
	}

	for _, key := range keys {
		if _, hasKey := auths.Configs[key]; !hasKey {
			t.Errorf("Returned auths did not include expected auth key %q", key)
		}
	}
}

func TestAuthConfigurationsFromDockerCfgError(t *testing.T) {
	auths, err := newAuthConfigurationsFromDockerCfg([]string{"this/doesnt/exist.json"})
	if err == nil {
		t.Fatalf("unexpected <nil> error, returned auth config: %#v", auths)
	}
}

func TestAuthLegacyConfig(t *testing.T) {
	t.Parallel()
	auth := base64.StdEncoding.EncodeToString([]byte("user:pa:ss"))
	read := strings.NewReader(fmt.Sprintf(`{"docker.io":{"auth":"%s","email":"user@example.com"}}`, auth))
	ac, err := NewAuthConfigurations(read)
	if err != nil {
		t.Error(err)
	}
	c, ok := ac.Configs["docker.io"]
	if !ok {
		t.Error("NewAuthConfigurations: Expected Configs to contain docker.io")
	}
	if got, want := c.Email, "user@example.com"; got != want {
		t.Errorf(`AuthConfigurations.Configs["docker.io"].Email: wrong result. Want %q. Got %q`, want, got)
	}
	if got, want := c.Username, "user"; got != want {
		t.Errorf(`AuthConfigurations.Configs["docker.io"].Username: wrong result. Want %q. Got %q`, want, got)
	}
	if got, want := c.Password, "pa:ss"; got != want {
		t.Errorf(`AuthConfigurations.Configs["docker.io"].Password: wrong result. Want %q. Got %q`, want, got)
	}
	if got, want := c.ServerAddress, "docker.io"; got != want {
		t.Errorf(`AuthConfigurations.Configs["docker.io"].ServerAddress: wrong result. Want %q. Got %q`, want, got)
	}
}

func TestAuthBadConfig(t *testing.T) {
	t.Parallel()
	auth := base64.StdEncoding.EncodeToString([]byte("userpass"))
	read := strings.NewReader(fmt.Sprintf(`{"docker.io":{"auth":"%s","email":"user@example.com"}}`, auth))
	ac, err := NewAuthConfigurations(read)
	if !errors.Is(err, ErrCannotParseDockercfg) {
		t.Errorf("Incorrect error returned %v\n", err)
	}
	if ac != nil {
		t.Errorf("Invalid auth configuration returned, should be nil %v\n", ac)
	}
}

func TestAuthMixedWithKeyChain(t *testing.T) {
	t.Parallel()
	auth := base64.StdEncoding.EncodeToString([]byte("user:pass"))
	read := strings.NewReader(fmt.Sprintf(`{"auths":{"docker.io":{},"localhost:5000":{"auth":"%s"}},"credsStore":"osxkeychain"}`, auth))
	ac, err := NewAuthConfigurations(read)
	if err != nil {
		t.Fatal(err)
	}
	c, ok := ac.Configs["localhost:5000"]
	if !ok {
		t.Error("NewAuthConfigurations: Expected Configs to contain localhost:5000")
	}
	if got, want := c.Username, "user"; got != want {
		t.Errorf(`AuthConfigurations.Configs["docker.io"].Username: wrong result. Want %q. Got %q`, want, got)
	}
	if got, want := c.Password, "pass"; got != want {
		t.Errorf(`AuthConfigurations.Configs["docker.io"].Password: wrong result. Want %q. Got %q`, want, got)
	}
	if got, want := c.ServerAddress, "localhost:5000"; got != want {
		t.Errorf(`AuthConfigurations.Configs["localhost:5000"].ServerAddress: wrong result. Want %q. Got %q`, want, got)
	}
}

func TestAuthAndOtherFields(t *testing.T) {
	t.Parallel()
	auth := base64.StdEncoding.EncodeToString([]byte("user:pass"))
	read := strings.NewReader(fmt.Sprintf(`{
		"auths":{"docker.io":{"auth":"%s","email":"user@example.com"}},
		"detachKeys": "ctrl-e,e",
		"HttpHeaders": { "MyHeader": "MyValue" }}`, auth))

	ac, err := NewAuthConfigurations(read)
	if err != nil {
		t.Error(err)
	}
	c, ok := ac.Configs["docker.io"]
	if !ok {
		t.Error("NewAuthConfigurations: Expected Configs to contain docker.io")
	}
	if got, want := c.Email, "user@example.com"; got != want {
		t.Errorf(`AuthConfigurations.Configs["docker.io"].Email: wrong result. Want %q. Got %q`, want, got)
	}
	if got, want := c.Username, "user"; got != want {
		t.Errorf(`AuthConfigurations.Configs["docker.io"].Username: wrong result. Want %q. Got %q`, want, got)
	}
	if got, want := c.Password, "pass"; got != want {
		t.Errorf(`AuthConfigurations.Configs["docker.io"].Password: wrong result. Want %q. Got %q`, want, got)
	}
	if got, want := c.ServerAddress, "docker.io"; got != want {
		t.Errorf(`AuthConfigurations.Configs["docker.io"].ServerAddress: wrong result. Want %q. Got %q`, want, got)
	}
}

func TestAuthConfig(t *testing.T) {
	t.Parallel()
	auth := base64.StdEncoding.EncodeToString([]byte("user:pass"))
	read := strings.NewReader(fmt.Sprintf(`{"auths":{"docker.io":{"auth":"%s","email":"user@example.com"}}}`, auth))
	ac, err := NewAuthConfigurations(read)
	if err != nil {
		t.Error(err)
	}
	c, ok := ac.Configs["docker.io"]
	if !ok {
		t.Error("NewAuthConfigurations: Expected Configs to contain docker.io")
	}
	if got, want := c.Email, "user@example.com"; got != want {
		t.Errorf(`AuthConfigurations.Configs["docker.io"].Email: wrong result. Want %q. Got %q`, want, got)
	}
	if got, want := c.Username, "user"; got != want {
		t.Errorf(`AuthConfigurations.Configs["docker.io"].Username: wrong result. Want %q. Got %q`, want, got)
	}
	if got, want := c.Password, "pass"; got != want {
		t.Errorf(`AuthConfigurations.Configs["docker.io"].Password: wrong result. Want %q. Got %q`, want, got)
	}
	if got, want := c.ServerAddress, "docker.io"; got != want {
		t.Errorf(`AuthConfigurations.Configs["docker.io"].ServerAddress: wrong result. Want %q. Got %q`, want, got)
	}
}

func TestAuthConfigIdentityToken(t *testing.T) {
	t.Parallel()
	auth := base64.StdEncoding.EncodeToString([]byte("someuser:"))
	read := strings.NewReader(fmt.Sprintf(`{"auths":{"docker.io":{"auth":"%s","identitytoken":"sometoken"}}}`, auth))
	ac, err := NewAuthConfigurations(read)
	if err != nil {
		t.Fatal(err)
	}

	c, ok := ac.Configs["docker.io"]
	if !ok {
		t.Error("NewAuthConfigurations: Expected Configs to contain docker.io")
	}
	if got, want := c.Username, "someuser"; got != want {
		t.Errorf(`AuthConfigurations.Configs["docker.io"].Username: wrong result. Want %q. Got %q`, want, got)
	}
	if got, want := c.IdentityToken, "sometoken"; got != want {
		t.Errorf(`AuthConfigurations.Configs["docker.io"].IdentityToken: wrong result. Want %q. Got %q`, want, got)
	}
}

func TestAuthConfigRegistryToken(t *testing.T) {
	t.Parallel()
	auth := base64.StdEncoding.EncodeToString([]byte("someuser:"))
	read := strings.NewReader(fmt.Sprintf(`{"auths":{"docker.io":{"auth":"%s","registrytoken":"sometoken"}}}`, auth))
	ac, err := NewAuthConfigurations(read)
	if err != nil {
		t.Fatal(err)
	}

	c, ok := ac.Configs["docker.io"]
	if !ok {
		t.Error("NewAuthConfigurations: Expected Configs to contain docker.io")
	}
	if got, want := c.Username, "someuser"; got != want {
		t.Errorf(`AuthConfigurations.Configs["docker.io"].Username: wrong result. Want %q. Got %q`, want, got)
	}
	if got, want := c.RegistryToken, "sometoken"; got != want {
		t.Errorf(`AuthConfigurations.Configs["docker.io"].RegistryToken: wrong result. Want %q. Got %q`, want, got)
	}
}

func TestAuthCheck(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{status: http.StatusOK}
	client := newTestClient(fakeRT)
	if _, err := client.AuthCheck(nil); err == nil {
		t.Fatalf("expected error on nil auth config")
	}
	// test good auth
	if _, err := client.AuthCheck(&AuthConfiguration{}); err != nil {
		t.Fatal(err)
	}
	*fakeRT = FakeRoundTripper{status: http.StatusUnauthorized}
	if _, err := client.AuthCheck(&AuthConfiguration{}); err == nil {
		t.Fatal("expected failure from unauthorized auth")
	}
}

func TestAuthConfigurationsMerge(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		left     AuthConfigurations
		right    AuthConfigurations
		expected AuthConfigurations
	}{
		{
			name:     "empty configs",
			expected: AuthConfigurations{},
		},
		{
			name: "empty left config",
			right: AuthConfigurations{
				Configs: map[string]AuthConfiguration{
					"docker.io": {Email: "user@example.com"},
				},
			},
			expected: AuthConfigurations{
				Configs: map[string]AuthConfiguration{
					"docker.io": {Email: "user@example.com"},
				},
			},
		},
		{
			name: "empty right config",
			left: AuthConfigurations{
				Configs: map[string]AuthConfiguration{
					"docker.io": {Email: "user@example.com"},
				},
			},
			expected: AuthConfigurations{
				Configs: map[string]AuthConfiguration{
					"docker.io": {Email: "user@example.com"},
				},
			},
		},
		{
			name: "no conflicts",
			left: AuthConfigurations{
				Configs: map[string]AuthConfiguration{
					"docker.io": {Email: "user@example.com"},
				},
			},
			right: AuthConfigurations{
				Configs: map[string]AuthConfiguration{
					"us.gcr.io": {Email: "user@google.com"},
				},
			},
			expected: AuthConfigurations{
				Configs: map[string]AuthConfiguration{
					"docker.io": {Email: "user@example.com"},
					"us.gcr.io": {Email: "user@google.com"},
				},
			},
		},
		{
			name: "no conflicts",
			left: AuthConfigurations{
				Configs: map[string]AuthConfiguration{
					"docker.io": {Email: "user@example.com"},
					"us.gcr.io": {Email: "google-user@example.com"},
				},
			},
			right: AuthConfigurations{
				Configs: map[string]AuthConfiguration{
					"us.gcr.io": {Email: "user@google.com"},
				},
			},
			expected: AuthConfigurations{
				Configs: map[string]AuthConfiguration{
					"docker.io": {Email: "user@example.com"},
					"us.gcr.io": {Email: "google-user@example.com"},
				},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			test.left.merge(test.right)

			if !reflect.DeepEqual(test.left, test.expected) {
				t.Errorf("wrong configuration map after merge\nwant %#v\ngot  %#v", test.expected, test.left)
			}
		})
	}
}

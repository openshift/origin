package dockercredentials

import (
	"net/url"
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/credentialprovider"

	_ "github.com/openshift/origin/pkg/api/install"
)

type mockKeyring struct {
	calls []string
}

func (k *mockKeyring) Lookup(image string) ([]credentialprovider.LazyAuthConfiguration, bool) {
	k.calls = append(k.calls, image)
	return nil, false
}

func TestHubFallback(t *testing.T) {
	k := &mockKeyring{}
	BasicFromKeyring(k, &url.URL{Host: "auth.docker.io", Path: "/token"})
	if !reflect.DeepEqual([]string{"auth.docker.io/token", "index.docker.io", "docker.io"}, k.calls) {
		t.Errorf("unexpected calls: %v", k.calls)
	}
}

func Test_BasicFromKeyring(t *testing.T) {
	fn := func(host string, entry credentialprovider.DockerConfigEntry) credentialprovider.DockerKeyring {
		k := &credentialprovider.BasicDockerKeyring{}
		k.Add(map[string]credentialprovider.DockerConfigEntry{host: entry})
		return k
	}
	def := credentialprovider.DockerConfigEntry{
		Username: "local_user",
		Password: "local_pass",
	}
	type args struct {
		keyring credentialprovider.DockerKeyring
		target  *url.URL
	}
	tests := []struct {
		name     string
		args     args
		user     string
		password string
	}{
		{name: "exact", args: args{keyring: fn("localhost", def), target: &url.URL{Host: "localhost"}}, user: def.Username, password: def.Password},
		{name: "https scheme", args: args{keyring: fn("localhost", def), target: &url.URL{Scheme: "https", Host: "localhost"}}, user: def.Username, password: def.Password},
		{name: "canonical https", args: args{keyring: fn("localhost", def), target: &url.URL{Scheme: "https", Host: "localhost:443"}}, user: def.Username, password: def.Password},
		{name: "only https", args: args{keyring: fn("https://localhost", def), target: &url.URL{Host: "localhost"}}, user: def.Username, password: def.Password},
		{name: "only https scheme", args: args{keyring: fn("https://localhost", def), target: &url.URL{Scheme: "https", Host: "localhost"}}, user: def.Username, password: def.Password},

		{name: "mismatched scheme - http", args: args{keyring: fn("http://localhost", def), target: &url.URL{Scheme: "https", Host: "localhost"}}, user: def.Username, password: def.Password},
		{name: "don't assume port 80 in keyring is https", args: args{keyring: fn("localhost:80", def), target: &url.URL{Scheme: "http", Host: "localhost"}}, user: def.Username, password: def.Password},
		{name: "exact http", args: args{keyring: fn("localhost:80", def), target: &url.URL{Scheme: "http", Host: "localhost:80"}}, user: def.Username, password: def.Password},

		// this is not allowed by the credential keyring, but should be
		{name: "exact http", args: args{keyring: fn("http://localhost", def), target: &url.URL{Scheme: "http", Host: "localhost:80"}}, user: "", password: ""},
		{name: "keyring canonical https", args: args{keyring: fn("localhost:443", def), target: &url.URL{Scheme: "https", Host: "localhost"}}, user: "", password: ""},

		// these should not be allowed
		{name: "host is for port 80 only", args: args{keyring: fn("localhost:80", def), target: &url.URL{Host: "localhost"}}, user: "", password: ""},
		{name: "host is for port 443 only", args: args{keyring: fn("localhost:443", def), target: &url.URL{Host: "localhost"}}, user: "", password: ""},
		{name: "canonical http", args: args{keyring: fn("localhost", def), target: &url.URL{Scheme: "http", Host: "localhost:80"}}, user: "", password: ""},
		{name: "http scheme", args: args{keyring: fn("localhost", def), target: &url.URL{Scheme: "http", Host: "localhost"}}, user: "", password: ""},
		{name: "https not canonical", args: args{keyring: fn("localhost", def), target: &url.URL{Scheme: "https", Host: "localhost:80"}}, user: "", password: ""},
		{name: "http not canonical", args: args{keyring: fn("localhost", def), target: &url.URL{Scheme: "http", Host: "localhost:443"}}, user: "", password: ""},
		{name: "mismatched scheme", args: args{keyring: fn("https://localhost", def), target: &url.URL{Scheme: "http", Host: "localhost"}}, user: "", password: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, password := BasicFromKeyring(tt.args.keyring, tt.args.target)
			if user != tt.user {
				t.Errorf("BasicFromKeyring() user = %v, actual = %v", user, tt.user)
			}
			if password != tt.password {
				t.Errorf("BasicFromKeyring() password = %v, actual = %v", password, tt.password)
			}
		})
	}
}

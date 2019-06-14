package importer

import (
	"io/ioutil"
	"net/url"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapiv1 "k8s.io/kubernetes/pkg/apis/core/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"

	_ "github.com/openshift/origin/pkg/api/install"
)

func TestCredentialsForSecrets(t *testing.T) {
	data, err := ioutil.ReadFile("../../../test/testdata/image-secrets.json")
	if err != nil {
		t.Fatal(err)
	}
	obj, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(), data)
	if err != nil {
		t.Fatal(err)
	}
	secrets := obj.(*kapi.SecretList)
	secretsv1 := make([]corev1.Secret, len(secrets.Items))
	for i, secret := range secrets.Items {
		err := kapiv1.Convert_core_Secret_To_v1_Secret(&secret, &secretsv1[i], nil)
		if err != nil {
			t.Logf("Unable to make the Docker keyring for %s/%s secret: %v", secret.Name, secret.Namespace, err)
			continue
		}
	}
	store := NewCredentialsForSecrets(secretsv1)
	user, pass := store.Basic(&url.URL{Scheme: "https", Host: "172.30.213.112:5000"})
	if user != "serviceaccount" || len(pass) == 0 {
		t.Errorf("unexpected username and password: %s %s", user, pass)
	}
}

type mockKeyring struct {
	calls []string
}

func (k *mockKeyring) Lookup(image string) ([]credentialprovider.LazyAuthConfiguration, bool) {
	k.calls = append(k.calls, image)
	return nil, false
}

func TestHubFallback(t *testing.T) {
	k := &mockKeyring{}
	basicCredentialsFromKeyring(k, &url.URL{Host: "auth.docker.io", Path: "/token"})
	if !reflect.DeepEqual([]string{"auth.docker.io/token", "index.docker.io", "docker.io"}, k.calls) {
		t.Errorf("unexpected calls: %v", k.calls)
	}
}

func Test_basicCredentialsFromKeyring(t *testing.T) {
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
			user, password := basicCredentialsFromKeyring(tt.args.keyring, tt.args.target)
			if user != tt.user {
				t.Errorf("basicCredentialsFromKeyring() user = %v, actual = %v", user, tt.user)
			}
			if password != tt.password {
				t.Errorf("basicCredentialsFromKeyring() password = %v, actual = %v", password, tt.password)
			}
		})
	}
}

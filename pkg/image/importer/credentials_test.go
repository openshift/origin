package importer

import (
	"io/ioutil"
	"net/url"
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/runtime"

	_ "github.com/openshift/origin/pkg/api/install"
)

func TestCredentialsForSecrets(t *testing.T) {
	data, err := ioutil.ReadFile("../../../test/testdata/image-secrets.json")
	if err != nil {
		t.Fatal(err)
	}
	obj, err := runtime.Decode(kapi.Codecs.UniversalDecoder(), data)
	if err != nil {
		t.Fatal(err)
	}
	store := NewCredentialsForSecrets(obj.(*kapi.SecretList).Items)
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

func TestBasicCredentials(t *testing.T) {
	creds := NewBasicCredentials()
	creds.Add(&url.URL{Host: "localhost"}, "test", "other")
	if u, p := creds.Basic(&url.URL{Host: "test"}); u != "" || p != "" {
		t.Fatalf("unexpected response: %s %s", u, p)
	}
	if u, p := creds.Basic(&url.URL{Host: "localhost"}); u != "test" || p != "other" {
		t.Fatalf("unexpected response: %s %s", u, p)
	}
	if u, p := creds.Basic(&url.URL{Host: "localhost", Path: "/foo"}); u != "test" || p != "other" {
		t.Fatalf("unexpected response: %s %s", u, p)
	}
}

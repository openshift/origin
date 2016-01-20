package importer

import (
	"io/ioutil"
	"net/url"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	_ "k8s.io/kubernetes/pkg/api/v1"
)

func TestCredentialsForSecrets(t *testing.T) {
	data, err := ioutil.ReadFile("../../../test/fixtures/image-secrets.json")
	if err != nil {
		t.Fatal(err)
	}
	obj, err := kapi.Codec.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	store := NewCredentialsForSecrets(obj.(*kapi.SecretList).Items)
	user, pass := store.Basic(&url.URL{Scheme: "https", Host: "172.30.213.112:5000"})
	if user != "serviceaccount" || len(pass) == 0 {
		t.Errorf("unexpected username and password: %s %s", user, pass)
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

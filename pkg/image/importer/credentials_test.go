package importer

import (
	"io/ioutil"
	"net/url"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/api"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"

	"github.com/golang/glog"
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
	secrets := obj.(*kapi.SecretList)
	secretsv1 := make([]kapiv1.Secret, len(secrets.Items))
	for i, secret := range secrets.Items {
		err := kapiv1.Convert_api_Secret_To_v1_Secret(&secret, &secretsv1[i], nil)
		if err != nil {
			glog.V(2).Infof("Unable to make the Docker keyring for %s/%s secret: %v", secret.Name, secret.Namespace, err)
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

package config

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	kubeclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	clientapi "github.com/openshift/origin/pkg/cmd/client/api"
)

func TestApplyInvalidConfig(t *testing.T) {
	clients := clientapi.ClientMappings{
		"InvalidClientMapping": {"InvalidClientResource", nil, nil},
	}
	invalidConfigs := []string{
		`{}`,
		`{ "foo": "bar" }`,
		`{ "items": null }`,
		`{ "items": "bar" }`,
		`{ "items": [ null ] }`,
		`{ "items": [ { "foo": "bar" } ] }`,
		`{ "items": [ { "kind": "", "apiVersion": "v1beta1" } ] }`,
		`{ "items": [ { "kind": "UnknownResource", "apiVersion": "v1beta1" } ] }`,
		`{ "items": [ { "kind": "InvalidClientResource", "apiVersion": "v1beta1" } ] }`,
	}
	for _, invalidConfig := range invalidConfigs {
		errs := Apply([]byte(invalidConfig), clients)
		if len(errs) == 0 {
			t.Errorf("Expected error while applying invalid Config '%v'", invalidConfig)
		}
	}
}

type FakeResource struct {
	kubeapi.JSONBase `json:",inline" yaml:",inline"`
}

func (*FakeResource) IsAnAPIObject() {}

func TestApplySendsData(t *testing.T) {
	fakeScheme := runtime.NewScheme()
	// TODO: The below should work with "FakeResource" name instead.
	fakeScheme.AddKnownTypeWithName("", "", &FakeResource{})
	fakeScheme.AddKnownTypeWithName("v1beta1", "", &FakeResource{})
	fakeCodec := runtime.CodecFor(fakeScheme, "v1beta1")

	received := make(chan bool, 1)
	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- true
		if r.RequestURI != "/api/v1beta1/FakeMapping" {
			t.Errorf("Unexpected RESTClient RequestURI. Expected: %v, got: %v.", "/api/v1beta1/FakeMapping", r.RequestURI)
		}
	}))

	uri, _ := url.Parse(fakeServer.URL + "/api/v1beta1")
	fakeClient := kubeclient.NewRESTClient(uri, fakeCodec)
	clients := clientapi.ClientMappings{
		"FakeMapping": {"FakeResource", fakeClient, fakeCodec},
	}
	config := `{ "apiVersion": "v1beta1", "items": [ { "kind": "FakeResource", "apiVersion": "v1beta1" } ] }`

	errs := Apply([]byte(config), clients)
	if len(errs) != 0 {
		t.Errorf("Unexpected error while applying valid Config '%v': %v", config, errs)
	}

	<-received
}

func TestGetClientAndPath(t *testing.T) {
	kubeClient, _ := kubeclient.New(&kubeclient.Config{Host: "127.0.0.1"})
	testClientMappings := clientapi.ClientMappings{
		"pods":     {"Pod", kubeClient.RESTClient, klatest.Codec},
		"services": {"Service", kubeClient.RESTClient, klatest.Codec},
	}
	client, path, _ := getClientAndPath("Service", testClientMappings)
	if client != kubeClient.RESTClient {
		t.Errorf("Failed to get client for Service")
	}
	if path != "services" {
		t.Errorf("Failed to get path for Service")
	}
}

func ExampleApply() {
	kubeClient, _ := kubeclient.New(&kubeclient.Config{Host: "127.0.0.1"})
	testClientMappings := clientapi.ClientMappings{
		"pods":     {"Pod", kubeClient.RESTClient, klatest.Codec},
		"services": {"Service", kubeClient.RESTClient, klatest.Codec},
	}
	data, _ := ioutil.ReadFile("config_test.json")
	Apply(data, testClientMappings)
}

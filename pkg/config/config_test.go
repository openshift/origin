package config

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	kubeclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	clientapi "github.com/openshift/origin/pkg/cmd/client/api"
)

func TestParseKindAndItem(t *testing.T) {
	data, _ := ioutil.ReadFile("config_test.json")
	conf := configJSON{}
	if err := json.Unmarshal(data, &conf); err != nil {
		t.Errorf("Failed to parse Config: %v", err)
	}

	kind, itemID, err := parseKindAndID(conf.Items[0])
	if len(err) != 0 {
		t.Errorf("Failed to parse kind and id from the Config item: %v", err)
	}

	if kind != "Service" && itemID != "frontend" {
		t.Errorf("Invalid kind and id, should be Service and frontend: %s, %s", kind, itemID)
	}
}

func TestApply(t *testing.T) {
	clients := clientapi.ClientMappings{}
	invalidData := []byte(`{"items": [ { "foo": "bar" } ]}`)
	errs := Apply(invalidData, clients)
	if len(errs) == 0 {
		t.Errorf("Expected missing kind field for Config item, got %v", errs)
	}
	uErrs := Apply([]byte(`{ "foo": }`), clients)
	if len(uErrs) == 0 {
		t.Errorf("Expected unmarshal error, got nothing")
	}
}

func TestGetClientAndPath(t *testing.T) {
	kubeClient, _ := kubeclient.New("127.0.0.1", "", nil)
	testClientMappings := clientapi.ClientMappings{
		"pods":     {"Pod", kubeClient.RESTClient, klatest.Codec},
		"services": {"Service", kubeClient.RESTClient, klatest.Codec},
	}
	client, path := getClientAndPath("Service", testClientMappings)
	if client != kubeClient.RESTClient {
		t.Errorf("Failed to get client for Service")
	}
	if path != "services" {
		t.Errorf("Failed to get path for Service")
	}
}

func ExampleApply() {
	kubeClient, _ := kubeclient.New("127.0.0.1", "", nil)
	testClientMappings := clientapi.ClientMappings{
		"pods":     {"Pod", kubeClient.RESTClient, klatest.Codec},
		"services": {"Service", kubeClient.RESTClient, klatest.Codec},
	}
	data, _ := ioutil.ReadFile("config_test.json")
	Apply(data, testClientMappings)
}

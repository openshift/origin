package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"testing"

	kubeclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	clientapi "github.com/openshift/origin/pkg/cmd/client/api"
)

func TestParseKindAndItem(t *testing.T) {
	data, _ := ioutil.ReadFile("../../examples/guestbook/config.json")
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
	kubeClient, _ := kubeclient.New("127.0.0.1", nil)
	testClientMappings := clientapi.ClientMappings{
		"pods":     {"Pod", kubeClient.RESTClient},
		"services": {"Service", kubeClient.RESTClient},
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
	kubeClient, _ := kubeclient.New("127.0.0.1", nil)
	testClientMappings := clientapi.ClientMappings{
		"pods":     {"Pod", kubeClient.RESTClient},
		"services": {"Service", kubeClient.RESTClient},
	}
	data, _ := ioutil.ReadFile("../../examples/guestbook/config.json")
	errs := Apply(data, testClientMappings)
	fmt.Println(errs)
	// Output:
	// [[Service#frontend] Failed to create: Post http://127.0.0.1/api/v1beta1/services: dial tcp 127.0.0.1:80: connection refused [Service#redismaster] Failed to create: Post http://127.0.0.1/api/v1beta1/services: dial tcp 127.0.0.1:80: connection refused [Service#redisslave] Failed to create: Post http://127.0.0.1/api/v1beta1/services: dial tcp 127.0.0.1:80: connection refused [Pod#redis-master-2] Failed to create: Post http://127.0.0.1/api/v1beta1/pods: dial tcp 127.0.0.1:80: connection refused The resource ReplicationController is not a known type - unable to create frontendController The resource ReplicationController is not a known type - unable to create redisSlaveController]
}

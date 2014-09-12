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
	invalidData := []byte(`{"items": [ { "foo": "bar" } ]}`)
	invalidConf := configJSON{}
	if err := json.Unmarshal(invalidData, &invalidConf); err != nil {
		t.Errorf("Failed to parse Config: %v", err)
	}
	clients := clientapi.ClientMappings{}
	errs := Apply(invalidData, clients)
	if len(errs) == 0 {
		t.Errorf("Expected missing kind field for Config item, got %v", errs)
	}
	uErrs := Apply([]byte(`{ "foo": }`), clients)
	if len(uErrs) == 0 {
		t.Errorf("Expected unmarshal error, got nothing")
	}
}

func ExampleApply() {
	kubeClient, _ := kubeclient.New("127.0.0.1", nil)
	clients := clientapi.ClientMappings{
		"pods": {
			Kind:   "Pod",
			Client: kubeClient.RESTClient,
		},
		"services": {
			Kind:   "Service",
			Client: kubeClient.RESTClient,
		},
	}
	data, _ := ioutil.ReadFile("../../examples/guestbook/config.json")
	errs := Apply(data, clients)
	fmt.Println(errs)
	// Output:
	// [The resource Service is not a known type - unable to create frontend The resource Service is not a known type - unable to create redismaster The resource Service is not a known type - unable to create redisslave The resource Pod is not a known type - unable to create redis-master-2 The resource ReplicationController is not a known type - unable to create frontendController The resource ReplicationController is not a known type - unable to create redisSlaveController]
	//
}

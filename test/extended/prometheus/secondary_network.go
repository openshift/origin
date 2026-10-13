package prometheus

import (
	"fmt"
	"time"

	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"

	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type dynClientSet struct {
	dc dynamic.Interface
}

func (dcs dynClientSet) Networks() dynamic.ResourceInterface {
	return dcs.dc.Resource(schema.GroupVersionResource{Group: "operator.openshift.io", Resource: "networks", Version: "v1"})
}

func (dcs dynClientSet) NetworkAttachmentDefinitions(namespace string) dynamic.ResourceInterface {
	return dcs.dc.Resource(schema.GroupVersionResource{Group: "k8s.cni.cncf.io", Resource: "network-attachment-definitions", Version: "v1"}).Namespace(namespace)
}

func newDynClientSet() (*dynClientSet, error) {
	cfg, err := e2e.LoadConfig()
	if err != nil {
		return nil, err
	}

	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &dynClientSet{
		dc: dc,
	}, nil
}

func addNetwork(client *dynClientSet, name, namespace string) error {
	clusterNetwork, err := client.Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Failed to get cluster network %v", err)
	}

	nn, found, err := unstructured.NestedSlice(clusterNetwork.Object, "spec", "additionalNetworks")
	if err != nil {
		return fmt.Errorf("Failed to get cluster additional networks %v", err)
	}

	var newAdditionalNetworks []interface{}
	if found {
		newAdditionalNetworks = nn
	} else {
		newAdditionalNetworks = make([]interface{}, 0)
	}

	newAdditionalNetworks = append(newAdditionalNetworks, newMacVlan(name, namespace))

	toUpdate := clusterNetwork.DeepCopy()

	// can't directly use SetNestedSlice because the added network can't be deepcopied
	// with the error `cannot deep copy []map[string]interface {}`
	newSpec, found, err := unstructured.NestedMap(toUpdate.Object, "spec")
	if err != nil || !found {
		return fmt.Errorf("Failed to get spec from cluster network")
	}

	newSpec["additionalNetworks"] = newAdditionalNetworks
	toUpdate.Object["spec"] = newSpec

	_, err = client.Networks().Update(context.Background(), toUpdate, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("Failed to update cluster network %v", err)
	}

	err = waitForNetworkAttachmentDefinition(client, name, namespace)
	if err != nil {
		return fmt.Errorf("Failed waiting for network attachment definition %v", err)
	}
	return nil
}

func removeNetwork(client *dynClientSet, name, namespace string) error {
	clusterNetwork, err := client.Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Failed to get cluster network %v", err)
	}

	nn, found, err := unstructured.NestedSlice(clusterNetwork.Object, "spec", "additionalNetworks")
	if err != nil {
		return fmt.Errorf("Failed to get cluster additional networks %v", err)
	}
	if !found {
		return fmt.Errorf("Failed to fetch additionalNetworks for cluster network")
	}

	for idx, n := range nn {
		network, ok := n.(map[string]interface{})
		if !ok {
			return fmt.Errorf("Failed to convert network to map")
		}
		if network["name"] == name && network["namespace"] == namespace {
			nn = append(nn[:idx], nn[idx+1:]...)
			break
		}
	}

	newClusterNetwork := clusterNetwork.DeepCopy()
	unstructured.SetNestedSlice(newClusterNetwork.Object, nn, "spec", "additionalNetworks")

	_, err = client.Networks().Update(context.Background(), newClusterNetwork, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("Failed to update cluster network %v", err)
	}
	err = waitForNetworkAttachmentDefinitionDeleted(client, name, namespace)
	if err != nil {
		return fmt.Errorf("Failed to waiting for network attachment deletion %v", err)
	}

	return nil
}

func waitForNetworkAttachmentDefinition(client *dynClientSet, name, namespace string) error {
	return wait.Poll(5*time.Second, 2*time.Minute,
		func() (bool, error) {
			_, err := client.NetworkAttachmentDefinitions(namespace).Get(context.Background(), name, metav1.GetOptions{})
			if err != nil && errors.IsNotFound(err) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			return true, nil
		})
}

func waitForNetworkAttachmentDefinitionDeleted(client *dynClientSet, name, namespace string) error {
	return wait.Poll(5*time.Second, 2*time.Minute,
		func() (bool, error) {
			_, err := client.NetworkAttachmentDefinitions(namespace).Get(context.Background(), name, metav1.GetOptions{})
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}
			if err != nil {
				return false, err
			}
			return false, nil
		})
}

func newMacVlan(name, namespace string) interface{} {
	return map[string]interface{}{
		"name":      name,
		"namespace": namespace,
		"type":      "SimpleMacvlan",
		"simpleMacvlanConfig": map[string]interface{}{
			"ipamConfig": map[string]interface{}{
				"type": "static",
				"staticIPAMConfig": map[string]interface{}{
					"addresses": []map[string]interface{}{
						{
							"address": "10.1.1.0/24",
						},
					},
				},
			},
		},
	}
}

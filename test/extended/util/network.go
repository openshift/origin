package util

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

func CreateNetworkAttachmentDefinition(config *rest.Config, namespace string, name string, nadConfig string) error {
	nadClient, err := networkAttachmentDefinitionClient(config)
	if err != nil {
		return err
	}
	networkAttachmentDefintion := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "k8s.cni.cncf.io/v1",
			"kind":       "NetworkAttachmentDefinition",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"config": nadConfig,
			},
		},
	}
	_, err = nadClient.Namespace(namespace).Create(context.TODO(), networkAttachmentDefintion, metav1.CreateOptions{})
	return err
}

func networkAttachmentDefinitionClient(config *rest.Config) (dynamic.NamespaceableResourceInterface, error) {
	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	nadGVR := schema.GroupVersionResource{
		Group:    "k8s.cni.cncf.io",
		Version:  "v1",
		Resource: "network-attachment-definitions",
	}
	nadClient := dynClient.Resource(nadGVR)
	return nadClient, nil
}

package genericoperatorclient

import (
	"time"

	"github.com/imdario/mergo"

	"k8s.io/apimachinery/pkg/runtime"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const globalConfigName = "cluster"

func NewClusterScopedOperatorClient(config *rest.Config, gvr schema.GroupVersionResource) (v1helpers.OperatorClient, dynamicinformer.DynamicSharedInformerFactory, error) {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	client := dynamicClient.Resource(gvr)

	informers := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 12*time.Hour)
	informer := informers.ForResource(gvr)

	return &dynamicOperatorClient{
		informer: informer,
		client:   client,
	}, informers, nil
}

type dynamicOperatorClient struct {
	informer informers.GenericInformer
	client   dynamic.ResourceInterface
}

func (c dynamicOperatorClient) Informer() cache.SharedIndexInformer {
	return c.informer.Informer()
}

func (c dynamicOperatorClient) GetOperatorState() (*operatorv1.OperatorSpec, *operatorv1.OperatorStatus, string, error) {
	uncastInstance, err := c.informer.Lister().Get(globalConfigName)
	if err != nil {
		return nil, nil, "", err
	}
	instance := uncastInstance.(*unstructured.Unstructured)

	spec, err := getOperatorSpecFromUnstructured(instance.UnstructuredContent())
	if err != nil {
		return nil, nil, "", err
	}
	status, err := getOperatorStatusFromUnstructured(instance.UnstructuredContent())
	if err != nil {
		return nil, nil, "", err
	}

	return spec, status, instance.GetResourceVersion(), nil
}

func (c dynamicOperatorClient) UpdateOperatorSpec(resourceVersion string, spec *operatorv1.OperatorSpec) (*operatorv1.OperatorSpec, string, error) {
	uncastOriginal, err := c.informer.Lister().Get(globalConfigName)
	if err != nil {
		return nil, "", err
	}
	original := uncastOriginal.(*unstructured.Unstructured)

	copy := original.DeepCopy()
	copy.SetResourceVersion(resourceVersion)
	if err := setOperatorSpecFromUnstructured(copy.UnstructuredContent(), spec); err != nil {
		return nil, "", err
	}

	ret, err := c.client.Update(copy, metav1.UpdateOptions{})
	if err != nil {
		return nil, "", err
	}
	retSpec, err := getOperatorSpecFromUnstructured(ret.UnstructuredContent())
	if err != nil {
		return nil, "", err
	}

	return retSpec, ret.GetResourceVersion(), nil
}

func (c dynamicOperatorClient) UpdateOperatorStatus(resourceVersion string, status *operatorv1.OperatorStatus) (*operatorv1.OperatorStatus, error) {
	uncastOriginal, err := c.informer.Lister().Get(globalConfigName)
	if err != nil {
		return nil, err
	}
	original := uncastOriginal.(*unstructured.Unstructured)

	copy := original.DeepCopy()
	copy.SetResourceVersion(resourceVersion)
	if err := setOperatorStatusFromUnstructured(copy.UnstructuredContent(), status); err != nil {
		return nil, err
	}

	ret, err := c.client.UpdateStatus(copy, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	retStatus, err := getOperatorStatusFromUnstructured(ret.UnstructuredContent())
	if err != nil {
		return nil, err
	}

	return retStatus, nil
}

func getOperatorSpecFromUnstructured(obj map[string]interface{}) (*operatorv1.OperatorSpec, error) {
	uncastSpec, exists, err := unstructured.NestedMap(obj, "spec")
	if !exists {
		return &operatorv1.OperatorSpec{}, nil
	}
	if err != nil {
		return nil, err
	}

	ret := &operatorv1.OperatorSpec{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(uncastSpec, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func setOperatorSpecFromUnstructured(obj map[string]interface{}, spec *operatorv1.OperatorSpec) error {
	// we cannot simply set the entire map because doing so would stomp unknown fields, like say a static pod operator spec when cast as an operator spec
	newUnstructuredSpec, err := runtime.DefaultUnstructuredConverter.ToUnstructured(spec)
	if err != nil {
		return err
	}

	originalUnstructuredSpec, exists, err := unstructured.NestedMap(obj, "spec")
	if !exists {
		return unstructured.SetNestedMap(obj, newUnstructuredSpec, "spec")
	}
	if err != nil {
		return err
	}
	if err := mergo.Merge(&originalUnstructuredSpec, newUnstructuredSpec, mergo.WithOverride); err != nil {
		return err
	}

	return unstructured.SetNestedMap(obj, originalUnstructuredSpec, "spec")
}

func getOperatorStatusFromUnstructured(obj map[string]interface{}) (*operatorv1.OperatorStatus, error) {
	uncastStatus, exists, err := unstructured.NestedMap(obj, "status")
	if !exists {
		return &operatorv1.OperatorStatus{}, nil
	}
	if err != nil {
		return nil, err
	}

	ret := &operatorv1.OperatorStatus{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(uncastStatus, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func setOperatorStatusFromUnstructured(obj map[string]interface{}, spec *operatorv1.OperatorStatus) error {
	// we cannot simply set the entire map because doing so would stomp unknown fields, like say a static pod operator spec when cast as an operator spec
	newUnstructuredStatus, err := runtime.DefaultUnstructuredConverter.ToUnstructured(spec)
	if err != nil {
		return err
	}

	originalUnstructuredStatus, exists, err := unstructured.NestedMap(obj, "status")
	if !exists {
		return unstructured.SetNestedMap(obj, newUnstructuredStatus, "status")
	}
	if err != nil {
		return err
	}
	if err := mergo.Merge(&originalUnstructuredStatus, newUnstructuredStatus, mergo.WithOverride); err != nil {
		return err
	}

	return unstructured.SetNestedMap(obj, originalUnstructuredStatus, "status")
}

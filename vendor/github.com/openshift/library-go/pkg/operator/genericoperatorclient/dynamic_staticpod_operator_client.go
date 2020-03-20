package genericoperatorclient

import (
	"context"
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
	"k8s.io/client-go/rest"
)

func NewStaticPodOperatorClient(config *rest.Config, gvr schema.GroupVersionResource) (v1helpers.StaticPodOperatorClient, dynamicinformer.DynamicSharedInformerFactory, error) {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	client := dynamicClient.Resource(gvr)

	informers := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 12*time.Hour)
	informer := informers.ForResource(gvr)

	return &dynamicStaticPodOperatorClient{
		dynamicOperatorClient: dynamicOperatorClient{
			informer: informer,
			client:   client,
		},
	}, informers, nil
}

type dynamicStaticPodOperatorClient struct {
	dynamicOperatorClient
}

func (c dynamicStaticPodOperatorClient) GetStaticPodOperatorState() (*operatorv1.StaticPodOperatorSpec, *operatorv1.StaticPodOperatorStatus, string, error) {
	uncastInstance, err := c.informer.Lister().Get("cluster")
	if err != nil {
		return nil, nil, "", err
	}
	instance := uncastInstance.(*unstructured.Unstructured)

	spec, err := getStaticPodOperatorSpecFromUnstructured(instance.UnstructuredContent())
	if err != nil {
		return nil, nil, "", err
	}
	status, err := getStaticPodOperatorStatusFromUnstructured(instance.UnstructuredContent())
	if err != nil {
		return nil, nil, "", err
	}

	return spec, status, instance.GetResourceVersion(), nil
}

func (c dynamicStaticPodOperatorClient) GetStaticPodOperatorStateWithQuorum() (*operatorv1.StaticPodOperatorSpec, *operatorv1.StaticPodOperatorStatus, string, error) {
	instance, err := c.client.Get(context.TODO(), "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, nil, "", err
	}

	spec, err := getStaticPodOperatorSpecFromUnstructured(instance.UnstructuredContent())
	if err != nil {
		return nil, nil, "", err
	}
	status, err := getStaticPodOperatorStatusFromUnstructured(instance.UnstructuredContent())
	if err != nil {
		return nil, nil, "", err
	}

	return spec, status, instance.GetResourceVersion(), nil
}

func (c dynamicStaticPodOperatorClient) UpdateStaticPodOperatorSpec(resourceVersion string, spec *operatorv1.StaticPodOperatorSpec) (*operatorv1.StaticPodOperatorSpec, string, error) {
	uncastOriginal, err := c.informer.Lister().Get("cluster")
	if err != nil {
		return nil, "", err
	}
	original := uncastOriginal.(*unstructured.Unstructured)

	copy := original.DeepCopy()
	copy.SetResourceVersion(resourceVersion)
	if err := setStaticPodOperatorSpecFromUnstructured(copy.UnstructuredContent(), spec); err != nil {
		return nil, "", err
	}

	ret, err := c.client.Update(context.TODO(), copy, metav1.UpdateOptions{})
	if err != nil {
		return nil, "", err
	}
	retSpec, err := getStaticPodOperatorSpecFromUnstructured(ret.UnstructuredContent())
	if err != nil {
		return nil, "", err
	}

	return retSpec, ret.GetResourceVersion(), nil
}

func (c dynamicStaticPodOperatorClient) UpdateStaticPodOperatorStatus(resourceVersion string, status *operatorv1.StaticPodOperatorStatus) (*operatorv1.StaticPodOperatorStatus, error) {
	uncastOriginal, err := c.informer.Lister().Get("cluster")
	if err != nil {
		return nil, err
	}
	original := uncastOriginal.(*unstructured.Unstructured)

	copy := original.DeepCopy()
	copy.SetResourceVersion(resourceVersion)
	if err := setStaticPodOperatorStatusFromUnstructured(copy.UnstructuredContent(), status); err != nil {
		return nil, err
	}

	ret, err := c.client.UpdateStatus(context.TODO(), copy, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	retStatus, err := getStaticPodOperatorStatusFromUnstructured(ret.UnstructuredContent())
	if err != nil {
		return nil, err
	}

	return retStatus, nil
}

func getStaticPodOperatorSpecFromUnstructured(obj map[string]interface{}) (*operatorv1.StaticPodOperatorSpec, error) {
	uncastSpec, exists, err := unstructured.NestedMap(obj, "spec")
	if !exists {
		return &operatorv1.StaticPodOperatorSpec{}, nil
	}
	if err != nil {
		return nil, err
	}

	ret := &operatorv1.StaticPodOperatorSpec{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(uncastSpec, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func setStaticPodOperatorSpecFromUnstructured(obj map[string]interface{}, spec *operatorv1.StaticPodOperatorSpec) error {
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

func getStaticPodOperatorStatusFromUnstructured(obj map[string]interface{}) (*operatorv1.StaticPodOperatorStatus, error) {
	uncastStatus, exists, err := unstructured.NestedMap(obj, "status")
	if !exists {
		return &operatorv1.StaticPodOperatorStatus{}, nil
	}
	if err != nil {
		return nil, err
	}

	ret := &operatorv1.StaticPodOperatorStatus{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(uncastStatus, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func setStaticPodOperatorStatusFromUnstructured(obj map[string]interface{}, spec *operatorv1.StaticPodOperatorStatus) error {
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

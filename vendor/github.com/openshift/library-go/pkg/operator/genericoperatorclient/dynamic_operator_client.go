package genericoperatorclient

import (
	"context"
	"reflect"
	"strings"
	"time"

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

// UpdateOperatorSpec overwrites the operator object spec with the values given
// in operatorv1.OperatorSpec while preserving pre-existing spec fields that have
// no correspondence in operatorv1.OperatorSpec.
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

	ret, err := c.client.Update(context.TODO(), copy, metav1.UpdateOptions{})
	if err != nil {
		return nil, "", err
	}
	retSpec, err := getOperatorSpecFromUnstructured(ret.UnstructuredContent())
	if err != nil {
		return nil, "", err
	}

	return retSpec, ret.GetResourceVersion(), nil
}

// UpdateOperatorStatus overwrites the operator object status with the values given
// in operatorv1.OperatorStatus while preserving pre-existing status fields that have
// no correspondence in operatorv1.OperatorStatus.
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

	ret, err := c.client.UpdateStatus(context.TODO(), copy, metav1.UpdateOptions{})
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
	// we cannot simply set the entire map because doing so would stomp unknown fields,
	// like say a static pod operator spec when cast as an operator spec
	newSpec, err := runtime.DefaultUnstructuredConverter.ToUnstructured(spec)
	if err != nil {
		return err
	}

	origSpec, preExistingSpec, err := unstructured.NestedMap(obj, "spec")
	if err != nil {
		return err
	}
	if preExistingSpec {
		flds := topLevelFields(*spec)
		for k, v := range origSpec {
			if !flds[k] {
				if err := unstructured.SetNestedField(newSpec, v, k); err != nil {
					return err
				}
			}
		}
	}
	return unstructured.SetNestedMap(obj, newSpec, "spec")
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

func setOperatorStatusFromUnstructured(obj map[string]interface{}, status *operatorv1.OperatorStatus) error {
	// we cannot simply set the entire map because doing so would stomp unknown fields,
	// like say a static pod operator status when cast as an operator status
	newStatus, err := runtime.DefaultUnstructuredConverter.ToUnstructured(status)
	if err != nil {
		return err
	}

	origStatus, preExistingStatus, err := unstructured.NestedMap(obj, "status")
	if err != nil {
		return err
	}
	if preExistingStatus {
		flds := topLevelFields(*status)
		for k, v := range origStatus {
			if !flds[k] {
				if err := unstructured.SetNestedField(newStatus, v, k); err != nil {
					return err
				}
			}
		}
	}
	return unstructured.SetNestedMap(obj, newStatus, "status")
}

func topLevelFields(obj interface{}) map[string]bool {
	ret := map[string]bool{}
	t := reflect.TypeOf(obj)
	for i := 0; i < t.NumField(); i++ {
		fld := t.Field(i)
		fieldName := fld.Name
		if jsonTag := fld.Tag.Get("json"); jsonTag == "-" {
			continue
		} else if jsonTag != "" {
			// check for possible comma as in "...,omitempty"
			var commaIdx int
			if commaIdx = strings.Index(jsonTag, ","); commaIdx < 0 {
				commaIdx = len(jsonTag)
			}
			fieldName = jsonTag[:commaIdx]
		}
		ret[fieldName] = true
	}
	return ret
}

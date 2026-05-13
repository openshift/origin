package common

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/ptr"
)

var (
	DeviceClassGVR = schema.GroupVersionResource{
		Group:    "resource.k8s.io",
		Version:  "v1",
		Resource: "deviceclasses",
	}
	ResourceClaimGVR = schema.GroupVersionResource{
		Group:    "resource.k8s.io",
		Version:  "v1",
		Resource: "resourceclaims",
	}
	ResourceClaimTemplateGVR = schema.GroupVersionResource{
		Group:    "resource.k8s.io",
		Version:  "v1",
		Resource: "resourceclaimtemplates",
	}
)

// ConvertToUnstructured converts a typed object to Unstructured
func ConvertToUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	unstructuredObj := &unstructured.Unstructured{}
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	unstructuredObj.Object = content
	return unstructuredObj, nil
}

// CreateDeviceClass creates a DeviceClass
func CreateDeviceClass(ctx context.Context, client dynamic.Interface, deviceClass interface{}) error {
	unstructuredObj, err := ConvertToUnstructured(deviceClass)
	if err != nil {
		return err
	}
	_, err = client.Resource(DeviceClassGVR).Create(ctx, unstructuredObj, metav1.CreateOptions{})
	return err
}

// DeleteDeviceClass deletes a DeviceClass
func DeleteDeviceClass(ctx context.Context, client dynamic.Interface, name string) error {
	return client.Resource(DeviceClassGVR).Delete(ctx, name, metav1.DeleteOptions{
		GracePeriodSeconds: ptr.To[int64](0),
	})
}

// CreateResourceClaim creates a ResourceClaim
func CreateResourceClaim(ctx context.Context, client dynamic.Interface, namespace string, claim interface{}) error {
	unstructuredObj, err := ConvertToUnstructured(claim)
	if err != nil {
		return err
	}
	_, err = client.Resource(ResourceClaimGVR).Namespace(namespace).Create(ctx, unstructuredObj, metav1.CreateOptions{})
	return err
}

// DeleteResourceClaim deletes a ResourceClaim
func DeleteResourceClaim(ctx context.Context, client dynamic.Interface, namespace, name string) error {
	return client.Resource(ResourceClaimGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{
		GracePeriodSeconds: ptr.To[int64](0),
	})
}

// CreateResourceClaimTemplate creates a ResourceClaimTemplate
func CreateResourceClaimTemplate(ctx context.Context, client dynamic.Interface, namespace string, template interface{}) error {
	unstructuredObj, err := ConvertToUnstructured(template)
	if err != nil {
		return err
	}
	_, err = client.Resource(ResourceClaimTemplateGVR).Namespace(namespace).Create(ctx, unstructuredObj, metav1.CreateOptions{})
	return err
}

// DeleteResourceClaimTemplate deletes a ResourceClaimTemplate
func DeleteResourceClaimTemplate(ctx context.Context, client dynamic.Interface, namespace, name string) error {
	return client.Resource(ResourceClaimTemplateGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{
		GracePeriodSeconds: ptr.To[int64](0),
	})
}

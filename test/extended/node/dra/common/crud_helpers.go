package common

import (
	"context"

	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateDeviceClass creates a DeviceClass using the typed client.
func CreateDeviceClass(ctx context.Context, client kubernetes.Interface, deviceClass *resourceapi.DeviceClass) error {
	_, err := client.ResourceV1().DeviceClasses().Create(ctx, deviceClass, metav1.CreateOptions{})
	return err
}

// DeleteDeviceClass deletes a DeviceClass
func DeleteDeviceClass(ctx context.Context, client kubernetes.Interface, name string) error {
	return client.ResourceV1().DeviceClasses().Delete(ctx, name, metav1.DeleteOptions{})
}

// CreateResourceClaim creates a ResourceClaim using the typed client.
func CreateResourceClaim(ctx context.Context, client kubernetes.Interface, namespace string, claim *resourceapi.ResourceClaim) error {
	_, err := client.ResourceV1().ResourceClaims(namespace).Create(ctx, claim, metav1.CreateOptions{})
	return err
}

// DeleteResourceClaim deletes a ResourceClaim
func DeleteResourceClaim(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	return client.ResourceV1().ResourceClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// CreateResourceClaimTemplate creates a ResourceClaimTemplate using the typed client.
func CreateResourceClaimTemplate(ctx context.Context, client kubernetes.Interface, namespace string, template *resourceapi.ResourceClaimTemplate) error {
	_, err := client.ResourceV1().ResourceClaimTemplates(namespace).Create(ctx, template, metav1.CreateOptions{})
	return err
}

// DeleteResourceClaimTemplate deletes a ResourceClaimTemplate
func DeleteResourceClaimTemplate(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	return client.ResourceV1().ResourceClaimTemplates(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

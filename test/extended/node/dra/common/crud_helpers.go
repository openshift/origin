package common

import (
	"context"
	"fmt"
	"time"

	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
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

// DeleteResourceClaimAndWait deletes a ResourceClaim and polls until it is
// gone.  This is important in Ordered Ginkgo contexts where later specs depend
// on capacity being fully released before they run.
func DeleteResourceClaimAndWait(ctx context.Context, client kubernetes.Interface, namespace, name string, timeout time.Duration) error {
	if err := client.ResourceV1().ResourceClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return wait.PollUntilContextTimeout(ctx, 1*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := client.ResourceV1().ResourceClaims(namespace).Get(ctx, name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, fmt.Errorf("error waiting for ResourceClaim %s/%s deletion: %w", namespace, name, err)
		}
		return false, nil
	})
}

// DeletePodAndWait deletes a Pod with a short grace period and polls until it
// is gone.  Mirrors DeleteResourceClaimAndWait for Ordered contexts where
// capacity must be fully released between specs.
func DeletePodAndWait(ctx context.Context, client kubernetes.Interface, namespace, name string, timeout time.Duration) error {
	gracePeriod := int64(10)
	if err := client.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return wait.PollUntilContextTimeout(ctx, 1*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, fmt.Errorf("error waiting for Pod %s/%s deletion: %w", namespace, name, err)
		}
		return false, nil
	})
}

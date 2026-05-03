package example

import (
	"context"
	"fmt"

	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

// DeviceValidator validates DRA device allocation and ResourceSlice state for the example driver.
type DeviceValidator struct {
	client    kubernetes.Interface
	framework *framework.Framework
}

// NewDeviceValidator creates a DeviceValidator using the provided test framework.
func NewDeviceValidator(f *framework.Framework) *DeviceValidator {
	return &DeviceValidator{
		client:    f.ClientSet,
		framework: f,
	}
}

// ValidateDeviceAllocation checks that the given ResourceClaim has exactly expectedCount devices allocated.
func (dv *DeviceValidator) ValidateDeviceAllocation(ctx context.Context, namespace, claimName string, expectedCount int) error {
	framework.Logf("Validating ResourceClaim allocation for %s/%s (expected %d device(s))", namespace, claimName, expectedCount)

	claim, err := dv.client.ResourceV1().ResourceClaims(namespace).Get(ctx, claimName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ResourceClaim %s/%s: %w", namespace, claimName, err)
	}

	if claim.Status.Allocation == nil {
		return fmt.Errorf("ResourceClaim %s/%s is not allocated", namespace, claimName)
	}

	deviceCount := len(claim.Status.Allocation.Devices.Results)
	if deviceCount != expectedCount {
		return fmt.Errorf("ResourceClaim %s/%s expected %d device(s) but got %d",
			namespace, claimName, expectedCount, deviceCount)
	}

	framework.Logf("ResourceClaim %s/%s has %d device(s) allocated", namespace, claimName, deviceCount)

	for i, result := range claim.Status.Allocation.Devices.Results {
		if result.Driver != exampleDriverName {
			return fmt.Errorf("device %d has incorrect driver %q, expected %q", i, result.Driver, exampleDriverName)
		}
		if result.Pool == "" {
			return fmt.Errorf("device %d has empty pool field", i)
		}
		if result.Device == "" {
			return fmt.Errorf("device %d has empty device field", i)
		}
		if result.Request == "" {
			return fmt.Errorf("device %d has empty request field", i)
		}

		framework.Logf("Device %d validated: driver=%s, pool=%s, device=%s, request=%s",
			i, result.Driver, result.Pool, result.Device, result.Request)
	}

	return nil
}

// ValidateResourceSlice finds and validates the ResourceSlice published by the example driver on the given node.
func (dv *DeviceValidator) ValidateResourceSlice(ctx context.Context, nodeName string) (*resourceapi.ResourceSlice, error) {
	framework.Logf("Validating ResourceSlice for node %s", nodeName)

	sliceList, err := dv.client.ResourceV1().ResourceSlices().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list ResourceSlices: %w", err)
	}

	var nodeSlice *resourceapi.ResourceSlice
	totalDevices := 0
	for i := range sliceList.Items {
		slice := &sliceList.Items[i]
		if slice.Spec.NodeName != nil && *slice.Spec.NodeName == nodeName &&
			slice.Spec.Driver == exampleDriverName {
			totalDevices += len(slice.Spec.Devices)
			if nodeSlice == nil && len(slice.Spec.Devices) > 0 {
				nodeSlice = slice
			}
		}
	}

	if nodeSlice == nil {
		return nil, fmt.Errorf("no ResourceSlice with devices found for driver %s on node %s", exampleDriverName, nodeName)
	}

	framework.Logf("Node %s has %d total device(s) across matching ResourceSlices (returning slice %s)",
		nodeName, totalDevices, nodeSlice.Name)
	return nodeSlice, nil
}

// GetTotalDeviceCount returns the total number of devices published by the example driver across all nodes.
func (dv *DeviceValidator) GetTotalDeviceCount(ctx context.Context) (int, error) {
	framework.Logf("Counting total devices from %s driver via ResourceSlices", exampleDriverName)

	sliceList, err := dv.client.ResourceV1().ResourceSlices().List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to list ResourceSlices: %w", err)
	}

	totalDevices := 0
	for _, slice := range sliceList.Items {
		if slice.Spec.Driver == exampleDriverName {
			totalDevices += len(slice.Spec.Devices)
		}
	}

	framework.Logf("Found %d total device(s) from %s driver", totalDevices, exampleDriverName)
	return totalDevices, nil
}

// IsDriverPublishingDevices returns true if the example driver has published at least one device.
func (dv *DeviceValidator) IsDriverPublishingDevices(ctx context.Context) bool {
	count, err := dv.GetTotalDeviceCount(ctx)
	if err != nil {
		framework.Logf("Failed to check if %s is publishing devices: %v", exampleDriverName, err)
		return false
	}
	return count > 0
}

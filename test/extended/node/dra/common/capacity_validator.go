package common

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

// CapacityValidator provides helpers for validating consumable capacity
// structures on DRA devices — per-device Capacity maps, AllowMultipleAllocations,
// RequestPolicy, and ConsumedCapacity in allocation results.
type CapacityValidator struct {
	client     kubernetes.Interface
	driverName string
}

func NewCapacityValidator(client kubernetes.Interface, driverName string) *CapacityValidator {
	return &CapacityValidator{
		client:     client,
		driverName: driverName,
	}
}

func (cv *CapacityValidator) listOptions() metav1.ListOptions {
	return metav1.ListOptions{
		FieldSelector: resourceapi.ResourceSliceSelectorDriver + "=" + cv.driverName,
	}
}

// HasCapacityDevices returns true if the driver publishes devices with non-empty
// Capacity maps, indicating consumable capacity support.
func (cv *CapacityValidator) HasCapacityDevices(ctx context.Context) bool {
	sliceList, err := cv.client.ResourceV1().ResourceSlices().List(ctx, cv.listOptions())
	if err != nil {
		framework.Logf("Failed to check for capacity devices: %v", err)
		return false
	}
	for _, slice := range sliceList.Items {
		for _, device := range slice.Spec.Devices {
			if len(device.Capacity) > 0 {
				return true
			}
		}
	}
	return false
}

// ValidateDeviceCapacity verifies that every device published by the driver has
// the expected capacity entries with non-zero values and a RequestPolicy set.
func (cv *CapacityValidator) ValidateDeviceCapacity(ctx context.Context, expectedCapacities []string) error {
	sliceList, err := cv.client.ResourceV1().ResourceSlices().List(ctx, cv.listOptions())
	if err != nil {
		return fmt.Errorf("failed to list ResourceSlices: %w", err)
	}

	deviceCount := 0
	for _, slice := range sliceList.Items {
		for _, device := range slice.Spec.Devices {
			if len(device.Capacity) == 0 {
				continue
			}
			deviceCount++
			for _, name := range expectedCapacities {
				cap, exists := device.Capacity[resourceapi.QualifiedName(name)]
				if !exists {
					return fmt.Errorf("device %s in slice %s missing capacity %q", device.Name, slice.Name, name)
				}
				if cap.Value.IsZero() {
					return fmt.Errorf("device %s capacity %q has zero value", device.Name, name)
				}
				if cap.RequestPolicy == nil {
					return fmt.Errorf("device %s capacity %q has no RequestPolicy", device.Name, name)
				}
				framework.Logf("Device %s: %s=%s (has RequestPolicy)", device.Name, name, cap.Value.String())
			}
		}
	}

	if deviceCount == 0 {
		return fmt.Errorf("no devices with Capacity found for driver %s", cv.driverName)
	}
	framework.Logf("Validated capacity across %d device(s) for driver %s", deviceCount, cv.driverName)
	return nil
}

// ValidateAllowMultipleAllocations verifies that every device published by the
// driver has AllowMultipleAllocations set to true.
func (cv *CapacityValidator) ValidateAllowMultipleAllocations(ctx context.Context) error {
	sliceList, err := cv.client.ResourceV1().ResourceSlices().List(ctx, cv.listOptions())
	if err != nil {
		return fmt.Errorf("failed to list ResourceSlices: %w", err)
	}

	deviceCount := 0
	for _, slice := range sliceList.Items {
		for _, device := range slice.Spec.Devices {
			if len(device.Capacity) == 0 {
				continue
			}
			deviceCount++
			if device.AllowMultipleAllocations == nil || !*device.AllowMultipleAllocations {
				return fmt.Errorf("device %s in slice %s does not have AllowMultipleAllocations=true", device.Name, slice.Name)
			}
		}
	}

	if deviceCount == 0 {
		return fmt.Errorf("no devices with Capacity found for driver %s", cv.driverName)
	}
	framework.Logf("Validated AllowMultipleAllocations=true on %d device(s)", deviceCount)
	return nil
}

// ValidateConsumedCapacity verifies that the allocation result for a ResourceClaim
// has ConsumedCapacity entries for each of the expected capacity names.
func (cv *CapacityValidator) ValidateConsumedCapacity(ctx context.Context, namespace, claimName string, expectedCapacities []string) error {
	claim, err := cv.client.ResourceV1().ResourceClaims(namespace).Get(ctx, claimName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ResourceClaim %s/%s: %w", namespace, claimName, err)
	}
	if claim.Status.Allocation == nil {
		return fmt.Errorf("ResourceClaim %s/%s is not allocated", namespace, claimName)
	}

	for i, result := range claim.Status.Allocation.Devices.Results {
		if len(result.ConsumedCapacity) == 0 {
			return fmt.Errorf("device result %d (%s) has no ConsumedCapacity", i, result.Device)
		}
		for _, name := range expectedCapacities {
			qty, exists := result.ConsumedCapacity[resourceapi.QualifiedName(name)]
			if !exists {
				return fmt.Errorf("device result %d (%s) missing ConsumedCapacity for %q", i, result.Device, name)
			}
			if qty.IsZero() {
				return fmt.Errorf("device result %d (%s) ConsumedCapacity %q is zero", i, result.Device, name)
			}
			framework.Logf("Device result %d (%s): ConsumedCapacity[%s]=%s", i, result.Device, name, qty.String())
		}
	}
	return nil
}

// GetNodeWithDevices returns the name of a schedulable worker node where the
// driver is publishing devices with capacity. Avoids master/control-plane nodes.
func (cv *CapacityValidator) GetNodeWithDevices(ctx context.Context) (string, error) {
	sliceList, err := cv.client.ResourceV1().ResourceSlices().List(ctx, cv.listOptions())
	if err != nil {
		return "", fmt.Errorf("failed to list ResourceSlices: %w", err)
	}

	nodeList, err := cv.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list nodes: %w", err)
	}

	taintedNodes := make(map[string]bool)
	for _, node := range nodeList.Items {
		for _, taint := range node.Spec.Taints {
			if taint.Effect == corev1.TaintEffectNoSchedule || taint.Effect == corev1.TaintEffectNoExecute {
				taintedNodes[node.Name] = true
				break
			}
		}
	}

	var fallback string
	for _, slice := range sliceList.Items {
		if slice.Spec.NodeName == nil || *slice.Spec.NodeName == "" {
			continue
		}
		hasCapacityDevices := false
		for _, device := range slice.Spec.Devices {
			if len(device.Capacity) > 0 {
				hasCapacityDevices = true
				break
			}
		}
		if !hasCapacityDevices {
			continue
		}

		name := *slice.Spec.NodeName
		if !taintedNodes[name] {
			return name, nil
		}
		if fallback == "" {
			fallback = name
		}
	}
	if fallback != "" {
		framework.Logf("Warning: no untainted node with capacity devices found, falling back to tainted node %s", fallback)
		return fallback, nil
	}
	return "", fmt.Errorf("no node found publishing capacity devices for driver %s", cv.driverName)
}

// GetTotalDeviceCount returns the total number of devices with capacity
// published by the driver.
func (cv *CapacityValidator) GetTotalDeviceCount(ctx context.Context) (int, error) {
	sliceList, err := cv.client.ResourceV1().ResourceSlices().List(ctx, cv.listOptions())
	if err != nil {
		return 0, fmt.Errorf("failed to list ResourceSlices: %w", err)
	}
	count := 0
	for _, slice := range sliceList.Items {
		for _, device := range slice.Spec.Devices {
			if len(device.Capacity) > 0 {
				count++
			}
		}
	}
	return count, nil
}

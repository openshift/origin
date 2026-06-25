package common

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

// CounterValidator provides helpers for validating ResourceSlice counter
// structures introduced by the DRAPartitionableDevices feature (KEP-4815).
// It separates ResourceSlices into counter slices (SharedCounters only) and
// device slices (Devices with ConsumesCounters), matching the two-slice model
// that partitionable drivers publish.
type CounterValidator struct {
	client     kubernetes.Interface
	driverName string
}

// NewCounterValidator creates a validator for the given driver.
func NewCounterValidator(client kubernetes.Interface, driverName string) *CounterValidator {
	return &CounterValidator{
		client:     client,
		driverName: driverName,
	}
}

// GetResourceSlicesByType lists all ResourceSlices for the driver and separates
// them into counter slices (have SharedCounters but no Devices) and device slices
// (have Devices, may have ConsumesCounters on individual devices).
func (cv *CounterValidator) GetResourceSlicesByType(ctx context.Context) (counterSlices, deviceSlices []resourceapi.ResourceSlice, err error) {
	sliceList, err := cv.client.ResourceV1().ResourceSlices().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list ResourceSlices: %w", err)
	}

	for _, slice := range sliceList.Items {
		if slice.Spec.Driver != cv.driverName {
			continue
		}
		if len(slice.Spec.SharedCounters) > 0 && len(slice.Spec.Devices) == 0 {
			counterSlices = append(counterSlices, slice)
		}
		if len(slice.Spec.Devices) > 0 {
			deviceSlices = append(deviceSlices, slice)
		}
	}
	return counterSlices, deviceSlices, nil
}

// ValidateSharedCounters verifies that at least one counter slice exists and
// that every CounterSet in those slices contains the expected counters with
// non-zero values. Returns an error describing the first violation found.
func (cv *CounterValidator) ValidateSharedCounters(ctx context.Context, expectedCounters []string) error {
	counterSlices, _, err := cv.GetResourceSlicesByType(ctx)
	if err != nil {
		return err
	}
	if len(counterSlices) == 0 {
		return fmt.Errorf("no ResourceSlices with SharedCounters found for driver %s", cv.driverName)
	}

	for _, slice := range counterSlices {
		for _, cs := range slice.Spec.SharedCounters {
			for _, name := range expectedCounters {
				counter, exists := cs.Counters[name]
				if !exists {
					return fmt.Errorf("CounterSet %q in slice %s missing counter %q", cs.Name, slice.Name, name)
				}
				if counter.Value.IsZero() {
					return fmt.Errorf("CounterSet %q counter %q has zero value", cs.Name, name)
				}
				framework.Logf("CounterSet %s: %s=%s", cs.Name, name, counter.Value.String())
			}
		}
	}
	framework.Logf("Validated SharedCounters across %d counter slice(s) for driver %s", len(counterSlices), cv.driverName)
	return nil
}

// ValidateDeviceConsumesCounters verifies that every device in the driver's
// device slices has at least one ConsumesCounters entry pointing to a named
// CounterSet.
func (cv *CounterValidator) ValidateDeviceConsumesCounters(ctx context.Context) error {
	_, deviceSlices, err := cv.GetResourceSlicesByType(ctx)
	if err != nil {
		return err
	}
	if len(deviceSlices) == 0 {
		return fmt.Errorf("no ResourceSlices with Devices found for driver %s", cv.driverName)
	}

	for _, slice := range deviceSlices {
		for _, device := range slice.Spec.Devices {
			if len(device.ConsumesCounters) == 0 {
				return fmt.Errorf("device %s in slice %s has no ConsumesCounters", device.Name, slice.Name)
			}
			for _, cc := range device.ConsumesCounters {
				if cc.CounterSet == "" {
					return fmt.Errorf("device %s has ConsumesCounters with empty CounterSet name", device.Name)
				}
			}
			framework.Logf("Device %s consumes from %d counter set(s)", device.Name, len(device.ConsumesCounters))
		}
	}
	return nil
}

// CountPartitionDevices returns the number of devices whose names contain
// "partition" across all device slices for the driver.
func (cv *CounterValidator) CountPartitionDevices(ctx context.Context) (int, error) {
	_, deviceSlices, err := cv.GetResourceSlicesByType(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, slice := range deviceSlices {
		for _, device := range slice.Spec.Devices {
			if strings.Contains(device.Name, "partition") {
				count++
			}
		}
	}
	return count, nil
}

// HasSharedCounters returns true if the driver is publishing ResourceSlices
// that contain SharedCounters.
func (cv *CounterValidator) HasSharedCounters(ctx context.Context) bool {
	counterSlices, _, err := cv.GetResourceSlicesByType(ctx)
	if err != nil {
		framework.Logf("Failed to check for SharedCounters: %v", err)
		return false
	}
	return len(counterSlices) > 0
}

// GetNodeWithDevices returns the name of a schedulable worker node where the
// driver is publishing devices. It avoids master/control-plane nodes whose
// taints would prevent regular test pods from being scheduled there. Falls
// back to any node with devices if no untainted node is found.
func (cv *CounterValidator) GetNodeWithDevices(ctx context.Context) (string, error) {
	sliceList, err := cv.client.ResourceV1().ResourceSlices().List(ctx, metav1.ListOptions{})
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
		if slice.Spec.Driver != cv.driverName || slice.Spec.NodeName == nil || *slice.Spec.NodeName == "" {
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
		framework.Logf("Warning: no untainted node with devices found, falling back to tainted node %s", fallback)
		return fallback, nil
	}
	return "", fmt.Errorf("no node found publishing devices for driver %s", cv.driverName)
}

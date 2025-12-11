package example

import (
	"context"
	"fmt"
	"testing"
	"time"

	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	helper "github.com/openshift/origin/test/extended/dra/helper"
)

func NewExampleDRADriver(t testing.TB, clientset kubernetes.Interface, p helper.HelmParameters) *ExampleDRADriver {
	return &ExampleDRADriver{
		t:         t,
		name:      "gpu.example.com",
		clientset: clientset,
		helm:      helper.NewHelmInstaller(t, p),
		namespace: p.Namespace,
	}
}

type ExampleDRADriver struct {
	t         testing.TB
	name      string
	helm      *helper.HelmInstaller
	clientset kubernetes.Interface
	namespace string
}

func (d *ExampleDRADriver) Class() string                     { return d.name }
func (d *ExampleDRADriver) Setup(ctx context.Context) error   { return d.helm.Install(ctx) }
func (d *ExampleDRADriver) Cleanup(ctx context.Context) error { return d.helm.Remove(ctx) }

func (d ExampleDRADriver) Ready(ctx context.Context) error {
	client := d.clientset.AppsV1()
	ds, err := client.DaemonSets(d.namespace).Get(ctx, "dra-example-driver-kubeletplugin", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get dra-example-driver-kubeletplugin: %w", err)
	}

	ready, scheduled := ds.Status.NumberReady, ds.Status.DesiredNumberScheduled
	if ready != scheduled {
		return fmt.Errorf("dra-example-driver-kubeletplugin is not ready, DesiredNumberScheduled: %d, NumberReady: %d", scheduled, ready)
	}
	return nil
}

func (d *ExampleDRADriver) EventuallyPublishResources(ctx context.Context, node *corev1.Node) (dc *resourceapi.DeviceClass, slices []resourceapi.ResourceSlice) {
	class := d.name
	o.Eventually(ctx, func(ctx context.Context) error {
		// has the driver published the device class?
		obj, err := d.clientset.ResourceV1().DeviceClasses().Get(ctx, class, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("still waiting for the driver to advertise its DeviceClass")
		}

		result, err := d.clientset.ResourceV1().ResourceSlices().List(ctx, metav1.ListOptions{
			FieldSelector: resourceapi.ResourceSliceSelectorDriver + "=" + class + "," +
				resourceapi.ResourceSliceSelectorNodeName + "=" + node.Name,
		})
		if err != nil || len(result.Items) == 0 {
			return fmt.Errorf("still waiting for the driver to advertise its ResourceSlice  - %w", err)
		}
		dc = obj
		slices = result.Items
		return nil
	}).WithPolling(2*time.Second).Should(o.BeNil(), "timeout while waiting for the driver to advertise its resources")

	return dc, slices
}

func (d *ExampleDRADriver) ListPublishedDevices(ctx context.Context, node *corev1.Node) ([]string, error) {
	result, err := d.clientset.ResourceV1().ResourceSlices().List(ctx, metav1.ListOptions{
		FieldSelector: resourceapi.ResourceSliceSelectorDriver + "=" + d.name + "," +
			resourceapi.ResourceSliceSelectorNodeName + "=" + node.Name,
	})
	if err != nil || len(result.Items) == 0 {
		return nil, fmt.Errorf("still waiting for the driver to advertise its ResourceSlice  - %w", err)
	}

	devices := []string{}
	for _, rs := range result.Items {
		for _, device := range rs.Spec.Devices {
			devices = append(devices, device.Name)
		}
	}
	return devices, nil
}

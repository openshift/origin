package utility

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func NewExampleDRADriver(t testing.TB, clientset kubernetes.Interface, p HelmParameters) *ExampleDRADriver {
	return &ExampleDRADriver{
		t:         t,
		name:      "gpu.example.com",
		clientset: clientset,
		helm:      NewHelmInstaller(t, p),
		namespace: p.Namespace,
	}
}

type ExampleDRADriver struct {
	t         testing.TB
	name      string
	helm      *HelmInstaller
	clientset kubernetes.Interface
	namespace string
}

func (d *ExampleDRADriver) DeviceClassName() string           { return d.name }
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

func (d *ExampleDRADriver) GetPublishedResources(ctx context.Context, node *corev1.Node) (*resourceapi.DeviceClass, []resourceapi.ResourceSlice, error) {
	class := d.name
	client := d.clientset.ResourceV1beta1()

	// has the driver published the device class?
	dc, err := client.DeviceClasses().Get(ctx, class, metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("still waiting for the driver to advertise its DeviceClass")
	}
	result, err := client.ResourceSlices().List(ctx, metav1.ListOptions{
		FieldSelector: resourceapi.ResourceSliceSelectorDriver + "=" + class + "," +
			resourceapi.ResourceSliceSelectorNodeName + "=" + node.Name,
	})
	if err != nil || len(result.Items) == 0 {
		return nil, nil, fmt.Errorf("still waiting for the driver to advertise its ResourceSlice  - %w", err)
	}
	return dc, result.Items, nil
}

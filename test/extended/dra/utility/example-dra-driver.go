package utility

import (
	"context"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func NewExampleDRADriver(t testing.TB, clientset kubernetes.Interface, p HelmParameters) *ExampleDRADriver {
	return &ExampleDRADriver{
		t:         t,
		clientset: clientset,
		helm:      NewHelmInstaller(t, p),
		namespace: p.Namespace,
	}
}

type ExampleDRADriver struct {
	t         testing.TB
	helm      *HelmInstaller
	clientset kubernetes.Interface
	namespace string
}

func (d *ExampleDRADriver) DeviceClassName() string           { return "gpu.example.com" }
func (d *ExampleDRADriver) Setup() error                      { return d.helm.Install() }
func (d *ExampleDRADriver) Cleanup(ctx context.Context) error { return d.helm.Remove() }

func (d ExampleDRADriver) Ready() error {
	client := d.clientset.AppsV1()
	ds, err := client.DaemonSets(d.namespace).Get(context.Background(), "dra-example-driver-kubeletplugin", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get dra-example-driver-kubeletplugin: %w", err)
	}

	ready, scheduled := ds.Status.NumberReady, ds.Status.DesiredNumberScheduled
	if ready != scheduled {
		return fmt.Errorf("dra-example-driver-kubeletplugin is not ready, DesiredNumberScheduled: %d, NumberReady: %d", scheduled, ready)
	}
	return nil
}

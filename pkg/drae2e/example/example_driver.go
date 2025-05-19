package example

import (
	"context"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/openshift/origin/pkg/drae2e"
)

func NewDriver(t testing.TB, clientset kubernetes.Interface, p drae2e.HelmParameters) *driver {
	return &driver{
		t:         t,
		clientset: clientset,
		helm:      drae2e.NewHelmInstaller(t, p),
		namespace: p.Namespace,
	}
}

type driver struct {
	t         testing.TB
	helm      *drae2e.Helm
	clientset kubernetes.Interface
	namespace string
}

func (d *driver) DeviceClassName() string           { return "gpu.example.com" }
func (d *driver) Setup() error                      { return d.helm.Install() }
func (d *driver) Cleanup(ctx context.Context) error { return d.helm.Remove() }

func (d driver) Ready() error {
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

package nvidia

import (
	"context"
	"testing"

	"k8s.io/client-go/kubernetes"

	"github.com/openshift/origin/pkg/drae2e"
)

func NewDriver(t testing.TB, clientset kubernetes.Interface, p drae2e.HelmParameters) *driver {
	return &driver{
		t:         t,
		namespace: p.Namespace,
		clientset: clientset,
		helm:      drae2e.NewHelmInstaller(t, p),
	}
}

type driver struct {
	t         testing.TB
	helm      *drae2e.Helm
	clientset kubernetes.Interface
	namespace string
}

func (d *driver) DeviceClassName() string           { return "gpu.nvidia.com" }
func (d *driver) Setup() error                      { return d.helm.Install() }
func (d *driver) Cleanup(ctx context.Context) error { return d.helm.Remove() }
func (d driver) Ready() error                       { return nil }

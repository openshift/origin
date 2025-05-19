package utility

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	testutils "k8s.io/kubernetes/test/utils"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
)

func NewDRADriverGPU(t testing.TB, clientset kubernetes.Interface, p HelmParameters) *DRADriverGPU {
	return &DRADriverGPU{
		t:         t,
		namespace: p.Namespace,
		clientset: clientset,
		helm:      NewHelmInstaller(t, p),
	}
}

type DRADriverGPU struct {
	t         testing.TB
	helm      *HelmInstaller
	clientset kubernetes.Interface
	namespace string
}

func (d *DRADriverGPU) DeviceClassName() string           { return "gpu.nvidia.com" }
func (d *DRADriverGPU) Setup() error                      { return d.helm.Install() }
func (d *DRADriverGPU) Cleanup(ctx context.Context) error { return d.helm.Remove() }
func (d *DRADriverGPU) Ready(node *corev1.Node) error {
	client := d.clientset.CoreV1().Pods(d.helm.Namespace)

	for _, probe := range []struct {
		component string
		enabled   bool
		options   metav1.ListOptions
	}{
		{
			enabled:   true,
			component: "nvidia-dra-driver-gpu",
			options: metav1.ListOptions{
				LabelSelector: "app.kubernetes.io/name" + "=" + "nvidia-dra-driver-gpu",
				FieldSelector: "spec.nodeName" + "=" + node.Name,
			},
		},
	} {
		if probe.enabled {
			g.By(fmt.Sprintf("waiting for %s to be ready", probe.component))

			var name string
			o.Eventually(func() error {
				result, err := client.List(context.Background(), probe.options)
				if err != nil || len(result.Items) == 0 {
					return fmt.Errorf("[%s] still waiting for pod to show up - %w", probe.component, err)
				}
				name = result.Items[0].Name
				return nil
			}).WithPolling(5*time.Second).WithTimeout(10*time.Minute).Should(o.BeNil(), fmt.Sprintf("[%s] pod never showed up", probe.component))
			d.t.Logf("[%s] found pod: %s", probe.component, name)

			o.Eventually(func() error {
				pod, err := client.Get(context.Background(), name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				ready, err := testutils.PodRunningReady(pod)
				if err != nil || !ready {
					err = fmt.Errorf("[%s] still waiting for pod: %s to be ready: %v", probe.component, pod.Name, err)
					d.t.Logf(err.Error())
					return err
				}
				return nil
			}).WithPolling(5*time.Second).WithTimeout(10*time.Minute).Should(o.BeNil(), "nvidia gpu driver pod never showed up")
		}
	}

	return nil
}

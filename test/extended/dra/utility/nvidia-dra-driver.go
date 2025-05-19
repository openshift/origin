package utility

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
)

func NewNvidiaDRADriverGPU(t testing.TB, clientset kubernetes.Interface, p HelmParameters) *NvidiaDRADriverGPU {
	return &NvidiaDRADriverGPU{
		t:         t,
		name:      "nvidia-dra-driver-gpu",
		class:     "gpu.nvidia.com",
		namespace: p.Namespace,
		clientset: clientset,
		helm:      NewHelmInstaller(t, p),
	}
}

type NvidiaDRADriverGPU struct {
	t         testing.TB
	name      string
	class     string
	helm      *HelmInstaller
	clientset kubernetes.Interface
	namespace string
}

func (d *NvidiaDRADriverGPU) DeviceClassName() string           { return "gpu.nvidia.com" }
func (d *NvidiaDRADriverGPU) Setup(ctx context.Context) error   { return d.helm.Install(ctx) }
func (d *NvidiaDRADriverGPU) Cleanup(ctx context.Context) error { return d.helm.Remove(ctx) }
func (d *NvidiaDRADriverGPU) Ready(ctx context.Context, node *corev1.Node) error {
	for _, probe := range []struct {
		component string
		enabled   bool
		options   metav1.ListOptions
	}{
		{
			enabled:   true,
			component: d.name,
			options: metav1.ListOptions{
				LabelSelector: "app.kubernetes.io/name" + "=" + d.name,
				FieldSelector: "spec.nodeName" + "=" + node.Name,
			},
		},
	} {
		if probe.enabled {
			g.By(fmt.Sprintf("waiting for %s to be ready", probe.component))
			o.Eventually(func() error {
				return PodRunningReady(ctx, d.t, d.clientset, probe.component, d.helm.Namespace, probe.options)
			}).WithPolling(5*time.Second).
				WithTimeout(10*time.Minute).Should(o.BeNil(), fmt.Sprintf("[%s] pod should be ready", probe.component))
		}
	}

	return nil
}

func (d *NvidiaDRADriverGPU) GetPublishedResources(ctx context.Context, node *corev1.Node) (*resourceapi.DeviceClass, []resourceapi.ResourceSlice, error) {
	client := d.clientset.ResourceV1beta1()

	// has the driver published the device class?
	dc, err := client.DeviceClasses().Get(context.Background(), d.class, metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("still waiting for the driver to advertise its DeviceClass")
	}
	result, err := client.ResourceSlices().List(context.Background(), metav1.ListOptions{
		FieldSelector: resourceapi.ResourceSliceSelectorDriver + "=" + d.class + "," +
			resourceapi.ResourceSliceSelectorNodeName + "=" + node.Name,
	})
	if err != nil || len(result.Items) == 0 {
		return nil, nil, fmt.Errorf("still waiting for the driver to advertise its ResourceSlice  - %w", err)
	}
	return dc, result.Items, nil
}

func (d *NvidiaDRADriverGPU) RemovePluginFromNode(node *corev1.Node) error {
	client := d.clientset.CoreV1().Pods(d.helm.Namespace)
	result, err := client.List(context.Background(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name" + "=" + d.name,
		FieldSelector: "spec.nodeName" + "=" + node.Name,
	})
	if err != nil || len(result.Items) == 0 {
		return fmt.Errorf("[%s] expected a pod running on node: %s - %w", d.name, node.Name, err)
	}

	pod := result.Items[0]
	return e2epod.DeletePodWithWait(context.Background(), d.clientset, &pod)
}

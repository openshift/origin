package nvidia

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/ptr"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	helper "github.com/openshift/origin/test/extended/dra/helper"
)

func NewNvidiaDRADriverGPU(t testing.TB, clientset kubernetes.Interface, p helper.HelmParameters) *NvidiaDRADriverGPU {
	return &NvidiaDRADriverGPU{
		t:         t,
		name:      "nvidia-dra-driver-gpu",
		class:     "gpu.nvidia.com",
		namespace: p.Namespace,
		clientset: clientset,
		helm:      helper.NewHelmInstaller(t, p),
	}
}

type NvidiaDRADriverGPU struct {
	t         testing.TB
	name      string
	class     string
	helm      *helper.HelmInstaller
	clientset kubernetes.Interface
	namespace string
}

func (d *NvidiaDRADriverGPU) Class() string                     { return "gpu.nvidia.com" }
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
				return helper.PodRunningReady(ctx, d.t, d.clientset, probe.component, d.helm.Namespace, probe.options)
			}).WithPolling(5*time.Second).
				WithTimeout(10*time.Minute).Should(o.BeNil(), fmt.Sprintf("[%s] pod should be ready", probe.component))
		}
	}

	return nil
}

func (d *NvidiaDRADriverGPU) EventuallyPublishResources(ctx context.Context, node *corev1.Node) (dc *resourceapi.DeviceClass, slices []resourceapi.ResourceSlice) {
	o.Eventually(ctx, func(ctx context.Context) error {
		// has the driver published the device class?
		obj, err := d.clientset.ResourceV1beta1().DeviceClasses().Get(ctx, d.class, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("still waiting for the driver to advertise its DeviceClass")
		}

		result, err := d.clientset.ResourceV1beta1().ResourceSlices().List(ctx, metav1.ListOptions{
			FieldSelector: resourceapi.ResourceSliceSelectorDriver + "=" + d.class + "," +
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

func (d *NvidiaDRADriverGPU) RemovePluginFromNode(ctx context.Context, node *corev1.Node) error {
	pods, err := d.clientset.CoreV1().Pods(d.helm.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name" + "=" + d.name,
		FieldSelector: "spec.nodeName" + "=" + node.Name,
	})
	if err != nil || len(pods.Items) == 0 {
		return fmt.Errorf("[%s] expected a pod running on node: %s - %w", d.name, node.Name, err)
	}

	pod := pods.Items[0]
	if err := e2epod.DeletePodWithWait(context.Background(), d.clientset, &pod); err != nil {
		return err
	}

	slices, err := d.clientset.ResourceV1beta1().ResourceSlices().List(ctx, metav1.ListOptions{
		FieldSelector: resourceapi.ResourceSliceSelectorDriver + "=" + d.class + "," +
			resourceapi.ResourceSliceSelectorNodeName + "=" + node.Name,
	})
	if err != nil || len(slices.Items) == 0 {
		return err
	}
	return d.clientset.ResourceV1beta1().ResourceSlices().Delete(ctx, slices.Items[0].Name, metav1.DeleteOptions{})
}

func (d *NvidiaDRADriverGPU) GetGPUFromResourceSlice(ctx context.Context, node *corev1.Node, device string) (NvidiaGPU, error) {
	devices, err := d.ListPublishedDevicesFromResourceSlice(ctx, node)
	if err != nil {
		return NvidiaGPU{}, err
	}
	if matching := devices.FilterBy(func(gpu NvidiaGPU) bool { return gpu.Name == device }); len(matching) > 0 {
		return matching[0], nil
	}
	return NvidiaGPU{}, nil
}

func (d *NvidiaDRADriverGPU) GetMPSControlDaemonForClaim(ctx context.Context, node *corev1.Node, claim *resourceapi.ResourceClaim) (*corev1.Pod, error) {
	// MpsControlDaemonNameFmt      = "mps-control-daemon-%v" // Fill with ClaimUID
	// https://github.com/NVIDIA/k8s-dra-driver-gpu/blob/9951be59ac1bf827d02a1f0f3dfc0cea65c7aca3/cmd/gpu-kubelet-plugin/sharing.go#L55
	prefix := fmt.Sprintf("mps-control-daemon-%v", claim.UID)
	d.t.Logf("MPS control daemon prefix: %s", prefix)

	// MPS control daemon is located in the same namespace as the driver
	result, err := d.clientset.CoreV1().Pods(d.namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName" + "=" + node.Name,
	})
	if err != nil || len(result.Items) == 0 {
		return nil, fmt.Errorf("did not find MPS control daemon pod for %s on node: %s - %w", claim.Name, node.Name, err)
	}

	for i := range result.Items {
		pod := &result.Items[i]
		if strings.HasPrefix(pod.Name, prefix) {
			return pod, nil
		}
	}
	return nil, nil
}

func (d *NvidiaDRADriverGPU) ListPublishedDevicesFromResourceSlice(ctx context.Context, node *corev1.Node) (NvidiaGPUs, error) {
	result, err := d.clientset.ResourceV1beta1().ResourceSlices().List(ctx, metav1.ListOptions{
		FieldSelector: resourceapi.ResourceSliceSelectorDriver + "=" + d.class + "," +
			resourceapi.ResourceSliceSelectorNodeName + "=" + node.Name,
	})
	if err != nil || len(result.Items) == 0 {
		return nil, fmt.Errorf("still waiting for the driver to advertise its ResourceSlice  - %w", err)
	}

	devices := NvidiaGPUs{}
	for _, rs := range result.Items {
		for _, got := range rs.Spec.Devices {
			gpu := NvidiaGPU{Name: got.Name}
			if got.Basic != nil {
				if attribute, ok := got.Basic.Attributes["type"]; ok {
					gpu.Type = ptr.Deref[string](attribute.StringValue, "")
				}
				if attribute, ok := got.Basic.Attributes["uuid"]; ok {
					gpu.UUID = ptr.Deref[string](attribute.StringValue, "")
				}
				if attribute, ok := got.Basic.Attributes["index"]; ok && attribute.IntValue != nil {
					gpu.Index = strconv.Itoa(int(*attribute.IntValue))
				}
			}
			devices = append(devices, gpu)
		}
	}
	return devices, nil
}

type NvidiaGPU struct {
	Type  string
	UUID  string
	Index string
	Name  string
}

func (gpu NvidiaGPU) String() string {
	return fmt.Sprintf("name: %s, type: %s, uuid: %s, index: %s", gpu.Name, gpu.Type, gpu.UUID, gpu.Index)
}

type NvidiaGPUs []NvidiaGPU

func (s NvidiaGPUs) FilterBy(f func(gpu NvidiaGPU) bool) NvidiaGPUs {
	gpus := NvidiaGPUs{}
	for _, gpu := range s {
		if f(gpu) {
			gpus = append(gpus, gpu)
		}
	}
	return gpus
}

func (s NvidiaGPUs) Names() []string {
	names := []string{}
	for _, gpu := range s {
		names = append(names, gpu.Name)
	}
	return names
}

func (s NvidiaGPUs) UUIDs() []string {
	uuids := []string{}
	for _, gpu := range s {
		uuids = append(uuids, gpu.UUID)
	}
	return uuids
}

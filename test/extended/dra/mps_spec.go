package dra

import (
	"context"
	"fmt"
	"strings"
	"testing"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epodutil "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/ptr"

	helper "github.com/openshift/origin/test/extended/dra/helper"
	nvidia "github.com/openshift/origin/test/extended/dra/nvidia"
)

// references:
//   - https://github.com/NVIDIA/k8s-dra-driver-gpu/blob/main/demo/specs/quickstart/gpu-test-mps.yaml
//   - https://docs.nvidia.com/deploy/mps/index.html
//
// MPS with CUDA
// One pod, 2 containers share GPU using MPS
type mpsWithCUDASpec struct {
	f     *framework.Framework
	class string
	// the node onto which the pod is expected to run
	node   *corev1.Node
	dra    *nvidia.NvidiaDRADriverGPU
	driver *nvidia.GpuOperator
}

func (spec mpsWithCUDASpec) Test(ctx context.Context, t testing.TB) {
	namespace := spec.f.Namespace.Name
	clientset := spec.f.ClientSet

	// MPS strategy
	const (
		mpsStrategy = `{
    "apiVersion": "resource.nvidia.com/v1beta1",
    "kind": "GpuConfig",
    "sharing": {
        "strategy": "MPS",
        "mpsConfig": {
            "defaultActiveThreadPercentage": 50,
            "defaultPinnedDeviceMemoryLimit": "10Gi"
        }
    }
}
`
	)
	template := &resourceapi.ResourceClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "shared-gpu",
		},
	}
	template.Spec.Spec.Devices.Requests = []resourceapi.DeviceRequest{
		{
			Name:    "mps-gpu",
			Exactly: &resourceapi.ExactDeviceRequest{DeviceClassName: spec.class},
		},
	}
	template.Spec.Spec.Devices.Config = []resourceapi.DeviceClaimConfiguration{
		{
			Requests: []string{"mps-gpu"},
			DeviceConfiguration: resourceapi.DeviceConfiguration{
				Opaque: &resourceapi.OpaqueDeviceConfiguration{
					Driver:     spec.class,
					Parameters: runtime.RawExtension{Raw: []byte(mpsStrategy)},
				},
			},
		},
	}

	newContainer := func(name string) corev1.Container {
		ctr := corev1.Container{
			Name:    name,
			Image:   "nvcr.io/nvidia/k8s/cuda-sample:nbody-cuda11.6.0-ubuntu18.04",
			Command: []string{"bash", "-c"},
			Args:    []string{"trap 'exit 0' TERM; /tmp/sample --benchmark --numbodies=4226048 & wait"},
		}
		ctr.Resources.Claims = []corev1.ResourceClaim{{Name: "shared-gpu", Request: "mps-gpu"}}
		return ctr
	}
	// one pod, two containers, both containers share a single gpu
	pod := helper.NewPod(namespace, "pod")
	pod.Spec.Containers = []corev1.Container{
		newContainer("mps-ctr0"),
		newContainer("mps-ctr1"),
	}
	pod.Spec.ResourceClaims = []corev1.PodResourceClaim{
		{
			Name:                      "shared-gpu",
			ResourceClaimTemplateName: ptr.To(template.Name),
		},
	}
	pod.Spec.Tolerations = []corev1.Toleration{
		{
			Key:      "nvidia.com/gpu",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		},
	}

	g.By("creating external claim and pod")
	t.Logf("creating resource template: \n%s\n", framework.PrettyPrintJSON(template))
	_, err := clientset.ResourceV1().ResourceClaimTemplates(namespace).Create(ctx, template, metav1.CreateOptions{})
	o.Expect(err).To(o.BeNil())

	t.Logf("creating pod: \n%s\n", framework.PrettyPrintJSON(pod))
	pod, err = clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	o.Expect(err).To(o.BeNil())

	g.DeferCleanup(func(ctx context.Context) {
		g.By(fmt.Sprintf("listing resources in namespace: %s", namespace))
		t.Logf("pod in test namespace: %s\n%s", namespace, framework.PrettyPrintJSON(pod))

		result, err := clientset.ResourceV1().ResourceClaims(namespace).List(ctx, metav1.ListOptions{})
		o.Expect(err).Should(o.BeNil())
		t.Logf("resource claim in test namespace: %s\n%s", namespace, framework.PrettyPrintJSON(result))
	})

	g.By(fmt.Sprintf("waiting for pod %s/%s to be running", pod.Namespace, pod.Name))
	err = e2epodutil.WaitForPodRunningInNamespace(ctx, clientset, pod)
	o.Expect(err).To(o.BeNil())

	// the pod should run on the expected node
	pod, err = clientset.CoreV1().Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	o.Expect(err).To(o.BeNil())
	o.Expect(pod.Spec.NodeName).To(o.Equal(spec.node.Name))

	claim, err := helper.GetResourceClaimFor(ctx, clientset, pod)
	o.Expect(err).To(o.BeNil())
	o.Expect(claim).ToNot(o.BeNil())

	device, pool, err := helper.GetAllocatedDeviceForRequest("mps-gpu", claim)
	o.Expect(err).To(o.BeNil())
	o.Expect(device).ToNot(o.BeEmpty())
	o.Expect(pool).ToNot(o.BeEmpty())
	t.Logf("device %q has been allocated to claim: %q", device, claim.Name)

	// get the index of the gpu allocated to us
	gpu, err := spec.dra.GetGPUFromResourceSlice(ctx, spec.node, device)
	o.Expect(err).To(o.BeNil())
	o.Expect(gpu.UUID).ToNot(o.BeEmpty())
	o.Expect(gpu.Index).ToNot(o.BeEmpty())

	// we expect at least three processes to be running
	// a) two instances of "/tmp/sample" type: M+C
	// b) one instance of "nvidia-cuda-mps-server" type: C
	const (
		sample = "/tmp/sample"
		server = "nvidia-cuda-mps-server"
	)
	processes, err := spec.driver.QueryCompute(ctx, spec.node, gpu.Index)
	o.Expect(err).To(o.BeNil())
	o.Expect(len(processes)).To(o.BeNumerically(">=", 3))

	names := processes.FilterBy(func(p nvidia.NvidiaCompute) bool { return p.Name == sample || p.Name == server }).Names()
	o.Expect(names).To(o.ConsistOf([]string{sample, sample, server}))

	// TODO: nvidia-smi --query-compute-apps does not recognize a key to
	// list the process type, for now we run nvidia-smi
	lines, err := spec.driver.RunNvidiSMI(ctx, spec.node)
	o.Expect(err).To(o.BeNil())
	samples := []string{}
	for _, line := range lines {
		if strings.Contains(line, sample) {
			samples = append(samples, line)
		}
	}
	o.Expect(len(samples)).To(o.Equal(2))
	o.Expect(samples).Should(o.HaveExactElements(o.ContainSubstring("M+C"), o.ContainSubstring("M+C")))

	g.By("capturing MPS control daemon logs")
	mpsControlPod, err := spec.dra.GetMPSControlDaemonForClaim(ctx, spec.node, claim)
	o.Expect(err).To(o.BeNil())
	o.Expect(mpsControlPod).ToNot(o.BeNil())
	t.Logf("MPS control daemon pod: %s", mpsControlPod.Name)
	logs, err := helper.GetLogs(ctx, clientset, mpsControlPod.Namespace, mpsControlPod.Name, "mps-control-daemon")
	o.Expect(err).To(o.BeNil())
	t.Logf("logs from MPS daemon control pod, container: %s\n%s\n", "mps-control-daemon", logs)

	// chroot /driver-root sh -c "echo get_default_active_thread_percentage | nvidia-cuda-mps-control"
	lines, err = helper.ExecIntoContainer(ctx, t, spec.f, mpsControlPod.Name, mpsControlPod.Namespace,
		"mps-control-daemon",
		[]string{"chroot", "/driver-root", "sh", "-c", "echo get_default_active_thread_percentage | nvidia-cuda-mps-control"})
	o.Expect(err).To(o.BeNil())
	o.Expect(len(lines)).To(o.Equal(1))
	o.Expect(lines[0]).To(o.Equal("50.0"))

	// TODO: memory limit does not show up as expected
	_, err = helper.ExecIntoContainer(ctx, t, spec.f, pod.Name, pod.Namespace,
		"mps-control-daemon",
		[]string{"chroot", "/driver-root", "sh", "-c", "echo get_default_device_pinned_mem_limit 0 | nvidia-cuda-mps-control"})

	g.By(fmt.Sprintf("retrieving logs for pod %s/%s", pod.Namespace, pod.Name))
	for _, ctr := range pod.Spec.Containers {
		logs, err := helper.GetLogs(ctx, spec.f.ClientSet, pod.Namespace, pod.Name, ctr.Name)
		o.Expect(err).To(o.BeNil())
		t.Logf("logs from container: %s\n%s\n", ctr.Name, logs)
	}
}

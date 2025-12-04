package dra

import (
	"context"
	"fmt"
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

// sharing a gpu with TimeSlicing
type gpuTimeSlicingWithCUDASpec struct {
	f     *framework.Framework
	class string
	// the node onto which the pod is expected to run
	node   *corev1.Node
	driver *nvidia.GpuOperator
	dra    *nvidia.NvidiaDRADriverGPU
}

// One pod, two containers, each container sharing a gpu with TimeSlicing
func (spec gpuTimeSlicingWithCUDASpec) Test(ctx context.Context, t testing.TB) {
	namespace := spec.f.Namespace.Name
	clientset := spec.f.ClientSet

	// TimeSlicing strategy
	const (
		timeSlicingStrategy = `{
    "apiVersion": "resource.nvidia.com/v1beta1",
    "kind": "GpuConfig",
    "sharing": {
        "strategy": "TimeSlicing",
        "timeSlicingConfig": {
            "interval": "Long"
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
			Name:    "ts-gpu",
			Exactly: &resourceapi.ExactDeviceRequest{DeviceClassName: spec.class},
		},
	}
	template.Spec.Spec.Devices.Config = []resourceapi.DeviceClaimConfiguration{
		{
			Requests: []string{"ts-gpu"},
			DeviceConfiguration: resourceapi.DeviceConfiguration{
				Opaque: &resourceapi.OpaqueDeviceConfiguration{
					Driver:     spec.class,
					Parameters: runtime.RawExtension{Raw: []byte(timeSlicingStrategy)},
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
		ctr.Resources.Claims = []corev1.ResourceClaim{{Name: "shared-gpu", Request: "ts-gpu"}}
		return ctr
	}
	// one pod, two containers, each time sharing the device
	pod := helper.NewPod(namespace, "pod")
	pod.Spec.Containers = []corev1.Container{
		newContainer("ts-ctr0"),
		newContainer("ts-ctr1"),
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

		client := clientset.ResourceV1().ResourceClaims(namespace)
		result, err := client.List(ctx, metav1.ListOptions{})
		o.Expect(err).Should(o.BeNil())
		t.Logf("resource claim in test namespace: %s\n%s", namespace, framework.PrettyPrintJSON(result))
	})

	g.By(fmt.Sprintf("waiting for pod %s/%s to be running", pod.Namespace, pod.Name))
	err = e2epodutil.WaitForPodRunningInNamespace(ctx, spec.f.ClientSet, pod)
	o.Expect(err).To(o.BeNil())

	// the pod should run on the expected node
	pod, err = clientset.CoreV1().Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	o.Expect(err).To(o.BeNil())
	o.Expect(pod.Spec.NodeName).To(o.Equal(spec.node.Name))

	claim, err := helper.GetResourceClaimFor(ctx, clientset, pod)
	o.Expect(err).To(o.BeNil())
	o.Expect(claim).ToNot(o.BeNil())

	device, pool, err := helper.GetAllocatedDeviceForRequest("ts-gpu", claim)
	o.Expect(err).To(o.BeNil())
	o.Expect(device).ToNot(o.BeEmpty())
	o.Expect(pool).ToNot(o.BeEmpty())
	t.Logf("device %q has been allocated to claim: %q", device, claim.Name)

	// get the index of the gpu allocated to us
	gpu, err := spec.dra.GetGPUFromResourceSlice(ctx, spec.node, device)
	o.Expect(err).To(o.BeNil())
	o.Expect(gpu.UUID).ToNot(o.BeEmpty())
	o.Expect(gpu.Index).ToNot(o.BeEmpty())

	lines, err := spec.driver.RunNvidiSMI(ctx, spec.node, "-i", gpu.Index)
	o.Expect(err).To(o.BeNil())
	o.Expect(lines).Should(o.ContainElements(o.ContainSubstring("/tmp/sample"), o.ContainSubstring("/tmp/sample")))

	g.By(fmt.Sprintf("retrieving logs for pod %s/%s", pod.Namespace, pod.Name))
	for _, ctr := range pod.Spec.Containers {
		logs, err := helper.GetLogs(ctx, spec.f.ClientSet, pod.Namespace, pod.Name, ctr.Name)
		o.Expect(err).To(o.BeNil())
		t.Logf("logs from container: %s\n%s\n", ctr.Name, logs)
	}
}

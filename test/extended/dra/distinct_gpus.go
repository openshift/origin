package dra

import (
	"context"
	"fmt"
	"testing"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epodutil "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/ptr"

	helper "github.com/openshift/origin/test/extended/dra/helper"
	nvidia "github.com/openshift/origin/test/extended/dra/nvidia"
)

// two pods, one container each, and each container uses two distinct gpus
type distinctGPUsSpec struct {
	f     *framework.Framework
	class string
	// the node onto which the pods are expected to run
	node *corev1.Node
	// UUID of the two distinct gpus
	gpu0, gpu1 string
}

func (spec distinctGPUsSpec) Test(ctx context.Context, t testing.TB) {
	namespace := spec.f.Namespace.Name
	newResourceClaim := func(name, request, gpu string) *resourceapi.ResourceClaim {
		claim := &resourceapi.ResourceClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
		}
		claim.Spec.Devices.Requests = []resourceapi.DeviceRequest{
			{
				Name:            request,
				DeviceClassName: spec.class,
				Selectors: []resourceapi.DeviceSelector{
					{
						CEL: &resourceapi.CELDeviceSelector{
							Expression: fmt.Sprintf("device.attributes['%s'].uuid == \"%s\"", spec.class, gpu),
						},
					},
				},
			},
		}
		return claim
	}
	newContainer := func(name string) corev1.Container {
		return corev1.Container{
			Name:    name,
			Image:   "nvidia/cuda:12.4.1-devel-ubuntu22.04",
			Command: []string{"bash", "-c"},
			Args:    []string{"nvidia-smi -L; trap 'exit 0' TERM; sleep 9999 & wait"},
		}
	}
	newPod := func(podName, ctrName, claim, request string) *corev1.Pod {
		pod := helper.NewPod(namespace, podName)

		ctr := newContainer(ctrName)
		ctr.Resources.Claims = []corev1.ResourceClaim{
			{
				Name:    "gpu",
				Request: request,
			},
		}
		pod.Spec.Containers = append(pod.Spec.Containers, ctr)

		pod.Spec.ResourceClaims = []corev1.PodResourceClaim{
			{
				Name:              "gpu",
				ResourceClaimName: ptr.To(claim),
			},
		}
		pod.Spec.Tolerations = []corev1.Toleration{
			{
				Key:      "nvidia.com/gpu",
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoSchedule,
			},
		}
		return pod
	}

	g.By("creating resourceclaims")
	for _, claim := range []*resourceapi.ResourceClaim{
		newResourceClaim("pod-a", "nvidia-gpu-0", spec.gpu0),
		newResourceClaim("pod-b", "nvidia-gpu-1", spec.gpu1),
	} {
		t.Logf("creating resourceclaim: \n%s\n", framework.PrettyPrintJSON(claim))
		_, err := spec.f.ClientSet.ResourceV1beta1().ResourceClaims(namespace).Create(ctx, claim, metav1.CreateOptions{})
		o.Expect(err).To(o.BeNil())
	}

	pods := []*corev1.Pod{}
	for _, want := range []*corev1.Pod{
		newPod("pod-a", "a-ctr", "pod-a", "nvidia-gpu-0"),
		newPod("pod-b", "b-ctr", "pod-b", "nvidia-gpu-1"),
	} {
		t.Logf("creating pod: \n%s\n", framework.PrettyPrintJSON(want))
		pod, err := spec.f.ClientSet.CoreV1().Pods(namespace).Create(ctx, want, metav1.CreateOptions{})
		o.Expect(err).To(o.BeNil())
		pods = append(pods, pod)
	}

	g.DeferCleanup(func(ctx context.Context) {
		g.By(fmt.Sprintf("listing resources in namespace: %s", namespace))
		pods, err := spec.f.ClientSet.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		t.Logf("pod in test namespace: %s\n%s", namespace, framework.PrettyPrintJSON(pods))

		claims, err := spec.f.ClientSet.ResourceV1beta1().ResourceClaims(namespace).List(ctx, metav1.ListOptions{})
		o.Expect(err).Should(o.BeNil())
		t.Logf("resource claim in test namespace: %s\n%s", namespace, framework.PrettyPrintJSON(claims))
	})

	want := map[string]bool{
		spec.gpu0: true,
		spec.gpu1: true,
	}
	for _, pod := range pods {
		g.By(fmt.Sprintf("waiting for pod %s/%s to be running", pod.Namespace, pod.Name))
		err := e2epodutil.WaitForPodRunningInNamespace(ctx, spec.f.ClientSet, pod)
		o.Expect(err).To(o.BeNil())

		// get the latest revision of the pod so we can verify its expected state
		pod, err = spec.f.ClientSet.CoreV1().Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		o.Expect(err).To(o.BeNil())

		// the pods should run on the expected node
		o.Expect(pod.Spec.NodeName).To(o.Equal(spec.node.Name))

		claim, err := spec.f.ClientSet.ResourceV1beta1().ResourceClaims(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		o.Expect(err).To(o.BeNil())
		o.Expect(claim).ToNot(o.BeNil())
		o.Expect(claim.Status.Allocation).NotTo(o.BeNil())
		o.Expect(len(claim.Status.Allocation.Devices.Results)).To(o.Equal(1))

		result := claim.Status.Allocation.Devices.Results[0]
		o.Expect(result.Request).To(o.Equal(pod.Spec.Containers[0].Resources.Claims[0].Request))
		o.Expect(result.Device).ToNot(o.BeEmpty())

		t.Logf("pod %s/%s has been allocated nvidia gpu: %s", pod.Namespace, pod.Name, result.Device)

		ctr := pod.Spec.Containers[0]
		g.By(fmt.Sprintf("running nvidia-smi command into pod %s/%s container: %s", pod.Namespace, pod.Name, ctr.Name))
		gpus, err := nvidia.QueryGPUUsedByContainer(ctx, t, spec.f, pod.Name, pod.Namespace, ctr.Name)
		o.Expect(err).To(o.BeNil())
		o.Expect(len(gpus)).To(o.Equal(1))

		o.Expect(want).To(o.HaveKey(gpus[0].UUID))
		delete(want, gpus[0].UUID)
	}

	o.Expect(len(want)).To(o.Equal(0))
}

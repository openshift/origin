package dra

import (
	"context"
	"fmt"
	"testing"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	drae2eutility "github.com/openshift/origin/test/extended/dra/utility"
	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epodutil "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/ptr"
)

// exercises a use case with static MIG devices
type mpsWithCUDASpec struct {
	t            testing.TB
	f            *framework.Framework
	class        string
	newContainer func(name string) corev1.Container
	// the node onto which the pod is expected to run
	node *corev1.Node
}

// One pod, N containers, each asking for a MIG device on a shared mig-enabled GPU
func (spec mpsWithCUDASpec) Test(ctx context.Context, t g.GinkgoTInterface) {
	namespace := spec.f.Namespace.Name
	newDeviceConfiguration := func(driver string, raw []byte) resourceapi.DeviceConfiguration {
		return resourceapi.DeviceConfiguration{
			Opaque: &resourceapi.OpaqueDeviceConfiguration{
				Driver:     driver,
				Parameters: runtime.RawExtension{Raw: raw},
			},
		}
	}

	// create a resource claim template that uses both time slicing and MPS
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

	template := drae2eutility.NewResourceClaimTemplate(namespace, "shared-gpu")
	template.Spec.Spec.Devices.Requests = []resourceapi.DeviceRequest{
		{Name: "mps-gpu", DeviceClassName: spec.class},
	}
	template.Spec.Spec.Devices.Config = []resourceapi.DeviceClaimConfiguration{
		{Requests: []string{"mps-gpu"}, DeviceConfiguration: newDeviceConfiguration(spec.class, []byte(mpsStrategy))},
	}

	// one pod, and one container for each of the devices
	pod := drae2eutility.NewPod(namespace, "pod")
	ctrs := []corev1.Container{
		spec.newContainer("mps-ctr0"),
		spec.newContainer("mps-ctr1"),
	}
	for i := range ctrs {
		ctrs[i].Resources.Claims = []corev1.ResourceClaim{{Name: "shared-gpu", Request: "mps-gpu"}}
	}
	pod.Spec.Containers = append(pod.Spec.Containers, ctrs...)
	pod.Spec.ResourceClaims = []corev1.PodResourceClaim{
		{
			Name:                      template.Name,
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
	resource := spec.f.ClientSet.ResourceV1beta1()
	t.Logf("creating resource template: \n%s\n", framework.PrettyPrintJSON(template))
	_, err := resource.ResourceClaimTemplates(namespace).Create(ctx, template, metav1.CreateOptions{})
	o.Expect(err).To(o.BeNil())

	core := spec.f.ClientSet.CoreV1()
	t.Logf("creating pod: \n%s\n", framework.PrettyPrintJSON(pod))
	pod, err = core.Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	o.Expect(err).To(o.BeNil())

	g.DeferCleanup(func(ctx context.Context) {
		g.By(fmt.Sprintf("listing resources in namespace: %s", namespace))
		t.Logf("pod in test namespace: %s\n%s", namespace, framework.PrettyPrintJSON(pod))

		client := spec.f.ClientSet.ResourceV1beta1().ResourceClaims(namespace)
		result, err := client.List(ctx, metav1.ListOptions{})
		o.Expect(err).Should(o.BeNil())
		t.Logf("resource claim in test namespace: %s\n%s", namespace, framework.PrettyPrintJSON(result))
	})

	g.By(fmt.Sprintf("waiting for pod %s/%s to be running", pod.Namespace, pod.Name))
	err = e2epodutil.WaitForPodRunningInNamespace(ctx, spec.f.ClientSet, pod)
	o.Expect(err).To(o.BeNil())

	// the pod should run on the expected node
	pod, err = core.Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	o.Expect(err).To(o.BeNil())
	o.Expect(pod.Spec.NodeName).To(o.Equal(spec.node.Name))

	_, err = drae2eutility.ExecIntoPod(ctx, spec.t, spec.f, pod.Name, pod.Namespace,
		"mps-ctr0",
		[]string{"echo", "get_default_active_thread_percentage", "|", "nvidia-cuda-mps-control"})
	_, err = drae2eutility.ExecIntoPod(ctx, spec.t, spec.f, pod.Name, pod.Namespace,
		"mps-ctr1",
		[]string{"echo", "get_default_active_thread_percentage", "|", "nvidia-cuda-mps-control"})

	_, err = resource.ResourceClaims(namespace).List(ctx, metav1.ListOptions{})
	o.Expect(err).To(o.BeNil())

	g.By(fmt.Sprintf("retrieving logs for pod %s/%s", pod.Namespace, pod.Name))
	for _, ctr := range pod.Spec.Containers {
		logs, err := drae2eutility.GetLogs(ctx, spec.f.ClientSet, pod.Namespace, pod.Name, ctr.Name)
		o.Expect(err).To(o.BeNil())
		t.Logf("logs from container: %s\n%s\n", ctr.Name, logs)
	}
}

package dra

import (
	"context"
	"fmt"
	"testing"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	helper "github.com/openshift/origin/test/extended/dra/helper"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epodutil "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/ptr"
)

// expected to run on both the example DRA driver and the nvidia DRA driver
// one pod, one container, requests a distinct gpu
type commonSpec struct {
	f *framework.Framework
	// driver name
	class        string
	newContainer func(name string) corev1.Container
	// device names in the resourceslices advertised by the driver
	deviceNamesAdvertised []string
	// the node onto which the pod is expected to run
	node *corev1.Node
}

func (spec commonSpec) Test(ctx context.Context, t testing.TB) {
	namespace := spec.f.Namespace.Name

	template := &resourceapi.ResourceClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "single-gpu",
		},
	}
	template.Spec.Spec.Devices.Requests = []resourceapi.DeviceRequest{
		{
			Name:    "gpu",
			Exactly: &resourceapi.ExactDeviceRequest{DeviceClassName: spec.class},
		},
	}

	// one pod, one container
	pod := helper.NewPod(namespace, "pod")
	ctr := spec.newContainer("ctr")
	ctr.Resources.Claims = []corev1.ResourceClaim{
		{
			Name: "gpu",
		},
	}
	pod.Spec.Containers = append(pod.Spec.Containers, ctr)
	pod.Spec.ResourceClaims = []corev1.PodResourceClaim{
		{
			Name:                      "gpu",
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
	resource := spec.f.ClientSet.ResourceV1()
	_, err := resource.ResourceClaimTemplates(namespace).Create(ctx, template, metav1.CreateOptions{})
	o.Expect(err).To(o.BeNil())

	core := spec.f.ClientSet.CoreV1()
	pod, err = core.Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	o.Expect(err).To(o.BeNil())

	g.DeferCleanup(func(ctx context.Context) {
		g.By(fmt.Sprintf("listing resources in namespace: %s", namespace))
		t.Logf("pod in test namespace: %s\n%s", namespace, framework.PrettyPrintJSON(pod))

		client := spec.f.ClientSet.ResourceV1().ResourceClaims(namespace)
		result, err := client.List(ctx, metav1.ListOptions{})
		o.Expect(err).Should(o.BeNil())
		t.Logf("resource claim in test namespace: %s\n%s", namespace, framework.PrettyPrintJSON(result))
	})

	g.By(fmt.Sprintf("waiting for pod %s/%s to be running", pod.Namespace, pod.Name))
	err = e2epodutil.WaitForPodRunningInNamespace(ctx, spec.f.ClientSet, pod)
	o.Expect(err).To(o.BeNil())

	pod, err = core.Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	o.Expect(err).To(o.BeNil())
	o.Expect(pod.Spec.NodeName).To(o.Equal(spec.node.Name))

	claim, err := helper.GetResourceClaimFor(ctx, spec.f.ClientSet, pod)
	o.Expect(err).To(o.BeNil())
	o.Expect(claim).ToNot(o.BeNil())
	o.Expect(claim.Status.Allocation).NotTo(o.BeNil())
	o.Expect(len(claim.Status.Allocation.Devices.Results)).To(o.Equal(1))

	allocation := claim.Status.Allocation.Devices.Results[0]
	o.Expect(allocation.Request).To(o.Equal("gpu"))
	o.Expect(allocation.Driver).To(o.Equal(spec.class))
	o.Expect(spec.deviceNamesAdvertised).To(o.ContainElement(allocation.Device))

	g.By(fmt.Sprintf("retrieving logs for pod %s/%s", pod.Namespace, pod.Name))
	logs, err := helper.GetLogs(ctx, spec.f.ClientSet, pod.Namespace, pod.Name, ctr.Name)
	o.Expect(err).To(o.BeNil())
	t.Logf("logs from container: %s\n%s\n", ctr.Name, logs)
}

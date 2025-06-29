package dra

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	drae2eutility "github.com/openshift/origin/test/extended/dra/utility"
	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epodutil "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/ptr"
)

// exercises a use case with static MIG devices
type gpuMIGSpec struct {
	f            *framework.Framework
	class        string
	newContainer func(name string) corev1.Container
	// the node onto which the pod is expected to run
	node *corev1.Node
	// MIG devices, same MIG device can appear twice or more
	devices []string
	// device names in the resourceslices advertised by the driver
	deviceNamesAdvertised []string
}

// One pod, N containers, each asking for a MIG device on a shared mig-enabled GPU
func (spec gpuMIGSpec) Test(ctx context.Context, t g.GinkgoTInterface) {
	namespace := spec.f.Namespace.Name
	newSelectors := func(migDeviceName string) []resourceapi.DeviceSelector {
		return []resourceapi.DeviceSelector{
			{
				CEL: &resourceapi.CELDeviceSelector{
					Expression: "device.attributes['" + spec.class + "'].profile == '" + migDeviceName + "'",
				},
			},
		}
	}

	// create a resource claim template that contains a request for each mig device
	template := drae2eutility.NewResourceClaimTemplate(namespace, "mig-devices")
	for i, device := range spec.devices {
		name := fmt.Sprintf("%s-%d", strings.ReplaceAll(device, ".", "-"), i)
		req := resourceapi.DeviceRequest{Name: name, DeviceClassName: "mig.nvidia.com", Selectors: newSelectors(device)}
		template.Spec.Spec.Devices.Requests = append(template.Spec.Spec.Devices.Requests, req)
	}
	template.Spec.Spec.Devices.Constraints = []resourceapi.DeviceConstraint{
		{
			MatchAttribute: ptr.To(resourceapi.FullyQualifiedName(spec.class + "/parentUUID")),
		},
	}

	// one pod, and one container for each of the devices
	pod := drae2eutility.NewPod(namespace, "pod")
	for i, request := range template.Spec.Spec.Devices.Requests {
		ctr := spec.newContainer(fmt.Sprintf("ctr%d", i))
		ctr.Resources.Claims = []corev1.ResourceClaim{{Name: "mig-devices", Request: request.Name}}
		pod.Spec.Containers = append(pod.Spec.Containers, ctr)
	}
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
	_, err := resource.ResourceClaimTemplates(namespace).Create(ctx, template, metav1.CreateOptions{})
	o.Expect(err).To(o.BeNil())

	core := spec.f.ClientSet.CoreV1()
	pod, err = core.Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	o.Expect(err).To(o.BeNil())

	g.DeferCleanup(func(_ context.Context) {
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

	result, err := resource.ResourceClaims(namespace).List(ctx, metav1.ListOptions{})
	o.Expect(err).To(o.BeNil())
	o.Expect(len(result.Items)).To(o.Equal(1))
	rc := result.Items[0]
	o.Expect(rc.Status.Allocation).NotTo(o.BeNil())
	o.Expect(len(rc.Status.Allocation.Devices.Results)).To(o.Equal(len(spec.devices)))
	for i, alloc := range rc.Status.Allocation.Devices.Results {
		o.Expect(alloc.Request).To(o.Equal(template.Spec.Spec.Devices.Requests[i].Name))
		o.Expect(alloc.Driver).To(o.Equal(spec.class))
		o.Expect(spec.deviceNamesAdvertised).To(o.ContainElement(alloc.Device))
	}

	g.By(fmt.Sprintf("retrieving logs for pod %s/%s", pod.Namespace, pod.Name))
	devicesUsed := []string{}
	for i, ctr := range pod.Spec.Containers {
		logs, err := drae2eutility.GetLogs(ctx, spec.f.ClientSet, pod.Namespace, pod.Name, ctr.Name)
		o.Expect(err).To(o.BeNil())
		t.Logf("logs from container: %s\n%s\n", ctr.Name, logs)

		got := drae2eutility.ExtractMIGDevices(logs)
		o.Expect(len(got)).To(o.Equal(1))
		o.Expect(got[0]).To(o.Equal(spec.devices[i]))

		devicesUsed = append(devicesUsed, got...)
	}
	o.Expect(spec.devices).To(o.Equal(devicesUsed))
}

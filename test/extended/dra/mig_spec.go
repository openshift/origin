package dra

import (
	"context"
	"fmt"
	"strings"
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

// exercises a use case with static MIG devices
// reference:
//   - https://docs.nvidia.com/datacenter/tesla/mig-user-guide/
//   - https://github.com/NVIDIA/mig-parted
type gpuMIGSpec struct {
	f     *framework.Framework
	class string
	// the node onto which the pod is expected to run
	node *corev1.Node
	// MIG devices, same MIG device can appear twice or more
	devices []string
	// UUIDs of the MIG devices advertised by the gpu driver
	uuids []string
}

// One pod, N containers, each asking for a MIG device on a shared mig-enabled GPU
func (spec gpuMIGSpec) Test(ctx context.Context, t testing.TB) {
	namespace := spec.f.Namespace.Name
	clientset := spec.f.ClientSet

	// create a resource claim template that contains a request for each mig device
	template := &resourceapi.ResourceClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "mig-devices",
		},
	}
	for i, device := range spec.devices {
		name := fmt.Sprintf("%s-%d", strings.ReplaceAll(device, ".", "-"), i)
		template.Spec.Spec.Devices.Requests = append(template.Spec.Spec.Devices.Requests, resourceapi.DeviceRequest{
			Name:            name,
			DeviceClassName: "mig.nvidia.com",
			Selectors: []resourceapi.DeviceSelector{
				{
					CEL: &resourceapi.CELDeviceSelector{
						Expression: "device.attributes['" + spec.class + "'].profile == '" + device + "'",
					},
				},
			},
		})
	}
	template.Spec.Spec.Devices.Constraints = []resourceapi.DeviceConstraint{
		{
			MatchAttribute: ptr.To(resourceapi.FullyQualifiedName(spec.class + "/parentUUID")),
		},
	}

	// one pod, N container(s), each wants a MIG device
	pod := helper.NewPod(namespace, "pod")
	for i, request := range template.Spec.Spec.Devices.Requests {
		ctr := corev1.Container{
			Name:    fmt.Sprintf("ctr%d", i),
			Image:   "ubuntu:22.04",
			Command: []string{"bash", "-c"},
			Args:    []string{"nvidia-smi -L; trap 'exit 0' TERM; sleep 9999 & wait"},
		}
		ctr.Resources.Claims = []corev1.ResourceClaim{{Name: "mig-devices", Request: request.Name}}
		pod.Spec.Containers = append(pod.Spec.Containers, ctr)
	}
	pod.Spec.ResourceClaims = []corev1.PodResourceClaim{
		{
			Name:                      "mig-devices",
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
	_, err := clientset.ResourceV1beta1().ResourceClaimTemplates(namespace).Create(ctx, template, metav1.CreateOptions{})
	o.Expect(err).To(o.BeNil())

	pod, err = clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	o.Expect(err).To(o.BeNil())

	g.DeferCleanup(func(ctx context.Context) {
		g.By(fmt.Sprintf("listing resources in namespace: %s", namespace))
		t.Logf("pod in test namespace: %s\n%s", namespace, framework.PrettyPrintJSON(pod))

		result, err := clientset.ResourceV1beta1().ResourceClaims(namespace).List(ctx, metav1.ListOptions{})
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

	o.Expect(claim.Status.Allocation).NotTo(o.BeNil())
	o.Expect(len(claim.Status.Allocation.Devices.Results)).To(o.Equal(len(spec.devices)))

	migUsed := nvidia.NvidiaGPUs{}
	for _, ctr := range pod.Spec.Containers {
		g.By(fmt.Sprintf("running nvidia-smi command into pod %s/%s container: %s", pod.Namespace, pod.Name, ctr.Name))
		lines, err := helper.ExecIntoContainer(ctx, t, spec.f, pod.Name, pod.Namespace, ctr.Name,
			[]string{"nvidia-smi", "-L"})
		o.Expect(err).To(o.BeNil())
		got := nvidia.ExtractMIGDeviceInfoFromNvidiaSMILines(lines)
		migUsed = append(migUsed, got...)
	}
	o.Expect(migUsed.UUIDs()).To(o.Equal(spec.uuids))
}

package dra

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	drae2eutility "github.com/openshift/origin/test/extended/dra/utility"
	exutil "github.com/openshift/origin/test/extended/util"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epodutil "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/ptr"
)

type driver interface {
	DeviceClassName() string
}

type commonSpec struct {
	f  *framework.Framework
	oc *exutil.CLI

	driver       driver
	newContainer func(name string) corev1.Container
	// the node onto which the pod is expected to run
	node *corev1.Node
}

func (spec commonSpec) Test(t g.GinkgoTInterface) {
	// create a namespace
	ns, err := spec.f.CreateNamespace(context.TODO(), "dra", nil)
	framework.ExpectNoError(err)

	deviceclass := spec.driver.DeviceClassName()
	helper := drae2eutility.NewHelper(ns.Name, spec.driver.DeviceClassName())

	claim := &resourceapi.ResourceClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns.Name,
			Name:      "single-gpu",
		},
		Spec: resourceapi.ResourceClaimTemplateSpec{
			Spec: resourceapi.ResourceClaimSpec{
				Devices: resourceapi.DeviceClaim{
					Requests: []resourceapi.DeviceRequest{
						{
							Name:            "gpu",
							DeviceClassName: deviceclass,
						},
					},
				},
			},
		},
	}

	// one pod, one container
	pod := helper.NewPod("pod")
	ctr := spec.newContainer("ctr")
	ctr.Resources.Claims = []corev1.ResourceClaim{{Name: "gpu"}}
	pod.Spec.Containers = append(pod.Spec.Containers, ctr)
	pod.Spec.ResourceClaims = []corev1.PodResourceClaim{
		{
			Name:                      "gpu",
			ResourceClaimTemplateName: ptr.To(claim.Name),
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
	claim, err = resource.ResourceClaimTemplates(ns.Name).Create(context.Background(), claim, metav1.CreateOptions{})
	o.Expect(err).To(o.BeNil())

	core := spec.f.ClientSet.CoreV1()
	pod, err = core.Pods(ns.Name).Create(context.Background(), pod, metav1.CreateOptions{})
	o.Expect(err).To(o.BeNil())

	g.By(fmt.Sprintf("waiting for pod %s/%s to be running", pod.Namespace, pod.Name))
	err = e2epodutil.WaitForPodRunningInNamespace(context.Background(), spec.f.ClientSet, pod)
	o.Expect(err).To(o.BeNil())

	pod, err = core.Pods(ns.Name).Get(context.Background(), pod.Name, metav1.GetOptions{})
	o.Expect(err).To(o.BeNil())
	o.Expect(pod.Spec.NodeName).To(o.Equal(spec.node.Name))

	result, err := resource.ResourceClaims(ns.Name).List(context.Background(), metav1.ListOptions{})
	o.Expect(err).To(o.BeNil())
	o.Expect(len(result.Items)).To(o.Equal(1))
	rc := result.Items[0]
	o.Expect(rc.Status.Allocation).NotTo(o.BeNil())
	o.Expect(len(rc.Status.Allocation.Devices.Results)).To(o.Equal(1))

	allocation := rc.Status.Allocation.Devices.Results[0]
	o.Expect(allocation.Request).To(o.Equal("gpu"))
	o.Expect(allocation.Driver).To(o.Equal(deviceclass))
	o.Expect(allocation.Device).To(o.Equal("gpu-0"))

	g.By(fmt.Sprintf("retrieving logs for pod %s/%s", pod.Namespace, pod.Name))
	args := []string{
		"-n", pod.Namespace, pod.Name, "--all-containers",
	}
	logs, err := spec.oc.AsAdmin().Run("logs").Args(args...).Output()
	o.Expect(err).To(o.BeNil())
	t.Logf("\n%s\n", logs)
}

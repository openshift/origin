package node

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	nodeutils "github.com/openshift/origin/test/extended/node"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-node] [Jira:Node/Kubelet] Network namespace cleanup", func() {
	var (
		oc = exutil.NewCLIWithoutNamespace("netns-cleanup")
	)

	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Skipping test on MicroShift cluster")
		}
	})

	//author: bgudi@redhat.com
	g.It("[OTP] kubelet/crio will delete netns when a pod is deleted [OCP-56266]", ote.Informing(), func() {
		ctx := context.Background()
		oc.SetupProject()
		namespace := oc.Namespace()
		podName := "pod-56266"

		g.By("Create a test pod")
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: namespace,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:    "hello-openshift",
						Image:   "image-registry.openshift-image-registry.svc:5000/openshift/tools:latest",
						Command: []string{"sleep", "infinity"},
					},
				},
			},
		}
		pod, err := oc.KubeClient().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create pod")

		g.By("Wait for pod to be ready")
		err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeClient(), pod)
		o.Expect(err).NotTo(o.HaveOccurred(), "pod did not become ready")

		g.By("Get pod's node name")
		podObj, err := oc.KubeClient().CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get pod")
		nodeName := podObj.Spec.NodeName
		o.Expect(nodeName).NotTo(o.BeEmpty(), "pod node name is empty")
		e2e.Logf("Pod is running on node: %s", nodeName)

		g.By("Get pod's network namespace path")
		netNsPath, err := nodeutils.GetPodNetNs(oc, nodeName, podName)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get pod NetNS")
		e2e.Logf("Pod NetNS path: %s", netNsPath)

		g.By("Verify NetNS file exists before pod deletion")
		_, err = nodeutils.ExecOnNodeWithChroot(oc, nodeName, "test", "-e", netNsPath)
		o.Expect(err).NotTo(o.HaveOccurred(), "NetNS file does not exist before pod deletion")

		g.By("Delete the pod")
		err = oc.KubeClient().CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to delete pod")

		g.By("Wait for pod to be fully deleted")
		err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 1*time.Minute, true, func(ctx context.Context) (bool, error) {
			_, pollErr := oc.KubeClient().CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if apierrors.IsNotFound(pollErr) {
				e2e.Logf("Pod deleted successfully")
				return true, nil
			}
			if pollErr != nil {
				e2e.Logf("Error checking pod deletion: %v", pollErr)
				return false, nil
			}
			e2e.Logf("Waiting for pod to be deleted")
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "pod was not deleted")

		g.By("Verify that the NetNS file has been cleaned up on the node")
		err = nodeutils.CheckNetNsCleaned(oc, nodeName, netNsPath)
		o.Expect(err).NotTo(o.HaveOccurred(), "NetNS file was not cleaned up")
	})
})

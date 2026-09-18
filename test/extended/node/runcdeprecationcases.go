package node

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	configv1 "github.com/openshift/api/config/v1"
	mcclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

// runc Deprecation Test Suite

var _ = g.Describe("[Jira:Node][sig-node] runc deprecation cases", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("runc-deprecation")

	g.BeforeEach(func(ctx context.Context) {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to check if cluster is MicroShift")
		if isMicroShift {
			g.Skip("Skipping runc deprecation tests on MicroShift")
		}

		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get control plane topology")
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("Skipping runc deprecation tests on Hypershift — MachineConfig API unavailable")
		}
	})

	// A cluster on RHCOS 9 should use crun as the default container runtime
	// with no custom ContainerRuntimeConfig present.
	g.It("RHCOS 9 cluster install should use crun as the default container runtime", func(ctx context.Context) {

		g.By("Checking ClusterVersion is Available and not Progressing")
		cv, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get ClusterVersion")
		var cvAvailable, cvProgressing bool
		for _, cond := range cv.Status.Conditions {
			switch cond.Type {
			case configv1.OperatorAvailable:
				cvAvailable = cond.Status == configv1.ConditionTrue
			case configv1.OperatorProgressing:
				cvProgressing = cond.Status == configv1.ConditionTrue
			}
		}
		o.Expect(cvAvailable).To(o.BeTrue(), "ClusterVersion should be Available")
		o.Expect(cvProgressing).To(o.BeFalse(), "ClusterVersion should not be Progressing")
		framework.Logf("ClusterVersion %s: Available=%v Progressing=%v",
			cv.Status.Desired.Version, cvAvailable, cvProgressing)

		g.By("Getting a worker node for inspection")
		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to list worker nodes")
		o.Expect(workerNodes).NotTo(o.BeEmpty(), "expected at least one worker node")
		targetNode := workerNodes[0].Name
		framework.Logf("Using worker node: %s", targetNode)

		g.By("Checking RHCOS 9 is reported in /etc/os-release on the worker node")
		osRelease, err := ExecOnNodeWithChroot(oc, targetNode, "cat", "/etc/os-release")
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to read /etc/os-release on node %s", targetNode)
		o.Expect(osRelease).To(o.ContainSubstring("VARIANT_ID=coreos"),
			"expected RHCOS (VARIANT_ID=coreos) on node %s", targetNode)
		o.Expect(osRelease).To(o.ContainSubstring(`VERSION_ID="9.`),
			"expected RHCOS 9 (VERSION_ID=\"9.x\") on node %s", targetNode)
		framework.Logf("Confirmed: node %s is running RHCOS 9", targetNode)

		g.By("Checking no ContainerRuntimeConfig resources exist")
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create MachineConfig client")
		ctrcfgList, err := mcClient.MachineconfigurationV1().ContainerRuntimeConfigs().List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to list ContainerRuntimeConfigs")
		o.Expect(ctrcfgList.Items).To(o.BeEmpty(),
			"expected no ContainerRuntimeConfigs on a fresh cluster, found %d item(s)", len(ctrcfgList.Items))
		framework.Logf("Confirmed: no custom ContainerRuntimeConfig exists")

		g.By("Checking CRI-O default_runtime is crun on the worker node")
		crioConfig, err := ExecOnNodeWithChroot(oc, targetNode, "crio", "status", "config")
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get CRI-O status config on node %s", targetNode)
		o.Expect(crioConfig).To(o.ContainSubstring(`default_runtime = "crun"`),
			"expected CRI-O default_runtime to be crun on node %s", targetNode)
		framework.Logf("Confirmed: CRI-O default_runtime is crun on node %s", targetNode)

		g.By("Checking machine-config ClusterOperator is Available and not Degraded")
		co, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(ctx, "machine-config", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get machine-config ClusterOperator")
		var mcAvailable, mcProgressing, mcDegraded bool
		for _, cond := range co.Status.Conditions {
			switch cond.Type {
			case configv1.OperatorAvailable:
				mcAvailable = cond.Status == configv1.ConditionTrue
			case configv1.OperatorProgressing:
				mcProgressing = cond.Status == configv1.ConditionTrue
			case configv1.OperatorDegraded:
				mcDegraded = cond.Status == configv1.ConditionTrue
			}
		}
		o.Expect(mcAvailable).To(o.BeTrue(), "machine-config ClusterOperator should be Available")
		o.Expect(mcProgressing).To(o.BeFalse(), "machine-config ClusterOperator should not be Progressing")
		o.Expect(mcDegraded).To(o.BeFalse(), "machine-config ClusterOperator should not be Degraded")
		framework.Logf("Confirmed: machine-config ClusterOperator is healthy (Available=true Progressing=false Degraded=false)")

		g.By("Creating a test pod with ubi9-minimal image")
		namespace := oc.Namespace()
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-fresh-crun",
				Namespace: namespace,
			},
			Spec: corev1.PodSpec{
				NodeSelector: map[string]string{
					"kubernetes.io/hostname": targetNode,
				},
				RestartPolicy: corev1.RestartPolicyNever,
				Containers: []corev1.Container{
					{
						Name:    "ubi-minimal",
						Image:   image.ShellImage(),
						Command: []string{"echo", "crun-fresh-install-test-passed"},
					},
				},
			},
		}
		_, err = oc.KubeClient().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create test pod")

		g.By("Waiting for the test pod to reach Succeeded phase")
		err = e2epod.WaitForPodSuccessInNamespaceTimeout(ctx, oc.KubeClient(), pod.Name, namespace, 2*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "test pod did not complete successfully with crun runtime")

		resultPod, err := oc.KubeClient().CoreV1().Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get test pod after completion")
		o.Expect(resultPod.Status.Phase).To(o.Equal(corev1.PodSucceeded),
			"test pod should have Succeeded, got phase: %s", resultPod.Status.Phase)
		framework.Logf("Test PASSED: RHCOS 9 cluster uses crun as default runtime and workloads run successfully")
	})
})

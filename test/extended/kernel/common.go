package kernel

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	// Kernel versions contain rt for realtime
	// Version Formats:
	// - 5.14.0-284.57.1.rt14.342.el9_2.x86_64
	// - 5.14.0-430.el9.x86_64+rt
	// Continue using regex to tighten the match for both versions
	realTimeKernelRE = regexp.MustCompile(".*[.+]rt.*")
	rtNamespace      = "ci-realtime-testbed"
	rtPodName        = "rt-tests"
)

func rtEnvFixture() string {
	return exutil.FixturePath("testdata", "kernel", "rt-tests-environment.yaml")
}

func rtPodFixture() string {
	return exutil.FixturePath("testdata", "kernel", "rt-tests-pod.yaml")
}

func failIfNotRT(oc *exutil.CLI) {
	g.By("checking kernel configuration")

	rtNodes, err := getRealTimeWorkerNodes(oc)
	o.Expect(err).NotTo(o.HaveOccurred(), "unable to retrieve realtime worker nodes")
	o.Expect(len(rtNodes)).NotTo(o.BeZero(), "no realtime nodes are configured")
}

func getRealTimeWorkerNodes(oc *exutil.CLI) (nodes map[string]int, err error) {
	nodes = make(map[string]int)

	kubeNodes, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/worker="})
	if err != nil {
		return nodes, err
	}

	for _, node := range kubeNodes.Items {
		if realTimeKernelRE.MatchString(node.Status.NodeInfo.KernelVersion) {
			nodes[node.Name] = 0
		}
	}

	return nodes, nil
}

// Setup the cluster infra needed for running RT tests
func configureRealtimeTestEnvironment(oc *exutil.CLI) {
	g.By("Setting up the privileged environment needed for realtime tests")
	err := oc.SetNamespace(rtNamespace).Run("apply").Args("-f", rtEnvFixture()).Execute()
	o.Expect(err).NotTo(o.HaveOccurred(), "unable to create namespace and service accounts for rt-tests")
}

// Tear down the infra setup we used for testing
func cleanupRealtimeTestEnvironment(oc *exutil.CLI) {
	g.By("Cleaning up the privileged environment needed for realtime tests")
	err := oc.SetNamespace(rtNamespace).Run("delete").Args("-f", rtEnvFixture()).Execute()
	o.Expect(err).NotTo(o.HaveOccurred(), "unable to clean up the namespace and service accounts for rt-tests")

	err = wait.PollImmediate(1*time.Second, 60*time.Second, func() (bool, error) {
		_, err := oc.AsAdmin().ProjectClient().ProjectV1().Projects().Get(context.Background(), rtNamespace, metav1.GetOptions{})

		if err != nil && apierrors.IsNotFound(err) {
			return true, nil
		}

		return false, err
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "timed out cleaning up the namespace and service accounts for rt-tests")
}

// Setup the pod that will be used to run the test
func startRtTestPod(oc *exutil.CLI) {
	g.By("Setting up the pod needed for realtime tests")
	err := oc.SetNamespace(rtNamespace).Run("apply").Args("-f", rtPodFixture()).Execute()
	o.Expect(err).NotTo(o.HaveOccurred(), "unable to create test pod for rt-tests")

	// Wait for the container to be ready to go
	_, err = exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(rtNamespace), labels.Everything(), exutil.CheckPodIsRunning, 1, 5*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred(), "test pod for rt-tests never became ready")
}

// Cleanup the pod used for the test
func cleanupRtTestPod(oc *exutil.CLI) {
	g.By("Cleaning up the pod needed for realtime tests")
	err := oc.SetNamespace(rtNamespace).Run("delete").Args("-f", rtPodFixture()).Execute()
	o.Expect(err).NotTo(o.HaveOccurred(), "unable to clean up test pod for rt-tests")

	// Wait for the container to be ready to go
	err = exutil.WaitForNoPodsRunning(oc.SetNamespace(rtNamespace))
	o.Expect(err).NotTo(o.HaveOccurred(), "test pod for rt-tests never became ready")
}

// Write out test artifacts
func writeTestArtifacts(fname string, content string) {
	// Create the artifact dir for rt-tests if it does not exist
	artifactDir := filepath.Join(exutil.ArtifactDirPath(), "rt-tests")
	err := os.MkdirAll(artifactDir, 0755)
	o.Expect(err).NotTo(o.HaveOccurred())

	err = os.WriteFile(filepath.Join(artifactDir, fname), []byte(content), 0644)
	o.Expect(err).NotTo(o.HaveOccurred())
}

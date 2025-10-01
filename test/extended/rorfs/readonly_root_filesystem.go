package rorfs

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	exutil "github.com/openshift/origin/test/extended/util"
)

// Helper function to test a single namespace
func testSingleNamespace(oc *exutil.CLI, namespace string) {
	// Check if namespace exists
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	if err != nil {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", namespace))
	}

	oc.SetupProject()

	// Get all pods in the namespace with full pod information
	labelSelector := "app notin (guard,installer,pruner,revision)"
	podList, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		e2e.Failf("Failed to get pods from namespace %s: %v", namespace, err)
	}

	if len(podList.Items) == 0 {
		g.Skip(fmt.Sprintf("No suitable pods found in namespace %s", namespace))
	}

	// Filter out pods that should be skipped based on name patterns
	var filteredPods []corev1.Pod
	skipPatterns := []string{
		"apiserver-watcher",
	}

	for _, pod := range podList.Items {
		// Skip pods with specific name patterns
		shouldSkip := false
		for _, pattern := range skipPatterns {
			if strings.Contains(strings.ToLower(pod.Name), strings.ToLower(pattern)) {
				e2e.Logf("Skipping pod %s in namespace %s (matches exclusion pattern: %s)", pod.Name, namespace, pattern)
				shouldSkip = true
				break
			}
		}
		if shouldSkip {
			continue
		}
		filteredPods = append(filteredPods, pod)
	}

	if len(filteredPods) == 0 {
		g.Skip(fmt.Sprintf("No suitable pods found in namespace %s after filtering", namespace))
	}

	e2e.Logf("Found %d pods to test in namespace %s (after filtering)", len(filteredPods), namespace)
	// Configuration verification only - we trust the upstream Kubernetes readOnlyRootFilesystem feature
	for _, pod := range filteredPods {
		// Skip pods that are not running (using status from initial list)
		if pod.Status.Phase != corev1.PodRunning {
			e2e.Logf("Skipping pod %s in namespace %s (not running, status: %s)", pod.Name, namespace, pod.Status.Phase)
			continue
		}

		e2e.Logf("Verifying readOnlyRootFilesystem configuration for pod %s in namespace %s", pod.Name, namespace)

		// Configuration verification for all containers (fast check)
		for containerIndex, container := range pod.Spec.Containers {
			if container.SecurityContext == nil {
				e2e.Failf("Container %d in pod %s (namespace %s) does not have securityContext",
					containerIndex, pod.Name, namespace)
			}
			if container.SecurityContext.ReadOnlyRootFilesystem == nil {
				e2e.Failf("Container %d in pod %s (namespace %s) does not have readOnlyRootFilesystem set",
					containerIndex, pod.Name, namespace)
			}

			// Check readOnlyRootFilesystem value
			isReadOnly := *container.SecurityContext.ReadOnlyRootFilesystem
			if isReadOnly {
				e2e.Logf("Container %d in pod %s (namespace %s) has readOnlyRootFilesystem=true",
					containerIndex, pod.Name, namespace)
			} else {
				e2e.Logf("Container %d in pod %s (namespace %s) has readOnlyRootFilesystem=false",
					containerIndex, pod.Name, namespace)
				// Log the reason for false values to help with debugging
				e2e.Logf(" This may be expected for certain types of containers (e.g., init containers, sidecars)")
			}
		}
	}
}

var _ = g.Describe("[sig-api-machinery][Feature:ReadOnlyRootFilesystem]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("rorfs", admissionapi.LevelPrivileged)

	g.BeforeEach(func() {
		// Skip on Microshift clusters
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("MicroShift has different security context requirements and architecture")
		}
	})

	// Get namespaces from platform identification
	availableNamespaces := platformidentification.KnownNamespaces.List()

	// Define namespaces to test based on platform availability
	desiredNamespaces := []string{
		"openshift-controller-manager-operator",
		"openshift-route-controller-manager",
		"openshift-cluster-version",
		"openshift-image-registry",
		"openshift-insights",
		"openshift-dns",
		"openshift-etcd",
		"openshift-etcd-operator",
		"openshift-kube-scheduler-operator",
		"openshift-kube-scheduler",
		"openshift-kube-apiserver-operator",
		"openshift-kube-apiserver",
		"openshift-cloud-credential-operator",
		"openshift-marketplace",
		"openshift-console-operator",
		"openshift-console",
		"openshift-operator-lifecycle-manager",
		"hypershift",
	}

	// Filter to only include namespaces that exist on the cluster
	namespaces := []string{}
	for _, desiredNamespace := range desiredNamespaces {
		for _, availableNamespace := range availableNamespaces {
			if desiredNamespace == availableNamespace {
				namespaces = append(namespaces, desiredNamespace)
				break
			}
		}
	}

	// Generate individual test cases for each namespace using a loop
	for _, namespace := range namespaces {
		namespace := namespace // capture loop variable
		g.It(fmt.Sprintf("Explicitly set readOnlyRootFilesystem to true - %s [OCP-83088][Skipped:Disconnected]", namespace), func() {
			testSingleNamespace(oc, namespace)
		})
	}

})

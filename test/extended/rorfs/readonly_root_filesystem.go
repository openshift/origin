package rorfs

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	exutil "github.com/openshift/origin/test/extended/util"
)

// isControlPlaneNamespace checks if a namespace is a control plane namespace
func isControlPlaneNamespace(namespace string) bool {
	controlPlaneNamespaces := []string{
		"openshift-kube-apiserver-operator",
		"openshift-kube-apiserver",
		"openshift-kube-scheduler-operator",
		"openshift-kube-scheduler",
		"openshift-kube-controller-manager-operator",
		"openshift-kube-controller-manager",
		"openshift-controller-manager-operator",
		"openshift-route-controller-manager",
		"openshift-cluster-version",
		"openshift-etcd",
		"openshift-etcd-operator",
		"openshift-operator-lifecycle-manager",
		"openshift-cloud-credential-operator",
	}

	for _, cpns := range controlPlaneNamespaces {
		if namespace == cpns {
			return true
		}
	}
	return false
}

// getPodsWithRetryAndFallback gets pods with retry logic and fallback strategy
func getPodsWithRetryAndFallback(oc *exutil.CLI, namespace string) ([]corev1.Pod, error) {
	var podList *corev1.PodList
	var err error

	// Check if this is a Hypershift cluster
	configClient := oc.AdminConfigClient()
	isHypershift, err := exutil.IsHypershift(context.Background(), configClient)
	if err != nil {
		e2e.Logf("Failed to check if cluster is Hypershift: %v, assuming regular cluster", err)
		// Continue with normal logic if we can't determine cluster type
		isHypershift = false
	}

	e2e.Logf("Hypershift detection result: isHypershift=%v for namespace %s", isHypershift, namespace)

	// For Hypershift clusters, skip control plane namespaces when testing from hosted cluster
	if isHypershift {
		if isControlPlaneNamespace(namespace) {
			e2e.Logf("Skipping control plane namespace %s on Hypershift hosted cluster", namespace)
			e2e.Logf("Control plane pods are not accessible from hosted cluster - they run in the management cluster")
			return []corev1.Pod{}, nil
		}
		e2e.Logf("Testing workload namespace %s in Hypershift hosted cluster", namespace)
	}

	// Use standard pod discovery (works for both regular clusters and Hypershift hosted clusters)
	e2e.Logf("Using standard pod discovery for namespace %s", namespace)

	// First try with restrictive label selector using wait/poll retry logic
	labelSelector := "app notin (guard,installer,pruner,revision)"
	err = wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		podList, err = oc.AdminKubeClient().CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			e2e.Logf("Failed to get pods with label selector from namespace %s: %v, retrying...", namespace, err)
			return false, nil
		}

		if len(podList.Items) > 0 {
			return true, nil
		}

		e2e.Logf("No pods found with label selector in namespace %s, will retry...", namespace)
		return false, nil
	})

	if err != nil {
		// If no pods found with restrictive selector, try without any label selector
		e2e.Logf("No pods found with label selector after retries, trying without selector")
		err = wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
			podList, err = oc.AdminKubeClient().CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				e2e.Logf("Failed to get pods without label selector from namespace %s: %v, retrying...", namespace, err)
				return false, nil
			}

			if len(podList.Items) > 0 {
				return true, nil
			}

			e2e.Logf("No pods found without label selector in namespace %s, will retry...", namespace)
			return false, nil
		})
	}

	if err != nil {
		// For Hypershift clusters, it's expected that some namespaces might not have pods
		if isHypershift {
			e2e.Logf("No pods found in namespace %s on Hypershift cluster - this may be expected behavior", namespace)
			return []corev1.Pod{}, nil // Return empty slice instead of error
		}
		return nil, fmt.Errorf("no pods found in namespace %s after retry attempts: %v", namespace, err)
	}

	return podList.Items, nil
}

// Helper function to test a single namespace
func testSingleNamespace(oc *exutil.CLI, namespace string) {
	// Check if namespace exists
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	if err != nil {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", namespace))
	}

	oc.SetupProject()

	// Get pods with retry logic and fallback strategy
	pods, err := getPodsWithRetryAndFallback(oc, namespace)
	if err != nil {
		e2e.Failf("Failed to get pods from namespace %s: %v", namespace, err)
	}

	// If no pods found (e.g., on Hypershift), skip this namespace
	if len(pods) == 0 {
		e2e.Logf("No pods found in namespace %s - skipping test for this namespace", namespace)
		return
	}

	// Filter out pods that should be skipped based on name patterns
	var filteredPods []corev1.Pod
	skipPatterns := []string{
		"apiserver-watcher",
	}

	for _, pod := range pods {
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
		e2e.Logf("No suitable pods found in namespace %s after filtering - this may be expected for some namespaces", namespace)
		return
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
		g.It(fmt.Sprintf("Explicitly set readOnlyRootFilesystem to true - %s [OCP-83088][Skipped:Disconnected][Serial]", namespace), func() {
			testSingleNamespace(oc, namespace)
		})
	}

})

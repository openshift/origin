package rorfs

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/pborman/uuid"

	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-api-machinery][Serial][Feature:ReadOnlyRootFilesystem]", func() {
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

	g.It("Explicitly set readOnlyRootFilesystem to true [OCP-83088][Timeout:50m][Skipped:Disconnected][Serial]", func() {
		var (
			randmStr = uuid.New()
			testNs1  = "test-rofs-cm-" + randmStr
			testNs2  = "test-rofs-rm-" + randmStr
		)
		// Define namespaces to test based on platform availability
		allNamespaces := []string{
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

		// Filter namespaces that actually exist in the cluster using kube client
		var namespaces []string
		for _, ns := range allNamespaces {
			_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(context.Background(), ns, metav1.GetOptions{})
			if err == nil {
				namespaces = append(namespaces, ns)
				e2e.Logf("Namespace %s exists, will be tested", ns)
			} else {
				e2e.Logf("Namespace %s does not exist, skipping", ns)
			}
		}

		if len(namespaces) == 0 {
			g.Skip("No testable namespaces found in this cluster")
		}

		oc.SetupProject()

		for _, ns := range namespaces {
			// Get pods using optimized function with label selector to exclude unwanted pods
			labelSelector := "app notin (guard,installer,pruner,revision)"
			podNames, err := exutil.GetPodsListByLabel(oc.AsAdmin(), ns, labelSelector)
			if err != nil {
				e2e.Failf("Failed to get pods from namespace %s: %v", ns, err)
			}

			if len(podNames) == 0 {
				e2e.Logf("No suitable pods found in namespace %s, skipping", ns)
				continue
			}

			// Filter out pods that should be skipped based on name patterns
			var filteredPodNames []string
			skipPatterns := []string{
				"apiserver-watcher",
			}

			for _, podName := range podNames {
				// Skip pods with specific name patterns
				shouldSkip := false
				for _, pattern := range skipPatterns {
					if strings.Contains(strings.ToLower(podName), strings.ToLower(pattern)) {
						e2e.Logf("Skipping pod %s in namespace %s (matches exclusion pattern: %s)", podName, ns, pattern)
						shouldSkip = true
						break
					}
				}
				if shouldSkip {
					continue
				}
				filteredPodNames = append(filteredPodNames, podName)
			}

			if len(filteredPodNames) == 0 {
				e2e.Logf("No suitable pods found in namespace %s after filtering, skipping", ns)
				continue
			}

			e2e.Logf("Found %d pods to test in namespace %s (after filtering)", len(filteredPodNames), ns)
			for _, podName := range filteredPodNames {
				err := exutil.AssertPodToBeReady(oc, podName, ns)
				if err != nil {
					e2e.Logf("Pod %s in namespace %s is not ready: %v, skipping", podName, ns, err)
					continue
				}

				e2e.Logf("Inspect the %s Pod's securityContext.", ns)
				pod, err := oc.AdminKubeClient().CoreV1().Pods(ns).Get(context.Background(), podName, metav1.GetOptions{})
				if err != nil {
					e2e.Failf("Failed to get pod %s details: %v", podName, err)
				}

				// Check ALL containers in the pod, not just the first one
				for i, container := range pod.Spec.Containers {
					if container.SecurityContext == nil {
						e2e.Failf("Container %d in pod %s (namespace %s) does not have securityContext",
							i, podName, ns)
					}
					if container.SecurityContext.ReadOnlyRootFilesystem == nil {
						e2e.Failf("Container %d in pod %s (namespace %s) does not have readOnlyRootFilesystem set",
							i, podName, ns)
					}

					// Check readOnlyRootFilesystem value
					isReadOnly := *container.SecurityContext.ReadOnlyRootFilesystem
					if isReadOnly {
						e2e.Logf("Container %d in pod %s (namespace %s) has readOnlyRootFilesystem=true",
							i, podName, ns)
					} else {
						e2e.Logf("Container %d in pod %s (namespace %s) has readOnlyRootFilesystem=false",
							i, podName, ns)
					}
				}

				// Test file creation in various restricted paths
				testPaths := []string{
					"/usr/local/bin/testfile",
					"/etc/testfile",
					"/usr/bin/testfile",
				}

				// Test write access for each container in the pod
				for containerIndex, container := range pod.Spec.Containers {
					if container.SecurityContext == nil || container.SecurityContext.ReadOnlyRootFilesystem == nil {
						continue
					}

					isReadOnly := *container.SecurityContext.ReadOnlyRootFilesystem
					e2e.Logf("Testing file system write access on container %d (%s) in pod %s (readOnlyRootFilesystem=%t)",
						containerIndex, container.Name, podName, isReadOnly)

					for _, testPath := range testPaths {
						// Test writing to specific container
						var out string
						var err error

						// If there's only one container, use default exec
						if len(pod.Spec.Containers) == 1 {
							out, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args(podName, "-n", ns, "--", "touch", testPath).Output()
						} else {
							// For multi-container pods, specify the container
							out, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args(podName, "-c", container.Name, "-n", ns, "--", "touch", testPath).Output()
						}

						// Log any exec errors for debugging
						if err != nil {
							e2e.Logf("Exec error for container %s: %v", container.Name, err)
						}

						readOnlyMsg := fmt.Sprintf("cannot touch '%s': Read-only file system", testPath)
						permissionMsg := fmt.Sprintf("cannot touch '%s': Permission denied", testPath)

						hasReadOnlyError := strings.Contains(out, readOnlyMsg)
						hasPermissionError := strings.Contains(out, permissionMsg)

						// Both readOnlyRootFilesystem=true and false pods should prevent writes
						o.Expect(hasReadOnlyError || hasPermissionError).To(o.BeTrue(),
							"container %s in pod %s (namespace %s, readOnlyRootFilesystem=%t) should not allow writing to %s. Got output: %s",
							container.Name, podName, ns, isReadOnly, testPath, out)
						e2e.Logf("Container %s correctly prevented writing to %s (readOnlyRootFilesystem=%t)",
							container.Name, testPath, isReadOnly)
					}
				}
			}
		}

		g.By("openshift-controller-manager: create Deployment, scale, verify")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("project", testNs1, "--ignore-not-found").Output()
		_, err := oc.AsAdmin().WithoutNamespace().Run("new-project").Args(testNs1).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "nginx",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: int32Ptr(1),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "nginx"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "nginx"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "nginx",
								Image: "quay.io/openshifttest/nginx-alpine@sha256:f78c5a93df8690a5a937a6803ef4554f5b6b1ef7af4f19a441383b8976304b4c",
							},
						},
					},
				},
			},
		}
		_, err = oc.AdminKubeClient().AppsV1().Deployments(testNs1).Create(context.Background(), deployment, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		deployment.Spec.Replicas = int32Ptr(3)
		_, err = oc.AdminKubeClient().AppsV1().Deployments(testNs1).Update(context.Background(), deployment, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Wait for pods to be Ready")
		podNames, err := exutil.GetPodsList(oc.AsAdmin(), testNs1)
		if err != nil {
			e2e.Failf("Failed to get pods from namespace %s: %v", testNs1, err)
		} else if len(podNames) > 0 {
			err = exutil.AssertPodToBeReady(oc, podNames[0], testNs1)
			if err != nil {
				e2e.Failf("Pod %s is not ready: %v", podNames[0], err)
			}
		}

		g.By("openshift-route-controller-manager: create Route, check")
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("project", testNs2, "--ignore-not-found").Output()
		_, err = oc.AsAdmin().WithoutNamespace().Run("new-project").Args(testNs2).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		deployment2 := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-app",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: int32Ptr(1),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "my-app"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "my-app"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "nginx",
								Image: "quay.io/openshifttest/nginx-alpine@sha256:f78c5a93df8690a5a937a6803ef4554f5b6b1ef7af4f19a441383b8976304b4c",
								Ports: []corev1.ContainerPort{
									{ContainerPort: 8080},
								},
							},
						},
					},
				},
			},
		}
		_, err = oc.AdminKubeClient().AppsV1().Deployments(testNs2).Create(context.Background(), deployment2, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-app",
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{"app": "my-app"},
				Ports: []corev1.ServicePort{
					{
						Port:       80,
						TargetPort: intstr.FromInt(8080),
					},
				},
			},
		}
		_, err = oc.AdminKubeClient().CoreV1().Services(testNs2).Create(context.Background(), service, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		route := &routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-app",
			},
			Spec: routev1.RouteSpec{
				To: routev1.RouteTargetReference{
					Kind: "Service",
					Name: "my-app",
				},
				Port: &routev1.RoutePort{
					TargetPort: intstr.FromInt(8080),
				},
			},
		}
		_, err = oc.AdminRouteClient().RouteV1().Routes(testNs2).Create(context.Background(), route, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Wait for deployment to be ready")
		o.Eventually(func() bool {
			deployment, err := oc.AdminKubeClient().AppsV1().Deployments(testNs2).Get(context.Background(), "my-app", metav1.GetOptions{})
			if err != nil {
				return false
			}
			return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas
		}, 2*time.Minute, 10*time.Second).Should(o.BeTrue(), "Deployment was not ready")

		g.By("Wait for pods to be ready")
		podNames, podListErr := exutil.GetPodsListByLabel(oc.AsAdmin(), testNs2, "app=my-app")
		if podListErr != nil {
			e2e.Failf("Failed to get pods: %v", podListErr)
		}

		for _, podName := range podNames {
			podErr := exutil.AssertPodToBeReady(oc, podName, testNs2)
			if podErr != nil {
				e2e.Failf("Pod %s is not ready: %v", podName, podErr)
			}
		}

		g.By("Wait for service endpoints to be ready")
		o.Eventually(func() bool {
			endpoints, err := oc.AdminKubeClient().CoreV1().Endpoints(testNs2).Get(context.Background(), "my-app", metav1.GetOptions{})
			if err != nil {
				e2e.Logf("Failed to get endpoints: %v", err)
				return false
			}
			if len(endpoints.Subsets) == 0 {
				e2e.Logf("No endpoint subsets found")
				return false
			}
			for _, subset := range endpoints.Subsets {
				if len(subset.Addresses) == 0 {
					e2e.Logf("No addresses in endpoint subset")
					return false
				}
				e2e.Logf("Found %d endpoint addresses", len(subset.Addresses))
			}
			e2e.Logf("Service endpoints are ready!")
			return true
		}, 2*time.Minute, 10*time.Second).Should(o.BeTrue(), "Service endpoints were not ready")

		g.By("Wait for Route & test")
		var routeHost string
		o.Eventually(func() string {
			route, err := oc.AdminRouteClient().RouteV1().Routes(testNs2).Get(context.Background(), "my-app", metav1.GetOptions{})
			if err != nil {
				return ""
			}
			routeHost = route.Spec.Host
			return route.Spec.Host
		}, 2*time.Minute, 10*time.Second).ShouldNot(o.BeEmpty(), "Route was not created")

		g.By("Attempt curl to the route")
		url := fmt.Sprintf("http://%s", routeHost)
		output, err := exutil.ClientCurl(oc, "GET", "", url)
		if err != nil {
			e2e.Logf("Route access failed: %v, trying pod-based fallback for OVN clusters", err)

			// Pod-based fallback for OVN clusters where external route access may be restricted
			g.By("Attempt pod-based fallback")
			podNames, podListErr := exutil.GetPodsListByLabel(oc.AsAdmin(), testNs2, "app=my-app")
			if podListErr != nil || len(podNames) == 0 {
				e2e.Failf("Failed to get pods for fallback: %v", podListErr)
			}

			podName := podNames[0]
			e2e.Logf("Testing application from within pod %s using route URL", podName)
			output, err = oc.AsAdmin().Run("exec").Args(podName, "-n", testNs2, "--", "curl", "-s", url).Output()
			if err != nil {
				e2e.Failf("Failed to access application via any method: route=%v, pod=%v", err, err)
			} else {
				e2e.Logf("Successfully accessed application from within pod")
			}
		} else {
			e2e.Logf("Successfully accessed route")
		}

		o.Expect(output).To(o.ContainSubstring("Hello-OpenShift"), "Nginx welcome page not reachable")
	})
})

// int32Ptr returns a pointer to an int32
func int32Ptr(i int32) *int32 {
	return &i
}

// testContainerWriteOperations tests various write operations in a specific container
func testContainerWriteOperations(oc *exutil.CLI, podName, containerName, namespace string, testPaths []string, isMultiContainer bool) error {
	writeCommands := []struct {
		name string
		cmd  []string
	}{
		{"touch", []string{"touch"}},
		{"echo_redirect", []string{"sh", "-c", "echo test >"}},
		{"mkdir", []string{"mkdir", "-p"}},
	}

	for _, testPath := range testPaths {
		for _, writeCmd := range writeCommands {
			var out string
			var err error
			var args []string

			// Build command arguments
			if isMultiContainer {
				args = []string{podName, "-c", containerName, "-n", namespace, "--"}
			} else {
				args = []string{podName, "-n", namespace, "--"}
			}

			// Add the specific write command
			switch writeCmd.name {
			case "touch":
				args = append(args, append(writeCmd.cmd, testPath)...)
			case "echo_redirect":
				args = append(args, writeCmd.cmd[0], writeCmd.cmd[1], writeCmd.cmd[2]+" "+testPath)
			case "mkdir":
				args = append(args, append(writeCmd.cmd, testPath+"-dir")...)
			}

			out, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args(args...).Output()
			if err != nil {
				e2e.Logf("Write command %s failed as expected for container %s: %v", writeCmd.name, containerName, err)
			}

			// Check for expected error messages
			readOnlyMsg := "Read-only file system"
			permissionMsg := "Permission denied"

			hasReadOnlyError := strings.Contains(out, readOnlyMsg)
			hasPermissionError := strings.Contains(out, permissionMsg)

			if !hasReadOnlyError && !hasPermissionError {
				return fmt.Errorf("container %s should not allow %s operation on %s. Got output: %s",
					containerName, writeCmd.name, testPath, out)
			}

			e2e.Logf("Container %s correctly blocked %s operation on %s", containerName, writeCmd.name, testPath)
		}
	}
	return nil
}

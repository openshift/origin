package apiserver

import (
	"context"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-auth][Feature:ControlPlaneSecurity]", func() {
	defer g.GinkgoRecover()
	ctx := context.Background()
	oc := exutil.NewCLIWithPodSecurityLevel("control-plane-security", admissionapi.LevelPrivileged)

	// Verifies that control plane containers have proper securityContext.privileged settings
	// This ensures the control plane components can perform necessary privileged operations
	// Related issues:
	// OCP-32383: Control plane security context verification
	//bug 1793694: Init container security context
	g.It("should have privileged securityContext for control plane init and main containers", g.Label("Size:S"), func() {
		// Skip on MicroShift clusters
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("MicroShift has different security context requirements and architecture")
		}

		// Skip on Hypershift clusters (control plane pods run in management cluster)
		isHyperShift, err := exutil.IsHypershift(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isHyperShift {
			g.Skip("Hypershift control plane pods are not accessible from hosted cluster")
		}

		checkItems := []struct {
			namespace            string
			containerName        string
			expectedHostPath     string
			expectHostNetwork    bool
			requireHostPathMount bool
		}{
			{
				namespace:            "openshift-kube-apiserver",
				containerName:        "kube-apiserver",
				expectedHostPath:     "/etc/kubernetes",
				expectHostNetwork:    true,
				requireHostPathMount: true,
			},
			{
				namespace:            "openshift-apiserver",
				containerName:        "openshift-apiserver",
				expectedHostPath:     "",
				expectHostNetwork:    false,
				requireHostPathMount: false,
			},
			{
				namespace:            "openshift-oauth-apiserver",
				containerName:        "oauth-apiserver",
				expectedHostPath:     "",
				expectHostNetwork:    false,
				requireHostPathMount: false,
			},
		}

		for _, checkItem := range checkItems {
			g.By("Getting pods in " + checkItem.namespace)
			e2e.Logf("Checking namespace: %s", checkItem.namespace)

			podList, err := oc.AdminKubeClient().CoreV1().Pods(checkItem.namespace).List(ctx, metav1.ListOptions{
				LabelSelector: "apiserver",
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(podList.Items).NotTo(o.BeEmpty(), "Expected to find at least one pod in %s", checkItem.namespace)

			pod := podList.Items[0]
			e2e.Logf("Found pod: %s in namespace %s", pod.Name, checkItem.namespace)

			g.By("Verifying container securityContext.privileged for " + checkItem.containerName)

			// Find the specified container
			var targetContainer *corev1.Container
			for i := range pod.Spec.Containers {
				if pod.Spec.Containers[i].Name == checkItem.containerName {
					targetContainer = &pod.Spec.Containers[i]
					break
				}
			}
			o.Expect(targetContainer).NotTo(o.BeNil(), "Container %s not found in pod %s", checkItem.containerName, pod.Name)

			// Verify the container has securityContext
			o.Expect(targetContainer.SecurityContext).NotTo(o.BeNil(),
				"Container %s in pod %s does not have securityContext", checkItem.containerName, pod.Name)

			o.Expect(targetContainer.SecurityContext.Privileged).NotTo(o.BeNil(),
				"Container %s in pod %s does not have securityContext.privileged set", checkItem.containerName, pod.Name)
			o.Expect(*targetContainer.SecurityContext.Privileged).To(o.BeTrue(),
				"Container %s in pod %s should have securityContext.privileged=true", checkItem.containerName, pod.Name)
			e2e.Logf("Container %s has securityContext.privileged=true", checkItem.containerName)

			var runAsUser *int64
			if targetContainer.SecurityContext.RunAsUser != nil {
				runAsUser = targetContainer.SecurityContext.RunAsUser
				e2e.Logf("Container %s has container-level runAsUser set", checkItem.containerName)
			} else if pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.RunAsUser != nil {
				runAsUser = pod.Spec.SecurityContext.RunAsUser
				e2e.Logf("Container %s inherits pod-level runAsUser", checkItem.containerName)
			}

			// If runAsUser is explicitly set (either at container or pod level), verify it's 0
			// If not set, the container runs as root by default when privileged=true
			if runAsUser != nil {
				o.Expect(*runAsUser).To(o.Equal(int64(0)),
					"Container %s in pod %s should have runAsUser=0 (root), got %d", checkItem.containerName, pod.Name, *runAsUser)
				e2e.Logf("Container %s has runAsUser=0 (root)", checkItem.containerName)
			} else {
				// When privileged=true and runAsUser is not set, container runs as root by default
				e2e.Logf("Container %s runs as root (privileged=true, runAsUser not explicitly set)", checkItem.containerName)
			}
			o.Expect(pod.Spec.HostNetwork).To(o.Equal(checkItem.expectHostNetwork),
				"Pod %s should have hostNetwork=%v", pod.Name, checkItem.expectHostNetwork)
			e2e.Logf("Pod %s has hostNetwork=%v", pod.Name, checkItem.expectHostNetwork)

			// Verify critical hostPath mounts (for static pods only)
			// Deployment-based API servers use ConfigMaps/Secrets instead of hostPath mounts
			if checkItem.requireHostPathMount {
				foundHostPath := false
				for _, volMount := range targetContainer.VolumeMounts {
					if strings.HasPrefix(volMount.MountPath, checkItem.expectedHostPath) {
						foundHostPath = true
						e2e.Logf("✓ Container %s mounts %s at %s", checkItem.containerName, checkItem.expectedHostPath, volMount.MountPath)
						break
					}
				}
				o.Expect(foundHostPath).To(o.BeTrue(),
					"Container %s in pod %s should mount %s hostPath", checkItem.containerName, pod.Name, checkItem.expectedHostPath)
			} else {
				e2e.Logf("Container %s is a deployment (uses ConfigMaps/Secrets, not hostPath)", checkItem.containerName)
			}

			g.By("Verifying init container securityContext.privileged")

			// Verify all init containers have privileged=true
			o.Expect(pod.Spec.InitContainers).NotTo(o.BeEmpty(),
				"Expected to find at least one init container in pod %s", pod.Name)

			for _, initContainer := range pod.Spec.InitContainers {
				o.Expect(initContainer.SecurityContext).NotTo(o.BeNil(),
					"Init container %s in pod %s does not have securityContext", initContainer.Name, pod.Name)
				o.Expect(initContainer.SecurityContext.Privileged).NotTo(o.BeNil(),
					"Init container %s in pod %s does not have securityContext.privileged set", initContainer.Name, pod.Name)
				o.Expect(*initContainer.SecurityContext.Privileged).To(o.BeTrue(),
					"Init container %s in pod %s should have securityContext.privileged=true", initContainer.Name, pod.Name)

				e2e.Logf("Init container %s has securityContext.privileged=true", initContainer.Name)
			}
		}
	})
})

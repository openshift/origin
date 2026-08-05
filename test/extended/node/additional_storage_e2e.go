package node

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
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/ptr"

	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	mcclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

// Additional Storage E2E Tests - trigger MCO reconciliation (MCP rollouts)
// and run in the disruptive-longrunning suite.
var _ = g.Describe("[Skipped:Disconnected][apigroup:config.openshift.io][apigroup:machineconfiguration.openshift.io][Jira:Node/CRI-O][sig-node][Feature:AdditionalStorageSupport][OCPFeatureGate:AdditionalStorageConfig][Serial][Disruptive][Suite:openshift/disruptive-longrunning] Additional Storage E2E Tests", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("additional-storage-e2e")

	g.BeforeEach(func(ctx context.Context) {
		SkipOnMicroShift(oc)

		isSingleNode, err := exutil.IsSingleNode(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isSingleNode {
			g.Skip("MCP rollouts don't work reliably on single-node clusters")
		}

		EnsureNodesReady(ctx, oc)

		enabled, reason := IsAdditionalStorageConfigEnabled(ctx, oc)
		if !enabled {
			g.Skip(reason)
		}
	})

	// This test verifies configuration for all three storage types (image, layer, artifact)
	// and performs functional testing for image stores (pre-population and fallback).
	// Artifact stores are tested with filesystem operations, and layer stores are verified
	// via storage.conf :ref suffix configuration.
	g.It("should verify configuration of all storage types and functionally test image stores", func(ctx context.Context) {
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		// SETUP: Create single-node MachineConfigPool for faster rollouts
		g.By("Getting worker node for test")
		testNode := GetFirstReadyWorkerNode(oc)

		mcpName := "combined-func-test"
		var mcpConfig *CustomMCPConfig

		g.By("Creating single-node MachineConfigPool for test isolation")
		mcpConfig, err = CreateCustomMCPForNode(ctx, oc, mcClient, mcpName, testNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Custom MCP %s created, targeting node %s", mcpName, testNode)

		testNamespace := oc.Namespace()

		// Phase 1: Setup directories and prepare for testing
		g.By("Phase 1: Creating storage directories and pre-populating test image")
		imageStorePath := "/var/lib/combined-imagestore"
		artifactStorePath := "/var/lib/combined-artifactstore"
		layerStorePath := "/var/lib/stargz-store/store"
		allDirs := []string{imageStorePath, artifactStorePath, layerStorePath}

		g.DeferCleanup(cleanupDirectoriesOnNode, oc, testNode, allDirs)
		g.DeferCleanup(func() {
			cleanupCtx := context.Background()
			if err := CleanupContainerRuntimeConfig(cleanupCtx, mcClient, "combined-func-test", mcpName); err != nil {
				framework.Logf("Warning: failed to cleanup ContainerRuntimeConfig: %v", err)
			}
			if mcpConfig != nil {
				if err := CleanupCustomMCP(cleanupCtx, mcpConfig); err != nil {
					framework.Logf("Warning: failed to cleanup custom MCP: %v", err)
				}
			}
		})

		err = createDirectoriesOnNode(oc, testNode, allDirs)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Pre-populate test image in image store
		testImage := "registry.k8s.io/e2e-test-images/agnhost:2.59"
		framework.Logf("Pre-populating image %s to %s on node %s", testImage, imageStorePath, testNode)

		_, err = ExecOnNodeWithChroot(ctx, oc, testNode, "podman", "--root", imageStorePath, "pull", testImage)
		o.Expect(err).NotTo(o.HaveOccurred())

		lsOutput, err := ExecOnNodeWithChroot(ctx, oc, testNode, "podman", "--root", imageStorePath, "images", "--format", "{{.Repository}}:{{.Tag}}")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(lsOutput).To(o.ContainSubstring(testImage))
		framework.Logf("Image pre-populated successfully: %s", lsOutput)

		// Ensure image is NOT in primary CRI-O store (cleanup from previous tests)
		framework.Logf("Checking if image exists in primary CRI-O store BEFORE configuring additional stores")
		imageIDOutput, err := ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "images", "-q", testImage)
		o.Expect(err).NotTo(o.HaveOccurred())

		if strings.TrimSpace(imageIDOutput) != "" {
			framework.Logf("Image %s found in primary CRI-O store (from previous tests), removing it", testImage)
			_, err = ExecOnNodeWithChroot(ctx, oc, testNode, "crictl", "rmi", strings.TrimSpace(imageIDOutput))
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("Image removed from primary CRI-O store")
		}
		framework.Logf("Verified: image %s is NOT in primary CRI-O store (only in %s)", testImage, imageStorePath)

		// Phase 2: Create ContainerRuntimeConfig with all three storage types
		g.By("Phase 2: Creating ContainerRuntimeConfig with all three storage types")
		ctrcfg := &machineconfigv1.ContainerRuntimeConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "combined-func-test",
			},
			Spec: machineconfigv1.ContainerRuntimeConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"machineconfiguration.openshift.io/pool": mcpName,
					},
				},
				ContainerRuntimeConfig: &machineconfigv1.ContainerRuntimeConfiguration{
					AdditionalLayerStores: []machineconfigv1.AdditionalLayerStore{
						{Path: machineconfigv1.StorePath(layerStorePath)},
					},
					AdditionalImageStores: []machineconfigv1.AdditionalImageStore{
						{Path: machineconfigv1.StorePath(imageStorePath)},
					},
					AdditionalArtifactStores: []machineconfigv1.AdditionalArtifactStore{
						{Path: machineconfigv1.StorePath(artifactStorePath)},
					},
				},
			},
		}

		g.By("Applying ContainerRuntimeConfig and waiting for MCP rollout")
		err = ApplyContainerRuntimeConfigAndWaitForMCP(ctx, mcClient, ctrcfg, mcpName, 15*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		// CONFIGURATION VERIFICATION - verify configs without usage

		// Phase 3: Verify storage configuration
		g.By("Phase 3: Verifying storage.conf and CRI-O configuration")
		storageConfOutput, err := ExecOnNodeWithChroot(ctx, oc, testNode, "cat", "/etc/containers/storage.conf")
		o.Expect(err).NotTo(o.HaveOccurred())

		// Verify image stores
		o.Expect(storageConfOutput).To(o.ContainSubstring(imageStorePath))
		framework.Logf("Image stores verified in storage.conf")

		// Verify layer stores with :ref suffix (MCO automatically appends :ref)
		expectedPathWithRef := layerStorePath + ":ref"
		o.Expect(storageConfOutput).To(o.ContainSubstring("additionallayerstores"),
			"storage.conf should contain additionallayerstores on node %s", testNode)
		o.Expect(storageConfOutput).To(o.ContainSubstring(expectedPathWithRef),
			"storage.conf should contain %s with :ref suffix on node %s", expectedPathWithRef, testNode)
		framework.Logf("Layer stores verified in storage.conf with path %s (MCO added :ref)", expectedPathWithRef)

		g.By("Verifying CRI-O config contains artifact stores")
		crioOutput, err := ExecOnNodeWithChroot(ctx, oc, testNode, "cat", "/etc/crio/crio.conf.d/01-ctrcfg-additionalArtifactStores")
		o.Expect(err).NotTo(o.HaveOccurred())

		expectedArtifactConfig := fmt.Sprintf(`additional_artifact_stores = ["%s"]`, artifactStorePath)
		o.Expect(crioOutput).To(o.ContainSubstring(expectedArtifactConfig))
		framework.Logf("Artifact stores verified in CRI-O config")

		g.By("Verifying CRI-O is active with new configuration")
		crioStatus, err := ExecOnNodeWithChroot(ctx, oc, testNode, "systemctl", "is-active", "crio")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.TrimSpace(crioStatus)).To(o.Equal("active"))
		framework.Logf("CRI-O is active")

		// FUNCTIONAL VERIFICATION - actually use the stores

		// Phase 4: Test image store functionality - prepopulation and fallback
		// Test prepopulated image by creating a pod
		// Use ImagePullPolicy: Never to force using only local stores (proves additional store is used)
		g.By("Phase 4: Creating pod using prepopulated image from additional store")
		testPod1 := createTestPod("imagestore-prepop-pod", testNamespace, testImage, testNode, corev1.PullNever)
		startTime1 := time.Now()
		_ = CreatePodAndWaitForRunning(ctx, oc, testPod1)
		pod1Time := time.Since(startTime1)
		framework.Logf("Pod using prepopulated image started in %v", pod1Time)

		// Test fallback to registry
		g.By("Phase 5: Testing fallback to registry when image removed from additional store")
		err = e2epod.DeletePodWithWaitByName(ctx, oc.AdminKubeClient(), testPod1.Name, testNamespace)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Remove image from additional store
		_, err = ExecOnNodeWithChroot(ctx, oc, testNode, "podman", "--root", imageStorePath, "rmi", testImage)
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Removed image from additional store to test fallback")

		// Create second pod - should fall back to registry
		// Use ImagePullPolicy: IfNotPresent to test actual CRI-O fallback path
		// CRI-O checks local stores (empty), then falls back to registry
		testPod2 := createTestPod("imagestore-fallback-pod", testNamespace, testImage, testNode, corev1.PullIfNotPresent)
		g.DeferCleanup(func() {
			err := e2epod.DeletePodWithWaitByName(context.Background(), oc.AdminKubeClient(), testPod2.Name, testNamespace)
			if err != nil {
				framework.Logf("Warning: failed to cleanup pod %s: %v", testPod2.Name, err)
			}
		})
		startTime2 := time.Now()
		_ = CreatePodAndWaitForRunning(ctx, oc, testPod2)
		pod2Time := time.Since(startTime2)
		framework.Logf("Pod using registry fallback started in %v", pod2Time)

		// Verify pod pulled from registry
		err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
			events, err := oc.AdminKubeClient().CoreV1().Events(testNamespace).List(ctx, metav1.ListOptions{
				FieldSelector: fmt.Sprintf("involvedObject.name=%s", testPod2.Name),
			})
			if err != nil {
				return false, err
			}
			for _, event := range events.Items {
				if event.Reason == "Pulled" && strings.Contains(event.Message, "Successfully pulled") {
					framework.Logf("SUCCESS: Image pulled from registry - %s", event.Message)
					return true, nil
				}
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("Phase 5 PASSED: Fallback to registry works when image not in additional store")

		framework.Logf("Test PASSED: Configuration verified for all storage types and image stores functionally tested")
	})
})

func createTestPod(name, namespace, image, nodeName string, pullPolicy corev1.PullPolicy) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Tolerations: []corev1.Toleration{
				{
					Operator: corev1.TolerationOpExists,
				},
			},
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:    ptr.To[int64](1000),
				RunAsNonRoot: ptr.To(true),
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
			Containers: []corev1.Container{
				{
					Name:            "test",
					Image:           image,
					Command:         []string{"/agnhost", "pause"},
					ImagePullPolicy: pullPolicy,
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: ptr.To(false),
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
						},
						RunAsNonRoot: ptr.To(true),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
}

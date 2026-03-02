package internalreleaseimage

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/utils/ptr"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	IRIResourceName = "cluster"
	iriRegistry     = 22625
)

var _ = g.Describe("[sig-installer][Feature:NoRegistryClusterInstall][Serial] InternalReleaseImage", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("no-registry")

	g.BeforeEach(func() {
		skipIfNoRegistryFeatureNotEnabled(oc)
	})

	g.It("have valid resource and status [apigroup:machineconfiguration.openshift.io]", func() {
		mcClient := oc.MachineConfigurationClient().MachineconfigurationV1alpha1()

		g.By("Verifying exactly one IRI resource exists cluster-wide")
		iriList, err := mcClient.InternalReleaseImages().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list InternalReleaseImage resources")
		o.Expect(iriList.Items).Should(o.HaveLen(1))

		g.By("Getting the InternalReleaseImage resource")
		iri, err := mcClient.InternalReleaseImages().Get(context.Background(), IRIResourceName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get InternalReleaseImage resource")

		g.By("Verifying resource is properly configured")
		o.Expect(iri.Name).Should(o.Equal(IRIResourceName),
			"IRI resource should have name %s", IRIResourceName)
		o.Expect(iri.Namespace).Should(o.BeEmpty())
		o.Expect(iri.Spec.Releases).ShouldNot(o.BeEmpty())

		g.By("Verifying status.releases is properly populated")
		o.Expect(iri.Status.Releases).NotTo(o.BeNil())
		o.Expect(iri.Status.Releases).ShouldNot(o.BeEmpty())

		e2e.Logf("IRI resource exists with %d spec releases and %d status releases",
			len(iri.Spec.Releases), len(iri.Status.Releases))

		g.By("Verifying all spec releases appear in status")
		for _, specRelease := range iri.Spec.Releases {
			found := false
			for _, statusRelease := range iri.Status.Releases {
				if statusRelease.Name == specRelease.Name {
					found = true
					break
				}
			}
			o.Expect(found).To(o.BeTrue())
		}
		e2e.Logf("All spec releases are present in status")

		g.By("Validating each release status")
		for i, release := range iri.Status.Releases {
			// Verify required fields are populated
			o.Expect(release.Name).ShouldNot(o.BeEmpty())
			o.Expect(release.Image).ShouldNot(o.BeEmpty())

			// Verify valid SHA256 digest
			o.Expect(release.Image).Should(o.ContainSubstring("@sha256:"),
				"Release %s image should contain @sha256: digest", release.Name)
			digest := extractImageDigest(release.Image)
			o.Expect(digest).Should(o.HaveLen(64))

			// Verify conditions are populated
			o.Expect(release.Conditions).NotTo(o.BeNil())
			o.Expect(release.Conditions).ShouldNot(o.BeEmpty())

			// Verify Available condition
			availableCond := findIRICondition(release.Conditions, "Available")
			o.Expect(availableCond).NotTo(o.BeNil())
			o.Expect(availableCond.Status).Should(o.Equal(metav1.ConditionTrue),
				"Release %s should have Available=True", release.Name)
			o.Expect(availableCond.Reason).Should(o.Equal("Installed"),
				"Release %s Available condition should have Reason=Installed", release.Name)
			o.Expect(availableCond.Message).Should(o.ContainSubstring("Release bundle is available"))

			// Check for Progressing condition if present
			progressingCond := findIRICondition(release.Conditions, "Progressing")
			if progressingCond != nil {
				o.Expect(progressingCond.LastTransitionTime.IsZero()).To(o.BeFalse(),
					"Release %s Progressing condition should have lastTransitionTime set", release.Name)
				e2e.Logf("Release %s: Progressing=%s, Reason=%s",
					release.Name, progressingCond.Status, progressingCond.Reason)
			}

			// Check for Degraded condition if present
			degradedCond := findIRICondition(release.Conditions, "Degraded")
			if degradedCond != nil {
				if degradedCond.Status == metav1.ConditionTrue {
					o.Expect(degradedCond.Reason).ShouldNot(o.BeEmpty())
					o.Expect(degradedCond.Message).ShouldNot(o.BeEmpty())
					e2e.Logf("Release %s is degraded: Reason=%s", release.Name, degradedCond.Reason)
				}
			}

			e2e.Logf("Release[%d] %s: valid with %d conditions, digest=%s...",
				i, release.Name, len(release.Conditions), digest[:16])
		}

		g.By("Verifying at least one release matches cluster version")
		cv, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(
			context.Background(), "version", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get ClusterVersion resource")

		clusterVersion := cv.Status.Desired.Version
		o.Expect(clusterVersion).ShouldNot(o.BeEmpty(), "ClusterVersion should have a desired version")

		versionParts := strings.Split(clusterVersion, ".")
		o.Expect(len(versionParts)).Should(o.BeNumerically(">=", 2),
			"Cluster version should have at least major.minor format")

		majorMinor := fmt.Sprintf("%s.%s", versionParts[0], versionParts[1])
		foundMatch := false
		for _, release := range iri.Status.Releases {
			if strings.Contains(release.Name, majorMinor) {
				foundMatch = true
				e2e.Logf("Found release matching cluster version %s: %s", majorMinor, release.Name)
				break
			}
		}
		o.Expect(foundMatch).To(o.BeTrue())

		g.By("Verifying IRI controller created MachineConfigs with proper ownership")
		iriMCs, err := getIRIMachineConfigs(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to list MachineConfigs")
		o.Expect(iriMCs).ShouldNot(o.BeEmpty(), "IRI should have created MachineConfigs")

		mcClientV1 := oc.MachineConfigurationClient().MachineconfigurationV1()
		for _, mcName := range iriMCs {
			mc, err := mcClientV1.MachineConfigs().Get(
				context.Background(), mcName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "MachineConfig %s should exist", mcName)
			o.Expect(mc.OwnerReferences).ShouldNot(o.BeEmpty(),
				"MachineConfig %s should have owner references", mcName)
			o.Expect(mc.OwnerReferences[0].Kind).Should(o.Equal("InternalReleaseImage"),
				"MachineConfig %s should be owned by InternalReleaseImage", mcName)
		}
		e2e.Logf("Verified %d MachineConfigs with IRI owner references", len(iriMCs))

		e2e.Logf("Status validation complete: all %d releases are valid and healthy", len(iri.Status.Releases))
	})

	g.It("restore deleted MachineConfigs [Disruptive][apigroup:machineconfiguration.openshift.io]", func() {
		mcClient := oc.MachineConfigurationClient().MachineconfigurationV1()

		g.By("Getting all IRI-owned MachineConfigs")
		iriMCs, err := getIRIMachineConfigs(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get IRI MachineConfigs")
		o.Expect(iriMCs).ShouldNot(o.BeEmpty())

		originalCount := len(iriMCs)
		e2e.Logf("Found %d IRI MachineConfigs to test reconciliation", originalCount)

		g.By(fmt.Sprintf("Deleting all %d IRI MachineConfigs", originalCount))
		for _, mcName := range iriMCs {
			err := mcClient.MachineConfigs().Delete(context.Background(), mcName, metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(),
				"Failed to delete MachineConfig %s", mcName)
			e2e.Logf("Deleted MachineConfig: %s", mcName)
		}

		g.By("Waiting for IRI controller to restore deleted MachineConfigs")
		err = wait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 5*time.Minute, true,
			func(_ context.Context) (bool, error) {
				restored, err := getIRIMachineConfigs(oc)
				if err != nil {
					return false, err
				}
				e2e.Logf("Reconciliation progress: %d/%d MachineConfigs restored", len(restored), originalCount)
				return len(restored) == originalCount, nil
			})
		o.Expect(err).NotTo(o.HaveOccurred(),
			"IRI controller should restore all %d MachineConfigs within 5 minutes", originalCount)

		e2e.Logf("Successfully verified restoration of all %d MachineConfigs", originalCount)
	})

	g.It("prevent deletion when in use [Disruptive][apigroup:machineconfiguration.openshift.io]", func() {
		mcClient := oc.MachineConfigurationClient().MachineconfigurationV1alpha1()

		g.By("Getting the InternalReleaseImage resource")
		iri, err := mcClient.InternalReleaseImages().Get(context.Background(), IRIResourceName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get InternalReleaseImage resource")
		o.Expect(iri.Status.Releases).ShouldNot(o.BeEmpty())

		g.By("Getting current cluster release image")
		cv, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(
			context.Background(), "version", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get cluster version")

		clusterReleaseImage := cv.Status.Desired.Image
		o.Expect(clusterReleaseImage).ShouldNot(o.BeEmpty())
		e2e.Logf("Cluster using release image: %s", clusterReleaseImage)

		g.By("Verifying at least one IRI release matches cluster release")
		foundMatch := false
		for _, release := range iri.Status.Releases {
			clusterDigest := extractImageDigest(clusterReleaseImage)
			releaseDigest := extractImageDigest(release.Image)

			if clusterDigest != "" && releaseDigest != "" && clusterDigest == releaseDigest {
				foundMatch = true
				e2e.Logf("Found matching release: %s (digest: %s...)", release.Name, releaseDigest[:16])
				break
			}
			if release.Image == clusterReleaseImage {
				foundMatch = true
				e2e.Logf("Found matching release: %s", release.Name)
				break
			}
		}
		o.Expect(foundMatch).To(o.BeTrue())

		g.By("Attempting to delete IRI resource (should fail)")
		err = mcClient.InternalReleaseImages().Delete(context.Background(), IRIResourceName, metav1.DeleteOptions{})
		o.Expect(err).To(o.HaveOccurred())

		g.By("Verifying deletion was blocked by ValidatingAdmissionPolicy")
		o.Expect(apierrors.IsInvalid(err)).To(o.BeTrue())

		o.Expect(err.Error()).Should(o.ContainSubstring("Cannot delete InternalReleaseImage"))

		e2e.Logf("Deletion correctly blocked: %v", err)

		g.By("Verifying IRI still exists after failed deletion")
		iri, err = mcClient.InternalReleaseImages().Get(context.Background(), IRIResourceName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(),
			"IRI should still exist after deletion was blocked")
		o.Expect(iri).NotTo(o.BeNil())

		e2e.Logf("IRI resource still exists as expected")
	})

	g.It("maintain high availability of registry across master nodes [Disruptive][apigroup:machineconfiguration.openshift.io]", func() {
		clientset, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Getting all master nodes")
		masterNodes, err := clientset.CoreV1().Nodes().List(context.Background(),
			metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master"})
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list master nodes")
		o.Expect(len(masterNodes.Items)).Should(o.BeNumerically(">=", 3),
			"This test requires at least 3 master nodes for HA testing, found %d", len(masterNodes.Items))

		masterIPs := make([]string, 0, len(masterNodes.Items))
		for _, node := range masterNodes.Items {
			for _, addr := range node.Status.Addresses {
				if addr.Type == corev1.NodeInternalIP {
					masterIPs = append(masterIPs, addr.Address)
					break
				}
			}
		}
		e2e.Logf("Testing HA across %d master nodes: %v", len(masterIPs), masterIPs)

		g.By("Verifying registry is accessible on all master nodes initially")
		for i, ip := range masterIPs {
			accessible := checkIRIRegistryAccessible(ip)
			o.Expect(accessible).To(o.BeTrue())
			e2e.Logf("Registry accessible on master node %d: %s", i+1, ip)
		}

		g.By("Selecting first master node for registry disruption test")
		targetNode := masterNodes.Items[0]
		targetIP := masterIPs[0]
		e2e.Logf("Target node for disruption: %s (IP: %s)", targetNode.Name, targetIP)

		g.By("Stopping registry service on target master node")
		_, err = oc.AsAdmin().Run("debug").Args(
			fmt.Sprintf("node/%s", targetNode.Name),
			"--",
			"chroot", "/host", "systemctl", "stop", "iri-registry",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to stop registry on node %s", targetNode.Name)
		e2e.Logf("Registry service stopped on %s", targetNode.Name)

		defer func() {
			g.By("Restoring registry service on target node")
			_, err := oc.AsAdmin().Run("debug").Args(
				fmt.Sprintf("node/%s", targetNode.Name),
				"--",
				"chroot", "/host", "systemctl", "start", "iri-registry",
			).Output()
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to restart registry on node %s", targetNode.Name)
			e2e.Logf("Registry service restored on %s", targetNode.Name)
		}()

		g.By("Waiting for registry to be unavailable on target node")
		err = wait.PollUntilContextTimeout(context.Background(), 2*time.Second, 30*time.Second, true,
			func(context.Context) (bool, error) {
				return !checkIRIRegistryAccessible(targetIP), nil
			})
		o.Expect(err).NotTo(o.HaveOccurred(), "Target node registry should become unavailable after service stop")

		g.By("Verifying registry is still accessible on other master nodes")
		for i := 1; i < len(masterIPs); i++ {
			accessible := checkIRIRegistryAccessible(masterIPs[i])
			o.Expect(accessible).To(o.BeTrue())
			e2e.Logf("HA verified: Registry still accessible on master node %d: %s", i+1, masterIPs[i])
		}

		e2e.Logf("High availability verified: registry accessible on %d/%d nodes with 1 node down",
			len(masterIPs)-1, len(masterIPs))
	})

	g.It("allow workloads to pull images from internal registry [apigroup:machineconfiguration.openshift.io]", func() {
		clientset, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())

		mcClient := oc.MachineConfigurationClient().MachineconfigurationV1alpha1()

		g.By("Getting the InternalReleaseImage resource")
		iri, err := mcClient.InternalReleaseImages().Get(context.Background(), IRIResourceName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get InternalReleaseImage resource")
		o.Expect(iri.Status.Releases).ShouldNot(o.BeEmpty(), "IRI should have releases")

		g.By("Selecting an image from the release payload")
		releaseImage := iri.Status.Releases[0].Image
		o.Expect(releaseImage).ShouldNot(o.BeEmpty(), "Release image should not be empty")
		e2e.Logf("Using release image: %s", releaseImage)

		g.By("Creating a test namespace for workload")
		testNS := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "iri-workload-test",
			},
		}
		_, err = clientset.CoreV1().Namespaces().Create(context.Background(), testNS, metav1.CreateOptions{})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create test namespace")
		}

		defer func() {
			g.By("Cleaning up test namespace")
			err := clientset.CoreV1().Namespaces().Delete(context.Background(), testNS.Name, metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				e2e.Logf("Warning: failed to delete test namespace: %v", err)
			}
		}()

		g.By("Creating a pod that pulls from the internal registry")
		testPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "iri-registry-test-pod",
				Namespace: testNS.Name,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:    "test-container",
						Image:   releaseImage,
						Command: []string{"sleep", "30"},
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

		_, err = clientset.CoreV1().Pods(testNS.Name).Create(context.Background(), testPod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create test pod")
		e2e.Logf("Created test pod: %s/%s", testNS.Name, testPod.Name)

		g.By("Waiting for pod to pull image and start")
		err = wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 3*time.Minute, true,
			func(ctx context.Context) (bool, error) {
				pod, err := clientset.CoreV1().Pods(testNS.Name).Get(ctx, testPod.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}

				if pod.Status.Phase == corev1.PodFailed {
					return false, fmt.Errorf("pod failed: %s", pod.Status.Message)
				}

				for _, containerStatus := range pod.Status.ContainerStatuses {
					if containerStatus.State.Waiting != nil {
						if strings.Contains(containerStatus.State.Waiting.Reason, "ImagePullBackOff") ||
							strings.Contains(containerStatus.State.Waiting.Reason, "ErrImagePull") {
							return false, fmt.Errorf("image pull failed: %s - %s",
								containerStatus.State.Waiting.Reason,
								containerStatus.State.Waiting.Message)
						}
						e2e.Logf("Pod waiting: %s", containerStatus.State.Waiting.Reason)
						return false, nil
					}
					if containerStatus.State.Running != nil || containerStatus.State.Terminated != nil {
						e2e.Logf("Container started successfully, image pulled from internal registry")
						return true, nil
					}
				}

				e2e.Logf("Pod phase: %s, waiting for container to start", pod.Status.Phase)
				return false, nil
			})
		o.Expect(err).NotTo(o.HaveOccurred(),
			"Pod should successfully pull image from internal registry and start")

		g.By("Verifying pod pulled image successfully")
		pod, err := clientset.CoreV1().Pods(testNS.Name).Get(context.Background(), testPod.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get pod status")

		imagePulled := false
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.ImageID != "" {
				imagePulled = true
				e2e.Logf("Container image pulled: %s", containerStatus.ImageID)
			}
		}
		o.Expect(imagePulled).To(o.BeTrue(), "Container should have pulled image from internal registry")

		e2e.Logf("Workload successfully pulled and ran image from internal IRI registry")
	})

	g.It("allow new worker nodes to connect to registry on port 22625 [Disruptive][Slow][apigroup:machineconfiguration.openshift.io]", func() {
		clientset, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Selecting a MachineSet to scale")
		machineSetName, err := oc.AsAdmin().Run("get").Args(
			"machinesets",
			"-n", "openshift-machine-api",
			"-o", "jsonpath={.items[0].metadata.name}",
		).Output()
		if err != nil || strings.TrimSpace(machineSetName) == "" {
			g.Skip("Cluster does not support MachineSet scaling (may be bare metal or single node)")
		}
		e2e.Logf("Selected MachineSet: %s", machineSetName)

		g.By("Getting current worker count")
		workerNodes, err := clientset.CoreV1().Nodes().List(context.Background(),
			metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/worker"})
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list worker nodes")
		initialWorkerCount := len(workerNodes.Items)
		e2e.Logf("Initial worker count: %d", initialWorkerCount)

		g.By("Getting current MachineSet replica count")
		currentReplicasStr, err := oc.AsAdmin().Run("get").Args(
			"machineset", machineSetName,
			"-n", "openshift-machine-api",
			"-o", "jsonpath={.spec.replicas}",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get MachineSet replicas")

		currentReplicas, err := strconv.Atoi(strings.TrimSpace(currentReplicasStr))
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to parse replica count: %s", currentReplicasStr)
		e2e.Logf("Current MachineSet replicas: %d", currentReplicas)

		newReplicaCount := currentReplicas + 1
		g.By(fmt.Sprintf("Scaling MachineSet from %d to %d replicas", currentReplicas, newReplicaCount))
		_, err = oc.AsAdmin().Run("scale").Args(
			"machineset", machineSetName,
			"-n", "openshift-machine-api",
			fmt.Sprintf("--replicas=%d", newReplicaCount),
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to scale MachineSet")
		e2e.Logf("Scaled MachineSet %s to %d replicas", machineSetName, newReplicaCount)

		defer func() {
			g.By(fmt.Sprintf("Scaling MachineSet back to original count (%d replicas)", currentReplicas))
			_, err := oc.AsAdmin().Run("scale").Args(
				"machineset", machineSetName,
				"-n", "openshift-machine-api",
				fmt.Sprintf("--replicas=%d", currentReplicas),
			).Output()
			if err != nil {
				e2e.Logf("Warning: failed to scale MachineSet back: %v", err)
			} else {
				e2e.Logf("Scaled MachineSet back to %d replicas", currentReplicas)
			}
		}()

		g.By("Waiting for new worker node to join and become Ready")
		var newNode *corev1.Node
		err = wait.PollUntilContextTimeout(context.Background(), 30*time.Second, 15*time.Minute, true,
			func(ctx context.Context) (bool, error) {
				nodes, err := clientset.CoreV1().Nodes().List(ctx,
					metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/worker"})
				if err != nil {
					return false, err
				}

				if len(nodes.Items) <= initialWorkerCount {
					e2e.Logf("Waiting for new worker node to appear (%d/%d)", len(nodes.Items), initialWorkerCount+1)
					return false, nil
				}

				// Find the newest node
				for i := range nodes.Items {
					node := &nodes.Items[i]
					isNew := true
					for _, oldNode := range workerNodes.Items {
						if node.Name == oldNode.Name {
							isNew = false
							break
						}
					}
					if isNew {
						// Check if node is Ready
						for _, condition := range node.Status.Conditions {
							if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
								newNode = node
								e2e.Logf("New worker node %s is Ready", node.Name)
								return true, nil
							}
						}
						e2e.Logf("New worker node %s found but not Ready yet", node.Name)
						return false, nil
					}
				}

				return false, nil
			})
		o.Expect(err).NotTo(o.HaveOccurred(), "New worker node should join cluster and become Ready within 15 minutes")
		o.Expect(newNode).NotTo(o.BeNil(), "New worker node should be identified")

		g.By(fmt.Sprintf("Verifying new worker node %s can access IRI registry", newNode.Name))
		nodeReady := false
		for _, condition := range newNode.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				nodeReady = true
				break
			}
		}
		o.Expect(nodeReady).To(o.BeTrue(), "New worker node should be Ready, indicating successful registry access")

		e2e.Logf("New worker node %s successfully connected to IRI registry on port %d", newNode.Name, iriRegistry)
	})
})

// Helper functions

func skipIfNoRegistryFeatureNotEnabled(oc *exutil.CLI) {
	g.By("Checking if NoRegistryClusterInstall feature is enabled")

	featureGate, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(
		context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		g.Skip(fmt.Sprintf("Failed to get FeatureGate: %v", err))
	}

	enabled := false
	if featureGate.Status.FeatureGates != nil {
		for _, fg := range featureGate.Status.FeatureGates {
			for _, feature := range fg.Enabled {
				if feature.Name == "NoRegistryClusterInstall" {
					enabled = true
					break
				}
			}
		}
	}

	if !enabled {
		g.Skip("NoRegistryClusterInstall feature gate is not enabled")
	}
}

func findIRICondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}

func extractImageDigest(imageRef string) string {
	parts := strings.Split(imageRef, "@")
	if len(parts) == 2 {
		return strings.TrimPrefix(parts[1], "sha256:")
	}
	return ""
}

func getIRIMachineConfigs(oc *exutil.CLI) ([]string, error) {
	mcClient := oc.MachineConfigurationClient().MachineconfigurationV1()
	mcList, err := mcClient.MachineConfigs().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var iriMCs []string
	for _, mc := range mcList.Items {
		for _, ownerRef := range mc.OwnerReferences {
			if ownerRef.Kind == "InternalReleaseImage" && ownerRef.Name == IRIResourceName {
				iriMCs = append(iriMCs, mc.Name)
				break
			}
		}
	}
	return iriMCs, nil
}

func checkIRIRegistryAccessible(ip string) bool {
	registryURL := fmt.Sprintf("https://%s:%d/v2/", ip, iriRegistry)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS12,
			},
		},
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, registryURL, nil)
	if err != nil {
		e2e.Logf("Failed to create request for %s: %v", registryURL, err)
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		e2e.Logf("Registry not accessible at %s: %v", registryURL, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		e2e.Logf("Registry responded with unexpected status %d at %s", resp.StatusCode, registryURL)
		return false
	}

	apiVersion := resp.Header.Get("Docker-Distribution-Api-Version")
	if apiVersion != "registry/2.0" {
		e2e.Logf("Registry API version mismatch at %s: %s", registryURL, apiVersion)
		return false
	}

	e2e.Logf("Registry accessible and healthy at %s", registryURL)
	return true
}

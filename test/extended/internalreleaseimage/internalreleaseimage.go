package internalreleaseimage

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-installer][Feature:NoRegistryClusterInstall] InternalReleaseImage maintains valid resource configuration and status after cluster install", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("no-registry")
	var helper *IRITestHelper

	g.BeforeEach(func() {
		skipIfNoRegistryFeatureUnsupported(oc)
		helper = NewIRITestHelper(oc)
	})

	g.Context("when the NoRegistryClusterInstall feature is enabled", func() {
		g.It("should have a exactly one InternalReleaseImage resource [apigroup:machineconfiguration.openshift.io]", func() {
			g.By("Verifying IRI resource exists and spec release is installed and available")
			iri := helper.GetIRI()
			o.Expect(iri.Spec.Releases).Should(o.HaveLen(1), "IRI should have exactly one release in spec")
			o.Expect(iri.Status.Releases).Should(o.HaveLen(1), "IRI should have exactly one release in status")
			o.Expect(iri.Status.Releases[0].Name).Should(o.Equal(iri.Spec.Releases[0].Name), "Status release name should match spec release name")
		})

		g.It("should create MachineConfigs with proper ownership references to InternalReleaseImage [apigroup:machineconfiguration.openshift.io]", func() {
			g.By("Verifying MachineConfigs with IRI ownership exist")
			iriMCs := helper.GetIRIMachineConfigs()
			o.Expect(iriMCs).NotTo(o.BeEmpty(), "IRI should have created at least one MachineConfig with ownership references")
			e2e.Logf("Verified %d MachineConfigs with IRI owner references", len(iriMCs))
		})
	})
})

var _ = g.Describe("[sig-installer][Feature:NoRegistryClusterInstall][Serial] InternalReleaseImage controller maintains ownership of MachineConfigs", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("no-registry")
	var helper *IRITestHelper

	g.BeforeEach(func() {
		skipIfNoRegistryFeatureUnsupported(oc)
		helper = NewIRITestHelper(oc)
	})

	g.Context("when IRI-owned MachineConfigs are deleted", func() {
		g.It("should restore all deleted MachineConfigs [apigroup:machineconfiguration.openshift.io]", func() {
			g.By("Deleting all IRI-owned MachineConfigs and verifying controller recreates them")

			// Get all IRI-owned MachineConfigs with UIDs and timestamps
			originalMCs := helper.GetIRIMachineConfigsWithMetadata()
			originalCount := len(originalMCs)
			e2e.Logf("Found %d IRI-owned MachineConfigs maintained by the controller", originalCount)

			// Delete all IRI-owned MachineConfigs
			e2e.Logf("Deleting all %d IRI-owned MachineConfigs to test controller reconciliation", originalCount)
			for mcName := range originalMCs {
				helper.DeleteMachineConfig(mcName)
			}

			// Wait for controller to recreate all MachineConfigs with new UIDs and newer timestamps
			// Track which MCs are pending verification, remove them as they're confirmed recreated
			e2e.Logf("Waiting for controller to recreate all MachineConfigs with new UIDs and newer timestamps")
			pendingMCs := make(map[string]MCInfo)
			for name, info := range originalMCs {
				pendingMCs[name] = info
			}

			err := wait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 5*time.Minute, true,
				func(_ context.Context) (bool, error) {
					// Get current state of all IRI-owned MachineConfigs
					current, err := helper.tryGetIRIMachineConfigsWithMetadata()
					if err != nil {
						e2e.Logf("Transient error listing MachineConfigs, retrying: %v", err)
						return false, nil
					}

					// Check each pending MC to see if it's been recreated
					for mcName, originalInfo := range pendingMCs {
						currentInfo, exists := current[mcName]
						if !exists {
							// MC doesn't exist yet - controller hasn't recreated it, keep waiting
							continue
						}

						// MC exists - verify UID changed and timestamp is newer, then remove from pending list
						if currentInfo.UID != originalInfo.UID && currentInfo.CreationTimestamp.After(originalInfo.CreationTimestamp.Time) {
							delete(pendingMCs, mcName)
							e2e.Logf("Verified MachineConfig %s recreated with new UID and newer timestamp", mcName)
						}
					}

					// Report progress and check if we're done
					remaining := len(pendingMCs)
					if remaining > 0 {
						e2e.Logf("Controller reconciliation progress: %d/%d MachineConfigs recreated (%d remaining)", originalCount-remaining, originalCount, remaining)
					}
					return remaining == 0, nil
				})
			o.Expect(err).NotTo(o.HaveOccurred(), "IRI controller should recreate all %d MachineConfigs with new UIDs and newer timestamps", originalCount)

			e2e.Logf("IRI controller successfully recreated all %d MachineConfigs with new UIDs and newer timestamps", originalCount)
		})
	})
})

var _ = g.Describe("[sig-installer][Feature:NoRegistryClusterInstall] InternalReleaseImage prevents deletion when in use", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("no-registry")
	var helper *IRITestHelper

	g.BeforeEach(func() {
		skipIfNoRegistryFeatureUnsupported(oc)
		helper = NewIRITestHelper(oc)
	})

	g.Context("when the cluster is using a release from the InternalReleaseImage resource", func() {
		g.It("should block deletion attempts with a ValidatingAdmissionPolicy error [apigroup:machineconfiguration.openshift.io]", func() {
			g.By("Attempting to delete IRI resource and verifying ValidatingAdmissionPolicy blocks it")

			e2e.Logf("Attempting to delete InternalReleaseImage resource while cluster is using it")
			err := helper.DeleteIRI()
			o.Expect(err).To(o.HaveOccurred(), "Deletion should be blocked when resource is in use")
			o.Expect(apierrors.IsInvalid(err) || apierrors.IsForbidden(err)).To(o.BeTrue(), "Deletion should be blocked by ValidatingAdmissionPolicy")

			errMsg := err.Error()
			o.Expect(errMsg).Should(o.ContainSubstring("Cannot delete InternalReleaseImage while the cluster is using a release bundle from this resource. The current cluster release image matches a release stored in this InternalReleaseImage. Please upgrade or downgrade to a different release before deletion"), "Error should explain that IRI deletion is blocked while cluster is using the resource")

			e2e.Logf("InternalReleaseImage deletion correctly rejected by ValidatingAdmissionPolicy: %v", err)
		})
	})
})

var _ = g.Describe("[sig-installer][Feature:NoRegistryClusterInstall] Cluster operates without external registry using managed OCP release bundle images", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("no-registry")
	var helper *IRITestHelper

	g.BeforeEach(func() {
		skipIfNoRegistryFeatureUnsupported(oc)
		helper = NewIRITestHelper(oc)
	})

	g.Context("when a workload based on the maintained OCP release bundle images is created", func() {
		g.It("should run successfully [apigroup:machineconfiguration.openshift.io]", func() {
			iri := helper.GetIRI()
			releaseImage := iri.Status.Releases[0].Image
			e2e.Logf("Using OCP release bundle image: %s", releaseImage)

			// Verify the image repo is present in IDMS (proves it will be pulled from mirror)
			g.By("Verifying image repo is present in ImageDigestMirrorSet")
			helper.VerifyIDMSConfigured(releaseImage)
			g.By("Creating test namespace and pod")
			ns := helper.CreateSimpleNamespace()
			defer helper.DeleteNamespace(ns)

			pod := helper.CreateTestPod(ns, releaseImage)
			defer helper.DeleteTestPod(ns, pod.Name)

			g.By("Waiting for pod to complete successfully")
			err := e2epod.WaitForPodSuccessInNamespace(context.Background(), oc.AdminKubeClient(), pod.Name, ns)
			o.Expect(err).NotTo(o.HaveOccurred(), "Pod should pull image from mirror registry and run successfully")

			// Get final pod status to log image ID
			completedPod, err := oc.AdminKubeClient().CoreV1().Pods(ns).Get(context.Background(), pod.Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get completed pod status")
			o.Expect(completedPod.Status.ContainerStatuses).NotTo(o.BeEmpty(), "Pod should have at least one container status")
			e2e.Logf("Workload successfully pulled image from mirror registry (ImageID: %s)", completedPod.Status.ContainerStatuses[0].ImageID)
		})
	})
})

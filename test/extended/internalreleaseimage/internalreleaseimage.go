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

const (
	IRIResourceName = "cluster"
)

var _ = g.Describe("[sig-installer][Feature:NoRegistryClusterInstall] InternalReleaseImage maintains valid resource configuration and status after cluster install", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("no-registry")
	var helper *IRITestHelper

	g.BeforeEach(func() {
		skipIfNoRegistryFeatureNotEnabled(oc)
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
			e2e.Logf("Verified %d MachineConfigs with IRI owner references", len(iriMCs))
		})
	})
})

var _ = g.Describe("[sig-installer][Feature:NoRegistryClusterInstall][Serial] InternalReleaseImage controller maintains ownership of MachineConfigs", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("no-registry")
	var helper *IRITestHelper

	g.BeforeEach(func() {
		skipIfNoRegistryFeatureNotEnabled(oc)
		helper = NewIRITestHelper(oc)
	})

	g.Context("when IRI-owned MachineConfigs are deleted", func() {
		g.It("should restore all deleted MachineConfigs [apigroup:machineconfiguration.openshift.io]", func() {
			g.By("Deleting all IRI-owned MachineConfigs and verifying controller restores them")

			// Get all IRI-owned MachineConfigs
			iriMCs := helper.GetIRIMachineConfigs()
			originalCount := len(iriMCs)
			e2e.Logf("Found %d IRI-owned MachineConfigs maintained by the controller", originalCount)

			// Delete all IRI-owned MachineConfigs
			e2e.Logf("Deleting all %d IRI-owned MachineConfigs to test controller reconciliation", originalCount)
			for _, mcName := range iriMCs {
				helper.DeleteMachineConfig(mcName)
			}

			// First, confirm deletions are observed to avoid immediate false-positive success
			err := wait.PollUntilContextTimeout(context.TODO(), 2*time.Second, 2*time.Minute, true,
				func(_ context.Context) (bool, error) {
					current, err := helper.tryGetIRIMachineConfigs()
					if err != nil {
						// Retry on transient list failures during reconciliation
						return false, nil
					}
					return !containsAllMachineConfigs(iriMCs, current), nil
				})
			o.Expect(err).NotTo(o.HaveOccurred(), "Expected deleted MachineConfigs to disappear before reconciliation check")

			// Wait for controller to restore deleted MachineConfigs
			err = wait.PollUntilContextTimeout(context.TODO(), 5*time.Second, 5*time.Minute, true,
				func(_ context.Context) (bool, error) {
					restored, err := helper.tryGetIRIMachineConfigs()
					if err != nil {
						return false, nil
					}
					e2e.Logf("Controller reconciliation progress: %d/%d MachineConfigs restored", len(restored), originalCount)
					return containsAllMachineConfigs(iriMCs, restored), nil
				})
			o.Expect(err).NotTo(o.HaveOccurred(), "IRI controller should restore all %d MachineConfigs", originalCount)

			e2e.Logf("IRI controller successfully maintained ownership and restored all %d MachineConfigs", originalCount)
		})
	})
})

var _ = g.Describe("[sig-installer][Feature:NoRegistryClusterInstall] InternalReleaseImage prevents deletion when in use", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("no-registry")
	var helper *IRITestHelper

	g.BeforeEach(func() {
		skipIfNoRegistryFeatureNotEnabled(oc)
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
			o.Expect(errMsg).Should(o.ContainSubstring("ValidatingAdmissionPolicy"), "Error should mention ValidatingAdmissionPolicy mechanism")
			o.Expect(errMsg).Should(o.ContainSubstring("internalreleaseimage-deletion-guard"), "Error should reference the specific deletion guard policy")
			o.Expect(errMsg).Should(o.ContainSubstring("Cannot delete InternalReleaseImage"), "Error should clearly state IRI deletion is not allowed")
			o.Expect(errMsg).Should(o.ContainSubstring("while the cluster is using"), "Error should explain cluster is using the resource")

			e2e.Logf("InternalReleaseImage deletion correctly rejected by ValidatingAdmissionPolicy: %v", err)
		})
	})
})

var _ = g.Describe("[sig-installer][Feature:NoRegistryClusterInstall] Cluster operates without external registry using managed OCP release bundle images", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("no-registry")
	var helper *IRITestHelper

	g.BeforeEach(func() {
		skipIfNoRegistryFeatureNotEnabled(oc)
		helper = NewIRITestHelper(oc)
	})

	g.Context("when a workload based on the maintained OCP release bundle images is created", func() {
		g.It("should run successfully [apigroup:machineconfiguration.openshift.io]", func() {
			iri := helper.GetIRI()
			releaseImage := iri.Status.Releases[0].Image
			e2e.Logf("Using OCP release bundle image: %s", releaseImage)

			g.By("Creating simple test namespace without registry secret dependencies")
			ns := helper.CreateSimpleNamespace("no-registry-pod")
			defer helper.DeleteNamespace(ns)

			g.By("Creating test pod with OCP release bundle image")
			pod := helper.CreateTestPod(ns, releaseImage)
			defer helper.DeleteTestPod(ns, pod.Name)

			g.By("Waiting for pod to complete successfully")
			err := e2epod.WaitForPodSuccessInNamespace(context.Background(), oc.AdminKubeClient(), pod.Name, ns)
			o.Expect(err).NotTo(o.HaveOccurred(), "Pod should pull image from IRI registry and run successfully")

			// Get final pod status to log image ID
			completedPod, err := oc.AdminKubeClient().CoreV1().Pods(ns).Get(context.Background(), pod.Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get completed pod status")
			o.Expect(completedPod.Status.ContainerStatuses).NotTo(o.BeEmpty(), "Pod should have at least one container status")
			e2e.Logf("Workload successfully pulled image from internal IRI registry (ImageID: %s)", completedPod.Status.ContainerStatuses[0].ImageID)
		})
	})
})

package internalreleaseimage

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	configv1 "github.com/openshift/api/config/v1"
	machineconfigv1alpha1types "github.com/openshift/api/machineconfiguration/v1alpha1"
	machineconfigv1 "github.com/openshift/client-go/machineconfiguration/clientset/versioned/typed/machineconfiguration/v1"
	machineconfigv1alpha1 "github.com/openshift/client-go/machineconfiguration/clientset/versioned/typed/machineconfiguration/v1alpha1"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	IRIResourceName = "cluster"
)

// IRITestHelper is a helper class for InternalReleaseImage tests
type IRITestHelper struct {
	oc               *exutil.CLI
	McClientV1       machineconfigv1.MachineconfigurationV1Interface
	McClientV1alpha1 machineconfigv1alpha1.MachineconfigurationV1alpha1Interface
}

// MCInfo holds MachineConfig metadata for reconciliation verification
type MCInfo struct {
	UID               string
	CreationTimestamp metav1.Time
}

// NewIRITestHelper creates a new test helper instance
func NewIRITestHelper(oc *exutil.CLI) *IRITestHelper {
	return &IRITestHelper{
		oc:               oc,
		McClientV1:       oc.MachineConfigurationClient().MachineconfigurationV1(),
		McClientV1alpha1: oc.MachineConfigurationClient().MachineconfigurationV1alpha1(),
	}
}

// GetIRI gets the InternalReleaseImage resource and fails the test if not found
func (h *IRITestHelper) GetIRI() *machineconfigv1alpha1types.InternalReleaseImage {
	iri, err := h.McClientV1alpha1.InternalReleaseImages().Get(context.Background(), IRIResourceName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get InternalReleaseImage resource")
	return iri
}

// GetIRIMachineConfigs returns all MachineConfigs owned by InternalReleaseImage
func (h *IRITestHelper) GetIRIMachineConfigs() []string {
	mcList, err := h.McClientV1.MachineConfigs().List(context.Background(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list MachineConfigs")

	var iriMCs []string
	for _, mc := range mcList.Items {
		for _, ownerRef := range mc.OwnerReferences {
			if ownerRef.Kind == "InternalReleaseImage" && ownerRef.Name == IRIResourceName {
				iriMCs = append(iriMCs, mc.Name)
				break
			}
		}
	}

	o.Expect(iriMCs).ShouldNot(o.BeEmpty(), "IRI should have created MachineConfigs with ownership references")
	return iriMCs
}

// tryGetIRIMachineConfigs returns MachineConfigs without failing (for use in polling)
// GetIRIMachineConfigsWithMetadata returns MachineConfigs with their UIDs and timestamps
func (h *IRITestHelper) GetIRIMachineConfigsWithMetadata() map[string]MCInfo {
	mcList, err := h.McClientV1.MachineConfigs().List(context.Background(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list MachineConfigs")

	iriMCs := make(map[string]MCInfo)
	for _, mc := range mcList.Items {
		for _, ownerRef := range mc.OwnerReferences {
			if ownerRef.Kind == "InternalReleaseImage" && ownerRef.Name == IRIResourceName {
				iriMCs[mc.Name] = MCInfo{
					UID:               string(mc.UID),
					CreationTimestamp: mc.CreationTimestamp,
				}
				break
			}
		}
	}

	o.Expect(iriMCs).ShouldNot(o.BeEmpty(), "IRI should have created MachineConfigs with ownership references")
	return iriMCs
}

// tryGetIRIMachineConfigsWithMetadata returns MachineConfigs with metadata without failing
func (h *IRITestHelper) tryGetIRIMachineConfigsWithMetadata() (map[string]MCInfo, error) {
	mcList, err := h.McClientV1.MachineConfigs().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	iriMCs := make(map[string]MCInfo)
	for _, mc := range mcList.Items {
		for _, ownerRef := range mc.OwnerReferences {
			if ownerRef.Kind == "InternalReleaseImage" && ownerRef.Name == IRIResourceName {
				iriMCs[mc.Name] = MCInfo{
					UID:               string(mc.UID),
					CreationTimestamp: mc.CreationTimestamp,
				}
				break
			}
		}
	}

	return iriMCs, nil
}

// DeleteMachineConfig deletes a MachineConfig by name
func (h *IRITestHelper) DeleteMachineConfig(name string) {
	err := h.McClientV1.MachineConfigs().Delete(context.Background(), name, metav1.DeleteOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to delete MachineConfig %s", name)
}

// DeleteIRI attempts to delete the InternalReleaseImage resource
func (h *IRITestHelper) DeleteIRI() error {
	return h.McClientV1alpha1.InternalReleaseImages().Delete(context.Background(), IRIResourceName, metav1.DeleteOptions{})
}

// VerifyIDMSConfigured verifies that the test image repo is present in the image-digest-mirror IDMS
func (h *IRITestHelper) VerifyIDMSConfigured(releaseImage string) {
	e2e.Logf("Verifying image repo is present in image-digest-mirror IDMS: %s", releaseImage)

	// Get the specific IDMS created for NoRegistryClusterInstall
	idms, err := h.oc.AdminConfigClient().ConfigV1().ImageDigestMirrorSets().Get(context.Background(), "image-digest-mirror", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get image-digest-mirror IDMS")

	// Extract the source from the release image (remove @sha256:... digest)
	// Example: "registry.ci.openshift.org/ocp/4.22-2026-03-27-160521@sha256:abc" -> "registry.ci.openshift.org/ocp/4.22-2026-03-27-160521"
	imageSource := strings.Split(releaseImage, "@")[0]
	e2e.Logf("Extracted image source: %s", imageSource)

	// Verify that the image source is covered by at least one IDMS mirror
	foundMatch := false
	for _, mirrorSet := range idms.Spec.ImageDigestMirrors {
		// Check if this IDMS source matches our image source exactly
		if mirrorSet.Source == imageSource {
			o.Expect(mirrorSet.Mirrors).NotTo(o.BeEmpty(), "IDMS source %s should have at least one mirror", mirrorSet.Source)
			e2e.Logf("Found IDMS match: source %s -> mirrors %v", mirrorSet.Source, mirrorSet.Mirrors)
			foundMatch = true
			break
		}
	}

	o.Expect(foundMatch).To(o.BeTrue(), "Image source %s must be present in image-digest-mirror IDMS to ensure mirrored pull", imageSource)
	e2e.Logf("Confirmed: test image repo is covered by IDMS, will be pulled from mirror registry")
}

// CreateTestPod creates a test pod with the specified release image in the given namespace
func (h *IRITestHelper) CreateTestPod(namespace, releaseImage string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "iri-registry-test-" + string(uuid.NewUUID()),
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			RestartPolicy:   corev1.RestartPolicyNever,
			SecurityContext: e2epod.GetRestrictedPodSecurityContext(),
			Containers: []corev1.Container{
				{
					Name:            "test",
					Image:           releaseImage,
					Command:         []string{"echo", "success"},
					SecurityContext: e2epod.GetRestrictedContainerSecurityContext(),
				},
			},
		},
	}

	createdPod, err := h.oc.AdminKubeClient().CoreV1().Pods(namespace).Create(context.Background(), pod, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create test pod")
	e2e.Logf("Created test pod: %s/%s", createdPod.Namespace, createdPod.Name)

	return createdPod
}

// DeleteTestPod deletes a test pod by name from the specified namespace
func (h *IRITestHelper) DeleteTestPod(namespace, name string) {
	err := h.oc.AdminKubeClient().CoreV1().Pods(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		e2e.Logf("Warning: failed to delete test pod %s/%s: %v", namespace, name, err)
	}
}

// CreateSimpleNamespace creates a basic namespace without waiting for service account secrets
func (h *IRITestHelper) CreateSimpleNamespace() string {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "e2e-test-" + string(uuid.NewUUID()),
		},
	}

	createdNs, err := h.oc.AdminKubeClient().CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create namespace")
	e2e.Logf("Created namespace: %s", createdNs.Name)

	return createdNs.Name
}

// DeleteNamespace deletes a namespace
func (h *IRITestHelper) DeleteNamespace(name string) {
	err := h.oc.AdminKubeClient().CoreV1().Namespaces().Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		e2e.Logf("Warning: failed to delete namespace %s: %v", name, err)
	} else {
		e2e.Logf("Deleted namespace: %s", name)
	}
}

// skipIfNoRegistryFeatureUnsupported skips the test if NoRegistryClusterInstall is not supported
// This checks: platform type (BareMetal/None) and feature gate enablement
func skipIfNoRegistryFeatureUnsupported(oc *exutil.CLI) {
	g.By("Checking if NoRegistryClusterInstall feature is supported")

	// Platform must be BareMetal or None
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		g.Skip(fmt.Sprintf("Failed to get Infrastructure: %v", err))
	}

	if infra.Status.PlatformStatus == nil {
		g.Skip("Infrastructure status does not have platform information")
	}

	platformType := infra.Status.PlatformStatus.Type
	if platformType != configv1.BareMetalPlatformType && platformType != configv1.NonePlatformType {
		g.Skip(fmt.Sprintf("NoRegistryClusterInstall is only supported on BareMetal and None platforms, current platform: %s", platformType))
	}

	// Feature gate NoRegistryClusterInstall must be enabled
	featureGate, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		g.Skip(fmt.Sprintf("Failed to get FeatureGate: %v", err))
	}

	featureEnabled := false
	if featureGate.Status.FeatureGates != nil {
		for _, fg := range featureGate.Status.FeatureGates {
			for _, feature := range fg.Enabled {
				if feature.Name == "NoRegistryClusterInstall" {
					featureEnabled = true
					break
				}
			}
		}
	}

	if !featureEnabled {
		g.Skip("NoRegistryClusterInstall feature gate is not enabled")
	}

	// Check if InternalReleaseImage resource is present. If not present, the feature is not enabled.
	_, err = oc.MachineConfigurationClient().MachineconfigurationV1alpha1().InternalReleaseImages().Get(context.Background(), IRIResourceName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			g.Skip("InternalReleaseImage resource not found, feature not enabled")
		}
		g.Skip(fmt.Sprintf("error while checking for InternalReleaseImage availability: %v", err))
	}
}

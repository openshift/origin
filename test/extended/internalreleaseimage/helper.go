package internalreleaseimage

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	configv1 "github.com/openshift/api/config/v1"
	v1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigv1 "github.com/openshift/client-go/machineconfiguration/clientset/versioned/typed/machineconfiguration/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	IRIResourceName = "cluster"
)

// IRITestHelper is a helper class for InternalReleaseImage tests
type IRITestHelper struct {
	oc         *exutil.CLI
	McClientV1 machineconfigv1.MachineconfigurationV1Interface
}

// MCInfo holds MachineConfig metadata for reconciliation verification
type MCInfo struct {
	UID               string
	CreationTimestamp metav1.Time
}

// NewIRITestHelper creates a new test helper instance
func NewIRITestHelper(oc *exutil.CLI) *IRITestHelper {
	return &IRITestHelper{
		oc:         oc,
		McClientV1: oc.MachineConfigurationClient().MachineconfigurationV1(),
	}
}

// GetIRI gets the InternalReleaseImage resource and fails the test if not found
func (h *IRITestHelper) GetIRI() *v1.InternalReleaseImage {
	iri, err := h.McClientV1.InternalReleaseImages().Get(context.Background(), IRIResourceName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get InternalReleaseImage resource")
	return iri
}

// GetIRIMachineConfigs returns all MachineConfigs owned by InternalReleaseImage
func (h *IRITestHelper) tryGetIRIMachineConfigs() ([]*v1.MachineConfig, error) {
	mcList, err := h.McClientV1.MachineConfigs().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var iriMCs []*v1.MachineConfig
	for _, mc := range mcList.Items {
		if strings.HasSuffix(mc.Name, "-internalreleaseimage") {
			iriMCs = append(iriMCs, mc.DeepCopy())
		}
	}

	return iriMCs, nil
}

func (h *IRITestHelper) GetIRIMachineConfigs() []*v1.MachineConfig {
	iriMCs, err := h.tryGetIRIMachineConfigs()
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list MachineConfigs")
	return iriMCs
}

// DeleteMachineConfig deletes a MachineConfig by name
func (h *IRITestHelper) DeleteMachineConfig(name string) {
	err := h.McClientV1.MachineConfigs().Delete(context.Background(), name, metav1.DeleteOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to delete MachineConfig %s", name)
}

// DeleteIRI attempts to delete the InternalReleaseImage resource
func (h *IRITestHelper) DeleteIRI() error {
	return h.McClientV1.InternalReleaseImages().Delete(context.Background(), IRIResourceName, metav1.DeleteOptions{})
}

// VerifyIDMSConfigured verifies that the test image repo is present as a mirror in at least one IDMS
func (h *IRITestHelper) VerifyIDMSConfigured(releaseImage string) {
	e2e.Logf("Verifying image repo is present in image-digest-mirror IDMS: %s", releaseImage)

	// List all IDMS resources
	idmsList, err := h.oc.AdminConfigClient().ConfigV1().ImageDigestMirrorSets().List(context.Background(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to list ImageDigestMirrorSets")

	// Extract the repo from the release image (remove @sha256:... digest)
	// Example: "api-int.example.com:22625/openshift/release-images@sha256:abc" -> "api-int.example.com:22625/openshift/release-images"
	imageSource := strings.Split(releaseImage, "@")[0]
	e2e.Logf("Extracted image source: %s", imageSource)

	// Verify that the image source is listed as a mirror in at least one IDMS
	foundMatch := false
	for _, idms := range idmsList.Items {
		for _, mirrorSet := range idms.Spec.ImageDigestMirrors {
			for _, mirror := range mirrorSet.Mirrors {
				if string(mirror) == imageSource {
					e2e.Logf("Found IDMS match in %s: source %s -> mirror %s", idms.Name, mirrorSet.Source, mirror)
					foundMatch = true
					break
				}
			}
			if foundMatch {
				break
			}
		}
		if foundMatch {
			break
		}
	}

	o.Expect(foundMatch).To(o.BeTrue(), "Image source %s must be present as a mirror in at least one IDMS to ensure mirrored pull", imageSource)
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

// CreateSimpleNamespace creates a basic namespace and waits for SCC annotations
func (h *IRITestHelper) CreateSimpleNamespace() string {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "e2e-test-" + string(uuid.NewUUID()),
		},
	}

	createdNs, err := h.oc.AdminKubeClient().CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create namespace")
	e2e.Logf("Created namespace: %s", createdNs.Name)

	// Wait for the namespace controller to set the SCC uid-range annotation,
	// which is required by the admission controller before pods can be created.
	err = wait.PollUntilContextTimeout(context.Background(), 500*time.Millisecond, 60*time.Second, true, func(ctx context.Context) (bool, error) {
		updatedNs, err := h.oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, createdNs.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		_, exists := updatedNs.Annotations["openshift.io/sa.scc.uid-range"]
		return exists, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "Timed out waiting for namespace %s to get SCC uid-range annotation", createdNs.Name)

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
	_, err = oc.MachineConfigurationClient().MachineconfigurationV1().InternalReleaseImages().Get(context.Background(), IRIResourceName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			g.Skip("InternalReleaseImage resource not found, feature not enabled")
		}
		g.Skip(fmt.Sprintf("error while checking for InternalReleaseImage availability: %v", err))
	}
}

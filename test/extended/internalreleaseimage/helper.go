package internalreleaseimage

import (
	"context"
	"fmt"

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

// IRITestHelper is a helper class for InternalReleaseImage tests
type IRITestHelper struct {
	oc               *exutil.CLI
	McClientV1       machineconfigv1.MachineconfigurationV1Interface
	McClientV1alpha1 machineconfigv1alpha1.MachineconfigurationV1alpha1Interface
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
func (h *IRITestHelper) tryGetIRIMachineConfigs() ([]string, error) {
	mcList, err := h.McClientV1.MachineConfigs().List(context.Background(), metav1.ListOptions{})
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

// DeleteMachineConfig deletes a MachineConfig by name
func (h *IRITestHelper) DeleteMachineConfig(name string) {
	err := h.McClientV1.MachineConfigs().Delete(context.Background(), name, metav1.DeleteOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to delete MachineConfig %s", name)
}

// DeleteIRI attempts to delete the InternalReleaseImage resource
func (h *IRITestHelper) DeleteIRI() error {
	return h.McClientV1alpha1.InternalReleaseImages().Delete(context.Background(), IRIResourceName, metav1.DeleteOptions{})
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
func (h *IRITestHelper) CreateSimpleNamespace(baseName string) string {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("e2e-test-%s-", baseName),
		},
	}

	createdNs, err := h.oc.AdminKubeClient().CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create namespace")
	e2e.Logf("Created simple namespace: %s", createdNs.Name)

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

// containsAllMachineConfigs checks if all expected MachineConfigs are present in the actual list
func containsAllMachineConfigs(expected, actual []string) bool {
	if len(expected) != len(actual) {
		return false
	}

	actualSet := make(map[string]bool)
	for _, name := range actual {
		actualSet[name] = true
	}

	for _, name := range expected {
		if !actualSet[name] {
			return false
		}
	}

	return true
}

// Helper functions

func skipIfNoRegistryFeatureNotEnabled(oc *exutil.CLI) {
	g.By("Checking if NoRegistryClusterInstall feature is available")

	// Platform must be BareMetal or None
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(
		context.Background(), "cluster", metav1.GetOptions{})
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
	featureGate, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(
		context.Background(), "cluster", metav1.GetOptions{})
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

	// InternalReleaseImage resource must be present
	mcClient := oc.MachineConfigurationClient().MachineconfigurationV1alpha1()
	_, err = mcClient.InternalReleaseImages().Get(context.Background(), IRIResourceName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			g.Skip("InternalReleaseImage resource not found - NoRegistryClusterInstall feature not available")
		}
		g.Skip(fmt.Sprintf("Failed to get InternalReleaseImage resource: %v", err))
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

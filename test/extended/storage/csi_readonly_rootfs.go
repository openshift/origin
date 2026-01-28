package storage

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// csiResourceCheck defines a check for CSI controller or node resources
type csiResourceCheck struct {
	ResourceType ResourceType
	Namespace    string
	Name         string
	Platform     string
}

// Use the existing ResourceType from the package

var _ = g.Describe("[sig-storage][OCPFeature:CSIReadOnlyRootFilesystem][Jira:Storage] CSI Driver ReadOnly Root Filesystem", func() {
	defer g.GinkgoRecover()
	var (
		oc              = exutil.NewCLI("csi-readonly-rootfs")
		currentPlatform = e2e.TestContext.Provider
	)

	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("CSI ReadOnlyRootFilesystem tests are not supported on MicroShift")
		}

		// Check to see if we have Storage enabled
		isStorageEnabled, err := exutil.IsCapabilityEnabled(oc, configv1.ClusterVersionCapabilityStorage)
		if err != nil || !isStorageEnabled {
			g.Skip("skipping, this test is only expected to work with storage enabled clusters")
		}
	})

	g.It("should verify CSI controller and node containers have readOnlyRootFilesystem set to true", func() {
		// Get all CSI drivers present in the cluster
		allCSIResources := getCSIResourcesInCluster(oc, currentPlatform)

		runReadOnlyRootFsChecks(oc, allCSIResources, currentPlatform)
	})
})

// getCSIResourcesInCluster dynamically discovers OpenShift CSI driver resources present in the cluster
func getCSIResourcesInCluster(oc *exutil.CLI, currentPlatform string) []csiResourceCheck {
	ctx := context.TODO()
	var resources []csiResourceCheck

	// Check if this is a Hypershift deployment
	isHypershift := isHypershiftCluster(oc)
	if isHypershift {
		e2e.Logf("Detected Hypershift cluster - will only check CSI node components (controllers are on management cluster)")
	}

	// Known OpenShift CSI driver prefixes
	openshiftCSIDrivers := []string{
		"aws-ebs-csi-driver",
		"aws-efs-csi-driver",
		"azure-disk-csi-driver",
		"azure-file-csi-driver",
		"gcp-pd-csi-driver",
		"gcp-filestore-csi-driver",
		"vmware-vsphere-csi-driver",
		"ibm-vpc-block-csi",
		"openstack-cinder-csi-driver",
		"openstack-manila-csi",
		"smb-csi-driver",
	}

	isOpenShiftCSIDriver := func(name string) bool {
		for _, driver := range openshiftCSIDrivers {
			if strings.HasPrefix(name, driver) {
				return true
			}
		}
		return false
	}

	// Check CSI namespace for deployments and daemonsets
	checkNamespace := func(namespace string) {
		// Get all deployments in the namespace
		deployments, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			e2e.Logf("Failed to list deployments in namespace %s: %v", namespace, err)
			return
		}

		for _, deployment := range deployments.Items {
			// Check if it's an OpenShift CSI controller deployment
			if isOpenShiftCSIDriver(deployment.Name) && strings.Contains(deployment.Name, "controller") {
				// Skip controller deployments for Hypershift as they run on management cluster
				if isHypershift {
					e2e.Logf("Skipping controller deployment %s in Hypershift (runs on management cluster)", deployment.Name)
					continue
				}
				resources = append(resources, csiResourceCheck{
					ResourceType: ResourceTypeDeployment,
					Namespace:    namespace,
					Name:         deployment.Name,
					Platform:     currentPlatform,
				})
			}
		}

		// Get all daemonsets in the namespace
		daemonsets, err := oc.AdminKubeClient().AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			e2e.Logf("Failed to list daemonsets in namespace %s: %v", namespace, err)
			return
		}

		for _, daemonset := range daemonsets.Items {
			// Check if it's an OpenShift CSI node daemonset
			if isOpenShiftCSIDriver(daemonset.Name) && strings.Contains(daemonset.Name, "node") {
				resources = append(resources, csiResourceCheck{
					ResourceType: ResourceTypeDaemonSet,
					Namespace:    namespace,
					Name:         daemonset.Name,
					Platform:     currentPlatform,
				})
			}
		}
	}

	// Check main CSI namespace
	checkNamespace(CSINamespace)
	// Check Manila CSI namespace (OpenStack)
	checkNamespace(ManilaCSINamespace)

	e2e.Logf("Found %d OpenShift CSI resources in cluster", len(resources))
	for _, resource := range resources {
		e2e.Logf("OpenShift CSI Resource: %s %s/%s", resource.ResourceType, resource.Namespace, resource.Name)
	}

	return resources
}

// isHypershiftCluster detects if this is a Hypershift (HCP) deployment
func isHypershiftCluster(oc *exutil.CLI) bool {
	ctx := context.TODO()

	// Check control plane topology in Infrastructure
	infrastructure, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err == nil && infrastructure.Status.ControlPlaneTopology == configv1.ExternalTopologyMode {
		return true
	}

	return false
}

// runReadOnlyRootFsChecks verifies that all containers in the resource have readOnlyRootFilesystem set
func runReadOnlyRootFsChecks(oc *exutil.CLI, resources []csiResourceCheck, currentPlatform string) {
	results := []string{}
	hasFail := false

	for _, resource := range resources {

		resourceName := fmt.Sprintf("%s %s/%s", resource.ResourceType, resource.Namespace, resource.Name)

		var podSpec *corev1.PodSpec
		var found bool

		switch resource.ResourceType {
		case ResourceTypeDeployment:
			deployment, err := oc.AdminKubeClient().AppsV1().Deployments(resource.Namespace).Get(context.TODO(), resource.Name, metav1.GetOptions{})
			if err != nil {
				g.Fail(fmt.Sprintf("Error fetching %s: %v", resourceName, err))
			}
			podSpec = &deployment.Spec.Template.Spec
			found = true

		case ResourceTypeDaemonSet:
			daemonset, err := oc.AdminKubeClient().AppsV1().DaemonSets(resource.Namespace).Get(context.TODO(), resource.Name, metav1.GetOptions{})
			if err != nil {
				g.Fail(fmt.Sprintf("Error fetching %s: %v", resourceName, err))
			}
			podSpec = &daemonset.Spec.Template.Spec
			found = true

		default:
			g.Fail(fmt.Sprintf("Unsupported resource type: %s", resource.ResourceType))
		}

		if !found {
			continue
		}

		// Check all containers and init containers
		containersWithoutReadOnlyRootFs := []string{}
		allContainers := append([]corev1.Container{}, podSpec.Containers...)
		allContainers = append(allContainers, podSpec.InitContainers...)

		for _, container := range allContainers {
			if container.SecurityContext == nil || container.SecurityContext.ReadOnlyRootFilesystem == nil || !*container.SecurityContext.ReadOnlyRootFilesystem {
				containersWithoutReadOnlyRootFs = append(containersWithoutReadOnlyRootFs, container.Name)
			}
		}

		if len(containersWithoutReadOnlyRootFs) > 0 {
			results = append(results, fmt.Sprintf("[FAIL] %s has containers without readOnlyRootFilesystem: %s", resourceName, strings.Join(containersWithoutReadOnlyRootFs, ", ")))
			hasFail = true
		} else {
			results = append(results, fmt.Sprintf("[PASS] %s (all %d containers have readOnlyRootFilesystem: true)", resourceName, len(allContainers)))
		}
	}

	if hasFail {
		summary := strings.Join(results, "\n")
		g.Fail(fmt.Sprintf("Some CSI resources have containers without readOnlyRootFilesystem:\n\n%s\n", summary))
	} else {
		e2e.Logf("All checked CSI resources have readOnlyRootFilesystem set correctly:\n%s", strings.Join(results, "\n"))
	}
}

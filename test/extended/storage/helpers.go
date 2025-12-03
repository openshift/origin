package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	imageregistry "github.com/openshift/client-go/imageregistry/clientset/versioned"
	clusteroperatorhelpers "github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

// Define test waiting time const
const (
	defaultMaxWaitingTime = 200 * time.Second
	defaultPollingTime    = 2 * time.Second
)

// Storage operator and CSI driver namespace constants
const (
	CSONamespace       = "openshift-cluster-storage-operator" // Cluster Storage Operator namespace
	CSINamespace       = "openshift-cluster-csi-drivers"      // Default CSI driver operators namespace
	ManilaCSINamespace = "openshift-manila-csi-driver"        // Manila CSI driver namespace (OpenStack only)
)

// IsCSOHealthy checks whether the Cluster Storage Operator is healthy
func IsCSOHealthy(oc *exutil.CLI) (bool, error) {
	// CSO healthyStatus:[degradedStatus:False, progressingStatus:False, availableStatus:True, upgradeableStatus:True]
	clusterStorageOperator, getOperatorErr := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(context.Background(), "storage", metav1.GetOptions{})
	if getOperatorErr != nil {
		e2e.Logf("Error getting storage operator: %v", getOperatorErr)
		return false, getOperatorErr
	}
	return clusteroperatorhelpers.IsStatusConditionTrue(clusterStorageOperator.Status.Conditions, configv1.OperatorAvailable) &&
		clusteroperatorhelpers.IsStatusConditionTrue(clusterStorageOperator.Status.Conditions, configv1.OperatorUpgradeable) &&
		clusteroperatorhelpers.IsStatusConditionFalse(clusterStorageOperator.Status.Conditions, configv1.OperatorDegraded) &&
		clusteroperatorhelpers.IsStatusConditionFalse(clusterStorageOperator.Status.Conditions, configv1.OperatorProgressing), nil
}

// WaitForCSOHealthy waits for Cluster Storage Operator become healthy
func WaitForCSOHealthy(oc *exutil.CLI) {
	o.Eventually(func() bool {
		IsCSOHealthy, getCSOHealthyErr := IsCSOHealthy(oc)
		if getCSOHealthyErr != nil {
			e2e.Logf(`Get CSO status failed of: "%v", try again`, getCSOHealthyErr)
		}
		return IsCSOHealthy
	}).WithTimeout(defaultMaxWaitingTime).WithPolling(defaultPollingTime).Should(o.BeTrue(), "Waiting for CSO become healthy timeout")
}

func skipIfNotS3Storage(oc *exutil.CLI) {
	g.By("checking storage type")

	imageRegistryConfigClient, err := imageregistry.NewForConfig(oc.AdminConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	imageRegistryConfig, err := imageRegistryConfigClient.ImageregistryV1().Configs().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	if imageRegistryConfig.Spec.Storage.S3 == nil {
		e2eskipper.Skipf("No S3 storage detected")
	}
}

// getCSINodeAllocatableCount gets the allocatable count for a specific CSI driver from a CSINode
func getCSINodeAllocatableCountByDriver(ctx context.Context, oc *exutil.CLI, nodeName, driverName string) int32 {
	csiNode, err := oc.AdminKubeClient().StorageV1().CSINodes().Get(ctx, nodeName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("failed to get CSINode for node %s", nodeName))

	for _, driver := range csiNode.Spec.Drivers {
		if driver.Name == driverName {
			if driver.Allocatable != nil && driver.Allocatable.Count != nil {
				return *driver.Allocatable.Count
			}
		}
	}
	e2e.Failf("CSI driver %s not found in CSINode %s", driverName, nodeName)
	return 0
}

// getAWSInstanceIDFromNode extracts the AWS instance ID from a node's providerID
func getAWSInstanceIDFromNode(ctx context.Context, oc *exutil.CLI, nodeName string) string {
	node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("failed to get node %s", nodeName))

	providerID := node.Spec.ProviderID
	o.Expect(providerID).NotTo(o.BeEmpty(), "node providerID should not be empty")

	parts := strings.Split(providerID, "/")
	o.Expect(len(parts)).To(o.BeNumerically(">=", 5), "invalid AWS providerID format")

	instanceID := parts[len(parts)-1]
	o.Expect(instanceID).To(o.HavePrefix("i-"), "instance ID should start with 'i-'")

	return instanceID
}

// getAttachedVolumeCountFromVolumeAttachments returns how many VolumeAttachments
// currently target the given node.
func getAttachedVolumeCountFromVolumeAttachments(ctx context.Context, oc *exutil.CLI, nodeName string) int32 {
	vaList, err := oc.AdminKubeClient().
		StorageV1().
		VolumeAttachments().
		List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to list VolumeAttachments")

	var count int32 = 0
	for _, va := range vaList.Items {
		if va.Spec.NodeName == nodeName && va.Status.Attached {
			count++
		}
	}
	return count
}

// GetCSIStorageClassByProvisioner finds a StorageClass that uses the CSI driver provisioner
// it will return the name of the first matched StorageClass if found, otherwise it will fail the test
func GetCSIStorageClassByProvisioner(ctx context.Context, oc *exutil.CLI, provisioner string) string {
	storageClasses, err := oc.AdminKubeClient().StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to list StorageClasses")

	for _, sc := range storageClasses.Items {
		if sc.Provisioner == ebsCSIDriverName {
			e2e.Logf("Found CSI StorageClass: %s", sc.Name)
			return sc.Name
		}
	}

	e2e.Failf("No StorageClass found with provisioner %s", ebsCSIDriverName)
	return ""
}

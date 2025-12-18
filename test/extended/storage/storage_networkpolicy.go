package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// storage_networkpolicy.go contains tests for verifying network policy configurations
// for storage-related operators in OpenShift.
//
// This test suite validates that storage operators and their workload resources have
// the correct network policy labels to ensure proper network segmentation and security.
//
// Test Coverage:
// 1. CSO (Cluster Storage Operator) resources - verifies network policy labels on
//    cluster-storage-operator, csi-snapshot-controller, and related deployments
// 2. CSI driver resources - verifies network policy labels on platform-specific CSI
//    drivers (AWS EBS/EFS, Azure Disk/File, GCP PD/Filestore, vSphere, IBM Cloud,
//    OpenStack Cinder/Manila, SMB)
// 3. LSO (Local Storage Operator) resources - verifies network policy labels on
//    related deployment and diskmaker daemonsets
// 4. NetworkPolicy resources - ensures required NetworkPolicies exist with correct
//    PodSelector labels for CSO, CSI, and LSO namespaces
//
// LSO-specific Design Considerations:
// - LSONamespace is defined as a variable (not constant) because LSO can be installed
//   in a user-specified namespace, allowing for customization based on actual deployment

// ResourceType defines the type of Openshift workload resource
type ResourceType string

const (
	ResourceTypeDeployment ResourceType = "Deployment"
	ResourceTypeDaemonSet  ResourceType = "DaemonSet"
)

// lsoInfo contains information about the Local Storage Operator installation
type lsoInfo struct {
	Installed bool
	Namespace string
	Version   string
}

// resourceCheck defines a check for a workload resource (Deployment, DaemonSet, etc.)
type resourceCheck struct {
	ResourceType   ResourceType
	Namespace      string
	Name           string
	Platform       string
	RequiredLabels map[string]string
}

var (
	npLabelAPI                  = map[string]string{"openshift.storage.network-policy.api-server": "allow"}
	npLabelDNS                  = map[string]string{"openshift.storage.network-policy.dns": "allow"}
	npLabelOperatorMetrics      = map[string]string{"openshift.storage.network-policy.operator-metrics": "allow"}
	npLabelOperatorMetricsRange = map[string]string{"openshift.storage.network-policy.operator-metrics-range": "allow"}
	npLabelMetricsRange         = map[string]string{"openshift.storage.network-policy.metrics-range": "allow"}
	npLabelAllEgress            = map[string]string{"openshift.storage.network-policy.all-egress": "allow"}
	// LSO specific network policy labels
	npLabelLSOAPIServer        = map[string]string{"openshift.storage.network-policy.lso.api-server": "allow"}
	npLabelLSODNS              = map[string]string{"openshift.storage.network-policy.lso.dns": "allow"}
	npLabelLSOOperatorMetrics  = map[string]string{"openshift.storage.network-policy.lso.operator-metrics": "allow"}
	npLabelLSODiskmakerMetrics = map[string]string{"openshift.storage.network-policy.lso.diskmaker-metrics": "allow"}
)

func mergeLabels(maps ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

var (
	csoOperatorRequiredLabels                = mergeLabels(npLabelAPI, npLabelDNS, npLabelOperatorMetrics)
	csoOperatorWithAllEgressRequiredLabels   = mergeLabels(npLabelAPI, npLabelDNS, npLabelOperatorMetrics, npLabelAllEgress)
	csoControllerRequiredLabels              = mergeLabels(npLabelAPI, npLabelDNS)
	csoControllerWithAllEgressRequiredLabels = mergeLabels(npLabelAPI, npLabelDNS, npLabelAllEgress)
	csiOperatorRequiredLabels                = mergeLabels(npLabelAPI, npLabelDNS, npLabelOperatorMetricsRange)
	csiOperatorWithAllEgressRequiredLabels   = mergeLabels(npLabelAPI, npLabelDNS, npLabelOperatorMetricsRange, npLabelAllEgress)
	csiControllerRequiredLabels              = mergeLabels(npLabelAPI, npLabelDNS, npLabelMetricsRange)
	csiControllerWithAllEgressRequiredLabels = mergeLabels(npLabelAPI, npLabelDNS, npLabelMetricsRange, npLabelAllEgress)
	// LSO specific required labels
	lsoOperatorRequiredLabels  = mergeLabels(npLabelLSOAPIServer, npLabelLSODNS, npLabelLSOOperatorMetrics)
	lsoDiskmakerRequiredLabels = mergeLabels(npLabelLSOAPIServer, npLabelLSODNS, npLabelLSODiskmakerMetrics)
)

type npCheck struct {
	Namespace            string
	Optional             bool
	RequiredPodSelectors []map[string]string
}

var networkPolicyChecks = []npCheck{
	{
		Namespace: CSONamespace,
		RequiredPodSelectors: []map[string]string{
			npLabelAPI,
			npLabelDNS,
			npLabelOperatorMetrics,
			npLabelAllEgress,
		},
	},
	{
		Namespace: CSINamespace,
		RequiredPodSelectors: []map[string]string{
			npLabelAPI,
			npLabelDNS,
			npLabelMetricsRange,
			npLabelOperatorMetricsRange,
			npLabelAllEgress,
		},
	},
	// TODO: Re-enable ManilaCSINamespace check once OCPBUGS-61175 is resolved
	// https://issues.redhat.com/browse/OCPBUGS-61175
	// {
	// 	Namespace: ManilaCSINamespace,
	// 	Optional:  true,
	// 	RequiredPodSelectors: []map[string]string{
	// 		npLabelAPI,
	// 		npLabelDNS,
	// 		npLabelMetricsRange,
	// 		npLabelOperatorMetricsRange,
	// 		npLabelAllEgress,
	// 	},
	// },
}

// getLSONetworkPolicyCheck returns the LSO network policy check configuration
// based on the detected LSO installation information
func getLSONetworkPolicyCheck(lso *lsoInfo) npCheck {
	return npCheck{
		Namespace: lso.Namespace,
		RequiredPodSelectors: []map[string]string{
			npLabelLSOAPIServer,
			npLabelLSODNS,
			npLabelLSOOperatorMetrics,
			npLabelLSODiskmakerMetrics,
		},
	}
}

var _ = g.Describe("[sig-storage][OCPFeature:StorageNetworkPolicy] Storage Network Policy", func() {
	defer g.GinkgoRecover()
	var (
		oc              = exutil.NewCLI("storage-network-policy")
		currentPlatform = e2e.TestContext.Provider
		lsoInstallInfo  *lsoInfo // LSO installation information detected once per suite
	)

	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Storage Network Policy tests are not supported on MicroShift")
		}

		// Detect LSO installation only once (cache the result)
		if lsoInstallInfo == nil {
			lsoInstallInfo, err = getLSOInfo(oc)
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to detect LSO installation")

			if lsoInstallInfo.Installed {
				supported := isLSOVersionSupported(lsoInstallInfo.Version)
				g.By(fmt.Sprintf("Detected LSO installed in namespace: %s, version: %s (network policy support: %v)",
					lsoInstallInfo.Namespace, lsoInstallInfo.Version, supported))
			} else {
				g.By("LSO is not installed on this cluster")
			}
		}
	})

	g.It("should verify required labels for CSO related Operators", func() {
		CSOResourcesToCheck := []resourceCheck{
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSONamespace,
				Name:           "cluster-storage-operator",
				Platform:       "all",
				RequiredLabels: csoOperatorRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSONamespace,
				Name:           "vsphere-problem-detector-operator",
				Platform:       "vsphere",
				RequiredLabels: csoOperatorWithAllEgressRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSONamespace,
				Name:           "csi-snapshot-controller-operator",
				Platform:       "all",
				RequiredLabels: csoOperatorRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSONamespace,
				Name:           "csi-snapshot-controller",
				Platform:       "all",
				RequiredLabels: csoControllerWithAllEgressRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSONamespace,
				Name:           "volume-data-source-validator",
				Platform:       "all",
				RequiredLabels: csoControllerRequiredLabels,
			},
		}
		runResourceChecks(oc, CSOResourcesToCheck, currentPlatform)
	})

	g.It("should verify required labels for CSI related Operators", func() {
		CSIResourcesToCheck := []resourceCheck{
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSINamespace,
				Name:           "aws-ebs-csi-driver-operator",
				Platform:       "aws",
				RequiredLabels: csiOperatorRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSINamespace,
				Name:           "aws-ebs-csi-driver-controller",
				Platform:       "aws",
				RequiredLabels: csiControllerWithAllEgressRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSINamespace,
				Name:           "aws-efs-csi-driver-operator",
				Platform:       "aws",
				RequiredLabels: csiOperatorRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSINamespace,
				Name:           "azure-disk-csi-driver-operator",
				Platform:       "azure",
				RequiredLabels: csiOperatorRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSINamespace,
				Name:           "azure-disk-csi-driver-controller",
				Platform:       "azure",
				RequiredLabels: csiControllerWithAllEgressRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSINamespace,
				Name:           "azure-file-csi-driver-operator",
				Platform:       "azure",
				RequiredLabels: csiOperatorRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSINamespace,
				Name:           "azure-file-csi-driver-controller",
				Platform:       "azure",
				RequiredLabels: csiControllerWithAllEgressRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSINamespace,
				Name:           "gcp-pd-csi-driver-operator",
				Platform:       "gcp",
				RequiredLabels: csiOperatorRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSINamespace,
				Name:           "gcp-filestore-csi-driver-operator",
				Platform:       "gcp",
				RequiredLabels: csiOperatorRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSINamespace,
				Name:           "vmware-vsphere-csi-driver-operator",
				Platform:       "vsphere",
				RequiredLabels: csiOperatorWithAllEgressRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSINamespace,
				Name:           "vmware-vsphere-csi-driver-controller",
				Platform:       "vsphere",
				RequiredLabels: csiControllerWithAllEgressRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSINamespace,
				Name:           "ibm-vpc-block-csi-driver-operator",
				Platform:       "ibmcloud",
				RequiredLabels: csiOperatorWithAllEgressRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSINamespace,
				Name:           "ibm-vpc-block-csi-controller",
				Platform:       "ibmcloud",
				RequiredLabels: csiControllerWithAllEgressRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSINamespace,
				Name:           "openstack-cinder-csi-driver-operator",
				Platform:       "openstack",
				RequiredLabels: csiOperatorWithAllEgressRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSINamespace,
				Name:           "openstack-cinder-csi-driver-controller",
				Platform:       "openstack",
				RequiredLabels: csiControllerWithAllEgressRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSINamespace,
				Name:           "manila-csi-driver-operator",
				Platform:       "openstack",
				RequiredLabels: csiOperatorWithAllEgressRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      ManilaCSINamespace,
				Name:           "openstack-manila-csi-controllerplugin",
				Platform:       "openstack",
				RequiredLabels: csiControllerWithAllEgressRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSINamespace,
				Name:           "smb-csi-driver-operator",
				Platform:       "all",
				RequiredLabels: csiOperatorRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      CSINamespace,
				Name:           "smb-csi-driver-controller",
				Platform:       "all",
				RequiredLabels: csiControllerWithAllEgressRequiredLabels,
			},
		}
		runResourceChecks(oc, CSIResourcesToCheck, currentPlatform)
	})

	g.It("should verify required labels for LSO related resources", func() {
		// Skip if LSO is not installed or version is lower than 4.21.0
		if !lsoInstallInfo.Installed {
			g.Skip("LSO is not installed on this cluster")
		}

		if !isLSOVersionSupported(lsoInstallInfo.Version) {
			g.Skip(fmt.Sprintf("LSO network policy support requires version >= 4.21.0, current version: %s", lsoInstallInfo.Version))
		}

		LSOResourcesToCheck := []resourceCheck{
			{
				ResourceType:   ResourceTypeDeployment,
				Namespace:      lsoInstallInfo.Namespace,
				Name:           "local-storage-operator",
				Platform:       "all",
				RequiredLabels: lsoOperatorRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDaemonSet,
				Namespace:      lsoInstallInfo.Namespace,
				Name:           "diskmaker-manager",
				Platform:       "all",
				RequiredLabels: lsoDiskmakerRequiredLabels,
			},
			{
				ResourceType:   ResourceTypeDaemonSet,
				Namespace:      lsoInstallInfo.Namespace,
				Name:           "diskmaker-discovery",
				Platform:       "all",
				RequiredLabels: lsoDiskmakerRequiredLabels,
			},
		}

		runResourceChecks(oc, LSOResourcesToCheck, currentPlatform)
	})

	g.It("should ensure required NetworkPolicies exist with correct labels for LSO", func() {
		// Skip if LSO is not installed or version is lower than 4.21.0
		if !lsoInstallInfo.Installed {
			g.Skip("LSO is not installed on this cluster")
		}

		if !isLSOVersionSupported(lsoInstallInfo.Version) {
			g.Skip(fmt.Sprintf("LSO network policy support requires version >= 4.21.0, current version: %s", lsoInstallInfo.Version))
		}

		// Get LSO network policy check configuration
		lsoCheck := getLSONetworkPolicyCheck(lsoInstallInfo)

		verifyNetworkPolicyPodSelectors(oc, lsoCheck.Namespace, lsoCheck.RequiredPodSelectors)
	})

	g.It("should ensure required NetworkPolicies exist with correct labels", func() {
		for _, c := range networkPolicyChecks {
			_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(context.TODO(), c.Namespace, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					if c.Optional {
						g.By(fmt.Sprintf("Skipping optional namespace %s (not found)", c.Namespace))
						continue
					}
					o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("namespace %s should exist", c.Namespace))
				}
				g.Fail(fmt.Sprintf("Error fetching namespace %s: %v", c.Namespace, err))
			}

			verifyNetworkPolicyPodSelectors(oc, c.Namespace, c.RequiredPodSelectors)
		}
	})
})

func runResourceChecks(oc *exutil.CLI, resources []resourceCheck, currentPlatform string) {
	results := []string{}
	hasFail := false
	for _, res := range resources {
		if res.Platform != "" && res.Platform != currentPlatform && res.Platform != "all" {
			results = append(results, fmt.Sprintf("[SKIP] %s %s/%s (platform mismatch: %s)", res.ResourceType, res.Namespace, res.Name, res.Platform))
			continue
		}

		var podTemplateLabels map[string]string
		resourceName := fmt.Sprintf("%s %s/%s", res.ResourceType, res.Namespace, res.Name)

		switch res.ResourceType {
		case ResourceTypeDeployment:
			deployment, err := oc.AdminKubeClient().AppsV1().Deployments(res.Namespace).Get(context.TODO(), res.Name, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					results = append(results, fmt.Sprintf("[SKIP] %s not found", resourceName))
					continue
				}
				g.Fail(fmt.Sprintf("Error fetching %s: %v", resourceName, err))
			}
			podTemplateLabels = deployment.Spec.Template.Labels

		case ResourceTypeDaemonSet:
			daemonset, err := oc.AdminKubeClient().AppsV1().DaemonSets(res.Namespace).Get(context.TODO(), res.Name, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					results = append(results, fmt.Sprintf("[SKIP] %s not found", resourceName))
					continue
				}
				g.Fail(fmt.Sprintf("Error fetching %s: %v", resourceName, err))
			}
			podTemplateLabels = daemonset.Spec.Template.Labels

		default:
			g.Fail(fmt.Sprintf("Unsupported resource type: %s", res.ResourceType))
		}

		missingLabels := []string{}
		for key, val := range res.RequiredLabels {
			if podTemplateLabels[key] != val {
				missingLabels = append(missingLabels, fmt.Sprintf("%s=%s", key, val))
			}
		}

		if len(missingLabels) > 0 {
			results = append(results, fmt.Sprintf("[FAIL] %s missing labels: %s", resourceName, strings.Join(missingLabels, ", ")))
			hasFail = true
		} else {
			results = append(results, fmt.Sprintf("[PASS] %s", resourceName))
		}
	}

	if hasFail {
		summary := strings.Join(results, "\n")
		g.Fail(fmt.Sprintf("Some resources are missing required labels:\n\n%s\n", summary))
	}
}

// isLSOVersionSupported checks if the LSO version is 4.21.0 or higher
// Supported version formats: "4.21.0", "4.21.0-202511252120"
func isLSOVersionSupported(versionStr string) bool {
	// Minimum required version for LSO network policy support
	minVersion := semver.MustParse("4.21.0")

	// Parse the LSO version
	// The version string may contain build metadata (e.g., "4.21.0-202511252120")
	// semver.Parse handles this correctly
	version, err := semver.Parse(versionStr)
	if err != nil {
		e2e.Logf("Failed to parse LSO version %q: %v", versionStr, err)
		return false
	}

	// Compare versions: returns true if version >= minVersion
	return version.GTE(minVersion)
}

// getLSOInfo detects if LSO is installed by searching for local-storage-operator CSV
// across all namespaces and returns its namespace and version information
func getLSOInfo(oc *exutil.CLI) (*lsoInfo, error) {
	info := &lsoInfo{
		Installed: false,
	}

	// Create controller-runtime client
	clusterConfig := oc.AdminConfig()
	clusterClient, err := client.New(clusterConfig, client.Options{})
	if err != nil {
		return info, fmt.Errorf("failed to create controller-runtime client: %v", err)
	}

	// Add operatorsv1alpha1 to scheme
	err = operatorsv1alpha1.AddToScheme(clusterClient.Scheme())
	if err != nil {
		return info, fmt.Errorf("failed to add operators.coreos.com/v1alpha1 to scheme: %v", err)
	}

	// List all ClusterServiceVersions across all namespaces
	csvList := &operatorsv1alpha1.ClusterServiceVersionList{}
	err = clusterClient.List(context.TODO(), csvList)
	if err != nil {
		return info, fmt.Errorf("failed to list ClusterServiceVersions: %v", err)
	}

	// Search for local-storage-operator CSV
	for _, csv := range csvList.Items {
		// Match CSV name pattern: local-storage-operator.*
		if strings.HasPrefix(csv.Name, "local-storage-operator") {
			// Only consider CSVs in Succeeded phase
			if csv.Status.Phase == operatorsv1alpha1.CSVPhaseSucceeded {
				info.Installed = true
				info.Namespace = csv.Namespace
				info.Version = csv.Spec.Version.String()
				return info, nil
			}
		}
	}

	// LSO not found or not in Succeeded phase
	return info, nil
}

// podSelectorContainsLabels checks if actualLabels contains all key-value pairs from requiredLabels
func podSelectorContainsLabels(actualLabels map[string]string, requiredLabels map[string]string) bool {
	for key, value := range requiredLabels {
		if actualLabels[key] != value {
			return false
		}
	}
	return true
}

// verifyNetworkPolicyPodSelectors verifies that for each required PodSelector, at least one NetworkPolicy has it
func verifyNetworkPolicyPodSelectors(oc *exutil.CLI, namespace string, requiredPodSelectors []map[string]string) {
	// List all NetworkPolicies in the namespace
	npList, err := oc.AdminKubeClient().NetworkingV1().NetworkPolicies(namespace).List(context.TODO(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("failed to list NetworkPolicies in namespace %s", namespace))

	// For each required PodSelector, verify that at least one NetworkPolicy has it
	for _, requiredSelector := range requiredPodSelectors {
		found := false
		var matchedNPName string

		for _, np := range npList.Items {
			if podSelectorContainsLabels(np.Spec.PodSelector.MatchLabels, requiredSelector) {
				found = true
				matchedNPName = np.Name
				break
			}
		}

		// Format the required selector for error messages
		labelPairs := []string{}
		for k, v := range requiredSelector {
			labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", k, v))
		}
		selectorDesc := strings.Join(labelPairs, ", ")

		if found {
			g.By(fmt.Sprintf("Found NetworkPolicy %s/%s with required PodSelector labels: %s", namespace, matchedNPName, selectorDesc))
		} else {
			o.Expect(found).To(o.BeTrue(), fmt.Sprintf("No NetworkPolicy in namespace %s has PodSelector with labels: %s", namespace, selectorDesc))
		}
	}
}

package storage

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// ResourceType defines the type of Kubernetes workload resource
type ResourceType string

const (
	ResourceTypeDeployment ResourceType = "Deployment"
	ResourceTypeDaemonSet  ResourceType = "DaemonSet"
)

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

var _ = g.Describe("[sig-storage][OCPFeature:StorageNetworkPolicy] Storage Network Policy", func() {
	defer g.GinkgoRecover()
	var (
		oc              = exutil.NewCLI("storage-network-policy")
		currentPlatform = e2e.TestContext.Provider
	)

	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Storage Network Policy tests are not supported on MicroShift")
		}
	})

	g.It("should verify required labels for CSO related Operators", g.Label("Size:S"), func() {
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

	g.It("should verify required labels for CSI related Operators", g.Label("Size:S"), func() {
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

	g.It("should ensure required NetworkPolicies exist with correct labels", g.Label("Size:S"), func() {
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

			// List all NetworkPolicies in the namespace
			npList, err := oc.AdminKubeClient().NetworkingV1().NetworkPolicies(c.Namespace).List(context.TODO(), metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("failed to list NetworkPolicies in namespace %s", c.Namespace))

			// For each required PodSelector, verify that at least one NetworkPolicy has it
			for _, requiredSelector := range c.RequiredPodSelectors {
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
					g.By(fmt.Sprintf("Found NetworkPolicy %s/%s with required PodSelector labels: %s", c.Namespace, matchedNPName, selectorDesc))
				} else {
					o.Expect(found).To(o.BeTrue(), fmt.Sprintf("No NetworkPolicy in namespace %s has PodSelector with labels: %s", c.Namespace, selectorDesc))
				}
			}
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

// podSelectorContainsLabels checks if actualLabels contains all key-value pairs from requiredLabels
func podSelectorContainsLabels(actualLabels map[string]string, requiredLabels map[string]string) bool {
	for key, value := range requiredLabels {
		if actualLabels[key] != value {
			return false
		}
	}
	return true
}

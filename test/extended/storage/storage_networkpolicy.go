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

type deploymentCheck struct {
	Namespace      string
	Name           string
	Platform       string
	RequiredLabels map[string]string
}

type npCheck struct {
	Namespace string
	Optional  bool
	Policies  map[string]map[string]string
}

var (
	npLabelAPI                  = map[string]string{"openshift.storage.network-policy.api-server": "allow"}
	npLabelDNS                  = map[string]string{"openshift.storage.network-policy.dns": "allow"}
	npLabelOperatorMetrics      = map[string]string{"openshift.storage.network-policy.operator-metrics": "allow"}
	npLabelOperatorMetricsRange = map[string]string{"openshift.storage.network-policy.operator-metrics-range": "allow"}
	npLabelMetricsRange         = map[string]string{"openshift.storage.network-policy.metrics-range": "allow"}
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
	csoOperatorRequiredLabels   = mergeLabels(npLabelAPI, npLabelDNS, npLabelOperatorMetrics)
	csoControllerRequiredLabels = mergeLabels(npLabelAPI, npLabelDNS)
	csiOperatorRequiredLabels   = mergeLabels(npLabelAPI, npLabelDNS, npLabelOperatorMetricsRange)
	csiControllerRequiredLabels = mergeLabels(npLabelAPI, npLabelDNS, npLabelMetricsRange)
)

var (
	npNameAPI                  = "allow-egress-to-api-server"
	npNameDNS                  = "allow-to-dns"
	npNameOperatorMetrics      = "allow-ingress-to-operator-metrics"
	npNameMetricsRange         = "allow-ingress-to-metrics-range"
	npNameOperatorMetricsRange = "allow-ingress-to-operator-metrics-range"
)

var networkPolicyChecks = []npCheck{
	{
		Namespace: CSONamespace,
		Policies: map[string]map[string]string{
			npNameAPI:             npLabelAPI,
			npNameDNS:             npLabelDNS,
			npNameOperatorMetrics: npLabelOperatorMetrics,
		},
	},
	{
		Namespace: CSINamespace,
		Policies: map[string]map[string]string{
			npNameAPI:                  npLabelAPI,
			npNameDNS:                  npLabelDNS,
			npNameMetricsRange:         npLabelMetricsRange,
			npNameOperatorMetricsRange: npLabelOperatorMetricsRange,
		},
	},
	{
		Namespace: ManilaCSINamespace,
		Optional:  true,
		Policies: map[string]map[string]string{
			npNameAPI:                  npLabelAPI,
			npNameDNS:                  npLabelDNS,
			npNameMetricsRange:         npLabelMetricsRange,
			npNameOperatorMetricsRange: npLabelOperatorMetricsRange,
		},
	},
}

var _ = g.Describe("[sig-storage][OCPFeature:StorageNetworkPolicy] Storage Network Policy", g.Ordered, g.Label("Conformance"), g.Label("Parallel"), func() {
	defer g.GinkgoRecover()
	var (
		oc              = exutil.NewCLI("storage-network-policy")
		currentPlatform = e2e.TestContext.Provider
	)

	g.It("should verify required labels for CSO related Operators", func() {
		CSODeploymentsToCheck := []deploymentCheck{
			{
				Namespace:      CSONamespace,
				Name:           "cluster-storage-operator",
				Platform:       "all",
				RequiredLabels: csoOperatorRequiredLabels,
			},
			{
				Namespace:      CSONamespace,
				Name:           "vsphere-problem-detector-operator",
				Platform:       "vsphere",
				RequiredLabels: csoOperatorRequiredLabels,
			},
			{
				Namespace:      CSONamespace,
				Name:           "csi-snapshot-controller-operator",
				Platform:       "all",
				RequiredLabels: csoOperatorRequiredLabels,
			},
			{
				Namespace:      CSONamespace,
				Name:           "csi-snapshot-controller",
				Platform:       "all",
				RequiredLabels: csoControllerRequiredLabels,
			},
		}
		runDeploymentChecks(oc, CSODeploymentsToCheck, currentPlatform)
	})

	g.It("should verify required labels for CSI related Operators", func() {
		CSIdeploymentsToCheck := []deploymentCheck{
			{
				Namespace:      CSINamespace,
				Name:           "aws-ebs-csi-driver-operator",
				Platform:       "aws",
				RequiredLabels: csiOperatorRequiredLabels,
			},
			{
				Namespace:      CSINamespace,
				Name:           "aws-ebs-csi-driver-controller",
				Platform:       "aws",
				RequiredLabels: csiControllerRequiredLabels,
			},
			{
				Namespace:      CSINamespace,
				Name:           "aws-efs-csi-driver-operator",
				Platform:       "aws",
				RequiredLabels: csiOperatorRequiredLabels,
			},
			{
				Namespace:      CSINamespace,
				Name:           "azure-disk-csi-driver-operator",
				Platform:       "azure",
				RequiredLabels: csiOperatorRequiredLabels,
			},
			{
				Namespace:      CSINamespace,
				Name:           "azure-disk-csi-driver-controller",
				Platform:       "azure",
				RequiredLabels: csiControllerRequiredLabels,
			},
			{
				Namespace:      CSINamespace,
				Name:           "azure-file-csi-driver-operator",
				Platform:       "azure",
				RequiredLabels: csiOperatorRequiredLabels,
			},
			{
				Namespace:      CSINamespace,
				Name:           "azure-file-csi-driver-controller",
				Platform:       "azure",
				RequiredLabels: csiControllerRequiredLabels,
			},
			{
				Namespace:      CSINamespace,
				Name:           "gcp-pd-csi-driver-operator",
				Platform:       "gcp",
				RequiredLabels: csiOperatorRequiredLabels,
			},
			{
				Namespace:      CSINamespace,
				Name:           "gcp-filestore-csi-driver-operator",
				Platform:       "gcp",
				RequiredLabels: csiOperatorRequiredLabels,
			},
			{
				Namespace:      CSINamespace,
				Name:           "vmware-vsphere-csi-driver-operator",
				Platform:       "vsphere",
				RequiredLabels: csiOperatorRequiredLabels,
			},
			{
				Namespace:      CSINamespace,
				Name:           "vmware-vsphere-csi-driver-controller",
				Platform:       "vsphere",
				RequiredLabels: csiControllerRequiredLabels,
			},
			{
				Namespace:      CSINamespace,
				Name:           "ibm-vpc-block-csi-driver-operator",
				Platform:       "ibmcloud",
				RequiredLabels: csiOperatorRequiredLabels,
			},
			{
				Namespace:      CSINamespace,
				Name:           "ibm-vpc-block-csi-controller",
				Platform:       "ibmcloud",
				RequiredLabels: csiControllerRequiredLabels,
			},
			{
				Namespace:      CSINamespace,
				Name:           "openstack-cinder-csi-driver-operator",
				Platform:       "openstack",
				RequiredLabels: csiOperatorRequiredLabels,
			},
			{
				Namespace:      CSINamespace,
				Name:           "openstack-cinder-csi-driver-controller",
				Platform:       "openstack",
				RequiredLabels: csiControllerRequiredLabels,
			},
			{
				Namespace:      CSINamespace,
				Name:           "manila-csi-driver-operator",
				Platform:       "openstack",
				RequiredLabels: csiOperatorRequiredLabels,
			},
			{
				Namespace:      ManilaCSINamespace,
				Name:           "openstack-manila-csi-controllerplugin",
				Platform:       "openstack",
				RequiredLabels: csiControllerRequiredLabels,
			},
			{
				Namespace:      CSINamespace,
				Name:           "smb-csi-driver-operator",
				Platform:       "all",
				RequiredLabels: csiOperatorRequiredLabels,
			},
			{
				Namespace:      CSINamespace,
				Name:           "smb-csi-driver-controller",
				Platform:       "all",
				RequiredLabels: csiControllerRequiredLabels,
			},
		}
		runDeploymentChecks(oc, CSIdeploymentsToCheck, currentPlatform)
	})

	g.It("should ensure required NetworkPolicies exist with correct labels", func() {
		for _, c := range networkPolicyChecks {
			_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(context.TODO(), c.Namespace, metav1.GetOptions{})
			if err != nil {
				if c.Optional {
					g.By(fmt.Sprintf("Skipping optional namespace %s (not found)", c.Namespace))
					continue
				}
				o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("namespace %s should exist", c.Namespace))
			}

			for npName, labels := range c.Policies {
				np, err := oc.AdminKubeClient().NetworkingV1().NetworkPolicies(c.Namespace).Get(context.TODO(), npName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("NetworkPolicy %s/%s should exist", c.Namespace, npName))

				for key, val := range labels {
					gotVal, ok := np.Spec.PodSelector.MatchLabels[key]
					o.Expect(ok).To(o.BeTrue(), fmt.Sprintf("NetworkPolicy %s/%s missing label %s", c.Namespace, npName, key))
					o.Expect(gotVal).To(o.Equal(val), fmt.Sprintf("NetworkPolicy %s/%s label %s mismatch (got=%s, want=%s)", c.Namespace, npName, key, gotVal, val))
				}
			}
		}
	})
})

func runDeploymentChecks(oc *exutil.CLI, deployments []deploymentCheck, currentPlatform string) {
	results := []string{}
	hasFail := false
	for _, dep := range deployments {
		if dep.Platform != "" && dep.Platform != currentPlatform && dep.Platform != "all" {
			results = append(results, fmt.Sprintf("[SKIP] %s/%s (platform mismatch: %s)", dep.Namespace, dep.Name, dep.Platform))
			continue
		}

		deployment, err := oc.AdminKubeClient().AppsV1().Deployments(dep.Namespace).Get(context.TODO(), dep.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				results = append(results, fmt.Sprintf("[SKIP] %s/%s not found", dep.Namespace, dep.Name))
				continue
			}
			g.Fail(fmt.Sprintf("Error fetching deployment %s/%s: %v", dep.Namespace, dep.Name, err))
		}

		missingLabels := []string{}
		for key, val := range dep.RequiredLabels {
			if deployment.Spec.Template.Labels[key] != val {
				missingLabels = append(missingLabels, fmt.Sprintf("%s=%s", key, val))
			}
		}

		if len(missingLabels) > 0 {
			results = append(results, fmt.Sprintf("[FAIL] %s/%s missing labels: %s", dep.Namespace, dep.Name, strings.Join(missingLabels, ", ")))
			hasFail = true
		} else {
			results = append(results, fmt.Sprintf("[PASS] %s/%s", dep.Namespace, dep.Name))
		}
	}

	if hasFail {
		summary := strings.Join(results, "\n")
		g.Fail(fmt.Sprintf("Some deployments are missing required labels:\n\n%s\n", summary))
	}
}

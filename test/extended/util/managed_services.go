package util

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
)

// ManagedServiceNamespaces is the set of namespaces used by managed service platforms
// like ROSA, ARO, etc. These are typically exempt from the requirements we impose on
// core platform namespaces. Consulted https://github.com/openshift/managed-cluster-config/blob/master/deploy/osd-managed-resources/managed-namespaces.ConfigMap.yaml,
// to pull out openshift-* namespaces that aren't owned by OCP.
var ManagedServiceNamespaces = sets.New[string](
	"openshift-addon-operator",
	"openshift-aqua",
	"openshift-aws-vpce-operator",
	"openshift-backplane",
	"openshift-backplane-cee",
	"openshift-backplane-csa",
	"openshift-backplane-cse",
	"openshift-backplane-csm",
	"openshift-backplane-managed-scripts",
	"openshift-backplane-mcs-tier-two",
	"openshift-backplane-mobb",
	"openshift-backplane-sdcicd",
	"openshift-backplane-srep",
	"openshift-backplane-tam",
	"openshift-cloud-ingress-operator",
	"openshift-codeready-workspaces",
	"openshift-compliance",
	"openshift-compliance-monkey",
	"openshift-container-security",
	"openshift-custom-domains-operator",
	"openshift-customer-monitoring",
	"openshift-deployment-validation-operator",
	"openshift-file-integrity",
	"openshift-logging",
	"openshift-managed-node-metadata-operator",
	"openshift-managed-upgrade-operator",
	"openshift-marketplace",
	"openshift-must-gather-operator",
	"openshift-nmstate",
	"openshift-observability-operator",
	"openshift-ocm-agent-operator",
	"openshift-operators-redhat",
	"openshift-osd-metrics",
	"openshift-package-operator",
	"openshift-rbac-permissions",
	"openshift-route-monitor-operator",
	"openshift-scanning",
	"openshift-security",
	"openshift-splunk-forwarder-operator",
	"openshift-sre-pruning",
	"openshift-suricata",
	"openshift-validation-webhook",
	"openshift-velero",
)

// IsAroHCP checks if the HyperShift operator deployment has MANAGED_SERVICE=ARO-HCP environment variable.
func IsAroHCP(ctx context.Context, namespace string, kubeClient kubernetes.Interface) (bool, error) {
	// List deployments with the correct label that actually exists on the deployment
	deployments, err := kubeClient.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "hypershift.openshift.io/managed-by=control-plane-operator",
	})
	if err != nil {
		logrus.Infof("Failed to list deployments in namespace %s: %v", namespace, err)
		return false, nil // Not an error if we can't list deployments, just means it's not ARO HCP
	}

	if len(deployments.Items) == 0 {
		logrus.Infof("No control-plane-operator deployments found in namespace %s", namespace)
		return false, nil
	}

	logrus.Infof("Found %d control-plane-operator deployments in namespace %s", len(deployments.Items), namespace)

	// Look through all matching deployments
	for _, deployment := range deployments.Items {

		// Look for the control-plane-operator container directly in the deployment spec
		for _, container := range deployment.Spec.Template.Spec.Containers {
			if container.Name == "control-plane-operator" {
				logrus.Infof("Found container 'control-plane-operator' in deployment %s", deployment.Name)

				result := HasEnvVar(&container, "MANAGED_SERVICE", "ARO-HCP")
				logrus.Infof("hasEnvVar result for MANAGED_SERVICE=ARO-HCP: %v", result)
				return result, nil
			}
		}
	}

	logrus.Infof("No deployment found with control-plane-operator container in namespace %s", namespace)
	return false, nil
}

// IsBareMetalHyperShiftCluster checks if the HyperShift cluster is running on bare metal
// by checking the platform type of the hosted cluster. It uses kubectl commands to query
// the hosted cluster's platform type and returns true if it's "None" or "Agent".
func IsBareMetalHyperShiftCluster(ctx context.Context, managementOC *CLI) (bool, error) {
	// Get the hosted cluster namespace
	_, hcpNamespace, err := GetHypershiftManagementClusterConfigAndNamespace()
	if err != nil {
		return false, fmt.Errorf("failed to get hypershift management cluster config and namespace: %v", err)
	}

	// Get the first hosted cluster name
	clusterNames, err := managementOC.AsAdmin().WithoutNamespace().Run("get").Args(
		"-n", hcpNamespace, "hostedclusters", "-o=jsonpath={.items[*].metadata.name}").Output()
	if err != nil {
		return false, fmt.Errorf("failed to get hosted cluster names: %v", err)
	}

	if len(clusterNames) == 0 {
		return false, fmt.Errorf("no hosted clusters found")
	}

	// Get the first hosted cluster name
	clusterName := strings.Split(strings.TrimSpace(clusterNames), " ")[0]

	// Get the platform type of the hosted cluster
	platformType, err := managementOC.AsAdmin().WithoutNamespace().Run("get").Args(
		"hostedcluster", clusterName, "-n", hcpNamespace, `-ojsonpath={.spec.platform.type}`).Output()
	if err != nil {
		return false, fmt.Errorf("failed to get hosted cluster platform type: %v", err)
	}

	// Check if it's bare metal (None or Agent platform)
	platformTypeStr := strings.TrimSpace(platformType)
	return platformTypeStr == "None" || platformTypeStr == "Agent", nil
}

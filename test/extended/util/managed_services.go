package util

import (
	"context"

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

package alerts

import (
	"context"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	helper "github.com/openshift/origin/test/extended/util/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AllowedAlertsDuringConformance lists all alerts that are allowed to be pending or firing during
// conformance testing.
// WARNING: there is a parallel list for allowed alerts during upgrade in allowed_upgrade.go,
// ensure that alerts we want to allow in both are added to both.
func AllowedAlertsDuringConformance(configClient configclient.Interface) (allowedFiringWithBugs, allowedFiring, allowedPendingWithBugs, allowedPending helper.MetricConditions) {

	firingAlertsWithBugs := helper.MetricConditions{}
	allowedFiringAlerts := helper.MetricConditions{
		{
			Selector: map[string]string{"alertname": "TargetDown", "namespace": "openshift-e2e-loki"},
			Text:     "Loki is nice to have, but we can allow it to be down",
		},
		{
			Selector: map[string]string{"alertname": "KubePodNotReady", "namespace": "openshift-e2e-loki"},
			Text:     "Loki is nice to have, but we can allow it to be down",
		},
		{
			Selector: map[string]string{"alertname": "KubeDeploymentReplicasMismatch", "namespace": "openshift-e2e-loki"},
			Text:     "Loki is nice to have, but we can allow it to be down",
		},
		{
			Selector: map[string]string{"alertname": "HighOverallControlPlaneCPU"},
			Text:     "high CPU utilization during e2e runs is normal",
		},
		{
			Selector: map[string]string{"alertname": "ExtremelyHighIndividualControlPlaneCPU"},
			Text:     "high CPU utilization during e2e runs is normal",
		},
	}
	pendingAlertsWithBugs := helper.MetricConditions{}
	allowedPendingAlerts := helper.MetricConditions{
		{
			Selector: map[string]string{"alertname": "HighOverallControlPlaneCPU"},
			Text:     "high CPU utilization during e2e runs is normal",
		},
		{
			Selector: map[string]string{"alertname": "ExtremelyHighIndividualControlPlaneCPU"},
			Text:     "high CPU utilization during e2e runs is normal",
		},
	}

	if featureGates, err := configClient.ConfigV1().FeatureGates().Get(context.TODO(), "cluster", metav1.GetOptions{}); err == nil {
		switch featureGates.Spec.FeatureSet {
		case configv1.TechPreviewNoUpgrade:
			allowedFiringAlerts = append(
				allowedFiringAlerts,
				helper.MetricCondition{
					Selector: map[string]string{"alertname": "TechPreviewNoUpgrade"},
					Text:     "Allow testing of TechPreviewNoUpgrade clusters, this will only fire when a FeatureGate has been enabled",
				},
				helper.MetricCondition{
					Selector: map[string]string{"alertname": "ClusterNotUpgradeable"},
					Text:     "Allow testing of ClusterNotUpgradeable clusters, this will only fire when a FeatureGate has been enabled",
				})
		}
	}

	return firingAlertsWithBugs, allowedFiringAlerts, pendingAlertsWithBugs, allowedPendingAlerts
}

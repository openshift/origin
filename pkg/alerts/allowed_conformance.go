package alerts

import (
	"context"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	helper "github.com/openshift/origin/test/extended/util/prometheus"
	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

// AllowedAlertsDuringConformance lists all alerts that are allowed to be pending or firing during
// conformance testing.
// WARNING: there is a parallel list for allowed alerts during upgrade in allowed_upgrade.go,
// ensure that alerts we want to allow in both are added to both.
func AllowedAlertsDuringConformance(configClient configclient.Interface) (allowedFiringWithBugs, allowedFiring, allowedPendingWithBugs, allowedPending helper.MetricConditions) {

	firingAlertsWithBugs := helper.MetricConditions{
		{
			Selector: map[string]string{"alertname": "ClusterOperatorDegraded", "name": "authentication"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1939580",
		},
		{
			Selector: map[string]string{"alertname": "ClusterOperatorDown", "name": "authentication"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1939580",
		},
		{
			Selector: map[string]string{"alertname": "HighlyAvailableWorkloadIncorrectlySpread", "namespace": "openshift-monitoring", "workload": "prometheus-k8s"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1949262",
		},
		{
			Selector: map[string]string{"alertname": "HighlyAvailableWorkloadIncorrectlySpread", "namespace": "openshift-monitoring", "workload": "alertmanager-main"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1955489",
		},
		{
			Selector: map[string]string{"alertname": "KubeAPIErrorBudgetBurn"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1953798",
			Matches: func(_ *model.Sample) bool {
				return framework.ProviderIs("gce")
			},
		},
		{
			Selector: map[string]string{"alertname": "KubeJobFailed", "namespace": "openshift-multus"}, // not sure how to do a job_name prefix
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=2054426",
		},
	}
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

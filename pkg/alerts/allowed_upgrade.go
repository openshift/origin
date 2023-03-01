package alerts

import (
	configv1 "github.com/openshift/api/config/v1"
	helper "github.com/openshift/origin/test/extended/util/prometheus"
)

// AllowedAlertsDuringUpgrade lists all alerts that are allowed to be pending or firing during
// upgrade.
// WARNING: there is a parallel list for allowed alerts during conformance in allowed_conformance.go,
// ensure that alerts we want to allow in both are added to both.
func AllowedAlertsDuringUpgrade(featureSet configv1.FeatureSet) (allowedFiringWithBugs, allowedFiring, allowedPendingWithBugs, allowedPending helper.MetricConditions) {

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
	}
	pendingAlertsWithBugs := helper.MetricConditions{}
	allowedPendingAlerts := helper.MetricConditions{
		{
			Selector: map[string]string{"alertname": "etcdMemberCommunicationSlow"},
			Text:     "Excluded because it triggers during upgrade (detects ~5m of high latency immediately preceeding the end of the test), and we don't want to change the alert because it is correct",
		},
	}

	switch featureSet {
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

	return firingAlertsWithBugs, allowedFiringAlerts, pendingAlertsWithBugs, allowedPendingAlerts
}

package alerts

import (
	configv1 "github.com/openshift/api/config/v1"
)

// AllowedAlertsDuringConformance lists all alerts that are allowed to be pending or firing during
// conformance testing.
// WARNING: there is a parallel list for allowed alerts during upgrade in allowed_upgrade.go,
// ensure that alerts we want to allow in both are added to both.
func AllowedAlertsDuringConformance(featureSet configv1.FeatureSet) (allowedFiringWithBugs, allowedFiring, allowedPendingWithBugs, allowedPending MetricConditions) {

	firingAlertsWithBugs := MetricConditions{
		{
			AlertName: "VirtHandlerRESTErrorsHigh",
			Text:      "https://issues.redhat.com/browse/CNV-50418",
		},
		{
			AlertName: "VirtControllerRESTErrorsHigh",
			Text:      "https://issues.redhat.com/browse/CNV-50418",
		},
	}
	allowedFiringAlerts := MetricConditions{
		{
			AlertName:      "TargetDown",
			AlertNamespace: "openshift-e2e-loki",
			Text:           "Loki is nice to have, but we can allow it to be down",
		},
		{
			AlertName:      "KubePodNotReady",
			AlertNamespace: "openshift-e2e-loki",
			Text:           "Loki is nice to have, but we can allow it to be down",
		},
		{
			AlertName:      "KubeDeploymentReplicasMismatch",
			AlertNamespace: "openshift-e2e-loki",
			Text:           "Loki is nice to have, but we can allow it to be down",
		},
		{
			AlertName: "HighOverallControlPlaneCPU",
			Text:      "high CPU utilization during e2e runs is normal",
		},
		{
			AlertName: "ExtremelyHighIndividualControlPlaneCPU",
			Text:      "high CPU utilization during e2e runs is normal",
		},
		{
			AlertName: "CDIDefaultStorageClassDegraded",
			Text:      "not having rwx storage class should not be a must",
		},
	}
	pendingAlertsWithBugs := MetricConditions{}
	allowedPendingAlerts := MetricConditions{
		{
			AlertName: "HighOverallControlPlaneCPU",
			Text:      "high CPU utilization during e2e runs is normal",
		},
		{
			AlertName: "ExtremelyHighIndividualControlPlaneCPU",
			Text:      "high CPU utilization during e2e runs is normal",
		},
	}

	switch featureSet {
	case configv1.TechPreviewNoUpgrade:
		allowedFiringAlerts = append(
			allowedFiringAlerts,
			MetricCondition{
				AlertName: "TechPreviewNoUpgrade",
				Text:      "Allow testing of TechPreviewNoUpgrade clusters, this will only fire when a FeatureGate has been enabled",
			},
			MetricCondition{
				AlertName: "ClusterNotUpgradeable",
				Text:      "Allow testing of ClusterNotUpgradeable clusters, this will only fire when a FeatureGate has been enabled",
			})
	}

	return firingAlertsWithBugs, allowedFiringAlerts, pendingAlertsWithBugs, allowedPendingAlerts
}

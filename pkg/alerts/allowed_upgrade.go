package alerts

import (
	"context"
	"strings"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	helper "github.com/openshift/origin/test/extended/util/prometheus"
	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

// AllowedAlertsDuringUpgrade lists all alerts that are allowed to be pending or firing during
// upgrade.
// WARNING: there is a parallel list for allowed alerts during conformance in allowed_conformance.go,
// ensure that alerts we want to allow in both are added to both.
func AllowedAlertsDuringUpgrade(configClient configclient.Interface) (allowedFiringWithBugs, allowedFiring, allowedPendingWithBugs, allowedPending helper.MetricConditions) {

	firingAlertsWithBugs := helper.MetricConditions{
		{
			Selector: map[string]string{"alertname": "AggregatedAPIDown", "namespace": "default", "name": "v1beta1.metrics.k8s.io"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1970624",
			Matches: func(_ *model.Sample) bool {
				return framework.ProviderIs("gce")
			},
		},
		{
			Selector: map[string]string{"alertname": "ClusterOperatorDegraded", "name": "authentication"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1939580",
		},
		{
			Selector: map[string]string{"alertname": "ClusterOperatorDegraded", "name": "openshift-apiserver"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1939580",
		},
		{
			Selector: map[string]string{"alertname": "ClusterOperatorDown", "name": "authentication"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1939580",
		},
		{
			Selector: map[string]string{"alertname": "ClusterOperatorDown", "name": "machine-config"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1955300",
		},
		{
			// Should be removed one release after the attached bugzilla is fixed, or after that bug is fixed in a backport to the previous minor.
			Selector: map[string]string{"alertname": "ExtremelyHighIndividualControlPlaneCPU"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1985073",
			Matches: func(_ *model.Sample) bool {
				return framework.ProviderIs("gce")
			},
		},
		{
			// Should be removed one release after the attached bugzilla is fixed.
			Selector: map[string]string{"alertname": "HighlyAvailableWorkloadIncorrectlySpread", "namespace": "openshift-monitoring", "workload": "prometheus-k8s"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1949262",
		},
		{
			// Should be removed one release after the attached bugzilla is fixed.
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
			Selector: map[string]string{"alertname": "KubeDaemonSetRolloutStuck"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1943667",
		},
		{
			Selector: map[string]string{"alertname": "KubeJobFailed", "namespace": "openshift-multus"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=2054426",
			Matches: func(sample *model.Sample) bool {
				// Only match if the job_name label starts with ip-reconciler:
				if strings.HasPrefix(string(sample.Metric[model.LabelName("job_name")]), "ip-reconciler-") {
					return true
				}
				return false
			},
		},
		{
			Selector: map[string]string{"alertname": "KubePodNotReady", "namespace": "openshift-kube-apiserver-operator"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1939580",
		},
		{
			Selector: map[string]string{"alertname": "KubePodNotReady", "namespace": "openshift-kube-apiserver-operator"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1939580",
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
	}
	pendingAlertsWithBugs := helper.MetricConditions{
		{
			Selector: map[string]string{"alertname": "ClusterMonitoringOperatorReconciliationErrors"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1932624",
		},
		{
			Selector: map[string]string{"alertname": "KubeClientErrors"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=1925698",
		},
		{
			Selector: map[string]string{"alertname": "NetworkPodsCrashLooping"},
			Text:     "https://bugzilla.redhat.com/show_bug.cgi?id=2009078",
		},
	}
	allowedPendingAlerts := helper.MetricConditions{
		{
			Selector: map[string]string{"alertname": "etcdMemberCommunicationSlow"},
			Text:     "Excluded because it triggers during upgrade (detects ~5m of high latency immediately preceeding the end of the test), and we don't want to change the alert because it is correct",
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

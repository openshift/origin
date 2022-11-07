package alerts

import (
	"context"
	"fmt"
	"strings"

	o "github.com/onsi/gomega"
	"github.com/openshift/origin/pkg/synthetictests/allowedalerts"
	testresult "github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	helper "github.com/openshift/origin/test/extended/util/prometheus"
	"github.com/prometheus/common/model"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/test/e2e/framework"
)

func main2() {
	// Watchdog and AlertmanagerReceiversNotConfigured are expected.
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

	if isTechPreviewCluster(oc) {
		allowedFiringAlerts = append(
			allowedFiringAlerts,
			helper.MetricCondition{
				Selector: map[string]string{"alertname": "TechPreviewNoUpgrade"},
				Text:     "Allow testing of TechPreviewNoUpgrade clusters, this will only fire when a FeatureGate has been installed",
			},
			helper.MetricCondition{
				Selector: map[string]string{"alertname": "ClusterNotUpgradeable"},
				Text:     "Allow testing of ClusterNotUpgradeable clusters, this will only fire when a FeatureGate has been installed",
			})
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

	// we exclude alerts that have their own separate tests.
	for _, alertTest := range allowedalerts.AllAlertTests(context.TODO(), nil, 0) {
		switch alertTest.AlertState() {
		case allowedalerts.AlertPending:
			// a pending test covers pending and everything above (firing)
			allowedPendingAlerts = append(allowedPendingAlerts,
				helper.MetricCondition{
					Selector: map[string]string{"alertname": alertTest.AlertName()},
					Text:     "has a separate e2e test",
				},
			)
			allowedFiringAlerts = append(allowedFiringAlerts,
				helper.MetricCondition{
					Selector: map[string]string{"alertname": alertTest.AlertName()},
					Text:     "has a separate e2e test",
				},
			)
		case allowedalerts.AlertInfo:
			// an info test covers all firing
			allowedFiringAlerts = append(allowedFiringAlerts,
				helper.MetricCondition{
					Selector: map[string]string{"alertname": alertTest.AlertName()},
					Text:     "has a separate e2e test",
				},
			)
		}
	}

	knownViolations := sets.NewString()
	unexpectedViolations := sets.NewString()
	unexpectedViolationsAsFlakes := sets.NewString()
	debug := sets.NewString()

	// we only consider samples since the beginning of the test
	testDuration := exutil.DurationSinceStartInSeconds().String()

	// Invariant: No non-info level alerts should have fired during the test run
	firingAlertQuery := fmt.Sprintf(`
sort_desc(
count_over_time(ALERTS{alertstate="firing",severity!="info",alertname!~"Watchdog|AlertmanagerReceiversNotConfigured"}[%[1]s:1s])
) > 0
`, testDuration)
	result, err := helper.RunQuery(context.TODO(), oc.NewPrometheusClient(context.TODO()), firingAlertQuery)
	o.Expect(err).NotTo(o.HaveOccurred(), "unable to check firing alerts during test")
	for _, series := range result.Data.Result {
		labels := helper.StripLabels(series.Metric, "alertname", "alertstate", "prometheus")
		violation := fmt.Sprintf("alert %s fired for %s seconds with labels: %s", series.Metric["alertname"], series.Value, helper.LabelsAsSelector(labels))
		if cause := allowedFiringAlerts.Matches(series); cause != nil {
			debug.Insert(fmt.Sprintf("%s (allowed: %s)", violation, cause.Text))
			continue
		}
		if cause := firingAlertsWithBugs.Matches(series); cause != nil {
			knownViolations.Insert(fmt.Sprintf("%s (open bug: %s)", violation, cause.Text))
		} else {
			unexpectedViolations.Insert(violation)
		}
	}

	// Invariant: There should be no pending alerts after the test run
	pendingAlertQuery := fmt.Sprintf(`
sort_desc(
  time() * ALERTS + 1
  -
  last_over_time((
    time() * ALERTS{alertname!~"Watchdog|AlertmanagerReceiversNotConfigured",alertstate="pending",severity!="info"}
    unless
    ALERTS offset 1s
  )[%[1]s:1s])
)
`, testDuration)
	result, err = helper.RunQuery(context.TODO(), oc.NewPrometheusClient(context.TODO()), pendingAlertQuery)
	o.Expect(err).NotTo(o.HaveOccurred(), "unable to retrieve pending alerts after upgrade")
	for _, series := range result.Data.Result {
		labels := helper.StripLabels(series.Metric, "alertname", "alertstate", "prometheus")
		violation := fmt.Sprintf("alert %s pending for %s seconds with labels: %s", series.Metric["alertname"], series.Value, helper.LabelsAsSelector(labels))
		if cause := allowedPendingAlerts.Matches(series); cause != nil {
			debug.Insert(fmt.Sprintf("%s (allowed: %s)", violation, cause.Text))
			continue
		}
		if cause := pendingAlertsWithBugs.Matches(series); cause != nil {
			knownViolations.Insert(fmt.Sprintf("%s (open bug: %s)", violation, cause.Text))
		} else {
			// treat pending errors as a flake right now because we are still trying to determine the scope
			// TODO: move this to unexpectedViolations later
			unexpectedViolationsAsFlakes.Insert(violation)
		}
	}

	if len(debug) > 0 {
		framework.Logf("Alerts were detected during test run which are allowed:\n\n%s", strings.Join(debug.List(), "\n"))
	}
	if len(unexpectedViolations) > 0 {
		framework.Failf("Unexpected alerts fired or pending after the test run:\n\n%s", strings.Join(unexpectedViolations.List(), "\n"))
	}
	if flakes := sets.NewString().Union(knownViolations).Union(unexpectedViolations).Union(unexpectedViolationsAsFlakes); len(flakes) > 0 {
		testresult.Flakef("Unexpected alert behavior during test:\n\n%s", strings.Join(flakes.List(), "\n"))
	}
	framework.Logf("No alerts fired during test run")

}

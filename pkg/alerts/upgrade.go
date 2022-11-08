package alerts

import (
	"context"
	"fmt"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/synthetictests/allowedalerts"
	"github.com/openshift/origin/test/extended/util/disruption"
	helper "github.com/openshift/origin/test/extended/util/prometheus"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/test/e2e/framework"
)

type allowedAlertsFunc func(configclient.Interface) (allowedFiringWithBugs, allowedFiring, allowedPendingWithBugs, allowedPending helper.MetricConditions)

func CheckAlerts(allowancesFunc allowedAlertsFunc, prometheusClient prometheusv1.API, configClient configclient.Interface, startTime time.Time) {
	firingAlertsWithBugs, allowedFiringAlerts, pendingAlertsWithBugs, allowedPendingAlerts :=
		allowancesFunc(configClient)

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

	testDuration := time.Now().Sub(startTime).Round(time.Second)

	// Invariant: No non-info level alerts should have fired during the upgrade
	firingAlertQuery := fmt.Sprintf(`
sort_desc(
count_over_time(ALERTS{alertstate="firing",severity!="info",alertname!~"Watchdog|AlertmanagerReceiversNotConfigured"}[%[1]s:1s])
) > 0
`, testDuration)
	result, err := helper.RunQuery(context.TODO(), prometheusClient, firingAlertQuery)
	o.Expect(err).NotTo(o.HaveOccurred(), "unable to check firing alerts during upgrade")
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

	// Invariant: There should be no pending alerts 1m after the upgrade completes
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
	result, err = helper.RunQuery(context.TODO(), prometheusClient, pendingAlertQuery)
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
		framework.Logf("Alerts were detected during upgrade which are allowed:\n\n%s", strings.Join(debug.List(), "\n"))
	}
	if len(unexpectedViolations) > 0 {
		framework.Failf("Unexpected alerts fired or pending during the upgrade:\n\n%s", strings.Join(unexpectedViolations.List(), "\n"))
	}
	if flakes := sets.NewString().Union(knownViolations).Union(unexpectedViolations).Union(unexpectedViolationsAsFlakes); len(flakes) > 0 {
		disruption.FrameworkFlakef(f, "Unexpected alert behavior during upgrade:\n\n%s", strings.Join(flakes.List(), "\n"))

		/*
			framework.Logf(format, options...)
			f.TestSummaries = append(f.TestSummaries, flakeSummary(fmt.Sprintf(format, options...)))
		*/
	}
	framework.Logf("No alerts fired during upgrade")

}

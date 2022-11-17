package alerts

import (
	"context"
	"fmt"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/synthetictests/allowedalerts"
	testresult "github.com/openshift/origin/pkg/test/ginkgo/result"
	"github.com/openshift/origin/test/extended/util/disruption"
	helper "github.com/openshift/origin/test/extended/util/prometheus"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/test/e2e/framework"
)

type allowedAlertsFunc func(configclient.Interface) (allowedFiringWithBugs, allowedFiring, allowedPendingWithBugs, allowedPending helper.MetricConditions)

// CheckAlerts will query prometheus and ensure no-unexpected alerts were pending or firing.
// Used both post-upgrade and post-conformance, with different allowances for each.
func CheckAlerts(allowancesFunc allowedAlertsFunc, prometheusClient prometheusv1.API, configClient configclient.Interface, testDuration time.Duration, f *framework.Framework) {
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

	// Invariant: No non-info level alerts should have fired
	firingAlertQuery := fmt.Sprintf(`
sort_desc(
count_over_time(ALERTS{alertstate="firing",severity!="info",alertname!~"Watchdog|AlertmanagerReceiversNotConfigured"}[%[1]s:1s])
) > 0
`, testDuration)
	result, err := helper.RunQuery(context.TODO(), prometheusClient, firingAlertQuery)
	o.Expect(err).NotTo(o.HaveOccurred(), "unable to check firing alerts")
	for _, series := range result.Data.Result {
		labels := helper.StripLabels(series.Metric, "alertname", "alertstate", "prometheus")
		violation := fmt.Sprintf("alert %s fired for %s seconds with labels: %s", series.Metric["alertname"], series.Value, helper.LabelsAsSelector(labels))
		if cause := allowedFiringAlerts.Matches(series); cause != nil {
			// TODO: this seems to never be firing? no search.ci results show allowed
			debug.Insert(fmt.Sprintf("%s result=allow (%s)", violation, cause.Text))
			continue
		}
		if cause := firingAlertsWithBugs.Matches(series); cause != nil {
			knownViolations.Insert(fmt.Sprintf("%s result=allow bug=%s", violation, cause.Text))
		} else {
			unexpectedViolations.Insert(fmt.Sprintf("%s result=reject", violation))
		}
	}

	// Invariant: There should be no pending alerts
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
	o.Expect(err).NotTo(o.HaveOccurred(), "unable to retrieve pending alerts")
	for _, series := range result.Data.Result {
		labels := helper.StripLabels(series.Metric, "alertname", "alertstate", "prometheus")
		violation := fmt.Sprintf("alert %s pending for %s seconds with labels: %s", series.Metric["alertname"], series.Value, helper.LabelsAsSelector(labels))
		if cause := allowedPendingAlerts.Matches(series); cause != nil {
			// TODO: this seems to never be firing? no search.ci results show allowed
			debug.Insert(fmt.Sprintf("%s result=allow (%s)", violation, cause.Text))
			continue
		}
		if cause := pendingAlertsWithBugs.Matches(series); cause != nil {
			knownViolations.Insert(fmt.Sprintf("%s result=allow bug=%s", violation, cause.Text))
		} else {
			// treat pending errors as a flake right now because we are still trying to determine the scope
			// TODO: move this to unexpectedViolations later
			unexpectedViolationsAsFlakes.Insert(fmt.Sprintf("%s result=allow", violation))
		}
	}

	if len(debug) > 0 {
		framework.Logf("Alerts were detected which are allowed:\n\n%s", strings.Join(debug.List(), "\n"))
	}
	if flakes := sets.NewString().Union(knownViolations).Union(unexpectedViolations).Union(unexpectedViolationsAsFlakes); len(flakes) > 0 {
		// TODO: The two tests that had this duplicated code had slightly different ways of reporting flakes
		// that I do not fully understand the implications of. Fork the logic here.
		if f != nil {
			// when called from alert.go within an UpgradeTest with a framework available
			// f.TestSummaries is the part I'm unsure about here.
			disruption.FrameworkFlakef(f, "Unexpected alert behavior:\n\n%s", strings.Join(flakes.List(), "\n"))
		} else {
			// when called from prometheus.go with no framework available
			testresult.Flakef("Unexpected alert behavior:\n\n%s", strings.Join(flakes.List(), "\n"))
		}
	}
	framework.Logf("No alerts fired")

}

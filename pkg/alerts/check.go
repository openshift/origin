package alerts

import (
	"context"
	"fmt"
	"strings"
	"time"

	allowedalerts2 "github.com/openshift/origin/pkg/monitortestlibrary/allowedalerts"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"

	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	testresult "github.com/openshift/origin/pkg/test/ginkgo/result"
	"github.com/openshift/origin/test/extended/util/disruption"
	helper "github.com/openshift/origin/test/extended/util/prometheus"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
)

type AllowedAlertsFunc func(featureSet configv1.FeatureSet) (allowedFiringWithBugs, allowedFiring, allowedPendingWithBugs, allowedPending helper.MetricConditions)

// CheckAlerts will query prometheus and ensure no-unexpected alerts were pending or firing.
// Used by both upgrade and conformance suites, with different allowances for each.
func CheckAlerts(allowancesFunc AllowedAlertsFunc,
	restConfig *rest.Config,
	prometheusClient prometheusv1.API, // TODO: remove
	configClient configclient.Interface, // TODO: remove
	testDuration time.Duration,
	f *framework.Framework) {

	featureSet := configv1.Default
	featureGate, err := configClient.ConfigV1().FeatureGates().Get(context.TODO(), "cluster", metav1.GetOptions{})
	if err != nil {
		framework.Logf("ERROR: error checking feature gates in cluster, ignoring: %v", err)
	} else {
		featureSet = featureGate.Spec.FeatureSet
	}
	firingAlertsWithBugs, allowedFiringAlerts, pendingAlertsWithBugs, allowedPendingAlerts :=
		allowancesFunc(featureSet)

	// In addition to the alert allowances passed in (which can differ for upgrades vs conformance),
	// we also exclude alerts that have their own separate tests codified. This is a backstop test for
	// everything else.
	for _, alertTest := range allowedalerts2.AllAlertTests(&platformidentification.JobType{},
		allowedalerts2.DefaultAllowances) {

		switch alertTest.AlertState() {
		case allowedalerts2.AlertPending:
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
		case allowedalerts2.AlertInfo:
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
		// The two duplicated code paths merged together here had slightly different ways of reporting flakes:
		if f != nil {
			// when called from alert.go within an UpgradeTest with a framework available
			disruption.FrameworkFlakef(f, "Unexpected alert behavior:\n\n%s", strings.Join(flakes.List(), "\n"))
		} else {
			// when called from prometheus.go with no framework available
			testresult.Flakef("Unexpected alert behavior:\n\n%s", strings.Join(flakes.List(), "\n"))
		}
	}
	framework.Logf("No alerts fired")

}

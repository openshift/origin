package legacytestframeworkmonitortests

import (
	"context"
	"fmt"
	"strings"
	"time"

	allowedalerts2 "github.com/openshift/origin/pkg/monitortestlibrary/allowedalerts"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/sirupsen/logrus"

	"github.com/openshift/origin/pkg/alerts"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	helper "github.com/openshift/origin/test/extended/util/prometheus"

	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
)

func testAlerts(events monitorapi.Intervals,
	allowancesFunc alerts.AllowedAlertsFunc,
	jobType *platformidentification.JobType,
	restConfig *rest.Config,
	duration time.Duration,
	recordedResource monitorapi.ResourcesMap) []*junitapi.JUnitTestCase {

	// Work with the cluster under test before we run the alert tests. For testing the tests purposes,
	// please keep any use of the rest.Config isolated to this function and do not have the actual
	// invariant tests themselves hitting a live cluster.

	configClient := configv1client.NewForConfigOrDie(restConfig)
	featureSet := configv1.Default
	featureGate, err := configClient.ConfigV1().FeatureGates().Get(context.TODO(), "cluster", metav1.GetOptions{})
	if err != nil {
		framework.Logf("ERROR: error checking feature gates in cluster, ignoring: %v", err)
	} else {
		featureSet = featureGate.Spec.FeatureSet
	}

	var etcdAllowance allowedalerts2.AlertTestAllowanceCalculator
	etcdAllowance = allowedalerts2.DefaultAllowances
	// if we have a restConfig,  use it.
	var kubeClient *kubernetes.Clientset
	if restConfig != nil {
		kubeClient, err = kubernetes.NewForConfig(restConfig)
		if err != nil {
			panic(err)
		}
		etcdAllowance, err = allowedalerts2.NewAllowedWhenEtcdRevisionChange(context.TODO(),
			kubeClient, duration)
		if err != nil {
			panic(err)
		}
		_, err = kubeClient.CoreV1().Namespaces().Get(context.Background(), "openshift-monitoring", metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return []*junitapi.JUnitTestCase{}
		}
		if err != nil {
			panic(err)
		}
	}

	ret := RunAlertTests(jobType, allowancesFunc, featureSet, etcdAllowance, events, recordedResource)
	return ret
}

func RunAlertTests(jobType *platformidentification.JobType,
	allowancesFunc alerts.AllowedAlertsFunc,
	featureSet configv1.FeatureSet,
	etcdAllowance allowedalerts2.AlertTestAllowanceCalculator,
	events monitorapi.Intervals,
	recordedResource monitorapi.ResourcesMap) []*junitapi.JUnitTestCase {

	ret := []*junitapi.JUnitTestCase{}
	alertTests := allowedalerts2.AllAlertTests(jobType, etcdAllowance)

	// Run the per-alert tests we've hardcoded:
	for i := range alertTests {
		alertTest := alertTests[i]

		junit, err := alertTest.InvariantCheck(events, recordedResource)
		if err != nil {
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: alertTest.InvariantTestName(),
				FailureOutput: &junitapi.FailureOutput{
					Output: err.Error(),
				},
				SystemOut: err.Error(),
			})
		}
		ret = append(ret, junit...)
	}

	// Run the backstop catch all for all other alerts:
	ret = append(ret, runBackstopTest(allowancesFunc, featureSet, events, alertTests)...)

	return ret
}

// runBackstopTest will process the intervals for any alerts which do not have their own explicit test,
// and look for any pending/firing intervals that are not within sufficient range.
func runBackstopTest(
	allowancesFunc alerts.AllowedAlertsFunc,
	featureSet configv1.FeatureSet,
	alertIntervals monitorapi.Intervals,
	alertTests []allowedalerts2.AlertTest) []*junitapi.JUnitTestCase {

	firingAlertsWithBugs, allowedFiringAlerts, pendingAlertsWithBugs, allowedPendingAlerts :=
		allowancesFunc(featureSet)

	pendingIntervals := alertIntervals.Filter(monitorapi.AlertPending())
	firingIntervals := alertIntervals.Filter(monitorapi.AlertFiring())
	logrus.Infof("filtered down to %d pending intervals", len(pendingIntervals))
	logrus.Infof("filtered down to %d firing intervals", len(firingIntervals))

	// In addition to the alert allowances passed in (which can differ for upgrades vs conformance),
	// we also exclude alerts that have their own separate tests codified. This is a backstop test for
	// everything else.
	for _, alertTest := range alertTests {

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

	// New version for alert testing against intervals instead of directly from prometheus:
	for _, firing := range firingIntervals {
		fan := monitorapi.AlertFromLocator(firing.Locator)
		seconds := firing.To.Sub(firing.From)
		violation := fmt.Sprintf("V2 alert %s fired for %s seconds with labels: %s", fan, seconds, firing.Message)
		if cause := allowedFiringAlerts.MatchesInterval(firing); cause != nil {
			// TODO: this seems to never be happening? no search.ci results show allowed
			debug.Insert(fmt.Sprintf("%s result=allow (%s)", violation, cause.Text))
			continue
		}
		if cause := firingAlertsWithBugs.MatchesInterval(firing); cause != nil {
			knownViolations.Insert(fmt.Sprintf("%s result=allow bug=%s", violation, cause.Text))
		} else {
			unexpectedViolations.Insert(fmt.Sprintf("%s result=reject", violation))
		}
	}
	// New version for alert testing against intervals instead of directly from prometheus:
	for _, pending := range pendingIntervals {
		fan := monitorapi.AlertFromLocator(pending.Locator)
		seconds := pending.To.Sub(pending.From)
		violation := fmt.Sprintf("V2 alert %s pending for %s seconds with labels: %s", fan, seconds, pending.Message)
		if cause := allowedPendingAlerts.MatchesInterval(pending); cause != nil {
			// TODO: this seems to never be happening? no search.ci results show allowed
			debug.Insert(fmt.Sprintf("%s result=allow (%s)", violation, cause.Text))
			continue
		}
		if cause := pendingAlertsWithBugs.MatchesInterval(pending); cause != nil {
			knownViolations.Insert(fmt.Sprintf("%s result=allow bug=%s", violation, cause.Text))
		} else {
			// treat pending errors as a flake right now because we are still trying to determine the scope
			// TODO: move this to unexpectedViolations later
			//unexpectedViolationsAsFlakes.Insert(fmt.Sprintf("%s result=allow", violation))
		}
	}

	ret := []*junitapi.JUnitTestCase{
		{
			// Success test to force a flake until we're ready to let things fail here.
			Name: "[sig-trt][invariant] No alerts without an explicit test should be firing/pending more than historically",
		},
	}

	if len(debug) > 0 {
		framework.Logf("Alerts were detected which are allowed:\n\n%s", strings.Join(debug.List(), "\n"))
		// TODO: make sure this info is showing up in output for the test, should this go somewhere else?
		// TODO: but this doesn't seem to be triggering
		logrus.Infof("Alerts were detected which are allowed:\n\n%s", strings.Join(debug.List(), "\n"))
	}
	if flakes := sets.NewString().Union(knownViolations).Union(unexpectedViolations).Union(unexpectedViolationsAsFlakes); len(flakes) > 0 {
		output := fmt.Sprintf("Unexpected alert behavior: \n\n%s", strings.Join(flakes.List(), "\n"))
		ret = append(ret, &junitapi.JUnitTestCase{
			Name: "[sig-trt][invariant] No alerts without an explicit test should be firing/pending more than historically",
			FailureOutput: &junitapi.FailureOutput{
				Output: output,
			},
			SystemOut: output,
		})
	}
	return ret
}

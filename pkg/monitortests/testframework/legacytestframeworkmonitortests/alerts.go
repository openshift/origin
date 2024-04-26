package legacytestframeworkmonitortests

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/alerts"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/allowedalerts"
	"github.com/openshift/origin/pkg/monitortestlibrary/historicaldata"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/sirupsen/logrus"

	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
)

type AllowedAlertsFunc func(featureSet configv1.FeatureSet) (allowedFiringWithBugs, allowedFiring, allowedPendingWithBugs, allowedPending alerts.MetricConditions)

func testAlerts(events monitorapi.Intervals,
	allowancesFunc AllowedAlertsFunc,
	jobType *platformidentification.JobType,
	clusterStability *monitortestframework.ClusterStabilityDuringTest,
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

	var etcdAllowance allowedalerts.AlertTestAllowanceCalculator
	etcdAllowance = allowedalerts.DefaultAllowances
	// if we have a restConfig,  use it.
	var kubeClient *kubernetes.Clientset
	if restConfig != nil {
		kubeClient, err = kubernetes.NewForConfig(restConfig)
		if err != nil {
			panic(err)
		}
		etcdAllowance, err = allowedalerts.NewAllowedWhenEtcdRevisionChange(context.TODO(),
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

	ret := RunAlertTests(jobType, clusterStability, allowancesFunc, featureSet, etcdAllowance, events, recordedResource)
	return ret
}

// RunAlertTests is a key entry point for running all per-Alert tests we've defined in all.go AllAlertTests,
// as well as backstop tests on things we observe outside those specific tests.
func RunAlertTests(jobType *platformidentification.JobType,
	clusterStability *monitortestframework.ClusterStabilityDuringTest,
	allowancesFunc AllowedAlertsFunc,
	featureSet configv1.FeatureSet,
	etcdAllowance allowedalerts.AlertTestAllowanceCalculator,
	events monitorapi.Intervals,
	recordedResource monitorapi.ResourcesMap) []*junitapi.JUnitTestCase {

	ret := []*junitapi.JUnitTestCase{}
	alertTests := allowedalerts.AllAlertTests(jobType, clusterStability, etcdAllowance)

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

	pendingIntervals := events.Filter(monitorapi.AlertPending())
	firingIntervals := events.Filter(monitorapi.AlertFiring())

	// Run the backstop catch all for all other alerts:
	ret = append(ret, runBackstopTest(allowancesFunc, featureSet, pendingIntervals, firingIntervals, alertTests)...)

	// TODO: Run a test to ensure no new alerts fired:
	ret = append(ret, runNoNewAlertsFiringTest(allowedalerts.GetHistoricalData(), firingIntervals)...)

	return ret
}

// runBackstopTest will process the intervals for any alerts which do not have their own explicit test,
// and look for any pending/firing intervals that are not within sufficient range.
func runBackstopTest(
	allowancesFunc AllowedAlertsFunc,
	featureSet configv1.FeatureSet,
	pendingIntervals monitorapi.Intervals,
	firingIntervals monitorapi.Intervals,
	alertTests []allowedalerts.AlertTest) []*junitapi.JUnitTestCase {

	firingAlertsWithBugs, allowedFiringAlerts, pendingAlertsWithBugs, allowedPendingAlerts :=
		allowancesFunc(featureSet)

	logrus.Infof("filtered down to %d pending intervals", len(pendingIntervals))
	logrus.Infof("filtered down to %d firing intervals", len(firingIntervals))

	// In addition to the alert allowances passed in (which can differ for upgrades vs conformance),
	// we also exclude alerts that have their own separate tests codified. This is a backstop test for
	// everything else.
	for _, alertTest := range alertTests {

		switch alertTest.AlertState() {
		case allowedalerts.AlertPending:
			// a pending test covers pending and everything above (firing)
			allowedPendingAlerts = append(allowedPendingAlerts,
				alerts.MetricCondition{
					AlertName: alertTest.AlertName(),
					Text:      "has a separate e2e test",
				},
			)
			allowedFiringAlerts = append(allowedFiringAlerts,
				alerts.MetricCondition{
					AlertName: alertTest.AlertName(),
					Text:      "has a separate e2e test",
				},
			)
		case allowedalerts.AlertInfo:
			// an info test covers all firing
			allowedFiringAlerts = append(allowedFiringAlerts,
				alerts.MetricCondition{
					AlertName: alertTest.AlertName(),
					Text:      "has a separate e2e test",
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
		fan := firing.Locator.Keys[monitorapi.LocatorAlertKey]
		if isSkippedAlert(fan) {
			continue
		}
		seconds := firing.To.Sub(firing.From)
		violation := fmt.Sprintf("V2 alert %s fired for %s seconds with labels: %s", fan, seconds, firing.Message.OldMessage())
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
		fan := pending.Locator.Keys[monitorapi.LocatorAlertKey]
		if isSkippedAlert(fan) {
			continue
		}
		seconds := pending.To.Sub(pending.From)
		violation := fmt.Sprintf("V2 alert %s pending for %s seconds with labels: %s", fan, seconds, pending.Message.OldMessage())
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
			// unexpectedViolationsAsFlakes.Insert(fmt.Sprintf("%s result=allow", violation))
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

func isSkippedAlert(alertName string) bool {
	// Some alerts we always skip over in CI:
	for _, a := range allowedalerts.AllowedAlertNames {
		if a == alertName {
			return true
		}
	}
	return false
}

// runNoNewAlertsFiringTest checks all firing non-info alerts to see if we:
//
//   - have no historical data for that alert in that namespace for this release, or
//   - have historical data but it was first observed less than 2 weeks ago
//
// If either is true, this test will fail. We do not want new product alerts being added to the product that
// will trigger routinely and affect the fleet when they ship.
// The two week limit is our window to address these kinds of problems, after that the failure will stop.
func runNoNewAlertsFiringTest(historicalData *historicaldata.AlertBestMatcher,
	firingIntervals monitorapi.Intervals) []*junitapi.JUnitTestCase {
	testName := "[sig-trt][invariant] No new alerts should be firing"
	// accumulate all alerts firing that we have no historical data for this release, or we know it only
	// recently appeared. (less than two weeks ago) Any alerts in either category will fail this test.
	newAlertsFiring := []string{}

	for _, interval := range firingIntervals {
		alertName := interval.Locator.Keys[monitorapi.LocatorAlertKey]

		if isSkippedAlert(alertName) {
			continue
		}

		// Skip alerts with severity info, I don't totally understand the semantics here but it appears some components
		// use this for informational alerts. (saw two examples from Insights)
		// We're only interested in warning+critical for the purposes of this test.
		if interval.Message.Annotations[monitorapi.AnnotationSeverity] == "info" {
			continue
		}

		// Scan historical data to see if this alert appears new. Ignore release, namespace, level, everything but
		// AlertName, just see if it's in the data file and when we first observed it in either of the two releases we expect in there.
		var firstObserved *time.Time
		var sawAlertName bool
		for _, hd := range historicalData.HistoricalData {
			if hd.AlertName == alertName {
				sawAlertName = true
				if firstObserved == nil || (!hd.FirstObserved.IsZero() && hd.FirstObserved.Before(*firstObserved)) {
					firstObserved = &hd.FirstObserved
				}
			}
		}

		if !sawAlertName {
			violation := fmt.Sprintf("%s has no test data, this alert appears new and should not be firing", alertName)
			logrus.Warn(violation)
			newAlertsFiring = append(newAlertsFiring, violation)
		} else if firstObserved != nil && time.Now().Sub(*firstObserved) <= 14*24*time.Hour {
			// if we did get data, check how old the first observed timestamp was, too new and we're going to fire as well.
			violation := fmt.Sprintf("%s was first observed on %s, this alert appears new and should not be firing", alertName, firstObserved.UTC().Format(time.RFC3339))
			logrus.Warn(violation)
			newAlertsFiring = append(newAlertsFiring, violation)
		}
		// if firstObserved was still nil, we can't do the two week check, but we saw the alert, so assume a test pass, the alert must be known
	}

	ret := []*junitapi.JUnitTestCase{}

	if len(newAlertsFiring) > 0 {
		// test failed
		output := fmt.Sprintf("Found alerts firing which are new or less than two weeks old, which should not be firing: \n\n%s",
			strings.Join(newAlertsFiring, "\n"))
		ret = append(ret, &junitapi.JUnitTestCase{
			Name: testName,
			FailureOutput: &junitapi.FailureOutput{
				Output: output,
			},
			SystemOut: output,
		})
	} else {
		// test passed
		ret = append(ret, &junitapi.JUnitTestCase{
			Name: testName,
		})
	}

	return ret
}

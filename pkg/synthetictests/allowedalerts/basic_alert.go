package allowedalerts

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/synthetictests/platformidentification"
	testresult "github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
)

type AlertTest interface {
	// TestNamePrefix is the prefix for this as a late test: [bz-component][late] something
	TestNamePrefix() string
	// LateTestNameSuffix is name for this as a late test: alert/foo should not be pending
	LateTestNameSuffix() string
	// InvariantTestName is name for this as an invariant test
	InvariantTestName() string

	// AlertName is the name of the alert
	AlertName() string
	// AlertState is the threshold this test applies to.
	AlertState() AlertState

	TestAlert(ctx context.Context, prometheusClient prometheusv1.API, restConfig *rest.Config) error
	InvariantCheck(ctx context.Context, restConfig *rest.Config, intervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error)
}

// AlertState is the state of the alert. They are logically ordered, so if a test says it limits on "pending", then
// any state above pending (like info or warning) will cause the test to fail.
type AlertState string

const (
	AlertPending  AlertState = "pending"
	AlertInfo     AlertState = "info"
	AlertWarning  AlertState = "warning"
	AlertCritical AlertState = "critical"
	AlertUnknown  AlertState = "unknown"
)

type basicAlertTest struct {
	bugzillaComponent string
	alertName         string
	alertState        AlertState

	allowanceCalculator AlertTestAllowanceCalculator
}

func newAlert(bugzillaComponent, alertName string) *basicAlertTest {
	return &basicAlertTest{
		bugzillaComponent:   bugzillaComponent,
		alertName:           alertName,
		alertState:          AlertPending,
		allowanceCalculator: defaultAllowances,
	}
}

func (a *basicAlertTest) withAllowance(allowanceCalculator AlertTestAllowanceCalculator) *basicAlertTest {
	a.allowanceCalculator = allowanceCalculator
	return a
}

func (a *basicAlertTest) pending() *basicAlertTest {
	a.alertState = AlertPending
	return a
}

func (a *basicAlertTest) firing() *basicAlertTest {
	a.alertState = AlertInfo
	return a
}

func (a *basicAlertTest) warning() *basicAlertTest {
	a.alertState = AlertWarning
	return a
}

func (a *basicAlertTest) critical() *basicAlertTest {
	a.alertState = AlertCritical
	return a
}

func (a *basicAlertTest) neverFail() *basicAlertTest {
	a.allowanceCalculator = neverFail(a.allowanceCalculator)
	return a
}

func (a *basicAlertTest) toTest() AlertTest {
	return a
}

func (a *basicAlertTest) TestNamePrefix() string {
	return fmt.Sprintf("[bz-%s][Late] Alerts", a.bugzillaComponent)
}

func (a *basicAlertTest) LateTestNameSuffix() string {
	return fmt.Sprintf("alert/%s should not be at or above %s", a.alertName, a.alertState)
}

func (a *basicAlertTest) InvariantTestName() string {
	return fmt.Sprintf("[bz-%v][invariant] alert/%s should not be at or above %s", a.bugzillaComponent, a.alertName, a.alertState)
}

func (a *basicAlertTest) AlertName() string {
	return a.alertName
}

func (a *basicAlertTest) AlertState() AlertState {
	return a.alertState
}

func (a *basicAlertTest) TestAlert(ctx context.Context, prometheusClient prometheusv1.API, restConfig *rest.Config) error {
	// TODO, could only do these based on what we're checking
	firingIntervals, err := monitor.WhenWasAlertFiring(ctx, prometheusClient, exutil.BestStartTime(), a.AlertName())
	if err != nil {
		return err
	}
	pendingIntervals, err := monitor.WhenWasAlertPending(ctx, prometheusClient, exutil.BestStartTime(), a.AlertName())
	if err != nil {
		return err
	}

	state, message := a.failOrFlake(ctx, restConfig, firingIntervals, pendingIntervals)
	switch state {
	case pass:
		return nil

	case flake:
		testresult.Flakef("%s", message)
		return nil

	case fail:
		framework.Failf("%s", message)
		return nil

	default:
		return fmt.Errorf("unrecognized state: %v", state)
	}
}

type testState int

const (
	pass testState = iota
	flake
	fail
)

func (a *basicAlertTest) failOrFlake(ctx context.Context, restConfig *rest.Config, firingIntervals, pendingIntervals monitorapi.Intervals) (testState, string) {
	var alertIntervals monitorapi.Intervals

	switch a.AlertState() {
	case AlertPending:
		alertIntervals = append(alertIntervals, pendingIntervals...)
		fallthrough

	case AlertInfo:
		alertIntervals = append(alertIntervals, firingIntervals.Filter(monitorapi.IsInfoEvent)...)
		fallthrough

	case AlertWarning:
		alertIntervals = append(alertIntervals, firingIntervals.Filter(monitorapi.IsWarningEvent)...)
		fallthrough

	case AlertCritical:
		alertIntervals = append(alertIntervals, firingIntervals.Filter(monitorapi.IsErrorEvent)...)

	default:
		return fail, fmt.Sprintf("unhandled alert state: %v", a.AlertState())
	}

	describe := alertIntervals.Strings()
	durationAtOrAboveLevel := alertIntervals.Duration(1 * time.Second)
	firingDuration := firingIntervals.Duration(1 * time.Second)
	pendingDuration := pendingIntervals.Duration(1 * time.Second)

	jobType, err := platformidentification.GetJobType(ctx, restConfig)
	if err != nil {
		return fail, err.Error()
	}

	failAfter, err := a.allowanceCalculator.FailAfter(a.alertName, *jobType)
	if err != nil {
		return fail, fmt.Sprintf("unable to calculate allowance for %s which was at %s, err %v\n\n%s", a.AlertName(), a.AlertState(), err, strings.Join(describe, "\n"))
	}
	flakeAfter := a.allowanceCalculator.FlakeAfter(a.alertName, *jobType)

	switch {
	case durationAtOrAboveLevel > failAfter:
		return fail, fmt.Sprintf("%s was at or above %s for at least %s on %#v (maxAllowed=%s): pending for %s, firing for %s:\n\n%s",
			a.AlertName(), a.AlertState(), durationAtOrAboveLevel, *jobType, failAfter, pendingDuration, firingDuration, strings.Join(describe, "\n"))

	case durationAtOrAboveLevel > flakeAfter:
		return flake, fmt.Sprintf("%s was at or above %s for at least %s on %#v (maxAllowed=%s): pending for %s, firing for %s:\n\n%s",
			a.AlertName(), a.AlertState(), durationAtOrAboveLevel, *jobType, flakeAfter, pendingDuration, firingDuration, strings.Join(describe, "\n"))
	}

	return pass, ""
}

func (a *basicAlertTest) InvariantCheck(ctx context.Context, restConfig *rest.Config, alertIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	pendingIntervals := alertIntervals.Filter(
		func(eventInterval monitorapi.EventInterval) bool {
			locatorParts := monitorapi.LocatorParts(eventInterval.Locator)
			alertName := monitorapi.AlertFrom(locatorParts)
			if alertName != a.alertName {
				return false
			}
			if strings.Contains(eventInterval.Message, `alertstate="pending"`) {
				return true
			}
			return false
		},
	)
	firingIntervals := alertIntervals.Filter(
		func(eventInterval monitorapi.EventInterval) bool {
			locatorParts := monitorapi.LocatorParts(eventInterval.Locator)
			alertName := monitorapi.AlertFrom(locatorParts)
			if alertName != a.alertName {
				return false
			}
			if strings.Contains(eventInterval.Message, `alertstate="firing"`) {
				return true
			}
			return false
		},
	)

	state, message := a.failOrFlake(ctx, restConfig, firingIntervals, pendingIntervals)
	switch state {
	case pass:
		return []*junitapi.JUnitTestCase{
			{
				Name: a.InvariantTestName(),
			},
		}, nil

	case flake:
		return []*junitapi.JUnitTestCase{
			{
				Name: a.InvariantTestName(),
			},
			{
				Name: a.InvariantTestName(),
				FailureOutput: &junitapi.FailureOutput{
					Output: message,
				},
				SystemOut: message,
			},
		}, nil

	case fail:
		return []*junitapi.JUnitTestCase{
			{
				Name: a.InvariantTestName(),
				FailureOutput: &junitapi.FailureOutput{
					Output: message,
				},
				SystemOut: message,
			},
		}, nil

	default:
		return nil, fmt.Errorf("unrecognized state: %v", state)
	}
}

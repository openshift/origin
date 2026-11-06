package allowedalerts

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
)

const (
	inducibleAlert = "OVNKubernetesResourceRetryFailure"

	// the name the OVN metrics test carries once it declares what it induces
	inducingTestName = "[Feature:Metrics][JIRA:Networking][OTP][Serial][Slow][Timeout:25m]" +
		"[InducesAlert:OVNKubernetesResourceRetryFailure][ovn-kubernetes-ote][sig-network] " +
		"OVN metrics 60708-Verify metrics ovnkube_resource_retry_failures_total"

	otherTestName = "[sig-network] some unrelated test"
)

func e2eTestInterval(testName string, from, to time.Time) monitorapi.Interval {
	return monitorapi.NewInterval(monitorapi.SourceE2ETest, monitorapi.Info).
		Locator(monitorapi.NewLocator().E2ETest(testName)).
		Message(monitorapi.NewMessage().
			HumanMessagef("e2e test finished As %q", "Passed").
			WithAnnotation(monitorapi.AnnotationStatus, "Passed")).
		Display().
		Build(from, to)
}

func firingAlertInterval(alertName string, from, to time.Time) monitorapi.Interval {
	return monitorapi.Interval{
		Condition: monitorapi.Condition{
			Level: monitorapi.Warning,
			Locator: monitorapi.Locator{
				Type: monitorapi.LocatorTypeAlert,
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorAlertKey: alertName,
				},
			},
			Message: monitorapi.Message{
				HumanMessage: "alert " + alertName,
				Annotations: map[monitorapi.AnnotationKey]string{
					monitorapi.AnnotationAlertState: "firing",
					monitorapi.AnnotationSeverity:   "warning",
				},
			},
		},
		Source: monitorapi.SourceAlert,
		From:   from,
		To:     to,
	}
}

func TestTestDeclaresInducedAlert(t *testing.T) {
	tests := []struct {
		name      string
		testName  string
		alertName string
		expected  bool
	}{
		{
			name:      "marker present for this alert",
			testName:  inducingTestName,
			alertName: inducibleAlert,
			expected:  true,
		},
		{
			name:      "marker present for a different alert",
			testName:  "[InducesAlert:KubePodNotReady][sig-network] t",
			alertName: inducibleAlert,
			expected:  false,
		},
		{
			name:      "no marker",
			testName:  otherTestName,
			alertName: inducibleAlert,
			expected:  false,
		},
		{
			name:      "alert name is a prefix of the marker",
			testName:  "[InducesAlert:OVNKubernetesResourceRetryFailureExtra][sig-network] t",
			alertName: inducibleAlert,
			expected:  false,
		},
		{
			name:      "several markers on one test",
			testName:  "[InducesAlert:SomethingElse][InducesAlert:OVNKubernetesResourceRetryFailure] t",
			alertName: inducibleAlert,
			expected:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, testDeclaresInducedAlert(test.testName, test.alertName))
		})
	}
}

func TestTestInducedAlertWindowsFor(t *testing.T) {
	start := time.Date(2026, 9, 30, 19, 16, 3, 0, time.UTC)
	end := time.Date(2026, 9, 30, 19, 32, 49, 0, time.UTC)

	t.Run("alert is not inducible", func(t *testing.T) {
		intervals := monitorapi.Intervals{
			e2eTestInterval("[InducesAlert:KubeAPIErrorBudgetBurn] t", start, end),
		}
		assert.Empty(t, testInducedAlertWindowsFor("KubeAPIErrorBudgetBurn", intervals),
			"a test must not be able to excuse an alert that is not on the inducible list")
	})

	t.Run("point intervals are ignored", func(t *testing.T) {
		// The raw started/finished intervals carry no duration and would produce a zero
		// length window, which would excuse nothing and hide the real one.
		intervals := monitorapi.Intervals{
			monitorapi.NewInterval(monitorapi.SourceE2ETest, monitorapi.Info).
				Locator(monitorapi.NewLocator().E2ETest(inducingTestName)).
				Message(monitorapi.NewMessage().HumanMessage("started").Reason(monitorapi.E2ETestStarted)).
				Build(start, start),
		}
		assert.Empty(t, testInducedAlertWindowsFor(inducibleAlert, intervals))
	})

	t.Run("one window per run, extended by the grace period", func(t *testing.T) {
		retryStart := end.Add(time.Hour)
		retryEnd := retryStart.Add(5 * time.Minute)
		intervals := monitorapi.Intervals{
			e2eTestInterval(inducingTestName, start, end),
			e2eTestInterval(otherTestName, start, end),
			e2eTestInterval(inducingTestName, retryStart, retryEnd),
		}

		windows := testInducedAlertWindowsFor(inducibleAlert, intervals)
		require.Len(t, windows, 2)
		assert.Equal(t, start, windows[0].start)
		assert.Equal(t, end.Add(10*time.Minute), windows[0].end)
		assert.Equal(t, retryStart, windows[1].start)
		assert.Equal(t, retryEnd.Add(10*time.Minute), windows[1].end)
	})
}

func TestFilterTestInducedAlertIntervals(t *testing.T) {
	testStart := time.Date(2026, 9, 30, 19, 16, 3, 0, time.UTC)
	testEnd := time.Date(2026, 9, 30, 19, 32, 49, 0, time.UTC)
	windows := []testInducedAlertWindow{
		{testName: inducingTestName, start: testStart, end: testEnd.Add(10 * time.Minute)},
	}

	tests := []struct {
		name            string
		alert           monitorapi.Interval
		expectedExcused bool
	}{
		{
			// the shape seen in the real job: fires 12s before the test ends and runs on
			// past it because increase(...[10m]) keeps it up
			name:            "starts inside the test and outlives it",
			alert:           firingAlertInterval(inducibleAlert, testEnd.Add(-12*time.Second), testEnd.Add(96*time.Second)),
			expectedExcused: true,
		},
		{
			name:            "starts inside the grace period",
			alert:           firingAlertInterval(inducibleAlert, testEnd.Add(9*time.Minute), testEnd.Add(20*time.Minute)),
			expectedExcused: true,
		},
		{
			name:            "starts after the grace period",
			alert:           firingAlertInterval(inducibleAlert, testEnd.Add(11*time.Minute), testEnd.Add(20*time.Minute)),
			expectedExcused: false,
		},
		{
			// already broken before the test began; the test only overlaps it
			name:            "already firing when the test started",
			alert:           firingAlertInterval(inducibleAlert, testStart.Add(-time.Minute), testEnd),
			expectedExcused: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			judged, excused := filterTestInducedAlertIntervals(
				inducibleAlert, monitorapi.Intervals{test.alert}, windows)
			if test.expectedExcused {
				assert.Len(t, excused, 1)
				assert.Empty(t, judged)
			} else {
				assert.Empty(t, excused)
				assert.Len(t, judged, 1)
			}
		})
	}

	t.Run("no windows leaves everything judged", func(t *testing.T) {
		alerts := monitorapi.Intervals{firingAlertInterval(inducibleAlert, testStart, testEnd)}
		judged, excused := filterTestInducedAlertIntervals(inducibleAlert, alerts, nil)
		assert.Equal(t, alerts, judged)
		assert.Empty(t, excused)
	})
}

// TestInvariantCheckExcusesTestInducedAlert replays the shape of
// pull-ci-openshift-ovn-kubernetes-main-e2e-aws-core-networking-serial-ote/2105359780603236352,
// where the firing() invariant failed on an alert that the test which passed had caused.
func TestInvariantCheckExcusesTestInducedAlert(t *testing.T) {
	testStart := time.Date(2026, 9, 30, 19, 16, 3, 0, time.UTC)
	testEnd := time.Date(2026, 9, 30, 19, 32, 49, 0, time.UTC)
	alertStart := time.Date(2026, 9, 30, 19, 32, 37, 0, time.UTC)
	alertEnd := time.Date(2026, 9, 30, 19, 34, 25, 0, time.UTC)

	jobType := &platformidentification.JobType{
		Release: "5.1", Platform: "aws", Architecture: "amd64", Network: "ovn", Topology: "ha",
	}

	// The shape of the firing() registration in all.go, which carries no neverFail(). The
	// allowance is pinned to failOnAny rather than DefaultAllowances so the assertion turns
	// on the excusing logic and not on whatever percentile the embedded historical data
	// happens to hold for this alert today.
	alertTest := &basicAlertTest{
		bugzillaComponent:   "bz-networking",
		alertName:           inducibleAlert,
		alertState:          AlertInfo,
		jobType:             jobType,
		allowanceCalculator: failOnAny(),
	}

	alertInterval := firingAlertInterval(inducibleAlert, alertStart, alertEnd)

	t.Run("test did not declare the alert, invariant still fails", func(t *testing.T) {
		junits, err := alertTest.InvariantCheck(monitorapi.Intervals{
			e2eTestInterval(otherTestName, testStart, testEnd),
			alertInterval,
		}, monitorapi.ResourcesMap{})
		require.NoError(t, err)
		require.Len(t, junits, 1)
		assert.NotNil(t, junits[0].FailureOutput, "expected a hard failure, got a pass")
	})

	t.Run("test declared the alert, invariant passes and says what it excused", func(t *testing.T) {
		junits, err := alertTest.InvariantCheck(monitorapi.Intervals{
			e2eTestInterval(inducingTestName, testStart, testEnd),
			alertInterval,
		}, monitorapi.ResourcesMap{})
		require.NoError(t, err)
		require.Len(t, junits, 1)
		assert.Nil(t, junits[0].FailureOutput)
		assert.Contains(t, junits[0].SystemOut, "e2e test declared it induces this alert")
		assert.Contains(t, junits[0].SystemOut, inducingTestName)
	})
}

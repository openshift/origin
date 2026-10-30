package allowedalerts

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// Some e2e tests verify alerting and metrics behaviour by deliberately putting the cluster
// into the state an alert watches for. The alert firing is then the test working, not the
// cluster breaking, but the alert invariants in all.go have no way to tell the two apart and
// fail the whole job on the back of a test that passed.
//
// The two blunt ways out are to mark the alert neverFail() for every job in every repo, or
// to stop running the test. Neither is right: the first throws away real coverage of an
// alert, the second throws away real coverage of a metric.
//
// Instead a test declares what it induces and that declaration is honoured only for the
// span of the test. Everything outside that span is judged exactly as before.

// testInducedAlertMarker matches the label an e2e test uses to declare that it deliberately
// drives an alert into firing, e.g.
//
//	g.It("...", g.Label("InducesAlert:OVNKubernetesResourceRetryFailure"), ...)
//
// which renders into the test name as [InducesAlert:OVNKubernetesResourceRetryFailure]. A
// test may carry more than one marker.
//
// The marker is a claim by the test, not a grant. It takes effect only for alerts listed in
// testInducibleAlerts, so an external test binary cannot silence an alert this repo has not
// agreed may be induced.
var testInducedAlertMarker = regexp.MustCompile(`\[InducesAlert:([A-Za-z][A-Za-z0-9_]*)\]`)

// testInducibleAlerts is the set of alerts an e2e test is allowed to induce, mapped to the
// grace period that applies after the test ends.
//
// The grace period exists because the alert does not stop firing when the test stops causing
// it. A rule built on increase(<counter>[w]) keeps firing for the whole of w after the final
// increment regardless of what the test tears down, so a test cannot clean up after itself
// here however carefully it is written. Set the grace period to that lookback window, and to
// the `for:` duration on top of it where the rule has one. Do not pad it beyond that: the
// grace period is the only thing bounding how long a test can excuse an alert for.
var testInducibleAlerts = map[string]time.Duration{
	// cluster-network-operator, bindata/network/ovn-kubernetes/common/alert-rules.yaml:
	//   expr: increase(ovnkube_resource_retry_failures_total[10m]) > 0   (no `for:` clause)
	//
	// Induced by the OVN metrics test that creates an EgressIP holding invalid addresses in
	// order to exhaust the OVN retry framework and increment
	// ovnkube_resource_retry_failures_total, which is the series this rule is written over.
	// Verifying the counter moves and keeping the alert quiet are mutually exclusive.
	"OVNKubernetesResourceRetryFailure": 10 * time.Minute,
}

// testInducedAlertWindow is a span in which a named alert may fire without counting against
// its invariant, because a test said it would cause it.
type testInducedAlertWindow struct {
	testName string
	start    time.Time
	end      time.Time
}

func (w testInducedAlertWindow) contains(t time.Time) bool {
	return !t.Before(w.start) && !t.After(w.end)
}

func (w testInducedAlertWindow) String() string {
	return fmt.Sprintf("%s - %s (test %q)",
		w.start.Format(time.RFC3339), w.end.Format(time.RFC3339), w.testName)
}

// testDeclaresInducedAlert reports whether testName carries a marker for alertName.
func testDeclaresInducedAlert(testName, alertName string) bool {
	for _, match := range testInducedAlertMarker.FindAllStringSubmatch(testName, -1) {
		if match[1] == alertName {
			return true
		}
	}
	return false
}

// testInducedAlertWindowsFor returns one window per run of an e2e test that declared it
// induces alertName. A test that was retried produces one window per attempt.
//
// Returns nil when the alert is not inducible, which is the common case and keeps the cost
// of this out of every other alert test.
func testInducedAlertWindowsFor(alertName string, allEventIntervals monitorapi.Intervals) []testInducedAlertWindow {
	grace, inducible := testInducibleAlerts[alertName]
	if !inducible {
		return nil
	}

	var ret []testInducedAlertWindow
	for _, interval := range allEventIntervals {
		// The summary interval built by the e2etestanalyzer spans the whole run of the test.
		// The raw E2ETestStarted/E2ETestFinished intervals are points and carry no duration,
		// so they are no use here.
		if interval.Source != monitorapi.SourceE2ETest || !interval.Display {
			continue
		}
		testName, ok := monitorapi.E2ETestFromLocator(interval.Locator)
		if !ok {
			continue
		}
		if !testDeclaresInducedAlert(testName, alertName) {
			continue
		}
		ret = append(ret, testInducedAlertWindow{
			testName: testName,
			start:    interval.From,
			end:      interval.To.Add(grace),
		})
	}
	return ret
}

// filterTestInducedAlertIntervals splits alertIntervals into the ones that still have to be
// judged and the ones a test declared it would cause.
//
// An interval is only excused when it *starts* inside a window. An alert already firing when
// the test began is a pre-existing problem that the test happens to overlap, and is judged as
// normal.
func filterTestInducedAlertIntervals(alertName string, alertIntervals monitorapi.Intervals, windows []testInducedAlertWindow) (judged, excused monitorapi.Intervals) {
	if len(windows) == 0 {
		return alertIntervals, nil
	}

	for _, alertInterval := range alertIntervals {
		matched := false
		for _, window := range windows {
			if window.contains(alertInterval.From) {
				logrus.Infof("alert %s firing from %s excused: induced by e2e test %q, window %s",
					alertName, alertInterval.From.Format(time.RFC3339), window.testName, window)
				excused = append(excused, alertInterval)
				matched = true
				break
			}
		}
		if !matched {
			judged = append(judged, alertInterval)
		}
	}
	return judged, excused
}

// describeTestInducedAlerts renders what was excused, for the junit output. Suppression that
// nobody can see is suppression nobody can review, so this goes into SystemOut even when the
// test passes.
func describeTestInducedAlerts(alertName string, windows []testInducedAlertWindow, excused monitorapi.Intervals) string {
	if len(excused) == 0 {
		return ""
	}

	lines := []string{
		fmt.Sprintf("%d %s interval(s) were not counted because an e2e test declared it induces this alert:",
			len(excused), alertName),
		"",
	}
	lines = append(lines, excused.Strings()...)
	lines = append(lines, "", "excused within:")
	for _, window := range windows {
		lines = append(lines, "  "+window.String())
	}
	lines = append(lines,
		"",
		fmt.Sprintf("grace period after each test: %s (lookback window of the alert's expression)",
			testInducibleAlerts[alertName]))

	return strings.Join(lines, "\n")
}

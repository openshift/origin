package pertestintervalserializer

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

var base = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func at(seconds int) time.Time {
	return base.Add(time.Duration(seconds) * time.Second)
}

func e2eWindow(name, status string, fromSec, toSec int) monitorapi.Interval {
	return monitorapi.Interval{
		Condition: monitorapi.Condition{
			Level:   monitorapi.Info,
			Locator: monitorapi.Locator{Type: "E2ETest", Keys: map[monitorapi.LocatorKey]string{monitorapi.LocatorE2ETestKey: name}},
			Message: monitorapi.Message{Annotations: map[monitorapi.AnnotationKey]string{monitorapi.AnnotationStatus: status}},
		},
		Source: monitorapi.SourceE2ETest,
		From:   at(fromSec),
		To:     at(toSec),
	}
}

func event(level monitorapi.IntervalLevel, keys map[monitorapi.LocatorKey]string, fromSec, toSec int) monitorapi.Interval {
	i := monitorapi.Interval{
		Condition: monitorapi.Condition{
			Level:   level,
			Locator: monitorapi.Locator{Type: "KubeEvent", Keys: keys},
		},
		Source: monitorapi.SourceKubeEvent,
		From:   at(fromSec),
	}
	if toSec >= 0 {
		i.To = at(toSec)
	}
	return i
}

func findWindow(r Report, name string) *TestWindow {
	for i := range r.TestWindows {
		if r.TestWindows[i].TestName == name {
			return &r.TestWindows[i]
		}
	}
	return nil
}

func TestBuildReport(t *testing.T) {
	// Windows A [0,10] and B [5,15] overlap in [5,10].
	windowA := e2eWindow("test A", "Passed", 0, 10)
	windowB := e2eWindow("test B", "Failed", 5, 15)

	// e1 at t=7 (instant) falls inside both A and B.
	e1 := event(monitorapi.Error, map[monitorapi.LocatorKey]string{
		monitorapi.LocatorNamespaceKey: "openshift-authentication",
		monitorapi.LocatorNameKey:      "oauth-openshift",
		monitorapi.LocatorHmsgKey:      "abc123",
	}, 7, 7)
	// e2 at t=12 (instant) falls inside B only.
	e2 := event(monitorapi.Warning, map[monitorapi.LocatorKey]string{monitorapi.LocatorNameKey: "later"}, 12, 12)
	// e3 at t=20 (instant) falls inside no window.
	e3 := event(monitorapi.Info, map[monitorapi.LocatorKey]string{monitorapi.LocatorNameKey: "outside"}, 20, 20)
	// e4 starts at t=-5 with a zero To. It must be treated as an instant at -5, not open-ended,
	// so it must NOT leak into any later window.
	e4 := event(monitorapi.Warning, map[monitorapi.LocatorKey]string{monitorapi.LocatorNameKey: "zero-to-before"}, -5, -1)
	e4.To = time.Time{}

	// Raw e2e point-events (From == To) must be ignored as windows.
	rawStart := monitorapi.Interval{
		Condition: monitorapi.Condition{Locator: monitorapi.Locator{Keys: map[monitorapi.LocatorKey]string{monitorapi.LocatorE2ETestKey: "test A"}}},
		Source:    monitorapi.SourceE2ETest,
		From:      at(0),
		To:        at(0),
	}

	report := buildReport(monitorapi.Intervals{windowA, windowB, e1, e2, e3, e4, rawStart}, base)

	if len(report.TestWindows) != 2 {
		t.Fatalf("expected 2 test windows, got %d", len(report.TestWindows))
	}

	twA := findWindow(report, "test A")
	twB := findWindow(report, "test B")
	if twA == nil || twB == nil {
		t.Fatalf("missing expected windows: A=%v B=%v", twA, twB)
	}

	// Window A: only e1.
	if twA.IntervalCount != 1 {
		t.Errorf("window A: expected 1 interval, got %d (%v)", twA.IntervalCount, twA.Intervals)
	}
	if twA.ErrorCount != 1 || twA.WarningCount != 0 || twA.InfoCount != 0 {
		t.Errorf("window A counts wrong: error=%d warning=%d info=%d", twA.ErrorCount, twA.WarningCount, twA.InfoCount)
	}
	if twA.Result != "Passed" {
		t.Errorf("window A result = %q, want Passed", twA.Result)
	}

	// Window B: e1 (error) and e2 (warning). e1 appears in both windows (many-to-many).
	if twB.IntervalCount != 2 {
		t.Errorf("window B: expected 2 intervals, got %d (%v)", twB.IntervalCount, twB.Intervals)
	}
	if twB.ErrorCount != 1 || twB.WarningCount != 1 || twB.InfoCount != 0 {
		t.Errorf("window B counts wrong: error=%d warning=%d info=%d", twB.ErrorCount, twB.WarningCount, twB.InfoCount)
	}

	// Flattened fields for e1.
	ri := twA.Intervals[0]
	if ri.Namespace != "openshift-authentication" {
		t.Errorf("namespace = %q, want openshift-authentication", ri.Namespace)
	}
	if ri.Name != "oauth-openshift" {
		t.Errorf("name = %q, want oauth-openshift", ri.Name)
	}
	if ri.Identifier != "abc123" {
		t.Errorf("identifier = %q, want abc123", ri.Identifier)
	}
	if ri.Source != "KubeEvent" {
		t.Errorf("source = %q, want KubeEvent", ri.Source)
	}
	if ri.Level != "Error" {
		t.Errorf("level = %q, want Error", ri.Level)
	}
}

func TestOverlaps(t *testing.T) {
	window := e2eWindow("w", "Passed", 10, 20)

	tests := []struct {
		name     string
		interval monitorapi.Interval
		want     bool
	}{
		{"instant inside", event(monitorapi.Info, nil, 15, 15), true},
		{"instant at start boundary", event(monitorapi.Info, nil, 10, 10), true},
		{"instant at end boundary", event(monitorapi.Info, nil, 20, 20), true},
		{"instant before", event(monitorapi.Info, nil, 5, 5), false},
		{"instant after", event(monitorapi.Info, nil, 25, 25), false},
		{"span overlapping start", event(monitorapi.Info, nil, 5, 12), true},
		{"span overlapping end", event(monitorapi.Info, nil, 18, 25), true},
		{"span entirely before", event(monitorapi.Info, nil, 0, 5), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := overlaps(tc.interval, window); got != tc.want {
				t.Errorf("overlaps = %v, want %v", got, tc.want)
			}
		})
	}

	// A zero/absent To before the window must be treated as an instant, not open-ended.
	zeroTo := event(monitorapi.Info, nil, 0, -1)
	zeroTo.To = time.Time{}
	if overlaps(zeroTo, window) {
		t.Errorf("zero-To interval starting before the window must not overlap it")
	}
}

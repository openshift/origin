package pertestintervalserializer

import (
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// attributionNote documents the many-to-many nature of the attribution so offline consumers do not
// mistake temporal overlap for causation or exclusivity.
const attributionNote = "An interval is attributed to every overlapping test window; parallel suites run tests concurrently so attribution is non-exclusive, and overlap is correlation, not causation."

// Report is the top-level per-test interval attribution artifact.
type Report struct {
	GeneratedAt string       `json:"generatedAt"`
	Note        string       `json:"note"`
	TestWindows []TestWindow `json:"testWindows"`
}

// TestWindow holds one e2e test run and the cluster intervals that overlapped it in time.
type TestWindow struct {
	TestName       string           `json:"testName"`
	StartTimestamp string           `json:"startTimestamp"`
	EndTimestamp   string           `json:"endTimestamp"`
	Result         string           `json:"result"`
	IntervalCount  int              `json:"intervalCount"`
	ErrorCount     int              `json:"errorCount"`
	WarningCount   int              `json:"warningCount"`
	InfoCount      int              `json:"infoCount"`
	Intervals      []ReportInterval `json:"intervals"`
}

// ReportInterval is a flattened, serialization-friendly view of a monitorapi.Interval.
type ReportInterval struct {
	Source      string            `json:"source"`
	Level       string            `json:"level"`
	Reason      string            `json:"reason"`
	Namespace   string            `json:"namespace"`
	Name        string            `json:"name"`
	LocatorType string            `json:"locatorType"`
	LocatorKeys map[string]string `json:"locatorKeys"`
	From        string            `json:"from"`
	To          string            `json:"to"`
	Message     string            `json:"message"`
	// Identifier is the watchevents message-hash (hmsg) when present; it lets offline tooling
	// aggregate repeated occurrences of the same underlying signal.
	Identifier string `json:"identifier"`
}

// buildReport clusters every non-e2e-test interval under the e2e test window(s) it overlaps.
func buildReport(finalIntervals monitorapi.Intervals, now time.Time) Report {
	windows := monitorapi.Intervals{}
	attributable := monitorapi.Intervals{}
	for _, interval := range finalIntervals {
		if interval.Source == monitorapi.SourceE2ETest {
			// Each test contributes two kinds of SourceE2ETest records: the raw started/finished
			// marker point-events (From == To) emitted by the ginkgo runner, and the single
			// computed duration interval (From == start, To == finish) emitted by e2etestanalyzer.
			// Only the duration interval describes a window; skipping the markers avoids duplicate
			// and zero-width window entries that would carry the same name/result but bucket nothing.
			//
			// NOTE: this depends on e2etestanalyzer being registered so its computed duration
			// intervals are present in finalIntervals. If that analyzer were ever disabled, no
			// windows would be produced here; we intentionally do not reconstruct windows from the
			// start/finish markers ourselves rather than duplicate that pairing logic.
			if !interval.From.Equal(interval.To) {
				windows = append(windows, interval)
			}
			continue
		}
		attributable = append(attributable, interval)
	}

	report := Report{
		GeneratedAt: now.UTC().Format(time.RFC3339),
		Note:        attributionNote,
		TestWindows: make([]TestWindow, 0, len(windows)),
	}

	for _, window := range windows {
		testName, _ := monitorapi.E2ETestFromLocator(window.Locator)
		tw := TestWindow{
			TestName:       testName,
			StartTimestamp: formatTime(window.From),
			EndTimestamp:   formatTime(window.To),
			Result:         window.Message.Annotations[monitorapi.AnnotationStatus],
			Intervals:      []ReportInterval{},
		}
		for _, candidate := range attributable {
			if !overlaps(candidate, window) {
				continue
			}
			tw.Intervals = append(tw.Intervals, toReportInterval(candidate))
			switch candidate.Level {
			case monitorapi.Error:
				tw.ErrorCount++
			case monitorapi.Warning:
				tw.WarningCount++
			default:
				tw.InfoCount++
			}
		}
		tw.IntervalCount = len(tw.Intervals)
		report.TestWindows = append(report.TestWindows, tw)
	}

	return report
}

// overlaps reports whether candidate's time range intersects window's closed [From, To] interval.
// Unlike utility.IntervalsOverlap, a zero/absent To is treated as an instant at From rather than as
// "open until the end of time", so point intervals are not attributed to every subsequent window.
func overlaps(candidate, window monitorapi.Interval) bool {
	end := candidate.To
	if end.IsZero() {
		end = candidate.From
	}
	return !candidate.From.After(window.To) && !end.Before(window.From)
}

func toReportInterval(interval monitorapi.Interval) ReportInterval {
	keys := map[string]string{}
	for k, v := range interval.Locator.Keys {
		keys[string(k)] = v
	}
	return ReportInterval{
		Source:      string(interval.Source),
		Level:       interval.Level.String(),
		Reason:      string(interval.Message.Reason),
		Namespace:   monitorapi.NamespaceFromLocator(interval.Locator),
		Name:        bestEffortName(interval.Locator),
		LocatorType: string(interval.Locator.Type),
		LocatorKeys: keys,
		From:        formatTime(interval.From),
		To:          formatTime(interval.To),
		Message:     interval.Message.HumanMessage,
		Identifier:  interval.Locator.Keys[monitorapi.LocatorHmsgKey],
	}
}

// bestEffortName returns the most specific object name available in the locator. The full set of
// keys is always preserved in LocatorKeys, so this is only a convenience for quick scanning.
func bestEffortName(l monitorapi.Locator) string {
	for _, k := range []monitorapi.LocatorKey{
		monitorapi.LocatorNameKey,
		monitorapi.LocatorPodKey,
		monitorapi.LocatorNodeKey,
		monitorapi.LocatorDeploymentKey,
		monitorapi.LocatorClusterOperatorKey,
		monitorapi.LocatorServiceKey,
		monitorapi.LocatorRouteKey,
	} {
		if v := l.Keys[k]; v != "" {
			return v
		}
	}
	return ""
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

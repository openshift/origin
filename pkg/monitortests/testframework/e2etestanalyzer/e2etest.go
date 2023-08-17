package e2etestanalyzer

import (
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func intervalsFromEvents_E2ETests(events monitorapi.Intervals, _ monitorapi.ResourcesMap, beginning, end time.Time) monitorapi.Intervals {
	ret := monitorapi.Intervals{}
	testNameToLastStart := map[string]time.Time{}

	for _, event := range events {
		testName, ok := monitorapi.E2ETestFromLocator(event.StructuredLocator)
		if !ok {
			continue
		}
		if event.StructuredMessage.Reason == monitorapi.E2ETestStarted {
			testNameToLastStart[testName] = event.From
			continue
		}
		testStatus, ok := event.StructuredMessage.Annotations[monitorapi.AnnotationStatus]
		if !ok {
			continue
		}

		from := beginning
		if lastStart := testNameToLastStart[testName]; !lastStart.IsZero() {
			from = lastStart
		}
		level := monitorapi.Info
		switch testStatus {
		case "Flaked":
			level = monitorapi.Warning
		case "Failed":
			level = monitorapi.Error
		case "Skipped":
			level = monitorapi.Info
		case "Passed":
			level = monitorapi.Info
		case "Unknown":
			level = monitorapi.Warning
		default:
			level = monitorapi.Warning
		}

		delete(testNameToLastStart, testName)
		ret = append(ret, monitorapi.NewInterval(monitorapi.SourceE2ETest, level).Locator(event.StructuredLocator).
			Message(monitorapi.NewMessage().
				HumanMessagef("e2e test finished As %q", testStatus).
				WithAnnotation(monitorapi.AnnotationStatus, testStatus)).
			Build(from, event.From))
	}

	for testName, testStart := range testNameToLastStart {
		ret = append(ret, monitorapi.NewInterval(monitorapi.SourceE2ETest, monitorapi.Warning).
			Locator(monitorapi.NewLocator().E2ETest(testName)).
			Message(monitorapi.NewMessage().
				HumanMessagef("e2e test did not finish %q", "DidNotFinish").
				WithAnnotation(monitorapi.AnnotationStatus, "DidNotFinish")).
			Build(testStart, end))
	}

	return ret
}

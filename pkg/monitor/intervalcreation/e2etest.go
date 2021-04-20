package intervalcreation

import (
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func IntervalsFromEvents_E2ETests(events monitorapi.Intervals, beginning, end time.Time) monitorapi.Intervals {
	ret := monitorapi.Intervals{}
	testNameToLastStart := map[string]time.Time{}

	for _, event := range events {
		testName, ok := monitorapi.E2ETestFromLocator(event.Locator)
		if !ok {
			continue
		}
		if event.Message == "started" {
			testNameToLastStart[testName] = event.From
			continue
		}
		if !strings.Contains(event.Message, "finishedStatus/") {
			continue
		}

		from := beginning
		if lastStart := testNameToLastStart[testName]; !lastStart.IsZero() {
			from = lastStart
		}
		level := monitorapi.Info
		endState := "MISSING"
		switch {
		case strings.Contains(event.Message, "finishedStatus/Flaked"):
			level = monitorapi.Warning
			endState = "Flaked"
		case strings.Contains(event.Message, "finishedStatus/Failed"):
			level = monitorapi.Error
			endState = "Failed"
		case strings.Contains(event.Message, "finishedStatus/Skipped"):
			level = monitorapi.Info
			endState = "Skipped"
		case strings.Contains(event.Message, "finishedStatus/Passed"):
			level = monitorapi.Info
			endState = "Passed"
		case strings.Contains(event.Message, "finishedStatus/Unknown"):
			level = monitorapi.Warning
			endState = "Unknown"
		}

		delete(testNameToLastStart, testName)
		ret = append(ret, monitorapi.EventInterval{
			Condition: monitorapi.Condition{
				Level:   level,
				Locator: event.Locator,
				Message: fmt.Sprintf("e2e test finished As %q", endState),
			},
			From: from,
			To:   event.From,
		})
	}

	for testName, testStart := range testNameToLastStart {
		ret = append(ret, monitorapi.EventInterval{
			Condition: monitorapi.Condition{
				Level:   monitorapi.Warning,
				Locator: monitorapi.OperatorLocator(testName),
				Message: fmt.Sprintf("e2e test did not finish %q", "DidNotFinish"),
			},
			From: testStart,
			To:   end,
		})
	}

	return ret
}

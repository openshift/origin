package intervalcreation

import (
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	log "github.com/sirupsen/logrus"
)

func IntervalsFromEvents_E2ETests(events monitorapi.Intervals, _ monitorapi.ResourcesMap, beginning, end time.Time) monitorapi.Intervals {
	ret := monitorapi.Intervals{}
	testLocatorToLastStart := map[string]time.Time{}

	for _, event := range events {
		log.Debugf("checking event: %v", event)
		_, ok := monitorapi.E2ETestFromLocator(event.Locator)
		if !ok {
			log.Debugf("not an e2e locator")
			continue
		}
		if event.Message == "started" {
			testLocatorToLastStart[event.Locator] = event.From
			continue
		}
		if !strings.Contains(event.Message, "finishedStatus/") {
			continue
		}

		from := beginning
		if lastStart := testLocatorToLastStart[event.Locator]; !lastStart.IsZero() {
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

		delete(testLocatorToLastStart, event.Locator)
		// add status/Passed to the locator for searching.
		locator := fmt.Sprintf("%s status/%s", event.Locator, endState)
		ret = append(ret, monitorapi.EventInterval{
			Condition: monitorapi.Condition{
				Level:   level,
				Locator: locator,
				Message: fmt.Sprintf("e2e test finished As %q", endState),
			},
			From: from,
			To:   event.From,
		})
	}

	for testLocator, testStart := range testLocatorToLastStart {
		locator := fmt.Sprintf("%s status/%s", testLocator, "DidNotFinish")
		ret = append(ret, monitorapi.EventInterval{
			Condition: monitorapi.Condition{
				Level:   monitorapi.Warning,
				Locator: locator,
				Message: fmt.Sprintf("e2e test did not finish %q", "DidNotFinish"),
			},
			From: testStart,
			To:   end,
		})
	}

	return ret
}

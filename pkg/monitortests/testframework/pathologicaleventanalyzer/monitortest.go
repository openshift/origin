package pathologicaleventanalyzer

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitortestlibrary/pathologicaleventlibrary"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

type pathologicalEventAnalyzer struct {
}

func NewAnalyzer() monitortestframework.MonitorTest {
	return &pathologicalEventAnalyzer{}
}

func (w *pathologicalEventAnalyzer) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *pathologicalEventAnalyzer) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (*pathologicalEventAnalyzer) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return markMissedPathologicalEvents(startingIntervals), nil
}

func (*pathologicalEventAnalyzer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (*pathologicalEventAnalyzer) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*pathologicalEventAnalyzer) Cleanup(ctx context.Context) error {
	return nil
}

var removeNTimes = regexp.MustCompile(`\s+\(\d+ times\)`)
var removeHmsg = regexp.MustCompile(`\s+(hmsg/[0-9a-f]+)`)

// markMissedPathologicalEvents goes through the list of events looking for all events marked
// as "pathological/true" (this implies the event was previously unknown and happened > 20 times).
// For each of those events, this function looks for previous occurrences of that event (with
// times < 20) and marks them as "pathological/true" so that all occurences of that event will
// show in the spyglass chart.
func markMissedPathologicalEvents(events monitorapi.Intervals) monitorapi.Intervals {
	pathologicalEvents := monitorapi.Intervals{}

	// Get the list of events already marked (abbreviated as "am") as pathological/true (this implies times > 20).
	amPathoEvents := map[string]string{}

	for _, pathologicalEvent := range events {
		if !strings.Contains(pathologicalEvent.Message, pathologicaleventlibrary.PathologicalMark) {
			// TODO make a single pathlogical marker to watch, this will require consruction here
			// We only are interested in those EventIntervals with pathological/true
			continue
		}
		if strings.Contains(pathologicalEvent.Message, pathologicaleventlibrary.InterestingMark) {
			// TODO make a single pathlogical marker to watch, this will require consruction here
			// If this message is known, we don't need to process it because we already
			// created an interval when it came in initially.
			continue
		}

		// Events marked as pathological/true have the mark and "n times" number on the message
		// and the locator ends with hmsg/xxxxxxxxxx.
		// Events that are to be marked don't have the pathological/true mark and don't have the hmsg (message hash).
		msgWithoutTimes := removeNTimes.ReplaceAllString(pathologicalEvent.Message, "")
		locWithoutHmsg := removeHmsg.ReplaceAllString(pathologicalEvent.Locator, "")
		// TODO mutate the locator with a constructed bit
		amPathoEvents[msgWithoutTimes+locWithoutHmsg] = pathologicalEvent.Locator
	}
	logrus.Infof("Number of pathological keys: %d", len(amPathoEvents))

	for i, scannedEvent := range events {
		msgWithPathoMark := fmt.Sprintf("%s %s", pathologicaleventlibrary.PathologicalMark, removeNTimes.ReplaceAllString(scannedEvent.Message, ""))
		if pLocator, ok := amPathoEvents[msgWithPathoMark+scannedEvent.Locator]; ok {
			constructedEventCopy := events[i].DeepCopy()

			// This is a match, so update the event with the pathological/true mark and locator that contains the hmsg (message hash).
			constructedEventCopy.Message = fmt.Sprintf("%s %s", pathologicaleventlibrary.PathologicalMark, scannedEvent.Message)
			constructedEventCopy.Locator = pLocator
			logrus.Infof("Found a times match: Locator=%s Message=%s", events[i].Locator, events[i].Message)
			pathologicalEvents = append(pathologicalEvents, *constructedEventCopy)
		}
	}

	return pathologicalEvents
}

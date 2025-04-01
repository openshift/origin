package pathologicaleventanalyzer

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

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

func (w *pathologicalEventAnalyzer) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
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

// getPathologicalEventMapKey returns a string key that can be used in a map to identify other occurrences of the same
// event.
func getPathologicalEventMapKey(interval monitorapi.Interval) string {
	return fmt.Sprintf("%s %s %s", interval.Locator.OldLocator(),
		interval.Message.Reason, interval.Message.HumanMessage)
}

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
		if pathologicalEvent.Message.Annotations[monitorapi.AnnotationPathological] != "true" {
			// We only are interested in those EventIntervals with pathological/true
			continue
		}
		if pathologicalEvent.Message.Annotations[monitorapi.AnnotationInteresting] == "true" {
			// If this message is known, we don't need to process it because we already
			// created an interval when it came in initially.
			continue
		}

		amPathoEvents[getPathologicalEventMapKey(pathologicalEvent)] = pathologicalEvent.Locator.OldLocator()
	}
	logrus.Infof("Number of pathological keys: %d", len(amPathoEvents))

	for i, scannedEvent := range events {
		if _, ok := amPathoEvents[getPathologicalEventMapKey(scannedEvent)]; ok {
			constructedEventCopy := events[i].DeepCopy()

			// This is a match, so update the event with the pathological/true mark and locator that contains the hmsg (message hash).
			constructedEventCopy.Message.Annotations[monitorapi.AnnotationPathological] = "true"
			logrus.Infof("Found a times match: Locator=%s Message=%s", events[i].Locator.OldLocator(), events[i].Message.HumanMessage)
			pathologicalEvents = append(pathologicalEvents, *constructedEventCopy)
		}
	}

	return pathologicalEvents
}

package operatorloganalyzer

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
)

type operatorLeaseCheck struct {
}

func OperatorLeaseCheck() monitortestframework.MonitorTest {
	return &operatorLeaseCheck{}
}

func (w *operatorLeaseCheck) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *operatorLeaseCheck) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *operatorLeaseCheck) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (*operatorLeaseCheck) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	ret := monitorapi.Intervals{}

	leaseIntervals := startingIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		if eventInterval.Message.Reason == monitorapi.LeaseAcquired || eventInterval.Message.Reason == monitorapi.LeaseAcquiringStarted {
			return true
		}
		return false
	})
	sort.Sort(byLeaseAcquisition(leaseIntervals))

	// Deduplicate intervals before processing, refer to https://issues.redhat.com/browse/OCPBUGS-66065
	dedupedLeaseIntervals := []monitorapi.Interval{}
	seen := map[string]bool{}
	for _, interval := range leaseIntervals {
		key := fmt.Sprintf("%v|%v|%v", interval.Locator.OldLocator(), interval.From.Unix(), interval.Message.Reason)
		if !seen[key] {
			seen[key] = true
			dedupedLeaseIntervals = append(dedupedLeaseIntervals, interval)
		}
	}

	podToLeaseIntervals := map[string][]monitorapi.Interval{}

	for _, interval := range dedupedLeaseIntervals {
		podToLeaseIntervals[interval.Locator.OldLocator()] = append(podToLeaseIntervals[interval.Locator.OldLocator()], interval)
	}

	errs := []error{}
	for podLocator, perPodLeaseIntervals := range podToLeaseIntervals {
		var lastAcquiringFrom *time.Time
		for _, interval := range perPodLeaseIntervals {
			switch interval.Message.Reason {
			case monitorapi.LeaseAcquiringStarted:
				// only overwrite if there isn't one already
				if lastAcquiringFrom == nil {
					lastAcquiringFrom = &interval.From
				}

			case monitorapi.LeaseAcquired:
				if lastAcquiringFrom == nil {
					allIntervals := []string{}
					for _, curr := range perPodLeaseIntervals {
						allIntervals = append(allIntervals, curr.String())
					}
					errs = append(errs, fmt.Errorf("missing acquiring stage for %v: all intervals\n\t%v", podLocator, strings.Join(allIntervals, "\n\t")))
				} else {
					ret = append(ret, monitorapi.NewInterval(monitorapi.SourcePodLog, monitorapi.Info).
						Locator(interval.Locator).
						Message(monitorapi.NewMessage().
							Reason(monitorapi.LeaseAcquiring).
							Constructed(monitorapi.ConstructionOwnerLeaseChecker).
							HumanMessage("Waiting for lease."),
						).
						Display().
						Build(*lastAcquiringFrom, interval.From),
					)
					lastAcquiringFrom = nil
				}
			}

		}
	}

	return ret, errors.Join(errs...)
}

type byLeaseAcquisition monitorapi.Intervals

func (intervals byLeaseAcquisition) Less(i, j int) bool {
	// currently synced with https://github.com/openshift/origin/blob/9b001745ec8006eb406bd92e3555d1070b9b656e/pkg/monitor/serialization/serialize.go#L175

	switch d := intervals[i].From.Sub(intervals[j].From); {
	case d < 0:
		return true
	case d > 0:
		return false
	}
	switch d := intervals[i].To.Sub(intervals[j].To); {
	case d < 0:
		return true
	case d > 0:
		return false
	}

	if intervals[i].Message.Reason != intervals[j].Message.Reason {
		// customization to lease acquiring first.  we have things very close in time
		if intervals[i].Message.Reason == monitorapi.LeaseAcquiringStarted {
			return true
		}
		if intervals[i].Message.Reason == monitorapi.LeaseAcquired {
			return false
		}

		return intervals[i].Message.Reason < intervals[j].Message.Reason
	}
	if intervals[i].Message.HumanMessage != intervals[j].Message.HumanMessage {
		return intervals[i].Message.HumanMessage < intervals[j].Message.HumanMessage
	}

	// TODO: this could be a bit slow, but leaving it simple if we can get away with it. Sorting structured locators
	// that use keys is trickier than the old flat string method.
	return intervals[i].Locator.OldLocator() < intervals[j].Locator.OldLocator()
}
func (intervals byLeaseAcquisition) Len() int { return len(intervals) }
func (intervals byLeaseAcquisition) Swap(i, j int) {
	intervals[i], intervals[j] = intervals[j], intervals[i]
}

// getNamespaceForJUnits makes a clone of the known namespaces
func getNamespacesForJUnits() sets.String {
	namespaces := platformidentification.KnownNamespaces.Clone()
	namespaces.Insert("")
	return namespaces
}

func (*operatorLeaseCheck) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	leaseIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		if eventInterval.Message.Reason == monitorapi.LeaseAcquiring {
			return true
		}
		return false
	})

	testNameToFailures := map[string][]string{}

	// we get intermittent hits for openshift-operators that pass
	// but don't show in all aggregated results, causing failures
	// init all known namespaces
	namespaces := getNamespacesForJUnits()
	for ns := range namespaces {
		testName := fmt.Sprintf("[sig-arch] all leases in ns/%s must gracefully release", ns)
		testNameToFailures[testName] = []string{}
	}
	for _, interval := range leaseIntervals {
		ns := monitorapi.NamespaceFromLocator(interval.Locator)
		testName := fmt.Sprintf("[sig-arch] all leases in ns/%s must gracefully release", ns)

		intervalDuration := interval.To.Sub(interval.From)
		if intervalDuration < 10*time.Second {
			_, ok := testNameToFailures[testName]
			if !ok {
				testNameToFailures[testName] = []string{}
			}
			continue
		}

		testNameToFailures[testName] = append(testNameToFailures[testName], interval.String())
	}

	ret := []*junitapi.JUnitTestCase{}
	for testName, failures := range testNameToFailures {
		if len(failures) == 0 {
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: testName,
			})
			continue
		}

		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("had %d non-graceful lease releases\n%s", len(failures), strings.Join(failures, "\n")),
				},
				SystemOut: "sysout",
				SystemErr: "syserr",
			},
			// this is nearly always failing, so make it flake to allow us to introduce it.
			&junitapi.JUnitTestCase{
				Name: testName,
			},
		)
	}

	return ret, nil
}

func (w *operatorLeaseCheck) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*operatorLeaseCheck) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}

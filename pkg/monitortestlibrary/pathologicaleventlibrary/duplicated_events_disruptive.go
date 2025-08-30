package pathologicaleventlibrary

import (
	"fmt"
	"regexp"
	"time"

	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/sirupsen/logrus"
)

type DisruptivePeriod struct {
	start time.Time
	end   time.Time
}

func (p *DisruptivePeriod) In(interval monitorapi.Interval) bool {
	return interval.From.After(p.start) && interval.To.Before(p.end)
}

func (p *DisruptivePeriod) String() string {
	return fmt.Sprintf("Disruptive Period from %s to %s", p.start.Format(time.RFC3339Nano), p.end.Format(time.RFC3339Nano))
}

type DisruptivePeriodList []DisruptivePeriod

func (l *DisruptivePeriodList) GetMatchingPeriod(interval monitorapi.Interval) *DisruptivePeriod {
	for _, disruptionPeriod := range *l {
		if disruptionPeriod.In(interval) {
			return &disruptionPeriod
		}
	}
	return nil
}

type DisruptionAwarePathologicalEventMatcher struct {
	// delegate is a normal event matcher.
	delegate *SimplePathologicalEventMatcher

	disruptivePeriodList DisruptivePeriodList
}

func (ade *DisruptionAwarePathologicalEventMatcher) Name() string {
	return ade.delegate.Name()
}

func (ade *DisruptionAwarePathologicalEventMatcher) Matches(i monitorapi.Interval) bool {
	return ade.delegate.Matches(i)
}

func (ade *DisruptionAwarePathologicalEventMatcher) Allows(i monitorapi.Interval, topology v1.TopologyMode) bool {

	// Check the delegate matcher first, if it matches, proceed to additional checks
	if !ade.delegate.Allows(i, topology) {
		return false
	}

	// Match the pathological event if it happens in-between a disruption period
	disruptionPeriod := ade.disruptivePeriodList.GetMatchingPeriod(i)
	if disruptionPeriod != nil {
		logrus.Infof("ignoring %s pathological event as they fall within range of the disruption period: %s", i, disruptionPeriod)
		return true
	}

	return false
}

func NewDisruptivePeriodList(startMatcher EventMatcher, stopMatcher EventMatcher, events monitorapi.Intervals) DisruptivePeriodList {
	periods := DisruptivePeriodList{}
	var currentPeriod *DisruptivePeriod
	for _, event := range events {
		if startMatcher.Matches(event) {
			currentPeriod = &DisruptivePeriod{
				start: event.From,
			}
		} else if stopMatcher.Matches(event) {
			if currentPeriod != nil {
				currentPeriod.end = event.To
				periods = append(periods, *currentPeriod)
			}
			currentPeriod = nil
		}
	}
	return periods
}

func NewDisruptivePeriodListCordonedPeriods(events monitorapi.Intervals) DisruptivePeriodList {
	startMatcher := &SimplePathologicalEventMatcher{
		name:               "CordonStartMatcher",
		messageReasonRegex: regexp.MustCompile(`^Cordon$`),
		messageHumanRegex:  regexp.MustCompile(`^Cordoned node to apply update$`),
	}
	stopMatcher := &SimplePathologicalEventMatcher{
		name:               "CordonStartMatcher",
		messageReasonRegex: regexp.MustCompile(`^Uncordon$`),
		messageHumanRegex:  regexp.MustCompile(`and node has been uncordoned$`),
	}
	return NewDisruptivePeriodList(startMatcher, stopMatcher, events)
}

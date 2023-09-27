package statetracker

import (
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/apimachinery/pkg/util/sets"
)

type stateTracker struct {
	beginning time.Time

	locatorToStateMap        map[string]stateMap
	locatorsToObservedStates map[string]sets.String

	constructedBy monitorapi.ConstructionOwner
	// intervalSource is used to type/categorize intervals by where they were created.
	intervalSource monitorapi.IntervalSource
}

type intervalCreationFunc func(locator monitorapi.Locator, from, to time.Time) (*monitorapi.IntervalBuilder, bool)

func SimpleInterval(constructedBy monitorapi.ConstructionOwner,
	source monitorapi.IntervalSource, level monitorapi.IntervalLevel,
	reason monitorapi.IntervalReason, message string) intervalCreationFunc {
	return func(locator monitorapi.Locator, from, to time.Time) (*monitorapi.IntervalBuilder, bool) {
		interval := monitorapi.NewInterval(source, level).Locator(locator).
			Message(monitorapi.NewMessage().Reason(reason).
				WithAnnotation(monitorapi.AnnotationConstructed, string(constructedBy)).
				HumanMessage(message))
		return interval, true
	}
}

func NewStateTracker(constructedBy monitorapi.ConstructionOwner,
	src monitorapi.IntervalSource, beginning time.Time) *stateTracker {
	return &stateTracker{
		beginning:                beginning,
		locatorToStateMap:        map[string]stateMap{},
		locatorsToObservedStates: map[string]sets.String{},
		constructedBy:            constructedBy,
		intervalSource:           src,
	}
}

// stateMap is a map from State name to last transition time.
type stateMap map[StateInfo]time.Time

type StateInfo struct {
	stateName string
	reason    monitorapi.IntervalReason
}

func (t *stateTracker) getStates(locator string) stateMap {
	if states, ok := t.locatorToStateMap[locator]; ok {
		return states
	}

	t.locatorToStateMap[locator] = stateMap{}
	return t.locatorToStateMap[locator]
}

func (t *stateTracker) getHasOpenedStates(locator string) sets.String {
	if openedStates, ok := t.locatorsToObservedStates[locator]; ok {
		return openedStates
	}

	t.locatorsToObservedStates[locator] = sets.String{}
	return t.locatorsToObservedStates[locator]
}

func (t *stateTracker) hasOpenedState(locator, stateName string) bool {
	states, ok := t.locatorsToObservedStates[locator]
	if !ok {
		return false
	}

	return states.Has(stateName)
}

func State(stateName string, reason monitorapi.IntervalReason) StateInfo {
	return StateInfo{
		stateName: stateName,
		reason:    reason,
	}
}

func (t *stateTracker) OpenInterval(locator string, state StateInfo, from time.Time) bool {
	states := t.getStates(locator)
	if _, ok := states[state]; ok {
		return true
	}

	states[state] = from
	t.locatorToStateMap[locator] = states

	openedStates := t.getHasOpenedStates(locator)
	openedStates.Insert(state.stateName)
	t.locatorsToObservedStates[locator] = openedStates

	return false
}
func (t *stateTracker) CloseIfOpenedInterval(locator string, state StateInfo, intervalCreator intervalCreationFunc, to time.Time) []monitorapi.Interval {
	states := t.getStates(locator)
	if _, ok := states[state]; !ok {
		return nil
	}

	return t.CloseInterval(locator, state, intervalCreator, to)
}

func (t *stateTracker) CloseInterval(locator string, state StateInfo, intervalCreator intervalCreationFunc, to time.Time) []monitorapi.Interval {
	states := t.getStates(locator)

	from, ok := states[state]
	if !ok {
		if t.hasOpenedState(locator, state.stateName) {
			return nil // nothing to add, this is an extra close for something that already opened at least once.
		}
		// if we have no from and have not opened at all, then this is closing an interval that was in this State from the beginning of the run.
		from = t.beginning
	}
	delete(states, state)
	t.locatorToStateMap[locator] = states

	condition, hasCondition := intervalCreator(locator, from, to)
	if !hasCondition {
		return nil
	}
	return []monitorapi.Interval{
		{
			Condition: condition,
			From:      from,
			To:        to,
		},
	}
}

func (t *stateTracker) CloseAllIntervals(locatorToMessageAnnotations map[string]map[string]string, end time.Time) []monitorapi.Interval {
	ret := []monitorapi.Interval{}
	for locator, states := range t.locatorToStateMap {
		annotationStrings := []string{}
		for k, v := range locatorToMessageAnnotations[locator] {
			annotationStrings = append(annotationStrings, fmt.Sprintf("%v/%v", k, v))
		}

		for stateName := range states {
			message := fmt.Sprintf("%v state/%v never completed", strings.Join(annotationStrings, " "), stateName.stateName)
			ret = append(ret, t.CloseInterval(locator, stateName, SimpleInterval(t.constructedBy, monitorapi.Warning, stateName.reason, message), end)...)
		}
	}

	return ret
}

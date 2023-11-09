package statetracker

import (
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/apimachinery/pkg/util/sets"
)

type stateTracker struct {
	beginning time.Time

	locatorToStateMap        map[string]stateMap
	locatorsToObservedStates map[string]sets.String

	// locators is a hack due to the fact we cannot use Locator objects as map keys because they contain
	// a non-comparable map within them. To work around, we serialize to strings to use as map keys. When closing
	// all remaining intervals we need to actually use the Locator objects themselves, which we don't want to
	// parse from strings lest we get into the troubles we're trying to avoid by using structured locators to begin
	// with. (ex. e2e-test/"my big long test name" which has historically caused parsing problems)
	// Track a map from locator string to Locator for every incoming locator, that we can use when closing remaining
	// intervals.
	locators map[string]monitorapi.Locator

	constructedBy monitorapi.ConstructionOwner
	// intervalSource is used to type/categorize intervals by where they were created.
	intervalSource monitorapi.IntervalSource
}

type intervalCreationFunc func(locator monitorapi.Locator,
	from, to time.Time) (*monitorapi.IntervalBuilder, bool)

func SimpleInterval(source monitorapi.IntervalSource, level monitorapi.IntervalLevel, messageBuilder *monitorapi.MessageBuilder) intervalCreationFunc {
	return func(locator monitorapi.Locator, from, to time.Time) (*monitorapi.IntervalBuilder, bool) {
		interval := monitorapi.NewInterval(source, level).Locator(locator).
			Message(messageBuilder)
		return interval, true
	}
}

func NewStateTracker(constructedBy monitorapi.ConstructionOwner,
	src monitorapi.IntervalSource, beginning time.Time) *stateTracker {
	return &stateTracker{
		beginning:                beginning,
		locatorToStateMap:        map[string]stateMap{},
		locatorsToObservedStates: map[string]sets.String{},
		locators:                 map[string]monitorapi.Locator{},
		constructedBy:            constructedBy,
		intervalSource:           src,
	}
}

// stateMap is a map from State name to last transition time.
type stateMap map[StateInfo]time.Time

type StateInfo struct {
	stateName string
	// row is an optional differentiator to be added as a locator row key, causing intervals to be broken out
	// into separate rows within one group/section. Leave blank to not use.
	row    string
	reason monitorapi.IntervalReason
}

func (t *stateTracker) getStates(locator monitorapi.Locator) stateMap {
	locatorKey := locator.OldLocator()
	if states, ok := t.locatorToStateMap[locatorKey]; ok {
		return states
	}

	t.locatorToStateMap[locatorKey] = stateMap{}
	t.locators[locatorKey] = locator
	return t.locatorToStateMap[locatorKey]
}

func (t *stateTracker) getHasOpenedStates(locator monitorapi.Locator) sets.String {
	locatorKey := locator.OldLocator()
	if openedStates, ok := t.locatorsToObservedStates[locatorKey]; ok {
		return openedStates
	}

	t.locatorsToObservedStates[locatorKey] = sets.String{}
	t.locators[locatorKey] = locator
	return t.locatorsToObservedStates[locatorKey]
}

func (t *stateTracker) hasOpenedState(locator monitorapi.Locator, stateName string) bool {
	states, ok := t.locatorsToObservedStates[locator.OldLocator()]
	if !ok {
		return false
	}

	return states.Has(stateName)
}

func State(stateName, row string, reason monitorapi.IntervalReason) StateInfo {
	return StateInfo{
		stateName: stateName,
		row:       row,
		reason:    reason,
	}
}

func (t *stateTracker) OpenInterval(locator monitorapi.Locator, state StateInfo, from time.Time) bool {
	states := t.getStates(locator)
	if _, ok := states[state]; ok {
		return true
	}

	states[state] = from
	locatorKey := locator.OldLocator()
	t.locatorToStateMap[locatorKey] = states
	t.locators[locatorKey] = locator

	openedStates := t.getHasOpenedStates(locator)
	openedStates.Insert(state.stateName)
	t.locatorsToObservedStates[locatorKey] = openedStates

	return false
}
func (t *stateTracker) CloseIfOpenedInterval(locator monitorapi.Locator, state StateInfo, intervalCreator intervalCreationFunc, to time.Time) []monitorapi.Interval {
	states := t.getStates(locator)
	if _, ok := states[state]; !ok {
		return nil
	}

	return t.CloseInterval(locator, state, intervalCreator, to)
}

func (t *stateTracker) CloseInterval(locator monitorapi.Locator, state StateInfo, intervalCreator intervalCreationFunc, to time.Time) []monitorapi.Interval {
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
	locatorKey := locator.OldLocator()
	t.locatorToStateMap[locatorKey] = states
	t.locators[locatorKey] = locator
	locatorWithRow := locator.DeepCopy()
	if state.row != "" {
		locatorWithRow.Keys[monitorapi.LocatorRowKey] = state.row
	}

	ib, hasCondition := intervalCreator(*locatorWithRow, from, to)
	if !hasCondition {
		return nil
	}
	return []monitorapi.Interval{ib.Build(from, to)}
}

func (t *stateTracker) CloseAllIntervals(locatorToMessageAnnotations map[string]map[string]string, end time.Time) []monitorapi.Interval {
	ret := []monitorapi.Interval{}
	for locator, states := range t.locatorToStateMap {
		annotations := map[monitorapi.AnnotationKey]string{}
		for k, v := range locatorToMessageAnnotations[locator] {
			annotations[monitorapi.AnnotationKey(k)] = v
		}

		l := t.locators[locator]
		for state := range states {
			annotations[monitorapi.AnnotationState] = state.stateName
			annotations[monitorapi.AnnotationConstructed] = string(t.constructedBy)
			mb := monitorapi.NewMessage().WithAnnotations(annotations).HumanMessage("never completed").Reason(state.reason)
			ret = append(ret, t.CloseInterval(l, state, SimpleInterval(t.intervalSource, monitorapi.Warning, mb), end)...)
		}
	}

	return ret
}

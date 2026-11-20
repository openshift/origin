package pathologicaleventlibrary

import (
	"cmp"
	"slices"

	v1 "github.com/openshift/api/config/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// mergeOverlappingIntervals returns the intervals sorted by start with overlapping or
// touching ones joined into one, so that coverage of the union has no holes an increment
// could be lost to. Gaps between intervals stay gaps: nothing merged across them is covered.
func mergeOverlappingIntervals(in monitorapi.Intervals) monitorapi.Intervals {
	if len(in) == 0 {
		return nil
	}
	sorted := append(monitorapi.Intervals{}, in...)
	slices.SortStableFunc(sorted, func(a, b monitorapi.Interval) int {
		if c := a.From.Compare(b.From); c != 0 {
			return c
		}
		return a.To.Compare(b.To)
	})
	merged := monitorapi.Intervals{sorted[0]}
	for _, next := range sorted[1:] {
		last := &merged[len(merged)-1]
		if next.From.After(last.To) {
			merged = append(merged, next)
			continue
		}
		if next.To.After(last.To) {
			last.To = next.To
		}
	}
	return merged
}

// crossEventMatcherSpec declares one cross-gate event kind: the matcher name and the
// repeat cap each gate enforces against its own occurrences.
type crossEventMatcherSpec struct {
	name string
	cap  int
}

// gateAllowance is one eligibility gate's in-window allowance: the windows its eligible
// passing tests authorize and the repeat cap that binds the occurrences inside them.
type gateAllowance struct {
	allowIfWithinIntervals  monitorapi.Intervals
	repeatThresholdOverride int
}

// crossGateCount is the tally of one Event object's occurrences at one observation.
type crossGateCount struct {
	// inUnionWindows counts the occurrences provably inside the union of all gates'
	// windows; Allows subtracts them from the count observed outside every window.
	inUnionWindows int
	// perGate counts, per gate, the occurrences attributed to that gate's windows. An
	// increment whose bounding observations fall in different gates' windows is counted
	// for both gates, so a cap is exceeded only by occurrences that gate plausibly
	// authorized, never by attribution luck.
	perGate []int
}

// CrossGateOverlapPathologicalEventMatcher allows a repeated event whose Kubernetes Event
// object crosses several eligibility gates: one locator accumulates a single cumulative
// counter across all gates' windows, so the gates cannot be evaluated as independent
// matchers combined by OR, where the gate that did not authorize an observation would
// re-admit it through its discount. One matcher owns the whole cross-gate evaluation:
//
//   - an observation inside one or more gates' windows is allowed when every containing
//     gate's own count of the object's occurrences stays within that gate's cap;
//   - an observation outside every gate's windows is allowed when, after subtracting the
//     occurrences the union of the windows covers, the residual count stays within
//     DuplicateEventThreshold, and no gate's cap was exceeded by its own occurrences.
type CrossGateOverlapPathologicalEventMatcher struct {
	// delegate is a normal event matcher; its repeatThresholdOverride is unused because
	// the caps come from the gates.
	delegate *SimplePathologicalEventMatcher
	// gates holds each gate's windows and cap, in registration order; perGate indexes follow it.
	gates []gateAllowance
	// useLastTimestamp attributes a repeated event to its most recent occurrence instead of
	// requiring the full lifetime of the Kubernetes Event object to fit within the interval.
	useLastTimestamp bool
	// unionWindows is the merge of all gates' windows; the outside discount credits and the
	// cross-gate increment coverage come from it.
	unionWindows monitorapi.Intervals
	// tally maps each observed revision of an Event object to its cross-gate count.
	// Observations with a missing or empty UID are excluded.
	tally map[eventObservation]crossGateCount
}

func (ade *CrossGateOverlapPathologicalEventMatcher) Name() string {
	return ade.delegate.Name()
}

func (ade *CrossGateOverlapPathologicalEventMatcher) Matches(i monitorapi.Interval) bool {
	return ade.delegate.Matches(i)
}

// discountInWindowOccurrences replays each matching Event object's cumulative counter and
// records, per observation, the tally that Allows enforces. The replay keeps the
// err-toward-rejecting rules of OverlapOtherIntervalsPathologicalEventMatcher.discountInWindowOccurrences:
// an increment counts as covered only when it is a single occurrence inside the union or when
// both bounding observations fall within one merged union interval; a count lower than the
// previous one means the Event object was recreated, so the tally restarts.
func (ade *CrossGateOverlapPathologicalEventMatcher) discountInWindowOccurrences(finalIntervals monitorapi.Intervals) {
	ade.unionWindows = mergeOverlappingIntervals(ade.gateWindows())
	ade.tally = map[eventObservation]crossGateCount{}

	observationsByEvent := map[string]monitorapi.Intervals{}
	for _, i := range finalIntervals {
		if i.Source != monitorapi.SourceKubeEvent || !ade.delegate.Matches(i) {
			continue
		}
		key, valid := eventObjectKey(i)
		if !valid {
			continue
		}
		observationsByEvent[key] = append(observationsByEvent[key], i)
	}

	for _, observations := range observationsByEvent {
		slices.SortStableFunc(observations, func(a, b monitorapi.Interval) int {
			if c := a.From.Compare(b.From); c != 0 {
				return c
			}
			return cmp.Compare(GetTimesAnEventHappened(a.Message), GetTimesAnEventHappened(b.Message))
		})

		var previous *monitorapi.Interval
		previousCount := 0
		count := crossGateCount{perGate: make([]int, len(ade.gates))}
		for idx := range observations {
			observation := observations[idx]
			counted := GetTimesAnEventHappened(observation.Message)
			increment := counted - previousCount
			if counted < previousCount {
				previous, increment = nil, counted
				count = crossGateCount{perGate: make([]int, len(ade.gates))}
			}
			if increment == 1 && ade.observationInUnion(observation) {
				// A single occurrence happened at the observed timestamp, so only the gates
				// whose windows contain the observation are charged.
				count.inUnionWindows += increment
				for gate := range ade.gates {
					if ade.observationInGate(gate, observation) {
						count.perGate[gate] += increment
					}
				}
			} else if previous != nil && ade.coveredByUnion(*previous, observation) {
				// A batched increment lies somewhere between the bounding observations; when
				// one merged union interval covers both, every gate whose windows contain either
				// bounding observation is charged, so the increment can never slip past a cap.
				count.inUnionWindows += increment
				for gate := range ade.gates {
					if ade.observationInGate(gate, *previous) || ade.observationInGate(gate, observation) {
						count.perGate[gate] += increment
					}
				}
			}
			previous, previousCount = &observations[idx], counted
			observedEvent := newEventObservation(observation)
			if observedEvent.event.valid {
				ade.tally[observedEvent] = crossGateCount{
					inUnionWindows: count.inUnionWindows,
					perGate:        append([]int(nil), count.perGate...),
				}
			} else {
				logrus.WithFields(logrus.Fields{
					"observedEvent": observedEvent,
					"observation":   observation,
				}).Info("observed event in observation skipped because missing unique identifier")
			}
		}
	}
}

// gateWindows concatenates every gate's windows, unmerged.
func (ade *CrossGateOverlapPathologicalEventMatcher) gateWindows() monitorapi.Intervals {
	var windows monitorapi.Intervals
	for _, gate := range ade.gates {
		windows = append(windows, gate.allowIfWithinIntervals...)
	}
	return windows
}

// observationInUnion reports whether the event falls inside the union of the gates' windows.
func (ade *CrossGateOverlapPathologicalEventMatcher) observationInUnion(i monitorapi.Interval) bool {
	return len(intervalsContaining(i, ade.useLastTimestamp, ade.unionWindows)) > 0
}

// observationInGate reports whether the event falls inside gate's own windows.
func (ade *CrossGateOverlapPathologicalEventMatcher) observationInGate(gate int, i monitorapi.Interval) bool {
	return len(intervalsContaining(i, ade.useLastTimestamp, ade.gates[gate].allowIfWithinIntervals)) > 0
}

// coveredByUnion reports whether a single merged union interval contains both events, so
// that everything that happened between them is inside the union too.
func (ade *CrossGateOverlapPathologicalEventMatcher) coveredByUnion(a, b monitorapi.Interval) bool {
	containsA := sets.New(intervalsContaining(a, ade.useLastTimestamp, ade.unionWindows)...)
	return containsA.HasAny(intervalsContaining(b, ade.useLastTimestamp, ade.unionWindows)...)
}

// Allows reports whether this matcher permits the repeated event, by the rules on the
// CrossGateOverlapPathologicalEventMatcher doc. Without a tally entry for the observation
// (no UID, or the observation is missing from finalIntervals), the cumulative count stands
// in for the gate-local count.
func (ade *CrossGateOverlapPathologicalEventMatcher) Allows(i monitorapi.Interval, topology v1.TopologyMode) bool {
	var containingGates []int
	for gate := range ade.gates {
		if ade.observationInGate(gate, i) {
			containingGates = append(containingGates, gate)
		}
	}
	if len(containingGates) > 0 {
		if !ade.delegate.allowsIgnoringRepeatThreshold(i, topology) {
			return false
		}
		counted := crossGateCount{}
		if observation, ok := ade.tally[newEventObservation(i)]; ok {
			counted = observation
		} else {
			// No tally entry: charge the whole cumulative count to every containing gate.
			counted.inUnionWindows = GetTimesAnEventHappened(i.Message)
			counted.perGate = make([]int, len(ade.gates))
			for _, gate := range containingGates {
				counted.perGate[gate] = counted.inUnionWindows
			}
		}
		for _, gate := range containingGates {
			if counted.perGate[gate] > ade.gates[gate].repeatThresholdOverride {
				logrus.WithField("allower", ade.Name).Debugf("event repeated over gate cap %d", ade.gates[gate].repeatThresholdOverride)
				return false
			}
		}
		logrus.Infof("%s was found to overlap with the allowed intervals of %s, ignoring pathological event", i, ade.Name())
		return true
	}

	if ade.tally == nil {
		return false
	}
	if !ade.delegate.allowsIgnoringRepeatThreshold(i, topology) {
		return false
	}
	observation := newEventObservation(i)
	if !observation.event.valid {
		logrus.WithFields(logrus.Fields{
			"observedEvent": observation,
			"observation":   i,
		}).Info("observed event in observation has an invalid unique identifier")
		return false
	}
	counted := ade.tally[observation]
	for gate := range ade.gates {
		if counted.perGate[gate] > ade.gates[gate].repeatThresholdOverride {
			logrus.WithField("allower", ade.Name).Debugf("event repeated over gate cap %d outside the gates' windows", ade.gates[gate].repeatThresholdOverride)
			return false
		}
	}
	outsideWindows := GetTimesAnEventHappened(i.Message) - counted.inUnionWindows
	if counted.inUnionWindows > 0 && outsideWindows <= DuplicateEventThreshold {
		logrus.Infof("%s repeated %d times outside the allowed intervals of %s, ignoring pathological event",
			i, outsideWindows, ade.Name())
		return true
	}
	return false
}

package monitorserialization

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Event is not an interval.  It is an instant.  The instant removes any ambiguity about "when"
type EventInterval struct {
	Level string `json:"level"`

	Locator string `json:"locator"`
	Message string `json:"message"`

	// TODO: we're hoping to move these to just locator/message when everything is ready.
	StructuredLocator monitorapi.Locator `json:"tempStructuredLocator"`
	StructuredMessage monitorapi.Message `json:"tempStructuredMessage"`

	From metav1.Time `json:"from"`
	To   metav1.Time `json:"to"`
}

// EventList is not an interval.  It is an instant.  The instant removes any ambiguity about "when"
type EventIntervalList struct {
	Items []EventInterval `json:"items"`
}

func EventsToFile(filename string, events monitorapi.Intervals) error {
	json, err := EventsToJSON(events)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, json, 0644)
}

func EventsFromFile(filename string) (monitorapi.Intervals, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return EventsFromJSON(data)
}

func EventsFromJSON(data []byte) (monitorapi.Intervals, error) {
	var list EventIntervalList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	events := make(monitorapi.Intervals, 0, len(list.Items))
	for _, interval := range list.Items {
		level, err := monitorapi.ConditionLevelFromString(interval.Level)
		if err != nil {
			return nil, err
		}
		events = append(events, monitorapi.Interval{
			Condition: monitorapi.Condition{
				Level:   level,
				Locator: interval.Locator,
				Message: interval.Message,
			},

			From: interval.From.Time,
			To:   interval.To.Time,
		})
	}

	return events, nil
}

func IntervalFromJSON(data []byte) (*monitorapi.Interval, error) {
	var serializedInterval EventInterval
	if err := json.Unmarshal(data, &serializedInterval); err != nil {
		return nil, err
	}
	level, err := monitorapi.ConditionLevelFromString(serializedInterval.Level)
	if err != nil {
		return nil, err
	}
	return &monitorapi.Interval{
		Condition: monitorapi.Condition{
			Level:   level,
			Locator: serializedInterval.Locator,
			Message: serializedInterval.Message,
		},

		From: serializedInterval.From.Time,
		To:   serializedInterval.To.Time,
	}, nil
}

func IntervalToOneLineJSON(interval monitorapi.Interval) ([]byte, error) {
	outputEvent := monitorEventIntervalToEventInterval(interval)

	spacedBytes, err := json.Marshal(outputEvent)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	if err := json.Compact(buf, spacedBytes); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func EventsToJSON(events monitorapi.Intervals) ([]byte, error) {
	outputEvents := []EventInterval{}
	for _, curr := range events {
		outputEvents = append(outputEvents, monitorEventIntervalToEventInterval(curr))
	}

	sort.Sort(byTime(outputEvents))
	list := EventIntervalList{Items: outputEvents}
	return json.MarshalIndent(list, "", "    ")
}

func EventsIntervalsToFile(filename string, events monitorapi.Intervals) error {
	json, err := EventsIntervalsToJSON(events)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, json, 0644)
}

func EventsIntervalsToJSON(events monitorapi.Intervals) ([]byte, error) {
	outputEvents := []EventInterval{}
	for _, curr := range events {
		if curr.From == curr.To && !curr.To.IsZero() {
			continue
		}
		outputEvents = append(outputEvents, monitorEventIntervalToEventInterval(curr))
	}

	sort.Sort(byTime(outputEvents))
	list := EventIntervalList{Items: outputEvents}
	return json.MarshalIndent(list, "", "    ")
}

func monitorEventIntervalToEventInterval(interval monitorapi.Interval) EventInterval {
	ret := EventInterval{
		Level:             fmt.Sprintf("%v", interval.Level),
		Locator:           interval.Locator,
		Message:           interval.Message,
		StructuredLocator: interval.StructuredLocator,
		StructuredMessage: interval.StructuredMessage,

		From: metav1.Time{Time: interval.From},
		To:   metav1.Time{Time: interval.To},
	}

	return ret
}

type byTime []EventInterval

func (intervals byTime) Less(i, j int) bool {
	switch d := intervals[i].From.Sub(intervals[j].From.Time); {
	case d < 0:
		return true
	case d > 0:
		return false
	}
	return intervals[i].Locator < intervals[j].Locator
}
func (intervals byTime) Len() int { return len(intervals) }
func (intervals byTime) Swap(i, j int) {
	intervals[i], intervals[j] = intervals[j], intervals[i]
}

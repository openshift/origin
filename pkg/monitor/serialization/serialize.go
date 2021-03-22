package monitorserialization

import (
	"encoding/json"
	"io/ioutil"
	"sort"

	"github.com/openshift/origin/pkg/monitor"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Event is not an interval.  It is an instant.  The instant removes any ambiguity about "when"
type Event struct {
	Level string `json:"level"`

	Locator string `json:"locator"`
	Message string `json:"message"`

	InitiatedAt metav1.Time `json:"initiatedAt"`
}

// EventList is not an interval.  It is an instant.  The instant removes any ambiguity about "when"
type EventList struct {
	Items []Event `json:"items"`
}

func EventsToFile(filename string, events []*monitor.EventInterval) error {
	json, err := EventsToJSON(events)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, json, 0644)
}

func EventsToJSON(events []*monitor.EventInterval) ([]byte, error) {
	outputEvents := []Event{}
	for _, curr := range events {
		outputEvents = append(outputEvents, monitorIntervalToEvent(curr)...)
	}

	sort.Sort(byTime(outputEvents))
	list := EventList{Items: outputEvents}
	return json.Marshal(list)
}

func monitorConditionToEvent(condition *monitor.Condition) Event {
	ret := Event{
		Locator:     condition.Locator,
		Message:     condition.Message,
		InitiatedAt: metav1.Time{condition.InitiatedAt},
	}
	switch condition.Level {
	case monitor.Error:
		ret.Level = "Error"
	case monitor.Warning:
		ret.Level = "Warning"
	case monitor.Info:
		ret.Level = "Info"
	default:
		ret.Level = "Unknown"
	}

	return ret
}

func monitorIntervalToEvent(curr *monitor.EventInterval) []Event {
	if curr.To == curr.From {
		return []Event{monitorConditionToEvent(curr.Condition)}
	}

	first := monitorConditionToEvent(curr.Condition)
	first.InitiatedAt = metav1.Time{curr.From}
	second := monitorConditionToEvent(curr.Condition)
	second.InitiatedAt = metav1.Time{curr.To}
	return []Event{first, second}
}

type byTime []Event

func (intervals byTime) Less(i, j int) bool {
	switch d := intervals[i].InitiatedAt.Sub(intervals[j].InitiatedAt.Time); {
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

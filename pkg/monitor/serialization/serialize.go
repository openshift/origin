package monitorserialization

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Event is not an interval.  It is an instant.  The instant removes any ambiguity about "when"
type EventInterval struct {
	Level string `json:"level"`

	Locator string `json:"locator"`
	Message string `json:"message"`

	From metav1.Time `json:"from"`
	To   metav1.Time `json:"to"`
}

// EventList is not an interval.  It is an instant.  The instant removes any ambiguity about "when"
type EventIntervalList struct {
	Items []EventInterval `json:"items"`
}

func EventsToFile(filename string, rawEventsFilename string, events monitorapi.Intervals, rawEvents []corev1.Event) error {
	json, err := EventsToJSON(events)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filename, json, 0644)
	if err != nil {
		return err
	}
	json, err = RawEventsToJSON(rawEvents)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(rawEventsFilename, json, 0644)
	if err != nil {
		return err
	}
	return nil
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
		level, err := monitorapi.EventLevelFromString(interval.Level)
		if err != nil {
			return nil, err
		}
		events = append(events, monitorapi.EventInterval{
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

func EventsToJSON(events monitorapi.Intervals) ([]byte, error) {
	outputEvents := []EventInterval{}
	for _, curr := range events {
		outputEvents = append(outputEvents, monitorEventIntervalToEventInterval(curr))
	}

	sort.Sort(byTime(outputEvents))
	list := EventIntervalList{Items: outputEvents}
	return json.MarshalIndent(list, "", "    ")
}

func RawEventsToJSON(rawEvents []corev1.Event) ([]byte, error) {
	type rawEventList struct {
		items []corev1.Event
	}
	fmt.Println("In RawEventsToJSON: ", len(rawEvents))
	list := rawEventList{items: rawEvents}
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

func monitorEventIntervalToEventInterval(interval monitorapi.EventInterval) EventInterval {
	ret := EventInterval{
		Level:   fmt.Sprintf("%v", interval.Level),
		Locator: interval.Locator,
		Message: interval.Message,

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

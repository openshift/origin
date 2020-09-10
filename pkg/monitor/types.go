package monitor

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type SamplerFunc func(time.Time) []*Condition

type Interface interface {
	Events(from, to time.Time) EventIntervals
	Conditions(from, to time.Time) EventIntervals
}

type Recorder interface {
	Record(conditions ...Condition)
	AddSampler(fn SamplerFunc)
}

type EventLevel int

const (
	Info EventLevel = iota
	Warning
	Error
)

var eventString = []string{
	"I",
	"W",
	"E",
}

type Event struct {
	Condition

	At time.Time
}

func (e *Event) String() string {
	return fmt.Sprintf("%s.%03d %s %s %s", e.At.Format("Jan 02 15:04:05"), e.At.Nanosecond()/1000000, eventString[e.Level], e.Locator, strings.Replace(e.Message, "\n", "\\n", -1))
}

type sample struct {
	at         time.Time
	conditions []*Condition
}

type Condition struct {
	Level EventLevel

	Locator string
	Message string

	InitiatedAt time.Time
}

type EventInterval struct {
	*Condition

	From time.Time
	To   time.Time
}

func (i *EventInterval) String() string {
	if i.From.Equal(i.To) {
		return fmt.Sprintf("%s.%03d %s %s %s", i.From.Format("Jan 02 15:04:05"), i.From.Nanosecond()/int(time.Millisecond), eventString[i.Level], i.Locator, strings.Replace(i.Message, "\n", "\\n", -1))
	}
	duration := i.To.Sub(i.From)
	if duration < time.Second {
		return fmt.Sprintf("%s.%03d - %-5s %s %s %s", i.From.Format("Jan 02 15:04:05"), i.From.Nanosecond()/int(time.Millisecond), strconv.Itoa(int(duration/time.Millisecond))+"ms", eventString[i.Level], i.Locator, strings.Replace(i.Message, "\n", "\\n", -1))
	}
	return fmt.Sprintf("%s.%03d - %-5s %s %s %s", i.From.Format("Jan 02 15:04:05"), i.From.Nanosecond()/int(time.Millisecond), strconv.Itoa(int(duration/time.Second))+"s", eventString[i.Level], i.Locator, strings.Replace(i.Message, "\n", "\\n", -1))
}

type EventIntervals []*EventInterval

var _ sort.Interface = EventIntervals{}

func (intervals EventIntervals) Less(i, j int) bool {
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
	return intervals[i].Message < intervals[j].Message
}
func (intervals EventIntervals) Len() int { return len(intervals) }
func (intervals EventIntervals) Swap(i, j int) {
	intervals[i], intervals[j] = intervals[j], intervals[i]
}

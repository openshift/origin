package monitorapi

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type EventLevel int

const (
	Info EventLevel = iota
	Warning
	Error
)

func (e EventLevel) String() string {
	switch e {
	case Info:
		return "Info"
	case Warning:
		return "Warning"
	case Error:
		return "Error"
	default:
		panic(fmt.Sprintf("did not define event level string for %d", e))
	}
}

func EventLevelFromString(s string) (EventLevel, error) {
	switch s {
	case "Info":
		return Info, nil
	case "Warning":
		return Warning, nil
	case "Error":
		return Error, nil
	default:
		return Error, fmt.Errorf("did not define event level string for %q", s)
	}

}

type Event struct {
	Condition

	At time.Time
}

func (e *Event) String() string {
	return fmt.Sprintf("%s.%03d %s %s %s", e.At.Format("Jan 02 15:04:05"), e.At.Nanosecond()/1000000, e.Level.String()[:1], e.Locator, strings.Replace(e.Message, "\n", "\\n", -1))
}

type Condition struct {
	Level EventLevel

	Locator string
	Message string
}

type EventInterval struct {
	Condition

	From time.Time
	To   time.Time
}

func (i EventInterval) String() string {
	if i.From.Equal(i.To) {
		return fmt.Sprintf("%s.%03d %s %s %s", i.From.Format("Jan 02 15:04:05"), i.From.Nanosecond()/int(time.Millisecond), i.Level.String()[:1], i.Locator, strings.Replace(i.Message, "\n", "\\n", -1))
	}
	duration := i.To.Sub(i.From)
	if duration < time.Second {
		return fmt.Sprintf("%s.%03d - %-5s %s %s %s", i.From.Format("Jan 02 15:04:05"), i.From.Nanosecond()/int(time.Millisecond), strconv.Itoa(int(duration/time.Millisecond))+"ms", i.Level.String()[:1], i.Locator, strings.Replace(i.Message, "\n", "\\n", -1))
	}
	return fmt.Sprintf("%s.%03d - %-5s %s %s %s", i.From.Format("Jan 02 15:04:05"), i.From.Nanosecond()/int(time.Millisecond), strconv.Itoa(int(duration/time.Second))+"s", i.Level.String()[:1], i.Locator, strings.Replace(i.Message, "\n", "\\n", -1))
}

type EventIntervals []EventInterval

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

type Events []*Event

var _ sort.Interface = Events{}

func (events Events) Less(i, j int) bool {
	switch d := events[i].At.Sub(events[j].At); {
	case d < 0:
		return true
	case d > 0:
		return false
	default:
		return true
	}
}
func (events Events) Len() int { return len(events) }
func (events Events) Swap(i, j int) {
	events[i], events[j] = events[j], events[i]
}

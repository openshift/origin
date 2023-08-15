package monitor

import (
	"fmt"
	"io"
	"os"
	"time"

	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/apimachinery/pkg/runtime"
)

type jsonlRecorder struct {
	delegate              monitorapi.Recorder
	intervalDisplayFilter monitorapi.EventIntervalMatchesFunc

	outfile io.Writer
}

func WrapWithJSONLRecorder(delegate monitorapi.Recorder, outfile io.Writer, intervalDisplayFilter monitorapi.EventIntervalMatchesFunc) monitorapi.Recorder {
	return &jsonlRecorder{
		delegate:              delegate,
		outfile:               outfile,
		intervalDisplayFilter: intervalDisplayFilter,
	}
}

var _ monitorapi.Recorder = &jsonlRecorder{}

func (m *jsonlRecorder) CurrentResourceState() monitorapi.ResourcesMap {
	return m.delegate.CurrentResourceState()
}

func (m *jsonlRecorder) RecordResource(resourceType string, obj runtime.Object) {
	m.delegate.RecordResource(resourceType, obj)
}

// Record captures one or more conditions at the current time. All conditions are recorded
// in monotonic order as EventInterval objects.
func (m *jsonlRecorder) Record(conditions ...monitorapi.Condition) {
	m.RecordAt(time.Now().UTC(), conditions...)
}

// AddIntervals provides a mechanism to directly inject eventIntervals
func (m *jsonlRecorder) AddIntervals(intervals ...monitorapi.Interval) {
	for _, curr := range intervals {
		m.writeInterval(&curr)
	}
	m.delegate.AddIntervals(intervals...)
}

// StartInterval inserts a record at time t with the provided condition and returns an opaque
// locator to the interval. The caller may close the sample at any point by invoking EndInterval().
func (m *jsonlRecorder) StartInterval(t time.Time, condition monitorapi.Condition) int {
	return m.delegate.StartInterval(t, condition)
}

// EndInterval updates the To of the interval started by StartInterval if it is greater than
// the from.
func (m *jsonlRecorder) EndInterval(startedInterval int, t time.Time) *monitorapi.Interval {
	ret := m.delegate.EndInterval(startedInterval, t)
	m.writeInterval(ret)

	return ret
}

func (m *jsonlRecorder) writeInterval(interval *monitorapi.Interval) {
	if interval == nil {
		return
	}
	if m.intervalDisplayFilter != nil && !m.intervalDisplayFilter(*interval) {
		return
	}

	intervalJSON, err := monitorserialization.IntervalToOneLineJSON(*interval)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error serializing: %v\n", err)
	}
	if _, err := m.outfile.Write([]byte(fmt.Sprintf("%v\n", string(intervalJSON)))); err != nil {
		fmt.Fprintf(os.Stderr, "error writing: %v\n", err)
	}
}

// RecordAt captures one or more conditions at the provided time. All conditions are recorded
// as EventInterval objects.
func (m *jsonlRecorder) RecordAt(t time.Time, conditions ...monitorapi.Condition) {
	if len(conditions) == 0 {
		return
	}
	intervals := monitorapi.Intervals{}
	for _, condition := range conditions {
		intervals = append(intervals, monitorapi.Interval{
			Condition: condition,
			From:      t,
			To:        t,
		})
	}
	m.AddIntervals(intervals...)
}

func (m *jsonlRecorder) Intervals(from, to time.Time) monitorapi.Intervals {
	return m.delegate.Intervals(from, to)
}

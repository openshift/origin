package etcdloganalyzer

import (
	"fmt"
	"sort"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// BatchEtcdLogIntervals batches EtcdLog intervals into 1-minute increments,
// grouped by locator and humanMessage. The count annotation tracks how many
// duplicate intervals were batched together.
// Only intervals marked with the batchable annotation are batched; leadership-related
// intervals are left unchanged to avoid breaking other parts of the code.
func BatchEtcdLogIntervals(intervals monitorapi.Intervals) monitorapi.Intervals {
	// Filter to only EtcdLog intervals that are marked as batchable
	etcdIntervals := intervals.Filter(func(i monitorapi.Interval) bool {
		return i.Source == monitorapi.SourceEtcdLog &&
			i.Message.Annotations[monitorapi.AnnotationBatchable] == "true"
	})

	if len(etcdIntervals) == 0 {
		return monitorapi.Intervals{}
	}

	// Sort by time first
	sort.Sort(etcdIntervals)

	// batchKey groups intervals by locator + humanMessage + minute bucket
	type batchKey struct {
		locator      string
		humanMessage string
		minuteBucket time.Time
	}

	// batchedInterval holds the aggregated data for a batch
	type batchedInterval struct {
		interval monitorapi.Interval
		count    int
	}

	batches := make(map[batchKey]*batchedInterval)
	var batchOrder []batchKey // preserve order for deterministic output

	for _, interval := range etcdIntervals {
		// Truncate to minute for bucketing
		minuteBucket := interval.From.Truncate(time.Minute)

		key := batchKey{
			locator:      interval.Locator.OldLocator(),
			humanMessage: interval.Message.HumanMessage,
			minuteBucket: minuteBucket,
		}

		if existing, ok := batches[key]; ok {
			existing.count++
			// Extend the interval to cover the full minute or to the latest event
			if interval.To.After(existing.interval.To) {
				existing.interval.To = interval.To
			}
		} else {
			// Create a new batch entry
			// Set the from time to the start of the minute bucket
			// Set the to time to the end of the minute bucket (or interval.To if later)
			batchFrom := minuteBucket
			batchTo := minuteBucket.Add(time.Minute)
			if interval.To.After(batchTo) {
				batchTo = interval.To
			}

			// Copy the interval and modify the times
			batchedInt := interval
			batchedInt.From = batchFrom
			batchedInt.To = batchTo

			batches[key] = &batchedInterval{
				interval: batchedInt,
				count:    1,
			}
			batchOrder = append(batchOrder, key)
		}
	}

	// Convert batches back to intervals, adding count annotation
	result := make(monitorapi.Intervals, 0, len(batches))
	for _, key := range batchOrder {
		batch := batches[key]

		// Create a new message with the count annotation, removing the batchable marker
		newAnnotations := make(map[monitorapi.AnnotationKey]string)
		for k, v := range batch.interval.Message.Annotations {
			newAnnotations[k] = v
		}
		delete(newAnnotations, monitorapi.AnnotationBatchable)
		newAnnotations[monitorapi.AnnotationCount] = fmt.Sprintf("%d", batch.count)

		resultInterval := monitorapi.Interval{
			Condition: monitorapi.Condition{
				Level:   batch.interval.Level,
				Locator: batch.interval.Locator,
				Message: monitorapi.Message{
					Reason:       batch.interval.Message.Reason,
					Cause:        batch.interval.Message.Cause,
					HumanMessage: batch.interval.Message.HumanMessage,
					Annotations:  newAnnotations,
				},
			},
			Source:  batch.interval.Source,
			Display: batch.interval.Display,
			From:    batch.interval.From,
			To:      batch.interval.To,
		}
		result = append(result, resultInterval)
	}

	// Sort by time for consistent output
	sort.Sort(result)

	return result
}

package operatorstateanalyzer

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperatorStateAnalyzer(t *testing.T) {
	tests := []struct {
		name            string
		intervals       monitorapi.Intervals
		expectedMetrics map[string]*OperatorStateMetrics
		expectedRows    []map[string]string
	}{
		{
			name: "single operator, progressing and degraded",
			intervals: monitorapi.Intervals{
				makeTestInterval("operator-a", "Progressing", 10),
				makeTestInterval("operator-a", "Progressing", 5),
				makeTestInterval("operator-a", "Degraded", 15),
			},
			expectedMetrics: map[string]*OperatorStateMetrics{
				"operator-a": {
					OperatorName:                    "operator-a",
					ProgressingCount:                2,
					TotalProgressingSeconds:         15,
					MaxIndividualProgressingSeconds: 10,
					DegradedCount:                   1,
					TotalDegradedSeconds:            15,
					MaxIndividualDegradedSeconds:    15,
				},
			},
			expectedRows: []map[string]string{
				{
					"Operator":                     "operator-a",
					"State":                        "Progressing",
					"Count":                        "2",
					"TotalSeconds":                 "15.000000",
					"MaxIndividualDurationSeconds": "10.000000",
				},
				{
					"Operator":                     "operator-a",
					"State":                        "Degraded",
					"Count":                        "1",
					"TotalSeconds":                 "15.000000",
					"MaxIndividualDurationSeconds": "15.000000",
				},
			},
		},
		{
			name: "multiple operators",
			intervals: monitorapi.Intervals{
				makeTestInterval("operator-a", "Progressing", 10),
				makeTestInterval("operator-b", "Degraded", 20),
			},
			expectedMetrics: map[string]*OperatorStateMetrics{
				"operator-a": {
					OperatorName:                    "operator-a",
					ProgressingCount:                1,
					TotalProgressingSeconds:         10,
					MaxIndividualProgressingSeconds: 10,
				},
				"operator-b": {
					OperatorName:                 "operator-b",
					DegradedCount:                1,
					TotalDegradedSeconds:         20,
					MaxIndividualDegradedSeconds: 20,
				},
			},
			expectedRows: []map[string]string{
				{
					"Operator":                     "operator-a",
					"State":                        "Progressing",
					"Count":                        "1",
					"TotalSeconds":                 "10.000000",
					"MaxIndividualDurationSeconds": "10.000000",
				},
				{
					"Operator":                     "operator-b",
					"State":                        "Degraded",
					"Count":                        "1",
					"TotalSeconds":                 "20.000000",
					"MaxIndividualDurationSeconds": "20.000000",
				},
			},
		},
		{
			name:            "no relevant intervals",
			intervals:       monitorapi.Intervals{},
			expectedMetrics: map[string]*OperatorStateMetrics{},
			expectedRows:    []map[string]string{},
		},
		{
			name: "operator with only degraded state",
			intervals: monitorapi.Intervals{
				makeTestInterval("operator-c", "Degraded", 30),
			},
			expectedMetrics: map[string]*OperatorStateMetrics{
				"operator-c": {
					OperatorName:                 "operator-c",
					DegradedCount:                1,
					TotalDegradedSeconds:         30,
					MaxIndividualDegradedSeconds: 30,
				},
			},
			expectedRows: []map[string]string{
				{
					"Operator":                     "operator-c",
					"State":                        "Degraded",
					"Count":                        "1",
					"TotalSeconds":                 "30.000000",
					"MaxIndividualDurationSeconds": "30.000000",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Test calculateOperatorStateMetrics
			metrics := calculateOperatorStateMetrics(tc.intervals)
			require.Equal(t, len(tc.expectedMetrics), len(metrics), "number of operators should match")
			for op, expected := range tc.expectedMetrics {
				actual, ok := metrics[op]
				require.True(t, ok, "operator %s not found in metrics", op)
				assert.Equal(t, expected.OperatorName, actual.OperatorName, "OperatorName should match")
				assert.Equal(t, expected.ProgressingCount, actual.ProgressingCount, "ProgressingCount should match")
				assert.InDelta(t, expected.TotalProgressingSeconds, actual.TotalProgressingSeconds, 0.001, "TotalProgressingSeconds should match")
				assert.InDelta(t, expected.MaxIndividualProgressingSeconds, actual.MaxIndividualProgressingSeconds, 0.001, "MaxIndividualProgressingSeconds should match")
				assert.Equal(t, expected.DegradedCount, actual.DegradedCount, "DegradedCount should match")
				assert.InDelta(t, expected.TotalDegradedSeconds, actual.TotalDegradedSeconds, 0.001, "TotalDegradedSeconds should match")
				assert.InDelta(t, expected.MaxIndividualDegradedSeconds, actual.MaxIndividualDegradedSeconds, 0.001, "MaxIndividualDegradedSeconds should match")
			}

			// Test generateRowsFromMetrics
			rows := generateRowsFromMetrics(metrics)
			assert.ElementsMatch(t, tc.expectedRows, rows, "generated rows should match expected rows")
		})
	}
}

// Helper function to create intervals for testing
func makeTestInterval(operatorName, condition string, durationSeconds float64) monitorapi.Interval {
	from := time.Unix(1, 0)
	to := from.Add(time.Duration(durationSeconds * float64(time.Second)))
	return monitorapi.Interval{
		Source: monitorapi.SourceOperatorState,
		Condition: monitorapi.Condition{
			Locator: monitorapi.Locator{
				Type: monitorapi.LocatorTypeClusterOperator,
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorClusterOperatorKey: operatorName,
				},
			},
			Message: monitorapi.Message{
				Annotations: map[monitorapi.AnnotationKey]string{
					monitorapi.AnnotationCondition: condition,
				},
			},
		},
		From: from,
		To:   to,
	}
}

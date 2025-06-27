package highcputestanalyzer

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/stretchr/testify/assert"
)

func TestFindE2EIntervalsOverlappingHighCPU(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		name      string
		intervals monitorapi.Intervals
		expected  []map[string]string
	}{
		{
			name:      "no intervals",
			intervals: monitorapi.Intervals{},
			expected:  []map[string]string{},
		},
		{
			name: "no alert intervals",
			intervals: monitorapi.Intervals{
				{
					Source: monitorapi.SourceE2ETest,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorE2ETestKey: "test1",
							},
						},
						Message: monitorapi.Message{
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationStatus: "Passed",
							},
						},
					},
					From: now,
					To:   now.Add(10 * time.Minute),
				},
			},
			expected: []map[string]string{},
		},
		{
			name: "no e2e test intervals",
			intervals: monitorapi.Intervals{
				{
					Source: monitorapi.SourceAlert,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								"alert": "ExtremelyHighIndividualControlPlaneCPU",
							},
						},
					},
					From: now,
					To:   now.Add(10 * time.Minute),
				},
			},
			expected: []map[string]string{},
		},
		{
			name: "e2e test overlaps with alert",
			intervals: monitorapi.Intervals{
				{
					Source: monitorapi.SourceAlert,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								"alert": "ExtremelyHighIndividualControlPlaneCPU",
							},
						},
					},
					From: now,
					To:   now.Add(10 * time.Minute),
				},
				{
					Source: monitorapi.SourceE2ETest,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorE2ETestKey: "test1",
							},
						},
						Message: monitorapi.Message{
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationStatus: "Passed",
							},
						},
					},
					From: now.Add(5 * time.Minute),
					To:   now.Add(15 * time.Minute),
				},
			},
			expected: []map[string]string{
				{
					"TestName": "test1",
					"Success":  "1",
				},
			},
		},
		{
			name: "e2e test contained within alert",
			intervals: monitorapi.Intervals{
				{
					Source: monitorapi.SourceAlert,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								"alert": "HighOverallControlPlaneCPU",
							},
						},
					},
					From: now,
					To:   now.Add(20 * time.Minute),
				},
				{
					Source: monitorapi.SourceE2ETest,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorE2ETestKey: "test2",
							},
						},
						Message: monitorapi.Message{
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationStatus: "Failed",
							},
						},
					},
					From: now.Add(5 * time.Minute),
					To:   now.Add(15 * time.Minute),
				},
			},
			expected: []map[string]string{
				{
					"TestName": "test2",
					"Success":  "0",
				},
			},
		},
		{
			name: "alert contained within e2e test",
			intervals: monitorapi.Intervals{
				{
					Source: monitorapi.SourceAlert,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								"alert": "ExtremelyHighIndividualControlPlaneCPU",
							},
						},
					},
					From: now.Add(5 * time.Minute),
					To:   now.Add(15 * time.Minute),
				},
				{
					Source: monitorapi.SourceE2ETest,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorE2ETestKey: "test3",
							},
						},
						Message: monitorapi.Message{
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationStatus: "Passed",
							},
						},
					},
					From: now,
					To:   now.Add(20 * time.Minute),
				},
			},
			expected: []map[string]string{
				{
					"TestName": "test3",
					"Success":  "1",
				},
			},
		},
		{
			name: "e2e test touches alert start",
			intervals: monitorapi.Intervals{
				{
					Source: monitorapi.SourceAlert,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								"alert": "ExtremelyHighIndividualControlPlaneCPU",
							},
						},
					},
					From: now.Add(10 * time.Minute),
					To:   now.Add(20 * time.Minute),
				},
				{
					Source: monitorapi.SourceE2ETest,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorE2ETestKey: "test4",
							},
						},
						Message: monitorapi.Message{
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationStatus: "Passed",
							},
						},
					},
					From: now,
					To:   now.Add(10 * time.Minute),
				},
			},
			expected: []map[string]string{
				{
					"TestName": "test4",
					"Success":  "1",
				},
			},
		},
		{
			name: "e2e test touches alert end",
			intervals: monitorapi.Intervals{
				{
					Source: monitorapi.SourceAlert,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								"alert": "HighOverallControlPlaneCPU",
							},
						},
					},
					From: now,
					To:   now.Add(10 * time.Minute),
				},
				{
					Source: monitorapi.SourceE2ETest,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorE2ETestKey: "test5",
							},
						},
						Message: monitorapi.Message{
							Annotations: map[monitorapi.AnnotationKey]string{},
						},
					},
					From: now.Add(10 * time.Minute),
					To:   now.Add(20 * time.Minute),
				},
			},
			expected: []map[string]string{
				{
					"TestName": "test5",
					"Success":  "0",
				},
			},
		},
		{
			name: "e2e test with zero end time",
			intervals: monitorapi.Intervals{
				{
					Source: monitorapi.SourceAlert,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								"alert": "ExtremelyHighIndividualControlPlaneCPU",
							},
						},
					},
					From: now,
					To:   now.Add(10 * time.Minute),
				},
				{
					Source: monitorapi.SourceE2ETest,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorE2ETestKey: "test6",
							},
						},
						Message: monitorapi.Message{
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationStatus: "Passed",
							},
						},
					},
					From: now.Add(5 * time.Minute),
					To:   time.Time{}, // Zero time
				},
			},
			expected: []map[string]string{
				{
					"TestName": "test6",
					"Success":  "1",
				},
			},
		},
		{
			name: "alert with zero end time",
			intervals: monitorapi.Intervals{
				{
					Source: monitorapi.SourceAlert,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								"alert": "HighOverallControlPlaneCPU",
							},
						},
					},
					From: now,
					To:   time.Time{}, // Zero time
				},
				{
					Source: monitorapi.SourceE2ETest,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorE2ETestKey: "test7",
							},
						},
						Message: monitorapi.Message{
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationStatus: "Failed",
							},
						},
					},
					From: now.Add(5 * time.Minute),
					To:   now.Add(10 * time.Minute),
				},
			},
			expected: []map[string]string{
				{
					"TestName": "test7",
					"Success":  "0",
				},
			},
		},
		{
			name: "multiple overlapping tests",
			intervals: monitorapi.Intervals{
				{
					Source: monitorapi.SourceAlert,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								"alert": "ExtremelyHighIndividualControlPlaneCPU",
							},
						},
					},
					From: now,
					To:   now.Add(30 * time.Minute),
				},
				{
					Source: monitorapi.SourceE2ETest,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorE2ETestKey: "test8",
							},
						},
						Message: monitorapi.Message{
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationStatus: "Passed",
							},
						},
					},
					From: now.Add(5 * time.Minute),
					To:   now.Add(15 * time.Minute),
				},
				{
					Source: monitorapi.SourceE2ETest,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorE2ETestKey: "test9",
							},
						},
						Message: monitorapi.Message{
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationStatus: "Failed",
							},
						},
					},
					From: now.Add(10 * time.Minute),
					To:   now.Add(20 * time.Minute),
				},
			},
			expected: []map[string]string{
				{
					"TestName": "test8",
					"Success":  "1",
				},
				{
					"TestName": "test9",
					"Success":  "0",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := findE2EIntervalsOverlappingHighCPU(tc.intervals)
			assert.ElementsMatch(t, tc.expected, result)
		})
	}
}

func TestOverlaps(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		name      string
		interval1 monitorapi.Interval
		interval2 monitorapi.Interval
		expected  bool
	}{
		{
			name: "intervals overlap",
			interval1: monitorapi.Interval{
				From: now,
				To:   now.Add(10 * time.Minute),
			},
			interval2: monitorapi.Interval{
				From: now.Add(5 * time.Minute),
				To:   now.Add(15 * time.Minute),
			},
			expected: true,
		},
		{
			name: "interval1 contains interval2",
			interval1: monitorapi.Interval{
				From: now,
				To:   now.Add(20 * time.Minute),
			},
			interval2: monitorapi.Interval{
				From: now.Add(5 * time.Minute),
				To:   now.Add(15 * time.Minute),
			},
			expected: true,
		},
		{
			name: "interval2 contains interval1",
			interval1: monitorapi.Interval{
				From: now.Add(5 * time.Minute),
				To:   now.Add(15 * time.Minute),
			},
			interval2: monitorapi.Interval{
				From: now,
				To:   now.Add(20 * time.Minute),
			},
			expected: true,
		},
		{
			name: "intervals touch at start",
			interval1: monitorapi.Interval{
				From: now,
				To:   now.Add(10 * time.Minute),
			},
			interval2: monitorapi.Interval{
				From: now.Add(10 * time.Minute),
				To:   now.Add(20 * time.Minute),
			},
			expected: true,
		},
		{
			name: "intervals touch at end",
			interval1: monitorapi.Interval{
				From: now.Add(10 * time.Minute),
				To:   now.Add(20 * time.Minute),
			},
			interval2: monitorapi.Interval{
				From: now,
				To:   now.Add(10 * time.Minute),
			},
			expected: true,
		},
		{
			name: "intervals don't overlap",
			interval1: monitorapi.Interval{
				From: now,
				To:   now.Add(10 * time.Minute),
			},
			interval2: monitorapi.Interval{
				From: now.Add(11 * time.Minute),
				To:   now.Add(20 * time.Minute),
			},
			expected: false,
		},
		{
			name: "interval1 has zero end time",
			interval1: monitorapi.Interval{
				From: now,
				To:   time.Time{},
			},
			interval2: monitorapi.Interval{
				From: now.Add(10 * time.Minute),
				To:   now.Add(20 * time.Minute),
			},
			expected: true,
		},
		{
			name: "interval2 has zero end time",
			interval1: monitorapi.Interval{
				From: now,
				To:   now.Add(10 * time.Minute),
			},
			interval2: monitorapi.Interval{
				From: now.Add(5 * time.Minute),
				To:   time.Time{},
			},
			expected: true,
		},
		{
			name: "both intervals have zero end time",
			interval1: monitorapi.Interval{
				From: now,
				To:   time.Time{},
			},
			interval2: monitorapi.Interval{
				From: now.Add(10 * time.Minute),
				To:   time.Time{},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := overlaps(tc.interval1, tc.interval2)
			assert.Equal(t, tc.expected, result)
		})
	}
}

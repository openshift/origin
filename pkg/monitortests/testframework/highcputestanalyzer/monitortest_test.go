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
								"alert": "ExtremelyHighIndividualControlPlaneCPU",
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
			expected: []map[string]string{},
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
			expected: []map[string]string{},
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
								"alert": "ExtremelyHighIndividualControlPlaneCPU",
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
		{
			name: "test overlaps with multiple alerts - should only be reported once",
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
					To:   now.Add(15 * time.Minute),
				},
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
					To:   now.Add(25 * time.Minute),
				},
				{
					Source: monitorapi.SourceE2ETest,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorE2ETestKey: "test10",
							},
						},
						Message: monitorapi.Message{
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationStatus: "Passed",
							},
						},
					},
					From: now.Add(5 * time.Minute),
					To:   now.Add(20 * time.Minute),
				},
			},
			expected: []map[string]string{
				{
					"TestName": "test10",
					"Success":  "1",
				},
			},
		},
		{
			name: "same test runs twice and overlaps high cpu alert each time",
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
					To:   now.Add(15 * time.Minute),
				},
				{
					Source: monitorapi.SourceAlert,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								"alert": "ExtremelyHighIndividualControlPlaneCPU",
							},
						},
					},
					From: now.Add(30 * time.Minute),
					To:   now.Add(45 * time.Minute),
				},
				{
					Source: monitorapi.SourceE2ETest,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorE2ETestKey: "test11",
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
				{
					Source: monitorapi.SourceE2ETest,
					Condition: monitorapi.Condition{
						Locator: monitorapi.Locator{
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorE2ETestKey: "test11",
							},
						},
						Message: monitorapi.Message{
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationStatus: "Passed",
							},
						},
					},
					From: now.Add(35 * time.Minute),
					To:   now.Add(40 * time.Minute),
				},
			},
			expected: []map[string]string{
				{
					"TestName": "test11",
					"Success":  "1",
				},
				{
					"TestName": "test11",
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

package utility

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/stretchr/testify/assert"
)

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
			expected: false,
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
			expected: false,
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
			result := IntervalsOverlap(tc.interval1, tc.interval2)
			assert.Equal(t, tc.expected, result)
		})
	}
}

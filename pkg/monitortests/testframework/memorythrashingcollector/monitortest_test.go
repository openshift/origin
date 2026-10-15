package memorythrashingcollector

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	prometheustypes "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func matrix(instance string, start time.Time, values ...float64) prometheustypes.Matrix {
	samples := make([]prometheustypes.SamplePair, len(values))
	for i, v := range values {
		samples[i] = prometheustypes.SamplePair{
			Timestamp: prometheustypes.TimeFromUnixNano(start.Add(time.Duration(i) * 30 * time.Second).UnixNano()),
			Value:     prometheustypes.SampleValue(v),
		}
	}
	return prometheustypes.Matrix{{
		Metric: prometheustypes.Metric{"instance": prometheustypes.LabelValue(instance)},
		Values: samples,
	}}
}

func TestCreateIntervalsFromMetrics(t *testing.T) {
	start := time.Date(2026, 9, 22, 15, 0, 0, 0, time.UTC)
	nodeInfoMap := map[string]nodeInfo{"master-0": {name: "master-0", nodeRole: "master"}}

	testCases := []struct {
		name           string
		faults         []float64
		pageIn         []float64
		expectedCount  int
		expectedStart  time.Time
		expectedEnd    time.Time
		expectedPeak   string
		expectedPageIn string
	}{
		{
			name:          "healthy node",
			faults:        []float64{0.5, 3, 6.5, 1},
			pageIn:        []float64{0, 1, 2, 0},
			expectedCount: 0,
		},
		{
			name:           "single thrashing period",
			faults:         []float64{1, 120, 1058, 300, 2},
			pageIn:         []float64{0, 119, 99, 121, 1},
			expectedCount:  1,
			expectedStart:  start.Add(30 * time.Second),
			expectedEnd:    start.Add(120 * time.Second),
			expectedPeak:   "1058.0",
			expectedPageIn: "121.0",
		},
		{
			name:          "two separate periods",
			faults:        []float64{80, 1, 1, 200, 400, 1},
			pageIn:        []float64{5, 0, 0, 3, 4, 0},
			expectedCount: 2,
		},
		{
			name:           "period extends to end of window",
			faults:         []float64{1, 60, 70},
			pageIn:         []float64{0, 2, 3},
			expectedCount:  1,
			expectedStart:  start.Add(30 * time.Second),
			expectedEnd:    start.Add(90 * time.Second),
			expectedPeak:   "70.0",
			expectedPageIn: "3.0",
		},
		{
			name:          "exactly at threshold is not thrashing",
			faults:        []float64{50, 50},
			pageIn:        []float64{0, 0},
			expectedCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			collector := &memoryThrashingCollector{majorFaultsThreshold: 50}
			intervals := collector.createIntervalsFromMetrics(logrus.WithField("test", tc.name),
				matrix("master-0", start, tc.faults...), matrix("master-0", start, tc.pageIn...), nodeInfoMap)

			require.Len(t, intervals, tc.expectedCount)
			require.Len(t, collector.intervals, tc.expectedCount, "intervals recorded for the autodl file")
			for _, interval := range intervals {
				assert.Equal(t, monitorapi.SourceMemoryMonitor, interval.Source)
				assert.Equal(t, monitorapi.IntervalReason("MemoryThrashing"), interval.Message.Reason)
				assert.Equal(t, "master-0", interval.Locator.Keys[monitorapi.LocatorNodeKey])
				assert.Equal(t, "50", interval.Message.Annotations["major_faults_threshold"])
			}
			if tc.expectedPeak != "" {
				assert.WithinDuration(t, tc.expectedStart, intervals[0].From, 0)
				assert.WithinDuration(t, tc.expectedEnd, intervals[0].To, 0)
				assert.Equal(t, tc.expectedPeak, intervals[0].Message.Annotations["peak_major_faults_per_second"])
				assert.Equal(t, tc.expectedPageIn, intervals[0].Message.Annotations["peak_page_in_mb_per_second"])
				assert.Equal(t, "master", collector.intervals[0].nodeRole)
			}
		})
	}
}

func TestCreateIntervalsSplitsOnMissingSamples(t *testing.T) {
	start := time.Date(2026, 9, 22, 15, 0, 0, 0, time.UTC)
	faults := matrix("master-0", start, 100, 200, 300, 400)
	// Drop the third sample, as Prometheus does for a step with no value.
	faults[0].Values = append(faults[0].Values[:2], faults[0].Values[3])

	collector := &memoryThrashingCollector{majorFaultsThreshold: 50}
	intervals := collector.createIntervalsFromMetrics(logrus.WithField("test", "gap"), faults, prometheustypes.Matrix{}, map[string]nodeInfo{})

	require.Len(t, intervals, 2)
	assert.WithinDuration(t, start, intervals[0].From, 0)
	assert.WithinDuration(t, start.Add(60*time.Second), intervals[0].To, 0)
	assert.WithinDuration(t, start.Add(90*time.Second), intervals[1].From, 0)
	assert.WithinDuration(t, start.Add(120*time.Second), intervals[1].To, 0)
}

func TestCreateIntervalsUnknownNodeFallsBackToInstance(t *testing.T) {
	start := time.Date(2026, 9, 22, 15, 0, 0, 0, time.UTC)
	collector := &memoryThrashingCollector{majorFaultsThreshold: 50}
	intervals := collector.createIntervalsFromMetrics(logrus.WithField("test", "fallback"),
		matrix("10.0.0.5:9100", start, 500), prometheustypes.Matrix{}, map[string]nodeInfo{})

	require.Len(t, intervals, 1)
	assert.Equal(t, 30*time.Second, intervals[0].To.Sub(intervals[0].From), "a single sample counts for one query step")
	assert.Equal(t, "10.0.0.5:9100", intervals[0].Locator.Keys[monitorapi.LocatorNodeKey])
	assert.Equal(t, "0.0", intervals[0].Message.Annotations["peak_page_in_mb_per_second"], "missing page-in data is recorded as zero")
}

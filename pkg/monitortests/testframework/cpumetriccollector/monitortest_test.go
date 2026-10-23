package cpumetriccollector

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	prometheustypes "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateIntervalsFromCPUMetrics(t *testing.T) {
	logger := logrus.WithField("test", "highcpu")

	testCases := []struct {
		name              string
		instance          string
		nodeName          string
		nodeRole          string
		threshold         float64
		values            []float64
		timestamps        []time.Time
		expectedCount     int
		expectedPeakUsage string
	}{
		{
			name:          "no high CPU usage",
			instance:      "192.168.1.10:9100",
			nodeName:      "test-node-1",
			nodeRole:      "worker",
			threshold:     95.0,
			values:        []float64{80.0, 85.0, 70.0, 90.0},
			timestamps:    createTimestamps("2024-01-01T10:00:00Z", 4, 30*time.Second),
			expectedCount: 0,
		},
		{
			name:              "single continuous high CPU period",
			instance:          "192.168.1.10:9100",
			nodeName:          "test-node-1",
			nodeRole:          "worker",
			threshold:         95.0,
			values:            []float64{90.0, 96.0, 98.0, 97.5, 89.0},
			timestamps:        createTimestamps("2024-01-01T10:00:00Z", 5, 30*time.Second),
			expectedCount:     1,
			expectedPeakUsage: "98.00",
		},
		{
			name:              "multiple separate high CPU periods",
			instance:          "192.168.1.10:9100",
			nodeName:          "test-node-1",
			nodeRole:          "master",
			threshold:         95.0,
			values:            []float64{96.0, 97.0, 80.0, 85.0, 98.5, 99.0, 70.0},
			timestamps:        createTimestamps("2024-01-01T10:00:00Z", 7, 30*time.Second),
			expectedCount:     2,
			expectedPeakUsage: "99.00", // Peak from the second period
		},
		{
			name:              "high CPU period at end of monitoring window",
			instance:          "192.168.1.10:9100",
			nodeName:          "test-node-1",
			nodeRole:          "worker",
			threshold:         95.0,
			values:            []float64{80.0, 85.0, 96.0, 98.0},
			timestamps:        createTimestamps("2024-01-01T10:00:00Z", 4, 30*time.Second),
			expectedCount:     1,
			expectedPeakUsage: "98.00",
		},
		{
			name:              "threshold exactly at limit",
			instance:          "192.168.1.10:9100",
			nodeName:          "test-node-1",
			nodeRole:          "worker",
			threshold:         95.0,
			values:            []float64{94.9, 95.0, 95.1, 94.9},
			timestamps:        createTimestamps("2024-01-01T10:00:00Z", 4, 30*time.Second),
			expectedCount:     1,
			expectedPeakUsage: "95.10",
		},
		{
			name:              "different threshold value",
			instance:          "192.168.1.10:9100",
			nodeName:          "test-node-1",
			nodeRole:          "worker",
			threshold:         90.0,
			values:            []float64{85.0, 91.0, 92.0, 88.0, 93.0, 85.0},
			timestamps:        createTimestamps("2024-01-01T10:00:00Z", 6, 30*time.Second),
			expectedCount:     2,
			expectedPeakUsage: "93.00",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			collector := &cpuMetricCollector{
				highCPUThreshold: tc.threshold,
			}

			// Create mock Prometheus matrix data
			samples := make([]prometheustypes.SamplePair, len(tc.values))
			for i, value := range tc.values {
				samples[i] = prometheustypes.SamplePair{
					Timestamp: prometheustypes.Time(tc.timestamps[i].Unix() * 1000), // Convert to milliseconds
					Value:     prometheustypes.SampleValue(value),
				}
			}

			metric := prometheustypes.Metric{
				"instance": prometheustypes.LabelValue(tc.instance),
			}

			sampleStream := &prometheustypes.SampleStream{
				Metric: metric,
				Values: samples,
			}

			matrix := prometheustypes.Matrix{sampleStream}

			// Create node info map for testing
			nodeInfoMap := map[string]nodeInfo{
				tc.instance: {
					name:     tc.nodeName,
					nodeRole: tc.nodeRole,
				},
			}

			// Test the function
			intervals, err := collector.createIntervalsFromCPUMetrics(logger, matrix, nodeInfoMap)
			require.NoError(t, err)

			// Assert the expected number of intervals
			assert.Equal(t, tc.expectedCount, len(intervals), "Expected %d intervals, got %d", tc.expectedCount, len(intervals))

			// If we expect intervals, verify their properties
			if tc.expectedCount > 0 {
				// Check that all intervals have the correct source and reason
				for _, interval := range intervals {
					assert.Equal(t, monitorapi.SourceCPUMonitor, interval.Source)
					assert.Equal(t, monitorapi.IntervalReason("HighCPUUsage"), interval.Message.Reason)
					assert.Equal(t, tc.nodeName, interval.Locator.Keys[monitorapi.LocatorNodeKey])
					assert.Contains(t, interval.Message.HumanMessage, "CPU usage above")

					// Check threshold annotation
					thresholdAnnotation := interval.Message.Annotations["cpu_threshold"]
					expectedThreshold := fmt.Sprintf("%.1f", tc.threshold)
					assert.Equal(t, expectedThreshold, thresholdAnnotation)

					// Check node role in locator (not in message annotations)
					nodeRoleLocator := interval.Locator.Keys[monitorapi.LocatorNodeRoleKey]
					assert.Equal(t, tc.nodeRole, nodeRoleLocator)
				}

				// For tests that specify expected peak usage, check the last interval
				if tc.expectedPeakUsage != "" {
					lastInterval := intervals[len(intervals)-1]
					peakUsage := lastInterval.Message.Annotations["peak_cpu_usage"]
					assert.Equal(t, tc.expectedPeakUsage, peakUsage)
				}
			}
		})
	}
}

func TestCreateIntervalsFromCPUMetrics_EmptyData(t *testing.T) {
	logger := logrus.WithField("test", "highcpu")
	collector := &cpuMetricCollector{
		highCPUThreshold: 95.0,
	}

	// Test with empty matrix
	emptyMatrix := prometheustypes.Matrix{}
	nodeInfoMap := map[string]nodeInfo{}
	intervals, err := collector.createIntervalsFromCPUMetrics(logger, emptyMatrix, nodeInfoMap)
	require.NoError(t, err)
	assert.Empty(t, intervals)
}

func TestCreateIntervalsFromCPUMetrics_InvalidPrometheusType(t *testing.T) {
	logger := logrus.WithField("test", "highcpu")
	collector := &cpuMetricCollector{
		highCPUThreshold: 95.0,
	}

	// Test with non-matrix type (scalar)
	scalar := prometheustypes.Scalar{
		Value:     prometheustypes.SampleValue(96.0),
		Timestamp: prometheustypes.Time(time.Now().Unix() * 1000),
	}

	nodeInfoMap := map[string]nodeInfo{}
	intervals, err := collector.createIntervalsFromCPUMetrics(logger, &scalar, nodeInfoMap)
	require.NoError(t, err)
	assert.Empty(t, intervals, "Should return empty intervals for non-matrix types")
}

func TestCreateCPUInterval(t *testing.T) {
	collector := &cpuMetricCollector{
		highCPUThreshold: 95.0,
	}

	start := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 1, 10, 5, 0, 0, time.UTC)
	peakUsage := 98.5
	nodeName := "test-node-1"
	nodeRole := "worker"

	// Create locator for the node with role
	locator := monitorapi.NewLocator().NodeFromNameWithRole(nodeName, nodeRole)

	interval := collector.createCPUInterval(locator, nodeName, nodeRole, start, end, peakUsage)

	// Verify the interval properties
	assert.Equal(t, start, interval.From)
	assert.Equal(t, end, interval.To)
	assert.Equal(t, monitorapi.SourceCPUMonitor, interval.Source)
	assert.Equal(t, monitorapi.IntervalReason("HighCPUUsage"), interval.Message.Reason)
	assert.Equal(t, nodeName, interval.Locator.Keys[monitorapi.LocatorNodeKey])
	assert.Contains(t, interval.Message.HumanMessage, "CPU usage above 95.0% threshold on node test-node-1")
	assert.Equal(t, "98.50", interval.Message.Annotations["peak_cpu_usage"])
	assert.Equal(t, "95.0", interval.Message.Annotations["cpu_threshold"])
	assert.Equal(t, nodeRole, interval.Locator.Keys[monitorapi.LocatorNodeRoleKey])
}

func TestCPUMetricCollector_DefaultThreshold(t *testing.T) {
	collector := NewCPUMetricCollector().(*cpuMetricCollector)
	assert.Equal(t, 95.0, collector.highCPUThreshold, "Default threshold should be 95.0")
}

// Helper function to create timestamps for testing
func createTimestamps(startTime string, count int, interval time.Duration) []time.Time {
	start, _ := time.Parse(time.RFC3339, startTime)
	timestamps := make([]time.Time, count)
	for i := 0; i < count; i++ {
		timestamps[i] = start.Add(time.Duration(i) * interval)
	}
	return timestamps
}

func runQueueMatrix(instance string, start time.Time, values ...float64) prometheustypes.Matrix {
	samples := make([]prometheustypes.SamplePair, len(values))
	for i, v := range values {
		samples[i] = prometheustypes.SamplePair{
			Timestamp: prometheustypes.TimeFromUnixNano(start.Add(time.Duration(i) * saturationQueryStep).UnixNano()),
			Value:     prometheustypes.SampleValue(v),
		}
	}
	return prometheustypes.Matrix{{
		Metric: prometheustypes.Metric{"instance": prometheustypes.LabelValue(instance)},
		Values: samples,
	}}
}

func TestCreateSaturationIntervals(t *testing.T) {
	logger := logrus.WithField("test", "saturation")
	start := time.Date(2026, 10, 19, 13, 57, 0, 0, time.UTC)
	nodeInfoMap := map[string]nodeInfo{"master-0": {name: "master-0", nodeRole: "master"}}

	testCases := []struct {
		name          string
		values        []float64
		expectedCount int
		expectedStart time.Time
		expectedEnd   time.Time
		expectedPeak  string
	}{
		{
			name:          "healthy node",
			values:        []float64{0.2, 0.8, 1.5, 0.3},
			expectedCount: 0,
		},
		{
			name:          "single saturated period",
			values:        []float64{1, 5.2, 6.5, 4.4, 2},
			expectedCount: 1,
			expectedStart: start.Add(30 * time.Second),
			expectedEnd:   start.Add(120 * time.Second),
			expectedPeak:  "6.50",
		},
		{
			name:          "two separate periods",
			values:        []float64{4.5, 5, 1, 1, 8.6, 7.3, 1},
			expectedCount: 2,
		},
		{
			name:          "single-sample blips are dropped",
			values:        []float64{1, 11.4, 1, 5.3, 1, 4.1},
			expectedCount: 0,
		},
		{
			name:          "period extends to end of window",
			values:        []float64{1, 4.1, 5},
			expectedCount: 1,
			expectedStart: start.Add(30 * time.Second),
			expectedEnd:   start.Add(90 * time.Second),
			expectedPeak:  "5.00",
		},
		{
			name:          "exactly at threshold is not saturated",
			values:        []float64{4, 4},
			expectedCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			collector := &cpuMetricCollector{runQueueThreshold: 4}
			intervals := collector.createSaturationIntervals(logger, runQueueMatrix("master-0", start, tc.values...), nodeInfoMap)

			require.Len(t, intervals, tc.expectedCount)
			for _, interval := range intervals {
				assert.Equal(t, monitorapi.SourceCPUSaturation, interval.Source)
				assert.Equal(t, monitorapi.IntervalReason("CPURunQueueSaturation"), interval.Message.Reason)
				assert.Equal(t, "master-0", interval.Locator.Keys[monitorapi.LocatorNodeKey])
				assert.Equal(t, "master", interval.Locator.Keys[monitorapi.LocatorNodeRoleKey])
				assert.Equal(t, "4.0", interval.Message.Annotations["run_queue_threshold"])
			}
			if tc.expectedPeak != "" {
				assert.WithinDuration(t, tc.expectedStart, intervals[0].From, 0)
				assert.WithinDuration(t, tc.expectedEnd, intervals[0].To, 0)
				assert.Equal(t, tc.expectedPeak, intervals[0].Message.Annotations["peak_run_queue_wait_per_core"])
			}
		})
	}
}

func TestCreateSaturationIntervals_SplitsOnMissingSamples(t *testing.T) {
	logger := logrus.WithField("test", "saturation")
	start := time.Date(2026, 10, 19, 13, 57, 0, 0, time.UTC)
	m := runQueueMatrix("master-0", start, 5, 6, 7, 8, 9)
	// Drop the third sample, as Prometheus does for a step with no value.
	m[0].Values = append(m[0].Values[:2], m[0].Values[3:]...)

	collector := &cpuMetricCollector{runQueueThreshold: 4}
	intervals := collector.createSaturationIntervals(logger, m, map[string]nodeInfo{})

	require.Len(t, intervals, 2)
	assert.WithinDuration(t, start.Add(60*time.Second), intervals[0].To, 0)
	assert.WithinDuration(t, start.Add(90*time.Second), intervals[1].From, 0)
	assert.WithinDuration(t, start.Add(150*time.Second), intervals[1].To, 0)
	assert.Equal(t, "master-0", intervals[0].Locator.Keys[monitorapi.LocatorNodeKey], "unknown nodes fall back to the instance label")
}

func TestCPUMetricCollector_DefaultRunQueueThreshold(t *testing.T) {
	collector := NewCPUMetricCollector().(*cpuMetricCollector)
	assert.Equal(t, 4.0, collector.runQueueThreshold)
}

func TestEvaluateTestsReportsNoResults(t *testing.T) {
	logger := logrus.WithField("test", "saturation")
	start := time.Date(2026, 10, 19, 13, 57, 0, 0, time.UTC)
	collector := &cpuMetricCollector{highCPUThreshold: 95, runQueueThreshold: 4}
	saturated := collector.createSaturationIntervals(logger, runQueueMatrix("master-0", start, 6.5, 68), map[string]nodeInfo{})
	require.NotEmpty(t, saturated)

	junits, err := collector.EvaluateTestsFromConstructedIntervals(context.Background(), saturated)
	require.NoError(t, err)
	assert.Empty(t, junits, "CPU saturation is recorded as intervals only")
}

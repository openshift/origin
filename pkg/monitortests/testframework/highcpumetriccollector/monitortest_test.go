package highcpumetriccollector

import (
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
		threshold         float64
		values            []float64
		timestamps        []time.Time
		expectedCount     int
		expectedPeakUsage string
	}{
		{
			name:          "no high CPU usage",
			instance:      "test-node-1",
			threshold:     95.0,
			values:        []float64{80.0, 85.0, 70.0, 90.0},
			timestamps:    createTimestamps("2024-01-01T10:00:00Z", 4, 30*time.Second),
			expectedCount: 0,
		},
		{
			name:              "single continuous high CPU period",
			instance:          "test-node-1",
			threshold:         95.0,
			values:            []float64{90.0, 96.0, 98.0, 97.5, 89.0},
			timestamps:        createTimestamps("2024-01-01T10:00:00Z", 5, 30*time.Second),
			expectedCount:     1,
			expectedPeakUsage: "98.00",
		},
		{
			name:              "multiple separate high CPU periods",
			instance:          "test-node-1",
			threshold:         95.0,
			values:            []float64{96.0, 97.0, 80.0, 85.0, 98.5, 99.0, 70.0},
			timestamps:        createTimestamps("2024-01-01T10:00:00Z", 7, 30*time.Second),
			expectedCount:     2,
			expectedPeakUsage: "99.00", // Peak from the second period
		},
		{
			name:              "high CPU period at end of monitoring window",
			instance:          "test-node-1",
			threshold:         95.0,
			values:            []float64{80.0, 85.0, 96.0, 98.0},
			timestamps:        createTimestamps("2024-01-01T10:00:00Z", 4, 30*time.Second),
			expectedCount:     1,
			expectedPeakUsage: "98.00",
		},
		{
			name:              "threshold exactly at limit",
			instance:          "test-node-1",
			threshold:         95.0,
			values:            []float64{94.9, 95.0, 95.1, 94.9},
			timestamps:        createTimestamps("2024-01-01T10:00:00Z", 4, 30*time.Second),
			expectedCount:     1,
			expectedPeakUsage: "95.10",
		},
		{
			name:              "different threshold value",
			instance:          "test-node-1",
			threshold:         90.0,
			values:            []float64{85.0, 91.0, 92.0, 88.0, 93.0, 85.0},
			timestamps:        createTimestamps("2024-01-01T10:00:00Z", 6, 30*time.Second),
			expectedCount:     2,
			expectedPeakUsage: "93.00",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			collector := &highCPUMetricCollector{
				cpuThreshold: tc.threshold,
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

			// Test the function
			intervals, err := collector.createIntervalsFromCPUMetrics(logger, matrix)
			require.NoError(t, err)

			// Assert the expected number of intervals
			assert.Equal(t, tc.expectedCount, len(intervals), "Expected %d intervals, got %d", tc.expectedCount, len(intervals))

			// If we expect intervals, verify their properties
			if tc.expectedCount > 0 {
				// Check that all intervals have the correct source and reason
				for _, interval := range intervals {
					assert.Equal(t, monitorapi.SourceCPUMonitor, interval.Source)
					assert.Equal(t, monitorapi.IntervalReason("HighCPUUsage"), interval.Message.Reason)
					assert.Equal(t, tc.instance, interval.Locator.Keys[monitorapi.LocatorNodeKey])
					assert.Contains(t, interval.Message.HumanMessage, "CPU usage above")

					// Check threshold annotation
					thresholdAnnotation := interval.Message.Annotations["cpu_threshold"]
					expectedThreshold := fmt.Sprintf("%.1f", tc.threshold)
					assert.Equal(t, expectedThreshold, thresholdAnnotation)
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
	collector := &highCPUMetricCollector{
		cpuThreshold: 95.0,
	}

	// Test with empty matrix
	emptyMatrix := prometheustypes.Matrix{}
	intervals, err := collector.createIntervalsFromCPUMetrics(logger, emptyMatrix)
	require.NoError(t, err)
	assert.Empty(t, intervals)
}

func TestCreateIntervalsFromCPUMetrics_InvalidPrometheusType(t *testing.T) {
	logger := logrus.WithField("test", "highcpu")
	collector := &highCPUMetricCollector{
		cpuThreshold: 95.0,
	}

	// Test with non-matrix type (scalar)
	scalar := prometheustypes.Scalar{
		Value:     prometheustypes.SampleValue(96.0),
		Timestamp: prometheustypes.Time(time.Now().Unix() * 1000),
	}

	intervals, err := collector.createIntervalsFromCPUMetrics(logger, &scalar)
	require.NoError(t, err)
	assert.Empty(t, intervals, "Should return empty intervals for non-matrix types")
}

func TestCreateCPUInterval(t *testing.T) {
	collector := &highCPUMetricCollector{
		cpuThreshold: 95.0,
	}

	start := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 1, 10, 5, 0, 0, time.UTC)
	peakUsage := 98.5
	instance := "test-node-1"

	// Create locator for the node
	locator := monitorapi.NewLocator().NodeFromName(instance)

	interval := collector.createCPUInterval(locator, instance, start, end, peakUsage)

	// Verify the interval properties
	assert.Equal(t, start, interval.From)
	assert.Equal(t, end, interval.To)
	assert.Equal(t, monitorapi.SourceCPUMonitor, interval.Source)
	assert.Equal(t, monitorapi.IntervalReason("HighCPUUsage"), interval.Message.Reason)
	assert.Equal(t, instance, interval.Locator.Keys[monitorapi.LocatorNodeKey])
	assert.Contains(t, interval.Message.HumanMessage, "CPU usage above 95.0% threshold on instance test-node-1")
	assert.Equal(t, "98.50", interval.Message.Annotations["peak_cpu_usage"])
	assert.Equal(t, "95.0", interval.Message.Annotations["cpu_threshold"])
}

func TestHighCPUMetricCollector_DefaultThreshold(t *testing.T) {
	collector := NewHighCPUMetricCollector().(*highCPUMetricCollector)
	assert.Equal(t, 95.0, collector.cpuThreshold, "Default threshold should be 95.0")
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

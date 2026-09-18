package legacytestframeworkmonitortests

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/historicaldata"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoNewAlertsFiringBackstop(t *testing.T) {

	awsJob := platformidentification.JobType{
		Release:      "4.15",
		FromRelease:  "4.14",
		Platform:     "aws",
		Architecture: "amd64",
		Network:      "ovn",
		Topology:     "ha",
	}
	fakeAlertKey := historicaldata.AlertDataKey{
		AlertName:      "FakeAlert",
		AlertNamespace: "fakens",
		AlertLevel:     "Warning",
		JobType:        awsJob,
	}
	firstObvRecent := time.Now().Add(-4 * 24 * time.Hour)   // 4 days ago
	firstObvAncient := time.Now().Add(-60 * 24 * time.Hour) // 60 days ago

	interval := monitorapi.Interval{
		Condition: monitorapi.Condition{
			Level: monitorapi.Warning,
			Locator: monitorapi.Locator{
				Type: monitorapi.LocatorTypeAlert,
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorAlertKey:     "FakeAlert",
					monitorapi.LocatorNamespaceKey: "fakens",
				},
			},
			Message: monitorapi.Message{
				HumanMessage: "jibberish",
				Annotations: map[monitorapi.AnnotationKey]string{
					monitorapi.AnnotationAlertState: "firing",
					monitorapi.AnnotationSeverity:   "warning",
				},
			},
		},
		Source:  monitorapi.SourceAlert,
		Display: false,
		From:    time.Now().Add(-5 * time.Hour),
		To:      time.Now().Add(-6 * time.Hour),
	}

	tests := []struct {
		name            string
		historicalData  *historicaldata.AlertBestMatcher
		firingIntervals monitorapi.Intervals
		expectedStatus  []string // "pass", "fail", in the order we expect them to appear, one of each for flakes
	}{
		{
			name: "firing alert first observed recently in few jobs",
			historicalData: historicaldata.NewAlertMatcherWithHistoricalData(
				map[historicaldata.AlertDataKey]historicaldata.AlertStatisticalData{
					fakeAlertKey: {
						AlertDataKey:  fakeAlertKey,
						Name:          "FakeAlert",
						FirstObserved: firstObvRecent,
						LastObserved:  time.Now(),
						JobRuns:       5, // less than 100
					},
				}),
			firingIntervals: monitorapi.Intervals{interval},
			expectedStatus:  []string{"fail"},
		},
		{
			name: "firing alert never seen before",
			historicalData: historicaldata.NewAlertMatcherWithHistoricalData(
				map[historicaldata.AlertDataKey]historicaldata.AlertStatisticalData{}),
			firingIntervals: monitorapi.Intervals{interval},
			expectedStatus:  []string{"fail"},
		},
		{
			name: "firing severity info alert never seen before",
			historicalData: historicaldata.NewAlertMatcherWithHistoricalData(
				map[historicaldata.AlertDataKey]historicaldata.AlertStatisticalData{}),
			firingIntervals: monitorapi.Intervals{monitorapi.Interval{
				Condition: monitorapi.Condition{
					Level: monitorapi.Warning,
					Locator: monitorapi.Locator{
						Type: monitorapi.LocatorTypeAlert,
						Keys: map[monitorapi.LocatorKey]string{
							monitorapi.LocatorAlertKey:     "FakeAlert",
							monitorapi.LocatorNamespaceKey: "fakens",
						},
					},
					Message: monitorapi.Message{
						HumanMessage: "jibberish",
						Annotations: map[monitorapi.AnnotationKey]string{
							monitorapi.AnnotationAlertState: "firing",
							monitorapi.AnnotationSeverity:   "info",
						},
					},
				},
				Source:  monitorapi.SourceAlert,
				Display: false,
				From:    time.Now().Add(-5 * time.Hour),
				To:      time.Now().Add(-6 * time.Hour),
			}},
			expectedStatus: []string{"pass"}, // info severity alerts should not fail this test
		},
		{
			name: "firing alert observed more than two weeks ago",
			historicalData: historicaldata.NewAlertMatcherWithHistoricalData(
				map[historicaldata.AlertDataKey]historicaldata.AlertStatisticalData{
					fakeAlertKey: {
						AlertDataKey:  fakeAlertKey,
						Name:          "FakeAlert",
						FirstObserved: firstObvAncient,
						LastObserved:  time.Now(),
						JobRuns:       5, // less than 100
					},
				}),
			firingIntervals: monitorapi.Intervals{interval},
			expectedStatus:  []string{"pass"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := runNoNewAlertsFiringTest(tt.historicalData, tt.firingIntervals)
			for _, r := range results {
				t.Logf("%s failure output was: %s", r.Name, r.FailureOutput)
			}
			require.Equal(t, len(tt.expectedStatus), len(results))
			for i := range tt.expectedStatus {
				switch tt.expectedStatus[i] {
				case "pass":
					assert.Nil(t, results[i].FailureOutput)
				case "fail":
					assert.NotNil(t, results[i].FailureOutput)
				default:
					t.Logf("invalid test input, should be 'pass' or 'fail': %s", tt.expectedStatus[i])
					t.Fail()
				}
			}
		})
	}

}

func TestFilterExternalTopologyStartupAlertIntervals(t *testing.T) {
	beginning := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	externalJob := &platformidentification.JobType{Topology: "external"}
	haJob := &platformidentification.JobType{Topology: "ha"}

	alertInterval := func(alertName, namespace string, from, to time.Duration) monitorapi.Interval {
		return monitorapi.Interval{
			Condition: monitorapi.Condition{
				Locator: monitorapi.Locator{
					Type: monitorapi.LocatorTypeAlert,
					Keys: map[monitorapi.LocatorKey]string{
						monitorapi.LocatorAlertKey:     alertName,
						monitorapi.LocatorNamespaceKey: namespace,
					},
				},
			},
			From: beginning.Add(from),
			To:   beginning.Add(to),
		}
	}

	tests := []struct {
		name     string
		jobType  *platformidentification.JobType
		start    time.Time
		interval monitorapi.Interval
		expected monitorapi.Intervals
	}{
		{
			name:     "startup interval is ignored for affected namespace",
			jobType:  externalJob,
			start:    beginning,
			interval: alertInterval("KubePodNotReady", "openshift-dns", time.Minute, 3*time.Minute),
			expected: nil,
		},
		{
			name:     "interval crossing grace period is clipped",
			jobType:  externalJob,
			start:    beginning,
			interval: alertInterval("KubePodNotReady", "openshift-insights", time.Minute, 7*time.Minute),
			expected: monitorapi.Intervals{alertInterval("KubePodNotReady", "openshift-insights", 5*time.Minute, 7*time.Minute)},
		},
		{
			name:     "interval after grace period is unchanged",
			jobType:  externalJob,
			start:    beginning,
			interval: alertInterval("KubePodNotReady", "openshift-ingress-canary", 6*time.Minute, 7*time.Minute),
			expected: monitorapi.Intervals{alertInterval("KubePodNotReady", "openshift-ingress-canary", 6*time.Minute, 7*time.Minute)},
		},
		{
			name:     "unrelated namespace is unchanged",
			jobType:  externalJob,
			start:    beginning,
			interval: alertInterval("KubePodNotReady", "openshift-kube-apiserver", time.Minute, 3*time.Minute),
			expected: monitorapi.Intervals{alertInterval("KubePodNotReady", "openshift-kube-apiserver", time.Minute, 3*time.Minute)},
		},
		{
			name:     "unrelated alert is unchanged",
			jobType:  externalJob,
			start:    beginning,
			interval: alertInterval("ClusterOperatorDegraded", "openshift-dns", time.Minute, 3*time.Minute),
			expected: monitorapi.Intervals{alertInterval("ClusterOperatorDegraded", "openshift-dns", time.Minute, 3*time.Minute)},
		},
		{
			name:     "non-external topology is unchanged",
			jobType:  haJob,
			start:    beginning,
			interval: alertInterval("KubePodNotReady", "openshift-dns", time.Minute, 3*time.Minute),
			expected: monitorapi.Intervals{alertInterval("KubePodNotReady", "openshift-dns", time.Minute, 3*time.Minute)},
		},
		{
			name:     "zero collection start is unchanged",
			jobType:  externalJob,
			start:    time.Time{},
			interval: alertInterval("KubePodNotReady", "openshift-dns", time.Minute, 3*time.Minute),
			expected: monitorapi.Intervals{alertInterval("KubePodNotReady", "openshift-dns", time.Minute, 3*time.Minute)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := filterExternalTopologyStartupAlertIntervals(monitorapi.Intervals{tt.interval}, tt.jobType, tt.start)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

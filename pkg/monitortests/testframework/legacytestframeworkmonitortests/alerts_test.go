package legacytestframeworkmonitortests

import (
	"testing"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/alerts"
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
	intervalWithAlertName := func(alertName string) monitorapi.Interval {
		alertInterval := interval
		alertInterval.Locator.Keys = map[monitorapi.LocatorKey]string{
			monitorapi.LocatorAlertKey:     alertName,
			monitorapi.LocatorNamespaceKey: "fakens",
		}
		return alertInterval
	}

	tests := []struct {
		name            string
		historicalData  *historicaldata.AlertBestMatcher
		firingIntervals monitorapi.Intervals
		expectedStatus  []string // "pass", "fail", in the order we expect them to appear, one of each for flakes
	}{
		{
			name: "excluded firing alert never seen before",
			historicalData: historicaldata.NewAlertMatcherWithHistoricalData(
				map[historicaldata.AlertDataKey]historicaldata.AlertStatisticalData{}),
			firingIntervals: monitorapi.Intervals{intervalWithAlertName("VirtPlatformDependencyMissing")},
			expectedStatus:  []string{"pass"},
		},
		{
			name: "similarly named firing alert never seen before",
			historicalData: historicaldata.NewAlertMatcherWithHistoricalData(
				map[historicaldata.AlertDataKey]historicaldata.AlertStatisticalData{}),
			firingIntervals: monitorapi.Intervals{intervalWithAlertName("VirtPlatformDependencyMissingOther")},
			expectedStatus:  []string{"fail"},
		},
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

func TestIsSkippedAlert(test *testing.T) {
	for _, testCase := range []struct {
		alertName string
		skipped   bool
	}{
		{alertName: "VirtPlatformDependencyMissing", skipped: true},
		{alertName: "VirtPlatformDependencyMissingOther"},
		{alertName: "OtherVirtPlatformDependencyMissing"},
		{alertName: "FakeAlert"},
	} {
		test.Run(testCase.alertName, func(test *testing.T) {
			assert.Equal(test, testCase.skipped, isSkippedAlert(testCase.alertName))
		})
	}
}

func TestSkippedAlertBackstop(test *testing.T) {
	for _, suite := range []struct {
		name       string
		allowances AllowedAlertsFunc
	}{
		{name: "conformance", allowances: alerts.AllowedAlertsDuringConformance},
		{name: "upgrade", allowances: alerts.AllowedAlertsDuringUpgrade},
	} {
		test.Run(suite.name, func(test *testing.T) {
			for _, featureSet := range []configv1.FeatureSet{configv1.Default, configv1.TechPreviewNoUpgrade} {
				test.Run(string(featureSet), func(test *testing.T) {
					for _, state := range []string{"pending", "firing"} {
						test.Run(state, func(test *testing.T) {
							for _, testCase := range []struct {
								alertName string
								skipped   bool
							}{
								{alertName: "VirtPlatformDependencyMissing", skipped: true},
								{alertName: "VirtPlatformDependencyMissingOther"},
								{alertName: "FakeAlert"},
							} {
								test.Run(testCase.alertName, func(test *testing.T) {
									interval := monitorapi.Interval{
										Condition: monitorapi.Condition{
											Level: monitorapi.Warning,
											Locator: monitorapi.Locator{
												Type: monitorapi.LocatorTypeAlert,
												Keys: map[monitorapi.LocatorKey]string{monitorapi.LocatorAlertKey: testCase.alertName},
											},
											Message: monitorapi.Message{Annotations: map[monitorapi.AnnotationKey]string{
												monitorapi.AnnotationAlertState: state,
												monitorapi.AnnotationSeverity:   "warning",
											}},
										},
										Source: monitorapi.SourceAlert,
										From:   time.Now().Add(-time.Hour),
										To:     time.Now(),
									}
									var pendingIntervals, firingIntervals monitorapi.Intervals
									if state == "pending" {
										pendingIntervals = monitorapi.Intervals{interval}
									} else {
										firingIntervals = monitorapi.Intervals{interval}
									}
									allowances := func(featureSet configv1.FeatureSet) (alerts.MetricConditions, alerts.MetricConditions, alerts.MetricConditions, alerts.MetricConditions) {
										firingWithBugs, firing, pendingWithBugs, pending := suite.allowances(featureSet)
										pendingWithBugs = append(pendingWithBugs, alerts.MetricCondition{AlertName: testCase.alertName, Text: "pending backstop regression"})
										return firingWithBugs, firing, pendingWithBugs, pending
									}
									results := runBackstopTest(allowances, featureSet, pendingIntervals, firingIntervals, nil)
									if testCase.skipped {
										require.Len(test, results, 1)
										assert.Nil(test, results[0].FailureOutput)
									} else {
										require.Len(test, results, 2)
										require.NotNil(test, results[1].FailureOutput)
										assert.Contains(test, results[1].FailureOutput.Output, testCase.alertName)
									}
								})
							}
						})
					}
				})
			}
		})
	}
}

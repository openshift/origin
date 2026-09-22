package allowedalerts

import (
	"testing"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatchdogInvariantCheck(t *testing.T) {
	stable := monitortestframework.Stable

	watchdogInterval := monitorapi.Interval{
		Condition: monitorapi.Condition{
			Level: monitorapi.Info,
			Locator: monitorapi.Locator{
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorAlertKey:     "Watchdog",
					monitorapi.LocatorNamespaceKey: "openshift-monitoring",
				},
			},
		},
		Source: monitorapi.SourceAlert,
	}

	tests := []struct {
		name      string
		topology  string
		intervals monitorapi.Intervals
		wantPass  bool
	}{
		{
			name:      "HA topology with Watchdog present passes",
			topology:  "ha",
			intervals: monitorapi.Intervals{watchdogInterval},
			wantPass:  true,
		},
		{
			name:      "HA topology without Watchdog fails",
			topology:  "ha",
			intervals: monitorapi.Intervals{},
			wantPass:  false,
		},
		{
			name:      "External topology without Watchdog passes",
			topology:  "external",
			intervals: monitorapi.Intervals{},
			wantPass:  true,
		},
		{
			name:      "External topology with Watchdog present passes",
			topology:  "external",
			intervals: monitorapi.Intervals{watchdogInterval},
			wantPass:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alert := &watchdogAlertTest{
				jobType: &platformidentification.JobType{
					Topology: tt.topology,
				},
				clusterStability: &stable,
			}

			results, err := alert.InvariantCheck(tt.intervals, nil)
			require.NoError(t, err)
			require.NotEmpty(t, results)

			if tt.wantPass {
				assert.Nil(t, results[0].FailureOutput, "expected pass but got failure: %v", results[0].FailureOutput)
			} else {
				assert.NotNil(t, results[0].FailureOutput, "expected failure but got pass")
				assert.Contains(t, results[0].FailureOutput.Output, "Watchdog alert not found")
			}
		})
	}
}

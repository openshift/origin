package onpremhaproxy

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

var testStart = time.Date(2024, time.October, 28, 7, 0, 0, 0, time.UTC)

// at is a helper returning a time relative to the beginning of the test run.
func at(seconds int) time.Time {
	return testStart.Add(time.Duration(seconds) * time.Second)
}

// haproxyDownInterval builds a constructed OnPremHaproxyDetectsDown interval the same way
// ConstructComputedIntervals does.
func haproxyDownInterval(reportingNode, backend string, from, to time.Time) monitorapi.Interval {
	return monitorapi.NewInterval(monitorapi.SourceHaproxyMonitor, monitorapi.Info).
		Locator(monitorapi.Locator{Keys: map[monitorapi.LocatorKey]string{
			monitorapi.LocatorOnPremKubeapiUnreachableFromHaproxyKey: fmt.Sprintf("%s___%s", reportingNode, backend),
		}}).
		Message(monitorapi.NewMessage().Reason(monitorapi.OnPremHaproxyDetectsDown).
			Constructed(monitorapi.ConstructionOwnerOnPremHaproxy).
			HumanMessage(fmt.Sprintf("Kubeapi on %s is detected dead by %s", backend, reportingNode))).
		Display().
		Build(from, to)
}

func TestFindFullAPIOutageWindows(t *testing.T) {
	tests := []struct {
		name      string
		intervals monitorapi.Intervals
		expected  map[string][]apiOutageWindow
	}{
		{
			name:      "no intervals",
			intervals: monitorapi.Intervals{},
			expected:  map[string][]apiOutageWindow{},
		},
		{
			name: "single backend flapping is not an outage",
			intervals: monitorapi.Intervals{
				haproxyDownInterval("master-0", "masters/master-1", at(0), at(60)),
				haproxyDownInterval("master-0", "masters/master-1", at(120), at(180)),
			},
			expected: map[string][]apiOutageWindow{},
		},
		{
			name: "two backends down at the same time is not a full outage",
			intervals: monitorapi.Intervals{
				haproxyDownInterval("master-0", "masters/master-1", at(0), at(60)),
				haproxyDownInterval("master-0", "masters/master-2", at(30), at(90)),
			},
			expected: map[string][]apiOutageWindow{},
		},
		{
			name: "three backends down at the same time",
			intervals: monitorapi.Intervals{
				haproxyDownInterval("master-0", "masters/master-0", at(0), at(50)),
				haproxyDownInterval("master-0", "masters/master-1", at(10), at(60)),
				haproxyDownInterval("master-0", "masters/master-2", at(20), at(40)),
			},
			expected: map[string][]apiOutageWindow{
				"master-0": {{from: at(20), to: at(40)}},
			},
		},
		{
			name: "backends down on different haproxy instances do not add up",
			intervals: monitorapi.Intervals{
				haproxyDownInterval("master-0", "masters/master-0", at(0), at(60)),
				haproxyDownInterval("master-1", "masters/master-1", at(0), at(60)),
				haproxyDownInterval("master-2", "masters/master-2", at(0), at(60)),
			},
			expected: map[string][]apiOutageWindow{},
		},
		{
			name: "two separate full outages",
			intervals: monitorapi.Intervals{
				haproxyDownInterval("master-0", "masters/master-0", at(0), at(60)),
				haproxyDownInterval("master-0", "masters/master-1", at(0), at(60)),
				haproxyDownInterval("master-0", "masters/master-2", at(0), at(60)),
				haproxyDownInterval("master-0", "masters/master-0", at(600), at(630)),
				haproxyDownInterval("master-0", "masters/master-1", at(600), at(630)),
				haproxyDownInterval("master-0", "masters/master-2", at(600), at(630)),
			},
			expected: map[string][]apiOutageWindow{
				"master-0": {
					{from: at(0), to: at(60)},
					{from: at(600), to: at(630)},
				},
			},
		},
		{
			name: "recovery at the same second as another backend goes down is not an overlap",
			intervals: monitorapi.Intervals{
				haproxyDownInterval("master-0", "masters/master-0", at(0), at(20)),
				haproxyDownInterval("master-0", "masters/master-1", at(0), at(20)),
				haproxyDownInterval("master-0", "masters/master-2", at(20), at(40)),
			},
			expected: map[string][]apiOutageWindow{},
		},
		{
			name: "one backend recovering and going down within the outage keeps a single window",
			intervals: monitorapi.Intervals{
				haproxyDownInterval("master-0", "masters/master-0", at(0), at(100)),
				haproxyDownInterval("master-0", "masters/master-1", at(0), at(100)),
				haproxyDownInterval("master-0", "masters/master-2", at(0), at(50)),
				haproxyDownInterval("master-0", "masters/master-2", at(50), at(100)),
			},
			expected: map[string][]apiOutageWindow{
				"master-0": {{from: at(0), to: at(100)}},
			},
		},
		{
			name: "outages tracked separately per haproxy instance",
			intervals: monitorapi.Intervals{
				haproxyDownInterval("master-0", "masters/master-0", at(0), at(60)),
				haproxyDownInterval("master-0", "masters/master-1", at(0), at(60)),
				haproxyDownInterval("master-0", "masters/master-2", at(0), at(60)),
				haproxyDownInterval("master-1", "masters/master-0", at(300), at(360)),
				haproxyDownInterval("master-1", "masters/master-1", at(300), at(360)),
				haproxyDownInterval("master-1", "masters/master-2", at(300), at(360)),
			},
			expected: map[string][]apiOutageWindow{
				"master-0": {{from: at(0), to: at(60)}},
				"master-1": {{from: at(300), to: at(360)}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := findFullAPIOutageWindows(tt.intervals, fullOutageBackendThreshold)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestEvaluateFullAPIOutages(t *testing.T) {
	// installOutage simulates the expected initial state: when haproxy starts during the
	// installation, all kube-apiservers are down until they come up for the first time.
	installOutage := func(reportingNode string) monitorapi.Intervals {
		return monitorapi.Intervals{
			haproxyDownInterval(reportingNode, "masters/master-0", at(0), at(300)),
			haproxyDownInterval(reportingNode, "masters/master-1", at(0), at(360)),
			haproxyDownInterval(reportingNode, "masters/master-2", at(0), at(420)),
		}
	}

	tests := []struct {
		name            string
		intervals       monitorapi.Intervals
		expectFailure   bool
		expectedOutputs []string
	}{
		{
			name:          "no intervals",
			intervals:     monitorapi.Intervals{},
			expectFailure: false,
		},
		{
			name:          "only the installation outage",
			intervals:     installOutage("master-0"),
			expectFailure: false,
		},
		{
			name: "full outage after the installation",
			intervals: append(installOutage("master-0"),
				haproxyDownInterval("master-0", "masters/master-0", at(3600), at(3630)),
				haproxyDownInterval("master-0", "masters/master-1", at(3600), at(3630)),
				haproxyDownInterval("master-0", "masters/master-2", at(3600), at(3630)),
			),
			expectFailure: true,
			expectedOutputs: []string{
				"haproxy on node master-0",
				at(3600).Format(time.RFC3339),
				at(3630).Format(time.RFC3339),
			},
		},
		{
			name:          "initial outage tolerated separately per haproxy instance",
			intervals:     append(installOutage("master-0"), installOutage("master-1")...),
			expectFailure: false,
		},
		{
			name: "partial outage after the installation does not fail",
			intervals: append(installOutage("master-0"),
				haproxyDownInterval("master-0", "masters/master-0", at(3600), at(3630)),
				haproxyDownInterval("master-0", "masters/master-1", at(3600), at(3630)),
			),
			expectFailure: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			junits := evaluateFullAPIOutages(tt.intervals)
			require.Len(t, junits, 1)

			if !tt.expectFailure {
				assert.Nil(t, junits[0].FailureOutput, "expected the test to pass")
				return
			}

			require.NotNil(t, junits[0].FailureOutput, "expected the test to fail")
			for _, expectedOutput := range tt.expectedOutputs {
				assert.Contains(t, junits[0].SystemOut, expectedOutput)
			}
		})
	}
}

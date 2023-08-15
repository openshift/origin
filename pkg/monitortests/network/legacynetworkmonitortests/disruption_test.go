package legacynetworkmonitortests

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/stretchr/testify/assert"
)

func Test_dnsOverlapDisruption(t *testing.T) {
	events := []monitorapi.Interval{
		{
			Condition: monitorapi.Condition{
				Locator: "disruption/openshift-api connection/new",
				Message: "reason/DisruptionSamplerOutageBegan DNS lookup timeouts began",
			},
			From: time.Now(),
			To:   time.Now().Add(1 * time.Minute),
		},
		{
			Condition: monitorapi.Condition{
				Locator: "disruption/openshift-api connection/new",
				Message: "reason/DisruptionSamplerOutageBegan DNS lookup timeouts began",
			},
			From: time.Now().Add(2 * time.Minute),
			To:   time.Now().Add(3 * time.Minute),
		},
		{
			Condition: monitorapi.Condition{
				Locator: "disruption/openshift-api connection/new",
				Message: "reason/DisruptionSamplerOutageBegan DNS lookup timeouts began",
			},
			From: time.Now().Add(4 * time.Minute),
			To:   time.Now().Add(5 * time.Minute),
		},
		{
			Condition: monitorapi.Condition{
				Locator: "disruption/openshift-api connection/new",
				Message: "reason/DisruptionBegan disruption",
			},
			From: time.Now().Add(6 * time.Minute),
			To:   time.Now().Add(7 * time.Minute),
		},
		{
			Condition: monitorapi.Condition{
				Locator: "disruption/openshift-api connection/new",
				Message: "reason/DisruptionBegan disruption",
			},
			From: time.Now().Add(8 * time.Minute),
			To:   time.Now().Add(9 * time.Minute),
		},
		{
			Condition: monitorapi.Condition{
				Locator: "disruption/openshift-api connection/new",
				Message: "reason/DisruptionBegan disruption",
			},
			From: time.Now().Add(6 * time.Minute),
			To:   time.Now().Add(8 * time.Minute),
		},
	}

	testCases := []struct {
		name      string
		events    monitorapi.Intervals
		expectErr bool
	}{
		{
			name:      "No overlap between DNS and disruption",
			events:    events,
			expectErr: false,
		},
		{
			name: "Partial Overlap between DNS and disruption",
			events: append(events, monitorapi.Interval{
				Condition: monitorapi.Condition{
					Message: "reason/DisruptionBegan disruption",
				},
				From: time.Now().Add(3*time.Minute + 50*time.Second),
				To:   time.Now().Add(4*time.Minute + 5*time.Second),
			}),
			expectErr: true,
		},
		{
			name: "Complete Overlap between DNS and disruption",
			events: append(events, monitorapi.Interval{
				Condition: monitorapi.Condition{
					Message: "reason/DisruptionSamplerOutageBegan DNS lookup timeouts began",
				},
				From: time.Now().Add(5 * time.Minute),
				To:   time.Now().Add(6 * time.Minute),
			}),
			expectErr: true,
		},
		{
			name: "Overlap within 10 seconds between DNS and disruption",
			events: append(events, monitorapi.Interval{
				Condition: monitorapi.Condition{
					Message: "reason/DisruptionSamplerOutageBegan DNS lookup timeouts began",
				},
				From: time.Now().Add(6*time.Minute + 5*time.Second),
				To:   time.Now().Add(6*time.Minute + 15*time.Second),
			}),
			expectErr: true,
		},
		{
			name: "Overlap between DNS and disruption with same start time",
			events: append(events, monitorapi.Interval{
				Condition: monitorapi.Condition{
					Message: "reason/DisruptionBegan disruption",
				},
				From: time.Now().Add(4 * time.Minute),
				To:   time.Now().Add(4*time.Minute + 10*time.Second),
			}),
			expectErr: true,
		},
		{
			name: "Overlap between DNS and disruption with same end time",
			events: append(events, monitorapi.Interval{
				Condition: monitorapi.Condition{
					Message: "reason/DisruptionBegan disruption",
				},
				From: time.Now().Add(2*time.Minute + 45*time.Second),
				To:   time.Now().Add(3 * time.Minute),
			}),
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expected := 1
			testResults := testDNSOverlapDisruption(tc.events)
			if tc.expectErr {
				expected = 2
			}
			assert.Equal(t, expected, len(testResults), "Test results did not match")
		})
	}
}

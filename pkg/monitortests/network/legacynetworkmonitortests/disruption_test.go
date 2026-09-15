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
				Locator: monitorapi.Locator{
					Type: monitorapi.LocatorTypeDisruption,
					Keys: map[monitorapi.LocatorKey]string{
						monitorapi.LocatorDisruptionKey: "openshift-api",
						monitorapi.LocatorConnectionKey: "new",
					},
				},
				Message: monitorapi.Message{
					Reason:       "DisruptionSamplerOutageBegan",
					HumanMessage: "DNS lookup timeouts began",
				},
			},
			From: time.Now(),
			To:   time.Now().Add(1 * time.Minute),
		},
		{
			Condition: monitorapi.Condition{
				Locator: monitorapi.Locator{
					Type: monitorapi.LocatorTypeDisruption,
					Keys: map[monitorapi.LocatorKey]string{
						monitorapi.LocatorDisruptionKey: "openshift-api",
						monitorapi.LocatorConnectionKey: "new",
					},
				},
				Message: monitorapi.Message{
					Reason:       "DisruptionSamplerOutageBegan",
					HumanMessage: "DNS lookup timeouts began",
				},
			},
			From: time.Now().Add(2 * time.Minute),
			To:   time.Now().Add(3 * time.Minute),
		},
		{
			Condition: monitorapi.Condition{
				Locator: monitorapi.Locator{
					Type: monitorapi.LocatorTypeDisruption,
					Keys: map[monitorapi.LocatorKey]string{
						monitorapi.LocatorDisruptionKey: "openshift-api",
						monitorapi.LocatorConnectionKey: "new",
					},
				},
				Message: monitorapi.Message{
					Reason:       "DisruptionSamplerOutageBegan",
					HumanMessage: "DNS lookup timeouts began",
				},
			},
			From: time.Now().Add(4 * time.Minute),
			To:   time.Now().Add(5 * time.Minute),
		},
		{
			Condition: monitorapi.Condition{
				Locator: monitorapi.Locator{
					Type: monitorapi.LocatorTypeDisruption,
					Keys: map[monitorapi.LocatorKey]string{
						monitorapi.LocatorDisruptionKey: "openshift-api",
						monitorapi.LocatorConnectionKey: "new",
					},
				},
				Message: monitorapi.Message{
					Reason:       "DisruptionBegan",
					HumanMessage: "disruption",
				},
			},
			From: time.Now().Add(6 * time.Minute),
			To:   time.Now().Add(7 * time.Minute),
		},
		{
			Condition: monitorapi.Condition{
				Locator: monitorapi.Locator{
					Type: monitorapi.LocatorTypeDisruption,
					Keys: map[monitorapi.LocatorKey]string{
						monitorapi.LocatorDisruptionKey: "openshift-api",
						monitorapi.LocatorConnectionKey: "new",
					},
				},
				Message: monitorapi.Message{
					Reason:       "DisruptionBegan",
					HumanMessage: "disruption",
				},
			},
			From: time.Now().Add(8 * time.Minute),
			To:   time.Now().Add(9 * time.Minute),
		},
		{
			Condition: monitorapi.Condition{
				Locator: monitorapi.Locator{
					Type: monitorapi.LocatorTypeDisruption,
					Keys: map[monitorapi.LocatorKey]string{
						monitorapi.LocatorDisruptionKey: "openshift-api",
						monitorapi.LocatorConnectionKey: "new",
					},
				},
				Message: monitorapi.Message{
					Reason:       "DisruptionBegan",
					HumanMessage: "disruption",
				},
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
					Locator: monitorapi.Locator{
						Type: monitorapi.LocatorTypeDisruption,
						Keys: map[monitorapi.LocatorKey]string{
							monitorapi.LocatorDisruptionKey: "openshift-api",
							monitorapi.LocatorConnectionKey: "new",
						},
					},
					Message: monitorapi.Message{
						Reason:       "DisruptionBegan",
						HumanMessage: "disruption",
					},
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
					Locator: monitorapi.Locator{
						Type: monitorapi.LocatorTypeDisruption,
						Keys: map[monitorapi.LocatorKey]string{
							monitorapi.LocatorDisruptionKey: "openshift-api",
							monitorapi.LocatorConnectionKey: "new",
						},
					},
					Message: monitorapi.Message{
						Reason:       "DisruptionBegan",
						HumanMessage: "disruption",
					},
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
					Locator: monitorapi.Locator{
						Type: monitorapi.LocatorTypeDisruption,
						Keys: map[monitorapi.LocatorKey]string{
							monitorapi.LocatorDisruptionKey: "openshift-api",
							monitorapi.LocatorConnectionKey: "new",
						},
					},
					Message: monitorapi.Message{
						Reason:       "DisruptionSamplerOutageBegan",
						HumanMessage: "DNS lookup timeouts began",
					},
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
					Locator: monitorapi.Locator{
						Type: monitorapi.LocatorTypeDisruption,
						Keys: map[monitorapi.LocatorKey]string{
							monitorapi.LocatorDisruptionKey: "openshift-api",
							monitorapi.LocatorConnectionKey: "new",
						},
					},
					Message: monitorapi.Message{
						Reason:       "DisruptionBegan",
						HumanMessage: "disruption",
					},
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
					Locator: monitorapi.Locator{
						Type: monitorapi.LocatorTypeDisruption,
						Keys: map[monitorapi.LocatorKey]string{
							monitorapi.LocatorDisruptionKey: "openshift-api",
							monitorapi.LocatorConnectionKey: "new",
						},
					},
					Message: monitorapi.Message{
						Reason:       "DisruptionBegan",
						HumanMessage: "disruption",
					},
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

func Test_noExcessiveDNSDisruption(t *testing.T) {
	makeDNSEvent := func() monitorapi.Interval {
		return monitorapi.Interval{
			Condition: monitorapi.Condition{
				Locator: monitorapi.Locator{
					Type: monitorapi.LocatorTypeDisruption,
					Keys: map[monitorapi.LocatorKey]string{
						monitorapi.LocatorDisruptionKey: "openshift-api",
						monitorapi.LocatorConnectionKey: "new",
					},
				},
				Message: monitorapi.Message{
					Reason:       monitorapi.DisruptionSamplerOutageBeganEventReason,
					HumanMessage: "DNS lookup timeouts began",
				},
			},
			From: time.Now(),
			To:   time.Now().Add(1 * time.Second),
		}
	}

	testCases := []struct {
		name       string
		eventCount int
		expectFail bool
	}{
		{
			name:       "no DNS events passes",
			eventCount: 0,
			expectFail: false,
		},
		{
			name:       "50 DNS events passes",
			eventCount: 50,
			expectFail: false,
		},
		{
			name:       "51 DNS events fails",
			eventCount: 51,
			expectFail: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			events := monitorapi.Intervals{}
			for i := 0; i < tc.eventCount; i++ {
				events = append(events, makeDNSEvent())
			}
			results := testNoExcessiveDNSDisruption(events)
			assert.Equal(t, 1, len(results))
			if tc.expectFail {
				assert.NotNil(t, results[0].FailureOutput, "expected failure")
			} else {
				assert.Nil(t, results[0].FailureOutput, "expected pass")
			}
		})
	}
}

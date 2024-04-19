package disruptionserializer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestComputeDisruptionData(t *testing.T) {
	tests := []struct {
		name      string
		intervals monitorapi.Intervals
		expected  map[string]BackendDisruption
	}{
		{
			name: "no disruption",
			intervals: []monitorapi.Interval{
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Info,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeDisruption,
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorBackendDisruptionNameKey: "kube-api-new-connections",
								monitorapi.LocatorDisruptionKey:            "kube-api",
								monitorapi.LocatorConnectionKey:            "new",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.DisruptionEndedEventReason,
							Cause:        "",
							HumanMessage: "disruption/kube-api connection/new started responding to GET requests over new connections",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationReason: string(monitorapi.DisruptionEndedEventReason),
							},
						},
					},
					From:   time.Now().Add(-60 * time.Minute),
					To:     time.Now().Add(-1 * time.Minute),
					Source: monitorapi.SourceDisruption,
				},
			},
			expected: map[string]BackendDisruption{
				"kube-api-new-connections": {
					Name:               "kube-api-new-connections",
					BackendName:        "kube-api-new-connections",
					ConnectionType:     "New",
					DisruptedDuration:  metav1.Duration{Duration: 0 * time.Second},
					DisruptionMessages: nil,
				},
			},
		},
		{
			name: "single backend single disruption",
			intervals: []monitorapi.Interval{
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Info,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeDisruption,
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorBackendDisruptionNameKey: "kube-api-new-connections",
								monitorapi.LocatorDisruptionKey:            "kube-api",
								monitorapi.LocatorConnectionKey:            "new",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.DisruptionEndedEventReason,
							Cause:        "",
							HumanMessage: "disruption/kube-api connection/new started responding to GET requests over new connections",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationReason: string(monitorapi.DisruptionEndedEventReason),
							},
						},
					},
					From:   time.Now().Add(-60 * time.Minute),
					To:     time.Now().Add(-30 * time.Minute),
					Source: monitorapi.SourceDisruption,
				},
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Error,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeDisruption,
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorBackendDisruptionNameKey: "kube-api-new-connections",
								monitorapi.LocatorDisruptionKey:            "kube-api",
								monitorapi.LocatorConnectionKey:            "new",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.DisruptionBeganEventReason,
							Cause:        "",
							HumanMessage: "foo",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationReason: string(monitorapi.DisruptionBeganEventReason),
							},
						},
					},
					From:   time.Now().Add(-30 * time.Minute),
					To:     time.Now().Add(-20 * time.Minute),
					Source: monitorapi.SourceDisruption,
				},
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Info,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeDisruption,
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorBackendDisruptionNameKey: "kube-api-new-connections",
								monitorapi.LocatorDisruptionKey:            "kube-api",
								monitorapi.LocatorConnectionKey:            "new",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.DisruptionEndedEventReason,
							Cause:        "",
							HumanMessage: "disruption/kube-api connection/new started responding to GET requests over new connections",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationReason: string(monitorapi.DisruptionEndedEventReason),
							},
						},
					},
					From:   time.Now().Add(-20 * time.Minute),
					To:     time.Now().Add(-10 * time.Minute),
					Source: monitorapi.SourceDisruption,
				},
			},
			expected: map[string]BackendDisruption{
				"kube-api-new-connections": {
					Name:              "kube-api-new-connections",
					BackendName:       "kube-api-new-connections",
					ConnectionType:    "New",
					DisruptedDuration: metav1.Duration{Duration: 10 * time.Minute},
				},
			},
		},
		{
			name: "single backend multi disruption",
			intervals: []monitorapi.Interval{
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Info,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeDisruption,
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorBackendDisruptionNameKey: "kube-api-new-connections",
								monitorapi.LocatorDisruptionKey:            "kube-api",
								monitorapi.LocatorConnectionKey:            "new",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.DisruptionEndedEventReason,
							Cause:        "",
							HumanMessage: "disruption/kube-api connection/new started responding to GET requests over new connections",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationReason: string(monitorapi.DisruptionEndedEventReason),
							},
						},
					},
					From:   time.Now().Add(-60 * time.Minute),
					To:     time.Now().Add(-50 * time.Minute),
					Source: monitorapi.SourceDisruption,
				},
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Error,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeDisruption,
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorBackendDisruptionNameKey: "kube-api-new-connections",
								monitorapi.LocatorDisruptionKey:            "kube-api",
								monitorapi.LocatorConnectionKey:            "new",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.DisruptionBeganEventReason,
							Cause:        "",
							HumanMessage: "foo",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationReason: string(monitorapi.DisruptionBeganEventReason),
							},
						},
					},
					From:   time.Now().Add(-50 * time.Minute),
					To:     time.Now().Add(-40 * time.Minute),
					Source: monitorapi.SourceDisruption,
				},
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Info,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeDisruption,
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorBackendDisruptionNameKey: "kube-api-new-connections",
								monitorapi.LocatorDisruptionKey:            "kube-api",
								monitorapi.LocatorConnectionKey:            "new",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.DisruptionEndedEventReason,
							Cause:        "",
							HumanMessage: "disruption/kube-api connection/new started responding to GET requests over new connections",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationReason: string(monitorapi.DisruptionEndedEventReason),
							},
						},
					},
					From:   time.Now().Add(-40 * time.Minute),
					To:     time.Now().Add(-30 * time.Minute),
					Source: monitorapi.SourceDisruption,
				},
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Error,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeDisruption,
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorBackendDisruptionNameKey: "kube-api-new-connections",
								monitorapi.LocatorDisruptionKey:            "kube-api",
								monitorapi.LocatorConnectionKey:            "new",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.DisruptionBeganEventReason,
							Cause:        "",
							HumanMessage: "foo",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationReason: string(monitorapi.DisruptionBeganEventReason),
							},
						},
					},
					From:   time.Now().Add(-30 * time.Minute),
					To:     time.Now().Add(-20 * time.Minute),
					Source: monitorapi.SourceDisruption,
				},
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Info,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeDisruption,
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorBackendDisruptionNameKey: "kube-api-new-connections",
								monitorapi.LocatorDisruptionKey:            "kube-api",
								monitorapi.LocatorConnectionKey:            "new",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.DisruptionEndedEventReason,
							Cause:        "",
							HumanMessage: "disruption/kube-api connection/new started responding to GET requests over new connections",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationReason: string(monitorapi.DisruptionEndedEventReason),
							},
						},
					},
					From:   time.Now().Add(-20 * time.Minute),
					To:     time.Now().Add(-10 * time.Minute),
					Source: monitorapi.SourceDisruption,
				},
			},
			expected: map[string]BackendDisruption{
				"kube-api-new-connections": {
					Name:              "kube-api-new-connections",
					BackendName:       "kube-api-new-connections",
					ConnectionType:    "New",
					DisruptedDuration: metav1.Duration{Duration: 20 * time.Minute},
				},
			},
		},
		{
			name: "multi backend single disruption",
			intervals: []monitorapi.Interval{
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Info,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeDisruption,
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorBackendDisruptionNameKey: "kube-api-new-connections",
								monitorapi.LocatorDisruptionKey:            "kube-api",
								monitorapi.LocatorConnectionKey:            "new",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.DisruptionEndedEventReason,
							Cause:        "",
							HumanMessage: "disruption/kube-api connection/new started responding to GET requests over new connections",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationReason: string(monitorapi.DisruptionEndedEventReason),
							},
						},
					},
					From:   time.Now().Add(-60 * time.Minute),
					To:     time.Now().Add(-30 * time.Minute),
					Source: monitorapi.SourceDisruption,
				},
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Error,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeDisruption,
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorBackendDisruptionNameKey: "kube-api-new-connections",
								monitorapi.LocatorDisruptionKey:            "kube-api",
								monitorapi.LocatorConnectionKey:            "new",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.DisruptionBeganEventReason,
							Cause:        "",
							HumanMessage: "foo",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationReason: string(monitorapi.DisruptionBeganEventReason),
							},
						},
					},
					From:   time.Now().Add(-30 * time.Minute),
					To:     time.Now().Add(-20 * time.Minute),
					Source: monitorapi.SourceDisruption,
				},
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Info,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeDisruption,
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorBackendDisruptionNameKey: "kube-api-new-connections",
								monitorapi.LocatorDisruptionKey:            "kube-api",
								monitorapi.LocatorConnectionKey:            "new",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.DisruptionEndedEventReason,
							Cause:        "",
							HumanMessage: "disruption/kube-api connection/new started responding to GET requests over new connections",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationReason: string(monitorapi.DisruptionEndedEventReason),
							},
						},
					},
					From:   time.Now().Add(-20 * time.Minute),
					To:     time.Now().Add(-10 * time.Minute),
					Source: monitorapi.SourceDisruption,
				},
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Info,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeDisruption,
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorBackendDisruptionNameKey: "openshift-api-new-connections",
								monitorapi.LocatorDisruptionKey:            "openshift-api",
								monitorapi.LocatorConnectionKey:            "new",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.DisruptionEndedEventReason,
							Cause:        "",
							HumanMessage: "disruption/openshift-api connection/new started responding to GET requests over new connections",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationReason: string(monitorapi.DisruptionEndedEventReason),
							},
						},
					},
					From:   time.Now().Add(-60 * time.Minute),
					To:     time.Now().Add(-30 * time.Minute),
					Source: monitorapi.SourceDisruption,
				},
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Error,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeDisruption,
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorBackendDisruptionNameKey: "openshift-api-new-connections",
								monitorapi.LocatorDisruptionKey:            "openshift-api",
								monitorapi.LocatorConnectionKey:            "new",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.DisruptionBeganEventReason,
							Cause:        "",
							HumanMessage: "foo",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationReason: string(monitorapi.DisruptionBeganEventReason),
							},
						},
					},
					From:   time.Now().Add(-30 * time.Minute),
					To:     time.Now().Add(-25 * time.Minute),
					Source: monitorapi.SourceDisruption,
				},
				{
					Condition: monitorapi.Condition{
						Level: monitorapi.Info,
						Locator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeDisruption,
							Keys: map[monitorapi.LocatorKey]string{
								monitorapi.LocatorBackendDisruptionNameKey: "openshift-api-new-connections",
								monitorapi.LocatorDisruptionKey:            "openshift-api",
								monitorapi.LocatorConnectionKey:            "new",
							},
						},
						Message: monitorapi.Message{
							Reason:       monitorapi.DisruptionEndedEventReason,
							Cause:        "",
							HumanMessage: "disruption/kube-api connection/new started responding to GET requests over new connections",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationReason: string(monitorapi.DisruptionEndedEventReason),
							},
						},
					},
					From:   time.Now().Add(-25 * time.Minute),
					To:     time.Now().Add(-10 * time.Minute),
					Source: monitorapi.SourceDisruption,
				},
			},
			expected: map[string]BackendDisruption{
				"kube-api-new-connections": {
					Name:              "kube-api-new-connections",
					BackendName:       "kube-api-new-connections",
					ConnectionType:    "New",
					DisruptedDuration: metav1.Duration{Duration: 10 * time.Minute},
				},
				"openshift-api-new-connections": {
					Name:              "openshift-api-new-connections",
					BackendName:       "openshift-api-new-connections",
					ConnectionType:    "New",
					DisruptedDuration: metav1.Duration{Duration: 5 * time.Minute},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			disruptions := computeDisruptionData(tt.intervals)
			for backend, expectedDisruption := range tt.expected {
				if !assert.Contains(t, disruptions.BackendDisruptions, backend) {
					continue
				}
				ad := disruptions.BackendDisruptions[backend]
				assert.Equal(t, expectedDisruption.Name, ad.Name)
				assert.Equal(t, expectedDisruption.BackendName, ad.BackendName)
				assert.Equal(t, expectedDisruption.ConnectionType, ad.ConnectionType)
				assert.Equal(t, expectedDisruption.DisruptedDuration, ad.DisruptedDuration)
				// NOTE: not checking the actual disruption messages, embedded timestamps make it cumbersome
			}
		})
	}
}

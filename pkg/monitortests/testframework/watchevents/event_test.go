package watchevents

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func Test_recordAddOrUpdateEvent(t *testing.T) {

	type args struct {
		ctx                    context.Context
		m                      monitorapi.Recorder
		client                 kubernetes.Interface
		reMatchFirstQuote      *regexp.Regexp
		significantlyBeforeNow time.Time
		kubeEvent              *corev1.Event
	}

	now := time.Now()

	tests := []struct {
		name            string
		args            args
		skip            bool
		kubeEvent       *corev1.Event
		expectedLocator string
		expectedMessage string
	}{
		{
			name: "simple event",
			args: args{
				ctx: context.TODO(),
				m:   monitor.NewRecorder(),
				kubeEvent: &corev1.Event{
					Count:  2,
					Reason: "SomethingHappened",
					InvolvedObject: corev1.ObjectReference{
						Kind:      "Pod",
						Namespace: "openshift-authentication",
						Name:      "testpod-927947",
					},
					Message:       "sample message",
					LastTimestamp: metav1.Now(),
				},
			},
			expectedLocator: "ns/openshift-authentication pod/testpod-927947",
			expectedMessage: "reason/SomethingHappened sample message (2 times)",
		},
		{
			name: "unknown pathological event",
			args: args{
				ctx: context.TODO(),
				m:   monitor.NewRecorder(),
				kubeEvent: &corev1.Event{
					Count:  40,
					Reason: "SomethingHappened",
					InvolvedObject: corev1.ObjectReference{
						Kind:      "Pod",
						Namespace: "openshift-authentication",
						Name:      "testpod-927947",
					},
					Message:       "sample message",
					LastTimestamp: metav1.Now(),
				},
			},
			expectedLocator: "ns/openshift-authentication pod/testpod-927947 hmsg/72c78c2ba1",
			expectedMessage: "pathological/true reason/SomethingHappened sample message (40 times)",
		},
		{
			name: "allowed pathological event",
			args: args{
				ctx: context.TODO(),
				m:   monitor.NewRecorder(),
				kubeEvent: &corev1.Event{
					Count:  40,
					Reason: "SomethingHappened",
					InvolvedObject: corev1.ObjectReference{
						Kind:      "Pod",
						Namespace: "openshift-e2e-loki",
						Name:      "loki-promtail-982739",
					},
					Message:       "Readiness probe failed",
					LastTimestamp: metav1.Now(),
				},
				significantlyBeforeNow: now.UTC().Add(-15 * time.Minute),
			},
			expectedLocator: "ns/openshift-e2e-loki pod/loki-promtail-982739 hmsg/04cd2d7fbb",
			expectedMessage: "pathological/true interesting/true reason/SomethingHappened Readiness probe failed (40 times)",
		},
		{
			name: "allowed pathological event with known bug",
			args: args{
				ctx: context.TODO(),
				m:   monitor.NewRecorder(),
				kubeEvent: &corev1.Event{
					Count:  40,
					Reason: "TopologyAwareHintsDisabled",
					InvolvedObject: corev1.ObjectReference{
						Kind:      "Pod",
						Namespace: "any",
						Name:      "any",
					},
					Message:       "irrelevant",
					LastTimestamp: metav1.Now(),
				},
				significantlyBeforeNow: now.UTC().Add(-15 * time.Minute),
			},
			expectedLocator: "ns/any pod/any hmsg/e13faa98ab",
			expectedMessage: "pathological/true interesting/true reason/TopologyAwareHintsDisabled irrelevant (40 times)",
		},
	}
	for _, tt := range tests {
		if tt.skip {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			significantlyBeforeNow := now.UTC().Add(-15 * time.Minute)
			recordAddOrUpdateEvent(tt.args.ctx, tt.args.m, nil, nil, significantlyBeforeNow, tt.args.kubeEvent)
			intervals := tt.args.m.Intervals(now.Add(-10*time.Minute), now.Add(10*time.Minute))
			assert.Equal(t, 1, len(intervals))
			interval := intervals[0]
			assert.Equal(t, tt.expectedLocator, interval.Locator)
			assert.Equal(t, tt.expectedMessage, interval.Message)
		})
	}
}

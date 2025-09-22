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

	first := time.Now().Add(-30 * time.Minute)
	now := time.Now()

	tests := []struct {
		name            string
		args            args
		skip            bool
		kubeEvent       *corev1.Event
		expectedLocator monitorapi.Locator
		expectedMessage monitorapi.Message
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
					Message:        "sample message",
					FirstTimestamp: metav1.NewTime(first),
					LastTimestamp:  metav1.NewTime(now),
				},
			},
			expectedLocator: monitorapi.Locator{
				Type: monitorapi.LocatorTypeKind,
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorNamespaceKey: "openshift-authentication",
					monitorapi.LocatorPodKey:       "testpod-927947",
					monitorapi.LocatorHmsgKey:      "59162c6b05",
				},
			},
			expectedMessage: monitorapi.NewMessage().Reason("SomethingHappened").
				HumanMessage("sample message").WithAnnotation(monitorapi.AnnotationCount, "2").
				WithAnnotation("firstTimestamp", first.Format(time.RFC3339)).
				WithAnnotation("lastTimestamp", now.Format(time.RFC3339)).
				Build(),
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
					Message:        "sample message",
					FirstTimestamp: metav1.NewTime(first),
					LastTimestamp:  metav1.NewTime(now),
				},
			},
			expectedLocator: monitorapi.Locator{
				Type: monitorapi.LocatorTypeKind,
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorNamespaceKey: "openshift-authentication",
					monitorapi.LocatorPodKey:       "testpod-927947",
					monitorapi.LocatorHmsgKey:      "59162c6b05",
				},
			},
			expectedMessage: monitorapi.NewMessage().Reason("SomethingHappened").
				HumanMessage("sample message").WithAnnotation(monitorapi.AnnotationCount, "40").
				WithAnnotation(monitorapi.AnnotationPathological, "true").
				WithAnnotation("firstTimestamp", first.Format(time.RFC3339)).
				WithAnnotation("lastTimestamp", now.Format(time.RFC3339)).
				Build(),
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
					Message:        "Readiness probe failed",
					FirstTimestamp: metav1.NewTime(first),
					LastTimestamp:  metav1.NewTime(now),
				},
				significantlyBeforeNow: now.UTC().Add(-15 * time.Minute),
			},
			expectedLocator: monitorapi.Locator{
				Type: monitorapi.LocatorTypeKind,
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorNamespaceKey: "openshift-e2e-loki",
					monitorapi.LocatorPodKey:       "loki-promtail-982739",
					monitorapi.LocatorHmsgKey:      "c166d9c33e",
				},
			},
			expectedMessage: monitorapi.NewMessage().Reason("SomethingHappened").
				HumanMessage("Readiness probe failed").
				WithAnnotation(monitorapi.AnnotationCount, "40").
				WithAnnotation(monitorapi.AnnotationPathological, "true").
				WithAnnotation(monitorapi.AnnotationInteresting, "true").
				WithAnnotation("firstTimestamp", first.Format(time.RFC3339)).
				WithAnnotation("lastTimestamp", now.Format(time.RFC3339)).
				Build(),
		},
	}
	for _, tt := range tests {
		if tt.skip {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			significantlyBeforeNow := now.UTC().Add(-15 * time.Minute)
			recordAddOrUpdateEvent(tt.args.ctx, tt.args.m, "", nil, significantlyBeforeNow, tt.args.kubeEvent, nil)
			intervals := tt.args.m.Intervals(now.Add(-10*time.Minute), now.Add(10*time.Minute))
			assert.Equal(t, 1, len(intervals))
			interval := intervals[0]
			assert.Equal(t, tt.expectedLocator, interval.Locator)
			assert.Equal(t, tt.expectedMessage, interval.Message)
		})
	}
}

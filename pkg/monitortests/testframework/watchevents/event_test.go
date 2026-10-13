package watchevents

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/pathologicaleventlibrary"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
		{
			name: "oauth scaling event captures identity and series evidence when present",
			args: args{
				ctx: context.TODO(),
				m:   monitor.NewRecorder(),
				kubeEvent: &corev1.Event{
					ObjectMeta: metav1.ObjectMeta{
						UID:             "abc-123-uid",
						ResourceVersion: "987654",
					},
					Count:  40,
					Reason: "ScalingReplicaSet",
					InvolvedObject: corev1.ObjectReference{
						Kind:      "Deployment",
						Namespace: "openshift-authentication",
						Name:      "oauth-openshift",
					},
					Message:        "Scaled up replica set oauth-openshift-788ffcfbc6 from 0 to 3",
					FirstTimestamp: metav1.NewTime(first),
					LastTimestamp:  metav1.NewTime(now),
					Series: &corev1.EventSeries{
						Count:            40,
						LastObservedTime: metav1.NewMicroTime(now),
					},
				},
			},
			expectedLocator: monitorapi.Locator{
				Type: monitorapi.LocatorTypeKind,
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorNamespaceKey:  "openshift-authentication",
					monitorapi.LocatorDeploymentKey: "oauth-openshift",
					monitorapi.LocatorHmsgKey:       "0a7dd7352a",
				},
			},
			expectedMessage: monitorapi.NewMessage().Reason("ScalingReplicaSet").
				HumanMessage("Scaled up replica set oauth-openshift-788ffcfbc6 from 0 to 3").
				WithAnnotation(monitorapi.AnnotationCount, "40").
				WithAnnotation(monitorapi.AnnotationPathological, "true").
				WithAnnotation("firstTimestamp", first.Format(time.RFC3339)).
				WithAnnotation("lastTimestamp", now.Format(time.RFC3339)).
				WithAnnotation("eventUID", "abc-123-uid").
				WithAnnotation("resourceVersion", "987654").
				WithAnnotation("seriesCount", "40").
				WithAnnotation("seriesLastObservedTime", now.UTC().Format(time.RFC3339Nano)).
				Build(),
		},
	}
	for _, tt := range tests {
		if tt.skip {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			significantlyBeforeNow := now.UTC().Add(-15 * time.Minute)
			recordAddOrUpdateEvent(tt.args.ctx, tt.args.m, "", nil, significantlyBeforeNow, tt.args.kubeEvent)
			intervals := tt.args.m.Intervals(now.Add(-10*time.Minute), now.Add(10*time.Minute))
			assert.Equal(t, 1, len(intervals))
			interval := intervals[0]
			assert.Equal(t, tt.expectedLocator, interval.Locator)
			assert.Equal(t, tt.expectedMessage, interval.Message)
		})
	}
}

// Test_SeriesLastObservedEndBoundary_ProducerConsumer is the regression for review finding B
// (discussion_r4261872213). It exercises BOTH the watcher producer (recordAddOrUpdateEvent,
// which must preserve the sub-second MicroTime of Series.LastObservedTime via RFC3339Nano)
// AND the accounting consumer (EvaluateOAuthBoundedAccountingShadow strict end-boundary
// containment). An oauth scaling step observed exactly at the window end is contained
// (attributed); the same step observed a sub-second after the end must stay Unresolved.
func Test_SeriesLastObservedEndBoundary_ProducerConsumer(t *testing.T) {
	const prefix = `[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxy][Serial]`
	const t4 = prefix + " oauth-server should perform full OIDC login flow through the proxy when auth proxy config is applied"

	base := time.Now().UTC().Truncate(time.Second)
	windowFrom := base
	windowTo := base.Add(5 * time.Minute)

	scaleEvent := func(rv string, count int32, observed, lastTS time.Time) *corev1.Event {
		return &corev1.Event{
			ObjectMeta:     metav1.ObjectMeta{UID: types.UID("uid-a"), ResourceVersion: rv},
			Count:          count,
			Reason:         "ScalingReplicaSet",
			InvolvedObject: corev1.ObjectReference{Kind: "Deployment", Namespace: "openshift-authentication", Name: "oauth-openshift"},
			Message:        "Scaled up replica set oauth-openshift-788ffcfbc6 from 0 to 3",
			FirstTimestamp: metav1.NewTime(lastTS),
			LastTimestamp:  metav1.NewTime(lastTS),
			Series:         &corev1.EventSeries{Count: count, LastObservedTime: metav1.NewMicroTime(observed)},
		}
	}
	produce := func(ev *corev1.Event) monitorapi.Interval {
		m := monitor.NewRecorder()
		recordAddOrUpdateEvent(context.TODO(), m, "", nil, time.Now().UTC().Add(-15*time.Minute), ev)
		ivs := m.Intervals(base.Add(-1*time.Hour), base.Add(24*time.Hour))
		require.Len(t, ivs, 1)
		return ivs[0]
	}

	e2eWindow := monitorapi.NewInterval(monitorapi.SourceE2ETest, monitorapi.Info).
		Locator(monitorapi.NewLocator().E2ETest(t4)).
		Message(monitorapi.NewMessage().HumanMessage("e2e test finished").WithAnnotation(monitorapi.AnnotationStatus, "Passed")).
		Build(windowFrom, windowTo)

	baseline := produce(scaleEvent("100", 5, base.Add(1*time.Minute), base.Add(1*time.Minute)))
	stepExact := produce(scaleEvent("101", 8, windowTo, windowTo))
	stepAfter := produce(scaleEvent("101", 8, windowTo.Add(500*time.Millisecond), windowTo))

	// Producer side: the sub-second component must survive serialization.
	assert.Contains(t, stepAfter.Message.Annotations[pathologicaleventlibrary.AnnotationSeriesLastObserved], ".",
		"producer must preserve fractional seconds (RFC3339Nano)")

	// Consumer side, exact boundary: contained -> attributed.
	reportExact, junitsExact := pathologicaleventlibrary.EvaluateOAuthBoundedAccountingShadow(
		monitorapi.Intervals{e2eWindow, baseline, stepExact}, nil)
	assert.Nil(t, junitsExact[0].FailureOutput, "shadow JUnit must never fail")
	assert.Equal(t, 1, reportExact.AttributedUnits, "step observed exactly at window end is contained")
	assert.Equal(t, 3, reportExact.PerWindowAttributed["T4"])

	// Consumer side, sub-second after end: strictly not contained -> unresolved.
	reportAfter, junitsAfter := pathologicaleventlibrary.EvaluateOAuthBoundedAccountingShadow(
		monitorapi.Intervals{e2eWindow, baseline, stepAfter}, nil)
	assert.Nil(t, junitsAfter[0].FailureOutput)
	assert.Equal(t, 0, reportAfter.AttributedUnits, "a sub-second-after-end observation must not be attributed")
	assert.GreaterOrEqual(t, reportAfter.UnresolvedUnits, 1)
}

// Test_NoSeriesSecondPrecisionFallback_NeverAttributes is the regression for review finding
// r4262473673. Without a Series, the only time the watcher records is the second-precision
// lastTimestamp. An event observed at window-end+500ms serializes to the end second, so a
// fallback to lastTimestamp would round it onto the boundary and wrongly attribute it. With the
// fallback removed, lack of a reliable seriesLastObservedTime leaves the step Unresolved
// (fail-closed) — it is never attributed. The reliable-Series positive controls remain in
// Test_SeriesLastObservedEndBoundary_ProducerConsumer.
func Test_NoSeriesSecondPrecisionFallback_NeverAttributes(t *testing.T) {
	const prefix = `[sig-auth][Suite:openshift/conformance/serial][Jira:"Authentication"][OCPFeatureGate:AuthenticationComponentProxy][Serial]`
	const t4 = prefix + " oauth-server should perform full OIDC login flow through the proxy when auth proxy config is applied"

	base := time.Now().UTC().Truncate(time.Second)
	windowFrom := base
	windowTo := base.Add(5 * time.Minute)

	// No Series on either event: the watcher will only write the second-precision lastTimestamp.
	noSeriesEvent := func(rv string, count int32, lastTS time.Time) *corev1.Event {
		return &corev1.Event{
			ObjectMeta:     metav1.ObjectMeta{UID: types.UID("uid-b"), ResourceVersion: rv},
			Count:          count,
			Reason:         "ScalingReplicaSet",
			InvolvedObject: corev1.ObjectReference{Kind: "Deployment", Namespace: "openshift-authentication", Name: "oauth-openshift"},
			Message:        "Scaled up replica set oauth-openshift-788ffcfbc6 from 0 to 3",
			FirstTimestamp: metav1.NewTime(lastTS),
			LastTimestamp:  metav1.NewTime(lastTS),
		}
	}
	produce := func(ev *corev1.Event) monitorapi.Interval {
		m := monitor.NewRecorder()
		recordAddOrUpdateEvent(context.TODO(), m, "", nil, time.Now().UTC().Add(-15*time.Minute), ev)
		ivs := m.Intervals(base.Add(-1*time.Hour), base.Add(24*time.Hour))
		require.Len(t, ivs, 1)
		return ivs[0]
	}

	e2eWindow := monitorapi.NewInterval(monitorapi.SourceE2ETest, monitorapi.Info).
		Locator(monitorapi.NewLocator().E2ETest(t4)).
		Message(monitorapi.NewMessage().HumanMessage("e2e test finished").WithAnnotation(monitorapi.AnnotationStatus, "Passed")).
		Build(windowFrom, windowTo)

	baseline := produce(noSeriesEvent("100", 5, base.Add(1*time.Minute)))             // in-window, second precision
	stepAfter := produce(noSeriesEvent("101", 8, windowTo.Add(500*time.Millisecond))) // rounds to window end second

	// The recorded annotation is second-precision and carries no fractional part, and there is no
	// seriesLastObservedTime at all.
	assert.NotContains(t, stepAfter.Message.Annotations[monitorapi.AnnotationKey("lastTimestamp")], ".")
	_, hasSeries := stepAfter.Message.Annotations[pathologicaleventlibrary.AnnotationSeriesLastObserved]
	assert.False(t, hasSeries, "no Series -> no seriesLastObservedTime annotation")

	report, junits := pathologicaleventlibrary.EvaluateOAuthBoundedAccountingShadow(
		monitorapi.Intervals{e2eWindow, baseline, stepAfter}, nil)
	assert.Nil(t, junits[0].FailureOutput)
	assert.Equal(t, 0, report.AttributedUnits, "no reliable sub-second series time -> never attributed (no lastTimestamp fallback)")
	assert.GreaterOrEqual(t, report.UnresolvedUnits, 1)
}

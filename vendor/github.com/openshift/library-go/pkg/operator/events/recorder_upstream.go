package events

import (
	"context"
	"fmt"
	"k8s.io/utils/clock"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
	"k8s.io/klog/v2"
)

// NewKubeRecorder returns new event recorder with tweaked correlator options.
func NewKubeRecorderWithOptions(client corev1client.EventInterface, options record.CorrelatorOptions, sourceComponentName string, involvedObjectRef *corev1.ObjectReference, clock clock.PassiveClock) Recorder {
	return (&upstreamRecorder{
		client:            client,
		component:         sourceComponentName,
		involvedObjectRef: involvedObjectRef,
		options:           options,
		fallbackRecorder:  NewRecorder(client, sourceComponentName, involvedObjectRef, clock),
	}).ForComponent(sourceComponentName)
}

// NewKubeRecorder returns new event recorder with default correlator options.
func NewKubeRecorder(client corev1client.EventInterface, sourceComponentName string, involvedObjectRef *corev1.ObjectReference, clock clock.PassiveClock) Recorder {
	return NewKubeRecorderWithOptions(client, record.CorrelatorOptions{}, sourceComponentName, involvedObjectRef, clock)
}

// upstreamRecorder is an implementation of Recorder interface.
type upstreamRecorder struct {
	client            corev1client.EventInterface
	clientCtx         context.Context
	component         string
	broadcaster       record.EventBroadcaster
	eventRecorder     record.EventRecorder
	involvedObjectRef *corev1.ObjectReference
	options           record.CorrelatorOptions

	// shuttingDown indicates that the broadcaster for this recorder is being shut down
	shuttingDown  bool
	shutdownMutex sync.RWMutex

	// fallbackRecorder is used when the kube recorder is shutting down
	// in that case we create the events directly.
	fallbackRecorder Recorder
}

func (r *upstreamRecorder) WithContext(ctx context.Context) Recorder {
	r.clientCtx = ctx
	return r
}

// RecommendedClusterSingletonCorrelatorOptions provides recommended event correlator options for components that produce
// many events (like operators).
func RecommendedClusterSingletonCorrelatorOptions() record.CorrelatorOptions {
	return record.CorrelatorOptions{
		BurstSize: 60,      // default: 25 (change allows a single source to send 50 events about object per minute)
		QPS:       1. / 1., // default: 1/300 (change allows refill rate to 1 new event every 1s)
		KeyFunc: func(event *corev1.Event) (aggregateKey string, localKey string) {
			return strings.Join([]string{
				event.Source.Component,
				event.Source.Host,
				event.InvolvedObject.Kind,
				event.InvolvedObject.Namespace,
				event.InvolvedObject.Name,
				string(event.InvolvedObject.UID),
				event.InvolvedObject.APIVersion,
				event.Type,
				event.Reason,
				// By default, KeyFunc don't use message for aggregation, this cause events with different message, but same reason not be lost as "similar events".
				event.Message,
			}, ""), event.Message
		},
	}
}

var eventsCounterMetric = metrics.NewCounterVec(&metrics.CounterOpts{
	Subsystem:      "event_recorder",
	Name:           "total_events_count",
	Help:           "Total count of events processed by this event recorder per involved object",
	StabilityLevel: metrics.ALPHA,
}, []string{"severity"})

func init() {
	(&sync.Once{}).Do(func() {
		legacyregistry.MustRegister(eventsCounterMetric)
	})
}

func (r *upstreamRecorder) ForComponent(componentName string) Recorder {
	newRecorderForComponent := upstreamRecorder{
		client:            r.client,
		fallbackRecorder:  r.fallbackRecorder.WithComponentSuffix(componentName),
		options:           r.options,
		involvedObjectRef: r.involvedObjectRef,
		shuttingDown:      r.shuttingDown,
	}

	// tweak the event correlator, so we don't loose important events.
	broadcaster := record.NewBroadcasterWithCorrelatorOptions(r.options)
	broadcaster.StartLogging(klog.Infof)
	broadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: newRecorderForComponent.client})

	newRecorderForComponent.eventRecorder = broadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: componentName})
	newRecorderForComponent.broadcaster = broadcaster
	newRecorderForComponent.component = componentName

	return &newRecorderForComponent
}

func (r *upstreamRecorder) Shutdown() {
	r.shutdownMutex.Lock()
	r.shuttingDown = true
	r.shutdownMutex.Unlock()
	// Wait for broadcaster to flush events (this is blocking)
	// TODO: There is still race condition in upstream that might cause panic() on events recorded after the shutdown
	//       is called as the event recording is not-blocking (go routine based).
	r.broadcaster.Shutdown()
}

func (r *upstreamRecorder) WithComponentSuffix(suffix string) Recorder {
	return r.ForComponent(fmt.Sprintf("%s-%s", r.ComponentName(), suffix))
}

func (r *upstreamRecorder) ComponentName() string {
	return r.component
}

// Eventf emits the normal type event and allow formatting of message.
func (r *upstreamRecorder) Eventf(reason, messageFmt string, args ...interface{}) {
	r.Event(reason, fmt.Sprintf(messageFmt, args...))
}

// Warningf emits the warning type event and allow formatting of message.
func (r *upstreamRecorder) Warningf(reason, messageFmt string, args ...interface{}) {
	r.Warning(reason, fmt.Sprintf(messageFmt, args...))
}

func (r *upstreamRecorder) incrementEventsCounter(severity string) {
	if r.involvedObjectRef == nil {
		return
	}
	eventsCounterMetric.WithLabelValues(severity).Inc()
}

// Event emits the normal type event.
func (r *upstreamRecorder) Event(reason, message string) {
	r.shutdownMutex.RLock()
	defer r.shutdownMutex.RUnlock()
	defer r.incrementEventsCounter(corev1.EventTypeNormal)
	if r.shuttingDown {
		r.fallbackRecorder.Event(reason, message)
		return
	}
	r.eventRecorder.Event(r.involvedObjectRef, corev1.EventTypeNormal, reason, message)
}

// Warning emits the warning type event.
func (r *upstreamRecorder) Warning(reason, message string) {
	r.shutdownMutex.RLock()
	defer r.shutdownMutex.RUnlock()
	defer r.incrementEventsCounter(corev1.EventTypeWarning)
	if r.shuttingDown {
		r.fallbackRecorder.Warning(reason, message)
		return
	}
	r.eventRecorder.Event(r.involvedObjectRef, corev1.EventTypeWarning, reason, message)
}

package eventwatch

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	corev1typed "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
)

const (
	// eventAckAnnotationName is an annotation we place on event that was processed by this controller.
	// This is used to not process same event multiple times.
	eventAckAnnotationName = "eventwatch.openshift.io/last-seen-count"
)

// Controller observes the events in given informer namespaces and match them with configured event handlers.
// If the event reason and namespace match the configured, then the process() function is called in handler on that event.
// The event is then acknowledged via annotation update, so each interesting event is only processed once.
// The process() function will received the observed event and can update Prometheus counter or manage operator conditions.
type Controller struct {
	events      []eventHandler
	eventClient corev1typed.EventsGetter
}

type eventHandler struct {
	reason    string
	namespace string
	process   func(event *corev1.Event) error
}

type Builder struct {
	eventConfig []eventHandler
}

// New returns a new event watch controller builder that allow to specify event handlers.
func New() *Builder {
	return &Builder{}
}

// WithEventHandler add handler for event matching the namespace and the reason.
// This can be called multiple times.
func (b *Builder) WithEventHandler(namespace, reason string, processEvent func(event *corev1.Event) error) *Builder {
	b.eventConfig = append(b.eventConfig, eventHandler{
		reason:    reason,
		namespace: namespace,
		process:   processEvent,
	})
	return b
}

// ToController returns a factory controller that can be run.
// The kubeInformersForTargetNamespace must have informer for namespaces which matching the registered event handlers.
// The event client is used to update/acknowledge events.
func (b *Builder) ToController(kubeInformersForTargetNamespace informers.SharedInformerFactory, eventClient corev1typed.EventsGetter, recorder events.Recorder) factory.Controller {
	c := &Controller{
		events:      b.eventConfig,
		eventClient: eventClient,
	}
	return factory.New().
		WithSync(c.sync).
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			event, ok := obj.(*corev1.Event)
			if !ok {
				return ""
			}
			return eventKeyFunc(event.Namespace, event.Name, event.Reason)
		}, kubeInformersForTargetNamespace.Core().V1().Events().Informer()).
		ToController("EventWatchController", recorder)
}

func eventKeyFunc(namespace, name, reason string) string {
	if len(namespace) == 0 || len(name) == 0 || len(reason) == 0 {
		return ""
	}
	return strings.Join([]string{namespace, name, reason}, "/")
}

func decomposeEventKey(key string) (string, string, string, bool) {
	parts := strings.Split(key, "/")
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

func (c *Controller) getEventHandler(eventKey string) *eventHandler {
	namespace, _, reason, ok := decomposeEventKey(eventKey)
	if !ok {
		return nil
	}
	for i := range c.events {
		if c.events[i].namespace == namespace && c.events[i].reason == reason {
			return &c.events[i]
		}
	}
	return nil
}

func isAcknowledgedEvent(e *corev1.Event) bool {
	if e.Annotations == nil {
		return false
	}
	lastSeenCount, ok := e.Annotations[eventAckAnnotationName]
	if !ok {
		return false
	}
	return fmt.Sprintf("%d", e.Count) == lastSeenCount
}

func (c *Controller) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	eventHandler := c.getEventHandler(syncCtx.QueueKey())
	if eventHandler == nil {
		return nil
	}

	namespace, name, _, ok := decomposeEventKey(syncCtx.QueueKey())
	if !ok {
		klog.Errorf("Unexpected queue key %q", syncCtx.QueueKey())
		return nil
	}

	event, err := c.eventClient.Events(namespace).Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		klog.Errorf("Event not found %s/%s: %v", namespace, name, err)
		return nil
	}
	if err != nil {
		return err
	}

	if isAcknowledgedEvent(event) {
		return nil
	}

	// acknowledge the event, so we won't process it multiple times
	seenEvent := event.DeepCopy()
	if seenEvent.Annotations == nil {
		seenEvent.Annotations = map[string]string{}
	}
	seenEvent.Annotations[eventAckAnnotationName] = fmt.Sprintf("%d", seenEvent.Count)
	if _, err := c.eventClient.Events(namespace).Update(ctx, seenEvent, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return eventHandler.process(seenEvent)
}

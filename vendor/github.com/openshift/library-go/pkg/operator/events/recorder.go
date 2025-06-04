package events

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

// Recorder is a simple event recording interface.
type Recorder interface {
	Event(reason, message string)
	Eventf(reason, messageFmt string, args ...interface{})
	Warning(reason, message string)
	Warningf(reason, messageFmt string, args ...interface{})

	// ForComponent allows to fiddle the component name before sending the event to sink.
	// Making more unique components will prevent the spam filter in upstream event sink from dropping
	// events.
	ForComponent(componentName string) Recorder

	// WithComponentSuffix is similar to ForComponent except it just suffix the current component name instead of overriding.
	WithComponentSuffix(componentNameSuffix string) Recorder

	// WithContext allows to set a context for event create API calls.
	WithContext(ctx context.Context) Recorder

	// ComponentName returns the current source component name for the event.
	// This allows to suffix the original component name with 'sub-component'.
	ComponentName() string

	Shutdown()
}

// podNameEnv is a name of environment variable inside container that specifies the name of the current replica set.
// This replica set name is then used as a source/involved object for operator events.
const podNameEnv = "POD_NAME"

// podNameEnvFunc allows to override the way we get the environment variable value (for unit tests).
var podNameEnvFunc = func() string {
	return os.Getenv(podNameEnv)
}

// GetControllerReferenceForCurrentPod provides an object reference to a controller managing the pod/container where this process runs.
// The pod name must be provided via the POD_NAME name.
// Even if this method returns an error, it always return valid reference to the namespace. It allows the callers to control the logging
// and decide to fail or accept the namespace.
func GetControllerReferenceForCurrentPod(ctx context.Context, client kubernetes.Interface, targetNamespace string, reference *corev1.ObjectReference) (*corev1.ObjectReference, error) {
	if reference == nil {
		// Try to get the pod name via POD_NAME environment variable
		reference := &corev1.ObjectReference{Kind: "Pod", Name: podNameEnvFunc(), Namespace: targetNamespace}
		if len(reference.Name) != 0 {
			return GetControllerReferenceForCurrentPod(ctx, client, targetNamespace, reference)
		}
		// If that fails, lets try to guess the pod by listing all pods in namespaces and using the first pod in the list
		reference, err := guessControllerReferenceForNamespace(ctx, client.CoreV1().Pods(targetNamespace))
		if err != nil {
			// If this fails, do not give up with error but instead use the namespace as controller reference for the pod
			// NOTE: This is last resort, if we see this often it might indicate something is wrong in the cluster.
			//       In some cases this might help with flakes.
			return getControllerReferenceForNamespace(targetNamespace), err
		}
		return GetControllerReferenceForCurrentPod(ctx, client, targetNamespace, reference)
	}

	switch reference.Kind {
	case "Pod":
		pod, err := client.CoreV1().Pods(reference.Namespace).Get(ctx, reference.Name, metav1.GetOptions{})
		if err != nil {
			return getControllerReferenceForNamespace(reference.Namespace), err
		}
		if podController := metav1.GetControllerOf(pod); podController != nil {
			return GetControllerReferenceForCurrentPod(ctx, client, targetNamespace, makeObjectReference(podController, targetNamespace))
		}
		// This is a bare pod without any ownerReference
		return makeObjectReference(&metav1.OwnerReference{Kind: "Pod", Name: pod.Name, UID: pod.UID, APIVersion: "v1"}, pod.Namespace), nil
	case "ReplicaSet":
		rs, err := client.AppsV1().ReplicaSets(reference.Namespace).Get(ctx, reference.Name, metav1.GetOptions{})
		if err != nil {
			return getControllerReferenceForNamespace(reference.Namespace), err
		}
		if rsController := metav1.GetControllerOf(rs); rsController != nil {
			return GetControllerReferenceForCurrentPod(ctx, client, targetNamespace, makeObjectReference(rsController, targetNamespace))
		}
		// This is a replicaSet without any ownerReference
		return reference, nil
	default:
		return reference, nil
	}
}

// getControllerReferenceForNamespace returns an object reference to the given namespace.
func getControllerReferenceForNamespace(targetNamespace string) *corev1.ObjectReference {
	return &corev1.ObjectReference{
		Kind:       "Namespace",
		Namespace:  targetNamespace,
		Name:       targetNamespace,
		APIVersion: "v1",
	}
}

// makeObjectReference makes object reference from ownerReference and target namespace
func makeObjectReference(owner *metav1.OwnerReference, targetNamespace string) *corev1.ObjectReference {
	return &corev1.ObjectReference{
		Kind:       owner.Kind,
		Namespace:  targetNamespace,
		Name:       owner.Name,
		UID:        owner.UID,
		APIVersion: owner.APIVersion,
	}
}

// guessControllerReferenceForNamespace tries to guess what resource to reference.
func guessControllerReferenceForNamespace(ctx context.Context, client corev1client.PodInterface) (*corev1.ObjectReference, error) {
	pods, err := client.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("unable to setup event recorder as %q env variable is not set and there are no pods", podNameEnv)
	}

	for _, pod := range pods.Items {
		ownerRef := metav1.GetControllerOf(&pod)
		if ownerRef == nil {
			continue
		}
		return &corev1.ObjectReference{
			Kind:       ownerRef.Kind,
			Namespace:  pod.Namespace,
			Name:       ownerRef.Name,
			UID:        ownerRef.UID,
			APIVersion: ownerRef.APIVersion,
		}, nil
	}
	return nil, errors.New("can't guess controller ref")
}

// NewRecorder returns new event recorder.
func NewRecorder(client corev1client.EventInterface, sourceComponentName string, involvedObjectRef *corev1.ObjectReference) Recorder {
	return &recorder{
		eventClient:       client,
		involvedObjectRef: involvedObjectRef,
		sourceComponent:   sourceComponentName,
	}
}

// recorder is an implementation of Recorder interface.
type recorder struct {
	eventClient       corev1client.EventInterface
	involvedObjectRef *corev1.ObjectReference
	sourceComponent   string

	// TODO: This is not the right way to pass the context, but there is no other way without breaking event interface
	ctx context.Context
}

func (r *recorder) ComponentName() string {
	return r.sourceComponent
}

func (r *recorder) Shutdown() {}

func (r *recorder) ForComponent(componentName string) Recorder {
	newRecorderForComponent := *r
	newRecorderForComponent.sourceComponent = componentName
	return &newRecorderForComponent
}

func (r *recorder) WithContext(ctx context.Context) Recorder {
	r.ctx = ctx
	return r
}

func (r *recorder) WithComponentSuffix(suffix string) Recorder {
	return r.ForComponent(fmt.Sprintf("%s-%s", r.ComponentName(), suffix))
}

// Event emits the normal type event and allow formatting of message.
func (r *recorder) Eventf(reason, messageFmt string, args ...interface{}) {
	r.Event(reason, fmt.Sprintf(messageFmt, args...))
}

// Warning emits the warning type event and allow formatting of message.
func (r *recorder) Warningf(reason, messageFmt string, args ...interface{}) {
	r.Warning(reason, fmt.Sprintf(messageFmt, args...))
}

// Event emits the normal type event.
func (r *recorder) Event(reason, message string) {
	event := makeEvent(r.involvedObjectRef, r.sourceComponent, corev1.EventTypeNormal, reason, message)
	ctx := context.Background()
	if r.ctx != nil {
		ctx = r.ctx
	}
	if _, err := r.eventClient.Create(ctx, event, metav1.CreateOptions{}); err != nil {
		klog.Warningf("Error creating event %+v: %v", event, err)
	}
}

// Warning emits the warning type event.
func (r *recorder) Warning(reason, message string) {
	event := makeEvent(r.involvedObjectRef, r.sourceComponent, corev1.EventTypeWarning, reason, message)
	ctx := context.Background()
	if r.ctx != nil {
		ctx = r.ctx
	}
	if _, err := r.eventClient.Create(ctx, event, metav1.CreateOptions{}); err != nil {
		klog.Warningf("Error creating event %+v: %v", event, err)
	}
}

func makeEvent(involvedObjRef *corev1.ObjectReference, sourceComponent string, eventType, reason, message string) *corev1.Event {
	currentTime := metav1.Time{Time: time.Now()}
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v.%x", involvedObjRef.Name, currentTime.UnixNano()),
			Namespace: involvedObjRef.Namespace,
		},
		InvolvedObject: *involvedObjRef,
		Reason:         reason,
		Message:        message,
		Type:           eventType,
		Count:          1,
		FirstTimestamp: currentTime,
		LastTimestamp:  currentTime,
	}
	event.Source.Component = sourceComponent
	return event
}

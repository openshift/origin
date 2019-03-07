package events

import (
	"fmt"
	"os"
	"time"

	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes"

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

	// ComponentName returns the current source component name for the event.
	// This allows to suffix the original component name with 'sub-component'.
	ComponentName() string
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
func GetControllerReferenceForCurrentPod(client kubernetes.Interface, targetNamespace string, reference *corev1.ObjectReference) (*corev1.ObjectReference, error) {
	if reference == nil {
		// Try to get the pod name via POD_NAME environment variable
		reference := &corev1.ObjectReference{Kind: "Pod", Name: podNameEnvFunc(), Namespace: targetNamespace}
		if len(reference.Name) != 0 {
			return GetControllerReferenceForCurrentPod(client, targetNamespace, reference)
		}
		// If that fails, lets try to guess the pod by listing all pods in namespaces and using the first pod in the list
		reference, err := guessControllerReferenceForNamespace(client.CoreV1().Pods(targetNamespace))
		if err != nil {
			return nil, err
		}
		return GetControllerReferenceForCurrentPod(client, targetNamespace, reference)
	}

	switch reference.Kind {
	case "Pod":
		pod, err := client.CoreV1().Pods(reference.Namespace).Get(reference.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if podController := metav1.GetControllerOf(pod); podController != nil {
			return GetControllerReferenceForCurrentPod(client, targetNamespace, makeObjectReference(podController, targetNamespace))
		}
		// This is a bare pod without any ownerReference
		return makeObjectReference(&metav1.OwnerReference{Kind: "Pod", Name: pod.Name, UID: pod.UID, APIVersion: "v1"}, pod.Namespace), nil
	case "ReplicaSet":
		rs, err := client.AppsV1().ReplicaSets(reference.Namespace).Get(reference.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if rsController := metav1.GetControllerOf(rs); rsController != nil {
			return GetControllerReferenceForCurrentPod(client, targetNamespace, makeObjectReference(rsController, targetNamespace))
		}
		// This is a replicaSet without any ownerReference
		return reference, nil
	default:
		return reference, nil
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
func guessControllerReferenceForNamespace(client corev1client.PodInterface) (*corev1.ObjectReference, error) {
	pods, err := client.List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("unable to setup event recorder as %q env variable is not set and there are no pods", podNameEnv)
	}

	pod := &pods.Items[0]
	ownerRef := metav1.GetControllerOf(pod)
	return &corev1.ObjectReference{
		Kind:       ownerRef.Kind,
		Namespace:  pod.Namespace,
		Name:       ownerRef.Name,
		UID:        ownerRef.UID,
		APIVersion: ownerRef.APIVersion,
	}, nil
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
}

func (r *recorder) ComponentName() string {
	return r.sourceComponent
}

func (r *recorder) ForComponent(componentName string) Recorder {
	newRecorderForComponent := *r
	newRecorderForComponent.sourceComponent = componentName
	return &newRecorderForComponent
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
	if _, err := r.eventClient.Create(event); err != nil {
		glog.Warningf("Error creating event %+v: %v", event, err)
	}
}

// Warning emits the warning type event.
func (r *recorder) Warning(reason, message string) {
	event := makeEvent(r.involvedObjectRef, r.sourceComponent, corev1.EventTypeWarning, reason, message)
	if _, err := r.eventClient.Create(event); err != nil {
		glog.Warningf("Error creating event %+v: %v", event, err)
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

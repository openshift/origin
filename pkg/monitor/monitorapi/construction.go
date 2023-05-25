package monitorapi

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/kube-openapi/pkg/util/sets"
)

type EventBuilder struct {
	level             EventLevel
	structuredLocator StructuredLocator
	structuredMessage StructuredMessage
}

func Event(level EventLevel) *EventBuilder {
	return &EventBuilder{
		level: level,
	}
}

func (b *EventBuilder) Event() Condition {
	ret := Condition{
		Level:             b.level,
		Locator:           b.structuredLocator.OldLocator(),
		StructuredLocator: b.structuredLocator,
		Message:           b.structuredMessage.OldMessage(),
		StructuredMessage: b.structuredMessage,
	}

	return ret
}

func (b *EventBuilder) Message(message StructuredMessage) *EventBuilder {
	b.structuredMessage = message
	return b
}

func (b *EventBuilder) Locator(locator StructuredLocator) *EventBuilder {
	b.structuredLocator = locator
	return b
}

type LocatorBuilder struct {
	targetType  StructuredType
	annotations map[LocatorKey]string
}

func Locator() *LocatorBuilder {
	return &LocatorBuilder{
		annotations: map[LocatorKey]string{},
	}
}

func LocateNodeFromName(nodeName string) *LocatorBuilder {
	ret := &LocatorBuilder{
		targetType: StructuredTypeNode,
		annotations: map[LocatorKey]string{
			LocatorNodeKey: nodeName,
		},
	}

	return ret
}

func LocateContainerFromPod(pod *corev1.Pod, containerName string) *LocatorBuilder {
	ret := LocatePodFromPod(pod)
	ret.targetType = StructuredTypeContainer
	ret.annotations[LocatorContainerKey] = containerName

	return ret
}

func LocateContainerFromNames(namespace, podName, uid, containerName string) *LocatorBuilder {
	ret := LocatePodFromNames(namespace, podName, uid)
	ret.targetType = StructuredTypeContainer
	ret.annotations[LocatorContainerKey] = containerName

	return ret
}

func LocatePodFromNames(namespace, name, uid string) *LocatorBuilder {
	ret := &LocatorBuilder{
		targetType: StructuredTypePod,
		annotations: map[LocatorKey]string{
			LocatorNamespaceKey: namespace,
			LocatorPodKey:       name,
			LocatorUIDKey:       uid,
		},
	}

	return ret
}

func LocatePodFromPod(pod *corev1.Pod) *LocatorBuilder {
	ret := LocatePodFromNames(pod.Namespace, pod.Name, string(pod.UID))
	// TODO, to be removed.  this should be in the message, not in the locator
	if len(pod.Spec.NodeName) > 0 {
		ret.annotations[LocatorNodeKey] = pod.Spec.NodeName
	}
	if mirrorUID := pod.Annotations["kubernetes.io/config.mirror"]; len(mirrorUID) > 0 {
		ret.annotations[LocatorMirrorUIDKey] = mirrorUID
	}

	return ret
}

func (m *LocatorBuilder) Structured() StructuredLocator {
	ret := StructuredLocator{
		Type:        m.targetType,
		LocatorKeys: map[LocatorKey]string{},
	}
	for k, v := range m.annotations {
		ret.LocatorKeys[k] = v
	}
	return ret
}

type MessageBuilder struct {
	annotations     map[AnnotationKey]string
	originalMessage string
}

func Message() *MessageBuilder {
	return &MessageBuilder{
		annotations: map[AnnotationKey]string{},
	}
}

func ExpandMessage(prevMessage string) *MessageBuilder {
	prevAnnotations := AnnotationsFromMessage(prevMessage)
	prevNonAnnotationMessage := NonAnnotationMessage(prevMessage)
	return &MessageBuilder{
		annotations:     prevAnnotations,
		originalMessage: prevNonAnnotationMessage,
	}
}

func (m *MessageBuilder) Reason(reason IntervalReason) *MessageBuilder {
	return m.WithAnnotation(AnnotationReason, string(reason))
}

func (m *MessageBuilder) Cause(cause string) *MessageBuilder {
	return m.WithAnnotation(AnnotationCause, cause)
}

func (m *MessageBuilder) Node(node string) *MessageBuilder {
	return m.WithAnnotation(AnnotationNode, node)
}

func (m *MessageBuilder) Constructed() *MessageBuilder {
	return m.WithAnnotation(AnnotationConstructed, "true")
}

func (m *MessageBuilder) WithAnnotation(name AnnotationKey, value string) *MessageBuilder {
	m.annotations[name] = value
	return m
}

func (m *MessageBuilder) WithAnnotations(annotations map[AnnotationKey]string) *MessageBuilder {
	for k, v := range annotations {
		m.annotations[k] = v
	}
	return m
}

func (m *MessageBuilder) NoDetails() string {
	keys := sets.NewString()
	for k := range m.annotations {
		keys.Insert(string(k))
	}

	annotations := []string{}
	for _, k := range keys.List() {
		v := m.annotations[AnnotationKey(k)]
		annotations = append(annotations, fmt.Sprintf("%v/%v", k, v))
	}
	annotationString := strings.Join(annotations, " ")
	return m.appendPreviousMessage(annotationString)
}

func (m *MessageBuilder) Messagef(messageFormat string, args ...interface{}) string {
	return m.Message(fmt.Sprintf(messageFormat, args...))
}

func (m *MessageBuilder) Message(message string) string {
	if len(message) == 0 {
		return m.NoDetails()
	}
	annotationString := m.NoDetails()
	return m.appendPreviousMessage(fmt.Sprintf("%v %v", annotationString, message))
}

func (m *MessageBuilder) StructuredNoDetails() StructuredMessage {
	ret := StructuredMessage{
		Annotations: map[AnnotationKey]string{},
	}
	for k, v := range m.annotations {
		ret.Annotations[k] = v
		switch k {
		case AnnotationReason:
			ret.Reason = IntervalReason(v)
		case AnnotationCause:
			ret.Cause = v
		}
	}
	return ret
}

func (m *MessageBuilder) StructuredMessagef(messageFormat string, args ...interface{}) StructuredMessage {
	return m.StructuredMessage(fmt.Sprintf(messageFormat, args...))
}

func (m *MessageBuilder) StructuredMessage(message string) StructuredMessage {
	if len(message) == 0 {
		return m.StructuredNoDetails()
	}
	ret := m.StructuredNoDetails()
	ret.HumanMessage = message
	return ret
}

func (m *MessageBuilder) appendPreviousMessage(newMessage string) string {
	if len(m.originalMessage) == 0 {
		return newMessage
	}
	return fmt.Sprintf("%v %v", m.originalMessage, newMessage)
}

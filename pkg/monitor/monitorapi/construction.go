package monitorapi

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/kube-openapi/pkg/util/sets"
)

type ConditionBuilder struct {
	level             ConditionLevel
	structuredLocator Locator
	structuredMessage Message
}

func NewCondition(level ConditionLevel) *ConditionBuilder {
	return &ConditionBuilder{
		level: level,
	}
}

func (b *ConditionBuilder) Build() Condition {
	ret := Condition{
		Level:             b.level,
		Locator:           b.structuredLocator.OldLocator(),
		StructuredLocator: b.structuredLocator,
		Message:           b.structuredMessage.OldMessage(),
		StructuredMessage: b.structuredMessage,
	}

	return ret
}

func (b *ConditionBuilder) Message(mb *MessageBuilder) *ConditionBuilder {
	b.structuredMessage = mb.build()
	return b
}

func (b *ConditionBuilder) Locator(lb *LocatorBuilder) *ConditionBuilder {
	b.structuredLocator = lb.Build()
	return b
}

type LocatorBuilder struct {
	targetType  LocatorType
	annotations map[LocatorKey]string
}

func NewLocator() *LocatorBuilder {
	return &LocatorBuilder{
		annotations: map[LocatorKey]string{},
	}
}

func (b *LocatorBuilder) NodeFromName(nodeName string) *LocatorBuilder {
	b.targetType = LocatorTypeNode
	b.annotations[LocatorNodeKey] = nodeName
	return b
}

func (b *LocatorBuilder) ContainerFromPod(pod *corev1.Pod, containerName string) *LocatorBuilder {
	b.PodFromPod(pod)
	b.targetType = LocatorTypeContainer
	b.annotations[LocatorContainerKey] = containerName
	return b
}

func (b *LocatorBuilder) ContainerFromNames(namespace, podName, uid, containerName string) *LocatorBuilder {
	b.PodFromNames(namespace, podName, uid)
	b.targetType = LocatorTypeContainer
	b.annotations[LocatorContainerKey] = containerName
	return b
}

func (b *LocatorBuilder) PodFromNames(namespace, podName, uid string) *LocatorBuilder {
	b.targetType = LocatorTypePod
	b.annotations[LocatorNamespaceKey] = namespace
	b.annotations[LocatorPodKey] = podName
	b.annotations[LocatorUIDKey] = uid

	return b
}

func (b *LocatorBuilder) PodFromPod(pod *corev1.Pod) *LocatorBuilder {
	b.PodFromNames(pod.Namespace, pod.Name, string(pod.UID))
	// TODO, to be removed.  this should be in the message, not in the locator
	if len(pod.Spec.NodeName) > 0 {
		b.annotations[LocatorNodeKey] = pod.Spec.NodeName
	}
	if mirrorUID := pod.Annotations["kubernetes.io/config.mirror"]; len(mirrorUID) > 0 {
		b.annotations[LocatorMirrorUIDKey] = mirrorUID
	}

	return b
}

func (b *LocatorBuilder) Build() Locator {
	ret := Locator{
		Type:        b.targetType,
		LocatorKeys: map[LocatorKey]string{},
	}
	for k, v := range b.annotations {
		ret.LocatorKeys[k] = v
	}
	return ret
}

type MessageBuilder struct {
	annotations  map[AnnotationKey]string
	humanMessage string
}

func NewMessage() *MessageBuilder {
	return &MessageBuilder{
		annotations: map[AnnotationKey]string{},
	}
}

// ExpandMessage parses a message that was collapsed into a string to extract each annotation
// and the original message.
func ExpandMessage(prevMessage string) *MessageBuilder {
	prevAnnotations := AnnotationsFromMessage(prevMessage)
	prevNonAnnotationMessage := NonAnnotationMessage(prevMessage)
	return &MessageBuilder{
		annotations:  prevAnnotations,
		humanMessage: prevNonAnnotationMessage,
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

func (m *MessageBuilder) Constructed(constructedBy ConstructionOwner) *MessageBuilder {
	return m.WithAnnotation(AnnotationConstructed, string(constructedBy))
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

// HumanMessage adds the human readable message. If called multiple times, the message is appended.
func (m *MessageBuilder) HumanMessage(message string) *MessageBuilder {
	if len(m.humanMessage) == 0 {
		m.humanMessage = message
		return m
	}
	// TODO: track a slice of human messages? we are aiming for structure here...
	m.humanMessage = fmt.Sprintf("%v %v", m.humanMessage, message)
	return m
}

// HumanMessagef adds a formatted string to the human readable message. If called multiple times, the message is appended.
func (m *MessageBuilder) HumanMessagef(messageFormat string, args ...interface{}) *MessageBuilder {
	return m.HumanMessage(fmt.Sprintf(messageFormat, args...))
}

// build creates the final StructuredMessage with all data assembled by this builder.
func (m *MessageBuilder) build() Message {
	ret := Message{
		Annotations: map[AnnotationKey]string{},
	}
	// TODO: what do we gain from a mStructuredMessage with fixed keys, vs fields on the StructuredMessage?
	// They're not really fixed, some WithAnnotation calls are floating around, but could those also be functions here?
	for k, v := range m.annotations {
		ret.Annotations[k] = v
		switch k {
		case AnnotationReason:
			ret.Reason = IntervalReason(v)
		case AnnotationCause:
			ret.Cause = v
		}
	}
	ret.HumanMessage = m.humanMessage
	return ret
}

// BuildString creates the final message as a flat single string.
// Each annotation is prepended in the form name/value, followed by the human message, if any.
func (m *MessageBuilder) BuildString() string {
	keys := sets.NewString()
	for k := range m.annotations {
		keys.Insert(string(k))
	}

	annotations := []string{}
	for _, k := range keys.List() {
		v := m.annotations[AnnotationKey(k)]
		annotations = append(annotations, fmt.Sprintf("%v/%v", k, v))
	}
	retString := strings.Join(annotations, " ")

	if len(m.humanMessage) > 0 {
		retString = fmt.Sprintf("%v %v", retString, m.humanMessage)
	}
	return retString
}

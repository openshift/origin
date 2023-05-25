package monitorapi

import (
	"fmt"
	"strings"

	"k8s.io/kube-openapi/pkg/util/sets"
)

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

func (m *MessageBuilder) Reason(reason EventReason) *MessageBuilder {
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

func (m *MessageBuilder) appendPreviousMessage(newMessage string) string {
	if len(m.originalMessage) == 0 {
		return newMessage
	}
	return fmt.Sprintf("%v %v", m.originalMessage, newMessage)
}

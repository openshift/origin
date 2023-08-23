package monitorapi

import (
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/kube-openapi/pkg/util/sets"
)

type IntervalBuilder struct {
	level             IntervalLevel
	source            IntervalSource
	display           bool
	structuredLocator Locator
	structuredMessage Message
}

func NewInterval(source IntervalSource, level IntervalLevel) *IntervalBuilder {
	return &IntervalBuilder{
		level:  level,
		source: source,
	}
}

// Display is a coarse grained hint that any UI should display this interval to a user.
func (b *IntervalBuilder) Display() *IntervalBuilder {
	b.display = true
	return b
}

// Deprecated: Use Build for a full Interval, we hope to remove Condition entirely and bubble it up into Interval
// directly.
func (b *IntervalBuilder) BuildCondition() Condition {
	ret := Condition{
		Level:             b.level,
		Locator:           b.structuredLocator.OldLocator(),
		StructuredLocator: b.structuredLocator,
		Message:           b.structuredMessage.OldMessage(),
		StructuredMessage: b.structuredMessage,
	}

	return ret
}

// Build creates the final interval with a mandatory from/to timestamp.
// Zero value times are allowed to indicate cases when the from or to is not known.
// This situation happens when the monitor doesn't observe a state change: imagine intervals for tracking graceful shutdown.
// If the monitor is stopped before the shutdown completes, it is incorrect to indicate a To time of "now".
// It is accurate to indicate "open" by using a zero value time.
func (b *IntervalBuilder) Build(from, to time.Time) Interval {
	ret := Interval{
		Condition: b.BuildCondition(),
		Display:   b.display,
		Source:    b.source,
		From:      from,
		To:        to,
	}

	return ret
}

func (b *IntervalBuilder) Message(mb *MessageBuilder) *IntervalBuilder {
	b.structuredMessage = mb.build()
	return b
}

func (b *IntervalBuilder) Locator(locator Locator) *IntervalBuilder {
	b.structuredLocator = locator
	return b
}

// LocatorBuilder is used to create locators. We do not want to allow chaining of locators however
// as this has led to illegal definitions of locators in the past. (such as node + namespace)
// Instead the builder serves primarily as a place to store the builder functions.
type LocatorBuilder struct {
	targetType  LocatorType
	annotations map[LocatorKey]string
}

func NewLocator() *LocatorBuilder {
	return &LocatorBuilder{
		annotations: map[LocatorKey]string{},
	}
}

func (b *LocatorBuilder) NodeFromName(nodeName string) Locator {
	return b.
		withTargetType(LocatorTypeNode).
		withNode(nodeName).
		Build()
}

func (b *LocatorBuilder) AlertFromNames(alertName, node, namespace, pod, container string) Locator {
	b.targetType = LocatorTypeAlert
	if len(alertName) > 0 {
		b.annotations[LocatorAlertKey] = alertName
	}
	if len(node) > 0 {
		b.annotations[LocatorNodeKey] = node
	}
	if len(namespace) > 0 {
		b.annotations[LocatorNamespaceKey] = namespace
	}
	if len(pod) > 0 {
		b.annotations[LocatorPodKey] = pod
	}
	if len(container) > 0 {
		b.annotations[LocatorContainerKey] = container
	}
	return b.Build()
}

func (b *LocatorBuilder) Disruption(backendDisruptionName, thisInstanceName, loadBalancer, protocol, target string, connectionType BackendConnectionType) Locator {
	b = b.withDisruptionRequiredOnly(backendDisruptionName, thisInstanceName).withConnectionType(connectionType)

	if len(loadBalancer) > 0 {
		b.annotations[LocatorLoadBalancerKey] = loadBalancer
	}
	if len(protocol) > 0 {
		b.annotations[LocatorProtocolKey] = protocol
	}
	if len(target) > 0 {
		b.annotations[LocatorTargetKey] = target
	}
	return b.Build()
}

// DisruptionRequiredOnly takes only the logically required data for backend-disruption.json and codifies it.
// backendDisruptionName is the value used to store and locate historical data related to the amount of disruption.
// thisInstanceName is used to show on a timeline which connection failed.
// For instance, the backendDisruptionName may be internal-load-balancer and the thisInstanceName may include,
// 1. from worker-a
// 2. new connections
// 3. to IP
// 4. this protocol
func (b *LocatorBuilder) DisruptionRequiredOnly(backendDisruptionName, thisInstanceName string) Locator {
	b = b.withDisruptionRequiredOnly(backendDisruptionName, thisInstanceName)
	return b.Build()
}

func (b *LocatorBuilder) withDisruptionRequiredOnly(backendDisruptionName, thisInstanceName string) *LocatorBuilder {
	b.targetType = LocatorTypeDisruption
	b.annotations[LocatorBackendDisruptionNameKey] = backendDisruptionName
	b.annotations[LocatorDisruptionKey] = thisInstanceName
	return b
}

func (b *LocatorBuilder) LocateNamespace(namespaceName string) Locator {
	return b.
		withNamespace(namespaceName).
		Build()
}

func (b *LocatorBuilder) withNamespace(namespace string) *LocatorBuilder {
	b.annotations[LocatorNamespaceKey] = namespace
	return b
}

func (b *LocatorBuilder) withNode(nodeName string) *LocatorBuilder {
	b.annotations[LocatorNodeKey] = nodeName
	return b
}

func (b *LocatorBuilder) withRoute(route string) *LocatorBuilder {
	b.annotations[LocatorRouteKey] = route
	return b
}

func (b *LocatorBuilder) withTargetType(targetType LocatorType) *LocatorBuilder {
	b.targetType = targetType
	return b
}

func (b *LocatorBuilder) withConnectionType(connectionType BackendConnectionType) *LocatorBuilder {
	b.annotations[LocatorConnectionKey] = string(connectionType)
	return b
}

func (b *LocatorBuilder) LocateRouteForDisruptionCheck(backendDisruptionName, thisInstanceName, ns, name string, connectionType BackendConnectionType) Locator {
	return b.
		withDisruptionRequiredOnly(backendDisruptionName, thisInstanceName).
		withNamespace(ns).
		withRoute(name).
		withConnectionType(connectionType).
		Build()
}

func (b *LocatorBuilder) LocateDisruptionCheck(backendDisruptionName, thisInstanceName string, connectionType BackendConnectionType) Locator {
	return b.
		withDisruptionRequiredOnly(backendDisruptionName, thisInstanceName).
		withConnectionType(connectionType).
		Build()
}

func (b *LocatorBuilder) LocateServer(serverName, nodeName, namespace, podName string, isShutdown bool) Locator {
	if isShutdown {
		return b.
			withShutdown().
			withServer(serverName).
			withNode(nodeName).
			withNamespace(namespace).
			withPodName(podName).
			Build()
	}
	return b.
		withServer(serverName).
		withNode(nodeName).
		withNamespace(namespace).
		withPodName(podName).
		Build()
}

// TODO remove this once we know what all breaks.
func (b *LocatorBuilder) withShutdown() *LocatorBuilder {
	b.annotations[LocatorShutdown] = "apiserver"
	return b
}

func (b *LocatorBuilder) withServer(serverName string) *LocatorBuilder {
	b.annotations[LocatorServerKey] = serverName
	return b
}

func (b *LocatorBuilder) KubeEvent(event *corev1.Event) Locator {
	b.targetType = LocatorTypeKubeEvent

	// WARNING: we're trying to use an enum for the locator keys, but we cannot know
	// all kinds in a cluster. Instead we'll split the kind and name into two different Keys
	// for Events:
	b.annotations[LocatorKindKey] = event.InvolvedObject.Kind
	b.annotations[LocatorNameKey] = event.InvolvedObject.Name

	if len(event.InvolvedObject.Namespace) > 0 {
		b.annotations[LocatorNamespaceKey] = event.InvolvedObject.Namespace
	}

	// TODO: node + namespace is illegal, look at original impl, it may have handled this better
	if len(event.Source.Host) > 0 && event.InvolvedObject.Kind != "Node" {
		b.annotations[LocatorNodeKey] = event.Source.Host
	}

	return b.Build()
}

func (b *LocatorBuilder) APIServerShutdown(loadBalancer string) Locator {
	b.targetType = LocatorTypeAPIServerShutdown
	b.annotations[LocatorShutdownKey] = "graceful"
	b.annotations[LocatorServerKey] = "kube-apiserver"
	if len(loadBalancer) > 0 {
		b.annotations[LocatorLoadBalancerKey] = loadBalancer
	}
	return b.Build()
}

func (b *LocatorBuilder) ContainerFromPod(pod *corev1.Pod, containerName string) Locator {
	b.PodFromPod(pod)
	b.targetType = LocatorTypeContainer
	b.annotations[LocatorContainerKey] = containerName
	return b.Build()
}

func (b *LocatorBuilder) ContainerFromNames(namespace, podName, uid, containerName string) Locator {
	b.PodFromNames(namespace, podName, uid)
	b.targetType = LocatorTypeContainer
	b.annotations[LocatorContainerKey] = containerName
	return b.Build()
}

func (b *LocatorBuilder) PodFromNames(namespace, podName, uid string) Locator {
	return b.
		withTargetType(LocatorTypePod).
		withNamespace(namespace).
		withPodName(podName).
		withUID(uid).
		Build()
}

func (b *LocatorBuilder) withPodName(podName string) *LocatorBuilder {
	b.annotations[LocatorPodKey] = podName
	return b
}

func (b *LocatorBuilder) withUID(uid string) *LocatorBuilder {
	b.annotations[LocatorUIDKey] = uid
	return b
}

func (b *LocatorBuilder) PodFromPod(pod *corev1.Pod) Locator {
	b.PodFromNames(pod.Namespace, pod.Name, string(pod.UID))
	// TODO, to be removed.  this should be in the message, not in the locator
	if len(pod.Spec.NodeName) > 0 {
		b.annotations[LocatorNodeKey] = pod.Spec.NodeName
	}
	if mirrorUID := pod.Annotations["kubernetes.io/config.mirror"]; len(mirrorUID) > 0 {
		b.annotations[LocatorMirrorUIDKey] = mirrorUID
	}

	return b.Build()
}

func (b *LocatorBuilder) E2ETest(testName string) Locator {
	b.targetType = LocatorTypeE2ETest
	b.annotations[LocatorE2ETestKey] = testName
	return b.Build()
}

func (b *LocatorBuilder) Build() Locator {
	ret := Locator{
		Type: b.targetType,
		Keys: map[LocatorKey]string{},
	}
	for k, v := range b.annotations {
		ret.Keys[k] = v
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

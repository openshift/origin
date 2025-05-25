package monitorapi

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	v1 "github.com/openshift/api/config/v1"
	"github.com/prometheus/common/model"
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

// NewInterval creates a new interval builder. Source is an indicator of what created this interval, used for
// safely identifying intervals we're looking for, and for grouping in charts.
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
		Level:   b.level,
		Locator: b.structuredLocator,
		Message: b.structuredMessage,
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

// BuildNow creates the final interval with a from/to timestamp of now.
func (b *IntervalBuilder) BuildNow() Interval {
	now := time.Now()
	ret := Interval{
		Condition: b.BuildCondition(),
		Display:   b.display,
		Source:    b.source,
		From:      now,
		To:        now,
	}

	return ret
}

func (b *IntervalBuilder) Message(mb *MessageBuilder) *IntervalBuilder {
	b.structuredMessage = mb.Build()
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

func (b *LocatorBuilder) DeploymentFromName(namespace, deploymentName string) Locator {
	return b.
		withTargetType(LocatorTypeDeployment).
		withNamespace(namespace).
		withDeployment(deploymentName).
		Build()
}

func (b *LocatorBuilder) DaemonSetFromName(namespace, daemonSetName string) Locator {
	return b.
		withTargetType(LocatorTypeDaemonSet).
		withNamespace(namespace).
		withDaemonSet(daemonSetName).
		Build()
}

func (b *LocatorBuilder) StatefulSetFromName(namespace, statefulSetName string) Locator {
	return b.
		withTargetType(LocatorTypeStatefulSet).
		withNamespace(namespace).
		withStatefuulSet(statefulSetName).
		Build()
}

func (b *LocatorBuilder) MachineFromName(machineName string) Locator {
	return b.
		withTargetType(LocatorTypeMachine).
		withMachine(machineName).
		Build()
}

func (b *LocatorBuilder) NodeFromNameWithRow(nodeName, row string) Locator {
	return b.
		withTargetType(LocatorTypeNode).
		withNode(nodeName).
		withRow(row).
		Build()
}

func (b *LocatorBuilder) CloudNodeMetric(nodeName string, metric string) Locator {
	return b.
		withTargetType(LocatorTypeCloudMetrics).
		withNode(nodeName).
		withMetric(metric).
		Build()
}

func (b *LocatorBuilder) ClusterVersion(cv *v1.ClusterVersion) Locator {
	b.targetType = LocatorTypeClusterVersion
	b.annotations[LocatorClusterVersionKey] = cv.Name
	return b.Build()
}

func (b *LocatorBuilder) AlertFromPromSampleStream(alert *model.SampleStream) Locator {
	b.targetType = LocatorTypeAlert

	alertName := string(alert.Metric[model.AlertNameLabel])
	if len(alertName) > 0 {
		b.annotations[LocatorAlertKey] = alertName
	}
	node := string(alert.Metric["instance"])
	if len(node) > 0 {
		b.annotations[LocatorNodeKey] = node
	}
	namespace := string(alert.Metric["namespace"])
	if len(namespace) > 0 {
		b.annotations[LocatorNamespaceKey] = namespace
	}
	pod := string(alert.Metric["pod"])
	if len(pod) > 0 {
		b.annotations[LocatorPodKey] = pod
	}
	container := string(alert.Metric["container"])
	if len(container) > 0 {
		b.annotations[LocatorContainerKey] = container
	}

	// Some alerts include a very useful name field, ClusterOperator[Down|Degraded] for example,
	// always comes from the namespace openshift-cluster-version, but this field is the actual
	// name of the operator that was detected to be down. This is very useful for locators and
	// analysis.
	additionalName := string(alert.Metric["name"])
	if len(additionalName) > 0 {
		b.annotations[LocatorNameKey] = additionalName
	}

	return b.Build()
}

func (b *LocatorBuilder) PrometheusTargetDownFromPromSampleStream(sample *model.SampleStream) Locator {
	b.targetType = LocatorTypeMetricsEndpoint

	node := string(sample.Metric["node"])
	if len(node) > 0 {
		b.annotations[LocatorNodeKey] = node
	}
	namespace := string(sample.Metric["namespace"])
	if len(namespace) > 0 {
		b.annotations[LocatorNamespaceKey] = namespace
	}
	instance := string(sample.Metric["instance"])
	if len(instance) > 0 {
		b.annotations[LocatorInstanceKey] = instance
	}
	metricsPath := string(sample.Metric["metrics_path"])
	if len(metricsPath) > 0 {
		b.annotations[LocatorMetricsPathKey] = metricsPath
	}
	service := string(sample.Metric["service"])
	if len(service) > 0 {
		b.annotations[LocatorServiceKey] = service
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

func (b *LocatorBuilder) withDeployment(deploymentName string) *LocatorBuilder {
	b.annotations[LocatorDeploymentKey] = deploymentName
	return b
}

func (b *LocatorBuilder) withDaemonSet(daemonSetName string) *LocatorBuilder {
	b.annotations[LocatorDaemonSetKey] = daemonSetName
	return b
}
func (b *LocatorBuilder) withStatefuulSet(statefulSetName string) *LocatorBuilder {
	b.annotations[LocatorStatefulSetKey] = statefulSetName
	return b
}

func (b *LocatorBuilder) withMachine(machineName string) *LocatorBuilder {
	b.annotations[LocatorMachineKey] = machineName
	return b
}

func (b *LocatorBuilder) withRow(row string) *LocatorBuilder {
	b.annotations[LocatorRowKey] = row
	return b
}

func (b *LocatorBuilder) withMetric(metricName string) *LocatorBuilder {
	b.annotations[LocatorMetricKey] = metricName
	return b
}

func (b *LocatorBuilder) withEtcdMember(memberName string) *LocatorBuilder {
	b.annotations[LocatorEtcdMemberKey] = memberName
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

func (b *LocatorBuilder) LocateServer(serverName, nodeName, namespace, podName string) Locator {
	return b.
		withServer(serverName).
		withNode(nodeName).
		withNamespace(namespace).
		withPodName(podName).
		Build()
}

func (b *LocatorBuilder) withServer(serverName string) *LocatorBuilder {
	b.annotations[LocatorServerKey] = serverName
	return b
}

func (b *LocatorBuilder) KubeAPIServerWithLB(loadBalancer string) Locator {
	b.targetType = LocatorTypeAPIServer
	b.annotations[LocatorServerKey] = "kube-apiserver"
	if len(loadBalancer) > 0 {
		b.annotations[LocatorLoadBalancerKey] = loadBalancer
	}
	return b.Build()
}

func (b *LocatorBuilder) WithAPIUnreachableFromClient(metric model.Metric, serviceNetworkIP, nodeName, nodeRole string) Locator {
	// the label 'host' is the endpoint used to contact the kube-apiserver
	getHost := func(metric model.Metric, serviceNetworkIP string) string {
		host := string(metric["host"])
		switch {
		case strings.HasPrefix(host, serviceNetworkIP):
			return "service-network"
		case strings.HasPrefix(host, "api-int."):
			return "internal-lb"
		case strings.HasPrefix(host, "api."):
			return "external-lb"
		case strings.HasPrefix(host, "[::1]:6443") || strings.HasPrefix(host, "localhost:6443"):
			return "localhost"
		// these are probably the apiserver trying to contact the e2e test webhooks
		case strings.HasPrefix(host, "e2e-test-webhook.e2e-webhook"):
			return "e2e-test-webhooks"
		}
		return host
	}

	b.targetType = LocatorTypeAPIUnreachableFromClient
	if host := getHost(metric, serviceNetworkIP); len(host) > 0 {
		b.annotations[LocatorAPIUnreachableHostKey] = host
	}
	if job := string(metric["job"]); len(job) > 0 {
		b.annotations[LocatorAPIUnreachableComponentKey] = job
	}

	b.annotations[LocatorNodeKey] = nodeName
	b.annotations[LocatorNodeRoleKey] = nodeRole
	return b.Build()
}

func (b *LocatorBuilder) WithEtcdDiskFsyncMetric(metric model.Metric) Locator {
	pod := string(metric["pod"])
	b.targetType = LocatorTypePod
	b.withPodName(pod)
	return b.Build()
}

// TODO decide whether we want to allow "random" locator keys.  deads2k is -1 on random locator keys and thinks we should enumerate every possible key we special case.
func (b *LocatorBuilder) KubeEvent(event *corev1.Event) Locator {

	// When Kube Events are displayed, we need repeats of the same event to appear on one line. To do this
	// we hash the event message, get the first ten characters, and add it as a hmsg locator key.
	hash := sha256.Sum256([]byte(event.Message))
	hashStr := fmt.Sprintf("%x", hash)[:10]
	b.annotations[LocatorHmsgKey] = hashStr

	if event.InvolvedObject.Kind == "Namespace" {
		// namespace better match the event itself.
		return b.
			withNamespace(event.InvolvedObject.Name).
			Build()
	} else if event.InvolvedObject.Kind == "Node" {
		return b.
			withTargetType(LocatorTypeNode).
			withNode(event.InvolvedObject.Name).
			Build()
	}

	// Otherwise we have to fall back to a generic "Kind" locator, likely what deads2k refers to above as sketchy.
	// For now just preserving the old logic.
	b.targetType = LocatorTypeKind
	b.annotations[LocatorKey(strings.ToLower(event.InvolvedObject.Kind))] = event.InvolvedObject.Name
	if len(event.Source.Host) > 0 && event.Source.Component == "kubelet" {
		b.annotations[LocatorNodeKey] = event.Source.Host
	}
	if len(event.InvolvedObject.Namespace) > 0 {
		b.annotations[LocatorNamespaceKey] = event.InvolvedObject.Namespace
	}
	return b.Build()
}

// KubeletSyncLoopProbe constructs a locator from a Kubelet SyncLoop
// probe event, typically kubelet log prints the events as follows:
// "SyncLoop (probe)" probe="readiness" status="ready" pod="openshift-etcd/etcd-ci-op-bzbjn2bk-206af-gfdsw-master-2"
func (b *LocatorBuilder) KubeletSyncLoopProbe(node, ns, podName, probeType string) Locator {
	b.targetType = LocatorTypeKubeletSyncLoopProbe
	b.withNode(node).
		withNamespace(ns).
		withPodName(podName)
	b.annotations[LocatorTypeKubeletSyncLoopProbeType] = probeType
	return b.Build()
}

func (b *LocatorBuilder) KubeletSyncLoopPLEG(node, ns, podName, eventType string) Locator {
	b.targetType = LocatorTypeKubeletSyncLoopPLEG
	b.withNode(node).
		withNamespace(ns).
		withPodName(podName)
	b.annotations[LocatorTypeKubeletSyncLoopPLEGType] = eventType
	return b.Build()
}

func (b *LocatorBuilder) StaticPodInstall(node, podType string) Locator {
	b.targetType = LocatorTypeStaticPodInstall
	b.withNode(node)
	b.annotations[LocatorStaticPodInstallType] = podType
	return b.Build()
}

func (b *LocatorBuilder) ContainerFromPod(pod *corev1.Pod, containerName string) Locator {
	b.PodFromPod(pod)
	b.targetType = LocatorTypeContainer
	b.annotations[LocatorContainerKey] = containerName
	return b.Build()
}

func (b *LocatorBuilder) EtcdMemberFromNames(nodeName, memberName string) Locator {
	return b.
		withNode(nodeName).
		withEtcdMember(memberName).
		Build()
}

func (b *LocatorBuilder) ContainerFromNames(namespace, podName, uid, containerName string) Locator {
	b.PodFromNames(namespace, podName, uid)
	b.targetType = LocatorTypeContainer
	b.annotations[LocatorContainerKey] = containerName
	return b.Build()
}

func (b *LocatorBuilder) PodFromNames(namespace, podName, uid string) Locator {
	bldr := b.withTargetType(LocatorTypePod)
	if len(namespace) > 0 {
		bldr = bldr.withNamespace(namespace)
	}
	if len(podName) > 0 {
		bldr = bldr.withPodName(podName)
	}
	if len(uid) > 0 {
		bldr = bldr.withUID(uid)
	}
	return bldr.Build()
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

func (b *LocatorBuilder) ClusterOperator(name string) Locator {
	b.targetType = LocatorTypeClusterOperator
	b.annotations[LocatorClusterOperatorKey] = name
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
func ExpandMessage(prevMessage Message) *MessageBuilder {
	prevAnnotations := prevMessage.Annotations
	prevNonAnnotationMessage := prevMessage.HumanMessage
	if prevAnnotations == nil {
		prevAnnotations = map[AnnotationKey]string{}
	}
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

// Build creates the final Message with all data assembled by this builder.
func (m *MessageBuilder) Build() Message {
	ret := Message{
		Annotations: map[AnnotationKey]string{},
	}
	// TODO: what do we gain from a mStructuredMessage with fixed keys, vs fields on the Message?
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

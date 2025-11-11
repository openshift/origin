package monitorapi

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
)

// TODO most consumers only need writers or readers.  switch.
type Recorder interface {
	RecorderReader
	RecorderWriter
}

type RecorderReader interface {
	// Intervals returns a sorted snapshot of intervals in the selected timeframe
	Intervals(from, to time.Time) Intervals
	// CurrentResourceState returns a list of all known resources of a given type at the instant called.
	CurrentResourceState() ResourcesMap
}

type RecorderWriter interface {
	// RecordResource stores a resource for later serialization.  Deletion is not tracked, so this can be used
	// to determine the final state of resource that are deleted in a namespace.
	// Annotations are added to indicate number of updates and the number of recreates.
	RecordResource(resourceType string, obj runtime.Object)

	Record(conditions ...Condition)
	RecordAt(t time.Time, conditions ...Condition)

	AddIntervals(eventIntervals ...Interval)
	StartInterval(interval Interval) int
	EndInterval(startedInterval int, t time.Time) *Interval
}

const (
	// ObservedUpdateCountAnnotation is an annotation added locally (in the monitor only), that tracks how many updates
	// we've seen to this resource.  This is useful during post-processing for determining if we have a hot resource.
	ObservedUpdateCountAnnotation = "monitor.openshift.io/observed-update-count"

	// ObservedRecreationCountAnnotation is an annotation added locally (in the monitor only), that tracks how many
	// time a resource has been recreated.  The internal cache doesn't remove an entry on delete.
	// This is useful during post-processing for determining if we have a hot resource.
	ObservedRecreationCountAnnotation = "monitor.openshift.io/observed-recreation-count"
)

type IntervalLevel int

const (
	Info IntervalLevel = iota
	Warning
	Error
)

func (e IntervalLevel) String() string {
	switch e {
	case Info:
		return "Info"
	case Warning:
		return "Warning"
	case Error:
		return "Error"
	default:
		panic(fmt.Sprintf("did not define event level string for %d", e))
	}
}

func ConditionLevelFromString(s string) (IntervalLevel, error) {
	switch s {
	case "Info":
		return Info, nil
	case "Warning":
		return Warning, nil
	case "Error":
		return Error, nil
	default:
		return Error, fmt.Errorf("did not define event level string for %q", s)
	}

}

type Condition struct {
	Level IntervalLevel

	Locator Locator
	Message Message
}

type LocatorType string

const (
	LocatorTypePod             LocatorType = "Pod"
	LocatorTypeContainer       LocatorType = "Container"
	LocatorTypeNode            LocatorType = "Node"
	LocatorTypeMachine         LocatorType = "Machine"
	LocatorTypeAlert           LocatorType = "Alert"
	LocatorTypeMetricsEndpoint LocatorType = "MetricsEndpoint"
	LocatorTypeClusterOperator LocatorType = "ClusterOperator"
	LocatorTypeDisruption      LocatorType = "Disruption"
	LocatorTypeKubeEvent       LocatorType = "KubeEvent"
	LocatorTypeE2ETest         LocatorType = "E2ETest"
	LocatorTypeAPIServer       LocatorType = "APIServer"
	LocatorTypeClusterVersion  LocatorType = "ClusterVersion"
	LocatorTypeKind            LocatorType = "Kind"
	LocatorTypeCloudMetrics    LocatorType = "CloudMetrics"
	LocatorTypeDeployment      LocatorType = "Deployment"
	LocatorTypeDaemonSet       LocatorType = "DaemonSet"
	LocatorTypeStatefulSet     LocatorType = "StatefulSet"

	LocatorTypeAPIUnreachableFromClient LocatorType = "APIUnreachableFromClient"

	LocatorTypeKubeletSyncLoopProbe LocatorType = "KubeletSyncLoopProbe"
	LocatorTypeKubeletSyncLoopPLEG  LocatorType = "KubeletSyncLoopPLEG"
	LocatorTypeStaticPodInstall     LocatorType = "StaticPodInstall"
)

type LocatorKey string

const (
	LocatorClusterOperatorKey LocatorKey = "clusteroperator"
	LocatorClusterVersionKey  LocatorKey = "clusterversion"
	LocatorNamespaceKey       LocatorKey = "namespace"
	LocatorDeploymentKey      LocatorKey = "deployment"
	LocatorDaemonSetKey       LocatorKey = "daemonset"
	LocatorStatefulSetKey     LocatorKey = "statefulset"
	LocatorNodeKey            LocatorKey = "node"
	LocatorMachineKey         LocatorKey = "machine"
	LocatorNodeRoleKey        LocatorKey = "node-role"
	LocatorEtcdMemberKey      LocatorKey = "etcd-member"
	LocatorNameKey            LocatorKey = "name"
	LocatorHmsgKey            LocatorKey = "hmsg"
	LocatorInstanceKey        LocatorKey = "instance"
	LocatorPodKey             LocatorKey = "pod"
	LocatorUIDKey             LocatorKey = "uid"
	LocatorMirrorUIDKey       LocatorKey = "mirror-uid"
	LocatorMetricsPathKey     LocatorKey = "metrics-path"
	LocatorServiceKey         LocatorKey = "service"
	LocatorContainerKey       LocatorKey = "container"
	LocatorAlertKey           LocatorKey = "alert"
	LocatorRouteKey           LocatorKey = "route"
	// LocatorBackendDisruptionNameKey holds the value used to store and locate historical data related to the amount of disruption.
	LocatorBackendDisruptionNameKey LocatorKey = "backend-disruption-name"
	LocatorDisruptionKey            LocatorKey = "disruption"
	LocatorE2ETestKey               LocatorKey = "e2e-test"
	LocatorLoadBalancerKey          LocatorKey = "load-balancer"
	LocatorConnectionKey            LocatorKey = "connection"
	LocatorProtocolKey              LocatorKey = "protocol"
	LocatorTargetKey                LocatorKey = "target"
	LocatorRowKey                   LocatorKey = "row"
	LocatorServerKey                LocatorKey = "server"
	LocatorMetricKey                LocatorKey = "metric"

	LocatorAPIUnreachableHostKey                  LocatorKey = "host"
	LocatorAPIUnreachableComponentKey             LocatorKey = "component"
	LocatorOnPremKubeapiUnreachableFromHaproxyKey LocatorKey = "onprem-haproxy"
	LocatorOnPremVIPMonitorKey                    LocatorKey = "onprem-keepalived"

	LocatorTypeKubeletSyncLoopProbeType LocatorKey = "probe"
	LocatorTypeKubeletSyncLoopPLEGType  LocatorKey = "plegType"
	LocatorStaticPodInstallType         LocatorKey = "podType"
)

type Locator struct {
	Type LocatorType `json:"type"`

	// annotations will include the Reason and Cause under their respective keys
	Keys map[LocatorKey]string `json:"keys"`
}

type IntervalReason string

const (
	IPTablesNotPermitted IntervalReason = "iptables-operation-not-permitted"

	DisruptionBeganEventReason              IntervalReason = "DisruptionBegan"
	DisruptionEndedEventReason              IntervalReason = "DisruptionEnded"
	DisruptionSamplerOutageBeganEventReason IntervalReason = "DisruptionSamplerOutageBegan"
	GracefulAPIServerShutdown               IntervalReason = "GracefulAPIServerShutdown"
	IncompleteAPIServerShutdown             IntervalReason = "IncompleteAPIServerShutdown"

	HttpClientConnectionLost IntervalReason = "HttpClientConnectionLost"

	PodPendingReason               IntervalReason = "PodIsPending"
	PodNotPendingReason            IntervalReason = "PodIsNotPending"
	PodReasonCreated               IntervalReason = "Created"
	PodReasonGracefulDeleteStarted IntervalReason = "GracefulDelete"
	PodReasonForceDelete           IntervalReason = "ForceDelete"
	PodReasonDeleted               IntervalReason = "Deleted"
	PodReasonScheduled             IntervalReason = "Scheduled"
	PodReasonEvicted               IntervalReason = "Evicted"
	PodReasonPreempted             IntervalReason = "Preempted"
	PodReasonFailed                IntervalReason = "Failed"
	PodReasonReady                 IntervalReason = "PodReady"
	PodReasonNotReady              IntervalReason = "PodNotReady"

	ContainerReasonContainerExit      IntervalReason = "ContainerExit"
	ContainerReasonContainerStart     IntervalReason = "ContainerStart"
	ContainerReasonContainerWait      IntervalReason = "ContainerWait"
	ContainerReasonReadinessFailed    IntervalReason = "ReadinessFailed"
	ContainerReasonReadinessErrored   IntervalReason = "ReadinessErrored"
	ContainerReasonStartupProbeFailed IntervalReason = "StartupProbeFailed"
	ContainerReasonReady              IntervalReason = "Ready"
	ContainerReasonRestarted          IntervalReason = "Restarted"
	ContainerReasonNotReady           IntervalReason = "NotReady"
	TerminationStateCleared           IntervalReason = "TerminationStateCleared"

	PodReasonDeletedBeforeScheduling IntervalReason = "DeletedBeforeScheduling"
	PodReasonDeletedAfterCompletion  IntervalReason = "DeletedAfterCompletion"

	NodeUpdateReason                IntervalReason = "NodeUpdate"
	NodeNotReadyReason              IntervalReason = "NotReady"
	NodeFailedLease                 IntervalReason = "FailedToUpdateLease"
	NodeUnexpectedReadyReason       IntervalReason = "UnexpectedNotReady"
	NodeUnexpectedUnreachableReason IntervalReason = "UnexpectedUnreachable"
	NodeUnreachable                 IntervalReason = "Unreachable"
	// Kubelet tries to get lease five times and then gives up
	NodeFailedLeaseBackoff IntervalReason = "FailedToUpdateLeaseInBackoff"
	NodeDiskPressure       IntervalReason = "NodeDiskPressure"
	NodeNoDiskPressure     IntervalReason = "NodeNoDiskPressure"
	NodeDeleted            IntervalReason = "Deleted"

	MachineConfigChangeReason  IntervalReason = "MachineConfigChange"
	MachineConfigReachedReason IntervalReason = "MachineConfigReached"

	MachineCreated      IntervalReason = "MachineCreated"
	MachineDeletedInAPI IntervalReason = "MachineDeletedInAPI"
	MachinePhaseChanged IntervalReason = "MachinePhaseChange"
	MachinePhase        IntervalReason = "MachinePhase"

	OnPremHaproxyDetectsDown  IntervalReason = "OnPremHaproxyDetectsDown"
	OnPremHaproxyStatusChange IntervalReason = "OnPremHaproxyStatusChange"
	OnPremLBPriorityChange    IntervalReason = "OnPremLBPriorityChange"
	OnPremLBTookVIP           IntervalReason = "OnPremLBTookVIP"
	OnPremLBLostVIP           IntervalReason = "OnPremLBLostVIP"

	Timeout IntervalReason = "Timeout"

	E2ETestStarted  IntervalReason = "E2ETestStarted"
	E2ETestFinished IntervalReason = "E2ETestFinished"

	CloudMetricsExtrenuous                IntervalReason = "CloudMetricsExtrenuous"
	CloudMetricsLBAvailability            IntervalReason = "CloudMetricsLBAvailability"
	CloudMetricsLBHealthEvent             IntervalReason = "CloudMetricsLBHealthEvent"
	FailedToDeleteCGroupsPath             IntervalReason = "FailedToDeleteCGroupsPath"
	FailedToAuthenticateWithOpenShiftUser IntervalReason = "FailedToAuthenticateWithOpenShiftUser"
	FailedContactingAPIReason             IntervalReason = "FailedContactingAPI"

	UnhealthyReason IntervalReason = "Unhealthy"

	UpgradeStartedReason  IntervalReason = "UpgradeStarted"
	UpgradeVersionReason  IntervalReason = "UpgradeVersion"
	UpgradeRollbackReason IntervalReason = "UpgradeRollback"
	UpgradeFailedReason   IntervalReason = "UpgradeFailed"
	UpgradeCompleteReason IntervalReason = "UpgradeComplete"

	NodeInstallerReason IntervalReason = "NodeInstaller"

	// client metrics show error connecting to the kube-apiserver
	APIUnreachableFromClientMetrics IntervalReason = "APIUnreachableFromClientMetrics"

	LeaseAcquiring        IntervalReason = "Acquiring"
	LeaseAcquiringStarted IntervalReason = "StartedAcquiring"
	LeaseAcquired         IntervalReason = "Acquired"

	ReasonBadOperatorApply  IntervalReason = "BadOperatorApply"
	ReasonKubeAPIServer500s IntervalReason = "KubeAPIServer500s"

	ReasonHighGeneration    IntervalReason = "HighGeneration"
	ReasonInvalidGeneration IntervalReason = "GenerationViolation"

	ReasonEtcdBootstrap     IntervalReason = "EtcdBootstrap"
	ReasonProcessDumpedCore IntervalReason = "ProcessDumpedCore"
)

type AnnotationKey string

const (
	AnnotationAlertState         AnnotationKey = "alertstate"
	AnnotationState              AnnotationKey = "state"
	AnnotationSeverity           AnnotationKey = "severity"
	AnnotationReason             AnnotationKey = "reason"
	AnnotationContainerExitCode  AnnotationKey = "code"
	AnnotationCause              AnnotationKey = "cause"
	AnnotationConfig             AnnotationKey = "config"
	AnnotationContainer          AnnotationKey = "container"
	AnnotationImage              AnnotationKey = "image"
	AnnotationInteresting        AnnotationKey = "interesting"
	AnnotationCount              AnnotationKey = "count"
	AnnotationNode               AnnotationKey = "node"
	AnnotationEtcdLocalMember    AnnotationKey = "local-member-id"
	AnnotationEtcdTerm           AnnotationKey = "term"
	AnnotationEtcdLeader         AnnotationKey = "leader"
	AnnotationPreviousEtcdLeader AnnotationKey = "prev-leader"
	AnnotationPathological       AnnotationKey = "pathological"
	AnnotationConstructed        AnnotationKey = "constructed"
	AnnotationPhase              AnnotationKey = "phase"
	AnnotationPreviousPhase      AnnotationKey = "previousPhase"
	AnnotationIsStaticPod        AnnotationKey = "mirrored"
	// TODO this looks wrong. seems like it ought to be set in the to/from
	AnnotationDuration         AnnotationKey = "duration"
	AnnotationRequestAuditID   AnnotationKey = "request-audit-id"
	AnnotationRoles            AnnotationKey = "roles"
	AnnotationStatus           AnnotationKey = "status"
	AnnotationCondition        AnnotationKey = "condition"
	AnnotationPercentage       AnnotationKey = "percentage"
	AnnotationPriority         AnnotationKey = "priority"
	AnnotationPreviousPriority AnnotationKey = "prev-priority"
	AnnotationVIP              AnnotationKey = "vip"
)

// ConstructionOwner was originally meant to signify that an interval was derived from other intervals.
// This allowed for the possibility of testing interval generation by feeding in only source intervals,
// and checking what was generated.
// TODO: likely want to drop this concept in favor of Source, plus a flag automatically applied to any
// intervals coming back from the monitor test call to generate calculated intervals. Source
// will replace the use of what constructed the interval, and the flag will allow us to see what is derived
// and what isn't.
type ConstructionOwner string

const (
	ConstructionOwnerNodeLifecycle    = "node-lifecycle-constructor"
	ConstructionOwnerPodLifecycle     = "pod-lifecycle-constructor"
	ConstructionOwnerEtcdLifecycle    = "etcd-lifecycle-constructor"
	ConstructionOwnerMachineLifecycle = "machine-lifecycle-constructor"
	ConstructionOwnerLeaseChecker     = "lease-checker"
	ConstructionOwnerOnPremHaproxy    = "on-prem-haproxy-constructor"
	ConstructionOwnerOnPremKeepalived = "on-prem-keepalived-constructor"
)

type Message struct {
	// TODO: reason/cause both fields and annotations...
	Reason       IntervalReason `json:"reason"`
	Cause        string         `json:"cause"`
	HumanMessage string         `json:"humanMessage"`

	// annotations will include the Reason and Cause under their respective keys
	Annotations map[AnnotationKey]string `json:"annotations"`
}

// IntervalSource is used to type/categorize all intervals based on what created them.
// This is intended to be used to group, and when combined with the display flag, signal that
// they should be visible by default in the UIs that render interval charts.
type IntervalSource string

const (
	SourceAlert                     IntervalSource = "Alert"
	SourceAPIServerShutdown         IntervalSource = "APIServerShutdown"
	SourceDisruption                IntervalSource = "Disruption"
	SourceE2ETest                   IntervalSource = "E2ETest"
	SourceKubeEvent                 IntervalSource = "KubeEvent"
	SourceNetworkManagerLog         IntervalSource = "NetworkMangerLog"
	SourceNodeMonitor               IntervalSource = "NodeMonitor"
	SourceHaproxyMonitor            IntervalSource = "OnPremHaproxyMonitor"
	SourceKeepalivedMonitor         IntervalSource = "OnPremKeepalivedMonitor"
	SourceUnexpectedReady           IntervalSource = "NodeUnexpectedNotReady"
	SourceUnreachable               IntervalSource = "NodeUnreachable"
	SourceKubeletLog                IntervalSource = "KubeletLog"
	SourceSystemdCoreDumpLog        IntervalSource = "SystemdCoreDumpLog"
	SourcePodLog                    IntervalSource = "PodLog"
	SourceEtcdLog                   IntervalSource = "EtcdLog"
	SourceEtcdLeadership            IntervalSource = "EtcdLeadership"
	SourcePodMonitor                IntervalSource = "PodMonitor"
	SourceMetricsEndpointDown       IntervalSource = "MetricsEndpointDown"
	APIServerGracefulShutdown       IntervalSource = "APIServerGracefulShutdown"
	APIServerClusterOperatorWatcher IntervalSource = "APIServerClusterOperatorWatcher"
	SourceAuditLog                  IntervalSource = "AuditLog"

	SourceTestData                IntervalSource = "TestData" // some tests have no real source to assign
	SourceOVSVswitchdLog          IntervalSource = "OVSVswitchdLog"
	SourcePathologicalEventMarker IntervalSource = "PathologicalEventMarker" // not sure if this is really helpful since the events all have a different origin
	SourceClusterOperatorMonitor  IntervalSource = "ClusterOperatorMonitor"
	SourceOperatorState           IntervalSource = "OperatorState"
	SourceVersionState            IntervalSource = "VersionState"
	SourceNodeState                              = "NodeState"
	SourcePodState                               = "PodState"
	SourceCloudMetrics                           = "CloudMetrics"

	SourceAPIUnreachableFromClient IntervalSource = "APIUnreachableFromClient"
	SourceMachine                  IntervalSource = "MachineMonitor"

	SourceGenerationMonitor IntervalSource = "GenerationMonitor"

	SourceStaticPodInstallMonitor  IntervalSource = "StaticPodInstallMonitor"
	SourceCPUMonitor               IntervalSource = "CPUMonitor"
	SourceEtcdDiskCommitDuration   IntervalSource = "EtcdDiskCommitDuration"
	SourceEtcdDiskWalFsyncDuration IntervalSource = "EtcdDiskWalFsyncDuration"
)

type Interval struct {
	// Deprecated: We hope to fold this into Interval itself.
	Condition
	Source IntervalSource

	// Display is a very coarse hint to any UI that this event was considered important enough to *possibly* be displayed by the source that produced it.
	// UI may apply further filtering.
	Display bool

	From time.Time
	To   time.Time
}

func (r IntervalReason) String() string {
	return string(r)
}

const TimeFormat = "Jan 02 15:04:05"

func (i Interval) String() string {
	if i.From.Equal(i.To) {
		return fmt.Sprintf("%s.%03d %s %s %s",
			i.From.Format(TimeFormat),
			i.From.Nanosecond()/int(time.Millisecond),
			i.Level.String()[:1],
			i.Locator.OldLocator(),
			strings.Replace(i.Message.OldMessage(), "\n", "\\n", -1))
	}
	duration := i.To.Sub(i.From)
	if duration < time.Second {
		return fmt.Sprintf("%s.%03d - %-5s %s %s %s",
			i.From.Format(TimeFormat),
			i.From.Nanosecond()/int(time.Millisecond),
			strconv.Itoa(int(duration/time.Millisecond))+"ms",
			i.Level.String()[:1],
			i.Locator.OldLocator(),
			strings.Replace(i.Message.OldMessage(), "\n", "\\n", -1))
	}
	return fmt.Sprintf("%s.%03d - %-5s %s %s %s",
		i.From.Format(TimeFormat),
		i.From.Nanosecond()/int(time.Millisecond), strconv.Itoa(int(duration/time.Second))+"s",
		i.Level.String()[:1],
		i.Locator.OldLocator(),
		strings.Replace(i.Message.OldMessage(), "\n", "\\n", -1))
}

func (i Message) OldMessage() string {
	keys := sets.NewString()
	for k := range i.Annotations {
		keys.Insert(string(k))
	}

	annotations := []string{}
	for _, k := range keys.List() {
		v := i.Annotations[AnnotationKey(k)]
		annotations = append(annotations, fmt.Sprintf("%v/%v", k, v))
	}
	annotationString := strings.Join(annotations, " ")

	if len(i.HumanMessage) == 0 {
		return annotationString
	}

	return strings.TrimSpace(fmt.Sprintf("%v %v", annotationString, i.HumanMessage))
}

func (i Locator) OldLocator() string {
	keys := []string{}
	for k := range i.Keys {
		keys = append(keys, string(k))
	}

	keys = sortKeys(keys)

	annotations := []string{}
	for _, k := range keys {
		v := i.Keys[LocatorKey(k)]
		if LocatorKey(k) == LocatorE2ETestKey {
			annotations = append(annotations, fmt.Sprintf("%v/%q", k, v))
		} else {
			annotations = append(annotations, fmt.Sprintf("%v/%v", k, v))
		}
	}

	annotationString := strings.Join(annotations, " ")

	return annotationString
}

func (i Locator) HasKey(k LocatorKey) bool {
	_, ok := i.Keys[k]
	return ok
}

// sortKeys ensures that some keys appear in the order we require (least specific to most), so rows with locators
// are grouped together. (i.e. keeping containers within the same pod together, or rows for a specific container)
// Blindly going through the keys results in alphabetical ordering, container comes first, and then we've
// got container events separated from their pod events on the intervals chart.
// This will hopefully eventually go away but for now we need it.
// Courtesy of ChatGPT but unit tested.
func sortKeys(keys []string) []string {

	// Ensure these keys appear in this order. Other keys can be mixed in and will appear at the end in alphabetical
	// order.
	orderedKeys := []string{"namespace", "node", "pod", "uid", "server", "container", "shutdown", "row"}

	// Create a map to store the indices of keys in the orderedKeys array.
	// This will allow us to efficiently check if a key is in orderedKeys and find its position.
	orderedKeyIndices := make(map[string]int)

	for i, key := range orderedKeys {
		orderedKeyIndices[key] = i
	}

	// Define a custom sorting function that orders the keys based on the orderedKeys array.
	sort.Slice(keys, func(i, j int) bool {
		// Get the indices of keys i and j in orderedKeys.
		indexI, existsI := orderedKeyIndices[keys[i]]
		indexJ, existsJ := orderedKeyIndices[keys[j]]

		// If both keys exist in orderedKeys, sort them based on their order.
		if existsI && existsJ {
			return indexI < indexJ
		}

		// If only one of the keys exists in orderedKeys, move it to the front.
		if existsI {
			return true
		} else if existsJ {
			return false
		}

		// If neither key is in orderedKeys, sort alphabetically so we have predictable ordering
		return keys[i] < keys[j]
	})

	return keys
}

type IntervalFilter func(i Interval) bool

type IntervalFilters []IntervalFilter

func (filters IntervalFilters) All(i Interval) bool {
	for _, filter := range filters {
		if !filter(i) {
			return false
		}
	}
	return true
}

func (filters IntervalFilters) Any(i Interval) bool {
	for _, filter := range filters {
		if filter(i) {
			return true
		}
	}
	return false
}

type Intervals []Interval

var _ sort.Interface = Intervals{}

func (intervals Intervals) Less(i, j int) bool {
	// currently synced with https://github.com/openshift/origin/blob/9b001745ec8006eb406bd92e3555d1070b9b656e/pkg/monitor/serialization/serialize.go#L175

	switch d := intervals[i].From.Sub(intervals[j].From); {
	case d < 0:
		return true
	case d > 0:
		return false
	}
	switch d := intervals[i].To.Sub(intervals[j].To); {
	case d < 0:
		return true
	case d > 0:
		return false
	}
	if intervals[i].Message.Reason != intervals[j].Message.Reason {
		return intervals[i].Message.Reason < intervals[j].Message.Reason
	}
	if intervals[i].Message.HumanMessage != intervals[j].Message.HumanMessage {
		return intervals[i].Message.HumanMessage < intervals[j].Message.HumanMessage
	}

	// TODO: this could be a bit slow, but leaving it simple if we can get away with it. Sorting structured locators
	// that use keys is trickier than the old flat string method.
	return intervals[i].Locator.OldLocator() < intervals[j].Locator.OldLocator()
}
func (intervals Intervals) Len() int { return len(intervals) }
func (intervals Intervals) Swap(i, j int) {
	intervals[i], intervals[j] = intervals[j], intervals[i]
}

// Strings returns the result of String() on each included interval.
func (intervals Intervals) Strings() []string {
	if len(intervals) == 0 {
		return []string(nil)
	}
	s := make([]string, 0, len(intervals))
	for _, interval := range intervals {
		s = append(s, interval.String())
	}
	return s
}

// Duration returns the sum of all intervals in the range. If To is less than or
// equal to From, 0 is used instead (use Clamp() if open intervals
// should be not considered instant).
// minDuration is the smallest duration to add.  If a duration is less than the minDuration,
// then the minDuration is used instead.  This is useful for measuring samples.
// For example, consider a case of one second polling for server availability.
// If a sample fails, you don't definitively know whether it was down just after t-1s or just before t.
// On average, it would be 500ms, but a useful minimum in this case could be 1s.
func (intervals Intervals) Duration(minCurrentDuration time.Duration) time.Duration {
	var totalDuration time.Duration
	for _, interval := range intervals {
		currentDuration := interval.To.Sub(interval.From)
		if currentDuration <= 0 {
			totalDuration += 0
		} else if currentDuration < minCurrentDuration {
			totalDuration += minCurrentDuration
		} else {
			totalDuration += currentDuration
		}
	}
	return totalDuration
}

// EventIntervalMatchesFunc is a function for matching eventIntervales
type EventIntervalMatchesFunc func(eventInterval Interval) bool

// IsErrorEvent returns true if the eventInterval is an Error
func IsErrorEvent(eventInterval Interval) bool {
	return eventInterval.Level == Error
}

// IsWarningEvent returns true if the eventInterval is an Warning
func IsWarningEvent(eventInterval Interval) bool {
	return eventInterval.Level == Warning
}

// IsInfoEvent returns true if the eventInterval is an Info
func IsInfoEvent(eventInterval Interval) bool {
	return eventInterval.Level == Info
}

// IsInE2ENamespace returns true if the eventInterval is in an e2e namespace
func IsInE2ENamespace(eventInterval Interval) bool {
	return strings.HasPrefix(NamespaceFromLocator(eventInterval.Locator), "e2e-")
}

func IsForDisruptionBackend(backend string) EventIntervalMatchesFunc {
	return func(eventInterval Interval) bool {
		if eventInterval.Locator.Keys[LocatorBackendDisruptionNameKey] == backend {
			return true
		}
		return false
	}
}

func IsInNamespaces(namespaces sets.String) EventIntervalMatchesFunc {
	return func(eventInterval Interval) bool {
		// For new, structured locators:
		if ns, ok := eventInterval.Locator.Keys[LocatorNamespaceKey]; ok {
			return namespaces.Has(ns)
		}
		return false
	}
}

// ContainsAllParts ensures that all listed key match at least one of the values.
func ContainsAllParts(matchers map[string][]*regexp.Regexp) EventIntervalMatchesFunc {
	return func(eventInterval Interval) bool {
		actualParts := eventInterval.Locator.Keys
		for key, possibleValues := range matchers {
			actualValue := actualParts[LocatorKey(key)]

			found := false
			for _, possibleValue := range possibleValues {
				if possibleValue.MatchString(actualValue) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}

		return true
	}
}

// NotContainsAllParts returns a function that returns false if any key matches.
func NotContainsAllParts(matchers map[string][]*regexp.Regexp) EventIntervalMatchesFunc {
	return func(eventInterval Interval) bool {
		actualParts := eventInterval.Locator.Keys
		for key, possibleValues := range matchers {
			actualValue := actualParts[LocatorKey(key)]

			for _, possibleValue := range possibleValues {
				if possibleValue.MatchString(actualValue) {
					return false
				}
			}
		}
		return true
	}
}

func And(filters ...EventIntervalMatchesFunc) EventIntervalMatchesFunc {
	return func(eventInterval Interval) bool {
		for _, filter := range filters {
			if !filter(eventInterval) {
				return false
			}
		}
		return true
	}
}

func Or(filters ...EventIntervalMatchesFunc) EventIntervalMatchesFunc {
	return func(eventInterval Interval) bool {
		for _, filter := range filters {
			if filter(eventInterval) {
				return true
			}
		}
		return false
	}
}

func Not(filter EventIntervalMatchesFunc) EventIntervalMatchesFunc {
	return func(eventInterval Interval) bool {
		return !filter(eventInterval)
	}
}

func StartedBefore(limit time.Time) EventIntervalMatchesFunc {
	return func(eventInterval Interval) bool {
		return eventInterval.From.Before(limit)
	}
}

func EndedAfter(limit time.Time) EventIntervalMatchesFunc {
	return func(eventInterval Interval) bool {
		return eventInterval.To.After(limit)
	}
}

func NodeUpdate(eventInterval Interval) bool {
	reason := eventInterval.Message.Reason
	return NodeUpdateReason == reason
}

func NodeLeaseBackoff(eventInterval Interval) bool {
	reason := eventInterval.Message.Reason
	return NodeFailedLeaseBackoff == reason
}

func AlertFiring() EventIntervalMatchesFunc {
	return func(eventInterval Interval) bool {
		return eventInterval.Message.Annotations[AnnotationAlertState] == "firing"
	}
}

func AlertPending() EventIntervalMatchesFunc {
	return func(eventInterval Interval) bool {
		return eventInterval.Message.Annotations[AnnotationAlertState] == "pending"
	}
}

// Filter returns a copy of intervals with only intervals that match the provided
// function.
func (intervals Intervals) Filter(eventFilterMatches EventIntervalMatchesFunc) Intervals {
	if len(intervals) == 0 {
		return Intervals(nil)
	}
	copied := make(Intervals, 0, len(intervals))
	for _, interval := range intervals {
		if eventFilterMatches(interval) {
			copied = append(copied, interval)
		}
	}
	return copied
}

// Cut creates a copy of intervals where all events (empty To) are
// within [from,to) and all intervals that overlap [from,to) are
// included, but with their from/to fields limited to that range.
func (intervals Intervals) Cut(from, to time.Time) Intervals {
	if len(intervals) == 0 {
		return Intervals(nil)
	}
	copied := make(Intervals, 0, len(intervals))
	for _, interval := range intervals {
		if interval.To.IsZero() {
			if interval.From.IsZero() {
				continue
			}
			if interval.From.Before(from) || !interval.From.Before(to) {
				continue
			}
		} else {
			if interval.To.Before(from) || !interval.From.Before(to) {
				continue
			}
			// limit the interval to the provided range
			if interval.To.After(to) {
				interval.To = to
			}
			if interval.From.Before(from) {
				interval.From = from
			}
		}
		copied = append(copied, interval)
	}
	return copied
}

// Slice works on a sorted Intervals list and returns the set of intervals
// The intervals start from the first Interval that ends AFTER the argument.From.
// The last interval is one before the first Interval that starts before the argument.To
// The zero value will return all elements. If intervals is unsorted the result is undefined. This
// runs in O(n).
func (intervals Intervals) Slice(from, to time.Time) Intervals {
	if from.IsZero() && to.IsZero() {
		return intervals
	}

	// forget being fancy, just iterate from the beginning.
	first := -1
	if from.IsZero() {
		first = 0
	} else {
		for i := range intervals {
			curr := intervals[i]
			if curr.To.IsZero() {
				if curr.From.After(from) || curr.From == from {
					first = i
					break
				}
			}
			if curr.To.After(from) || curr.To == from {
				first = i
				break
			}
		}
	}
	if first == -1 || len(intervals) == 0 {
		return Intervals{}
	}

	if to.IsZero() {
		return intervals[first:]
	}
	for i := first; i < len(intervals); i++ {
		if intervals[i].From.After(to) {
			return intervals[first:i]
		}
	}
	return intervals[first:]
}

// Clamp sets all zero value From or To fields to from or to.
func (intervals Intervals) Clamp(from, to time.Time) {
	for i := range intervals {
		if intervals[i].From.Before(from) {
			intervals[i].From = from
		}
		if intervals[i].To.IsZero() {
			intervals[i].To = to
		}
		if intervals[i].To.After(to) {
			intervals[i].To = to
		}
	}
}

type InstanceKey struct {
	Namespace string
	Name      string
	UID       string
}

type InstanceMap map[InstanceKey]runtime.Object
type ResourcesMap map[string]InstanceMap

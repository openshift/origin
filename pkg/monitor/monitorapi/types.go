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
	StartInterval(t time.Time, condition Condition) int
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

	// TODO: Goal here is to drop Locator/Message, and rename the structured variants to Locator/Message
	Locator           string
	StructuredLocator Locator
	Message           string
	StructuredMessage Message
}

type LocatorType string

const (
	LocatorTypePod               LocatorType = "Pod"
	LocatorTypeContainer         LocatorType = "Container"
	LocatorTypeNode              LocatorType = "Node"
	LocatorTypeAlert             LocatorType = "Alert"
	LocatorTypeClusterOperator   LocatorType = "ClusterOperator"
	LocatorTypeOther             LocatorType = "Other"
	LocatorTypeDisruption        LocatorType = "Disruption"
	LocatorTypeKubeEvent         LocatorType = "KubeEvent"
	LocatorTypeE2ETest           LocatorType = "E2ETest"
	LocatorTypeAPIServerShutdown LocatorType = "APIServerShutdown"
)

type LocatorKey string

const (
	LocatorServer             LocatorKey = "server"   // TODO this looks like a bad name.  Aggregated apiserver?  Do we even need it?
	LocatorShutdown           LocatorKey = "shutdown" // TODO this should not exist.  This is a reason and message
	LocatorClusterOperatorKey LocatorKey = "clusteroperator"
	LocatorNamespaceKey       LocatorKey = "namespace"
	LocatorNodeKey            LocatorKey = "node"
	LocatorKindKey            LocatorKey = "kind"
	LocatorNameKey            LocatorKey = "name"
	LocatorPodKey             LocatorKey = "pod"
	LocatorUIDKey             LocatorKey = "uid"
	LocatorMirrorUIDKey       LocatorKey = "mirror-uid"
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
	LocatorShutdownKey              LocatorKey = "shutdown"
	LocatorServerKey                LocatorKey = "server"
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
	GracefulAPIServerShutdown               IntervalReason = "GracefulShutdownWindow"

	HttpClientConnectionLost IntervalReason = "HttpClientConnectionLost"

	PodPendingReason               IntervalReason = "PodIsPending"
	PodNotPendingReason            IntervalReason = "PodIsNotPending"
	PodReasonCreated               IntervalReason = "Created"
	PodReasonGracefulDeleteStarted IntervalReason = "GracefulDelete"
	PodReasonForceDelete           IntervalReason = "ForceDelete"
	PodReasonDeleted               IntervalReason = "Deleted"
	PodReasonScheduled             IntervalReason = "Scheduled"

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

	NodeUpdateReason   IntervalReason = "NodeUpdate"
	NodeNotReadyReason IntervalReason = "NotReady"
	NodeFailedLease    IntervalReason = "FailedToUpdateLease"

	Timeout IntervalReason = "Timeout"

	E2ETestStarted  IntervalReason = "E2ETestStarted"
	E2ETestFinished IntervalReason = "E2ETestFinished"
)

type AnnotationKey string

const (
	AnnotationReason            AnnotationKey = "reason"
	AnnotationContainerExitCode AnnotationKey = "code"
	AnnotationCause             AnnotationKey = "cause"
	AnnotationNode              AnnotationKey = "node"
	AnnotationConstructed       AnnotationKey = "constructed"
	AnnotationPodPhase          AnnotationKey = "phase"
	AnnotationIsStaticPod       AnnotationKey = "mirrored"
	// TODO this looks wrong. seems like it ought to be set in the to/from
	AnnotationDuration       AnnotationKey = "duration"
	AnnotationRequestAuditID AnnotationKey = "request-audit-id"
	AnnotationStatus         AnnotationKey = "status"
	AnnotationCondition      AnnotationKey = "condition"
)

type ConstructionOwner string

const (
	ConstructionOwnerNodeLifecycle = "node-lifecycle-constructor"
	ConstructionOwnerPodLifecycle  = "pod-lifecycle-constructor"
)

type Message struct {
	// TODO: reason/cause both fields and annotations...
	Reason       IntervalReason `json:"reason"`
	Cause        string         `json:"cause"`
	HumanMessage string         `json:"humanMessage"`

	// annotations will include the Reason and Cause under their respective keys
	Annotations map[AnnotationKey]string `json:"annotations"`
}

type IntervalSource string

const (
	SourceAlert                   IntervalSource = "Alert"
	SourceAPIServerShutdown       IntervalSource = "APIServerShutdown"
	SourceDisruption              IntervalSource = "Disruption"
	SourceE2ETest                 IntervalSource = "E2ETest"
	SourceNetworkManagerLog       IntervalSource = "NetworkMangerLog"
	SourceNodeMonitor             IntervalSource = "NodeMonitor"
	SourcePodLog                  IntervalSource = "PodLog"
	SourcePodMonitor              IntervalSource = "PodMonitor"
	SourceKubeEvent               IntervalSource = "KubeEvent"
	SourceTestData                IntervalSource = "TestData"                // some tests have no real source to assign
	SourcePathologicalEventMarker IntervalSource = "PathologicalEventMarker" // not sure if this is really helpful since the events all have a different origin
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

func (i Interval) String() string {
	if i.From.Equal(i.To) {
		return fmt.Sprintf("%s.%03d %s %s %s", i.From.Format("Jan 02 15:04:05"), i.From.Nanosecond()/int(time.Millisecond), i.Level.String()[:1], i.Locator, strings.Replace(i.Message, "\n", "\\n", -1))
	}
	duration := i.To.Sub(i.From)
	if duration < time.Second {
		return fmt.Sprintf("%s.%03d - %-5s %s %s %s", i.From.Format("Jan 02 15:04:05"), i.From.Nanosecond()/int(time.Millisecond), strconv.Itoa(int(duration/time.Millisecond))+"ms", i.Level.String()[:1], i.Locator, strings.Replace(i.Message, "\n", "\\n", -1))
	}
	return fmt.Sprintf("%s.%03d - %-5s %s %s %s", i.From.Format("Jan 02 15:04:05"), i.From.Nanosecond()/int(time.Millisecond), strconv.Itoa(int(duration/time.Second))+"s", i.Level.String()[:1], i.Locator, strings.Replace(i.Message, "\n", "\\n", -1))
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
	keys := sets.NewString()
	for k := range i.Keys {
		keys.Insert(string(k))
	}

	annotations := []string{}
	for _, k := range keys.List() {
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
	return intervals[i].Message < intervals[j].Message
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
	// Old style
	if strings.Contains(eventInterval.Locator, "ns/e2e-") {
		return true
	}
	// New style
	if strings.Contains(eventInterval.Locator, "namespace/e2e-") {
		return true
	}
	return false
}

func IsInNamespaces(namespaces sets.String) EventIntervalMatchesFunc {
	return func(eventInterval Interval) bool {
		ns := NamespaceFromLocator(eventInterval.Locator)
		return namespaces.Has(ns)
	}
}

// ContainsAllParts ensures that all listed key match at least one of the values.
func ContainsAllParts(matchers map[string][]*regexp.Regexp) EventIntervalMatchesFunc {
	return func(eventInterval Interval) bool {
		actualParts := LocatorParts(eventInterval.Locator)
		for key, possibleValues := range matchers {
			actualValue := actualParts[key]

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
		actualParts := LocatorParts(eventInterval.Locator)
		for key, possibleValues := range matchers {
			actualValue := actualParts[key]

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
	reason := ReasonFrom(eventInterval.Message)
	return NodeUpdateReason == reason
}

func AlertFiring() EventIntervalMatchesFunc {
	return func(eventInterval Interval) bool {
		if strings.Contains(eventInterval.Message, `alertstate="firing"`) {
			return true
		}
		return false
	}
}

func AlertPending() EventIntervalMatchesFunc {
	return func(eventInterval Interval) bool {
		if strings.Contains(eventInterval.Message, `alertstate="pending"`) {
			return true
		}
		return false
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

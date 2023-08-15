package allowedalerts

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortestlibrary/historicaldata"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	platformidentification2 "github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"

	corev1 "k8s.io/api/core/v1"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/kubernetes/test/e2e/framework"
)

type AlertTest interface {
	// InvariantTestName is name for this as an invariant test
	InvariantTestName() string

	// AlertName is the name of the alert
	AlertName() string
	// AlertState is the threshold this test applies to.
	AlertState() AlertState

	InvariantCheck(intervals monitorapi.Intervals, r monitorapi.ResourcesMap) ([]*junitapi.JUnitTestCase, error)
}

// AlertState is the state of the alert. They are logically ordered, so if a test says it limits on "pending", then
// any state above pending (like info or warning) will cause the test to fail.
type AlertState string

const (
	AlertPending  AlertState = "pending"
	AlertInfo     AlertState = "info"
	AlertWarning  AlertState = "warning"
	AlertCritical AlertState = "critical"
	AlertUnknown  AlertState = "unknown"
)

type alertBuilder struct {
	bugzillaComponent  string
	divideByNamespaces bool
	alertName          string
	alertState         AlertState
	jobType            *platformidentification2.JobType

	allowanceCalculator AlertTestAllowanceCalculator
}

type basicAlertTest struct {
	bugzillaComponent string
	alertName         string
	namespace         string
	alertState        AlertState
	jobType           *platformidentification2.JobType

	allowanceCalculator AlertTestAllowanceCalculator
}

func newAlert(bugzillaComponent, alertName string, jobType *platformidentification2.JobType) *alertBuilder {
	return &alertBuilder{
		bugzillaComponent:   bugzillaComponent,
		alertName:           alertName,
		alertState:          AlertPending,
		allowanceCalculator: DefaultAllowances,
		jobType:             jobType,
	}
}

func newNamespacedAlert(alertName string, jobType *platformidentification2.JobType) *alertBuilder {
	return &alertBuilder{
		divideByNamespaces:  true,
		alertName:           alertName,
		alertState:          AlertPending,
		allowanceCalculator: DefaultAllowances,
		jobType:             jobType,
	}
}

func (a *alertBuilder) withAllowance(allowanceCalculator AlertTestAllowanceCalculator) *alertBuilder {
	a.allowanceCalculator = allowanceCalculator
	return a
}

func (a *alertBuilder) pending() *alertBuilder {
	a.alertState = AlertPending
	return a
}

func (a *alertBuilder) firing() *alertBuilder {
	a.alertState = AlertInfo
	return a
}

func (a *alertBuilder) warning() *alertBuilder {
	a.alertState = AlertWarning
	return a
}

func (a *alertBuilder) critical() *alertBuilder {
	a.alertState = AlertCritical
	return a
}

func (a *alertBuilder) neverFail() *alertBuilder {
	a.allowanceCalculator = neverFail(a.allowanceCalculator)
	return a
}

func (a *alertBuilder) toTests() []AlertTest {
	if !a.divideByNamespaces {
		return []AlertTest{
			&basicAlertTest{
				bugzillaComponent:   a.bugzillaComponent,
				alertName:           a.alertName,
				alertState:          a.alertState,
				allowanceCalculator: a.allowanceCalculator,
				jobType:             a.jobType,
			},
		}
	}

	ret := []AlertTest{}
	for namespace, bzComponent := range platformidentification2.GetNamespacesToBugzillaComponents() {
		ret = append(ret, &basicAlertTest{
			bugzillaComponent:   bzComponent,
			namespace:           namespace,
			alertName:           a.alertName,
			alertState:          a.alertState,
			allowanceCalculator: a.allowanceCalculator,
			jobType:             a.jobType,
		})
	}
	ret = append(ret, &basicAlertTest{
		bugzillaComponent:   "Unknown",
		namespace:           platformidentification2.NamespaceOther,
		alertName:           a.alertName,
		alertState:          a.alertState,
		allowanceCalculator: a.allowanceCalculator,
		jobType:             a.jobType,
	})

	return ret
}

func (a *basicAlertTest) InvariantTestName() string {
	switch {
	case len(a.namespace) == 0:
		return fmt.Sprintf("[bz-%v][invariant] alert/%s should not be at or above %s", a.bugzillaComponent, a.alertName, a.alertState)
	case a.namespace == platformidentification2.NamespaceOther:
		return fmt.Sprintf("[bz-%v][invariant] alert/%s should not be at or above %s in all the other namespaces", a.bugzillaComponent, a.alertName, a.alertState)
	default:
		return fmt.Sprintf("[bz-%v][invariant] alert/%s should not be at or above %s in ns/%s", a.bugzillaComponent, a.alertName, a.alertState, a.namespace)
	}
}

func (a *basicAlertTest) AlertName() string {
	return a.alertName
}

func (a *basicAlertTest) AlertState() AlertState {
	return a.alertState
}

type testState int

const (
	pass testState = iota
	flake
	fail
)

func (a *basicAlertTest) failOrFlake(firingIntervals, pendingIntervals monitorapi.Intervals) (testState, string) {
	var alertIntervals monitorapi.Intervals

	switch a.AlertState() {
	case AlertPending:
		alertIntervals = append(alertIntervals, pendingIntervals...)
		fallthrough

	case AlertInfo:
		alertIntervals = append(alertIntervals, firingIntervals.Filter(monitorapi.IsInfoEvent)...)
		fallthrough

	case AlertWarning:
		alertIntervals = append(alertIntervals, firingIntervals.Filter(monitorapi.IsWarningEvent)...)
		fallthrough

	case AlertCritical:
		alertIntervals = append(alertIntervals, firingIntervals.Filter(monitorapi.IsErrorEvent)...)

	default:
		return fail, fmt.Sprintf("unhandled alert state: %v", a.AlertState())
	}

	describe := alertIntervals.Strings()
	durationAtOrAboveLevel := alertIntervals.Duration(1 * time.Second)
	firingDuration := firingIntervals.Duration(1 * time.Second)
	pendingDuration := pendingIntervals.Duration(1 * time.Second)

	dataKey := historicaldata.AlertDataKey{
		AlertName:      a.alertName,
		AlertLevel:     string(a.alertState),
		AlertNamespace: a.namespace,
		JobType:        *a.jobType,
	}

	failAfter, err := a.allowanceCalculator.FailAfter(dataKey)
	if err != nil {
		return fail, fmt.Sprintf("unable to calculate allowance for %s which was at %s, err %v\n\n%s", a.AlertName(), a.AlertState(), err, strings.Join(describe, "\n"))
	}
	flakeAfter := a.allowanceCalculator.FlakeAfter(dataKey)

	switch {
	case durationAtOrAboveLevel > failAfter:
		return fail, fmt.Sprintf("%s was at or above %s for at least %s on %#v (maxAllowed=%s): pending for %s, firing for %s:\n\n%s",
			a.AlertName(), a.AlertState(), durationAtOrAboveLevel, *a.jobType, failAfter, pendingDuration, firingDuration, strings.Join(describe, "\n"))

	case durationAtOrAboveLevel > flakeAfter:
		return flake, fmt.Sprintf("%s was at or above %s for at least %s on %#v (maxAllowed=%s): pending for %s, firing for %s:\n\n%s",
			a.AlertName(), a.AlertState(), durationAtOrAboveLevel, *a.jobType, flakeAfter, pendingDuration, firingDuration, strings.Join(describe, "\n"))
	}

	return pass, ""
}

var unrecognizedSignatureRegEx = regexp.MustCompile("reason/ErrImagePull UnrecognizedSignatureFormat")

func kubePodNotReadyDueToErrParsingSignature(trackedEventResources monitorapi.InstanceMap, firingIntervals monitorapi.Intervals) bool {
	return kubePodNotReadyDueToRegExMatch(trackedEventResources, firingIntervals, unrecognizedSignatureRegEx)
}

var imagePullBackoffRegEx = regexp.MustCompile("Back-off pulling image .*registry.redhat.io")

// kubePodNotReadyDueToImagePullBackoff returns true if we searched pod events and determined that the
// KubePodNotReady alert for this pod fired due to an imagePullBackoff event on registry.redhat.io.
func kubePodNotReadyDueToImagePullBackoff(trackedEventResources monitorapi.InstanceMap, firingIntervals monitorapi.Intervals) bool {
	return kubePodNotReadyDueToRegExMatch(trackedEventResources, firingIntervals, imagePullBackoffRegEx)
}

func kubePodNotReadyDueToRegExMatch(trackedEventResources monitorapi.InstanceMap, firingIntervals monitorapi.Intervals, regexp *regexp.Regexp) bool {
	// Run the check for all firing intervals.
	for _, firingInterval := range firingIntervals {
		relatedPodRef := monitorapi.PodFrom(firingInterval.Locator)

		// Find an event
		foundRegexMatchEvent := false
		var tmpEvent *corev1.Event
		for _, obj := range trackedEventResources {
			tmpEvent = obj.(*corev1.Event)
			if tmpEvent.InvolvedObject.Name == relatedPodRef.Name &&
				tmpEvent.InvolvedObject.Namespace == relatedPodRef.Namespace &&
				regexp.MatchString(tmpEvent.Message) {
				foundRegexMatchEvent = true
				break
			}
		}
		if !foundRegexMatchEvent {
			// No event resources found so we can't do any checking.
			return false
		}
		regexMatchEventTime := tmpEvent.LastTimestamp.Time
		alertTime := firingInterval.From
		if alertTime.After(regexMatchEventTime) && alertTime.Sub(regexMatchEventTime) < time.Minute*10 {
			framework.Logf("KubePodNotReady alert failure suppressed due to %s on pod %s/%s", tmpEvent.Message,
				tmpEvent.ObjectMeta.Namespace, tmpEvent.ObjectMeta.Name)
		} else {
			return false
		}
	}
	return true
}

// redhatOperatorPodsNotPending returns true of we determined that there is a redhat-operator
// pod not in Pending state; this implies that the pod is up so we don't need to fail on the
// RedhatOperatorsCatalogError alert.
func redhatOperatorPodsNotPending(trackedPodResources monitorapi.InstanceMap, firingIntervals monitorapi.Intervals) bool {

	// Find the redhat-operators pod in the openshift-marketplace namespace.
	rhPodFound := false
	var rhPod *corev1.Pod
	for _, obj := range trackedPodResources {
		rhPod = obj.(*corev1.Pod)
		if namespace := rhPod.ObjectMeta.Namespace; namespace != "openshift-marketplace" {
			continue
		}
		if podName := rhPod.ObjectMeta.Name; !strings.HasPrefix(podName, "redhat-operators") {
			continue
		}
		rhPodFound = true
	}
	if !rhPodFound {
		// No redhat-operator pod found so we can't do any checking.
		return false
	}

	podStartTime := rhPod.Status.StartTime.Time
	for i := range firingIntervals {
		alertTime := firingIntervals[i].From
		if alertTime.Before(podStartTime) && podStartTime.Sub(alertTime) >= time.Minute*10 && rhPod.Status.Phase != corev1.PodPending {
			framework.Logf("RedhatOperatorsCatalogError alert interval %d failure suppressed since %s is not Pending 10+ minutes later", i, rhPod.ObjectMeta.Name)
		} else {
			return false
		}
	}
	return true
}

func (a *basicAlertTest) InvariantCheck(allEventIntervals monitorapi.Intervals, resourcesMap monitorapi.ResourcesMap) ([]*junitapi.JUnitTestCase, error) {

	if a.jobType == nil {
		// Hard fail if the higher level job type lookup from the actual cluster failed
		return []*junitapi.JUnitTestCase{
			{
				Name: a.InvariantTestName(),
				FailureOutput: &junitapi.FailureOutput{
					Output: "Unable to determine JobType for alert InvariantCheck",
				},
				SystemOut: "Unable to determine JobType for alert InvariantCheck",
			},
		}, nil
	}
	pendingIntervals := allEventIntervals.Filter(AlertPendingInNamespace(a.alertName, a.namespace))
	firingIntervals := allEventIntervals.Filter(AlertFiringInNamespace(a.alertName, a.namespace))

	state, message := a.failOrFlake(firingIntervals, pendingIntervals)

	switch a.alertName {
	case "KubePodNotReady":
		if state == fail && (kubePodNotReadyDueToImagePullBackoff(resourcesMap["events"], firingIntervals) || kubePodNotReadyDueToErrParsingSignature(resourcesMap["events"], firingIntervals)) {
			// Since this is due to imagePullBackoff, change the state to flake instead of fail
			state = flake
			break
		}

		// we only care about firing intervals that started before the nodes started updating or ended well after they finished
		nodeUpdates := allEventIntervals.Filter(monitorapi.NodeUpdate)
		if len(nodeUpdates) == 0 {
			break
		}
		earliestUpdateBegan := nodeUpdates[0].From
		lastUpdateFinished := nodeUpdates[len(nodeUpdates)-1].From.Add(15 * time.Minute) /* add grace period to wait for the alert to stop firing */
		firingIntervals = firingIntervals.Filter(
			monitorapi.Or(
				monitorapi.StartedBefore(earliestUpdateBegan),
				monitorapi.EndedAfter(lastUpdateFinished),
			),
		)

		// recheck the state and message.
		state, message = a.failOrFlake(firingIntervals, pendingIntervals)

	case "RedhatOperatorsCatalogError":
		if state == fail && redhatOperatorPodsNotPending(resourcesMap["pods"], firingIntervals) {
			state = flake
		}
	}

	switch state {
	case pass:
		return []*junitapi.JUnitTestCase{
			{
				Name: a.InvariantTestName(),
			},
		}, nil

	case flake:
		return []*junitapi.JUnitTestCase{
			{
				Name: a.InvariantTestName(),
			},
			{
				Name: a.InvariantTestName(),
				FailureOutput: &junitapi.FailureOutput{
					Output: message,
				},
				SystemOut: message,
			},
		}, nil

	case fail:
		return []*junitapi.JUnitTestCase{
			{
				Name: a.InvariantTestName(),
				FailureOutput: &junitapi.FailureOutput{
					Output: message,
				},
				SystemOut: message,
			},
		}, nil

	default:
		return nil, fmt.Errorf("unrecognized state: %v", state)
	}
}

func AlertFiringInNamespace(alertName, namespace string) monitorapi.EventIntervalMatchesFunc {
	return func(eventInterval monitorapi.Interval) bool {
		return monitorapi.And(
			func(eventInterval monitorapi.Interval) bool {
				locatorParts := monitorapi.LocatorParts(eventInterval.Locator)
				eventAlertName := monitorapi.AlertFrom(locatorParts)
				if eventAlertName != alertName {
					return false
				}
				if strings.Contains(eventInterval.Message, `alertstate="firing"`) {
					return true
				}
				return false
			},
			InNamespace(namespace),
		)(eventInterval)
	}
}

func AlertPendingInNamespace(alertName, namespace string) monitorapi.EventIntervalMatchesFunc {
	return func(eventInterval monitorapi.Interval) bool {
		return monitorapi.And(
			func(eventInterval monitorapi.Interval) bool {
				locatorParts := monitorapi.LocatorParts(eventInterval.Locator)
				eventAlertName := monitorapi.AlertFrom(locatorParts)
				if eventAlertName != alertName {
					return false
				}
				if strings.Contains(eventInterval.Message, `alertstate="pending"`) {
					return true
				}
				return false
			},
			InNamespace(namespace),
		)(eventInterval)
	}
}

// InNamespace if namespace == "", then every event matches, same as kube-api
func InNamespace(namespace string) func(event monitorapi.Interval) bool {
	return func(event monitorapi.Interval) bool {
		switch {
		case len(namespace) == 0:
			return true

		case namespace == platformidentification.NamespaceOther:
			eventNamespace := monitorapi.NamespaceFromLocator(event.Locator)
			return !platformidentification.KnownNamespaces.Has(eventNamespace)
		default:
			eventNamespace := monitorapi.NamespaceFromLocator(event.Locator)
			return eventNamespace == namespace
		}
	}
}

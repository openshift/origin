package intervalcreation

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	"github.com/openshift/origin/test/extended/testdata"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
)

type filenameBaseFunc func(timeSuffix string) string

type eventIntervalRenderer struct {
	name           string
	filenameBaseFn filenameBaseFunc
	filter         monitorapi.EventIntervalMatchesFunc
}

func NewSpyglassEventIntervalRenderer(name string, filter monitorapi.EventIntervalMatchesFunc) eventIntervalRenderer {
	return eventIntervalRenderer{
		name: name,
		filenameBaseFn: func(timeSuffix string) string {
			return fmt.Sprintf("e2e-timelines_%s%s", name, timeSuffix)
		},
		filter: filter,
	}
}

func NewNonSpyglassEventIntervalRenderer(name string, filter monitorapi.EventIntervalMatchesFunc) eventIntervalRenderer {
	return eventIntervalRenderer{
		name: name,
		filenameBaseFn: func(timeSuffix string) string {
			return fmt.Sprintf("e2e-timelines_%s%s", name, timeSuffix)
		},
		filter: filter,
	}
}

func (r eventIntervalRenderer) WriteRunData(artifactDir string, _ monitorapi.ResourcesMap, events monitorapi.Intervals, timeSuffix string) error {
	filenameBase := r.filenameBaseFn(timeSuffix)
	return r.writeEventData(artifactDir, filenameBase, events, timeSuffix)
}

func (r eventIntervalRenderer) writeEventData(artifactDir, filenameBase string, events monitorapi.Intervals, timeSuffix string) error {
	errs := []error{}
	interestingEvents := events.Filter(r.filter)

	if err := monitorserialization.EventsIntervalsToFile(filepath.Join(artifactDir, fmt.Sprintf("%s.json", filenameBase)), interestingEvents); err != nil {
		errs = append(errs, err)
	}

	eventIntervalsJSON, err := monitorserialization.EventsIntervalsToJSON(interestingEvents)
	if err != nil {
		errs = append(errs, err)
		return utilerrors.NewAggregate(errs)

	}
	e2eChartTemplate := testdata.MustAsset("e2echart/e2e-chart-template.html")
	e2eChartTitle := fmt.Sprintf("Intervals - %s%s", r.name, timeSuffix)
	e2eChartHTML := bytes.ReplaceAll(e2eChartTemplate, []byte("EVENT_INTERVAL_TITLE_GOES_HERE"), []byte(e2eChartTitle))
	e2eChartHTML = bytes.ReplaceAll(e2eChartHTML, []byte("EVENT_INTERVAL_JSON_GOES_HERE"), eventIntervalsJSON)
	e2eChartHTMLPath := filepath.Join(artifactDir, fmt.Sprintf("%s.html", filenameBase))
	if err := ioutil.WriteFile(e2eChartHTMLPath, e2eChartHTML, 0644); err != nil {
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}

func BelongsInEverything(eventInterval monitorapi.EventInterval) bool {
	if IsPodLifecycle(eventInterval) { // there are just too many
		return false
	}
	return true
}

func BelongsInSpyglass(eventInterval monitorapi.EventInterval) bool {
	if isLessInterestingAlert(eventInterval) {
		return false
	}
	if IsPodLifecycle(eventInterval) {
		return false
	}
	if IsPathologicalEvent(eventInterval) {
		return true
	}

	return true
}

func BelongsInOperatorRollout(eventInterval monitorapi.EventInterval) bool {
	if monitorapi.IsE2ETest(eventInterval.Locator) {
		return false
	}
	if IsPodLifecycle(eventInterval) {
		if isPlatformPodEvent(eventInterval) {
			return true
		}
		return false
	}

	return true
}

func BelongsInKubeAPIServer(eventInterval monitorapi.EventInterval) bool {
	if monitorapi.IsE2ETest(eventInterval.Locator) {
		return false
	}
	if isLessInterestingAlert(eventInterval) {
		return false
	}
	if IsPodLifecycle(eventInterval) {
		if isPlatformPodEvent(eventInterval) && isInterestingNamespace(eventInterval, kubeAPIServerDependentNamespaces) {
			return true
		}
		return false
	}
	if isInterestingNamespace(eventInterval, kubeControlPlaneNamespaces) {
		return true
	}

	return true
}

var pathologicalMessagePattern = regexp.MustCompile(`(?s)(.*) \((\d+) times\).*`)

// isPathologicalEvent returns true if the event message matches the pattern where the message
// says "(n times)"" where n is a number.  Even if n is less than the failure threshold, it is still
// counted as a pathological event (to be displayed in the spyglass chart).
func IsPathologicalEvent(eventInterval monitorapi.EventInterval) bool {
	return pathologicalMessagePattern.MatchString(eventInterval.Message)
}

func IsPodLifecycle(eventInterval monitorapi.EventInterval) bool {
	return strings.Contains(eventInterval.Message, "constructed/true")
}

func IsOriginalPodEvent(eventInterval monitorapi.EventInterval) bool {
	// constructed events are not original
	if strings.Contains(eventInterval.Message, "constructed/true") {
		return false
	}
	return strings.Contains(eventInterval.Locator, "pod/")
}

func isPlatformPodEvent(eventInterval monitorapi.EventInterval) bool {
	// only include pod events that were created in CreatePodIntervalsFromInstants
	if !strings.Contains(eventInterval.Message, "constructed/true") {
		return false
	}
	pod := monitorapi.PodFrom(eventInterval.Locator)
	if len(pod.UID) == 0 {
		return false
	}

	locatorParts := monitorapi.LocatorParts(eventInterval.Locator)
	namespace := monitorapi.NamespaceFrom(locatorParts)
	if strings.HasPrefix(namespace, "openshift-") {
		return true
	}

	return false
}

var kubeControlPlaneNamespaces = sets.NewString(
	"openshift-etcd-operator", "openshift-etcd",
	"openshift-kube-apiserver-operator", "openshift-kube-apiserver",
	"openshift-kube-controller-manager-operator", "openshift-kube-controller-manager",
	"openshift-kube-scheduler-operator", "openshift-kube-scheduler",
	"openshift-apiserver-operator", "openshift-apiserver",
	"openshift-authentication-operator", "openshift-authentication", "openshift-oauth-apiserver",
	"openshift-network-operator", "openshift-multus", "openshift-ovn-kubernetes",
)

var kubeAPIServerDependentNamespaces = sets.NewString(
	"openshift-etcd-operator", "openshift-etcd",
	"openshift-kube-apiserver-operator", "openshift-kube-apiserver",
	"openshift-apiserver-operator", "openshift-apiserver",
	"openshift-authentication-operator", "openshift-oauth-apiserver",
)

func isInterestingNamespace(eventInterval monitorapi.EventInterval, interestingNamespaces sets.String) bool {
	locatorParts := monitorapi.LocatorParts(eventInterval.Locator)
	namespace := monitorapi.NamespaceFrom(locatorParts)
	return interestingNamespaces.Has(namespace)
}

func isLessInterestingAlert(eventInterval monitorapi.EventInterval) bool {
	locatorParts := monitorapi.LocatorParts(eventInterval.Locator)
	alertName := monitorapi.AlertFrom(locatorParts)
	if len(alertName) == 0 {
		return false
	}
	if !strings.Contains(eventInterval.Message, "pending") {
		return false
	}
	if eventInterval.Level == monitorapi.Warning || eventInterval.Level == monitorapi.Error {
		return false
	}

	// some alerts are not interesting
	switch {
	case alertName == "KubePodNotReady":
		return true
	case alertName == "KubeContainerWaiting":
		return true
	}

	return false
}

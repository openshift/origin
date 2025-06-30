package timelineserializer

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
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

	if err := monitorserialization.IntervalsToFile(filepath.Join(artifactDir, fmt.Sprintf("%s.json", filenameBase)), interestingEvents); err != nil {
		errs = append(errs, err)
	}

	eventIntervalsJSON, err := monitorserialization.EventsIntervalsToJSON(interestingEvents)
	if err != nil {
		errs = append(errs, err)
		return utilerrors.NewAggregate(errs)

	}
	e2eChartTemplate := testdata.MustAsset("e2echart/e2e-chart-template.html")
	// choosing to intercept here because it should be temporary until TRT transitions to a new mechanism to display these intervals.
	if !strings.Contains(r.name, "spyglass") {
		e2eChartTemplate = testdata.MustAsset("e2echart/non-spyglass-e2e-chart-template.html")
	}
	e2eChartTitle := fmt.Sprintf("Intervals - %s%s", r.name, timeSuffix)
	e2eChartHTML := bytes.ReplaceAll(e2eChartTemplate, []byte("EVENT_INTERVAL_TITLE_GOES_HERE"), []byte(e2eChartTitle))
	e2eChartHTML = bytes.ReplaceAll(e2eChartHTML, []byte("EVENT_INTERVAL_JSON_GOES_HERE"), eventIntervalsJSON)
	e2eChartHTMLPath := filepath.Join(artifactDir, fmt.Sprintf("%s.html", filenameBase))
	if err := ioutil.WriteFile(e2eChartHTMLPath, e2eChartHTML, 0644); err != nil {
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}

func BelongsInEverything(eventInterval monitorapi.Interval) bool {
	return true
}

func BelongsInSpyglass(eventInterval monitorapi.Interval) bool {
	if isLessInterestingAlert(eventInterval) {
		return false
	}
	if IsPodLifecycle(eventInterval) {
		return false
	}
	if eventInterval.Source == monitorapi.SourcePodState {
		return false
	}
	// Pathologically repeating kube events:
	if eventInterval.Source == monitorapi.SourceKubeEvent {
		if eventInterval.Message.Annotations[monitorapi.AnnotationPathological] != "true" {
			return false
		}
		ns := monitorapi.NamespaceFromLocator(eventInterval.Locator)
		if strings.Contains(ns, "e2e") {
			return false
		}
	}
	return true
}

func BelongsInOperatorRollout(eventInterval monitorapi.Interval) bool {
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

func BelongsInKubeAPIServer(eventInterval monitorapi.Interval) bool {
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

func IsPodLifecycle(eventInterval monitorapi.Interval) bool {
	return eventInterval.Message.Annotations[monitorapi.AnnotationConstructed] == monitorapi.ConstructionOwnerPodLifecycle
}

func IsOriginalPodEvent(eventInterval monitorapi.Interval) bool {
	// constructed events are not original
	if len(eventInterval.Message.Annotations[monitorapi.AnnotationConstructed]) > 0 {
		return false
	}
	return eventInterval.Locator.HasKey(monitorapi.LocatorPodKey)
}

func isPlatformPodEvent(eventInterval monitorapi.Interval) bool {
	// only include pod events that were created in CreatePodIntervalsFromInstants
	if !(eventInterval.Source == monitorapi.SourcePodState) {
		return false
	}
	pod := monitorapi.PodFrom(eventInterval.Locator)
	if len(pod.UID) == 0 {
		return false
	}

	namespace := eventInterval.Locator.Keys[monitorapi.LocatorNamespaceKey]
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

func isInterestingNamespace(eventInterval monitorapi.Interval, interestingNamespaces sets.String) bool {
	return interestingNamespaces.Has(eventInterval.Locator.Keys[monitorapi.LocatorNamespaceKey])
}

func isLessInterestingAlert(eventInterval monitorapi.Interval) bool {
	if eventInterval.Source != monitorapi.SourceAlert {
		return false
	}
	alertName := eventInterval.Locator.Keys[monitorapi.LocatorAlertKey]

	// Normally we don't want to display pending alerts, but allow some to show so we can see high CPU
	// as it's detected.
	if eventInterval.Message.Annotations[monitorapi.AnnotationAlertState] == "pending" {
		switch alertName {
		// Alerts we DO want to see pending intervals for in the minimal spyglass file
		case "ExtremelyHighIndividualControlPlaneCPU":
		case "HighOverallControlPlaneCPU":
		case "HighOverallControlPlaneMemory":
		case "SystemMemoryExceedsReservation":
		case "etcdHighCommitDurations":
		case "etcdGRPCRequestsSlow":
		case "etcdHighFsyncDurations":
		case "etcdDatabaseHighFragmentationRatio":
		default:
			// Skip pending alerts other than the specific ones listed above:
			return true
		}
	}

	if len(alertName) == 0 {
		return true
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

package intervalcreation

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

type eventIntervalRenderer struct {
	name   string
	filter monitorapi.EventIntervalMatchesFunc
}

func NewEventIntervalRenderer(name string, filter monitorapi.EventIntervalMatchesFunc) eventIntervalRenderer {
	return eventIntervalRenderer{
		name:   name,
		filter: filter,
	}
}

func (r eventIntervalRenderer) WriteEventData(artifactDir string, events monitorapi.Intervals, timeSuffix string) error {
	errs := []error{}
	interestingEvents := events.Filter(r.filter)

	if err := monitorserialization.EventsIntervalsToFile(filepath.Join(artifactDir, fmt.Sprintf("e2e-intervals_%s%s.json", r.name, timeSuffix)), interestingEvents); err != nil {
		errs = append(errs, err)
	}

	eventIntervalsJSON, err := monitorserialization.EventsIntervalsToJSON(interestingEvents)
	if err != nil {
		errs = append(errs, err)
		return utilerrors.NewAggregate(errs)

	}
	e2eChartTemplate := testdata.MustAsset("e2echart/e2e-chart-template.html")
	e2eChartHTML := bytes.ReplaceAll(e2eChartTemplate, []byte("EVENT_INTERVAL_JSON_GOES_HERE"), eventIntervalsJSON)
	e2eChartHTMLPath := filepath.Join(artifactDir, fmt.Sprintf("e2e-intervals_%s%s.html", r.name, timeSuffix))
	if err := ioutil.WriteFile(e2eChartHTMLPath, e2eChartHTML, 0644); err != nil {
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}

func BelongsInEverything(eventInterval monitorapi.EventInterval) bool {
	return true
}

func BelongsInSpyglass(eventInterval monitorapi.EventInterval) bool {
	if isLessInterestingAlert(eventInterval) {
		return false
	}

	return true
}

func BelongsInOperatorRollout(eventInterval monitorapi.EventInterval) bool {
	if monitorapi.IsE2ETest(eventInterval.Locator) {
		return false
	}

	return true
}

func BelongsInKubeAPIServer(eventInterval monitorapi.EventInterval) bool {
	if monitorapi.IsE2ETest(eventInterval.Locator) {
		return false
	}
	if isInKubeControlPlaneNamespace(eventInterval) {
		return true
	}
	if isLessInterestingAlert(eventInterval) {
		return false
	}

	return true
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

func isInKubeControlPlaneNamespace(eventInterval monitorapi.EventInterval) bool {
	locatorParts := monitorapi.LocatorParts(eventInterval.Locator)
	namespace := monitorapi.NamespaceFrom(locatorParts)
	return kubeControlPlaneNamespaces.Has(namespace)
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

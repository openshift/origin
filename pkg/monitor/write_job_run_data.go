package monitor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	"github.com/openshift/origin/test/extended/testdata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

// WriteRunDataToArtifactsDir attempts to write useful run data to the specified directory.
func WriteRunDataToArtifactsDir(artifactDir string, monitor *Monitor, events monitorapi.Intervals, timeSuffix string) error {
	errors := []error{}
	if err := monitorserialization.EventsToFile(filepath.Join(artifactDir, fmt.Sprintf("e2e-events%s.json", timeSuffix)), events); err != nil {
		errors = append(errors, err)
	} else {
	}
	if err := monitorserialization.EventsIntervalsToFile(filepath.Join(artifactDir, fmt.Sprintf("e2e-intervals%s.json", timeSuffix)), events); err != nil {
		errors = append(errors, err)
	}
	if eventIntervalsJSON, err := monitorserialization.EventsIntervalsToJSON(events); err == nil {
		e2eChartTemplate := testdata.MustAsset("e2echart/e2e-chart-template.html")
		e2eChartHTML := bytes.ReplaceAll(e2eChartTemplate, []byte("EVENT_INTERVAL_JSON_GOES_HERE"), eventIntervalsJSON)
		e2eChartHTMLPath := filepath.Join(artifactDir, fmt.Sprintf("e2e-intervals%s.html", timeSuffix))
		if err := ioutil.WriteFile(e2eChartHTMLPath, e2eChartHTML, 0644); err != nil {
			errors = append(errors, err)
		}
	} else {
		errors = append(errors, err)
	}

	// write out the current state of resources that we explicitly tracked.
	resourcesMap := monitor.CurrentResourceState()
	for resourceType, instanceMap := range resourcesMap {
		targetFile := fmt.Sprintf("resource-%s%s.zip", resourceType, timeSuffix)
		if err := monitorserialization.InstanceMapToFile(filepath.Join(artifactDir, targetFile), resourceType, instanceMap); err != nil {
			errors = append(errors, err)
		}
	}

	backendDisruption := computeDisruptionData(events)
	if err := writeDisruptionData(filepath.Join(artifactDir, fmt.Sprintf("backend-disruption%s.json", timeSuffix)), backendDisruption); err != nil {
		errors = append(errors, err)
	}

	alertData := computeAlertData(events)
	if err := writeAlertData(filepath.Join(artifactDir, fmt.Sprintf("alerts%s.json", timeSuffix)), alertData); err != nil {
		errors = append(errors, err)
	}

	return utilerrors.NewAggregate(errors)
}

type BackendDisruptionList struct {
	// BackendDisruptions is keyed by name to make the consumption easier
	BackendDisruptions map[string]*BackendDisruption
}

type BackendDisruption struct {
	// Name ensure self-identification
	Name string
	// ConnectionType is New or Reused
	ConnectionType     string
	DisruptedDuration  metav1.Duration
	DisruptionMessages []string
}

func writeDisruptionData(filename string, disruption *BackendDisruptionList) error {
	jsonContent, err := json.MarshalIndent(disruption, "", "    ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, jsonContent, 0644)
}

func computeDisruptionData(events monitorapi.Intervals) *BackendDisruptionList {
	ret := &BackendDisruptionList{
		BackendDisruptions: map[string]*BackendDisruption{},
	}

	for locator, name := range BackendDisruptionLocatorsToName {
		disruptionDuration, disruptionMessages, connectionType := monitorapi.BackendDisruptionSeconds(locator, events)
		ret.BackendDisruptions[name] = &BackendDisruption{
			Name:               name,
			ConnectionType:     connectionType,
			DisruptedDuration:  metav1.Duration{Duration: disruptionDuration},
			DisruptionMessages: disruptionMessages,
		}
	}

	return ret
}

type AlertList struct {
	// Alerts is keyed by name to make the consumption easier
	Alerts []Alert
}

// name and namespace are consistent (usually) for every CI run
type AlertKey struct {
	Name      string
	Namespace string
	Level     AlertLevel
}

type AlertKeyByAlert []AlertKey

func (a AlertKeyByAlert) Len() int      { return len(a) }
func (a AlertKeyByAlert) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a AlertKeyByAlert) Less(i, j int) bool {
	if strings.Compare(a[i].Name, a[j].Name) < 0 {
		return true
	}
	if strings.Compare(a[i].Namespace, a[j].Namespace) < 0 {
		return true
	}
	if strings.Compare(string(a[i].Level), string(a[j].Level)) < 0 {
		return true
	}

	return false
}

type AlertLevel string

var (
	UnknownAlertLevel  AlertLevel = "Unknown"
	WarningAlertLevel  AlertLevel = "Warning"
	CriticalAlertLevel AlertLevel = "Critical"
)

type Alert struct {
	AlertKey `json:",inline"`
	Duration metav1.Duration
}

type AlertByKey []Alert

func (a AlertByKey) Len() int      { return len(a) }
func (a AlertByKey) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a AlertByKey) Less(i, j int) bool {
	if strings.Compare(a[i].Name, a[j].Name) < 0 {
		return true
	}
	if strings.Compare(a[i].Namespace, a[j].Namespace) < 0 {
		return true
	}
	if strings.Compare(string(a[i].Level), string(a[j].Level)) < 0 {
		return true
	}

	return false
}

func writeAlertData(filename string, alertData *AlertList) error {
	jsonContent, err := json.MarshalIndent(alertData, "", "    ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, jsonContent, 0644)
}

func getAlertLevelFromEvent(event monitorapi.EventInterval) AlertLevel {
	if event.Level == monitorapi.Error {
		return CriticalAlertLevel
	}
	if event.Level == monitorapi.Warning {
		return WarningAlertLevel
	}
	return UnknownAlertLevel
}

func computeAlertData(events monitorapi.Intervals) *AlertList {
	alertEvents := events.Filter(
		func(eventInterval monitorapi.EventInterval) bool {
			locator := monitorapi.LocatorParts(eventInterval.Locator)
			alertName := monitorapi.AlertFrom(locator)
			if len(alertName) == 0 {
				return false
			}
			if alertName == "Watchdog" {
				return false
			}
			// skip everything but warning and critical
			if getAlertLevelFromEvent(eventInterval) == UnknownAlertLevel {
				return false
			}
			return true
		},
	)

	alertMap := map[AlertKey]*Alert{}
	for _, alertInterval := range alertEvents {
		alertLocator := monitorapi.LocatorParts(alertInterval.Locator)
		alertKey := AlertKey{
			Name:      monitorapi.AlertFrom(alertLocator),
			Namespace: monitorapi.NamespaceFrom(alertLocator),
			Level:     getAlertLevelFromEvent(alertInterval),
		}
		alert, ok := alertMap[alertKey]
		if !ok {
			alert = &Alert{
				AlertKey: alertKey,
			}
		}
		//the same alert can fire multiple times, so we add all the durations together
		alert.Duration = metav1.Duration{Duration: alert.Duration.Duration + alertInterval.To.Sub(alertInterval.From)}
		alertMap[alertKey] = alert
	}

	alertList := []Alert{}
	for _, alert := range alertMap {
		alertList = append(alertList, *alert)
	}
	sort.Stable(AlertByKey(alertList))

	return &AlertList{
		Alerts: alertList,
	}
}

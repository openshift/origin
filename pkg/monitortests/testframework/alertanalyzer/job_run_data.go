package alertanalyzer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"
	"time"

	allowedalerts2 "github.com/openshift/origin/pkg/monitortestlibrary/allowedalerts"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func writeAlertDataForJobRun(artifactDir string, _ monitorapi.ResourcesMap, events monitorapi.Intervals, timeSuffix string) error {
	alertData := computeAlertData(events)
	addMissingAlertsForLevel(alertData, WarningAlertLevel)
	addMissingAlertsForLevel(alertData, CriticalAlertLevel)

	return writeAlertData(filepath.Join(artifactDir, fmt.Sprintf("alerts%s.json", timeSuffix)), alertData)
}

func addMissingAlertsForLevel(alertList *AlertList, level AlertLevel) {
	wellKnownAlerts := sets.NewString()
	for _, alertTest := range allowedalerts2.AllAlertTests(&platformidentification.JobType{}, allowedalerts2.DefaultAllowances) {
		wellKnownAlerts.Insert(alertTest.AlertName())
	}
	alertsFound := sets.NewString()
	for _, alert := range alertList.Alerts {
		if alert.Level == level {
			alertsFound.Insert(alert.Name)
		}
	}
	for _, missingAlert := range wellKnownAlerts.Difference(alertsFound).List() {
		alertList.Alerts = append(alertList.Alerts, Alert{
			AlertKey: AlertKey{
				Name:  missingAlert,
				Level: level,
			},
			Duration: metav1.Duration{Duration: 0 * time.Second},
		})
	}
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

func getAlertLevelFromEvent(event monitorapi.Interval) AlertLevel {
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
		func(eventInterval monitorapi.Interval) bool {
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

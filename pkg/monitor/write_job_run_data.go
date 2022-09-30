package monitor

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/kustomize/kyaml/sets"
)

func WriteEventsForJobRun(artifactDir string, _ monitorapi.ResourcesMap, events monitorapi.Intervals, timeSuffix string) error {
	return monitorserialization.EventsToFile(filepath.Join(artifactDir, fmt.Sprintf("e2e-events%s.json", timeSuffix)), events)
}

func WriteTrackedResourcesForJobRun(artifactDir string, recordedResources monitorapi.ResourcesMap, _ monitorapi.Intervals, timeSuffix string) error {
	errors := []error{}

	// write out the current state of resources that we explicitly tracked.
	for resourceType, instanceMap := range recordedResources {
		targetFile := fmt.Sprintf("resource-%s%s.zip", resourceType, timeSuffix)
		if err := monitorserialization.InstanceMapToFile(filepath.Join(artifactDir, targetFile), resourceType, instanceMap); err != nil {
			errors = append(errors, err)
		}
	}

	return utilerrors.NewAggregate(errors)
}

func WriteBackendDisruptionForJobRun(artifactDir string, _ monitorapi.ResourcesMap, events monitorapi.Intervals, timeSuffix string) error {
	backendDisruption := computeDisruptionData(events)
	return writeDisruptionData(filepath.Join(artifactDir, fmt.Sprintf("backend-disruption%s.json", timeSuffix)), backendDisruption)
}

type BackendDisruptionList struct {
	// BackendDisruptions is keyed by name to make the consumption easier
	BackendDisruptions map[string]*BackendDisruption
}

type BackendDisruption struct {
	// Name ensure self-identification, it includes the connection type
	Name string
	// BackendName is the name of backend.  It is the same across all connection types.
	BackendName string
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

func computeDisruptionData(eventIntervals monitorapi.Intervals) *BackendDisruptionList {
	ret := &BackendDisruptionList{
		BackendDisruptions: map[string]*BackendDisruption{},
	}

	allBackendLocators := sets.String{}
	allDisruptionEventsIntervals := eventIntervals.Filter(
		monitorapi.And(
			monitorapi.IsDisruptionEvent,
			monitorapi.Or(
				monitorapi.IsErrorEvent, // ignore Warning events, we use these for disruption we don't actually think was from the cluster under test (i.e. DNS)
				monitorapi.IsInfoEvent,  // Must keep including info disruption events as 0s disruptions don't get recorded otherwise
			),
		),
	)
	for _, eventInterval := range allDisruptionEventsIntervals {
		allBackendLocators.Insert(eventInterval.Locator)
	}

	for _, locator := range allBackendLocators.List() {
		locatorParts := monitorapi.LocatorParts(locator)
		disruptionBackend := monitorapi.DisruptionFrom(locatorParts)
		connectionType := monitorapi.DisruptionConnectionTypeFrom(locatorParts)
		aggregatedDisruptionName := strings.ToLower(fmt.Sprintf("%s-%s-connections", disruptionBackend, connectionType))

		disruptionDuration, disruptionMessages, connectionType := monitorapi.BackendDisruptionSeconds(locator, allDisruptionEventsIntervals)
		ret.BackendDisruptions[aggregatedDisruptionName] = &BackendDisruption{
			Name:               aggregatedDisruptionName,
			BackendName:        disruptionBackend,
			ConnectionType:     strings.Title(connectionType),
			DisruptedDuration:  metav1.Duration{Duration: disruptionDuration},
			DisruptionMessages: disruptionMessages,
		}
	}

	return ret
}

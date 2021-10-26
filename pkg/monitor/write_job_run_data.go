package monitor

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	"github.com/openshift/origin/test/extended/testdata"
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

	return utilerrors.NewAggregate(errors)
}

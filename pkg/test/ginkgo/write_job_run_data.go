package ginkgo

import (
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type RunDataWriter interface {
	WriteRunData(artifactDir string, monitor *monitor.Monitor, events monitorapi.Intervals, timeSuffix string) error
}

type EventDataWriter interface {
	WriteEventData(artifactDir string, events monitorapi.Intervals, timeSuffix string) error
}

type RunDataWriterFunc func(artifactDir string, monitor *monitor.Monitor, events monitorapi.Intervals, timeSuffix string) error

func (fn RunDataWriterFunc) WriteRunData(artifactDir string, monitor *monitor.Monitor, events monitorapi.Intervals, timeSuffix string) error {
	return fn(artifactDir, monitor, events, timeSuffix)
}

func AdaptEventDataWriter(w EventDataWriter) RunDataWriterFunc {
	return func(artifactDir string, monitor *monitor.Monitor, events monitorapi.Intervals, timeSuffix string) error {
		return w.WriteEventData(artifactDir, events, timeSuffix)
	}
}

// WriteRunDataToArtifactsDir attempts to write useful run data to the specified directory.
func (opt *Options) WriteRunDataToArtifactsDir(artifactDir string, monitor *monitor.Monitor, events monitorapi.Intervals, timeSuffix string) error {
	errs := []error{}

	for _, writer := range opt.RunDataWriters {
		currErr := writer.WriteRunData(artifactDir, monitor, events, timeSuffix)
		if currErr != nil {
			errs = append(errs, currErr)
		}
	}
	return utilerrors.NewAggregate(errs)
}

package ginkgo

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/intervalcreation"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	"github.com/openshift/origin/pkg/synthetictests/allowedalerts"
	"github.com/openshift/origin/test/extended/util/disruption/controlplane"
	"github.com/openshift/origin/test/extended/util/disruption/frontends"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type RunDataWriter interface {
	WriteRunData(artifactDir string, recordedResources monitorapi.ResourcesMap, events monitorapi.Intervals, timeSuffix string) error
}

type RunDataWriterFunc func(artifactDir string, recordedResources monitorapi.ResourcesMap, events monitorapi.Intervals, timeSuffix string) error

func (fn RunDataWriterFunc) WriteRunData(artifactDir string, recordedResources monitorapi.ResourcesMap, events monitorapi.Intervals, timeSuffix string) error {
	return fn(artifactDir, recordedResources, events, timeSuffix)
}

type MonitorEventsOptions struct {
	monitor   *monitor.Monitor
	startTime *time.Time
	endTime   *time.Time
	// recordedEvents is written during End
	recordedEvents monitorapi.Intervals
	// recordedResource is written during End
	recordedResources monitorapi.ResourcesMap

	Recorders      []monitor.StartEventIntervalRecorderFunc
	RunDataWriters []RunDataWriter
	Out            io.Writer
	ErrOut         io.Writer
}

func NewMonitorEventsOptions(out io.Writer, errOut io.Writer) *MonitorEventsOptions {
	return &MonitorEventsOptions{
		Recorders: []monitor.StartEventIntervalRecorderFunc{
			controlplane.StartAllAPIMonitoring,
			frontends.StartAllIngressMonitoring,
		},
		RunDataWriters: []RunDataWriter{
			// these produce the various intervals.  Different intervals focused on inspecting different problem spaces.
			intervalcreation.NewSpyglassEventIntervalRenderer("everything", intervalcreation.BelongsInEverything),
			intervalcreation.NewSpyglassEventIntervalRenderer("spyglass", intervalcreation.BelongsInSpyglass),
			// TODO add visualization of individual apiserver containers and their readiness on this page
			intervalcreation.NewSpyglassEventIntervalRenderer("kube-apiserver", intervalcreation.BelongsInKubeAPIServer),
			intervalcreation.NewSpyglassEventIntervalRenderer("operators", intervalcreation.BelongsInOperatorRollout),
			intervalcreation.NewPodEventIntervalRenderer(),
			intervalcreation.NewIngressServicePodIntervalRenderer(),

			RunDataWriterFunc(monitor.WriteEventsForJobRun),
			RunDataWriterFunc(monitor.WriteTrackedResourcesForJobRun),
			RunDataWriterFunc(monitor.WriteBackendDisruptionForJobRun),
			RunDataWriterFunc(allowedalerts.WriteAlertDataForJobRun),
		},
		Out:    out,
		ErrOut: errOut,
	}
}

func (o *MonitorEventsOptions) Start(ctx context.Context, restConfig *rest.Config) (monitor.Recorder, error) {
	if o.monitor != nil {
		return nil, fmt.Errorf("already started")
	}
	t := time.Now()
	o.startTime = &t

	m, err := monitor.Start(ctx, restConfig,
		[]monitor.StartEventIntervalRecorderFunc{
			controlplane.StartAllAPIMonitoring,
			frontends.StartAllIngressMonitoring,
		},
	)
	if err != nil {
		return nil, err
	}
	o.monitor = m

	return m, nil
}

func (o *MonitorEventsOptions) End(ctx context.Context, restConfig *rest.Config, artifactDir string) error {
	if o.monitor == nil {
		return fmt.Errorf("not started")
	}
	if o.endTime != nil {
		return fmt.Errorf("already ended")
	}

	t := time.Now()
	o.endTime = &t
	o.recordedResources = o.monitor.CurrentResourceState()

	var err error
	fromTime, endTime := time.Time{}, time.Time{}
	events := o.monitor.Intervals(fromTime, endTime)
	// this happens before calculation because events collected here could be used to drive later calculations
	events, err = intervalcreation.InsertIntervalsFromCluster(ctx, restConfig, events, o.recordedResources, fromTime, endTime)
	if err != nil {
		return fmt.Errorf("InsertIntervalsFromClusterError: %w", err)
	}
	// add events from alerts so we can create the intervals
	alertEventIntervals, err := monitor.FetchEventIntervalsForAllAlerts(ctx, restConfig, *o.startTime)
	if err != nil {
		return fmt.Errorf("AlertErr: %w", err)
	}
	events = append(events, alertEventIntervals...)
	events = intervalcreation.InsertCalculatedIntervals(events, o.recordedResources, fromTime, endTime)

	// read events from other test processes (individual tests for instance) that happened during this run.
	// this happens during upgrade tests to pass information back to the main monitor.
	if len(artifactDir) > 0 {
		var additionalEvents monitorapi.Intervals
		filepath.WalkDir(artifactDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasPrefix(d.Name(), "AdditionalEvents_") {
				return nil
			}
			saved, _ := monitorserialization.EventsFromFile(path)
			additionalEvents = append(additionalEvents, saved...)
			return nil
		})
		if len(additionalEvents) > 0 {
			events = append(events, additionalEvents.Cut(*o.startTime, *o.endTime)...)
		}
	}

	sort.Sort(events)
	events.Clamp(*o.startTime, *o.endTime)

	o.recordedEvents = events

	return nil
}

func (o *MonitorEventsOptions) GetEvents() monitorapi.Intervals {
	return o.recordedEvents
}

func (o *MonitorEventsOptions) GetRecordedResources() monitorapi.ResourcesMap {
	return o.recordedResources
}

func (o *MonitorEventsOptions) GetStartTime() *time.Time {
	return o.startTime
}

// WriteRunDataToArtifactsDir attempts to write useful run data to the specified directory.
func (o *MonitorEventsOptions) WriteRunDataToArtifactsDir(artifactDir string, timeSuffix string) error {
	if o.endTime == nil {
		return fmt.Errorf("not ended")
	}

	errs := []error{}

	// use custom sorting here so that we can prioritize the sort order to make the intervals html page as readable
	// as possible. This makes the events *not* sorted by time.
	events := make([]monitorapi.EventInterval, len(o.recordedEvents))
	for i := range o.recordedEvents {
		events[i] = o.recordedEvents[i]
	}
	sort.Stable(monitorapi.ByTimeWithNamespacedPods(events))

	for _, writer := range o.RunDataWriters {
		currErr := writer.WriteRunData(artifactDir, o.recordedResources, events, timeSuffix)
		if currErr != nil {
			errs = append(errs, currErr)
		}
	}
	return utilerrors.NewAggregate(errs)
}

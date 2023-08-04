package monitor

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/origin/pkg/test"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/sirupsen/logrus"

	"github.com/openshift/origin/pkg/invariants"

	configclientset "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/disruption/backend"
	"github.com/openshift/origin/pkg/disruption/backend/sampler"
	"github.com/openshift/origin/pkg/monitor/apiserveravailability"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitor/shutdown"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// Monitor records events that have occurred in memory and can also periodically
// sample results.
type Monitor struct {
	adminKubeConfig                  *rest.Config
	additionalEventIntervalRecorders []StartEventIntervalRecorderFunc
	invariantRegistry                invariants.InvariantRegistry
	storageDir                       string

	recorder monitorapi.Recorder
	junits   []*junitapi.JUnitTestCase

	lock      sync.Mutex
	stopFn    context.CancelFunc
	startTime time.Time
	stopTime  time.Time
}

// NewMonitor creates a monitor with the default sampling interval.
func NewMonitor(adminKubeConfig *rest.Config, storageDir string, additionalEventIntervalRecorders []StartEventIntervalRecorderFunc, invariantRegistry invariants.InvariantRegistry) *Monitor {
	return &Monitor{
		adminKubeConfig:                  adminKubeConfig,
		additionalEventIntervalRecorders: additionalEventIntervalRecorders,
		recorder:                         NewRecorder(),
		invariantRegistry:                invariantRegistry,
		storageDir:                       storageDir,
	}
}

var _ Interface = &Monitor{}

// Start begins monitoring the cluster referenced by the default kube configuration until context is finished.
func (m *Monitor) Start(ctx context.Context) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.stopFn != nil {
		return fmt.Errorf("monitor already started")
	}
	ctx, m.stopFn = context.WithCancel(ctx)
	m.startTime = time.Now()

	localJunits, err := m.invariantRegistry.StartCollection(ctx, m.adminKubeConfig, m.recorder)
	if err != nil {
		return err
	}
	m.junits = append(m.junits, localJunits...)

	client, err := kubernetes.NewForConfig(m.adminKubeConfig)
	if err != nil {
		return err
	}
	configClient, err := configclientset.NewForConfig(m.adminKubeConfig)
	if err != nil {
		return err
	}

	for _, additionalEventIntervalRecorder := range m.additionalEventIntervalRecorders {
		if err := additionalEventIntervalRecorder(ctx, m.recorder, m.adminKubeConfig, backend.ExternalLoadBalancerType); err != nil {
			return err
		}
	}

	// read the state of the cluster apiserver client access issues *before* any test (like upgrade) begins
	intervals, err := apiserveravailability.APIServerAvailabilityIntervalsFromCluster(client, time.Time{}, time.Time{})
	if err != nil {
		klog.Errorf("error reading initial apiserver availability: %v", err)
	}
	m.recorder.AddIntervals(intervals...)

	startNodeMonitoring(ctx, m.recorder, client)
	startEventMonitoring(ctx, m.recorder, client)
	shutdown.StartMonitoringGracefulShutdownEvents(ctx, m.recorder, client)

	// add interval creation at the same point where we add the monitors
	startClusterOperatorMonitoring(ctx, m.recorder, configClient)
	return nil
}

func (m *Monitor) Stop(ctx context.Context) error {
	fmt.Fprintf(os.Stderr, "Shutting down the monitor\n")

	m.lock.Lock()
	defer m.lock.Unlock()
	if m.stopFn == nil {
		return fmt.Errorf("monitor not started")
	}
	m.stopFn()
	m.stopFn = nil

	// we don't want this method to return until all te additional recorders and invariants have completed processing.
	// to do this correctly, we need closure channels or some kind of mechanism.
	// rather than properly wire this through, we'll wait until the backend disruption consumers timeout after the producer
	// close.
	// TODO once we have converted the backendsamplers to invariant tests, we can properly wait for completion
	time.Sleep(70 * time.Second)

	// set the stop time for after we finished.
	m.stopTime = time.Now()

	fmt.Fprintf(os.Stderr, "Collecting data.\n")
	collectedIntervals, collectionJunits, err := m.invariantRegistry.CollectData(ctx, m.storageDir, m.startTime, m.stopTime)
	if err != nil {
		// these errors are represented as junit, always continue to the next step
		fmt.Fprintf(os.Stderr, "Error collecting data, continuing, junit will reflect this. %v\n", err)
	}
	m.recorder.AddIntervals(collectedIntervals...)
	m.junits = append(m.junits, collectionJunits...)

	fmt.Fprintf(os.Stderr, "Computing intervals.\n")
	computedIntervals, computedJunit, err := m.invariantRegistry.ConstructComputedIntervals(
		ctx,
		m.recorder.Intervals(time.Time{}, time.Time{}), // compute intervals based on *all* the intervals.
		m.recorder.CurrentResourceState(),
		m.startTime, // still allow computation to understand the begining and end for bounding.
		m.stopTime)  // still allow computation to understand the begining and end for bounding.
	if err != nil {
		// these errors are represented as junit, always continue to the next step
		fmt.Fprintf(os.Stderr, "Error computing intervals, continuing, junit will reflect this. %v\n", err)
	}
	m.recorder.AddIntervals(computedIntervals...)
	m.junits = append(m.junits, computedJunit...)

	fmt.Fprintf(os.Stderr, "Evaluating tests.\n")
	invariantJunits, err := m.invariantRegistry.EvaluateTestsFromConstructedIntervals(
		ctx,
		m.recorder.Intervals(m.startTime, m.stopTime), // evaluate the tests on the intervals during our active time.
	)
	if err != nil {
		// these errors are represented as junit, always continue to the next step
		fmt.Fprintf(os.Stderr, "Error evaluating tests, continuing, junit will reflect this. %v\n", err)
	}
	m.junits = append(m.junits, invariantJunits...)

	fmt.Fprintf(os.Stderr, "Cleaning up.\n")
	cleanupJunits, err := m.invariantRegistry.Cleanup(ctx)
	if err != nil {
		// these errors are represented as junit, always continue to the next step
		fmt.Fprintf(os.Stderr, "Error cleaning up, continuing, junit will reflect this. %v\n", err)
	}
	m.junits = append(m.junits, cleanupJunits...)

	return nil
}

func (m *Monitor) SerializeResults(ctx context.Context, junitSuiteName, timeSuffix string) error {
	fmt.Fprintf(os.Stderr, "Serializing results.\n")
	m.lock.Lock()
	defer m.lock.Unlock()

	// don't bound the intervals that we return
	finalIntervals := m.recorder.Intervals(time.Time{}, time.Time{})
	finalResources := m.recorder.CurrentResourceState()
	// TODO stop taking timesuffix as an arg and make this authoritative.
	//timeSuffix := fmt.Sprintf("_%s", time.Now().UTC().Format("20060102-150405"))

	eventDir := filepath.Join(m.storageDir, monitorapi.EventDir)
	if err := os.MkdirAll(eventDir, os.ModePerm); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create monitor-events directory, err: %v\n", err)
		return err
	}

	fmt.Fprintf(os.Stderr, "Writing to storage.\n")
	invariantJunits, err := m.invariantRegistry.WriteContentToStorage(
		ctx,
		m.storageDir,
		timeSuffix,
		finalIntervals,
		finalResources,
	)
	if err != nil {
		// these errors are represented as junit, always continue to the next step
		fmt.Fprintf(os.Stderr, "Error writing to storage, continuing, junit will reflect this. %v\n", err)
	}
	m.junits = append(m.junits, invariantJunits...)

	fmt.Fprintf(os.Stderr, "Doing cleanup that needs to be moved.\n")
	if err := sampler.TearDownInClusterMonitors(m.adminKubeConfig); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write events from in-cluster monitors, err: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "Uploading to loki.\n")
	if err := UploadIntervalsToLoki(finalIntervals); err != nil {
		// Best effort, we do not want to error out here:
		logrus.WithError(err).Warn("unable to upload intervals to loki")
	}

	fmt.Fprintf(os.Stderr, "Writing junits.\n")
	if err := m.serializeJunit(ctx, m.storageDir, junitSuiteName, timeSuffix); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write junit xml, err: %v\n", err)
		return err
	}

	return nil
}

func (m *Monitor) serializeJunit(ctx context.Context, storageDir, junitSuiteName, fileSuffix string) error {
	junitSuite := junitapi.JUnitTestSuite{
		Name:       junitSuiteName,
		NumTests:   0,
		NumSkipped: 0,
		NumFailed:  0,
		Duration:   0,
		Properties: nil,
		TestCases:  nil,
		Children:   nil,
	}
	for i := range m.junits {
		currJunit := m.junits[i]

		junitSuite.NumTests++
		if currJunit.FailureOutput != nil {
			junitSuite.NumFailed++
		} else if currJunit.SkipMessage != nil {
			junitSuite.NumSkipped++
		}
		junitSuite.TestCases = append(junitSuite.TestCases, currJunit)
	}

	out, err := xml.Marshal(junitSuite)
	if err != nil {
		return err
	}
	filePrefix := "e2e-invariants"
	path := filepath.Join(storageDir, fmt.Sprintf("%s_%s.xml", filePrefix, fileSuffix))
	fmt.Fprintf(os.Stderr, "Writing JUnit report to %s\n", path)
	return os.WriteFile(path, test.StripANSI(out), 0640)
}

func (m *Monitor) CurrentResourceState() monitorapi.ResourcesMap {
	return m.recorder.CurrentResourceState()
}

func (m *Monitor) RecordResource(resourceType string, obj runtime.Object) {
	m.recorder.RecordResource(resourceType, obj)
}

// Record captures one or more conditions at the current time. All conditions are recorded
// in monotonic order as EventInterval objects.
func (m *Monitor) Record(conditions ...monitorapi.Condition) {
	m.recorder.Record(conditions...)
}

// AddIntervals provides a mechanism to directly inject eventIntervals
func (m *Monitor) AddIntervals(eventIntervals ...monitorapi.Interval) {
	m.recorder.AddIntervals(eventIntervals...)
}

// StartInterval inserts a record at time t with the provided condition and returns an opaque
// locator to the interval. The caller may close the sample at any point by invoking EndInterval().
func (m *Monitor) StartInterval(t time.Time, condition monitorapi.Condition) int {
	return m.recorder.StartInterval(t, condition)
}

// EndInterval updates the To of the interval started by StartInterval if it is greater than
// the from.
func (m *Monitor) EndInterval(startedInterval int, t time.Time) {
	m.recorder.EndInterval(startedInterval, t)
}

// RecordAt captures one or more conditions at the provided time. All conditions are recorded
// as EventInterval objects.
func (m *Monitor) RecordAt(t time.Time, conditions ...monitorapi.Condition) {
	m.recorder.RecordAt(t, conditions...)
}

// Intervals returns all events that occur between from and to, including
// any sampled conditions that were encountered during that period.
// Intervals are returned in order of their occurrence. The returned slice
// is a copy of the monitor's state and is safe to update.
func (m *Monitor) Intervals(from, to time.Time) monitorapi.Intervals {
	return m.recorder.Intervals(from, to)
}

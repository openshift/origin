package monitor

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"k8s.io/apimachinery/pkg/util/sets"

	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"

	"github.com/openshift/origin/pkg/test"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/client-go/rest"
)

type Monitor struct {
	adminKubeConfig     *rest.Config
	monitorTestRegistry monitortestframework.MonitorTestRegistry
	storageDir          string

	recorder monitorapi.Recorder
	junits   []*junitapi.JUnitTestCase

	lock      sync.Mutex
	stopFn    context.CancelFunc
	startTime time.Time
	stopTime  time.Time
}

// NewMonitor creates a monitor with the default sampling interval.
func NewMonitor(
	recorder monitorapi.Recorder,
	adminKubeConfig *rest.Config,
	storageDir string,
	monitorTestRegistry monitortestframework.MonitorTestRegistry) Interface {
	return &Monitor{
		adminKubeConfig:     adminKubeConfig,
		recorder:            recorder,
		monitorTestRegistry: monitorTestRegistry,
		storageDir:          storageDir,
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

	localJunits, err := m.monitorTestRegistry.StartCollection(ctx, m.adminKubeConfig, m.recorder)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting data collection, continuing, junit will reflect this. %v\n", err)
	}
	m.junits = append(m.junits, localJunits...)
	fmt.Printf("All monitor tests started.\n")

	return nil
}

func (m *Monitor) Stop(ctx context.Context) (ResultState, error) {
	fmt.Fprintf(os.Stderr, "Shutting down the monitor\n")

	m.lock.Lock()
	defer m.lock.Unlock()
	if m.stopFn == nil {
		return Failed, fmt.Errorf("monitor not started")
	}
	m.stopFn()
	m.stopFn = nil

	preStopTime := time.Now()

	fmt.Fprintf(os.Stderr, "Collecting data.\n")
	collectedIntervals, collectionJunits, err := m.monitorTestRegistry.CollectData(ctx, m.storageDir, m.startTime, preStopTime)
	if err != nil {
		// these errors are represented as junit, always continue to the next step
		fmt.Fprintf(os.Stderr, "Error collecting data, continuing, junit will reflect this. %v\n", err)
	}
	m.recorder.AddIntervals(collectedIntervals...)
	m.junits = append(m.junits, collectionJunits...)

	// set the stop time for after we finished.
	m.stopTime = time.Now()

	fmt.Fprintf(os.Stderr, "Computing intervals.\n")
	computedIntervals, computedJunit, err := m.monitorTestRegistry.ConstructComputedIntervals(
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
	finalEvents := m.recorder.Intervals(m.startTime, m.stopTime)
	filename := fmt.Sprintf("events_used_for_junits_%s.json", m.startTime.UTC().Format("20060102-150405"))
	if err := monitorserialization.EventsToFile(filepath.Join(m.storageDir, filename), finalEvents); err != nil {
		fmt.Fprintf(os.Stderr, "error: Failed to junit event info: %v\n", err)
	}
	monitorTestJunits, err := m.monitorTestRegistry.EvaluateTestsFromConstructedIntervals(
		ctx,
		finalEvents, // evaluate the tests on the intervals during our active time.
	)
	if err != nil {
		// these errors are represented as junit, always continue to the next step
		fmt.Fprintf(os.Stderr, "Error evaluating tests, continuing, junit will reflect this. %v\n", err)
	}
	m.junits = append(m.junits, monitorTestJunits...)

	fmt.Fprintf(os.Stderr, "Cleaning up.\n")
	cleanupJunits, err := m.monitorTestRegistry.Cleanup(ctx)
	if err != nil {
		// these errors are represented as junit, always continue to the next step
		fmt.Fprintf(os.Stderr, "Error cleaning up, continuing, junit will reflect this. %v\n", err)
	}
	m.junits = append(m.junits, cleanupJunits...)

	successfulTestNames := sets.NewString()
	failedTestNames := sets.NewString()
	resultState := Succeeded
	for _, junit := range m.junits {
		if junit.FailureOutput != nil {
			failedTestNames.Insert(junit.Name)
			continue
		}
		successfulTestNames.Insert(junit.Name)
	}
	onlyFailingTests := failedTestNames.Difference(successfulTestNames)
	if len(onlyFailingTests) > 0 {
		resultState = Failed
	}

	return resultState, nil
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
	monitorTestJunits, err := m.monitorTestRegistry.WriteContentToStorage(
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
	m.junits = append(m.junits, monitorTestJunits...)

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

	out, err := xml.MarshalIndent(junitSuite, "", "    ")
	if err != nil {
		return err
	}
	filePrefix := "e2e-monitor-tests"
	path := filepath.Join(storageDir, fmt.Sprintf("%s_%s.xml", filePrefix, fileSuffix))
	fmt.Fprintf(os.Stderr, "Writing JUnit report to %s\n", path)
	return os.WriteFile(path, test.StripANSI(out), 0640)
}

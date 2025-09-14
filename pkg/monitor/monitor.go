package monitor

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/riskanalysis"

	"github.com/openshift/origin/pkg/monitortestframework"

	"k8s.io/apimachinery/pkg/util/sets"

	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"

	"github.com/openshift/origin/pkg/test"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/client-go/rest"
)

const monitorAnnotation = "[Monitor:"
const defaultMonitorAnnotation = "[Monitor:Unknown]"

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

	localJunits, err := m.monitorTestRegistry.PrepareCollection(ctx, m.adminKubeConfig, m.recorder)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error preparing for data collection, continuing, junit will reflect this. %v\n", err)
	}
	m.junits = append(m.junits, localJunits...)

	localJunits, err = m.monitorTestRegistry.StartCollection(ctx, m.adminKubeConfig, m.recorder)
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
		m.startTime, // still allow computation to understand the beginning and end for bounding.
		m.stopTime)  // still allow computation to understand the beginning and end for bounding.
	if err != nil {
		// these errors are represented as junit, always continue to the next step
		fmt.Fprintf(os.Stderr, "Error computing intervals, continuing, junit will reflect this. %v\n", err)
	}
	m.recorder.AddIntervals(computedIntervals...)
	m.junits = append(m.junits, computedJunit...)

	fmt.Fprintf(os.Stderr, "Evaluating tests.\n")
	// compute intervals based on *all* the intervals.  Individual monitortests can choose how to restrict themselves.
	finalEvents := m.recorder.Intervals(time.Time{}, time.Time{})
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

	// We bound the intervals by the monitor stop/start time to limit the scope to when
	// the monitors run (e.g., during upgrade phase and during e2e phase post upgrade).
	// The tests will not be able to discover intervals outside of the current phase (e.g.,
	// tests that check intervals for the e2e phase will not see intervals during upgrade
	// phase and vice versa).  If it turns out visibility throughout the entire run yields
	// useful testing, we can comeback and tweak this accordingly.
	finalIntervals := m.recorder.Intervals(m.startTime, m.stopTime)

	finalResources := m.recorder.CurrentResourceState()
	// TODO stop taking timesuffix as an arg and make this authoritative.
	// timeSuffix := fmt.Sprintf("_%s", time.Now().UTC().Format("20060102-150405"))

	eventDir := filepath.Join(m.storageDir, monitorapi.EventDir)
	if err := os.MkdirAll(eventDir, os.ModePerm); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create monitor-events directory, err: %v\n", err)
		return err
	}

	fmt.Fprintf(os.Stderr, "Writing to storage.\n")
	fmt.Fprintf(os.Stderr, "  m.startTime = %s\n", m.startTime)
	fmt.Fprintf(os.Stderr, "  m.stopTime  = %s\n", m.stopTime)

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
	var junitSuite *junitapi.JUnitTestSuite
	if junitSuite, err = m.serializeJunit(ctx, m.storageDir, junitSuiteName, timeSuffix); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write junit xml, err: %v\n", err)
		return err
	}

	if err := riskanalysis.WriteJobRunTestFailureSummary(m.storageDir, timeSuffix, junitSuite, "", "_monitor"); err != nil {
		fmt.Fprintf(os.Stderr, "error: Unable to write e2e job run failures summary: %v", err)
	}

	return nil
}

func (m *Monitor) serializeJunit(ctx context.Context, storageDir, junitSuiteName, fileSuffix string) (*junitapi.JUnitTestSuite, error) {
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

		if !strings.Contains(currJunit.Name, monitorAnnotation) {
			// if we don't have the annotation add in the default
			currJunit.Name = fmt.Sprintf("%s%s", defaultMonitorAnnotation, currJunit.Name)
		}

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
		return nil, err
	}
	filePrefix := "e2e-monitor-tests"
	path := filepath.Join(storageDir, fmt.Sprintf("%s_%s.xml", filePrefix, fileSuffix))
	fmt.Fprintf(os.Stderr, "Writing JUnit report to %s\n", path)
	return &junitSuite, os.WriteFile(path, test.StripANSI(out), 0640)
}

package monitor

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"

	"github.com/openshift/origin/pkg/test"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"github.com/openshift/origin/pkg/invariants"

	configclientset "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitor/shutdown"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Monitor struct {
	adminKubeConfig   *rest.Config
	invariantRegistry invariants.InvariantRegistry
	storageDir        string

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
	invariantRegistry invariants.InvariantRegistry) Interface {
	return &Monitor{
		adminKubeConfig:   adminKubeConfig,
		recorder:          recorder,
		invariantRegistry: invariantRegistry,
		storageDir:        storageDir,
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
	fmt.Printf("All invariants started.\n")

	client, err := kubernetes.NewForConfig(m.adminKubeConfig)
	if err != nil {
		return err
	}
	configClient, err := configclientset.NewForConfig(m.adminKubeConfig)
	if err != nil {
		return err
	}

	startNodeMonitoring(ctx, m.recorder, client)
	startEventMonitoring(ctx, m.recorder, client)
	shutdown.StartMonitoringGracefulShutdownEvents(ctx, m.recorder, client)

	// add interval creation at the same point where we add the monitors
	startClusterOperatorMonitoring(ctx, m.recorder, configClient)
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

	// we don't want this method to return until all te additional recorders and invariants have completed processing.
	// to do this correctly, we need closure channels or some kind of mechanism.
	// rather than properly wire this through, we'll wait until the backend disruption consumers timeout after the producer
	// close.
	// TODO once we have converted the backendsamplers to invariant tests, we can properly wait for completion
	time.Sleep(7 * time.Second)

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
	finalEvents := m.recorder.Intervals(m.startTime, m.stopTime)
	filename := fmt.Sprintf("events_used_for_junits_%s.json", m.startTime.UTC().Format("20060102-150405"))
	if err := monitorserialization.EventsToFile(filepath.Join(m.storageDir, filename), finalEvents); err != nil {
		fmt.Fprintf(os.Stderr, "error: Failed to junit event info: %v\n", err)
	}
	invariantJunits, err := m.invariantRegistry.EvaluateTestsFromConstructedIntervals(
		ctx,
		finalEvents, // evaluate the tests on the intervals during our active time.
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

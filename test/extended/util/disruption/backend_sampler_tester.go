package disruption

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/synthetictests/allowedbackenddisruption"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

type BackendSampler interface {
	GetConnectionType() backenddisruption.BackendConnectionType
	GetDisruptionBackendName() string
	GetLocator() string
	GetURL() (string, error)
	RunEndpointMonitoring(ctx context.Context, m backenddisruption.Recorder, eventRecorder events.EventRecorder) error
	Stop()
}

type BackendDisruptionUpgradeTest interface {
	upgrades.Test
	DisplayName() string
}

func NewBackendDisruptionTest(testName string, backend BackendSampler) *backendDisruptionTest {
	ret := &backendDisruptionTest{
		testName: testName,
		backend:  backend,
	}
	ret.getAllowedDisruption = ret.historicalP95Disruption
	return ret
}

func (t *backendDisruptionTest) WithAllowedDisruption(allowedDisruptionFn AllowedDisruptionFunc) *backendDisruptionTest {
	t.getAllowedDisruption = allowedDisruptionFn
	return t
}

type SetupFunc func(f *framework.Framework, backendSampler BackendSampler) error

func (t *backendDisruptionTest) WithPreSetup(preSetup SetupFunc) *backendDisruptionTest {
	t.preSetup = preSetup
	return t
}

type TearDownFunc func(f *framework.Framework) error

func (t *backendDisruptionTest) WithPostTeardown(postTearDown TearDownFunc) *backendDisruptionTest {
	t.postTearDown = postTearDown
	return t
}

func (t *backendDisruptionTest) historicalP95Disruption(f *framework.Framework, totalDuration time.Duration) (*time.Duration, string, error) {
	backendName := t.backend.GetDisruptionBackendName() + "-" + string(t.backend.GetConnectionType()) + "-connections"
	return allowedbackenddisruption.GetAllowedDisruption(context.Background(), backendName, f.ClientConfig())
}

// returns allowedDuration, detailsString(for display), error
type AllowedDisruptionFunc func(f *framework.Framework, totalDuration time.Duration) (*time.Duration, string, error)

// availableTest tests that route frontends are available before, during, and
// after a cluster upgrade.
type backendDisruptionTest struct {
	// testName is the name to show in unit.
	testName string
	// backend describes a route that should be monitored.
	backend              BackendSampler
	getAllowedDisruption AllowedDisruptionFunc

	preSetup     SetupFunc
	postTearDown TearDownFunc
}

func (t *backendDisruptionTest) Name() string {
	return fmt.Sprintf("%v-%v", t.backend.GetDisruptionBackendName(), t.backend.GetConnectionType())
}
func (t *backendDisruptionTest) DisplayName() string {
	return t.testName
}

// Setup looks up the host of the route specified by the backendSampler and updates
// the backendSampler with the route's host.
func (t *backendDisruptionTest) Setup(f *framework.Framework) {
	if t.preSetup != nil {
		framework.ExpectNoError(t.preSetup(f, t.backend))
	}
}

// Test runs a connectivity check to a route.
func (t *backendDisruptionTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	stopCh := make(chan struct{})
	defer close(stopCh)

	newBroadcaster := events.NewBroadcaster(&events.EventSinkImpl{Interface: f.ClientSet.EventsV1()})
	eventRecorder := newBroadcaster.NewRecorder(scheme.Scheme, "openshift.io/"+t.backend.GetDisruptionBackendName())
	newBroadcaster.StartRecordingToSink(stopCh)

	start := time.Now()
	ginkgo.By(fmt.Sprintf("continuously hitting backend: %s", t.backend.GetLocator()))

	endpointMonitoringContext, endpointMonitoringCancel := context.WithCancel(context.Background())
	defer endpointMonitoringCancel() // final backstop on closure
	m := monitor.NewMonitorWithInterval(1 * time.Second)
	disruptionErrCh := make(chan error, 1)
	go func() {
		err := t.backend.RunEndpointMonitoring(endpointMonitoringContext, m, eventRecorder)
		disruptionErrCh <- err
	}()
	time.Sleep(1 * time.Second) // wait for some initial errors so we can fail early if it happens
	var disruptionErr error
	select {
	case disruptionErr = <-disruptionErrCh:
	default:
	}
	framework.ExpectNoError(disruptionErr, fmt.Sprintf("unable to monitor: %s", t.backend.GetLocator()))

	// Wait to ensure the backend is still available after the test ends.
	<-done
	ginkgo.By(fmt.Sprintf("waiting for any post disruption failures: %s", t.backend.GetLocator()))
	time.Sleep(30 * time.Second)
	t.backend.Stop() // stop the monitor from above

	// wait for completion of the monitor
	select {
	case disruptionErr = <-disruptionErrCh: // we should get an answer either way when the RunEndpointMonitoring from above finishes
	case <-time.After(1 * time.Minute):
		disruptionErr = fmt.Errorf("timed out waiting for the monitoring thread to end")
	}
	if disruptionErr != nil {
		framework.Logf(fmt.Sprintf("unable to finish: %s", t.backend.GetLocator()))
	}

	end := time.Now()

	allowedDisruption, disruptionDetails, err := t.getAllowedDisruption(f, end.Sub(start))
	framework.ExpectNoError(err)

	ginkgo.By(fmt.Sprintf("writing results: %s", t.backend.GetLocator()))
	ExpectNoDisruptionForDuration(
		f,
		*allowedDisruption,
		end.Sub(start),
		m.Intervals(time.Time{}, time.Time{}),
		fmt.Sprintf("%s was unreachable during disruption: %v", t.backend.GetLocator(), disruptionDetails),
	)

	ginkgo.By(fmt.Sprintf("results tallied: %s", t.backend.GetLocator()))

	// raise an error AFTER we add the test summary
	// TOOD restore.  suppressing this now to see what data we can get out without a panic.
	framework.ExpectNoError(disruptionErr, fmt.Sprintf("unable to finish: %s", t.backend.GetLocator()))
}

// Teardown cleans up any remaining resources.
func (t *backendDisruptionTest) Teardown(f *framework.Framework) {
	if t.postTearDown != nil {
		framework.ExpectNoError(t.postTearDown(f))
	}
}

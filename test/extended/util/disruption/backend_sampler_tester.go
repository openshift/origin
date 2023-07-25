package disruption

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/synthetictests/allowedbackenddisruption"
	"github.com/openshift/origin/pkg/synthetictests/platformidentification"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

type BackendSampler interface {
	GetConnectionType() monitorapi.BackendConnectionType
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
	ret.getAllowedDisruption = alwaysAllowOneSecond(ret.historicalAllowedDisruption)
	return ret
}

// NewBackendDisruptionTestWithFixedAllowedDisruption creates a new test with a fixed amount of disruption,
// rather than historical data.
// This is only useful in very rare situations and you most likely want the standard NewBackendDisruptionTest.
func NewBackendDisruptionTestWithFixedAllowedDisruption(testName string, backend BackendSampler,
	allowedDisruption *time.Duration, disruptionDescription string) *backendDisruptionTest {
	ret := &backendDisruptionTest{
		testName: testName,
		backend:  backend,
	}
	ret.getAllowedDisruption = func(f *framework.Framework) (*time.Duration, string, error) {
		return allowedDisruption, disruptionDescription, nil
	}
	return ret
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

func alwaysAllowOneSecond(delegateFn AllowedDisruptionFunc) AllowedDisruptionFunc {
	return func(f *framework.Framework) (*time.Duration, string, error) {
		delegateDuration, delegateReason, delegateError := delegateFn(f)
		if delegateError != nil {
			return delegateDuration, delegateReason, delegateError
		}
		if delegateDuration == nil {
			return delegateDuration, delegateReason, delegateError
		}

		oneSecond := 1 * time.Second
		if *delegateDuration < oneSecond {
			return &oneSecond, "always allow at least 1s", nil
		}

		return delegateDuration, delegateReason, delegateError
	}
}

func (t *backendDisruptionTest) historicalAllowedDisruption(f *framework.Framework) (*time.Duration, string, error) {
	backendName := t.backend.GetDisruptionBackendName() + "-" + string(t.backend.GetConnectionType()) + "-connections"
	jobType, err := platformidentification.GetJobType(context.TODO(), f.ClientConfig())
	if err != nil {
		return nil, "", err
	}
	framework.Logf("checking allowed disruption for job type: %+v", *jobType)

	return allowedbackenddisruption.GetAllowedDisruption(backendName, *jobType)
}

// returns allowedDuration, detailsString(for display), error
type AllowedDisruptionFunc func(f *framework.Framework) (*time.Duration, string, error)

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
func (t *backendDisruptionTest) Setup(ctx context.Context, f *framework.Framework) {
	if t.preSetup != nil {
		framework.ExpectNoError(t.preSetup(f, t.backend))
	}
}

// Test runs a connectivity check to a route.
func (t *backendDisruptionTest) Test(ctx context.Context, f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	stopCh := make(chan struct{})
	defer close(stopCh)

	newBroadcaster := events.NewBroadcaster(&events.EventSinkImpl{Interface: f.ClientSet.EventsV1()})
	eventRecorder := newBroadcaster.NewRecorder(scheme.Scheme, "openshift.io/"+t.backend.GetDisruptionBackendName())
	newBroadcaster.StartRecordingToSink(stopCh)

	start := time.Now()
	ginkgo.By(fmt.Sprintf("continuously hitting backend: %s", t.backend.GetLocator()))

	endpointMonitoringContext, endpointMonitoringCancel := context.WithCancel(ctx)
	defer endpointMonitoringCancel() // final backstop on closure
	m := monitor.NewMonitor(f.ClientConfig(), nil)
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

	allowedDisruption, disruptionDetails, err := t.getAllowedDisruption(f)
	framework.ExpectNoError(err)

	end := time.Now()

	fromTime, endTime := time.Time{}, time.Time{}
	events := m.Intervals(fromTime, endTime)
	ginkgo.By(fmt.Sprintf("writing results: %s", t.backend.GetLocator()))
	ExpectNoDisruptionForDuration(
		f,
		t.testName,
		allowedDisruption,
		end.Sub(start),
		events,
		fmt.Sprintf("%s was unreachable during disruption: %v", t.backend.GetLocator(), disruptionDetails),
	)

	ginkgo.By(fmt.Sprintf("results tallied: %s", t.backend.GetLocator()))

	// raise an error AFTER we add the test summary
	// TODO restore.  suppressing this now to see what data we can get out without a panic.
	framework.ExpectNoError(disruptionErr, fmt.Sprintf("unable to finish: %s", t.backend.GetLocator()))
}

// Teardown cleans up any remaining resources.
func (t *backendDisruptionTest) Teardown(ctx context.Context, f *framework.Framework) {
	if t.postTearDown != nil {
		framework.ExpectNoError(t.postTearDown(f))
	}
}

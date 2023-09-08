package disruptionlibrary

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/monitortestlibrary/allowedbackenddisruption"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"

	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/rest"
)

type Availability struct {
	newConnectionTestName    string
	reusedConnectionTestName string
	jobType                  *platformidentification.JobType

	newConnectionDisruptionSampler    *backenddisruption.BackendSampler
	reusedConnectionDisruptionSampler *backenddisruption.BackendSampler
}

func NewAvailabilityInvariant(
	newConnectionTestName, reusedConnectionTestName string,
	newConnectionDisruptionSampler, reusedConnectionDisruptionSampler *backenddisruption.BackendSampler) *Availability {
	return &Availability{
		newConnectionTestName:             newConnectionTestName,
		reusedConnectionTestName:          reusedConnectionTestName,
		newConnectionDisruptionSampler:    newConnectionDisruptionSampler,
		reusedConnectionDisruptionSampler: reusedConnectionDisruptionSampler,
	}
}

func (w *Availability) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	if w == nil {
		return fmt.Errorf("unable to start collection because instance is nil")
	}

	var err error
	w.jobType, err = platformidentification.GetJobType(ctx, adminRESTConfig)
	if err != nil {
		return err
	}

	if err := w.newConnectionDisruptionSampler.StartEndpointMonitoring(ctx, recorder, nil); err != nil {
		return err
	}
	if err := w.reusedConnectionDisruptionSampler.StartEndpointMonitoring(ctx, recorder, nil); err != nil {
		return err
	}

	return nil
}

func (w *Availability) CollectData(ctx context.Context) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if w == nil {
		return nil, nil, fmt.Errorf("unable to collected data because instance is nil")
	}

	// when it is time to collect data, we need to stop the collectors.  they both  have to drain, so stop in parallel
	wg := sync.WaitGroup{}

	var newRecoverErr error
	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				newRecoverErr = fmt.Errorf("panic in stop: %v", r)
			}
		}()

		defer wg.Done()
		w.newConnectionDisruptionSampler.Stop()
	}()

	var reusedRecoverErr error
	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				reusedRecoverErr = fmt.Errorf("panic in stop: %v", r)
			}
		}()

		defer wg.Done()
		w.reusedConnectionDisruptionSampler.Stop()
	}()

	wg.Wait()

	return nil, nil, utilerrors.NewAggregate([]error{newRecoverErr, reusedRecoverErr})
}

func createDisruptionJunit(testName string, allowedDisruption *time.Duration, disruptionDetails string, locator monitorapi.Locator, disruptedIntervals monitorapi.Intervals) *junitapi.JUnitTestCase {
	// Indicates there is no entry in the query_results.json data file, nor a valid fallback,
	// we do not wish to run the test. (this likely implies we do not have the required number of
	// runs in 3 weeks to do a reliable P99)
	if allowedDisruption == nil {
		return &junitapi.JUnitTestCase{
			Name: testName,
			SkipMessage: &junitapi.SkipMessage{
				Message: "No historical data to calculate allowedDisruption",
			},
		}
	}

	if *allowedDisruption < 1*time.Second {
		t := 1 * time.Second
		allowedDisruption = &t
		disruptionDetails = "always allow at least one second"
	}

	disruptionDuration := disruptedIntervals.Duration(1 * time.Second)
	roundedAllowedDisruption := allowedDisruption.Round(time.Second)
	roundedDisruptionDuration := disruptionDuration.Round(time.Second)

	if roundedDisruptionDuration <= roundedAllowedDisruption {
		return &junitapi.JUnitTestCase{
			Name: testName,
		}
	}

	reason := fmt.Sprintf("%v was unreachable during disruption: %v", locator.OldLocator(), disruptionDetails)
	describe := disruptedIntervals.Strings()
	failureMessage := fmt.Sprintf("%s for at least %s (maxAllowed=%s):\n\n%s", reason, roundedDisruptionDuration, roundedAllowedDisruption, strings.Join(describe, "\n"))

	return &junitapi.JUnitTestCase{
		Name: testName,
		FailureOutput: &junitapi.FailureOutput{
			Output: failureMessage,
		},
		SystemOut: failureMessage,
	}
}

func (w *Availability) junitForNewConnections(ctx context.Context, finalIntervals monitorapi.Intervals) (*junitapi.JUnitTestCase, error) {
	newConnectionAllowed, newConnectionDisruptionDetails, err := historicalAllowedDisruption(ctx, w.newConnectionDisruptionSampler, w.jobType)
	if err != nil {
		return nil, fmt.Errorf("unable to get new allowed disruption: %w", err)
	}
	return createDisruptionJunit(
			w.newConnectionTestName, newConnectionAllowed, newConnectionDisruptionDetails, w.newConnectionDisruptionSampler.GetLocator(),
			finalIntervals.Filter(
				monitorapi.And(
					monitorapi.IsEventForLocator(w.newConnectionDisruptionSampler.GetLocator().OldLocator()),
					monitorapi.IsErrorEvent,
				),
			),
		),
		nil
}

func (w *Availability) junitForReusedConnections(ctx context.Context, finalIntervals monitorapi.Intervals) (*junitapi.JUnitTestCase, error) {
	reusedConnectionAllowed, reusedConnectionDisruptionDetails, err := historicalAllowedDisruption(ctx, w.reusedConnectionDisruptionSampler, w.jobType)
	if err != nil {
		return nil, fmt.Errorf("unable to get reused allowed disruption: %w", err)
	}
	return createDisruptionJunit(
			w.reusedConnectionTestName, reusedConnectionAllowed, reusedConnectionDisruptionDetails, w.reusedConnectionDisruptionSampler.GetLocator(),
			finalIntervals.Filter(
				monitorapi.And(
					monitorapi.IsEventForLocator(w.reusedConnectionDisruptionSampler.GetLocator().OldLocator()),
					monitorapi.IsErrorEvent,
				),
			),
		),
		nil
}

func historicalAllowedDisruption(ctx context.Context, backend *backenddisruption.BackendSampler, jobType *platformidentification.JobType) (*time.Duration, string, error) {
	return allowedbackenddisruption.GetAllowedDisruption(backend.GetDisruptionBackendName(), *jobType)
}

func (w *Availability) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	if w == nil {
		return nil, fmt.Errorf("unable to evaluate tests because instance is nil")
	}

	newConnectionJunit, err := w.junitForNewConnections(ctx, finalIntervals)
	if err != nil {
		return nil, err
	}

	reusedConnectionJunit, err := w.junitForReusedConnections(ctx, finalIntervals)
	if err != nil {
		return nil, err
	}

	return []*junitapi.JUnitTestCase{newConnectionJunit, reusedConnectionJunit}, nil
}

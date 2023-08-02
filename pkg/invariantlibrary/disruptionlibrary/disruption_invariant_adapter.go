package disruptionlibrary

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/synthetictests/platformidentification"

	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/openshift/origin/test/extended/util/disruption"
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
	// when it is time to collect data, we need to stop the collectors.  they both  have to drain, so stop in parallel
	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		w.newConnectionDisruptionSampler.Stop()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		w.reusedConnectionDisruptionSampler.Stop()
	}()

	wg.Wait()

	return nil, nil, nil
}

func createDisruptionJunit(testName string, allowedDisruption *time.Duration, disruptionDetails, locator string, disruptedIntervals monitorapi.Intervals) *junitapi.JUnitTestCase {
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

	reason := fmt.Sprintf("%s was unreachable during disruption: %v", locator, disruptionDetails)
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
	newConnectionAllowed, newConnectionDisruptionDetails, err := disruption.HistoricalAllowedDisruption(ctx, w.newConnectionDisruptionSampler, w.jobType)
	if err != nil {
		return nil, fmt.Errorf("unable to get new allowed disruption: %w", err)
	}
	return createDisruptionJunit(
			w.newConnectionTestName, newConnectionAllowed, newConnectionDisruptionDetails, w.newConnectionDisruptionSampler.GetLocator(),
			finalIntervals.Filter(
				monitorapi.And(
					monitorapi.IsEventForLocator(w.newConnectionDisruptionSampler.GetLocator()),
					monitorapi.IsErrorEvent,
				),
			),
		),
		nil
}

func (w *Availability) junitForReusedConnections(ctx context.Context, finalIntervals monitorapi.Intervals) (*junitapi.JUnitTestCase, error) {
	reusedConnectionAllowed, reusedConnectionDisruptionDetails, err := disruption.HistoricalAllowedDisruption(ctx, w.reusedConnectionDisruptionSampler, w.jobType)
	if err != nil {
		return nil, fmt.Errorf("unable to get reused allowed disruption: %w", err)
	}
	return createDisruptionJunit(
			w.reusedConnectionTestName, reusedConnectionAllowed, reusedConnectionDisruptionDetails, w.reusedConnectionDisruptionSampler.GetLocator(),
			finalIntervals.Filter(
				monitorapi.And(
					monitorapi.IsEventForLocator(w.reusedConnectionDisruptionSampler.GetLocator()),
					monitorapi.IsErrorEvent,
				),
			),
		),
		nil
}

func (w *Availability) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {

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

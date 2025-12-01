package disruptionlibrary

import (
	"context"
	_ "embed"
	"fmt"
	"math"
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

	// store the rest config so we can
	// get the JobType at the end of the run
	// which will include any upgrade versions
	adminRESTConfig *rest.Config

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

	w.adminRESTConfig = adminRESTConfig

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

func createDisruptionJunit(
	testName string,
	allowedDisruption *time.Duration,
	disruptionDetails string,
	locator monitorapi.Locator,
	disruptedIntervals monitorapi.Intervals,
	jobType *platformidentification.JobType) *junitapi.JUnitTestCase {

	// Not sure what these are, but this will help find them, and we don't get any value from testing these:
	if jobType.Platform == "" {
		return &junitapi.JUnitTestCase{
			Name: testName,
			SkipMessage: &junitapi.SkipMessage{
				Message: "Unknown platform, skipping disruption testing",
			},
		}
	}

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

	disruptionDuration := disruptedIntervals.Duration(1 * time.Second)
	roundedDisruptionDuration := disruptionDuration.Round(time.Second)

	// Determine what amount of disruption we're willing to tolerate before we fail the test. We previously just
	// enforced being over a P99 over the past 3 weeks, however the P99 fluctuates wildly even under these
	// conditions, and the tests fail excessively on very low numbers. Thus we now also allow a grace amount to try to
	// establish this as a first line of defence to detect egregious regressions before they merge.
	//roundedAllowedDisruption, additionalDetails := calculateAllowedDisruptionWithGrace(*allowedDisruption)
	allowedDetails := []string{}
	allowedDetails = append(allowedDetails, fmt.Sprintf("P99 from historical data for similar jobs over past 3 weeks: %s",
		*allowedDisruption))
	if *allowedDisruption < 1*time.Second {
		t := 1 * time.Second
		allowedDisruption = &t
		allowedDetails = append(allowedDetails, "rounded P99 up to always allow one second")
	}

	// Allow grace of 5s or 20%, at this layer, with one sample, we're only hoping to find really severe disruption:
	// Single node clusters are frequently under heavier load and we need to be more forgiving. Allow grace of 10s or 40%
	allowedSecs := allowedDisruption.Seconds()
	graceSeconds, gracePercent := 5.0, 1.2
	secondDetails, percentDetails := "added an additional 5s of grace", "added an additional 20% of grace"

	if jobType.Topology == "single" {
		graceSeconds, gracePercent = 10.0, 1.4
		secondDetails, percentDetails = "added an additional 10s of grace for single node cluster", "added an additional 40% of grace for single node cluster"
	}

	allowedSecsPlusGrace := allowedSecs + graceSeconds
	allowedSecsPlusPercent := allowedSecs * gracePercent

	var allowedSecsWithGrace float64
	if allowedSecsPlusPercent > allowedSecsPlusGrace {
		allowedSecsWithGrace = allowedSecsPlusPercent
		allowedDetails = append(allowedDetails, percentDetails)
	} else {
		allowedSecsWithGrace = allowedSecsPlusGrace
		allowedDetails = append(allowedDetails, secondDetails)
	}
	roundedFinal := int64(math.Round(allowedSecsWithGrace))
	finalAllowedDisruption := time.Duration(roundedFinal) * time.Second

	if roundedDisruptionDuration <= finalAllowedDisruption {
		return &junitapi.JUnitTestCase{
			Name: testName,
		}
	}

	reason := fmt.Sprintf("%v was unreachable during disruption: %v", locator.OldLocator(), disruptionDetails)
	describe := disruptedIntervals.Strings()
	failureMessage := fmt.Sprintf("%s for at least %s (maxAllowed=%s):\n%s\n\n%s", reason,
		roundedDisruptionDuration, finalAllowedDisruption,
		strings.Join(allowedDetails, "\n"),
		strings.Join(describe, "\n"))

	return &junitapi.JUnitTestCase{
		Name: testName,
		FailureOutput: &junitapi.FailureOutput{
			Output: failureMessage,
		},
		SystemOut: failureMessage,
	}
}

func (w *Availability) junitForNewConnections(ctx context.Context, finalIntervals monitorapi.Intervals, jobType *platformidentification.JobType) (*junitapi.JUnitTestCase, error) {
	newConnectionAllowed, newConnectionDisruptionDetails, err := historicalAllowedDisruption(ctx, w.newConnectionDisruptionSampler, jobType)
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
			jobType,
		),
		nil
}

func (w *Availability) junitForReusedConnections(ctx context.Context, finalIntervals monitorapi.Intervals, jobType *platformidentification.JobType) (*junitapi.JUnitTestCase, error) {
	reusedConnectionAllowed, reusedConnectionDisruptionDetails, err := historicalAllowedDisruption(ctx, w.reusedConnectionDisruptionSampler, jobType)
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
			jobType,
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

	var err error
	jobType, err := platformidentification.GetJobType(ctx, w.adminRESTConfig)
	if err != nil {
		return nil, err
	}

	newConnectionJunit, err := w.junitForNewConnections(ctx, finalIntervals, jobType)
	if err != nil {
		return nil, err
	}

	reusedConnectionJunit, err := w.junitForReusedConnections(ctx, finalIntervals, jobType)
	if err != nil {
		return nil, err
	}

	return []*junitapi.JUnitTestCase{newConnectionJunit, reusedConnectionJunit}, nil
}

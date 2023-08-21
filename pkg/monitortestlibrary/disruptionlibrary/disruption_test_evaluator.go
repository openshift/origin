package disruptionlibrary

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortestlibrary/allowedbackenddisruption"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

type AvailabilityTestEvaluator struct {
	newConnectionTestName       string
	reusedConnectionTestName    string
	jobType                     platformidentification.JobType
	alwaysAtLeastFlakeOnNonZero bool

	newConnectionHistoricalBackendName    string
	reusedConnectionHistoricalBackendName string
}

func NewAvailabilityTestEvaluatorInvariant(
	newConnectionTestName, reusedConnectionTestName string,
	newConnectionHistoricalBackendName, reusedConnectionHistoricalBackendName string,
	jobType platformidentification.JobType) *AvailabilityTestEvaluator {
	return &AvailabilityTestEvaluator{
		newConnectionTestName:                 newConnectionTestName,
		reusedConnectionTestName:              reusedConnectionTestName,
		newConnectionHistoricalBackendName:    newConnectionHistoricalBackendName,
		reusedConnectionHistoricalBackendName: reusedConnectionHistoricalBackendName,
		jobType:                               jobType,
	}
}

func (w *AvailabilityTestEvaluator) AlwaysAtLeastFlakeOnNonZero() *AvailabilityTestEvaluator {
	w.alwaysAtLeastFlakeOnNonZero = true
	return w
}

func (w *AvailabilityTestEvaluator) createDisruptionJunit(testName string, allowedDisruption *time.Duration, disruptionDetails string, backendName string, finalIntervals monitorapi.Intervals) []*junitapi.JUnitTestCase {
	// Indicates there is no entry in the query_results.json data file, nor a valid fallback,
	// we do not wish to run the test. (this likely implies we do not have the required number of
	// runs in 3 weeks to do a reliable P99)
	if allowedDisruption == nil {
		return []*junitapi.JUnitTestCase{
			{
				Name: testName,
				SkipMessage: &junitapi.SkipMessage{
					Message: "No historical data to calculate allowedDisruption",
				},
			},
		}
	}

	if *allowedDisruption < 1*time.Second {
		t := 1 * time.Second
		allowedDisruption = &t
		disruptionDetails = "always allow at least one second"
	}

	roundedAllowedDisruption := allowedDisruption.Round(time.Second)
	roundedDisruptionDuration, describe := monitorapi.BackendDisruptionSeconds(backendName, finalIntervals)

	failureReason := fmt.Sprintf("%v was unreachable during disruption: %v", backendName, disruptionDetails)
	failureMessage := fmt.Sprintf("%s for at least %s (maxAllowed=%s):\n\n%s", failureReason, roundedDisruptionDuration, roundedAllowedDisruption, strings.Join(describe, "\n"))
	ret := []*junitapi.JUnitTestCase{}

	if w.alwaysAtLeastFlakeOnNonZero && roundedDisruptionDuration > 0 {
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: failureMessage,
				},
				SystemOut: failureMessage,
			})
	}

	if roundedDisruptionDuration <= roundedAllowedDisruption {
		return append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
			})
	}

	return []*junitapi.JUnitTestCase{
		{
			Name: testName,
			FailureOutput: &junitapi.FailureOutput{
				Output: failureMessage,
			},
			SystemOut: failureMessage,
		},
	}
}

func (w *AvailabilityTestEvaluator) junitForNewConnections(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	newConnectionAllowed, newConnectionDisruptionDetails, err := historicalAllowedDisruption(ctx, w.newConnectionHistoricalBackendName, w.jobType)
	if err != nil {
		return nil, fmt.Errorf("unable to get new allowed disruption: %w", err)
	}

	return w.createDisruptionJunit(
			w.newConnectionTestName,
			newConnectionAllowed,
			newConnectionDisruptionDetails,
			w.newConnectionHistoricalBackendName,
			finalIntervals,
		),
		nil
}

func (w *AvailabilityTestEvaluator) junitForReusedConnections(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	reusedConnectionAllowed, reusedConnectionDisruptionDetails, err := historicalAllowedDisruption(ctx, w.reusedConnectionHistoricalBackendName, w.jobType)
	if err != nil {
		return nil, fmt.Errorf("unable to get reused allowed disruption: %w", err)
	}
	return w.createDisruptionJunit(
			w.reusedConnectionTestName,
			reusedConnectionAllowed,
			reusedConnectionDisruptionDetails,
			w.reusedConnectionHistoricalBackendName,
			finalIntervals,
		),
		nil
}

func historicalAllowedDisruption(ctx context.Context, backendName string, jobType platformidentification.JobType) (*time.Duration, string, error) {
	return allowedbackenddisruption.GetAllowedDisruption(backendName, jobType)
}

func (w *AvailabilityTestEvaluator) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
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

	ret := []*junitapi.JUnitTestCase{}
	ret = append(ret, newConnectionJunit...)
	ret = append(ret, reusedConnectionJunit...)
	return ret, nil
}

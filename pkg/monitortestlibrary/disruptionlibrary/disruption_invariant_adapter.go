package disruptionlibrary

import (
	"context"
	_ "embed"
	"fmt"
	"sync"

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

func (w *Availability) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	if w == nil {
		return nil, fmt.Errorf("unable to evaluate tests because instance is nil")
	}
	if w.jobType == nil {
		return nil, fmt.Errorf("unable to evaluate tests because job type is missing")
	}

	newBackendName := w.newConnectionDisruptionSampler.GetDisruptionBackendName() + "-new-connections"
	reusedBackendName := w.reusedConnectionDisruptionSampler.GetDisruptionBackendName() + "-reused-connections"
	evaluator := NewAvailabilityTestEvaluatorInvariant(w.newConnectionTestName, w.reusedConnectionTestName, newBackendName, reusedBackendName, *w.jobType)
	return evaluator.EvaluateTestsFromConstructedIntervals(ctx, finalIntervals)
}

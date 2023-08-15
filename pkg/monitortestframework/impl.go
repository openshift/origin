package monitortestframework

import (
	"context"
	"fmt"
	"sync"
	"time"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

type monitorTestRegistry struct {
	monitorTests map[string]*monitorTesttItem
}

type monitorTesttItem struct {
	name          string
	jiraComponent string

	monitorTest MonitorTest
}

func NewMonitorTestRegistry() MonitorTestRegistry {
	return &monitorTestRegistry{
		monitorTests: map[string]*monitorTesttItem{},
	}
}

func (r *monitorTestRegistry) AddMonitorTest(name, jiraComponent string, monitorTest MonitorTest) error {
	if _, ok := r.monitorTests[name]; ok {
		return fmt.Errorf("%q is already registered", name)
	}
	r.monitorTests[name] = &monitorTesttItem{
		name:          name,
		jiraComponent: jiraComponent,
		monitorTest:   monitorTest,
	}

	return nil
}

func (r *monitorTestRegistry) AddMonitorTestOrDie(name, jiraComponent string, monitorTest MonitorTest) {
	err := r.AddMonitorTest(name, jiraComponent, monitorTest)
	if err != nil {
		panic(err)
	}
}

func (r *monitorTestRegistry) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) ([]*junitapi.JUnitTestCase, error) {
	wg := sync.WaitGroup{}
	junitCh := make(chan *junitapi.JUnitTestCase, len(r.monitorTests))
	errCh := make(chan error, len(r.monitorTests))

	for i := range r.monitorTests {
		wg.Add(1)
		go func(ctx context.Context, invariant *monitorTesttItem) {
			defer wg.Done()

			testName := fmt.Sprintf("[Jira:%q] monitor test %v setup", invariant.jiraComponent, invariant.name)
			fmt.Printf("  Starting %v for %v\n", invariant.name, invariant.jiraComponent)

			start := time.Now()
			err := startCollectionWithPanicProtection(ctx, invariant.monitorTest, adminRESTConfig, recorder)
			end := time.Now()
			duration := end.Sub(start)
			if err != nil {
				errCh <- err
				junitCh <- &junitapi.JUnitTestCase{
					Name:     testName,
					Duration: duration.Seconds(),
					FailureOutput: &junitapi.FailureOutput{
						Output: fmt.Sprintf("failed during setup\n%v", err),
					},
					SystemOut: fmt.Sprintf("failed during setup\n%v", err),
				}
				return
			}

			junitCh <- &junitapi.JUnitTestCase{
				Name:     testName,
				Duration: duration.Seconds(),
			}
		}(ctx, r.monitorTests[i])

	}

	wg.Wait()
	close(junitCh)
	close(errCh)

	junits := []*junitapi.JUnitTestCase{}
	errs := []error{}
	for curr := range junitCh {
		junits = append(junits, curr)
	}
	for curr := range errCh {
		errs = append(errs, curr)
	}

	return junits, utilerrors.NewAggregate(errs)
}

func (r *monitorTestRegistry) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	wg := sync.WaitGroup{}
	intervalsCh := make(chan monitorapi.Intervals, len(r.monitorTests))
	junitCh := make(chan []*junitapi.JUnitTestCase, 2*len(r.monitorTests))
	errCh := make(chan error, len(r.monitorTests))

	for i := range r.monitorTests {
		wg.Add(1)
		go func(ctx context.Context, monitorTest *monitorTesttItem) {
			defer wg.Done()
			testName := fmt.Sprintf("[Jira:%q] monitor test %v collection", monitorTest.jiraComponent, monitorTest.name)

			start := time.Now()
			localIntervals, localJunits, err := collectDataWithPanicProtection(ctx, monitorTest.monitorTest, storageDir, beginning, end)
			intervalsCh <- localIntervals
			junitCh <- localJunits
			end := time.Now()
			duration := end.Sub(start)
			if err != nil {
				junitCh <- []*junitapi.JUnitTestCase{
					{
						Name:     testName,
						Duration: duration.Seconds(),
						FailureOutput: &junitapi.FailureOutput{
							Output: fmt.Sprintf("failed during collection\n%v", err),
						},
						SystemOut: fmt.Sprintf("failed during collection\n%v", err),
					},
				}
				return
			}

			junitCh <- []*junitapi.JUnitTestCase{
				{
					Name:     testName,
					Duration: duration.Seconds(),
				},
			}
		}(ctx, r.monitorTests[i])
	}

	wg.Wait()
	close(intervalsCh)
	close(junitCh)
	close(errCh)

	intervals := monitorapi.Intervals{}
	junits := []*junitapi.JUnitTestCase{}
	errs := []error{}
	for curr := range intervalsCh {
		intervals = append(intervals, curr...)
	}
	for curr := range junitCh {
		junits = append(junits, curr...)
	}
	for curr := range errCh {
		errs = append(errs, curr)
	}

	return intervals, junits, utilerrors.NewAggregate(errs)
}

func (r *monitorTestRegistry) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	intervals := monitorapi.Intervals{}
	junits := []*junitapi.JUnitTestCase{}
	errs := []error{}

	for _, monitorTest := range r.monitorTests {
		testName := fmt.Sprintf("[Jira:%q] monitor test %v interval construction", monitorTest.jiraComponent, monitorTest.name)

		start := time.Now()
		localIntervals, err := constructComputedIntervalsWithPanicProtection(ctx, monitorTest.monitorTest, startingIntervals, recordedResources, beginning, end)
		intervals = append(intervals, localIntervals...)
		end := time.Now()
		duration := end.Sub(start)
		if err != nil {
			errs = append(errs, err)
			junits = append(junits, &junitapi.JUnitTestCase{
				Name:     testName,
				Duration: duration.Seconds(),
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("failed during interval construction\n%v", err),
				},
				SystemOut: fmt.Sprintf("failed during interval construction\n%v", err),
			})
			continue
		}

		junits = append(junits, &junitapi.JUnitTestCase{
			Name:     testName,
			Duration: duration.Seconds(),
		})
	}

	return intervals, junits, utilerrors.NewAggregate(errs)
}

func (r *monitorTestRegistry) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	junits := []*junitapi.JUnitTestCase{}
	errs := []error{}

	for _, monitorTest := range r.monitorTests {
		testName := fmt.Sprintf("[Jira:%q] monitor test %v test evaluation", monitorTest.jiraComponent, monitorTest.name)

		start := time.Now()
		localJunits, err := evaluateTestsFromConstructedIntervalsWithPanicProtection(ctx, monitorTest.monitorTest, finalIntervals)
		junits = append(junits, localJunits...)
		end := time.Now()
		duration := end.Sub(start)
		if err != nil {
			errs = append(errs, err)
			junits = append(junits, &junitapi.JUnitTestCase{
				Name:     testName,
				Duration: duration.Seconds(),
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("failed during test evaluation\n%v", err),
				},
				SystemOut: fmt.Sprintf("failed during test evaluation\n%v", err),
			})
			continue
		}

		junits = append(junits, &junitapi.JUnitTestCase{
			Name:     testName,
			Duration: duration.Seconds(),
		})
	}

	return junits, utilerrors.NewAggregate(errs)
}

func (r *monitorTestRegistry) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) ([]*junitapi.JUnitTestCase, error) {
	junits := []*junitapi.JUnitTestCase{}
	errs := []error{}

	for _, monitorTest := range r.monitorTests {
		testName := fmt.Sprintf("[Jira:%q] monitor test %v writing to storage", monitorTest.jiraComponent, monitorTest.name)

		start := time.Now()
		err := writeContentToStorageWithPanicProtection(ctx, monitorTest.monitorTest, storageDir, timeSuffix, finalIntervals, finalResourceState)
		end := time.Now()
		duration := end.Sub(start)
		if err != nil {
			errs = append(errs, err)
			junits = append(junits, &junitapi.JUnitTestCase{
				Name:     testName,
				Duration: duration.Seconds(),
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("failed during test evaluation\n%v", err),
				},
				SystemOut: fmt.Sprintf("failed during test evaluation\n%v", err),
			})
			continue
		}

		junits = append(junits, &junitapi.JUnitTestCase{
			Name:     testName,
			Duration: duration.Seconds(),
		})
	}

	return junits, utilerrors.NewAggregate(errs)
}

func (r *monitorTestRegistry) Cleanup(ctx context.Context) ([]*junitapi.JUnitTestCase, error) {
	junits := []*junitapi.JUnitTestCase{}
	errs := []error{}

	for _, monitorTest := range r.monitorTests {
		testName := fmt.Sprintf("[Jira:%q] monitor test %v cleanup", monitorTest.jiraComponent, monitorTest.name)

		start := time.Now()
		err := cleanupWithPanicProtection(ctx, monitorTest.monitorTest)
		end := time.Now()
		duration := end.Sub(start)
		if err != nil {
			errs = append(errs, err)
			junits = append(junits, &junitapi.JUnitTestCase{
				Name:     testName,
				Duration: duration.Seconds(),
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("failed during cleanup\n%v", err),
				},
				SystemOut: fmt.Sprintf("failed during cleanup\n%v", err),
			})
			continue
		}

		junits = append(junits, &junitapi.JUnitTestCase{
			Name:     testName,
			Duration: duration.Seconds(),
		})
	}

	return junits, utilerrors.NewAggregate(errs)
}

func (r *monitorTestRegistry) AddRegistryOrDie(registry MonitorTestRegistry) {
	for _, v := range registry.getMonitorTests() {
		r.AddMonitorTestOrDie(v.name, v.jiraComponent, v.monitorTest)
	}
}

func (r *monitorTestRegistry) getMonitorTests() map[string]*monitorTesttItem {
	return r.monitorTests
}

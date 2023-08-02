package invariants

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

type invariantRegistry struct {
	invariantTests map[string]*invariantItem

	collectData sync.Once
}

type invariantItem struct {
	name          string
	jiraComponent string

	invariantTest InvariantTest
}

func NewInvariantRegistry() InvariantRegistry {
	return &invariantRegistry{
		invariantTests: map[string]*invariantItem{},
	}
}

func (r *invariantRegistry) AddInvariant(name, jiraComponent string, invariantTest InvariantTest) error {
	if _, ok := r.invariantTests[name]; ok {
		return fmt.Errorf("%q is already registered", name)
	}
	r.invariantTests[name] = &invariantItem{
		name:          name,
		jiraComponent: jiraComponent,
		invariantTest: invariantTest,
	}

	return nil
}

func (r *invariantRegistry) AddInvariantOrDie(name, jiraComponent string, invariantTest InvariantTest) {
	err := r.AddInvariant(name, jiraComponent, invariantTest)
	if err != nil {
		panic(err)
	}
}

func (r *invariantRegistry) StartCollection(ctx context.Context, adminRESTConfig *rest.Config) ([]*junitapi.JUnitTestCase, error) {
	junits := []*junitapi.JUnitTestCase{}
	errs := []error{}

	for _, invariant := range r.invariantTests {
		testName := fmt.Sprintf("jira/%q invariant test %v setup", invariant.jiraComponent, invariant.name)

		start := time.Now()
		err := invariant.invariantTest.StartCollection(ctx, adminRESTConfig)
		end := time.Now()
		duration := end.Sub(start)
		if err != nil {
			errs = append(errs, err)
			junits = append(junits, &junitapi.JUnitTestCase{
				Name:     testName,
				Duration: duration.Seconds(),
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("failed during setup\n%v", err),
				},
				SystemOut: fmt.Sprintf("failed during setup\n%v", err),
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

func (r *invariantRegistry) CollectData(ctx context.Context) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	intervals := monitorapi.Intervals{}
	junits := []*junitapi.JUnitTestCase{}
	errs := []error{}

	r.collectData.Do(func() {
		for _, invariant := range r.invariantTests {
			testName := fmt.Sprintf("jira/%q invariant test %v collection", invariant.jiraComponent, invariant.name)

			start := time.Now()
			localIntervals, localJunits, err := invariant.invariantTest.CollectData(ctx)
			junits = append(junits, localJunits...)
			intervals = append(intervals, localIntervals...)
			end := time.Now()
			duration := end.Sub(start)
			if err != nil {
				errs = append(errs, err)
				junits = append(junits, &junitapi.JUnitTestCase{
					Name:     testName,
					Duration: duration.Seconds(),
					FailureOutput: &junitapi.FailureOutput{
						Output: fmt.Sprintf("failed during collection\n%v", err),
					},
					SystemOut: fmt.Sprintf("failed during collection\n%v", err),
				})
				continue
			}

			junits = append(junits, &junitapi.JUnitTestCase{
				Name:     testName,
				Duration: duration.Seconds(),
			})
		}
	})

	return intervals, junits, utilerrors.NewAggregate(errs)
}

func (r *invariantRegistry) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	intervals := monitorapi.Intervals{}
	junits := []*junitapi.JUnitTestCase{}
	errs := []error{}

	r.collectData.Do(func() {
		for _, invariant := range r.invariantTests {
			testName := fmt.Sprintf("jira/%q invariant test %v interval construction", invariant.jiraComponent, invariant.name)

			start := time.Now()
			localIntervals, err := invariant.invariantTest.ConstructComputedIntervals(ctx, startingIntervals)
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
	})

	return intervals, junits, utilerrors.NewAggregate(errs)
}

func (r *invariantRegistry) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	junits := []*junitapi.JUnitTestCase{}
	errs := []error{}

	r.collectData.Do(func() {
		for _, invariant := range r.invariantTests {
			testName := fmt.Sprintf("jira/%q invariant test %v test evaluation", invariant.jiraComponent, invariant.name)

			start := time.Now()
			localJunits, err := invariant.invariantTest.EvaluateTestsFromConstructedIntervals(ctx, finalIntervals)
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
	})

	return junits, utilerrors.NewAggregate(errs)
}

func (r *invariantRegistry) Cleanup(ctx context.Context) ([]*junitapi.JUnitTestCase, error) {
	junits := []*junitapi.JUnitTestCase{}
	errs := []error{}

	for _, invariant := range r.invariantTests {
		testName := fmt.Sprintf("jira/%q invariant test %v cleanup", invariant.jiraComponent, invariant.name)

		start := time.Now()
		err := invariant.invariantTest.Cleanup(ctx)
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

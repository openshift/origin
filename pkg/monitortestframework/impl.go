package monitortestframework

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
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

func (it *monitorTesttItem) Logger() logrus.FieldLogger {
	return logrus.WithFields(logrus.Fields{
		"jiraComponent": it.jiraComponent,
		"monitorTest":   it.name,
	})
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

func (r *monitorTestRegistry) GetRegistryFor(names ...string) (MonitorTestRegistry, error) {
	ret := NewMonitorTestRegistry().(*monitorTestRegistry)

	missingNames := []string{}
	for _, name := range names {
		monitorTestItem, ok := r.monitorTests[name]
		if !ok {
			missingNames = append(missingNames, name)
			continue
		}
		ret.monitorTests[name] = monitorTestItem
	}
	if len(missingNames) > 0 {
		return nil, fmt.Errorf("monitorTests named %v were missing", strings.Join(missingNames, ", "))
	}

	return ret, nil
}

func (r *monitorTestRegistry) ListMonitorTests() sets.String {
	return sets.StringKeySet(r.monitorTests)
}

func (r *monitorTestRegistry) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) ([]*junitapi.JUnitTestCase, error) {
	wg := sync.WaitGroup{}
	junitCh := make(chan *junitapi.JUnitTestCase, 2*len(r.monitorTests))
	errCh := make(chan error, len(r.monitorTests))

	for i := range r.monitorTests {
		wg.Add(1)
		go func(ctx context.Context, invariant *monitorTesttItem) {
			defer wg.Done()

			testName := fmt.Sprintf("[Jira:%q] monitor test %v setup", invariant.jiraComponent, invariant.name)
			invariant.Logger().Infof("began StartCollection")

			start := time.Now()
			err := startCollectionWithPanicProtection(ctx, invariant.monitorTest, adminRESTConfig, recorder)
			end := time.Now()
			duration := end.Sub(start)
			invariant.Logger().Infof("finished StartCollection after %s", duration)
			if err != nil {
				var nsErr *NotSupportedError
				if errors.As(err, &nsErr) {
					junitCh <- &junitapi.JUnitTestCase{
						Name:     testName,
						Duration: duration.Seconds(),
						SkipMessage: &junitapi.SkipMessage{
							Message: nsErr.Reason,
						},
					}
					return
				}
				errCh <- err
				junitCh <- &junitapi.JUnitTestCase{
					Name:     testName,
					Duration: duration.Seconds(),
					FailureOutput: &junitapi.FailureOutput{
						Output: fmt.Sprintf("failed during setup\n%v", err),
					},
					SystemOut: fmt.Sprintf("failed during setup\n%v", err),
				}
				var flakeErr *FlakeError
				if !errors.As(err, &flakeErr) {
					return
				}
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
	junitCh := make(chan []*junitapi.JUnitTestCase, 3*len(r.monitorTests))
	errCh := make(chan error, len(r.monitorTests))

	logrus.Infof("Starting CollectData for all monitor tests")
	for i := range r.monitorTests {
		wg.Add(1)
		go func(ctx context.Context, monitorTest *monitorTesttItem) {
			defer wg.Done()
			testName := fmt.Sprintf("[Jira:%q] monitor test %v collection", monitorTest.jiraComponent, monitorTest.name)

			start := time.Now()
			monitorTest.Logger().Infof("began CollectData")
			localIntervals, localJunits, err := collectDataWithPanicProtection(ctx, monitorTest.monitorTest, storageDir, beginning, end)
			intervalsCh <- localIntervals
			junitCh <- localJunits
			end := time.Now()
			duration := end.Sub(start)
			monitorTest.Logger().Infof("finished CollectData after %s", duration)
			if err != nil {
				var nsErr *NotSupportedError
				if errors.As(err, &nsErr) {
					junitCh <- []*junitapi.JUnitTestCase{
						{
							Name:     testName,
							Duration: duration.Seconds(),
							SkipMessage: &junitapi.SkipMessage{
								Message: nsErr.Reason,
							},
						},
					}
					logrus.WithFields(logrus.Fields{"reason": nsErr.Reason}).Warningf("  Finished CollectData for %s with not supported warning", testName)
					return
				}
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
				var flakeErr *FlakeError
				if !errors.As(err, &flakeErr) {
					logrus.WithError(flakeErr).Errorf("  Finished CollectData for %s with flake error", testName)
					return
				}
			}

			junitCh <- []*junitapi.JUnitTestCase{
				{
					Name:     testName,
					Duration: duration.Seconds(),
				},
			}
			logrus.Infof("  Finished CollectData for %s", testName)
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

	logrus.Infof("Finished CollectData for all monitor tests")
	return intervals, junits, utilerrors.NewAggregate(errs)
}

func (r *monitorTestRegistry) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	intervals := monitorapi.Intervals{}
	junits := []*junitapi.JUnitTestCase{}
	errs := []error{}

	for _, monitorTest := range r.monitorTests {
		testName := fmt.Sprintf("[Jira:%q] monitor test %v interval construction", monitorTest.jiraComponent, monitorTest.name)

		start := time.Now()
		monitorTest.Logger().Infof("began ConstructComputedIntervals")
		localIntervals, err := constructComputedIntervalsWithPanicProtection(ctx, monitorTest.monitorTest, startingIntervals, recordedResources, beginning, end)
		intervals = append(intervals, localIntervals...)
		end := time.Now()
		duration := end.Sub(start)
		monitorTest.Logger().Infof("finished ConstructComputedIntervals after %s", duration)
		if err != nil {
			var nsErr *NotSupportedError
			if errors.As(err, &nsErr) {
				junits = append(junits, &junitapi.JUnitTestCase{
					Name:     testName,
					Duration: duration.Seconds(),
					SkipMessage: &junitapi.SkipMessage{
						Message: nsErr.Reason,
					},
				})
				continue
			}

			errs = append(errs, err)
			junits = append(junits, &junitapi.JUnitTestCase{
				Name:     testName,
				Duration: duration.Seconds(),
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("failed during interval construction\n%v", err),
				},
				SystemOut: fmt.Sprintf("failed during interval construction\n%v", err),
			})
			var flakeErr *FlakeError
			if !errors.As(err, &flakeErr) {
				continue
			}
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
		monitorTest.Logger().Infof("began EvaluateTestsFromConstructedIntervals")
		localJunits, err := evaluateTestsFromConstructedIntervalsWithPanicProtection(ctx, monitorTest.monitorTest, finalIntervals)
		junits = append(junits, localJunits...)
		end := time.Now()
		duration := end.Sub(start)
		monitorTest.Logger().Infof("finished EvaluateTestsFromConstructedIntervals after %s", duration)
		if err != nil {
			var nsErr *NotSupportedError
			if errors.As(err, &nsErr) {
				junits = append(junits, &junitapi.JUnitTestCase{
					Name:     testName,
					Duration: duration.Seconds(),
					SkipMessage: &junitapi.SkipMessage{
						Message: nsErr.Reason,
					},
				})
				continue
			}

			errs = append(errs, err)
			junits = append(junits, &junitapi.JUnitTestCase{
				Name:     testName,
				Duration: duration.Seconds(),
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("failed during test evaluation\n%v", err),
				},
				SystemOut: fmt.Sprintf("failed during test evaluation\n%v", err),
			})
			var flakeErr *FlakeError
			if !errors.As(err, &flakeErr) {
				continue
			}
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

		var finalIntervalLength = len(finalIntervals)
		fmt.Fprintf(os.Stderr, "Processing monitorTest: %s\n", monitorTest.name)
		fmt.Fprintf(os.Stderr, "  finalIntervals size = %d\n", finalIntervalLength)
		if finalIntervalLength > 1 {
			fmt.Fprintf(os.Stderr, "  first interval time: From = %s; To = %s\n", finalIntervals[0].From, finalIntervals[0].To)
			fmt.Fprintf(os.Stderr, "  last interval time: From = %s; To = %s\n", finalIntervals[finalIntervalLength-1].From, finalIntervals[finalIntervalLength-1].To)
		}

		monitorTest.Logger().Infof("began WriteContentToStorage")
		err := writeContentToStorageWithPanicProtection(ctx, monitorTest.monitorTest, storageDir, timeSuffix, finalIntervals, finalResourceState)
		end := time.Now()
		duration := end.Sub(start)
		monitorTest.Logger().Infof("finished WriteContentToStorage after %s", duration)
		if err != nil {
			var nsErr *NotSupportedError
			if errors.As(err, &nsErr) {
				junits = append(junits, &junitapi.JUnitTestCase{
					Name:     testName,
					Duration: duration.Seconds(),
					SkipMessage: &junitapi.SkipMessage{
						Message: nsErr.Reason,
					},
				})
				continue
			}

			errs = append(errs, err)
			junits = append(junits, &junitapi.JUnitTestCase{
				Name:     testName,
				Duration: duration.Seconds(),
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("failed during test evaluation\n%v", err),
				},
				SystemOut: fmt.Sprintf("failed during test evaluation\n%v", err),
			})
			var flakeErr *FlakeError
			if !errors.As(err, &flakeErr) {
				continue
			}
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
		monitorTest.Logger().Infof("began Cleanup")
		err := cleanupWithPanicProtection(ctx, monitorTest.monitorTest)
		end := time.Now()
		duration := end.Sub(start)
		monitorTest.Logger().Infof("finished Cleanup after %s", duration)
		if err != nil {
			var nsErr *NotSupportedError
			if errors.As(err, &nsErr) {
				junits = append(junits, &junitapi.JUnitTestCase{
					Name:     testName,
					Duration: duration.Seconds(),
					SkipMessage: &junitapi.SkipMessage{
						Message: nsErr.Reason,
					},
				})
				continue
			}

			monitorTest.Logger().WithError(err).Error("failed during cleanup")
			errs = append(errs, err)
			junits = append(junits, &junitapi.JUnitTestCase{
				Name:     testName,
				Duration: duration.Seconds(),
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("failed during cleanup\n%v", err),
				},
				SystemOut: fmt.Sprintf("failed during cleanup\n%v", err),
			})
			var flakeErr *FlakeError
			if !errors.As(err, &flakeErr) {
				continue
			}
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

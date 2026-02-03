package generationanalyzer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortests/testframework/watchnamespaces"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const maxGenerationAllowed = 50

type generationWatcher struct {
	kubeClient kubernetes.Interface
}

func NewGenerationAnalyzer() monitortestframework.MonitorTest {
	return &generationWatcher{}
}

func (w *generationWatcher) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	kubeClient, err := kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}
	w.kubeClient = kubeClient
	startGenerationMonitoring(ctx, recorder, kubeClient)
	return nil
}

func (w *generationWatcher) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *generationWatcher) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	// because we are sharing a recorder that we're streaming into, we don't need to have a separate data collection step.
	return nil, nil, nil
}

func (w *generationWatcher) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *generationWatcher) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	platformNamespaces, err := watchnamespaces.GetAllPlatformNamespaces()
	if err != nil {
		return nil, err
	}

	intervalHighGenerationFailures := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Message.Reason == monitorapi.ReasonHighGeneration
	})

	namespaceToHighGenerationFailure := map[string][]string{}
	for _, failure := range intervalHighGenerationFailures {
		namespace := failure.Locator.Keys[monitorapi.LocatorNamespaceKey]
		namespaceToHighGenerationFailure[namespace] = append(namespaceToHighGenerationFailure[namespace], failure.String())
	}

	intervalInvalidGenerationFailures := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Message.Reason == monitorapi.ReasonInvalidGeneration
	})

	namespaceToInvalidGenerationFailure := map[string][]string{}
	for _, failure := range intervalInvalidGenerationFailures {
		namespace := failure.Locator.Keys[monitorapi.LocatorNamespaceKey]
		namespaceToInvalidGenerationFailure[namespace] = append(namespaceToInvalidGenerationFailure[namespace], failure.String())
	}

	ret := []*junitapi.JUnitTestCase{}
	for _, namespace := range platformNamespaces {
		// Generation should not be too high
		testNameHighGeneration := fmt.Sprintf("objects in ns/%s should not have too many generations", namespace)
		nsFailuresHighGeneration := namespaceToHighGenerationFailure[namespace]
		if len(nsFailuresHighGeneration) > 0 {
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: testNameHighGeneration,
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("objects had a metadata.Generation higher than %d\n%s", maxGenerationAllowed, strings.Join(nsFailuresHighGeneration, "\n")),
				},
			})
			// Flake for now
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: testNameHighGeneration,
			})
		} else {
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: testNameHighGeneration,
			})
		}

		// ObservedGeneration should increase monotonically
		testNameInvalidGeneration := fmt.Sprintf("objects in ns/%s should have generation increasing monotonically", namespace)
		nsFailuresInvalidGeneration := namespaceToInvalidGenerationFailure[namespace]
		if len(nsFailuresInvalidGeneration) > 0 {
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: testNameInvalidGeneration,
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("objects had observed generation increasing non-monotonically\n%s", strings.Join(nsFailuresInvalidGeneration, "\n")),
				},
			})
			// Flake for now
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: testNameInvalidGeneration,
			})
		} else {
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: testNameInvalidGeneration,
			})
		}

	}

	return ret, nil
}

func (w *generationWatcher) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (w *generationWatcher) Cleanup(ctx context.Context) error {
	return nil
}

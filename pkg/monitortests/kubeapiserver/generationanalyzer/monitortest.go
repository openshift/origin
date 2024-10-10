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

// FIXME: 3 is just to see it working
const maxGenerationAllowed = 3

type generationWatcher struct {
	kubeClient kubernetes.Interface
}

func NewGenerationAnalyzer() monitortestframework.MonitorTest {
	return &generationWatcher{}
}

func (w *generationWatcher) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	kubeClient, err := kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}
	w.kubeClient = kubeClient
	startGenerationMonitoring(ctx, recorder, kubeClient)
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

	intervalFailures := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Message.Reason == monitorapi.ReasonHighGeneration
	})

	namespaceToFailure := map[string][]string{}
	for _, failure := range intervalFailures {
		namespace := failure.Locator.Keys[monitorapi.LocatorNamespaceKey]
		namespaceToFailure[namespace] = append(namespaceToFailure[namespace], failure.String())
	}

	ret := []*junitapi.JUnitTestCase{}
	for _, namespace := range platformNamespaces {
		testName := fmt.Sprintf("objects in ns/%s should not have a generation greater than %d", namespace, maxGenerationAllowed)
		nsFailures := namespaceToFailure[namespace]
		if len(nsFailures) > 0 {
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Message: strings.Join(nsFailures, "\n"),
					Output:  fmt.Sprintf("objects had a metadata.Generation higher than %d", maxGenerationAllowed),
				},
			})
			// Flake for now
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: testName,
			})
		} else {
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: testName,
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

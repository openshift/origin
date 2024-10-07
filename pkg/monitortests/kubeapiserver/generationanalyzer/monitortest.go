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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	ret := []*junitapi.JUnitTestCase{}

	intervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Message.Reason == monitorapi.ReasonHighGeneration
	})

	highGenerationFailures := []string{}
	for _, interval := range intervals {
		highGenerationFailures = append(highGenerationFailures, interval.String())
	}

	// This test flag objects that have been created/updated/deleted and had a high generation value
	testName := fmt.Sprintf("[Jira:kube-apiserver] manipulated objects should not have a generation greater than %d", maxGenerationAllowed)
	if len(highGenerationFailures) > 0 {
		ret = append(ret, &junitapi.JUnitTestCase{
			Name: testName,
			FailureOutput: &junitapi.FailureOutput{
				Message: strings.Join(highGenerationFailures, "\n"),
				Output:  fmt.Sprintf("Objects had a metadata.Generation higher than %d", maxGenerationAllowed),
			},
		})
		// flake for now
		ret = append(ret, &junitapi.JUnitTestCase{
			Name: testName,
		})
	} else {
		ret = append(ret, &junitapi.JUnitTestCase{
			Name: testName,
		})
	}

	// Now check objects in all platform namespaces. It's possible that objects with high
	// generation existed beforetests ran, so they were not caught by the previous test.
	// This may catch the same failures as the previous test.
	allPlatformNamespaces, err := watchnamespaces.GetAllPlatformNamespaces()
	if err != nil {
		return nil, fmt.Errorf("problem getting platform namespaces: %w", err)
	}

	failures := []string{}
	for _, namespace := range allPlatformNamespaces {
		allDeployments, err := w.kubeClient.AppsV1().Deployments(namespace).List(context.TODO(), v1.ListOptions{})
		if err != nil {
			return ret, err
		}

		allDaemonSets, err := w.kubeClient.AppsV1().DaemonSets(namespace).List(context.TODO(), v1.ListOptions{})
		if err != nil {
			return ret, err
		}

		allStatefulSets, err := w.kubeClient.AppsV1().StatefulSets(namespace).List(context.TODO(), v1.ListOptions{})
		if err != nil {
			return ret, err
		}

		allObjs := []metav1.Object{}
		for _, deploy := range allDeployments.Items {
			allObjs = append(allObjs, &deploy)
		}
		for _, ds := range allDaemonSets.Items {
			allObjs = append(allObjs, &ds)
		}
		for _, sts := range allStatefulSets.Items {
			allObjs = append(allObjs, &sts)
		}

		for _, obj := range allObjs {
			if obj.GetGeneration() > maxGenerationAllowed {
				failures = append(failures, fmt.Sprintf("object %s/%s of type %T had generation %d", obj.GetNamespace(), obj.GetName(), obj, obj.GetGeneration()))
			}
		}
	}

	testName = fmt.Sprintf("[Jira:kube-apiserver] namespaces should not have objects with a generation greater than %d", maxGenerationAllowed)
	if len(failures) > 0 {
		ret = append(ret, &junitapi.JUnitTestCase{
			Name: testName,
			FailureOutput: &junitapi.FailureOutput{
				Message: strings.Join(failures, "\n"),
				Output:  fmt.Sprintf("Objects had a metadata.Generation higher than %d", maxGenerationAllowed),
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

	return ret, nil
}

func (w *generationWatcher) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (w *generationWatcher) Cleanup(ctx context.Context) error {
	return nil
}

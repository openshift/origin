package containerfailures

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortests/testframework/watchnamespaces"

	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
)

const (
	MonitorName = "kubelet-container-restarts"
)

type containerFailuresTests struct {
	adminRESTConfig *rest.Config
}

func NewContainerFailuresTests() monitortestframework.MonitorTest {
	return &containerFailuresTests{}
}

func (w *containerFailuresTests) PrepareCollection(context.Context, *rest.Config, monitorapi.RecorderWriter) error {
	return nil
}

func (w *containerFailuresTests) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, _ monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	return nil
}

func (w *containerFailuresTests) CollectData(context.Context, string, time.Time, time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (*containerFailuresTests) ConstructComputedIntervals(context.Context, monitorapi.Intervals, monitorapi.ResourcesMap, time.Time, time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *containerFailuresTests) EvaluateTestsFromConstructedIntervals(_ context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	openshiftNamespaces, err := watchnamespaces.GetAllPlatformNamespaces()
	if err != nil {
		// Should not happen
		return nil, fmt.Errorf("unable to get platform namespaces %w", err)
	}
	containerExitsByNamespace := map[string]map[string][]string{}
	failuresByNamespace := map[string][]string{}
	for _, event := range finalIntervals {
		namespace := event.Locator.Keys[monitorapi.LocatorNamespaceKey]

		reason := event.Message.Reason
		code := event.Message.Annotations[monitorapi.AnnotationContainerExitCode]
		switch {
		// errors during container start should be highlighted because they are unexpected
		case reason == monitorapi.ContainerReasonContainerWait:
			if event.Message.Annotations[monitorapi.AnnotationCause] == "ContainerCreating" {
				continue
			}
			failuresByNamespace[namespace] = append(failuresByNamespace[namespace], fmt.Sprintf("container failed to start at %v: %v - %v", event.From, event.Locator.OldLocator(), event.Message.OldMessage()))

		// workload containers should never exit non-zero during normal operations
		case reason == monitorapi.ContainerReasonContainerExit && code != "0":
			containerExits, ok := containerExitsByNamespace[namespace]
			if !ok {
				containerExits = map[string][]string{}
			}
			containerExits[event.Locator.OldLocator()] = append(containerExits[event.Locator.OldLocator()], fmt.Sprintf("non-zero exit at %v: %v", event.From, event.Message.OldMessage()))
			containerExitsByNamespace[namespace] = containerExits
		}
	}
	// This is a map of the tests we want to fail on
	// In this case, this is any container that restarts more than 3 times
	excessiveExitsByNamespaceForFailedTests := map[string][]string{}
	// We want to report restarts of openshift containers as flakes
	excessiveExitsByNamespaceForFlakeTests := map[string][]string{}

	maxRestartCountForFailures := 4
	maxRestartCountForFlakes := 2

	clusterDataPlatform, _ := platformidentification.BuildClusterData(context.Background(), w.adminRESTConfig)

	exclusions := Exclusion{clusterData: clusterDataPlatform}
	for namespace, containerExits := range containerExitsByNamespace {
		for locator, messages := range containerExits {
			if len(messages) > 0 {
				messageSet := sets.NewString(messages...)
				// Blanket fail for restarts over maxRestartCount
				if !isThisContainerRestartExcluded(locator, exclusions) && len(messages) > maxRestartCountForFailures {
					excessiveExitsByNamespaceForFailedTests[namespace] = append(excessiveExitsByNamespaceForFailedTests[namespace], fmt.Sprintf("%s restarted %d times at:\n%s", locator, len(messages), strings.Join(messageSet.List(), "\n")))
				} else if len(messages) >= maxRestartCountForFlakes {
					excessiveExitsByNamespaceForFlakeTests[namespace] = append(excessiveExitsByNamespaceForFlakeTests[namespace], fmt.Sprintf("%s restarted %d times at:\n%s", locator, len(messages), strings.Join(messageSet.List(), "\n")))
				}
			}
		}
	}
	for namespace, excessiveExitsFails := range excessiveExitsByNamespaceForFailedTests {
		sort.Strings(excessiveExitsFails)
		excessiveExitsByNamespaceForFailedTests[namespace] = excessiveExitsFails
	}
	for namespace, excessiveExitsFlakes := range excessiveExitsByNamespaceForFlakeTests {
		sort.Strings(excessiveExitsFlakes)
		excessiveExitsByNamespaceForFlakeTests[namespace] = excessiveExitsFlakes
	}

	var testCases []*junitapi.JUnitTestCase

	for _, namespace := range openshiftNamespaces { // this ensures we create test case for every namespace, even in success cases
		failures := failuresByNamespace[namespace]
		failToStartTestName := fmt.Sprintf("[sig-architecture] platform pods in ns/%s should not fail to start", namespace)
		if len(failures) > 0 {
			testCases = append(testCases, &junitapi.JUnitTestCase{
				Name:      failToStartTestName,
				SystemOut: strings.Join(failures, "\n"),
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("%d container starts had issues\n\n%s", len(failures), strings.Join(failures, "\n")),
				},
			})
		}
		// mark flaky for now while we debug
		testCases = append(testCases, &junitapi.JUnitTestCase{Name: failToStartTestName})
	}

	// We have identified more than 3 restarts as an excessive amount
	// This will not be tolerated anymore so the test will fail in this case.
	for _, namespace := range openshiftNamespaces { // this ensures we create test case for every namespace, even in success cases
		excessiveExits := excessiveExitsByNamespaceForFailedTests[namespace]
		excessiveRestartTestName := fmt.Sprintf("[sig-architecture] platform pods in ns/%s should not exit an excessive amount of times", namespace)
		if len(excessiveExits) > 0 {
			testCases = append(testCases, &junitapi.JUnitTestCase{
				Name:      excessiveRestartTestName,
				SystemOut: strings.Join(excessiveExits, "\n"),
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("%d containers with multiple restarts\n\n%s", len(excessiveExits), strings.Join(excessiveExits, "\n\n")),
				},
			})
		} else {
			testCases = append(testCases, &junitapi.JUnitTestCase{Name: excessiveRestartTestName})
		}
	}

	// We have indentified more than 2 restarts to be considered moderate.
	// We will investigate these as flakes and potentially bring these up as bugs to fix.
	for _, namespace := range openshiftNamespaces { // this ensures we create test case for every namespace, even in success cases
		excessiveExits := excessiveExitsByNamespaceForFlakeTests[namespace]
		excessiveRestartTestNameForFlakes := fmt.Sprintf("[sig-architecture] platform pods in ns/%s should not exit a moderate amount of times", namespace)
		if len(excessiveExits) > 0 {
			testCases = append(testCases, &junitapi.JUnitTestCase{
				Name:      excessiveRestartTestNameForFlakes,
				SystemOut: strings.Join(excessiveExits, "\n"),
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("%d containers with multiple restarts\n\n%s", len(excessiveExits), strings.Join(excessiveExits, "\n\n")),
				},
			})
		}
		testCases = append(testCases, &junitapi.JUnitTestCase{Name: excessiveRestartTestNameForFlakes})
	}

	return testCases, nil
}

func (*containerFailuresTests) WriteContentToStorage(context.Context, string, string, monitorapi.Intervals, monitorapi.ResourcesMap) error {
	return nil
}

func (*containerFailuresTests) Cleanup(context.Context) error {
	return nil
}

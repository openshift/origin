package containerfailures

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortests/testframework/watchnamespaces"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

func TestEvaluateTestsFromConstructedIntervalsWithUnavailableNamespaceData(t *testing.T) {
	if _, err := watchnamespaces.GetAllPlatformNamespaces(); err == nil {
		t.Fatal("test requires platform namespace data to be unavailable")
	}

	intervals := monitorapi.Intervals{
		containerExitInterval("openshift-z", "platform-z", "z-container", "0", 0),
		containerExitInterval("openshift-a", "platform-a", "a-container", "0", 1),
		containerExitInterval("openshift-z", "platform-z-duplicate", "z-container", "0", 2),
		containerExitInterval("workload", "workload-pod", "workload-container", "1", 3),
		containerExitInterval("", "pod-without-namespace", "container", "1", 4),
	}

	testCases := evaluateContainerFailures(t, intervals)

	expectedNames := []string{
		"[sig-architecture] platform pods in ns/openshift-a should not fail to start",
		"[sig-architecture] platform pods in ns/openshift-z should not fail to start",
		"[sig-architecture] platform pods in ns/openshift-a should not exit an excessive amount of times",
		"[sig-architecture] platform pods in ns/openshift-z should not exit an excessive amount of times",
		"[sig-architecture] platform pods in ns/openshift-a should not exit a moderate amount of times",
		"[sig-architecture] platform pods in ns/openshift-z should not exit a moderate amount of times",
	}

	assertTestCaseNames(t, testCases, expectedNames)
}

func TestEvaluateTestsFromConstructedIntervalsWithNoNamespaces(t *testing.T) {
	if _, err := watchnamespaces.GetAllPlatformNamespaces(); err == nil {
		t.Fatal("test requires platform namespace data to be unavailable")
	}

	testCases := evaluateContainerFailures(t, nil)
	if len(testCases) != 0 {
		t.Fatalf("expected no test cases for empty intervals, got %d: %v", len(testCases), testCaseNames(testCases))
	}
}

func TestEvaluateTestsFromConstructedIntervalsReportsRestartFailures(t *testing.T) {
	if _, err := watchnamespaces.GetAllPlatformNamespaces(); err == nil {
		t.Fatal("test requires platform namespace data to be unavailable")
	}

	var intervals monitorapi.Intervals
	for i := 0; i < 4; i++ {
		intervals = append(intervals, containerExitInterval("openshift-monitoring", "prometheus", "prometheus", "1", i))
	}

	testCases := evaluateContainerFailures(t, intervals)

	excessiveRestartTestName := "[sig-architecture] platform pods in ns/openshift-monitoring should not exit an excessive amount of times"
	excessiveRestartTest := findTestCase(t, testCases, excessiveRestartTestName)
	if excessiveRestartTest.FailureOutput == nil {
		t.Fatalf("expected %q to fail, got %#v", excessiveRestartTestName, excessiveRestartTest)
	}
	if want := "restarted 4 times"; !strings.Contains(excessiveRestartTest.FailureOutput.Output, want) {
		t.Fatalf("expected failure output to contain %q, got %q", want, excessiveRestartTest.FailureOutput.Output)
	}
}

func evaluateContainerFailures(t *testing.T, intervals monitorapi.Intervals) []*junitapi.JUnitTestCase {
	t.Helper()

	// The evaluator normally receives this config from StartCollection. Use an
	// unreachable local endpoint so cluster metadata lookup fails without
	// making the test depend on a real cluster.
	testMonitor := &containerFailuresTests{adminRESTConfig: &rest.Config{Host: "http://127.0.0.1:0"}}
	testCases, err := testMonitor.EvaluateTestsFromConstructedIntervals(context.Background(), intervals)
	if err != nil {
		t.Fatalf("EvaluateTestsFromConstructedIntervals returned an error: %v", err)
	}
	return testCases
}

func containerExitInterval(namespace, pod, container, code string, sequence int) monitorapi.Interval {
	return monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Error).
		Locator(monitorapi.NewLocator().ContainerFromNames(namespace, pod, "uid", container)).
		Message(monitorapi.NewMessage().
			Reason(monitorapi.ContainerReasonContainerExit).
			WithAnnotation(monitorapi.AnnotationContainerExitCode, code)).
		Build(time.Unix(int64(sequence), 0), time.Unix(int64(sequence+1), 0))
}

func assertTestCaseNames(t *testing.T, testCases []*junitapi.JUnitTestCase, expected []string) {
	t.Helper()

	actual := testCaseNames(testCases)
	if len(actual) != len(expected) {
		t.Fatalf("expected test case names %v, got %v", expected, actual)
	}
	for i := range expected {
		if actual[i] != expected[i] {
			t.Errorf("test case %d: expected %q, got %q", i, expected[i], actual[i])
		}
	}
}

func testCaseNames(testCases []*junitapi.JUnitTestCase) []string {
	names := make([]string, 0, len(testCases))
	for _, testCase := range testCases {
		names = append(names, testCase.Name)
	}
	return names
}

func findTestCase(t *testing.T, testCases []*junitapi.JUnitTestCase, name string) *junitapi.JUnitTestCase {
	t.Helper()

	for _, testCase := range testCases {
		if testCase.Name == name {
			return testCase
		}
	}
	t.Fatalf("test case %q not found in %v", name, testCaseNames(testCases))
	return nil
}

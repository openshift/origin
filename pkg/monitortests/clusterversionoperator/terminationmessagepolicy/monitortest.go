package terminationmessagepolicy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type terminationMessagePolicyChecker struct {
	kubeClient kubernetes.Interface
}

func NewAnalyzer() monitortestframework.MonitorTest {
	return &terminationMessagePolicyChecker{}
}

func (w *terminationMessagePolicyChecker) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	var err error
	w.kubeClient, err = kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}
	return nil
}

func (w *terminationMessagePolicyChecker) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if w.kubeClient == nil {
		return nil, nil, nil
	}
	allPods, err := w.kubeClient.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}

	failuresByNamespace := map[string][]string{}
	for _, pod := range allPods.Items {
		if !strings.HasPrefix(pod.Namespace, "openshift") {
			continue
		}
		// skip generated platform namespaces
		if strings.HasPrefix(pod.Namespace, "openshift-must-gather") {
			continue
		}
		if _, ok := failuresByNamespace[pod.Namespace]; !ok {
			failuresByNamespace[pod.Namespace] = []string{}
		}

		for _, container := range pod.Spec.InitContainers {
			if container.TerminationMessagePolicy != corev1.TerminationMessageFallbackToLogsOnError {
				failuresByNamespace[pod.Namespace] = append(failuresByNamespace[pod.Namespace],
					fmt.Sprintf("pods/%s initContainers[%v]", pod.Name, container.Name))
			}
		}
		for _, container := range pod.Spec.Containers {
			if container.TerminationMessagePolicy != corev1.TerminationMessageFallbackToLogsOnError {
				failuresByNamespace[pod.Namespace] = append(failuresByNamespace[pod.Namespace],
					fmt.Sprintf("pods/%s containers[%v]", pod.Name, container.Name))

			}
		}
		for _, container := range pod.Spec.EphemeralContainers {
			if container.TerminationMessagePolicy != corev1.TerminationMessageFallbackToLogsOnError {
				failuresByNamespace[pod.Namespace] = append(failuresByNamespace[pod.Namespace],
					fmt.Sprintf("pods/%s ephemeralContainers[%v]", pod.Name, container.Name))

			}
		}
	}

	// existingViolations is the list of violations already present, don't add to it once we start enforcing
	existingViolations := map[string]sets.String{
		"namespace": sets.NewString("pods/<name> <containerType>[<containerName>]"),
	}

	junits := []*junitapi.JUnitTestCase{}
	for _, namespace := range sets.StringKeySet(failuresByNamespace).List() {
		testName := fmt.Sprintf("[sig-arch] all containers in ns/%v must have terminationMessagePolicy=%v", namespace, corev1.TerminationMessageFallbackToLogsOnError)
		failingContainers := sets.NewString(failuresByNamespace[namespace]...)
		if len(failingContainers) == 0 {
			junits = append(junits, &junitapi.JUnitTestCase{
				Name:      testName,
				SystemOut: "",
				SystemErr: "",
			})
			continue
		}

		if existingViolationForNamespace, ok := existingViolations[namespace]; ok {
			newViolatingContainers := failingContainers.Difference(existingViolationForNamespace)
			if len(newViolatingContainers) == 0 {
				junits = append(junits, &junitapi.JUnitTestCase{
					Name:      testName,
					SystemOut: "",
					SystemErr: "",
				})
				continue
			}
			failingContainers = newViolatingContainers
		}

		failureMessages := []string{}
		for _, container := range failingContainers.List() {
			failureMessages = append(failureMessages,
				fmt.Sprintf("%v must have terminationMessagePolicy=%q",
					container, corev1.TerminationMessageFallbackToLogsOnError))
		}

		junits = append(junits,
			&junitapi.JUnitTestCase{
				Name:      testName,
				SystemOut: strings.Join(failureMessages, "\n"),
				FailureOutput: &junitapi.FailureOutput{
					Output: strings.Join(failureMessages, "\n"),
				},
			},
			// start as flake to build whitelist
			&junitapi.JUnitTestCase{
				Name:      testName,
				SystemOut: "",
				SystemErr: "",
			},
		)

	}

	return nil, junits, nil
}

func (*terminationMessagePolicyChecker) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*terminationMessagePolicyChecker) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (*terminationMessagePolicyChecker) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*terminationMessagePolicyChecker) Cleanup(ctx context.Context) error {
	return nil
}

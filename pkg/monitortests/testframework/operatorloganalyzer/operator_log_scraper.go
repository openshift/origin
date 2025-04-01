package operatorloganalyzer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortests/testframework/watchnamespaces"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/podaccess"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type operatorLogAnalyzer struct {
	kubeClient kubernetes.Interface
}

func InitialAndFinalOperatorLogScraper() monitortestframework.MonitorTest {
	return &operatorLogAnalyzer{}
}

func (w *operatorLogAnalyzer) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *operatorLogAnalyzer) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	var err error
	w.kubeClient, err = kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	if err := scanAllOperatorPods(ctx, w.kubeClient, newOperatorLogHandler(recorder)); err != nil {
		return fmt.Errorf("unable to scan operator logs: %w", err)
	}

	return nil
}

func scanAllOperatorPods(ctx context.Context, kubeClient kubernetes.Interface, logHandlers ...podaccess.LogHandler) error {
	pods, err := kubeClient.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("couldn't list pods: %w", err)
	}

	errs := []error{}
	for _, pod := range pods.Items {
		if !strings.HasPrefix(pod.Namespace, "openshift-") {
			continue
		}
		if !strings.Contains(pod.Name, "operator") {
			continue
		}
		// this is just a basic check to see if we can expect logs to be present. Unready, unhealthy, and failed pods all still have logs.
		if pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodUnknown {
			continue
		}

		for _, container := range pod.Spec.Containers {
			streamer := podaccess.NewOneTimePodStreamer(kubeClient, pod.Namespace, pod.Name, container.Name, logHandlers...)
			if err := streamer.ReadLog(ctx); err != nil && !apierrors.IsNotFound(err) {
				errs = append(errs, fmt.Errorf("error reading log for pods/%s -n %s -c %s: %w", pod.Name, pod.Namespace, container.Name, err))
			}
		}
	}

	return errors.Join(errs...)
}

func (w *operatorLogAnalyzer) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	localRecorder := monitor.NewRecorder()
	if err := scanAllOperatorPods(ctx, w.kubeClient, newOperatorLogHandlerAfterTime(localRecorder, beginning)); err != nil {
		return nil, nil, fmt.Errorf("unable to scan operator logs: %w", err)
	}

	return localRecorder.Intervals(time.Time{}, time.Time{}), nil, nil
}

func (*operatorLogAnalyzer) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*operatorLogAnalyzer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	platformNamespaces, err := watchnamespaces.GetAllPlatformNamespaces()
	if err != nil {
		return nil, err
	}

	ret := []*junitapi.JUnitTestCase{}

	applyFailures := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Message.Reason == monitorapi.ReasonBadOperatorApply
	})

	namespaceToApplyFailures := map[string][]string{}
	for _, applyFailure := range applyFailures {
		namespace := applyFailure.Locator.Keys[monitorapi.LocatorNamespaceKey]
		namespaceToApplyFailures[namespace] = append(namespaceToApplyFailures[namespace], applyFailure.String())
	}

	for _, nsName := range platformNamespaces {
		testName := fmt.Sprintf("operators in in ns/%s should not submit invalid apply statements", nsName)
		nsFailures := namespaceToApplyFailures[nsName]
		if len(nsFailures) > 0 {
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Message: strings.Join(nsFailures, "\n"),
					Output:  fmt.Sprintf("found %d invalid applies in the log", len(nsFailures)),
				},
			})
			// flake because Stephen will want it that way this week.
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

func (w *operatorLogAnalyzer) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*operatorLogAnalyzer) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}

type operatorLogHandler struct {
	recorder  monitorapi.RecorderWriter
	afterTime *time.Time
}

func newOperatorLogHandler(recorder monitorapi.RecorderWriter) operatorLogHandler {
	return operatorLogHandler{
		recorder: recorder,
	}
}

func newOperatorLogHandlerAfterTime(recorder monitorapi.RecorderWriter, afterTime time.Time) operatorLogHandler {
	return operatorLogHandler{
		recorder:  recorder,
		afterTime: &afterTime,
	}
}

func (g operatorLogHandler) HandleLogLine(logLine podaccess.LogLineContent) {
	if g.afterTime != nil {
		if logLine.Instant.Before(*g.afterTime) {
			return
		}
	}
	switch {
	case strings.Contains(logLine.Line, "attempting to acquire leader lease") &&
		!strings.Contains(logLine.Line, "Degraded"): // need to exclude lines that re-embed the kube-controller-manager log
		g.recorder.AddIntervals(
			monitorapi.NewInterval(monitorapi.SourcePodLog, monitorapi.Info).
				Locator(logLine.Locator).
				Message(monitorapi.NewMessage().
					Reason(monitorapi.LeaseAcquiringStarted).
					HumanMessage(logLine.Line),
				).
				Build(logLine.Instant, logLine.Instant.Add(time.Second)),
		)
	case strings.Contains(logLine.Line, "successfully acquired lease") &&
		!strings.Contains(logLine.Line, "Degraded"): // need to exclude lines that re-embed the kube-controller-manager log
		g.recorder.AddIntervals(
			monitorapi.NewInterval(monitorapi.SourcePodLog, monitorapi.Info).
				Locator(logLine.Locator).
				Message(monitorapi.NewMessage().
					Reason(monitorapi.LeaseAcquired).
					HumanMessage(logLine.Line),
				).
				Build(logLine.Instant, logLine.Instant.Add(time.Second)),
		)
	case strings.Contains(logLine.Line, "unable to ApplyStatus for operator") &&
		strings.Contains(logLine.Line, "is invalid"): // apply failures
		g.recorder.AddIntervals(
			monitorapi.NewInterval(monitorapi.SourcePodLog, monitorapi.Error).
				Locator(logLine.Locator).
				Message(monitorapi.NewMessage().
					Reason(monitorapi.ReasonBadOperatorApply).
					HumanMessage(logLine.Line),
				).
				Build(logLine.Instant, logLine.Instant.Add(time.Second)),
		)
	case strings.Contains(logLine.Line, "unable to Apply for operator") &&
		strings.Contains(logLine.Line, "is invalid"): // apply failures
		g.recorder.AddIntervals(
			monitorapi.NewInterval(monitorapi.SourcePodLog, monitorapi.Error).
				Locator(logLine.Locator).
				Message(monitorapi.NewMessage().
					Reason(monitorapi.ReasonBadOperatorApply).
					HumanMessage(logLine.Line),
				).
				Build(logLine.Instant, logLine.Instant.Add(time.Second)),
		)
	case strings.Contains(logLine.Line, "Removing bootstrap member") || strings.Contains(logLine.Line, "Successfully removed bootstrap member") || strings.Contains(logLine.Line, "Cluster etcd operator bootstrapped successfully"): // ceo removed bootstrap member
		g.recorder.AddIntervals(
			monitorapi.NewInterval(monitorapi.SourcePodLog, monitorapi.Info).
				Locator(logLine.Locator).
				Display().
				Message(monitorapi.NewMessage().
					Reason(monitorapi.ReasonEtcdBootstrap).
					HumanMessage(logLine.Line),
				).
				Build(logLine.Instant, logLine.Instant.Add(time.Second)),
		)
	}

}

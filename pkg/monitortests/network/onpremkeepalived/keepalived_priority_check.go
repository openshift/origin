package onpremkeepalived

import (
	"context"
	"errors"
	"fmt"
	"github.com/openshift/origin/pkg/monitor"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"regexp"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/podaccess"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type operatorLogAnalyzer struct {
	kubeClient kubernetes.Interface
}

func InitialAndFinalOperatorLogScraper() monitortestframework.MonitorTest {
	return &operatorLogAnalyzer{}
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
	onPremPlatforms := []string{"kni", "openstack", "vsphere"}
	errs := []error{}
	for _, platform := range onPremPlatforms {

		pods, err := kubeClient.CoreV1().Pods(fmt.Sprintf("openshift-%s-infra", platform)).List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("app=%s-infra-vrrp", platform)})
		if err != nil {
			return fmt.Errorf("couldn't list pods: %w", err)
		}

		for _, pod := range pods.Items {
			// this is just a basic check to see if we can expect logs to be present. Unready, unhealthy, and failed pods all still have logs.
			if pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodUnknown {
				continue
			}

			for _, container := range pod.Spec.Containers {
				if container.Name == "keepalived" {
					streamer := podaccess.NewOneTimePodStreamer(kubeClient, pod.Namespace, pod.Name, container.Name, logHandlers...)
					if err := streamer.ReadLog(ctx); err != nil && !apierrors.IsNotFound(err) {
						errs = append(errs, fmt.Errorf("error reading log for pods/%s -n %s -c %s: %w", pod.Name, pod.Namespace, container.Name, err))
					}
				}
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
	re := regexp.MustCompile("effective priority from (?P<PREV_PRIO>[\\d]+) to (?P<CURR_PRIO>[\\d]+)")
	if g.afterTime != nil {
		if logLine.Instant.Before(*g.afterTime) {
			return
		}
	}
	switch {
	case re.MatchString(logLine.Line):
		subMatches := re.FindStringSubmatch(logLine.Line)
		subNames := re.SubexpNames()
		previousPriority := ""
		newPriority := ""
		for i, name := range subNames {
			switch name {
			case "PREV_PRIO":
				previousPriority = subMatches[i]
			case "CURR_PRIO":
				newPriority = subMatches[i]
			}
		}
		g.recorder.AddIntervals(
			monitorapi.NewInterval(monitorapi.SourcePodLog, monitorapi.Info).
				Locator(logLine.Locator).
				Message(monitorapi.NewMessage().
					Reason(monitorapi.OnPremLBPriorityChange).
					WithAnnotation(monitorapi.AnnotationPreviousPriority, previousPriority).
					WithAnnotation(monitorapi.AnnotationPriority, newPriority).
					HumanMessage(logLine.Line),
				).
				Build(logLine.Instant, logLine.Instant),
		)
	}

}

func (*operatorLogAnalyzer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	leaseIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		if eventInterval.Message.Reason == monitorapi.OnPremLBPriorityChange {
			return true
		}
		return false
	})
	testName := fmt.Sprintf("[Jira:\"Networking / On-Prem Load Balancer\"] on-prem loadbalancer must achieve full priority")

	neededPriority := "65"
	achievedPriority := false
	for _, interval := range leaseIntervals {
		if interval.Message.Annotations[monitorapi.AnnotationPriority] == neededPriority {
			achievedPriority = true
		}
	}

	ret := []*junitapi.JUnitTestCase{}
	if achievedPriority {
		ret = append(ret, &junitapi.JUnitTestCase{
			Name: testName,
		})
	} else {
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Message: fmt.Sprintf("no master achieved priority %s", neededPriority),
					Output:  fmt.Sprintf("no master achieved priority %s", neededPriority),
				},
			},
		)
	}
	// Force the test to flake even if it failed
	ret = append(ret, &junitapi.JUnitTestCase{
		Name: testName,
	})

	return ret, nil
}

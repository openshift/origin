package onpremhaproxy

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

func (w *operatorLogAnalyzer) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *operatorLogAnalyzer) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	// move to prepare?
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
	infraPods := []corev1.Pod{}
	onPremPlatforms := []string{"kni", "openstack", "vsphere"}

	for _, platform := range onPremPlatforms {
		pods, err := kubeClient.CoreV1().
			Pods(fmt.Sprintf("openshift-%s-infra", platform)).
			List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("app=%s-infra-api-lb", platform)})
		if err != nil {
			return fmt.Errorf("couldn't list pods: %w", err)
		}

		for _, pod := range pods.Items {
			// this is just a basic check to see if we can expect logs to be present. Unready, unhealthy, and failed pods all still have logs.
			if pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodUnknown {
				continue
			}

			infraPods = append(infraPods, pod)
		}
	}

	errs := []error{}
	for _, pod := range infraPods {
		for _, container := range pod.Spec.Containers {
			// We have "haproxy", "haproxy-monitor" and some other. Logs only from the first one are interesting.
			if container.Name != "haproxy" {
				continue
			}
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
	constructedIntervals := monitorapi.Intervals{}

	allHaproxyChanges := startingIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		if eventInterval.Message.Reason == monitorapi.OnPremHaproxyStatusChange {
			return true
		}
		return false
	})

	// for every node we draw how it sees all the other nodes, so NxN array, 3x3 for 3-node cluster
	haproxyChanges := map[string][]monitorapi.Interval{}
	for _, change := range allHaproxyChanges {
		myKey := fmt.Sprintf("%s___%s", change.Locator.Keys[monitorapi.LocatorNodeKey], change.Message.Annotations[monitorapi.AnnotationNode])
		haproxyChanges[myKey] = append(haproxyChanges[myKey], change)
	}

	for key, allChangesForTheNode := range haproxyChanges {
		previousChangeTime := time.Time{}
		createdIntervals := monitorapi.Intervals(allChangesForTheNode).Filter(func(eventInterval monitorapi.Interval) bool {
			return eventInterval.Message.Reason == monitorapi.OnPremHaproxyStatusChange
		})
		if len(createdIntervals) > 0 {
			previousChangeTime = createdIntervals[0].From
		}
		lastAvailableStatus := ""

		for _, availableConditionChange := range allChangesForTheNode {
			currentStatus := availableConditionChange.Message.Annotations[monitorapi.AnnotationStatus]
			if currentStatus == lastAvailableStatus {
				continue
			}

			if currentStatus == "UP" && lastAvailableStatus == "DOWN" {
				constructedIntervals = append(constructedIntervals,
					monitorapi.NewInterval(monitorapi.SourceHaproxyMonitor, monitorapi.Info).
						//Locator(availableConditionChange.Locator).
						Locator(monitorapi.Locator{Keys: map[monitorapi.LocatorKey]string{
							monitorapi.LocatorOnPremKubeapiUnreachableFromHaproxyKey: key,
						}}).
						Message(monitorapi.NewMessage().Reason(monitorapi.OnPremHaproxyDetectsDown).
							Constructed(monitorapi.ConstructionOwnerOnPremHaproxy).
							HumanMessage(fmt.Sprintf("Kubeapi on %s is detected dead by %s", availableConditionChange.Message.Annotations[monitorapi.AnnotationNode], availableConditionChange.Locator.Keys["node"]))).
						Display().
						Build(previousChangeTime, availableConditionChange.From),
				)
			}

			previousChangeTime = availableConditionChange.From
			lastAvailableStatus = availableConditionChange.Message.Annotations[monitorapi.AnnotationStatus]
		}

		//deletionTime := time.Now()
		//deletedIntervals := monitorapi.Intervals(allChangesForTheNode).Filter(func(eventInterval monitorapi.Interval) bool {
		//	return eventInterval.Message.Reason == monitorapi.APIServiceDeletedInAPI
		//})
		//if len(deletedIntervals) > 0 {
		//	deletionTime = deletedIntervals[0].To
		//}
		//if len(lastAvailableStatus) > 0 {
		//	reason := monitorapi.APIServiceUnavailable
		//	intervalLevel := monitorapi.Error
		//	if lastAvailableStatus == "True" {
		//		reason = monitorapi.APIServiceAvailable
		//		intervalLevel = monitorapi.Info
		//	} else if lastAvailableStatus != "False" {
		//		reason = monitorapi.APIServiceUnknown
		//		intervalLevel = monitorapi.Warning
		//	}
		//	constructedIntervals = append(constructedIntervals,
		//		monitorapi.NewInterval(monitorapi.SourceAPIServiceMonitor, intervalLevel).
		//			Locator(apiserviceLocator).
		//			Message(monitorapi.NewMessage().Reason(reason).
		//				Constructed(monitorapi.ConstructionOwnerAPIServiceLifecycle).
		//				HumanMessage(previousHumanMesage)).
		//			Display().
		//			Build(previousChangeTime, deletionTime),
		//	)
		//}
	}

	return constructedIntervals, nil
}

func (*operatorLogAnalyzer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	leaseIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		if eventInterval.Message.Reason == monitorapi.OnPremHaproxyDetectsDown {
			return true
		}
		return false
	})

	testName := fmt.Sprint("[Jira: Networking / On-Prem Host Networking] Haproxy must be able to reach kubeapi server")
	success := &junitapi.JUnitTestCase{Name: testName}
	somethingFailed := false

	testNameToFailures := map[string][]string{}
	for _, interval := range leaseIntervals {
		if interval.Message.Reason == monitorapi.OnPremHaproxyDetectsDown {
			//CHOCOBOMB(mko) Status change itself is NOT an indication of failure. It's here only for development
			somethingFailed = true
		}

		intervalDuration := interval.To.Sub(interval.From)
		if intervalDuration < 10*time.Second {
			_, ok := testNameToFailures[testName]
			if !ok {
				testNameToFailures[testName] = []string{}
			}
			continue
		}

		testNameToFailures[testName] = append(testNameToFailures[testName], interval.String())
	}

	if !somethingFailed {
		return []*junitapi.JUnitTestCase{success}, nil
	}

	failure := &junitapi.JUnitTestCase{
		Name: testName,
		FailureOutput: &junitapi.FailureOutput{
			//Message: fmt.Sprint("something happened with haproxy"),
			Output: "Haproxy detected some kubeapi-servers down. It's not necessarily an issue, it's expected over the course of installation. Go and check messages. Look at intervals in sippy to see a full graph of which haproxy instance detected which kubeapi-server as down. Plotted on a time axis, you will see if at any point in time all the kubeapi-servers were down. Only then, it is an issue.",
		},
		SystemOut: strings.Join(testNameToFailures[testName], "\n"),
		//SystemErr: fmt.Sprintf("syserr; found %d lines in the failure map", len(testNameToFailures[testName])),
	}

	// Marked flaky until we have monitored it for consistency
	return []*junitapi.JUnitTestCase{failure, success}, nil
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
	reDown := regexp.MustCompile("Server (?P<HOST>[a-z0-9-\\/]*) is DOWN")
	reUp := regexp.MustCompile("Server (?P<HOST>[a-z0-9-\\/]*) is UP")
	if g.afterTime != nil {
		if logLine.Instant.Before(*g.afterTime) {
			return
		}
	}
	switch {
	case reDown.MatchString(logLine.Line):
		detectedHost := reDown.FindStringSubmatch(logLine.Line)[reDown.SubexpIndex("HOST")]
		reportingHost := logLine.Locator.Keys[monitorapi.LocatorNodeKey]

		g.recorder.AddIntervals(
			monitorapi.NewInterval(monitorapi.SourcePodLog, monitorapi.Info).
				Locator(logLine.Locator).
				Message(monitorapi.NewMessage().
					Reason(monitorapi.OnPremHaproxyStatusChange).
					WithAnnotation(monitorapi.AnnotationStatus, "DOWN").
					WithAnnotation(monitorapi.AnnotationNode, detectedHost).
					HumanMessage(fmt.Sprintf("Kubeapi on %s unreachable from %s", detectedHost, reportingHost)),
				).
				Build(logLine.Instant, logLine.Instant),
		)
	case reUp.MatchString(logLine.Line):
		detectedHost := reUp.FindStringSubmatch(logLine.Line)[reUp.SubexpIndex("HOST")]
		reportingHost := logLine.Locator.Keys[monitorapi.LocatorNodeKey]

		g.recorder.AddIntervals(
			monitorapi.NewInterval(monitorapi.SourcePodLog, monitorapi.Info).
				Locator(logLine.Locator).
				Message(monitorapi.NewMessage().
					Reason(monitorapi.OnPremHaproxyStatusChange).
					WithAnnotation(monitorapi.AnnotationStatus, "UP").
					WithAnnotation(monitorapi.AnnotationNode, detectedHost).
					HumanMessage(fmt.Sprintf("Kubeapi on %s reachable from %s", detectedHost, reportingHost)),
				).
				Build(logLine.Instant, logLine.Instant),
		)
	}

}

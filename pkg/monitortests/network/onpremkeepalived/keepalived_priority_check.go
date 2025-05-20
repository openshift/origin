package onpremkeepalived

import (
	"context"
	"errors"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"regexp"
	"sort"
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
	return nil, nil, nil
}

func (*operatorLogAnalyzer) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	constructedIntervals := monitorapi.Intervals{}
	vipMoves := map[string][]monitorapi.Interval{}
	for _, interval := range startingIntervals {
		// Collect messages about VIP movements. Ignore any where both the
		// start and end fall before or after the time range. Include any that
		// partially overlap the time range. We'll adjust them to fit later.
		if interval.Message.Reason == monitorapi.OnPremLBTookVIP || interval.Message.Reason == monitorapi.OnPremLBLostVIP && (!(interval.To.Before(beginning) && interval.From.Before(beginning)) || !(interval.To.After(end) && interval.From.After(end))) {
			nodeName := fmt.Sprintf("%s_%s", interval.Locator.Keys[monitorapi.LocatorNodeKey], interval.Message.Annotations[monitorapi.AnnotationVIP])
			vipMoves[nodeName] = append(vipMoves[nodeName], interval)
		}
	}
	for nodeName, nodeMoves := range vipMoves {
		tookVip := false
		// Working directly on constructedIntervals can result in some difficult to debug issues. Using a node-specific list avoids any bleedover.
		localIntervals := monitorapi.Intervals{}
		// Ensure that events are always sorted so Took comes before Lost with equal timestamps.
		sort.Slice(nodeMoves, func(i, j int) bool {
			return nodeMoves[j].From.After(nodeMoves[i].From) || (nodeMoves[i].From.Equal(nodeMoves[j].From) && nodeMoves[i].Message.Reason == monitorapi.OnPremLBTookVIP)
		})
		for _, move := range nodeMoves {
			if move.Message.Reason == monitorapi.OnPremLBTookVIP {
				tookVip = true
				// Create an interval to the end time. If we lose the VIP we'll shorten it later.
				locator := monitorapi.Locator{Keys: map[monitorapi.LocatorKey]string{monitorapi.LocatorOnPremVIPMonitorKey: nodeName}}
				message := monitorapi.NewMessage().Reason(monitorapi.OnPremLBTookVIP).
					Constructed(monitorapi.ConstructionOwnerOnPremKeepalived).
					HumanMessage(fmt.Sprintf("Node %s took the VIP", nodeName))
				localIntervals = append(localIntervals,
					monitorapi.NewInterval(monitorapi.SourceKeepalivedMonitor, monitorapi.Info).
						Locator(locator).
						Message(message).
						Display().
						Build(move.From, end),
				)
			} else if move.Message.Reason == monitorapi.OnPremLBLostVIP {
				// Ignore any Lost messages if we didn't previously have a Took message. This is normal behavior at startup since keepalived starts in the backup state.
				if tookVip {
					localIntervals[localIntervals.Len()-1].To = move.From
				}
				tookVip = false
			}
		}
		constructedIntervals = append(constructedIntervals, localIntervals...)
	}
	// Clamp all intervals to the selected range so we don't blow out the timeline
	for _, interval := range constructedIntervals {
		if interval.From.Before(beginning) {
			interval.From = beginning
		}
		if interval.To.After(end) {
			interval.To = end
		}
	}
	return constructedIntervals, nil
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
	priorityRe := regexp.MustCompile("effective priority from (?P<PREV_PRIO>[\\d]+) to (?P<CURR_PRIO>[\\d]+)")
	masterRe := regexp.MustCompile("Entering MASTER STATE")
	backupRe := regexp.MustCompile("Entering BACKUP STATE")
	api1Re := regexp.MustCompile("API_1")
	ingress0Re := regexp.MustCompile("INGRESS_0")
	ingress1Re := regexp.MustCompile("INGRESS_1")
	if g.afterTime != nil {
		if logLine.Instant.Before(*g.afterTime) {
			return
		}
	}
	vipType := "API_0"
	if api1Re.MatchString(logLine.Line) {
		vipType = "API_1"
	} else if ingress0Re.MatchString(logLine.Line) {
		vipType = "INGRESS_0"
	} else if ingress1Re.MatchString(logLine.Line) {
		vipType = "INGRESS_1"
	}
	switch {
	case priorityRe.MatchString(logLine.Line):
		subMatches := priorityRe.FindStringSubmatch(logLine.Line)
		subNames := priorityRe.SubexpNames()
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
			monitorapi.NewInterval(monitorapi.SourceKeepalivedMonitor, monitorapi.Info).
				Locator(logLine.Locator).
				Message(monitorapi.NewMessage().
					Reason(monitorapi.OnPremLBPriorityChange).
					WithAnnotation(monitorapi.AnnotationPreviousPriority, previousPriority).
					WithAnnotation(monitorapi.AnnotationPriority, newPriority).
					HumanMessage(logLine.Line),
				).
				Build(logLine.Instant, logLine.Instant),
		)
	case masterRe.MatchString(logLine.Line):
		g.recorder.AddIntervals(
			monitorapi.NewInterval(monitorapi.SourceKeepalivedMonitor, monitorapi.Info).
				Locator(logLine.Locator).
				Message(monitorapi.NewMessage().
					Reason(monitorapi.OnPremLBTookVIP).
					WithAnnotation(monitorapi.AnnotationVIP, vipType).
					HumanMessage(logLine.Line),
				).
				Build(logLine.Instant, logLine.Instant),
		)
	case backupRe.MatchString(logLine.Line):
		g.recorder.AddIntervals(
			monitorapi.NewInterval(monitorapi.SourceKeepalivedMonitor, monitorapi.Info).
				Locator(logLine.Locator).
				Message(monitorapi.NewMessage().
					Reason(monitorapi.OnPremLBLostVIP).
					WithAnnotation(monitorapi.AnnotationVIP, vipType).
					HumanMessage(logLine.Line),
				).
				Build(logLine.Instant, logLine.Instant),
		)
	}

}

func (*operatorLogAnalyzer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	priorityIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		if eventInterval.Message.Reason == monitorapi.OnPremLBPriorityChange {
			return true
		}
		return false
	})
	testName := fmt.Sprintf("[Jira:\"Networking / On-Prem Load Balancer\"] on-prem loadbalancer must achieve full priority")

	neededPriority := "65"
	ingressPriority := "80"
	achievedPriority := false
	ingressAchievedPriority := false
	// Note that we're relying on the fact that the require priorities are different from
	// API to ingress. If that ever changes for some reason we'll need to be more specific
	// about which log messages we look at for each.
	for _, interval := range priorityIntervals {
		if interval.Message.Annotations[monitorapi.AnnotationPriority] == neededPriority {
			achievedPriority = true
		}
		if interval.Message.Annotations[monitorapi.AnnotationPriority] == ingressPriority {
			ingressAchievedPriority = true
		}
	}

	ret := []*junitapi.JUnitTestCase{}
	if len(priorityIntervals) == 0 {
		// If we found no priority messages then keepalived wasn't running.
		// Either this is not an on-prem IPI job or something went catastrophically
		// wrong. In either case, we don't need this test.
		return ret, nil
	}
	if achievedPriority {
		ret = append(ret, &junitapi.JUnitTestCase{
			Name: testName,
		})
	} else {
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Message: fmt.Sprintf("no API achieved priority %s", neededPriority),
					Output:  fmt.Sprintf("no API achieved priority %s", neededPriority),
				},
			},
		)
	}
	if ingressAchievedPriority {
		ret = append(ret, &junitapi.JUnitTestCase{
			Name: testName,
		})
	} else {
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Message: fmt.Sprintf("no ingress achieved priority %s", ingressPriority),
					Output:  fmt.Sprintf("no ingress achieved priority %s", ingressPriority),
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

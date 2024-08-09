package nodeready

import (
	"context"
	"fmt"
	"github.com/openshift/origin/pkg/monitortestlibrary/watchresources"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type nodesShouldStayReady struct {
	adminRESTConfig *rest.Config
}

func NewNodesShouldStayReady() monitortestframework.MonitorTest {
	return &nodesShouldStayReady{}
}

func (w *nodesShouldStayReady) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	kubeClient, err := kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	listWatch := cache.NewListWatchFromClient(kubeClient.CoreV1().RESTClient(), "nodes", "", fields.Everything())
	customStore := watchresources.NewMonitoringStore(
		"pods",
		nil,
		[]watchresources.ObjUpdateFunc{w.watchNodeUpdateFunc},
		nil,
		recorder,
		recorder,
	)
	reflector := cache.NewReflector(listWatch, &corev1.Pod{}, customStore, 0)
	go reflector.Run(ctx.Done())

	return nil
}

func (w *nodesShouldStayReady) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (*nodesShouldStayReady) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	constructedIntervals := monitorapi.Intervals{}

	readyRelatedIntervals := startingIntervals.Filter(func(curr monitorapi.Interval) bool {
		if curr.Message.Reason == monitorapi.NodeUnexpectedNotReadyReason {
			return true
		}
		if curr.Message.Reason == monitorapi.NodeReadyReason {
			return true
		}
		return false
	})

	nodeNameToUnexpectedUnready := map[string]monitorapi.Interval{}
	for _, curr := range readyRelatedIntervals {
		key := curr.Locator.OldLocator()
		switch {
		case curr.Message.Reason == monitorapi.NodeUnexpectedNotReadyReason:
			nodeNameToUnexpectedUnready[key] = curr
		case curr.Message.Reason == monitorapi.NodeReadyReason:
			unexpectedUnreadyInterval, ok := nodeNameToUnexpectedUnready[key]
			if !ok {
				continue
			}
			constructedIntervals = append(constructedIntervals,
				monitorapi.NewInterval(unexpectedUnreadyInterval.Source, monitorapi.Error).
					Locator(unexpectedUnreadyInterval.Locator).
					Message(monitorapi.NewMessage().Reason(monitorapi.NodeUnexpectedNotReadyReason).
						WithAnnotations(unexpectedUnreadyInterval.Message.Annotations).
						Constructed(monitorapi.ConstructionOwnerNodeLifecycle).
						HumanMessage("unexpected node not ready")).
					Build(unexpectedUnreadyInterval.From, curr.From),
			)

		}
	}

	return constructedIntervals, nil

}

func (*nodesShouldStayReady) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	junits := []*junitapi.JUnitTestCase{}

	unexpectedlyUnreadyNodes := finalIntervals.Filter(func(curr monitorapi.Interval) bool {
		_, isConstructed := curr.Message.Annotations[monitorapi.AnnotationConstructed]
		if curr.Message.Reason == monitorapi.NodeUnexpectedNotReadyReason && isConstructed {
			return true
		}
		return false
	})

	if len(unexpectedlyUnreadyNodes) == 0 {
		junits = append(junits, &junitapi.JUnitTestCase{
			Name: `[Jira:"Node / Kubelet"] nodes should not go unready unexpectedly`,
		})
		return junits, nil
	}

	unreadyStrings := []string{}
	for _, interval := range unexpectedlyUnreadyNodes {
		unreadyStrings = append(unreadyStrings, interval.String())
	}

	summary := fmt.Sprintf("nodes went unexpectly unready %d times", len(unexpectedlyUnreadyNodes))
	junits = append(junits, &junitapi.JUnitTestCase{
		Name: `[Jira:"Node / Kubelet"] nodes should not go unready unexpectedly`,
		FailureOutput: &junitapi.FailureOutput{
			Message: summary,
			Output:  strings.Join(unreadyStrings, "\n"),
		},
		SystemOut: summary,
		SystemErr: summary,
	})
	return junits, nil
}

func (*nodesShouldStayReady) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*nodesShouldStayReady) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}

func (w *nodesShouldStayReady) nodeUpdated(node, oldNode *corev1.Node) []monitorapi.Interval {
	if oldNode == nil {
		return nil
	}

	now := time.Now()
	intervals := []monitorapi.Interval{}
	roles := watchresources.NodeRoles(node)

	newReady := false
	if c := watchresources.FindNodeCondition(node.Status.Conditions, corev1.NodeReady, 0); c != nil {
		newReady = c.Status == corev1.ConditionTrue
	}
	oldReady := false
	if c := watchresources.FindNodeCondition(oldNode.Status.Conditions, corev1.NodeReady, 0); c != nil {
		oldReady = c.Status == corev1.ConditionTrue
	}

	newCurrentConfig := node.Annotations["machineconfiguration.openshift.io/currentConfig"]
	newDesiredConfig := node.Annotations["machineconfiguration.openshift.io/desiredConfig"]
	machineConfigChanged := newCurrentConfig != newDesiredConfig

	if !newReady && oldReady && !machineConfigChanged {
		intervals = append(intervals,
			monitorapi.NewInterval(monitorapi.SourceNodeMonitor, monitorapi.Error).
				Locator(monitorapi.NewLocator().NodeFromName(node.Name)).
				Message(monitorapi.NewMessage().Reason(monitorapi.NodeUnexpectedNotReadyReason).
					WithAnnotations(map[monitorapi.AnnotationKey]string{
						monitorapi.AnnotationRoles: roles,
					}).
					HumanMessage("unexpected node not ready")).
				Build(now, now))
	}

	return nil
}

func (w *nodesShouldStayReady) watchNodeUpdateFunc(obj, oldObj interface{}) []monitorapi.Interval {
	return w.nodeUpdated(obj.(*corev1.Node), oldObj.(*corev1.Node))
}

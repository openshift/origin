package nodeready

import (
	"context"
	"github.com/openshift/origin/pkg/monitortestlibrary/watchresources"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
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
		toUpdateFns(w.nodeChangeFns),
		nil,
		recorder,
		recorder,
	)
	reflector := cache.NewReflector(listWatch, &corev1.Pod{}, customStore, 0)
	go reflector.Run(ctx.Done())

	return nil
}

func (w *nodesShouldStayReady) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	kubeClient, err := kubernetes.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return nil, nil, err
	}
	// MicroShift does not have a proper journal for the node logs api.
	isMicroShift, err := exutil.IsMicroShiftCluster(kubeClient)
	if err != nil {
		return nil, nil, err
	}
	if isMicroShift {
		return nil, nil, nil
	}

	intervals, err := intervalsFromNodeLogs(ctx, kubeClient, beginning, end)
	return intervals, nil, err
}

func (*nodesShouldStayReady) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*nodesShouldStayReady) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (*nodesShouldStayReady) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*nodesShouldStayReady) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}

func toUpdateFns(podUpdateFns []func(pod, oldPod *corev1.Node) []monitorapi.Interval) []watchresources.ObjUpdateFunc {
	ret := []watchresources.ObjUpdateFunc{}

	for i := range podUpdateFns {
		fn := podUpdateFns[i]
		ret = append(ret, func(obj, oldObj interface{}) []monitorapi.Interval {
			if oldObj == nil {
				return fn(obj.(*corev1.Node), nil)
			}
			return fn(obj.(*corev1.Node), oldObj.(*corev1.Node))
		})
	}

	return ret
}

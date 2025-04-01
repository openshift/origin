package watchnamespaces

import (
	"context"
	"fmt"
	"k8s.io/client-go/kubernetes"
	"sync"
	"sync/atomic"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
)

var (
	allNamespaceLock      sync.RWMutex
	afterCollectData      atomic.Bool
	allPlatformNamespaces = sets.Set[string]{}
)

func GetAllPlatformNamespaces() ([]string, error) {
	if !afterCollectData.Load() {
		return nil, fmt.Errorf("namespace information is only available after CollectData step is over")
	}

	allNamespaceLock.RLock()
	defer allNamespaceLock.RUnlock()
	return sets.List(allPlatformNamespaces), nil
}

type namespaceTracker struct {
	collectionContextCancel context.CancelFunc
}

func NewNamespaceWatcher() monitortestframework.MonitorTest {
	return &namespaceTracker{}
}

func (w *namespaceTracker) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *namespaceTracker) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	kubeClient, err := kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	collectionCtx, collectionCtxCancel := context.WithCancel(ctx)
	w.collectionContextCancel = collectionCtxCancel
	startNamespaceMonitoring(collectionCtx, recorder, kubeClient)

	return nil
}

func (w *namespaceTracker) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	w.collectionContextCancel()

	allNamespaceLock.Lock()
	defer allNamespaceLock.Unlock()
	for nsName := range allObservedPlatformNamespaces {
		allPlatformNamespaces.Insert(nsName)
	}
	afterCollectData.Store(true)

	// because we are sharing a recorder that we're streaming into, we don't need to have a separate data collection step.
	return nil, nil, nil
}

func (*namespaceTracker) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*namespaceTracker) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (*namespaceTracker) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*namespaceTracker) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}

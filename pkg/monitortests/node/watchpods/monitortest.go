package watchpods

import (
	"context"
	"errors"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type podWatcher struct {
	kubeClient  kubernetes.Interface
	podInformer coreinformers.PodInformer

	blockDeltas          sync.Mutex
	collectionCtxCancel  context.CancelFunc
	collectionPodLister  listerscorev1.PodLister
	collectionIndexer    cache.Indexer
	collectionController cache.Controller
}

func NewPodWatcher() monitortestframework.MonitorTest {
	return &podWatcher{}
}

func (w *podWatcher) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	var err error
	w.kubeClient, err = kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	w.podInformer = startPodMonitoring(ctx, recorder, w.kubeClient)

	collectionCtx, collectionCtxCancel := context.WithCancel(ctx)
	w.collectionCtxCancel = collectionCtxCancel
	// copy/pasted from the construction of sharedinformers
	w.collectionIndexer = cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, cache.Indexers{})
	fifo := cache.NewDeltaFIFOWithOptions(cache.DeltaFIFOOptions{
		KnownObjects:          w.collectionIndexer,
		EmitDeltaTypeReplaced: true,
	})
	listWatch := cache.NewListWatchFromClient(w.kubeClient.CoreV1().RESTClient(), "pods", "", fields.Everything())

	cfg := &cache.Config{
		Queue:             fifo,
		ListerWatcher:     listWatch,
		ObjectType:        &corev1.Pod{},
		ObjectDescription: "pods",
		FullResyncPeriod:  0,
		RetryOnError:      false,
		ShouldResync:      func() bool { return false },

		Process:           w.handleDeltas,
		WatchErrorHandler: func(r *cache.Reflector, err error) {},
	}
	w.collectionController = cache.New(cfg)
	go w.collectionController.Run(collectionCtx.Done())

	w.collectionPodLister = listerscorev1.NewPodLister(w.collectionIndexer)

	return nil
}

// copied from the shared_informer in upstream.  It's a simplified version for the one collectionIndexer that we have.
func (w *podWatcher) handleDeltas(obj interface{}, isInInitialList bool) error {
	w.blockDeltas.Lock()
	defer w.blockDeltas.Unlock()

	if deltas, ok := obj.(cache.Deltas); ok {
		// from oldest to newest
		for _, d := range deltas {
			obj := d.Object

			switch d.Type {
			case cache.Sync, cache.Replaced, cache.Added, cache.Updated:
				if _, exists, err := w.collectionIndexer.Get(obj); err == nil && exists {
					if err := w.collectionIndexer.Update(obj); err != nil {
						return err
					}
				} else {
					if err := w.collectionIndexer.Add(obj); err != nil {
						return err
					}
				}
			case cache.Deleted:
				if err := w.collectionIndexer.Delete(obj); err != nil {
					return err
				}
			}
		}
	}
	return errors.New("object given as Process argument is not Deltas")
}

func (w *podWatcher) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	// because we are sharing a recorder that we're streaming into, we don't need to have a separate data collection step.
	w.collectionCtxCancel()

	return nil, nil, nil
}

func (*podWatcher) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	constructedIntervals := monitorapi.Intervals{}
	constructedIntervals = append(constructedIntervals, createPodIntervalsFromInstants(startingIntervals, recordedResources, beginning, end)...)
	constructedIntervals = append(constructedIntervals, intervalsFromEvents_PodChanges(startingIntervals, beginning, end)...)

	return constructedIntervals, nil
}

func (w *podWatcher) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	if w.podInformer == nil {
		return nil, nil
	}
	ret := []*junitapi.JUnitTestCase{}

	cacheFailures, err := checkCacheState(ctx, w.kubeClient, w.podInformer)
	if err != nil {
		return nil, fmt.Errorf("error determining cache failures: %w", err)
	}

	testName := "[sig-apimachinery] informers must match live results at the same resource version"
	if len(cacheFailures) > 0 {
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Message: strings.Join(cacheFailures, "\n"),
					Output:  "abandon all hope",
				},
			},
		)
		// start as a flake
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
			},
		)
	} else {
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
			},
		)
	}

	actualResourceVersion, err := strconv.Atoi(w.collectionController.LastSyncResourceVersion())
	if err != nil {
		return nil, fmt.Errorf("error getting last RV %v: %w", w.collectionController.LastSyncResourceVersion(), err)
	}
	actualCachedPods, err := w.collectionPodLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("error listing cached pods: %w", err)
	}

	reflectorCachedFailures, err := checkCacheStateFromList(ctx, w.kubeClient, actualCachedPods, actualResourceVersion)
	if err != nil {
		return nil, fmt.Errorf("error determining cache failures: %w", err)
	}
	reflectorCachedTestName := "[sig-apimachinery] reflector must match live results at the same resource version"
	if len(reflectorCachedFailures) > 0 {
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: reflectorCachedTestName,
				FailureOutput: &junitapi.FailureOutput{
					Message: strings.Join(reflectorCachedFailures, "\n"),
					Output:  "abandon all hope",
				},
			},
		)
		// start as a flake
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: reflectorCachedTestName,
			},
		)
	} else {
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: reflectorCachedTestName,
			},
		)
	}

	return ret, nil
}

func (*podWatcher) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*podWatcher) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}

package watchpods

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	informercorev1 "k8s.io/client-go/informers/core/v1"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Recorder interface {
	Record(conditions ...monitorapi.Condition)
}

type SimultaneousPodIPController struct {
	recorder Recorder

	podIPsToCurrentPodLocators map[string]sets.String
	podLister                  listerscorev1.PodLister

	cachesToSync []cache.InformerSynced

	queue workqueue.RateLimitingInterface
}

func NewSimultaneousPodIPController(
	recorder Recorder,
	podInformer informercorev1.PodInformer,
) *SimultaneousPodIPController {
	c := &SimultaneousPodIPController{
		recorder:                   recorder,
		podIPsToCurrentPodLocators: map[string]sets.String{},
		podLister:                  podInformer.Lister(),
		cachesToSync:               []cache.InformerSynced{podInformer.Informer().HasSynced},
		queue:                      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "SimultaneousPodIPController"),
	}

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.Enqueue,
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod, oldOk := oldObj.(*corev1.Pod)
			newPod, newOk := newObj.(*corev1.Pod)
			if oldOk && newOk {
				// if the podIPs have not changed, then we don't need to requeue because we will have queued when the change happened.
				// if another pod is created that conflicts, we'll have a separate notification for that pod.
				if reflect.DeepEqual(oldPod.Status.PodIPs, newPod.Status.PodIPs) {
					return
				}
			}
			c.Enqueue(newObj)
		},
		DeleteFunc: c.Enqueue,
	})

	return c
}

func (c *SimultaneousPodIPController) Enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid queue key '%v': %v", obj, err))
		return
	}
	c.queue.Add(key)
}

func (c *SimultaneousPodIPController) Run(ctx context.Context) {
	defer utilruntime.HandleCrash()

	fmt.Printf("Starting SimultaneousPodIPController\n")
	defer func() {
		fmt.Printf("Shutting down SimultaneousPodIPController\n")
		c.queue.ShutDown()
		fmt.Printf("SimultaneousPodIPController shut down\n")
	}()

	if !cache.WaitForNamedCacheSync("SimultaneousPodIPController", ctx.Done(), c.cachesToSync...) {
		return
	}

	go func() {
		// this only works because we have a single thread consuming
		wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}()

	<-ctx.Done()
}

func (c *SimultaneousPodIPController) runWorker(ctx context.Context) {
	for c.processNextItem(ctx) {
	}
}

func (c *SimultaneousPodIPController) processNextItem(ctx context.Context) bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.sync(ctx, key.(string))

	if err == nil {
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with : %w", key, err))
	c.queue.AddRateLimited(key)

	return true
}

func (c *SimultaneousPodIPController) sync(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	pod, err := c.podLister.Pods(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	// only consider pod network pods because host network pods will have duplicated IPs
	if pod.Spec.HostNetwork {
		return nil
	}
	if isPodIPReleased(pod) {
		return nil
	}

	otherPods, err := c.podLister.List(labels.Everything())
	if err != nil {
		return err
	}

	podLocator := monitorapi.LocatePod(pod)
	for _, currIP := range pod.Status.PodIPs {
		currPodIP := currIP.IP
		podNames := sets.NewString()

		// iterate through every ip of every pod.  I wonder how badly this will scale.
		// with the filtering for pod updates to only include those that changed podIPs, it will likely be fine.
		for _, otherPod := range otherPods {
			if otherPod.Spec.HostNetwork {
				continue
			}
			if isPodIPReleased(otherPod) {
				continue
			}

			for _, otherIP := range otherPod.Status.PodIPs {
				otherPodIP := otherIP.IP
				if currPodIP == otherPodIP {
					otherPodLocator := monitorapi.LocatePod(otherPod)
					podNames.Insert(otherPodLocator)
				}
			}
		}

		if len(podNames) > 1 {
			// the .Record function adds a timestamp of now to the condition so we track time.
			c.recorder.Record(monitorapi.Condition{
				Level:   monitorapi.Error,
				Locator: podLocator,
				Message: monitorapi.NewMessage().Reason(monitorapi.PodIPReused).
					HumanMessagef("podIP %v is currently assigned to multiple pods: %v", currPodIP, strings.Join(podNames.List(), ";")).BuildString(),
			})
		}
	}

	return nil
}

// isPodIPReleased returns true if the podIP can be reused.
// This happens on pod deletion and when the pod will not start any more containers
func isPodIPReleased(pod *corev1.Pod) bool {
	if pod.DeletionTimestamp != nil {
		return true
	}

	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return true
	}

	return false
}

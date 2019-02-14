package staticpodstate

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	operatorv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

var (
	staticPodStateControllerFailing      = "StaticPodsFailing"
	staticPodStateControllerWorkQueueKey = "key"
)

// StaticPodStateController is a controller that watches static pods and will produce a failing status if the
//// static pods start crashing for some reason.
type StaticPodStateController struct {
	targetNamespace, staticPodName string

	operatorConfigClient v1helpers.StaticPodOperatorClient
	podsGetter           corev1client.PodsGetter
	eventRecorder        events.Recorder

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface
}

// NewStaticPodStateController creates a controller that watches static pods and will produce a failing status if the
// static pods start crashing for some reason.
func NewStaticPodStateController(
	targetNamespace, staticPodName string,
	kubeInformersForTargetNamespace informers.SharedInformerFactory,
	operatorConfigClient v1helpers.StaticPodOperatorClient,
	podsGetter corev1client.PodsGetter,
	eventRecorder events.Recorder,
) *StaticPodStateController {
	c := &StaticPodStateController{
		targetNamespace: targetNamespace,
		staticPodName:   staticPodName,

		operatorConfigClient: operatorConfigClient,
		podsGetter:           podsGetter,
		eventRecorder:        eventRecorder,

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "StaticPodStateController"),
	}

	operatorConfigClient.Informer().AddEventHandler(c.eventHandler())
	kubeInformersForTargetNamespace.Core().V1().Pods().Informer().AddEventHandler(c.eventHandler())

	return c
}

func (c *StaticPodStateController) sync() error {
	operatorSpec, originalOperatorStatus, _, err := c.operatorConfigClient.GetStaticPodOperatorState()
	if err != nil {
		return err
	}

	switch operatorSpec.ManagementState {
	case operatorv1.Unmanaged:
		return nil
	case operatorv1.Removed:
		// TODO probably just fail.  Static pod managers can't be removed.
		return nil
	}

	errs := []error{}
	for _, node := range originalOperatorStatus.NodeStatuses {
		pod, err := c.podsGetter.Pods(c.targetNamespace).Get(mirrorPodNameForNode(c.staticPodName, node.NodeName), metav1.GetOptions{})
		if err != nil {
			errs = append(errs, err)
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			if !containerStatus.Ready {
				errs = append(errs, fmt.Errorf("nodes/%s pods/%s container=%q is not ready", node.NodeName, pod.Name, containerStatus.Name))
			}
			if containerStatus.State.Waiting != nil {
				errs = append(errs, fmt.Errorf("nodes/%s pods/%s container=%q is waiting: %q - %q", node.NodeName, pod.Name, containerStatus.Name, containerStatus.State.Waiting.Reason, containerStatus.State.Waiting.Message))
			}
			if containerStatus.State.Terminated != nil {
				errs = append(errs, fmt.Errorf("nodes/%s pods/%s container=%q is terminated: %q - %q", node.NodeName, pod.Name, containerStatus.Name, containerStatus.State.Terminated.Reason, containerStatus.State.Terminated.Message))
			}
		}
	}

	// update failing condition
	cond := operatorv1.OperatorCondition{
		Type:   staticPodStateControllerFailing,
		Status: operatorv1.ConditionFalse,
	}
	if len(errs) > 0 {
		cond.Status = operatorv1.ConditionTrue
		cond.Reason = "Error"
		cond.Message = v1helpers.NewMultiLineAggregate(errs).Error()
	}
	if _, _, updateError := v1helpers.UpdateStaticPodStatus(c.operatorConfigClient, v1helpers.UpdateStaticPodConditionFn(cond), v1helpers.UpdateStaticPodConditionFn(cond)); updateError != nil {
		if err == nil {
			return updateError
		}
	}

	return err
}

func mirrorPodNameForNode(staticPodName, nodeName string) string {
	return staticPodName + "-" + nodeName
}

// Run starts the kube-apiserver and blocks until stopCh is closed.
func (c *StaticPodStateController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting StaticPodStateController")
	defer glog.Infof("Shutting down StaticPodStateController")

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *StaticPodStateController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *StaticPodStateController) processNextWorkItem() bool {
	dsKey, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(dsKey)

	err := c.sync()
	if err == nil {
		c.queue.Forget(dsKey)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", dsKey, err))
	c.queue.AddRateLimited(dsKey)

	return true
}

// eventHandler queues the operator to check spec and status
func (c *StaticPodStateController) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(staticPodStateControllerWorkQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(staticPodStateControllerWorkQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(staticPodStateControllerWorkQueueKey) },
	}
}

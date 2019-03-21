package staticpodstate

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/management"
	"github.com/openshift/library-go/pkg/operator/status"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

var (
	staticPodStateControllerFailing      = "StaticPodsFailing"
	staticPodStateControllerWorkQueueKey = "key"
)

// StaticPodStateController is a controller that watches static pods and will produce a failing status if the
//// static pods start crashing for some reason.
type StaticPodStateController struct {
	targetNamespace   string
	staticPodName     string
	operandName       string
	operatorNamespace string

	operatorConfigClient v1helpers.StaticPodOperatorClient
	configMapGetter      corev1client.ConfigMapsGetter
	podsGetter           corev1client.PodsGetter
	versionRecorder      status.VersionGetter
	eventRecorder        events.Recorder

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface
}

// NewStaticPodStateController creates a controller that watches static pods and will produce a failing status if the
// static pods start crashing for some reason.
func NewStaticPodStateController(
	targetNamespace, staticPodName, operatorNamespace, operandName string,
	kubeInformersForTargetNamespace informers.SharedInformerFactory,
	operatorConfigClient v1helpers.StaticPodOperatorClient,
	configMapGetter corev1client.ConfigMapsGetter,
	podsGetter corev1client.PodsGetter,
	versionRecorder status.VersionGetter,
	eventRecorder events.Recorder,
) *StaticPodStateController {
	c := &StaticPodStateController{
		targetNamespace:   targetNamespace,
		staticPodName:     staticPodName,
		operandName:       operandName,
		operatorNamespace: operatorNamespace,

		operatorConfigClient: operatorConfigClient,
		configMapGetter:      configMapGetter,
		podsGetter:           podsGetter,
		versionRecorder:      versionRecorder,
		eventRecorder:        eventRecorder.WithComponentSuffix("static-pod-state-controller"),

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

	if !management.IsOperatorManaged(operatorSpec.ManagementState) {
		return nil
	}

	errs := []error{}
	failingErrorCount := 0
	images := sets.NewString()
	for _, node := range originalOperatorStatus.NodeStatuses {
		pod, err := c.podsGetter.Pods(c.targetNamespace).Get(mirrorPodNameForNode(c.staticPodName, node.NodeName), metav1.GetOptions{})
		if err != nil {
			errs = append(errs, err)
			failingErrorCount++
			continue
		}
		images.Insert(pod.Spec.Containers[0].Image)

		for _, containerStatus := range pod.Status.ContainerStatuses {
			if !containerStatus.Ready {
				// When container is not ready, we can't determine whether the operator is failing or not and every container will become not
				// ready when created, so do not blip the failing state for it.
				// We will still reflect the container not ready state in error conditions, but we don't set the operator as failed.
				errs = append(errs, fmt.Errorf("nodes/%s pods/%s container=%q is not ready", node.NodeName, pod.Name, containerStatus.Name))
			}
			if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason != "PodInitializing" {
				errs = append(errs, fmt.Errorf("nodes/%s pods/%s container=%q is waiting: %q - %q", node.NodeName, pod.Name, containerStatus.Name, containerStatus.State.Waiting.Reason, containerStatus.State.Waiting.Message))
				failingErrorCount++
			}
			if containerStatus.State.Terminated != nil {
				// Containers can be terminated gracefully to trigger certificate reload, do not report these as failures.
				errs = append(errs, fmt.Errorf("nodes/%s pods/%s container=%q is terminated: %q - %q", node.NodeName, pod.Name, containerStatus.Name, containerStatus.State.Terminated.Reason, containerStatus.State.Terminated.Message))
				// Only in case when the termination was caused by error.
				if containerStatus.State.Terminated.ExitCode != 0 {
					failingErrorCount++
				}

			}
		}
	}

	if len(images) == 0 {
		c.eventRecorder.Warningf("MissingVersion", "no image found for operand pod")
	} else if len(images) > 1 {
		c.eventRecorder.Eventf("MultipleVersions", "multiple versions found, probably in transition: %v", strings.Join(images.List(), ","))
	} else {
		c.versionRecorder.SetVersion(
			c.operandName,
			status.VersionForOperand(c.operatorNamespace, images.List()[0], c.configMapGetter, c.eventRecorder),
		)
	}

	// update failing condition
	cond := operatorv1.OperatorCondition{
		Type:   staticPodStateControllerFailing,
		Status: operatorv1.ConditionFalse,
	}
	// Failing errors
	if failingErrorCount > 0 {
		cond.Status = operatorv1.ConditionTrue
		cond.Reason = "Error"
		cond.Message = v1helpers.NewMultiLineAggregate(errs).Error()
	}
	// Not failing errors
	if failingErrorCount == 0 && len(errs) > 0 {
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

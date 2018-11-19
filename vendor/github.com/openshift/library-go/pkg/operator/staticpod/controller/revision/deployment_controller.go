package revision

import (
	"fmt"
	"reflect"
	"time"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/common"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

const revisionControllerWorkQueueKey = "key"

type RevisionController struct {
	targetNamespace string
	// configMaps is the list of configmaps that are directly copied.A different actor/controller modifies these.
	// the first element should be the configmap that contains the static pod manifest
	configMaps []string
	// secrets is a list of secrets that are directly copied for the current values.  A different actor/controller modifies these.
	secrets []string

	operatorConfigClient common.OperatorClient

	kubeClient kubernetes.Interface

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface
}

func NewRevisionController(
	targetNamespace string,
	configMaps []string,
	secrets []string,
	kubeInformersForTargetNamespace informers.SharedInformerFactory,
	operatorConfigClient common.OperatorClient,
	kubeClient kubernetes.Interface,
) *RevisionController {
	c := &RevisionController{
		targetNamespace: targetNamespace,
		configMaps:      configMaps,
		secrets:         secrets,

		operatorConfigClient: operatorConfigClient,
		kubeClient:           kubeClient,

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "RevisionController"),
	}

	operatorConfigClient.Informer().AddEventHandler(c.eventHandler())
	kubeInformersForTargetNamespace.Core().V1().ConfigMaps().Informer().AddEventHandler(c.eventHandler())
	kubeInformersForTargetNamespace.Core().V1().Secrets().Informer().AddEventHandler(c.eventHandler())

	return c
}

// createRevisionIfNeeded takes care of creating content for the static pods to use.
// returns whether or not requeue and if an error happened when updating status.  Normally it updates status itself.
func (c RevisionController) createRevisionIfNeeded(operatorSpec *operatorv1.OperatorSpec, operatorStatusOriginal *operatorv1.StaticPodOperatorStatus, resourceVersion string) (bool, error) {
	operatorStatus := operatorStatusOriginal.DeepCopy()

	latestRevision := operatorStatus.LatestAvailableRevision
	isLatestRevisionCurrent, reason := c.isLatestRevisionCurrent(latestRevision)

	// check to make sure that the latestRevision has the exact content we expect.  No mutation here, so we start creating the next Revision only when it is required
	if isLatestRevisionCurrent {
		return false, nil
	}

	nextRevision := latestRevision + 1
	glog.Infof("new revision %d triggered by %q", nextRevision, reason)
	if err := c.createNewRevision(nextRevision); err != nil {
		v1helpers.SetOperatorCondition(&operatorStatus.Conditions, operatorv1.OperatorCondition{
			Type:    "RevisionControllerFailing",
			Status:  operatorv1.ConditionTrue,
			Reason:  "ContentCreationError",
			Message: err.Error(),
		})
		if !reflect.DeepEqual(operatorStatusOriginal, operatorStatus) {
			_, updateError := c.operatorConfigClient.UpdateStatus(resourceVersion, operatorStatus)
			return true, updateError
		}
		return true, nil
	}

	v1helpers.SetOperatorCondition(&operatorStatus.Conditions, operatorv1.OperatorCondition{
		Type:   "RevisionControllerFailing",
		Status: operatorv1.ConditionFalse,
	})
	operatorStatus.LatestAvailableRevision = nextRevision
	if !reflect.DeepEqual(operatorStatusOriginal, operatorStatus) {
		_, updateError := c.operatorConfigClient.UpdateStatus(resourceVersion, operatorStatus)
		if updateError != nil {
			return true, updateError
		}
	}

	return false, nil
}

func nameFor(name string, revision int32) string {
	return fmt.Sprintf("%s-%d", name, revision)
}

// isLatestRevisionCurrent returns whether the latest revision is up to date and an optional reason
func (c RevisionController) isLatestRevisionCurrent(revision int32) (bool, string) {
	for _, name := range c.configMaps {
		required, err := c.kubeClient.CoreV1().ConfigMaps(c.targetNamespace).Get(name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false, err.Error()
		}
		existing, err := c.kubeClient.CoreV1().ConfigMaps(c.targetNamespace).Get(nameFor(name, revision), metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false, err.Error()
		}
		if !equality.Semantic.DeepEqual(existing.Data, required.Data) {
			return false, fmt.Sprintf("configmap/%s has changed", required.Name)
		}
	}
	for _, name := range c.secrets {
		required, err := c.kubeClient.CoreV1().Secrets(c.targetNamespace).Get(name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false, err.Error()
		}
		existing, err := c.kubeClient.CoreV1().Secrets(c.targetNamespace).Get(nameFor(name, revision), metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false, err.Error()
		}
		if !equality.Semantic.DeepEqual(existing.Data, required.Data) {
			return false, fmt.Sprintf("secret/%s has changed", required.Name)
		}
	}

	return true, ""
}

func (c RevisionController) createNewRevision(revision int32) error {
	for _, name := range c.configMaps {
		obj, _, err := resourceapply.SyncConfigMap(c.kubeClient.CoreV1(), c.targetNamespace, name, c.targetNamespace, nameFor(name, revision))
		if err != nil {
			return err
		}
		if obj == nil {
			return apierrors.NewNotFound(corev1.Resource("configmaps"), name)
		}
	}
	for _, name := range c.secrets {
		obj, _, err := resourceapply.SyncSecret(c.kubeClient.CoreV1(), c.targetNamespace, name, c.targetNamespace, nameFor(name, revision))
		if err != nil {
			return err
		}
		if obj == nil {
			return apierrors.NewNotFound(corev1.Resource("secrets"), name)
		}
	}

	return nil
}

func (c RevisionController) sync() error {
	operatorSpec, originalOperatorStatus, resourceVersion, err := c.operatorConfigClient.Get()
	if err != nil {
		return err
	}
	operatorStatus := originalOperatorStatus.DeepCopy()

	switch operatorSpec.ManagementState {
	case operatorv1.Unmanaged:
		return nil
	case operatorv1.Removed:
		// TODO probably just fail.  Static pod managers can't be removed.
		return nil
	}

	requeue, syncErr := c.createRevisionIfNeeded(operatorSpec, operatorStatus, resourceVersion)
	if requeue && syncErr == nil {
		return fmt.Errorf("synthetic requeue request (err: %v)", syncErr)
	}
	err = syncErr

	if err != nil {
		v1helpers.SetOperatorCondition(&operatorStatus.Conditions, operatorv1.OperatorCondition{
			Type:    operatorv1.OperatorStatusTypeFailing,
			Status:  operatorv1.ConditionTrue,
			Reason:  "StatusUpdateError",
			Message: err.Error(),
		})
		if !reflect.DeepEqual(originalOperatorStatus, operatorStatus) {
			if _, updateError := c.operatorConfigClient.UpdateStatus(resourceVersion, operatorStatus); updateError != nil {
				glog.Error(updateError)
			}
		}
		return err
	}

	return nil
}

// Run starts the kube-apiserver and blocks until stopCh is closed.
func (c *RevisionController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting RevisionController")
	defer glog.Infof("Shutting down RevisionController")

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *RevisionController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *RevisionController) processNextWorkItem() bool {
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
func (c *RevisionController) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(revisionControllerWorkQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(revisionControllerWorkQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(revisionControllerWorkQueueKey) },
	}
}

package deployment

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

	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/v1alpha1helpers"
	"github.com/openshift/library-go/pkg/operator/v1alpha1staticpod/controller/common"
)

const deploymentControllerWorkQueueKey = "key"

type DeploymentController struct {
	targetNamespace string
	// configMaps is the list of configmaps that are directly copied.A different actor/controller modifies these.
	// the first element should be the configmap that contains the static pod manifest
	configMaps []string
	// secrets is a list of secrets that are directly copied for the current values.  A different actor/controller modifies these.
	secrets []string

	operatorConfigClient common.OperatorClient

	kubeClient    kubernetes.Interface
	eventRecorder events.Recorder

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface
}

func NewDeploymentController(
	targetNamespace string,
	configMaps []string,
	secrets []string,
	kubeInformersForTargetNamespace informers.SharedInformerFactory,
	operatorConfigClient common.OperatorClient,
	kubeClient kubernetes.Interface,
	eventRecorder events.Recorder,
) *DeploymentController {
	c := &DeploymentController{
		targetNamespace: targetNamespace,
		configMaps:      configMaps,
		secrets:         secrets,

		operatorConfigClient: operatorConfigClient,
		kubeClient:           kubeClient,
		eventRecorder:        eventRecorder,

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "DeploymentController"),
	}

	operatorConfigClient.Informer().AddEventHandler(c.eventHandler())
	kubeInformersForTargetNamespace.Core().V1().ConfigMaps().Informer().AddEventHandler(c.eventHandler())
	kubeInformersForTargetNamespace.Core().V1().Secrets().Informer().AddEventHandler(c.eventHandler())

	return c
}

// createDeploymentController takes care of creating content for the static pods to deploy.
// returns whether or not requeue and if an error happened when updating status.  Normally it updates status itself.
func (c DeploymentController) createDeploymentController(operatorSpec *operatorv1alpha1.OperatorSpec, operatorStatusOriginal *operatorv1alpha1.StaticPodOperatorStatus, resourceVersion string) (bool, error) {
	operatorStatus := operatorStatusOriginal.DeepCopy()

	latestDeploymentID := operatorStatus.LatestAvailableDeploymentGeneration
	isLatestDeploymentCurrent, reason := c.isLatestDeploymentCurrent(latestDeploymentID)

	// check to make sure that the latestDeploymentID has the exact content we expect.  No mutation here, so we start creating the next Deployment only when it is required
	if isLatestDeploymentCurrent {
		return false, nil
	}

	nextDeploymentID := latestDeploymentID + 1
	glog.Infof("new deployment %d triggered by %q", nextDeploymentID, reason)
	if err := c.createNewDeploymentController(nextDeploymentID); err != nil {
		v1alpha1helpers.SetOperatorCondition(&operatorStatus.Conditions, operatorv1alpha1.OperatorCondition{
			Type:    "DeploymentControllerFailing",
			Status:  operatorv1alpha1.ConditionTrue,
			Reason:  "ContentCreationError",
			Message: err.Error(),
		})
		if !reflect.DeepEqual(operatorStatusOriginal, operatorStatus) {
			_, updateError := c.operatorConfigClient.UpdateStatus(resourceVersion, operatorStatus)
			return true, updateError
		}
		return true, nil
	}

	v1alpha1helpers.SetOperatorCondition(&operatorStatus.Conditions, operatorv1alpha1.OperatorCondition{
		Type:   "DeploymentControllerFailing",
		Status: operatorv1alpha1.ConditionFalse,
	})
	operatorStatus.LatestAvailableDeploymentGeneration = nextDeploymentID
	if !reflect.DeepEqual(operatorStatusOriginal, operatorStatus) {
		_, updateError := c.operatorConfigClient.UpdateStatus(resourceVersion, operatorStatus)
		if updateError != nil {
			return true, updateError
		}
	}

	return false, nil
}

func nameFor(name string, deploymentID int32) string {
	return fmt.Sprintf("%s-%d", name, deploymentID)
}

// isLatestDeploymentCurrent returns whether the latest deployment is up to date and an optional reason
func (c DeploymentController) isLatestDeploymentCurrent(deploymentID int32) (bool, string) {
	for _, name := range c.configMaps {
		required, err := c.kubeClient.CoreV1().ConfigMaps(c.targetNamespace).Get(name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false, err.Error()
		}
		existing, err := c.kubeClient.CoreV1().ConfigMaps(c.targetNamespace).Get(nameFor(name, deploymentID), metav1.GetOptions{})
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
		existing, err := c.kubeClient.CoreV1().Secrets(c.targetNamespace).Get(nameFor(name, deploymentID), metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false, err.Error()
		}
		if !equality.Semantic.DeepEqual(existing.Data, required.Data) {
			return false, fmt.Sprintf("secret/%s has changed", required.Name)
		}
	}

	return true, ""
}

func (c DeploymentController) createNewDeploymentController(deploymentID int32) error {
	for _, name := range c.configMaps {
		obj, _, err := resourceapply.SyncConfigMap(c.kubeClient.CoreV1(), c.eventRecorder, c.targetNamespace, name, c.targetNamespace, nameFor(name,
			deploymentID))
		if err != nil {
			return err
		}
		if obj == nil {
			return apierrors.NewNotFound(corev1.Resource("configmaps"), name)
		}
	}
	for _, name := range c.secrets {
		obj, _, err := resourceapply.SyncSecret(c.kubeClient.CoreV1(), c.eventRecorder, c.targetNamespace, name, c.targetNamespace, nameFor(name, deploymentID))
		if err != nil {
			return err
		}
		if obj == nil {
			return apierrors.NewNotFound(corev1.Resource("secrets"), name)
		}
	}

	return nil
}

func (c DeploymentController) sync() error {
	operatorSpec, originalOperatorStatus, resourceVersion, err := c.operatorConfigClient.Get()
	if err != nil {
		return err
	}
	operatorStatus := originalOperatorStatus.DeepCopy()

	switch operatorSpec.ManagementState {
	case operatorv1alpha1.Unmanaged:
		return nil
	case operatorv1alpha1.Removed:
		// TODO probably just fail.  Static pod managers can't be removed.
		return nil
	}

	requeue, syncErr := c.createDeploymentController(operatorSpec, operatorStatus, resourceVersion)
	if requeue && syncErr == nil {
		return fmt.Errorf("synthetic requeue request (err: %v)", syncErr)
	}
	err = syncErr

	if err != nil {
		v1alpha1helpers.SetOperatorCondition(&operatorStatus.Conditions, operatorv1alpha1.OperatorCondition{
			Type:    operatorv1alpha1.OperatorStatusTypeFailing,
			Status:  operatorv1alpha1.ConditionTrue,
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
func (c *DeploymentController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting DeploymentController")
	defer glog.Infof("Shutting down DeploymentController")

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *DeploymentController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *DeploymentController) processNextWorkItem() bool {
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
func (c *DeploymentController) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(deploymentControllerWorkQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(deploymentControllerWorkQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(deploymentControllerWorkQueueKey) },
	}
}

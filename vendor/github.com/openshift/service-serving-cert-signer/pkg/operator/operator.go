package operator

import (
	"fmt"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	rbacclientv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	operatorsv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	scsclientv1alpha1 "github.com/openshift/client-go/servicecertsigner/clientset/versioned/typed/servicecertsigner/v1alpha1"
	scsinformerv1alpha1 "github.com/openshift/client-go/servicecertsigner/informers/externalversions/servicecertsigner/v1alpha1"
	"github.com/openshift/library-go/pkg/operator/v1alpha1helpers"
	"github.com/openshift/library-go/pkg/operator/versioning"
)

const (
	targetNamespaceName = "openshift-service-cert-signer"
	workQueueKey        = "key"
)

type ServiceCertSignerOperator struct {
	operatorConfigClient scsclientv1alpha1.ServiceCertSignerOperatorConfigsGetter

	appsv1Client appsclientv1.AppsV1Interface
	corev1Client coreclientv1.CoreV1Interface
	rbacv1Client rbacclientv1.RbacV1Interface

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface
}

func NewServiceCertSignerOperator(
	serviceCertSignerConfigInformer scsinformerv1alpha1.ServiceCertSignerOperatorConfigInformer,
	namespacedKubeInformers informers.SharedInformerFactory,
	operatorConfigClient scsclientv1alpha1.ServiceCertSignerOperatorConfigsGetter,
	appsv1Client appsclientv1.AppsV1Interface,
	corev1Client coreclientv1.CoreV1Interface,
	rbacv1Client rbacclientv1.RbacV1Interface,
) *ServiceCertSignerOperator {
	c := &ServiceCertSignerOperator{
		operatorConfigClient: operatorConfigClient,
		appsv1Client:         appsv1Client,
		corev1Client:         corev1Client,
		rbacv1Client:         rbacv1Client,

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ServiceCertSignerOperator"),
	}

	serviceCertSignerConfigInformer.Informer().AddEventHandler(c.eventHandler())
	namespacedKubeInformers.Core().V1().ConfigMaps().Informer().AddEventHandler(c.eventHandler())
	namespacedKubeInformers.Core().V1().ServiceAccounts().Informer().AddEventHandler(c.eventHandler())
	namespacedKubeInformers.Core().V1().Services().Informer().AddEventHandler(c.eventHandler())
	namespacedKubeInformers.Apps().V1().Deployments().Informer().AddEventHandler(c.eventHandler())

	// we only watch some namespaces
	namespacedKubeInformers.Core().V1().Namespaces().Informer().AddEventHandler(c.namespaceEventHandler())

	return c
}

func (c ServiceCertSignerOperator) sync() error {
	operatorConfig, err := c.operatorConfigClient.ServiceCertSignerOperatorConfigs().Get("instance", metav1.GetOptions{})
	if err != nil {
		return err
	}
	switch operatorConfig.Spec.ManagementState {
	case operatorsv1alpha1.Unmanaged:
		return nil

	case operatorsv1alpha1.Removed:
		// TODO probably need to watch until the NS is really gone
		if err := c.corev1Client.Namespaces().Delete(targetNamespaceName, nil); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		operatorConfig.Status.TaskSummary = "Remove"
		operatorConfig.Status.TargetAvailability = nil
		operatorConfig.Status.CurrentAvailability = nil
		operatorConfig.Status.Conditions = []operatorsv1alpha1.OperatorCondition{
			{
				Type:   operatorsv1alpha1.OperatorStatusTypeAvailable,
				Status: operatorsv1alpha1.ConditionFalse,
			},
		}
		if _, err := c.operatorConfigClient.ServiceCertSignerOperatorConfigs().Update(operatorConfig); err != nil {
			return err
		}
		return nil
	}

	var currentActualVerion *semver.Version

	if operatorConfig.Status.CurrentAvailability != nil {
		ver, err := semver.Parse(operatorConfig.Status.CurrentAvailability.Version)
		if err != nil {
			utilruntime.HandleError(err)
		} else {
			currentActualVerion = &ver
		}
	}
	desiredVersion, err := semver.Parse(operatorConfig.Spec.Version)
	if err != nil {
		// TODO report failing status, we may actually attempt to do this in the "normal" error handling
		return err
	}

	v310_00_to_unknown := versioning.NewRangeOrDie("3.10.0", "3.10.1")

	errors := []error{}
	switch {
	case v310_00_to_unknown.BetweenOrEmpty(currentActualVerion) && v310_00_to_unknown.Between(&desiredVersion):
		var versionAvailability operatorsv1alpha1.VersionAvailablity
		operatorConfig.Status.TaskSummary = "sync-[3.10.0,3.10.1)"
		operatorConfig.Status.TargetAvailability = nil
		versionAvailability, errors = sync_v311_00_to_latest(c, operatorConfig, operatorConfig.Status.CurrentAvailability)
		operatorConfig.Status.CurrentAvailability = &versionAvailability

	default:
		operatorConfig.Status.TaskSummary = "unrecognized"
		if _, err := c.operatorConfigClient.ServiceCertSignerOperatorConfigs().UpdateStatus(operatorConfig); err != nil {
			utilruntime.HandleError(err)
		}

		return fmt.Errorf("unrecognized state")
	}

	// given the VersionAvailability and the status.Version, we can compute availability
	availableCondition := operatorsv1alpha1.OperatorCondition{
		Type:   operatorsv1alpha1.OperatorStatusTypeAvailable,
		Status: operatorsv1alpha1.ConditionUnknown,
	}
	if operatorConfig.Status.CurrentAvailability != nil && operatorConfig.Status.CurrentAvailability.ReadyReplicas > 0 {
		availableCondition.Status = operatorsv1alpha1.ConditionTrue
	} else {
		availableCondition.Status = operatorsv1alpha1.ConditionFalse
	}
	v1alpha1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, availableCondition)

	syncSuccessfulCondition := operatorsv1alpha1.OperatorCondition{
		Type:   operatorsv1alpha1.OperatorStatusTypeSyncSuccessful,
		Status: operatorsv1alpha1.ConditionTrue,
	}
	if operatorConfig.Status.CurrentAvailability != nil && len(operatorConfig.Status.CurrentAvailability.Errors) > 0 {
		syncSuccessfulCondition.Status = operatorsv1alpha1.ConditionFalse
		syncSuccessfulCondition.Message = strings.Join(operatorConfig.Status.CurrentAvailability.Errors, "\n")
	}
	if operatorConfig.Status.TargetAvailability != nil && len(operatorConfig.Status.TargetAvailability.Errors) > 0 {
		syncSuccessfulCondition.Status = operatorsv1alpha1.ConditionFalse
		if len(syncSuccessfulCondition.Message) == 0 {
			syncSuccessfulCondition.Message = strings.Join(operatorConfig.Status.TargetAvailability.Errors, "\n")
		} else {
			syncSuccessfulCondition.Message = availableCondition.Message + "\n" + strings.Join(operatorConfig.Status.TargetAvailability.Errors, "\n")
		}
	}
	v1alpha1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, syncSuccessfulCondition)
	if syncSuccessfulCondition.Status == operatorsv1alpha1.ConditionTrue {
		operatorConfig.Status.ObservedGeneration = operatorConfig.ObjectMeta.Generation
	}

	if _, err := c.operatorConfigClient.ServiceCertSignerOperatorConfigs().UpdateStatus(operatorConfig); err != nil {
		errors = append(errors, err)
	}

	return utilerrors.NewAggregate(errors)
}

// Run starts the serviceCertSigner and blocks until stopCh is closed.
func (c *ServiceCertSignerOperator) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting ServiceCertSignerOperator")
	defer glog.Infof("Shutting down ServiceCertSignerOperator")

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *ServiceCertSignerOperator) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *ServiceCertSignerOperator) processNextWorkItem() bool {
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
func (c *ServiceCertSignerOperator) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(workQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(workQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(workQueueKey) },
	}
}

// this set of namespaces will include things like logging and metrics which are used to drive
var interestingNamespaces = sets.NewString(targetNamespaceName)

func (c *ServiceCertSignerOperator) namespaceEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ns, ok := obj.(*corev1.Namespace)
			if !ok {
				c.queue.Add(workQueueKey)
			}
			if ns.Name == targetNamespaceName {
				c.queue.Add(workQueueKey)
			}
		},
		UpdateFunc: func(old, new interface{}) {
			ns, ok := old.(*corev1.Namespace)
			if !ok {
				c.queue.Add(workQueueKey)
			}
			if ns.Name == targetNamespaceName {
				c.queue.Add(workQueueKey)
			}
		},
		DeleteFunc: func(obj interface{}) {
			ns, ok := obj.(*corev1.Namespace)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					utilruntime.HandleError(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
					return
				}
				ns, ok = tombstone.Obj.(*corev1.Namespace)
				if !ok {
					utilruntime.HandleError(fmt.Errorf("Tombstone contained object that is not a Namespace %#v", obj))
					return
				}
			}
			if ns.Name == targetNamespaceName {
				c.queue.Add(workQueueKey)
			}
		},
	}
}

/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeutil "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	checksum "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/checksum/versioned/v1alpha1"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	"github.com/kubernetes-incubator/service-catalog/pkg/brokerapi"
	servicecatalogclientset "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/typed/servicecatalog/v1alpha1"
	informers "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/externalversions/servicecatalog/v1alpha1"
	listers "github.com/kubernetes-incubator/service-catalog/pkg/client/listers_generated/servicecatalog/v1alpha1"
)

const (
	// maxRetries is the number of times a resource add/update will be retried before it is dropped out of the queue.
	// With the current rate-limiter in use (5ms*2^(maxRetries-1)) the following numbers represent the times
	// a resource is going to be requeued:
	//
	// 5ms, 10ms, 20ms, 40ms, 80ms, 160ms, 320ms, 640ms, 1.3s, 2.6s, 5.1s, 10.2s, 20.4s, 41s, 82s
	maxRetries = 15
	//
	pollingStartInterval      = 1 * time.Second
	pollingMaxBackoffDuration = 1 * time.Hour
)

// NewController returns a new Open Service Broker catalog controller.
func NewController(
	kubeClient kubernetes.Interface,
	serviceCatalogClient servicecatalogclientset.ServicecatalogV1alpha1Interface,
	brokerInformer informers.BrokerInformer,
	serviceClassInformer informers.ServiceClassInformer,
	instanceInformer informers.InstanceInformer,
	bindingInformer informers.BindingInformer,
	brokerClientCreateFunc brokerapi.CreateFunc,
	brokerRelistInterval time.Duration,
	osbAPIContextProfile bool,
	recorder record.EventRecorder,
) (Controller, error) {
	controller := &controller{
		kubeClient:                kubeClient,
		serviceCatalogClient:      serviceCatalogClient,
		brokerClientCreateFunc:    brokerClientCreateFunc,
		brokerRelistInterval:      brokerRelistInterval,
		enableOSBAPIContextProfle: osbAPIContextProfile,
		recorder:                  recorder,
		brokerQueue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "broker"),
		serviceClassQueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "service-class"),
		instanceQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "instance"),
		bindingQueue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "binding"),
		pollingQueue:              workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(pollingStartInterval, pollingMaxBackoffDuration), "poller"),
	}

	brokerInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.brokerAdd,
		UpdateFunc: controller.brokerUpdate,
		DeleteFunc: controller.brokerDelete,
	})
	controller.brokerLister = brokerInformer.Lister()

	serviceClassInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.serviceClassAdd,
		UpdateFunc: controller.serviceClassUpdate,
		DeleteFunc: controller.serviceClassDelete,
	})
	controller.serviceClassLister = serviceClassInformer.Lister()

	instanceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.instanceAdd,
		UpdateFunc: controller.instanceUpdate,
		DeleteFunc: controller.instanceDelete,
	})
	controller.instanceLister = instanceInformer.Lister()

	bindingInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.bindingAdd,
		UpdateFunc: controller.bindingUpdate,
		DeleteFunc: controller.bindingDelete,
	})
	controller.bindingLister = bindingInformer.Lister()

	return controller, nil
}

// Controller describes a controller that backs the service catalog API for
// Open Service Broker compliant Brokers.
type Controller interface {
	// Run runs the controller until the given stop channel can be read from.
	// workers specifies the number of goroutines, per resource, processing work
	// from the resource workqueues
	Run(workers int, stopCh <-chan struct{})
}

// controller is a concrete Controller.
type controller struct {
	kubeClient                kubernetes.Interface
	serviceCatalogClient      servicecatalogclientset.ServicecatalogV1alpha1Interface
	brokerClientCreateFunc    brokerapi.CreateFunc
	brokerLister              listers.BrokerLister
	serviceClassLister        listers.ServiceClassLister
	instanceLister            listers.InstanceLister
	bindingLister             listers.BindingLister
	brokerRelistInterval      time.Duration
	enableOSBAPIContextProfle bool
	recorder                  record.EventRecorder
	brokerQueue               workqueue.RateLimitingInterface
	serviceClassQueue         workqueue.RateLimitingInterface
	instanceQueue             workqueue.RateLimitingInterface
	bindingQueue              workqueue.RateLimitingInterface
	// pollingQueue is separate from instanceQueue because we want
	// it to have different backoff / timeout characteristics from
	//  a reconciling of an instance.
	// TODO(vaikas): get rid of two queues per instance.
	pollingQueue workqueue.RateLimitingInterface
}

// Run runs the controller until the given stop channel can be read from.
func (c *controller) Run(workers int, stopCh <-chan struct{}) {
	defer runtimeutil.HandleCrash()

	glog.Info("Starting service-catalog controller")

	for i := 0; i < workers; i++ {
		go wait.Until(worker(c.brokerQueue, "Broker", maxRetries, c.reconcileBrokerKey), time.Second, stopCh)
		go wait.Until(worker(c.serviceClassQueue, "ServiceClass", maxRetries, c.reconcileServiceClassKey), time.Second, stopCh)
		go wait.Until(worker(c.instanceQueue, "Instance", maxRetries, c.reconcileInstanceKey), time.Second, stopCh)
		go wait.Until(worker(c.bindingQueue, "Binding", maxRetries, c.reconcileBindingKey), time.Second, stopCh)
		go wait.Until(worker(c.pollingQueue, "Poller", maxRetries, c.reconcileInstanceKey), time.Second, stopCh)
	}

	<-stopCh
	glog.Info("Shutting down service-catalog controller")

	c.brokerQueue.ShutDown()
	c.serviceClassQueue.ShutDown()
	c.instanceQueue.ShutDown()
	c.bindingQueue.ShutDown()
	c.pollingQueue.ShutDown()
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// If reconciler returns an error, requeue the item up to maxRetries before giving up.
// It enforces that the reconciler is never invoked concurrently with the same key.
// TODO: Consider allowing the reconciler to return an error that either specifies whether
// this is recoverable or not, rather than always continuing on an error condition. Seems
// like it should be possible to return an error, yet stop any further polling work.
func worker(queue workqueue.RateLimitingInterface, resourceType string, maxRetries int, reconciler func(key string) error) func() {
	return func() {
		exit := false
		for !exit {
			exit = func() bool {
				key, quit := queue.Get()
				if quit {
					return true
				}
				defer queue.Done(key)

				err := reconciler(key.(string))
				if err == nil {
					queue.Forget(key)
					return false
				}

				if queue.NumRequeues(key) < maxRetries {
					glog.V(4).Infof("Error syncing %s %v: %v", resourceType, key, err)
					queue.AddRateLimited(key)
					return false
				}

				glog.V(4).Infof("Dropping %s %q out of the queue: %v", resourceType, key, err)
				queue.Forget(key)
				return false
			}()
		}
	}
}

// Broker handlers and control-loop

func (c *controller) brokerAdd(obj interface{}) {
	// DeletionHandlingMetaNamespaceKeyFunc returns a unique key for the resource and
	// handles the special case where the resource is of DeletedFinalStateUnknown type, which
	// acts a place holder for resources that have been deleted from storage but the watch event
	// confirming the deletion has not yet arrived.
	// Generally, the key is "namespace/name" for namespaced-scoped resources and
	// just "name" for cluster scoped resources.
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.brokerQueue.Add(key)
}

func (c *controller) brokerUpdate(oldObj, newObj interface{}) {
	c.brokerAdd(newObj)
}

func (c *controller) brokerDelete(obj interface{}) {
	broker, ok := obj.(*v1alpha1.Broker)
	if broker == nil || !ok {
		return
	}

	glog.V(4).Infof("Received delete event for Broker %v", broker.Name)
}

// the Message strings have a terminating period and space so they can
// be easily combined with a follow on specific message.
const (
	errorFetchingCatalogReason            string = "ErrorFetchingCatalog"
	errorFetchingCatalogMessage           string = "Error fetching catalog. "
	errorSyncingCatalogReason             string = "ErrorSyncingCatalog"
	errorSyncingCatalogMessage            string = "Error syncing catalog from Broker. "
	errorWithParameters                   string = "ErrorWithParameters"
	errorListingServiceClassesReason      string = "ErrorListingServiceClasses"
	errorListingServiceClassesMessage     string = "Error listing service classes."
	errorDeletingServiceClassReason       string = "ErrorDeletingServiceClass"
	errorDeletingServiceClassMessage      string = "Error deleting service class."
	errorNonexistentServiceClassReason    string = "ReferencesNonexistentServiceClass"
	errorNonexistentServiceClassMessage   string = "ReferencesNonexistentServiceClass"
	errorNonexistentServicePlanReason     string = "ReferencesNonexistentServicePlan"
	errorNonexistentBrokerReason          string = "ReferencesNonexistentBroker"
	errorNonexistentInstanceReason        string = "ReferencesNonexistentInstance"
	errorAuthCredentialsReason            string = "ErrorGettingAuthCredentials"
	errorFindingNamespaceInstanceReason   string = "ErrorFindingNamespaceForInstance"
	errorProvisionCalledReason            string = "ProvisionCallFailed"
	errorDeprovisionCalledReason          string = "DeprovisionCallFailed"
	errorBindCallReason                   string = "BindCallFailed"
	errorInjectingBindResultReason        string = "ErrorInjectingBindResult"
	errorEjectingBindReason               string = "ErrorEjectingBinding"
	errorEjectingBindMessage              string = "Error ejecting binding."
	errorUnbindCallReason                 string = "UnbindCallFailed"
	errorWithOngoingAsyncOperation        string = "ErrorAsyncOperationInProgress"
	errorWithOngoingAsyncOperationMessage string = "Another operation for this service instance is in progress. "
	errorNonbindableServiceClassReason    string = "ErrorNonbindableServiceClass"
	errorInstanceNotReadyReason           string = "ErrorInstanceNotReady"

	successInjectedBindResultReason  string = "InjectedBindResult"
	successInjectedBindResultMessage string = "Injected bind result"
	successDeprovisionReason         string = "DeprovisionedSuccessfully"
	successDeprovisionMessage        string = "The instance was deprovisioned successfully"
	successProvisionReason           string = "ProvisionedSuccessfully"
	successProvisionMessage          string = "The instance was provisioned successfully"
	successFetchedCatalogReason      string = "FetchedCatalog"
	successFetchedCatalogMessage     string = "Successfully fetched catalog entries from broker."
	successBrokerDeletedReason       string = "DeletedSuccessfully"
	successBrokerDeletedMessage      string = "The broker %v was deleted successfully."
	successUnboundReason             string = "UnboundSuccessfully"
	asyncProvisioningReason          string = "Provisioning"
	asyncProvisioningMessage         string = "The instance is being provisioned asynchronously"
	asyncDeprovisioningReason        string = "Derovisioning"
	asyncDeprovisioningMessage       string = "The instance is being deprovisioned asynchronously"
)

// shouldReconcileBroker determines whether a broker should be reconciled; it
// returns true unless the broker has a ready condition with status true and
// the controller's broker relist interval has not elapsed since the broker's
// ready condition became true.
func shouldReconcileBroker(broker *v1alpha1.Broker, now time.Time, relistInterval time.Duration) bool {
	if broker.DeletionTimestamp != nil || len(broker.Status.Conditions) == 0 {
		// If the deletion timestamp is set or the broker has no status
		// conditions, we should reconcile it.
		return true
	}

	// find the ready condition in the broker's status
	for _, condition := range broker.Status.Conditions {
		if condition.Type == v1alpha1.BrokerConditionReady {
			// The broker has a ready condition

			if condition.Status == v1alpha1.ConditionTrue {
				// The broker's ready condition has status true, meaning that
				// at some point, we successfully listed the broker's catalog.
				// We should reconcile the broker (relist the broker's
				// catalog) if it has been longer than the configured relist
				// interval since the broker's ready condition became true.
				return now.After(condition.LastTransitionTime.Add(relistInterval))
			}

			// The broker's ready condition wasn't true; we should try to re-
			// list the broker.
			return true
		}
	}

	// The broker didn't have a ready condition; we should reconcile it.
	return true
}

func (c *controller) reconcileBrokerKey(key string) error {
	broker, err := c.brokerLister.Get(key)
	if errors.IsNotFound(err) {
		glog.Infof("Not doing work for Broker %v because it has been deleted", key)
		return nil
	}
	if err != nil {
		glog.Infof("Unable to retrieve Broker %v from store: %v", key, err)
		return err
	}

	return c.reconcileBroker(broker)
}

// reconcileBroker is the control-loop that reconciles a Broker.
func (c *controller) reconcileBroker(broker *v1alpha1.Broker) error {
	glog.V(4).Infof("Processing Broker %v", broker.Name)

	// If the broker's ready condition is true and the relist interval has not
	// elapsed, do not reconcile it.
	if !shouldReconcileBroker(broker, time.Now(), c.brokerRelistInterval) {
		glog.V(10).Infof("Not processing Broker %v because relist interval has not elapsed since the broker became ready", broker.Name)
		return nil
	}

	username, password, err := getAuthCredentialsFromBroker(c.kubeClient, broker)
	if err != nil {
		s := fmt.Sprintf("Error getting broker auth credentials for broker %q: %s", broker.Name, err)
		glog.Info(s)
		c.recorder.Event(broker, api.EventTypeWarning, errorAuthCredentialsReason, s)
		c.updateBrokerCondition(broker, v1alpha1.BrokerConditionReady, v1alpha1.ConditionFalse, errorFetchingCatalogReason, errorFetchingCatalogMessage+s)
		return err
	}

	glog.V(4).Infof("Creating client for Broker %v, URL: %v", broker.Name, broker.Spec.URL)
	brokerClient := c.brokerClientCreateFunc(broker.Name, broker.Spec.URL, username, password)

	if broker.DeletionTimestamp == nil { // Add or update
		glog.V(4).Infof("Adding/Updating Broker %v", broker.Name)
		brokerCatalog, err := brokerClient.GetCatalog()
		if err != nil {
			s := fmt.Sprintf("Error getting broker catalog for broker %q: %s", broker.Name, err)
			glog.Warning(s)
			c.recorder.Eventf(broker, api.EventTypeWarning, errorFetchingCatalogReason, s)
			c.updateBrokerCondition(broker, v1alpha1.BrokerConditionReady, v1alpha1.ConditionFalse, errorFetchingCatalogReason,
				errorFetchingCatalogMessage+s)
			return err
		}
		glog.V(5).Infof("Successfully fetched %v catalog entries for Broker %v", len(brokerCatalog.Services), broker.Name)

		glog.V(4).Infof("Converting catalog response for Broker %v into service-catalog API", broker.Name)
		catalog, err := convertCatalog(brokerCatalog)
		if err != nil {
			s := fmt.Sprintf("Error converting catalog payload for broker %q to service-catalog API: %s", broker.Name, err)
			glog.Warning(s)
			c.recorder.Eventf(broker, api.EventTypeWarning, errorSyncingCatalogReason, s)
			c.updateBrokerCondition(broker, v1alpha1.BrokerConditionReady, v1alpha1.ConditionFalse, errorSyncingCatalogReason, errorSyncingCatalogMessage+s)
			return err
		}
		glog.V(5).Infof("Successfully converted catalog payload from Broker %v to service-catalog API", broker.Name)

		for _, serviceClass := range catalog {
			glog.V(4).Infof("Reconciling serviceClass %v (broker %v)", serviceClass.Name, broker.Name)
			if err := c.reconcileServiceClassFromBrokerCatalog(broker, serviceClass); err != nil {
				s := fmt.Sprintf("Error reconciling serviceClass %q (broker %q): %s", serviceClass.Name, broker.Name, err)
				glog.Warning(s)
				c.recorder.Eventf(broker, api.EventTypeWarning, errorSyncingCatalogReason, s)
				c.updateBrokerCondition(broker, v1alpha1.BrokerConditionReady, v1alpha1.ConditionFalse, errorSyncingCatalogReason,
					errorSyncingCatalogMessage+s)
				return err
			}

			glog.V(5).Infof("Reconciled serviceClass %v (broker %v)", serviceClass.Name, broker.Name)
		}

		c.updateBrokerCondition(broker, v1alpha1.BrokerConditionReady, v1alpha1.ConditionTrue, successFetchedCatalogReason, successFetchedCatalogMessage)
		c.recorder.Event(broker, api.EventTypeNormal, successFetchedCatalogReason, successFetchedCatalogMessage)
		return nil
	}

	// All updates not having a DeletingTimestamp will have been handled above
	// and returned early. If we reach this point, we're dealing with an update
	// that's actually a soft delete-- i.e. we have some finalization to do.
	// Since the potential exists for a broker to have multiple finalizers and
	// since those most be cleared in order, we proceed with the soft delete
	// only if it's "our turn--" i.e. only if the finalizer we care about is at
	// the head of the finalizers list.
	// TODO: Should we use a more specific string here?
	if len(broker.Finalizers) > 0 && broker.Finalizers[0] == "kubernetes" {
		glog.V(4).Infof("Finalizing Broker %v", broker.Name)

		// Get ALL ServiceClasses. Remove those that reference this Broker.
		svcClasses, err := c.serviceClassLister.List(labels.Everything())
		if err != nil {
			c.updateBrokerCondition(
				broker,
				v1alpha1.BrokerConditionReady,
				v1alpha1.ConditionUnknown,
				errorListingServiceClassesReason,
				errorListingServiceClassesMessage,
			)
			c.recorder.Eventf(broker, api.EventTypeWarning, errorListingServiceClassesReason, "%v %v", errorListingServiceClassesMessage, err)
			return err
		}

		// Delete ServiceClasses that are for THIS Broker.
		for _, svcClass := range svcClasses {
			if svcClass.BrokerName == broker.Name {
				err := c.serviceCatalogClient.ServiceClasses().Delete(svcClass.Name, &metav1.DeleteOptions{})
				if err != nil && !errors.IsNotFound(err) {
					s := fmt.Sprintf("Error deleting ServiceClass %q (Broker %q): %s", svcClass.Name, broker.Name, err)
					glog.Warning(s)
					c.updateBrokerCondition(
						broker,
						v1alpha1.BrokerConditionReady,
						v1alpha1.ConditionUnknown,
						errorDeletingServiceClassMessage,
						errorDeletingServiceClassReason+s,
					)
					c.recorder.Eventf(broker, api.EventTypeWarning, errorDeletingServiceClassReason, "%v %v", errorDeletingServiceClassMessage, s)
					return err
				}
			}
		}

		c.updateBrokerCondition(
			broker,
			v1alpha1.BrokerConditionReady,
			v1alpha1.ConditionFalse,
			successBrokerDeletedReason,
			"The broker was deleted successfully",
		)
		// Clear the finalizer
		c.updateBrokerFinalizers(broker, broker.Finalizers[1:])

		c.recorder.Eventf(broker, api.EventTypeNormal, successBrokerDeletedReason, successBrokerDeletedMessage, broker.Name)
		glog.V(5).Infof("Successfully deleted Broker %v", broker.Name)
		return nil
	}

	return nil
}

// reconcileServiceClassFromBrokerCatalog reconciles a ServiceClass after the
// Broker's catalog has been re-listed.
func (c *controller) reconcileServiceClassFromBrokerCatalog(broker *v1alpha1.Broker, serviceClass *v1alpha1.ServiceClass) error {
	serviceClass.BrokerName = broker.Name

	existingServiceClass, err := c.serviceClassLister.Get(serviceClass.Name)
	if errors.IsNotFound(err) {
		// An error returned from a lister Get call means that the object does
		// not exist.  Create a new ServiceClass.
		if _, err := c.serviceCatalogClient.ServiceClasses().Create(serviceClass); err != nil {
			glog.Errorf("Error creating serviceClass %v from Broker %v: %v", serviceClass.Name, broker.Name, err)
			return err
		}

		return nil
	} else if err != nil {
		glog.Errorf("Error getting serviceClass %v: %v", serviceClass.Name, err)
		return err
	}

	if existingServiceClass.BrokerName != broker.Name {
		errMsg := fmt.Sprintf("ServiceClass %q for Broker %q already exists for Broker %q", serviceClass.Name, broker.Name, existingServiceClass.BrokerName)
		glog.Error(errMsg)
		return fmt.Errorf(errMsg)
	}

	if existingServiceClass.ExternalID != serviceClass.ExternalID {
		errMsg := fmt.Sprintf("ServiceClass %q already exists with OSB guid %q, received different guid %q", serviceClass.Name, existingServiceClass.ExternalID, serviceClass.ExternalID)
		glog.Error(errMsg)
		return fmt.Errorf(errMsg)
	}

	glog.V(5).Infof("Found existing serviceClass %v; updating", serviceClass.Name)

	// There was an existing service class -- project the update onto it and
	// update it.
	clone, err := api.Scheme.DeepCopy(existingServiceClass)
	if err != nil {
		return err
	}

	toUpdate := clone.(*v1alpha1.ServiceClass)
	toUpdate.Bindable = serviceClass.Bindable
	toUpdate.Plans = serviceClass.Plans
	toUpdate.PlanUpdatable = serviceClass.PlanUpdatable
	toUpdate.AlphaTags = serviceClass.AlphaTags
	toUpdate.Description = serviceClass.Description
	toUpdate.AlphaRequires = serviceClass.AlphaRequires

	if _, err := c.serviceCatalogClient.ServiceClasses().Update(toUpdate); err != nil {
		glog.Errorf("Error updating serviceClass %v from Broker %v: %v", serviceClass.Name, broker.Name, err)
		return err
	}

	return nil
}

// updateBrokerReadyCondition updates the ready condition for the given Broker
// with the given status, reason, and message.
func (c *controller) updateBrokerCondition(broker *v1alpha1.Broker, conditionType v1alpha1.BrokerConditionType, status v1alpha1.ConditionStatus, reason, message string) error {
	clone, err := api.Scheme.DeepCopy(broker)
	if err != nil {
		return err
	}
	toUpdate := clone.(*v1alpha1.Broker)
	newCondition := v1alpha1.BrokerCondition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	}

	t := time.Now()

	if len(broker.Status.Conditions) == 0 {
		glog.Infof("Setting lastTransitionTime for Broker %q condition %q to %v", broker.Name, conditionType, t)
		newCondition.LastTransitionTime = metav1.NewTime(t)
		toUpdate.Status.Conditions = []v1alpha1.BrokerCondition{newCondition}
	} else {
		for i, cond := range broker.Status.Conditions {
			if cond.Type == conditionType {
				if cond.Status != newCondition.Status {
					glog.Infof("Found status change for Broker %q condition %q: %q -> %q; setting lastTransitionTime to %v", broker.Name, conditionType, cond.Status, status, t)
					newCondition.LastTransitionTime = metav1.NewTime(t)
				} else {
					newCondition.LastTransitionTime = cond.LastTransitionTime
				}

				toUpdate.Status.Conditions[i] = newCondition
				break
			}
		}
	}

	glog.V(4).Infof("Updating ready condition for Broker %v to %v", broker.Name, status)
	_, err = c.serviceCatalogClient.Brokers().UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf("Error updating ready condition for Broker %v: %v", broker.Name, err)
	} else {
		glog.V(5).Infof("Updated ready condition for Broker %v to %v", broker.Name, status)
	}

	return err
}

// updateBrokerFinalizers updates the given finalizers for the given Broker.
func (c *controller) updateBrokerFinalizers(
	broker *v1alpha1.Broker,
	finalizers []string) error {

	clone, err := api.Scheme.DeepCopy(broker)
	if err != nil {
		return err
	}
	toUpdate := clone.(*v1alpha1.Broker)

	toUpdate.Finalizers = finalizers

	logContext := fmt.Sprintf("finalizers for Broker %v to %v",
		broker.Name, finalizers)

	glog.V(4).Infof("Updating %v", logContext)
	_, err = c.serviceCatalogClient.Brokers().UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf("Error updating %v: %v", logContext, err)
	}
	return err
}

// Service class handlers and control-loop

func (c *controller) serviceClassAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.serviceClassQueue.Add(key)
}

func (c *controller) reconcileServiceClassKey(key string) error {
	serviceClass, err := c.serviceClassLister.Get(key)
	if errors.IsNotFound(err) {
		glog.Infof("Not doing work for ServiceClass %v because it has been deleted", key)
		return nil
	}
	if err != nil {
		glog.Errorf("Unable to retrieve ServiceClass %v from store: %v", key, err)
		return err
	}

	return c.reconcileServiceClass(serviceClass)
}

func (c *controller) reconcileServiceClass(serviceClass *v1alpha1.ServiceClass) error {
	glog.V(4).Infof("Processing ServiceClass %v", serviceClass.Name)
	return nil
}

func (c *controller) serviceClassUpdate(oldObj, newObj interface{}) {
	c.serviceClassAdd(newObj)
}

func (c *controller) serviceClassDelete(obj interface{}) {
	serviceClass, ok := obj.(*v1alpha1.ServiceClass)
	if serviceClass == nil || !ok {
		return
	}

	glog.V(4).Infof("Received delete event for ServiceClass %v", serviceClass.Name)
}

// Instance handlers and control-loop

func (c *controller) instanceAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	// TODO(vaikas): If the obj (which really is an Instance right?) has
	// AsyncOpInProgress flag set, just add it directly to c.pollingQueue
	// here? Why shouldn't we??
	c.instanceQueue.Add(key)
}

func (c *controller) reconcileInstanceKey(key string) error {
	// For namespace-scoped resources, SplitMetaNamespaceKey splits the key
	// i.e. "namespace/name" into two separate strings
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	instance, err := c.instanceLister.Instances(namespace).Get(name)
	if errors.IsNotFound(err) {
		glog.Infof("Not doing work for Instance %v because it has been deleted", key)
		return nil
	}
	if err != nil {
		glog.Errorf("Unable to retrieve Instance %v from store: %v", key, err)
		return err
	}

	return c.reconcileInstance(instance)
}

func (c *controller) instanceUpdate(oldObj, newObj interface{}) {
	c.instanceAdd(newObj)
}

// reconcileInstance is the control-loop for reconciling Instances.
func (c *controller) reconcileInstance(instance *v1alpha1.Instance) error {

	// If there's no async op in progress, determine whether the checksum
	// has been invalidated by a change to the object. If the instance's
	// checksum matches the calculated checksum, there is no work to do.
	// If there's an async op in progress, we need to keep polling, hence
	// do not bail if checksum hasn't changed.
	//
	// We only do this if the deletion timestamp is nil, because the deletion
	// timestamp changes the object's state in a way that we must reconcile,
	// but does not affect the checksum.
	if !instance.Status.AsyncOpInProgress {
		if instance.Status.Checksum != nil && instance.DeletionTimestamp == nil {
			instanceChecksum := checksum.InstanceSpecChecksum(instance.Spec)
			if instanceChecksum == *instance.Status.Checksum {
				glog.V(4).Infof("Not processing event for Instance %v/%v because checksum showed there is no work to do", instance.Namespace, instance.Name)
				return nil
			}
		}
	}

	glog.V(4).Infof("Processing Instance %v/%v", instance.Namespace, instance.Name)

	serviceClass, servicePlan, brokerName, brokerClient, err := c.getServiceClassPlanAndBroker(instance)
	if err != nil {
		return err
	}

	if instance.Status.AsyncOpInProgress {
		return c.pollInstance(serviceClass, servicePlan, brokerName, brokerClient, instance)
	}

	if instance.DeletionTimestamp == nil { // Add or update
		glog.V(4).Infof("Adding/Updating Instance %v/%v", instance.Namespace, instance.Name)

		var parameters map[string]interface{}
		if instance.Spec.Parameters != nil {
			parameters, err = unmarshalParameters(instance.Spec.Parameters.Raw)
			if err != nil {
				s := fmt.Sprintf("Failed to unmarshal Instance parameters\n%s\n %s", instance.Spec.Parameters, err)
				glog.Warning(s)
				c.updateInstanceCondition(
					instance,
					v1alpha1.InstanceConditionReady,
					v1alpha1.ConditionFalse,
					errorWithParameters,
					"Error unmarshaling instance parameters. "+s,
				)
				c.recorder.Event(instance, api.EventTypeWarning, errorWithParameters, s)
				return err
			}
		}

		ns, err := c.kubeClient.Core().Namespaces().Get(instance.Namespace, metav1.GetOptions{})
		if err != nil {
			s := fmt.Sprintf("Failed to get namespace %q during instance create: %s", instance.Namespace, err)
			glog.Info(s)
			c.updateInstanceCondition(
				instance,
				v1alpha1.InstanceConditionReady,
				v1alpha1.ConditionFalse,
				errorFindingNamespaceInstanceReason,
				"Error finding namespace for instance. "+s,
			)
			c.recorder.Event(instance, api.EventTypeWarning, errorFindingNamespaceInstanceReason, s)
			return err
		}

		request := &brokerapi.CreateServiceInstanceRequest{
			ServiceID:         serviceClass.ExternalID,
			PlanID:            servicePlan.ExternalID,
			Parameters:        parameters,
			OrgID:             string(ns.UID),
			SpaceID:           string(ns.UID),
			AcceptsIncomplete: true,
		}
		if c.enableOSBAPIContextProfle {
			request.ContextProfile = brokerapi.ContextProfile{
				Platform:  brokerapi.ContextProfilePlatformKubernetes,
				Namespace: instance.Namespace,
			}
		}

		glog.V(4).Infof("Provisioning a new Instance %v/%v of ServiceClass %v at Broker %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName)
		response, respCode, err := brokerClient.CreateServiceInstance(instance.Spec.ExternalID, request)
		if err != nil {
			s := fmt.Sprintf("Error provisioning Instance \"%s/%s\" of ServiceClass %q at Broker %q: %s", instance.Namespace, instance.Name, serviceClass.Name, brokerName, err)
			glog.Warning(s)
			c.updateInstanceCondition(
				instance,
				v1alpha1.InstanceConditionReady,
				v1alpha1.ConditionFalse,
				errorProvisionCalledReason,
				"Provision call failed. "+s)
			c.recorder.Event(instance, api.EventTypeWarning, errorProvisionCalledReason, s)
			return err
		}

		if response.DashboardURL != "" {
			instance.Status.DashboardURL = &response.DashboardURL
		}

		// Broker can return either a synchronous or asynchronous
		// response, if the response is StatusAccepted it's an async
		// and we need to add it to the polling queue. Broker can
		// optionally return 'Operation' that will then need to be
		// passed back to the broker during polling of last_operation.
		if respCode == http.StatusAccepted {
			glog.V(5).Infof("Received asynchronous provisioning response for Instance %v/%v of ServiceClass %v at Broker %v: response: %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName, response)
			if response.Operation != "" {
				instance.Status.LastOperation = &response.Operation
			}

			// Tag this instance as having an ongoing async operation so we can enforce
			// no other operations against it can start.
			instance.Status.AsyncOpInProgress = true

			c.updateInstanceCondition(
				instance,
				v1alpha1.InstanceConditionReady,
				v1alpha1.ConditionFalse,
				asyncProvisioningReason,
				asyncProvisioningMessage,
			)
			c.recorder.Eventf(instance, api.EventTypeNormal, asyncProvisioningReason, asyncProvisioningMessage)

			// Actually, start polling this Service Instance by adding it into the polling queue
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(instance)
			if err != nil {
				glog.Errorf("Couldn't create a key for object %+v: %v", instance, err)
				return fmt.Errorf("Couldn't create a key for object %+v: %v", instance, err)
			}
			c.pollingQueue.Add(key)
		} else {
			glog.V(5).Infof("Successfully provisioned Instance %v/%v of ServiceClass %v at Broker %v: response: %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName, response)

			// TODO: process response
			c.updateInstanceCondition(
				instance,
				v1alpha1.InstanceConditionReady,
				v1alpha1.ConditionTrue,
				successProvisionReason,
				successProvisionMessage,
			)
			c.recorder.Eventf(instance, api.EventTypeNormal, successProvisionReason, successProvisionMessage)
		}
		return nil
	}

	// All updates not having a DeletingTimestamp will have been handled above
	// and returned early. If we reach this point, we're dealing with an update
	// that's actually a soft delete-- i.e. we have some finalization to do.
	// Since the potential exists for an instance to have multiple finalizers and
	// since those most be cleared in order, we proceed with the soft delete
	// only if it's "our turn--" i.e. only if the finalizer we care about is at
	// the head of the finalizers list.
	// TODO: Should we use a more specific string here?
	if len(instance.Finalizers) > 0 && instance.Finalizers[0] == "kubernetes" {
		glog.V(4).Infof("Finalizing Instance %v/%v", instance.Namespace, instance.Name)

		request := &brokerapi.DeleteServiceInstanceRequest{
			ServiceID:         serviceClass.ExternalID,
			PlanID:            servicePlan.ExternalID,
			AcceptsIncomplete: true,
		}

		glog.V(4).Infof("Deprovisioning Instance %v/%v of ServiceClass %v at Broker %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName)
		response, respCode, err := brokerClient.DeleteServiceInstance(instance.Spec.ExternalID, request)

		if err != nil {
			s := fmt.Sprintf("Error deprovisioning Instance \"%s/%s\" of ServiceClass %q at Broker %q: %s", instance.Namespace, instance.Name, serviceClass.Name, brokerName, err)
			glog.Warning(s)
			c.updateInstanceCondition(
				instance,
				v1alpha1.InstanceConditionReady,
				v1alpha1.ConditionUnknown,
				errorDeprovisionCalledReason,
				"Deprovision call failed. "+s)
			c.recorder.Event(instance, api.EventTypeWarning, errorDeprovisionCalledReason, s)
			return err
		}

		if respCode == http.StatusAccepted {
			glog.V(5).Infof("Received asynchronous de-provisioning response for Instance %v/%v of ServiceClass %v at Broker %v: response: %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName, response)
			if response.Operation != "" {
				instance.Status.LastOperation = &response.Operation
			}

			// Tag this instance as having an ongoing async operation so we can enforce
			// no other operations against it can start.
			instance.Status.AsyncOpInProgress = true

			c.updateInstanceCondition(
				instance,
				v1alpha1.InstanceConditionReady,
				v1alpha1.ConditionFalse,
				asyncDeprovisioningReason,
				asyncDeprovisioningMessage,
			)
		} else {
			c.updateInstanceCondition(
				instance,
				v1alpha1.InstanceConditionReady,
				v1alpha1.ConditionFalse,
				successDeprovisionReason,
				successDeprovisionMessage,
			)
			// Clear the finalizer
			c.updateInstanceFinalizers(instance, instance.Finalizers[1:])
			c.recorder.Event(instance, api.EventTypeNormal, successDeprovisionReason, successDeprovisionMessage)
			glog.V(5).Infof("Successfully deprovisioned Instance %v/%v of ServiceClass %v at Broker %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName)
		}
	}

	return nil
}

func (c *controller) pollInstanceInternal(instance *v1alpha1.Instance) error {
	glog.V(4).Infof("Processing Instance %v/%v", instance.Namespace, instance.Name)

	serviceClass, servicePlan, brokerName, brokerClient, err := c.getServiceClassPlanAndBroker(instance)
	if err != nil {
		return err
	}
	return c.pollInstance(serviceClass, servicePlan, brokerName, brokerClient, instance)
}

func (c *controller) pollInstance(serviceClass *v1alpha1.ServiceClass, servicePlan *v1alpha1.ServicePlan, brokerName string, brokerClient brokerapi.BrokerClient, instance *v1alpha1.Instance) error {

	// There are some conditions that are different if we're
	// deleting, this is more readable than checking the
	// timestamps in various places.
	deleting := false
	if instance.DeletionTimestamp != nil {
		deleting = true
	}

	lastOperationRequest := &brokerapi.LastOperationRequest{
		ServiceID: serviceClass.ExternalID,
		PlanID:    servicePlan.ExternalID,
	}
	if instance.Status.LastOperation != nil && *instance.Status.LastOperation != "" {
		lastOperationRequest.Operation = *instance.Status.LastOperation
	}
	resp, rc, err := brokerClient.PollServiceInstance(instance.Spec.ExternalID, lastOperationRequest)
	if err != nil {
		glog.Warningf("Poll failed for %v/%v  : %s", instance.Namespace, instance.Name, err)
		return err
	}
	glog.V(4).Infof("Poll for %v/%v returned %q : %q", instance.Namespace, instance.Name, resp.State, resp.Description)

	// If the operation was for delete and we receive a http.StatusGone,
	// this is considered a success as per the spec, so mark as deleted
	// and remove any finalizers.
	if rc == http.StatusGone && deleting {
		instance.Status.AsyncOpInProgress = false
		// Clear the finalizer
		if len(instance.Finalizers) > 0 && instance.Finalizers[0] == "kubernetes" {
			c.updateInstanceFinalizers(instance, instance.Finalizers[1:])
		}
		c.updateInstanceCondition(
			instance,
			v1alpha1.InstanceConditionReady,
			v1alpha1.ConditionFalse,
			successDeprovisionReason,
			successDeprovisionMessage,
		)
		c.recorder.Event(instance, api.EventTypeNormal, successDeprovisionReason, successDeprovisionMessage)
		glog.V(5).Infof("Successfully deprovisioned Instance %v/%v of ServiceClass %v at Broker %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName)
		return nil
	}

	switch resp.State {
	case "in progress":
		// The way the worker keeps on requeueing is by returning an error, so
		// we need to keep on polling.
		// TODO(vaikas): Update the instance condition with progress message here?
		return fmt.Errorf("last operation not completed (still in progress) for %v/%v", instance.Namespace, instance.Name)
	case "succeeded":
		// this gets updated as a side effect in both cases below.
		instance.Status.AsyncOpInProgress = false

		// If we were asynchronously deleting a Service Instance, finish
		// the finalizers.
		if deleting {
			c.updateInstanceCondition(
				instance,
				v1alpha1.InstanceConditionReady,
				v1alpha1.ConditionFalse,
				successDeprovisionReason,
				successDeprovisionMessage,
			)
			// Clear the finalizer
			if len(instance.Finalizers) > 0 && instance.Finalizers[0] == "kubernetes" {
				c.updateInstanceFinalizers(instance, instance.Finalizers[1:])
			}
			c.recorder.Event(instance, api.EventTypeNormal, successDeprovisionReason, successDeprovisionMessage)
			glog.V(5).Infof("Successfully deprovisioned Instance %v/%v of ServiceClass %v at Broker %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName)
		} else {
			c.updateInstanceCondition(
				instance,
				v1alpha1.InstanceConditionReady,
				v1alpha1.ConditionTrue,
				successProvisionReason,
				successProvisionMessage,
			)
		}
	case "failed":
		s := fmt.Sprintf("Error deprovisioning Instance \"%s/%s\" of ServiceClass %q at Broker %q: %q", instance.Namespace, instance.Name, serviceClass.Name, brokerName, resp.Description)
		instance.Status.AsyncOpInProgress = false
		cond := v1alpha1.ConditionFalse
		reason := errorProvisionCalledReason
		msg := "Provision call failed: " + s
		if deleting {
			cond = v1alpha1.ConditionUnknown
			reason = errorDeprovisionCalledReason
			msg = "Deprovision call failed:" + s
		}
		c.updateInstanceCondition(
			instance,
			v1alpha1.InstanceConditionReady,
			cond,
			reason,
			msg,
		)
		c.recorder.Event(instance, api.EventTypeWarning, errorDeprovisionCalledReason, s)
	default:
		glog.Warningf("Got invalid state in LastOperationResponse: %q", resp.State)
		return fmt.Errorf("Got invalid state in LastOperationResponse: %q", resp.State)
	}
	return nil
}

func findServicePlan(name string, plans []v1alpha1.ServicePlan) *v1alpha1.ServicePlan {
	for _, plan := range plans {
		if name == plan.Name {
			return &plan
		}
	}

	return nil
}

// updateInstanceCondition updates the given condition for the given Instance
// with the given status, reason, and message.
func (c *controller) updateInstanceCondition(
	instance *v1alpha1.Instance,
	conditionType v1alpha1.InstanceConditionType,
	status v1alpha1.ConditionStatus,
	reason, message string) error {

	clone, err := api.Scheme.DeepCopy(instance)
	if err != nil {
		return err
	}
	toUpdate := clone.(*v1alpha1.Instance)

	newCondition := v1alpha1.InstanceCondition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	}

	t := time.Now()

	if len(instance.Status.Conditions) == 0 {
		glog.Infof(`Setting lastTransitionTime for Instance "%v/%v" condition %q to %v`, instance.Namespace, instance.Name, conditionType, t)
		newCondition.LastTransitionTime = metav1.NewTime(t)
		toUpdate.Status.Conditions = []v1alpha1.InstanceCondition{newCondition}
	} else {
		for i, cond := range instance.Status.Conditions {
			if cond.Type == conditionType {
				if cond.Status != newCondition.Status {
					glog.Infof(`Found status change for Instance "%v/%v" condition %q: %q -> %q; setting lastTransitionTime to %v`, instance.Namespace, instance.Name, conditionType, cond.Status, status, t)
					newCondition.LastTransitionTime = metav1.NewTime(t)
				} else {
					newCondition.LastTransitionTime = cond.LastTransitionTime
				}

				toUpdate.Status.Conditions[i] = newCondition
				break
			}
		}
	}

	glog.V(4).Infof("Updating %v condition for Instance %v/%v to %v", conditionType, instance.Namespace, instance.Name, status)
	_, err = c.serviceCatalogClient.Instances(instance.Namespace).UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf("Failed to update condition %v for Instance %v/%v to true: %v", conditionType, instance.Namespace, instance.Name, err)
	}

	return err
}

// updateInstanceFinalizers updates the given finalizers for the given Binding.
func (c *controller) updateInstanceFinalizers(
	instance *v1alpha1.Instance,
	finalizers []string) error {

	// Get the latest version of the instance so that we can avoid conflicts
	// (since we have probably just updated the status of the instance and are
	// now removing the last finalizer).
	instance, err := c.serviceCatalogClient.Instances(instance.Namespace).Get(instance.Name, metav1.GetOptions{})
	if err != nil {
		glog.Errorf("Error getting Instance %v/%v to finalize: %v", instance.Namespace, instance.Name, err)
	}

	clone, err := api.Scheme.DeepCopy(instance)
	if err != nil {
		return err
	}
	toUpdate := clone.(*v1alpha1.Instance)

	toUpdate.Finalizers = finalizers

	logContext := fmt.Sprintf("finalizers for Instance %v/%v to %v",
		instance.Namespace, instance.Name, finalizers)

	glog.V(4).Infof("Updating %v", logContext)
	_, err = c.serviceCatalogClient.Instances(instance.Namespace).UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf("Error updating %v: %v", logContext, err)
	}
	return err
}

func (c *controller) instanceDelete(obj interface{}) {
	instance, ok := obj.(*v1alpha1.Instance)
	if instance == nil || !ok {
		return
	}

	glog.V(4).Infof("Received delete event for Instance %v/%v", instance.Namespace, instance.Name)
}

// Binding handlers and control-loop

func (c *controller) bindingAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.bindingQueue.Add(key)
}

func (c *controller) reconcileBindingKey(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	binding, err := c.bindingLister.Bindings(namespace).Get(name)
	if errors.IsNotFound(err) {
		glog.Infof("Not doing work for Binding %v because it has been deleted", key)
		return nil
	}
	if err != nil {
		glog.Infof("Unable to retrieve Binding %v from store: %v", key, err)
		return err
	}

	return c.reconcileBinding(binding)
}

func (c *controller) bindingUpdate(oldObj, newObj interface{}) {
	c.bindingAdd(newObj)
}

func (c *controller) reconcileBinding(binding *v1alpha1.Binding) error {
	// Determine whether the checksum has been invalidated by a change to the
	// object.  If the binding's checksum matches the calculated checksum,
	// there is no work to do.
	//
	// We only do this if the deletion timestamp is nil, because the deletion
	// timestamp changes the object's state in a way that we must reconcile,
	// but does not affect the checksum.
	if binding.Status.Checksum != nil && binding.DeletionTimestamp == nil {
		bindingChecksum := checksum.BindingSpecChecksum(binding.Spec)
		if bindingChecksum == *binding.Status.Checksum {
			glog.V(4).Infof("Not processing event for Binding %v/%v because checksum showed there is no work to do", binding.Namespace, binding.Name)
			return nil
		}
	}

	glog.V(4).Infof("Processing Binding %v/%v", binding.Namespace, binding.Name)

	instance, err := c.instanceLister.Instances(binding.Namespace).Get(binding.Spec.InstanceRef.Name)
	if err != nil {
		s := fmt.Sprintf("Binding \"%s/%s\" references a non-existent Instance \"%s/%s\"", binding.Namespace, binding.Name, binding.Namespace, binding.Spec.InstanceRef.Name)
		glog.Warning(s)
		c.updateBindingCondition(
			binding,
			v1alpha1.BindingConditionReady,
			v1alpha1.ConditionFalse,
			errorNonexistentInstanceReason,
			"The binding references an Instance that does not exist. "+s,
		)
		c.recorder.Event(binding, api.EventTypeWarning, errorNonexistentInstanceReason, s)
		return err
	}

	if instance.Status.AsyncOpInProgress {
		s := fmt.Sprintf("Binding \"%s/%s\" trying to bind to Instance \"%s/%s\" that has ongoing asynchronous operation", binding.Namespace, binding.Name, binding.Namespace, binding.Spec.InstanceRef.Name)
		glog.Info(s)
		c.updateBindingCondition(
			binding,
			v1alpha1.BindingConditionReady,
			v1alpha1.ConditionFalse,
			errorWithOngoingAsyncOperation,
			errorWithOngoingAsyncOperationMessage,
		)
		c.recorder.Event(binding, api.EventTypeWarning, errorWithOngoingAsyncOperation, s)
		return fmt.Errorf("Ongoing Asynchronous operation")
	}

	serviceClass, servicePlan, brokerName, brokerClient, err := c.getServiceClassPlanAndBrokerForBinding(instance, binding)
	if err != nil {
		return err
	}

	if !isPlanBindable(serviceClass, servicePlan) {
		s := fmt.Sprintf("Binding \"%s/%s\" references a non-bindable ServiceClass (%q) and Plan (%q) combination", binding.Namespace, binding.Name, instance.Spec.ServiceClassName, instance.Spec.PlanName)
		glog.Warning(s)
		c.updateBindingCondition(
			binding,
			v1alpha1.BindingConditionReady,
			v1alpha1.ConditionFalse,
			errorNonbindableServiceClassReason,
			s,
		)
		c.recorder.Event(binding, api.EventTypeWarning, errorNonbindableServiceClassReason, s)
		return err
	}

	if binding.DeletionTimestamp == nil { // Add or update
		glog.V(4).Infof("Adding/Updating Binding %v/%v", binding.Namespace, binding.Name)

		var parameters map[string]interface{}
		if binding.Spec.Parameters != nil {
			parameters, err = unmarshalParameters(binding.Spec.Parameters.Raw)
			if err != nil {
				s := fmt.Sprintf("Failed to unmarshal Binding parameters\n%s\n %s", binding.Spec.Parameters, err)
				glog.Warning(s)
				c.updateBindingCondition(
					binding,
					v1alpha1.BindingConditionReady,
					v1alpha1.ConditionFalse,
					errorWithParameters,
					"Error unmarshaling binding parameters. "+s,
				)
				c.recorder.Event(binding, api.EventTypeWarning, errorWithParameters, s)
				return err
			}
		}

		ns, err := c.kubeClient.Core().Namespaces().Get(instance.Namespace, metav1.GetOptions{})
		if err != nil {
			s := fmt.Sprintf("Failed to get namespace %q during binding: %s", instance.Namespace, err)
			glog.Info(s)
			c.updateBindingCondition(
				binding,
				v1alpha1.BindingConditionReady,
				v1alpha1.ConditionFalse,
				errorFindingNamespaceInstanceReason,
				"Error finding namespace for instance. "+s,
			)
			c.recorder.Eventf(binding, api.EventTypeWarning, errorFindingNamespaceInstanceReason, s)
			return err
		}

		if !isInstanceReady(instance) {
			s := fmt.Sprintf(`Binding cannot begin because referenced instance "%v/%v" is not ready`, instance.Namespace, instance.Name)
			glog.Info(s)
			c.updateBindingCondition(
				binding,
				v1alpha1.BindingConditionReady,
				v1alpha1.ConditionFalse,
				errorInstanceNotReadyReason,
				s,
			)
			c.recorder.Eventf(binding, api.EventTypeWarning, errorInstanceNotReadyReason, s)
			return err
		}

		request := &brokerapi.BindingRequest{
			ServiceID:    serviceClass.ExternalID,
			PlanID:       servicePlan.ExternalID,
			Parameters:   parameters,
			AppGUID:      string(ns.UID),
			BindResource: map[string]interface{}{"app_guid": string(ns.UID)},
		}
		response, err := brokerClient.CreateServiceBinding(instance.Spec.ExternalID, binding.Spec.ExternalID, request)
		if err != nil {
			s := fmt.Sprintf("Error creating Binding \"%s/%s\" for Instance \"%s/%s\" of ServiceClass %q at Broker %q: %s", binding.Name, binding.Namespace, instance.Namespace, instance.Name, serviceClass.Name, brokerName, err)
			glog.Warning(s)
			c.updateBindingCondition(
				binding,
				v1alpha1.BindingConditionReady,
				v1alpha1.ConditionFalse,
				errorBindCallReason,
				"Bind call failed. "+s)
			c.recorder.Event(binding, api.EventTypeWarning, errorBindCallReason, s)
			return err
		}
		err = c.injectBinding(binding, &response.Credentials)
		if err != nil {
			s := fmt.Sprintf("Error injecting binding results for Binding \"%s/%s\": %s", binding.Namespace, binding.Name, err)
			glog.Warning(s)
			c.updateBindingCondition(
				binding,
				v1alpha1.BindingConditionReady,
				v1alpha1.ConditionFalse,
				errorInjectingBindResultReason,
				"Error injecting bind result "+s,
			)
			c.recorder.Event(binding, api.EventTypeWarning, errorInjectingBindResultReason, s)
			return err
		}
		c.updateBindingCondition(
			binding,
			v1alpha1.BindingConditionReady,
			v1alpha1.ConditionTrue,
			successInjectedBindResultReason,
			successInjectedBindResultMessage,
		)
		c.recorder.Event(binding, api.EventTypeNormal, successInjectedBindResultReason, successInjectedBindResultMessage)

		glog.V(5).Infof("Successfully bound to Instance %v/%v of ServiceClass %v at Broker %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName)

		return nil
	}

	// All updates not having a DeletingTimestamp will have been handled above
	// and returned early. If we reach this point, we're dealing with an update
	// that's actually a soft delete-- i.e. we have some finalization to do.
	// Since the potential exists for a binding to have multiple finalizers and
	// since those most be cleared in order, we proceed with the soft delete
	// only if it's "our turn--" i.e. only if the finalizer we care about is at
	// the head of the finalizers list.
	// TODO: Should we use a more specific string here?
	if len(binding.Finalizers) > 0 && binding.Finalizers[0] == "kubernetes" {
		glog.V(4).Infof("Finalizing Binding %v/%v", binding.Namespace, binding.Name)
		err = c.ejectBinding(binding)
		if err != nil {
			s := fmt.Sprintf("Error deleting secret: %s", err)
			glog.Warning(s)
			c.updateBindingCondition(
				binding,
				v1alpha1.BindingConditionReady,
				v1alpha1.ConditionUnknown,
				errorEjectingBindReason,
				errorEjectingBindMessage+s,
			)
			c.recorder.Eventf(binding, api.EventTypeWarning, errorEjectingBindReason, "%v %v", errorEjectingBindMessage, s)
			return err
		}
		err = brokerClient.DeleteServiceBinding(instance.Spec.ExternalID, binding.Spec.ExternalID, serviceClass.ExternalID, servicePlan.ExternalID)
		if err != nil {
			s := fmt.Sprintf("Error unbinding Binding \"%s/%s\" for Instance \"%s/%s\" of ServiceClass %q at Broker %q: %s", binding.Name, binding.Namespace, instance.Namespace, instance.Name, serviceClass.Name, brokerName, err)
			glog.Warning(s)
			c.updateBindingCondition(
				binding,
				v1alpha1.BindingConditionReady,
				v1alpha1.ConditionFalse,
				errorUnbindCallReason,
				"Unbind call failed. "+s)
			c.recorder.Event(binding, api.EventTypeWarning, errorUnbindCallReason, s)
			return err
		}

		c.updateBindingCondition(
			binding,
			v1alpha1.BindingConditionReady,
			v1alpha1.ConditionFalse,
			successUnboundReason,
			"The binding was deleted successfully",
		)
		// Clear the finalizer
		c.updateBindingFinalizers(binding, binding.Finalizers[1:])
		c.recorder.Event(binding, api.EventTypeNormal, successUnboundReason, "This binding was deleted successfully")

		glog.V(5).Infof("Successfully deleted Binding %v/%v of Instance %v/%v of ServiceClass %v at Broker %v", binding.Namespace, binding.Name, instance.Namespace, instance.Name, serviceClass.Name, brokerName)
	}

	return nil
}

// isPlanBindable returns whether the given ServiceClass and ServicePlan
// combination is bindable.  Plans may override the service-level bindable
// attribute, so if the plan provides a value, return that value.  Otherwise,
// return the Bindable field of the ServiceClass.
//
// Note: enforcing that the plan belongs to the given service class is the
// responsibility of the caller.
func isPlanBindable(serviceClass *v1alpha1.ServiceClass, plan *v1alpha1.ServicePlan) bool {
	if plan.Bindable != nil {
		return *plan.Bindable
	}

	return serviceClass.Bindable
}

func (c *controller) injectBinding(binding *v1alpha1.Binding, credentials *brokerapi.Credential) error {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      binding.Spec.SecretName,
			Namespace: binding.Namespace,
		},
		Data: make(map[string][]byte),
	}

	for k, v := range *credentials {
		var err error
		secret.Data[k], err = serialize(v)
		if err != nil {
			return fmt.Errorf("Unable to serialize credential value %q: %v; %s",
				k, v, err)
		}
	}

	found := false

	_, err := c.kubeClient.Core().Secrets(binding.Namespace).Get(binding.Spec.SecretName, metav1.GetOptions{})
	if err == nil {
		found = true
	}

	if found {
		_, err = c.kubeClient.Core().Secrets(binding.Namespace).Update(secret)
	} else {
		_, err = c.kubeClient.Core().Secrets(binding.Namespace).Create(secret)
	}

	return err
}

func (c *controller) ejectBinding(binding *v1alpha1.Binding) error {
	_, err := c.kubeClient.Core().Secrets(binding.Namespace).Get(binding.Spec.SecretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}

		glog.Errorf("Error getting secret %v/%v: %v", binding.Namespace, binding.Spec.SecretName, err)
		return err
	}

	glog.V(5).Infof("Deleting secret %v/%v", binding.Namespace, binding.Spec.SecretName)
	err = c.kubeClient.Core().Secrets(binding.Namespace).Delete(binding.Spec.SecretName, &metav1.DeleteOptions{})

	return err
}

// updateBindingCondition updates the given condition for the given Binding
// with the given status, reason, and message.
func (c *controller) updateBindingCondition(
	binding *v1alpha1.Binding,
	conditionType v1alpha1.BindingConditionType,
	status v1alpha1.ConditionStatus,
	reason, message string) error {

	clone, err := api.Scheme.DeepCopy(binding)
	if err != nil {
		return err
	}
	toUpdate := clone.(*v1alpha1.Binding)

	newCondition := v1alpha1.BindingCondition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	}

	t := time.Now()

	if len(binding.Status.Conditions) == 0 {
		glog.Infof(`Setting lastTransitionTime for Binding "%v/%v" condition %q to %v`, binding.Namespace, binding.Name, conditionType, t)
		newCondition.LastTransitionTime = metav1.NewTime(t)
		toUpdate.Status.Conditions = []v1alpha1.BindingCondition{newCondition}
	} else {
		for i, cond := range binding.Status.Conditions {
			if cond.Type == conditionType {
				if cond.Status != newCondition.Status {
					glog.Infof(`Found status change for Binding "%v/%v" condition %q: %q -> %q; setting lastTransitionTime to %v`, binding.Namespace, binding.Name, conditionType, cond.Status, status, t)
					newCondition.LastTransitionTime = metav1.NewTime(time.Now())
				} else {
					newCondition.LastTransitionTime = cond.LastTransitionTime
				}

				toUpdate.Status.Conditions[i] = newCondition
				break
			}
		}
	}

	logContext := fmt.Sprintf("%v condition for Binding %v/%v to %v (Reason: %q, Message: %q)",
		conditionType, binding.Namespace, binding.Name, status, reason, message)
	glog.V(4).Infof("Updating %v", logContext)
	_, err = c.serviceCatalogClient.Bindings(binding.Namespace).UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf("Error updating %v: %v", logContext, err)
	}
	return err
}

// updateBindingFinalizers updates the given finalizers for the given Binding.
func (c *controller) updateBindingFinalizers(
	binding *v1alpha1.Binding,
	finalizers []string) error {

	// Get the latest version of the binding so that we can avoid conflicts
	// (since we have probably just updated the status of the binding and are
	// now removing the last finalizer).
	binding, err := c.serviceCatalogClient.Bindings(binding.Namespace).Get(binding.Name, metav1.GetOptions{})
	if err != nil {
		glog.Errorf("Error getting Binding %v/%v to finalize: %v", binding.Namespace, binding.Name, err)
	}

	clone, err := api.Scheme.DeepCopy(binding)
	if err != nil {
		return err
	}
	toUpdate := clone.(*v1alpha1.Binding)

	toUpdate.Finalizers = finalizers

	logContext := fmt.Sprintf("finalizers for Binding %v/%v to %v",
		binding.Namespace, binding.Name, finalizers)

	glog.V(4).Infof("Updating %v", logContext)
	_, err = c.serviceCatalogClient.Bindings(binding.Namespace).UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf("Error updating %v: %v", logContext, err)
	}
	return err
}

func (c *controller) bindingDelete(obj interface{}) {
	binding, ok := obj.(*v1alpha1.Binding)
	if binding == nil || !ok {
		return
	}

	glog.V(4).Infof("Received delete event for Binding %v/%v", binding.Namespace, binding.Name)
}

// getServiceClassPlanAndBroker is a sequence of operations that's done in couple of
// places so this method fetches the Service Class, Service Plan and creates
// a brokerClient to use for that method given an Instance.
func (c *controller) getServiceClassPlanAndBroker(instance *v1alpha1.Instance) (*v1alpha1.ServiceClass, *v1alpha1.ServicePlan, string, brokerapi.BrokerClient, error) {
	serviceClass, err := c.serviceClassLister.Get(instance.Spec.ServiceClassName)
	if err != nil {
		s := fmt.Sprintf("Instance \"%s/%s\" references a non-existent ServiceClass %q", instance.Namespace, instance.Name, instance.Spec.ServiceClassName)
		glog.Info(s)
		c.updateInstanceCondition(
			instance,
			v1alpha1.InstanceConditionReady,
			v1alpha1.ConditionFalse,
			errorNonexistentServiceClassReason,
			"The instance references a ServiceClass that does not exist. "+s,
		)
		c.recorder.Event(instance, api.EventTypeWarning, errorNonexistentServiceClassReason, s)
		return nil, nil, "", nil, err
	}

	servicePlan := findServicePlan(instance.Spec.PlanName, serviceClass.Plans)
	if servicePlan == nil {
		s := fmt.Sprintf("Instance \"%s/%s\" references a non-existent ServicePlan %q on ServiceClass %q", instance.Namespace, instance.Name, instance.Spec.PlanName, serviceClass.Name)
		glog.Warning(s)
		c.updateInstanceCondition(
			instance,
			v1alpha1.InstanceConditionReady,
			v1alpha1.ConditionFalse,
			"ReferencesNonexistentServicePlan",
			"The instance references a ServicePlan that does not exist. "+s,
		)
		c.recorder.Event(instance, api.EventTypeWarning, errorNonexistentServicePlanReason, s)
		return nil, nil, "", nil, fmt.Errorf(s)
	}

	broker, err := c.brokerLister.Get(serviceClass.BrokerName)
	if err != nil {
		s := fmt.Sprintf("Instance \"%s/%s\" references a non-existent broker %q", instance.Namespace, instance.Name, serviceClass.BrokerName)
		glog.Warning(s)
		c.updateInstanceCondition(
			instance,
			v1alpha1.InstanceConditionReady,
			v1alpha1.ConditionFalse,
			errorNonexistentBrokerReason,
			"The instance references a Broker that does not exist. "+s,
		)
		c.recorder.Event(instance, api.EventTypeWarning, errorNonexistentBrokerReason, s)
		return nil, nil, "", nil, err
	}

	username, password, err := getAuthCredentialsFromBroker(c.kubeClient, broker)
	if err != nil {
		s := fmt.Sprintf("Error getting broker auth credentials for broker %q: %s", broker.Name, err)
		glog.Info(s)
		c.updateInstanceCondition(
			instance,
			v1alpha1.InstanceConditionReady,
			v1alpha1.ConditionFalse,
			errorAuthCredentialsReason,
			"Error getting auth credentials. "+s,
		)
		c.recorder.Event(instance, api.EventTypeWarning, errorAuthCredentialsReason, s)
		return nil, nil, "", nil, err
	}

	glog.V(4).Infof("Creating client for Broker %v, URL: %v", broker.Name, broker.Spec.URL)
	brokerClient := c.brokerClientCreateFunc(broker.Name, broker.Spec.URL, username, password)
	return serviceClass, servicePlan, broker.Name, brokerClient, nil
}

// getServiceClassPlanAndBrokerForBinding is a sequence of operations that's
// done to validate service plan, service class exist, and handles creating
// a brokerclient to use for a given Instance.
func (c *controller) getServiceClassPlanAndBrokerForBinding(instance *v1alpha1.Instance, binding *v1alpha1.Binding) (*v1alpha1.ServiceClass, *v1alpha1.ServicePlan, string, brokerapi.BrokerClient, error) {
	serviceClass, err := c.serviceClassLister.Get(instance.Spec.ServiceClassName)
	if err != nil {
		s := fmt.Sprintf("Binding \"%s/%s\" references a non-existent ServiceClass %q", binding.Namespace, binding.Name, instance.Spec.ServiceClassName)
		glog.Warning(s)
		c.updateBindingCondition(
			binding,
			v1alpha1.BindingConditionReady,
			v1alpha1.ConditionFalse,
			errorNonexistentServiceClassReason,
			"The binding references a ServiceClass that does not exist. "+s,
		)
		c.recorder.Event(binding, api.EventTypeWarning, "ReferencesNonexistentServiceClass", s)
		return nil, nil, "", nil, err
	}

	servicePlan := findServicePlan(instance.Spec.PlanName, serviceClass.Plans)
	if servicePlan == nil {
		s := fmt.Sprintf("Instance \"%s/%s\" references a non-existent ServicePlan %q on ServiceClass %q", instance.Namespace, instance.Name, instance.Spec.PlanName, serviceClass.Name)
		glog.Warning(s)
		c.updateBindingCondition(
			binding,
			v1alpha1.BindingConditionReady,
			v1alpha1.ConditionFalse,
			errorNonexistentServicePlanReason,
			"The Binding references an Instance which references ServicePlan that does not exist. "+s,
		)
		c.recorder.Event(binding, api.EventTypeWarning, errorNonexistentServicePlanReason, s)
		return nil, nil, "", nil, fmt.Errorf(s)
	}

	broker, err := c.brokerLister.Get(serviceClass.BrokerName)
	if err != nil {
		s := fmt.Sprintf("Binding \"%s/%s\" references a non-existent Broker %q", binding.Namespace, binding.Name, serviceClass.BrokerName)
		glog.Warning(s)
		c.updateBindingCondition(
			binding,
			v1alpha1.BindingConditionReady,
			v1alpha1.ConditionFalse,
			errorNonexistentBrokerReason,
			"The binding references a Broker that does not exist. "+s,
		)
		c.recorder.Event(binding, api.EventTypeWarning, errorNonexistentBrokerReason, s)
		return nil, nil, "", nil, err
	}

	username, password, err := getAuthCredentialsFromBroker(c.kubeClient, broker)
	if err != nil {
		s := fmt.Sprintf("Error getting broker auth credentials for broker %q: %s", broker.Name, err)
		glog.Warning(s)
		c.updateBindingCondition(
			binding,
			v1alpha1.BindingConditionReady,
			v1alpha1.ConditionFalse,
			errorAuthCredentialsReason,
			"Error getting auth credentials. "+s,
		)
		c.recorder.Event(binding, api.EventTypeWarning, errorAuthCredentialsReason, s)
		return nil, nil, "", nil, err
	}

	glog.V(4).Infof("Creating client for Broker %v, URL: %v", broker.Name, broker.Spec.URL)
	brokerClient := c.brokerClientCreateFunc(broker.Name, broker.Spec.URL, username, password)
	return serviceClass, servicePlan, broker.Name, brokerClient, nil
}

// Broker utility methods - move?

// getAuthCredentialsFromBroker returns the auth credentials, if any,
// contained in the secret referenced in the Broker's AuthSecret field, or
// returns an error. If the AuthSecret field is nil, empty values are
// returned.
func getAuthCredentialsFromBroker(client kubernetes.Interface, broker *v1alpha1.Broker) (username, password string, err error) {
	if broker.Spec.AuthSecret == nil {
		return "", "", nil
	}

	authSecret, err := client.Core().Secrets(broker.Spec.AuthSecret.Namespace).Get(broker.Spec.AuthSecret.Name, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}

	usernameBytes, ok := authSecret.Data["username"]
	if !ok {
		return "", "", fmt.Errorf("auth secret didn't contain username")
	}

	passwordBytes, ok := authSecret.Data["password"]
	if !ok {
		return "", "", fmt.Errorf("auth secret didn't contain password")
	}

	return string(usernameBytes), string(passwordBytes), nil
}

// convertCatalog converts a service broker catalog into an array of ServiceClasses
func convertCatalog(in *brokerapi.Catalog) ([]*v1alpha1.ServiceClass, error) {
	ret := make([]*v1alpha1.ServiceClass, len(in.Services))
	for i, svc := range in.Services {
		plans, err := convertServicePlans(svc.Plans)
		if err != nil {
			return nil, err
		}
		ret[i] = &v1alpha1.ServiceClass{
			Bindable:      svc.Bindable,
			Plans:         plans,
			PlanUpdatable: svc.PlanUpdateable,
			ExternalID:    svc.ID,
			AlphaTags:     svc.Tags,
			Description:   svc.Description,
			AlphaRequires: svc.Requires,
		}

		if svc.Metadata != nil {
			metadata, err := json.Marshal(svc.Metadata)
			if err != nil {
				err = fmt.Errorf("Failed to marshal metadata\n%+v\n %v", svc.Metadata, err)
				glog.Error(err)
				return nil, err
			}
			ret[i].ExternalMetadata = &runtime.RawExtension{Raw: metadata}
		}

		ret[i].SetName(svc.Name)
	}
	return ret, nil
}

func convertServicePlans(plans []brokerapi.ServicePlan) ([]v1alpha1.ServicePlan, error) {
	ret := make([]v1alpha1.ServicePlan, len(plans))
	for i := range plans {
		ret[i] = v1alpha1.ServicePlan{
			Name:        plans[i].Name,
			ExternalID:  plans[i].ID,
			Free:        plans[i].Free,
			Description: plans[i].Description,
		}
		if plans[i].Bindable != nil {
			b := *plans[i].Bindable
			ret[i].Bindable = &b
		}

		if plans[i].Metadata != nil {
			metadata, err := json.Marshal(plans[i].Metadata)
			if err != nil {
				err = fmt.Errorf("Failed to marshal metadata\n%+v\n %v", plans[i].Metadata, err)
				glog.Error(err)
				return nil, err
			}
			ret[i].ExternalMetadata = &runtime.RawExtension{Raw: metadata}
		}

	}
	return ret, nil
}

func unmarshalParameters(in []byte) (map[string]interface{}, error) {
	parameters := make(map[string]interface{})
	if len(in) > 0 {
		if err := yaml.Unmarshal(in, &parameters); err != nil {
			return parameters, err
		}
	}
	return parameters, nil
}

// isInstanceReady returns whether the given instance has a ready condition
// with status true.
func isInstanceReady(instance *v1alpha1.Instance) bool {
	for _, cond := range instance.Status.Conditions {
		if cond.Type == v1alpha1.InstanceConditionReady {
			return cond.Status == v1alpha1.ConditionTrue
		}
	}

	return false
}

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
	"fmt"
	"time"

	"github.com/golang/glog"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	"github.com/kubernetes-incubator/service-catalog/pkg/brokerapi"
	scfeatures "github.com/kubernetes-incubator/service-catalog/pkg/features"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/tools/cache"
)

// ServiceInstance handlers and control-loop

func (c *controller) instanceAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}

	c.instanceQueue.Add(key)
}

func (c *controller) instanceUpdate(oldObj, newObj interface{}) {
	// Instances with ongoing asynchronous operations will be manually added
	// to the polling queue by the reconciler. They should be ignored here in
	// order to enforce polling rate-limiting.
	instance := newObj.(*v1alpha1.ServiceInstance)
	if !instance.Status.AsyncOpInProgress {
		c.instanceAdd(newObj)
	}
}

func (c *controller) instanceDelete(obj interface{}) {
	instance, ok := obj.(*v1alpha1.ServiceInstance)
	if instance == nil || !ok {
		return
	}

	glog.V(4).Infof("Received delete event for ServiceInstance %v/%v; no further processing will occur", instance.Namespace, instance.Name)
}

// Async operations on instances have a somewhat convoluted flow in order to
// ensure that only a single goroutine works on an instance at any given time.
// The flow is:
//
// 1.  When the controller wants to begin polling the state of an operation on
//     an instance, it calls its beginPollingServiceInstance method (or
//     calls continuePollingServiceInstance, an alias of that method)
// 2.  begin/continuePollingServiceInstance do a rate-limited add to the polling queue
// 3.  the pollingQueue calls requeueServiceInstanceForPoll, which adds the instance's
//     key to the instance work queue
// 4.  the worker servicing the instance polling queue forgets the instances key,
//     requiring the controller to call continuePollingServiceInstance if additional
//     work is needed.
// 5.  the instance work queue is the single work queue that actually services
//     instances by calling reconcileServiceInstance
// 6.  when an asynchronous operation is completed, the controller calls
//     finishPollingServiceInstance to forget the instance from the polling queue

// requeueServiceInstanceForPoll adds the given instance key to the controller's work
// queue for instances.  It is used to trigger polling for the status of an
// async operation on and instance and is called by the worker servicing the
// instance polling queue.  After requeueServiceInstanceForPoll exits, the worker
// forgets the key from the polling queue, so the controller must call
// continuePollingServiceInstance if the instance requires additional polling.
func (c *controller) requeueServiceInstanceForPoll(key string) error {
	c.instanceQueue.Add(key)

	return nil
}

// beginPollingServiceInstance does a rate-limited add of the key for the given
// instance to the controller's instance polling queue.
func (c *controller) beginPollingServiceInstance(instance *v1alpha1.ServiceInstance) error {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(instance)
	if err != nil {
		glog.Errorf("Couldn't create a key for object %+v: %v", instance, err)
		return fmt.Errorf("Couldn't create a key for object %+v: %v", instance, err)
	}

	c.pollingQueue.AddRateLimited(key)

	return nil
}

// continuePollingServiceInstance does a rate-limited add of the key for the given
// instance to the controller's instance polling queue.
func (c *controller) continuePollingServiceInstance(instance *v1alpha1.ServiceInstance) error {
	return c.beginPollingServiceInstance(instance)
}

// finishPollingServiceInstance removes the instance's key from the controller's instance
// polling queue.
func (c *controller) finishPollingServiceInstance(instance *v1alpha1.ServiceInstance) error {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(instance)
	if err != nil {
		glog.Errorf("Couldn't create a key for object %+v: %v", instance, err)
		return fmt.Errorf("Couldn't create a key for object %+v: %v", instance, err)
	}

	c.pollingQueue.Forget(key)

	return nil
}

func (c *controller) reconcileServiceInstanceKey(key string) error {
	// For namespace-scoped resources, SplitMetaNamespaceKey splits the key
	// i.e. "namespace/name" into two separate strings
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	instance, err := c.instanceLister.ServiceInstances(namespace).Get(name)
	if errors.IsNotFound(err) {
		glog.Infof("Not doing work for ServiceInstance %v because it has been deleted", key)
		return nil
	}
	if err != nil {
		glog.Errorf("Unable to retrieve ServiceInstance %v from store: %v", key, err)
		return err
	}

	return c.reconcileServiceInstance(instance)
}

// reconcileServiceInstanceDelete is responsible for handling any instance whose
// deletion timestamp is set.
//
// TODO: may change when orphan mitigation is implemented.
func (c *controller) reconcileServiceInstanceDelete(instance *v1alpha1.ServiceInstance) error {
	// nothing to do...
	if instance.DeletionTimestamp == nil {
		return nil
	}

	// Determine if any credentials exist for this instance.  We don't want to
	// delete the instance if there are any associated creds
	credentialsLister := c.bindingLister.ServiceInstanceCredentials(instance.Namespace)

	selector := labels.NewSelector()
	credentialsList, err := credentialsLister.List(selector)
	if err != nil {
		return err
	}
	for _, credentials := range credentialsList {
		if instance.Name == credentials.Spec.ServiceInstanceRef.Name {

			// found credentials, block the deletion until they are removed
			clone, err := api.Scheme.DeepCopy(instance)
			if err != nil {
				return err
			}
			toUpdate := clone.(*v1alpha1.ServiceInstance)

			s := fmt.Sprintf(
				"Delete instance %v/%v blocked by existing ServiceInstanceCredentials associated with this instance.  All credentials must be removed first.",
				instance.Namespace,
				instance.Name)
			glog.Warning(s)

			setServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionFalse,
				errorDeprovisionCalledReason,
				"Delete instance failed. "+s)
			c.updateServiceInstanceStatus(toUpdate)
			c.recorder.Event(instance, api.EventTypeWarning, errorDeprovisionCalledReason, s)
			return nil
		}
	}

	finalizerToken := v1alpha1.FinalizerServiceCatalog
	finalizers := sets.NewString(instance.Finalizers...)
	if !finalizers.Has(finalizerToken) {
		return nil
	}

	// If there is no op in progress, and the instance was never provisioned,
	// we can just delete. this can happen if the service class name
	// referenced never existed.
	//
	// TODO: the above logic changes slightly once we handle orphan
	// mitigation.
	if !instance.Status.AsyncOpInProgress && instance.Status.ReconciledGeneration == 0 {
		finalizers.Delete(finalizerToken)
		// Clear the finalizer
		return c.updateServiceInstanceFinalizers(instance, finalizers.List())
	}

	// All updates not having a DeletingTimestamp will have been handled above
	// and returned early. If we reach this point, we're dealing with an update
	// that's actually a soft delete-- i.e. we have some finalization to do.
	serviceClass, servicePlan, brokerName, brokerClient, err := c.getServiceClassPlanAndServiceBroker(instance)
	if err != nil {
		return err
	}

	// we will definitely update the instance's status - make a deep copy now
	// for use later in this method.
	clone, err := api.Scheme.DeepCopy(instance)
	if err != nil {
		return err
	}
	toUpdate := clone.(*v1alpha1.ServiceInstance)

	glog.V(4).Infof("Finalizing ServiceInstance %v/%v", instance.Namespace, instance.Name)

	request := &osb.DeprovisionRequest{
		InstanceID:        instance.Spec.ExternalID,
		ServiceID:         serviceClass.ExternalID,
		PlanID:            servicePlan.ExternalID,
		AcceptsIncomplete: true,
	}

	if utilfeature.DefaultFeatureGate.Enabled(scfeatures.OriginatingIdentity) {
		originatingIdentity, err := buildOriginatingIdentity(instance.Spec.UserInfo)
		if err != nil {
			s := fmt.Sprintf(`Error building originating identity headers for deprovisioning ServiceInstance "%v/%v": %v`, instance.Namespace, instance.Name, err)
			glog.Warning(s)

			setServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionFalse,
				errorWithOriginatingIdentity,
				s,
			)
			c.updateServiceInstanceStatus(toUpdate)

			c.recorder.Event(instance, api.EventTypeWarning, errorWithOriginatingIdentity, s)
			return err
		}
		request.OriginatingIdentity = originatingIdentity
	}

	// If the instance is not failed, deprovision it at the broker.
	if !isServiceInstanceFailed(instance) {
		// it is arguable we should perform an extract-method refactor on this
		// code block

		now := metav1.Now()

		glog.V(4).Infof("Deprovisioning ServiceInstance %v/%v of ServiceClass %v at ServiceBroker %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName)
		response, err := brokerClient.DeprovisionInstance(request)
		if err != nil {
			httpErr, isError := osb.IsHTTPError(err)
			if isError {
				s := fmt.Sprintf(
					"Error deprovisioning ServiceInstance \"%s/%s\" of ServiceClass %q at ServiceBroker %q with status code %d: ErrorMessage: %v, Description: %v",
					instance.Namespace,
					instance.Name,
					serviceClass.Name,
					brokerName,
					httpErr.StatusCode,
					httpErr.ErrorMessage,
					httpErr.Description,
				)
				glog.Warning(s)

				setServiceInstanceCondition(
					toUpdate,
					v1alpha1.ServiceInstanceConditionReady,
					v1alpha1.ConditionUnknown,
					errorDeprovisionCalledReason,
					"Deprovision call failed. "+s)
				setServiceInstanceCondition(
					toUpdate,
					v1alpha1.ServiceInstanceConditionFailed,
					v1alpha1.ConditionTrue,
					errorDeprovisionCalledReason,
					s,
				)
				c.updateServiceInstanceStatus(toUpdate)
				c.recorder.Event(instance, api.EventTypeWarning, errorDeprovisionCalledReason, s)

				// Return nil so that the reconciler does not retry the deprovision
				return nil
			}

			s := fmt.Sprintf(
				"Error deprovisioning ServiceInstance \"%s/%s\" of ServiceClass %q at ServiceBroker %q with status code %d: ErrorMessage: %v, Description: %v",
				instance.Namespace,
				instance.Name,
				serviceClass.Name,
				brokerName,
				httpErr.StatusCode,
				httpErr.ErrorMessage,
				httpErr.Description,
			)
			glog.Warning(s)

			setServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionUnknown,
				errorDeprovisionCalledReason,
				"Deprovision call failed. "+s)

			c.recorder.Event(instance, api.EventTypeWarning, errorDeprovisionCalledReason, s)

			if instance.Status.OperationStartTime == nil {
				toUpdate.Status.OperationStartTime = &now
			} else if !time.Now().Before(instance.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
				s := fmt.Sprintf(`Stopping reconciliation retries on ServiceInstance "%v/%v" because too much time has elapsed`, instance.Namespace, instance.Name)
				glog.Info(s)
				c.recorder.Event(instance, api.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)
				setServiceInstanceCondition(toUpdate,
					v1alpha1.ServiceInstanceConditionFailed,
					v1alpha1.ConditionTrue,
					errorReconciliationRetryTimeoutReason,
					s)
				toUpdate.Status.OperationStartTime = nil
				c.updateServiceInstanceStatus(toUpdate)
				return nil
			}

			c.updateServiceInstanceStatus(toUpdate)
			return err
		}

		if response.Async {
			glog.V(5).Infof("Received asynchronous de-provisioning response for ServiceInstance %v/%v of ServiceClass %v at ServiceBroker %v: response: %+v", instance.Namespace, instance.Name, serviceClass.Name, brokerName, response)
			if response.OperationKey != nil && *response.OperationKey != "" {
				key := string(*response.OperationKey)
				toUpdate.Status.LastOperation = &key
			}

			toUpdate.Status.OperationStartTime = &now

			// Tag this instance as having an ongoing async operation so we can enforce
			// no other operations against it can start.
			toUpdate.Status.AsyncOpInProgress = true

			setServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionFalse,
				asyncDeprovisioningReason,
				asyncDeprovisioningMessage,
			)
			err := c.updateServiceInstanceStatus(toUpdate)
			if err != nil {
				return err
			}

			err = c.beginPollingServiceInstance(instance)
			if err != nil {
				return err
			}

			c.recorder.Eventf(instance, api.EventTypeNormal, asyncDeprovisioningReason, asyncDeprovisioningMessage)

			return nil
		}

		glog.V(5).Infof("Deprovision call to broker succeeded for ServiceInstance %v/%v, finalizing", instance.Namespace, instance.Name)

		toUpdate.Status.OperationStartTime = nil

		setServiceInstanceCondition(
			toUpdate,
			v1alpha1.ServiceInstanceConditionReady,
			v1alpha1.ConditionFalse,
			successDeprovisionReason,
			successDeprovisionMessage,
		)
		err = c.updateServiceInstanceStatus(toUpdate)
		if err != nil {
			return err
		}

		c.recorder.Event(instance, api.EventTypeNormal, successDeprovisionReason, successDeprovisionMessage)
		glog.V(5).Infof("Successfully deprovisioned ServiceInstance %v/%v of ServiceClass %v at ServiceBroker %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName)
		// In the success case, fall through to clearing the finalizer.
	}

	glog.V(5).Infof("Clearing catalog finalizer from ServiceInstance %v/%v", instance.Namespace, instance.Name)

	// Clear the finalizer
	finalizers.Delete(v1alpha1.FinalizerServiceCatalog)
	return c.updateServiceInstanceFinalizers(instance, finalizers.List())
}

// isServiceInstanceFailed returns whether the instance has a failed condition with
// status true.
func isServiceInstanceFailed(instance *v1alpha1.ServiceInstance) bool {
	for _, condition := range instance.Status.Conditions {
		if condition.Type == v1alpha1.ServiceInstanceConditionFailed && condition.Status == v1alpha1.ConditionTrue {
			return true
		}
	}

	return false
}

// reconcileServiceInstance is the control-loop for reconciling Instances. An
// error is returned to indicate that the binding has not been fully
// processed and should be resubmitted at a later time.
func (c *controller) reconcileServiceInstance(instance *v1alpha1.ServiceInstance) error {
	// Currently, we only set a failure condition if the initial provision
	// call fails, so if that condition is set, we only need to remove the
	// finalizer from the instance. We will need to reevaluate this logic as
	// we make any changes to capture permanent failure in new cases.
	//
	// TODO: this will change once we fully implement orphan mitigation, see:
	// https://github.com/kubernetes-incubator/service-catalog/issues/988
	if isServiceInstanceFailed(instance) && instance.ObjectMeta.DeletionTimestamp == nil {
		glog.V(4).Infof(
			"Not processing event for ServiceInstance %v/%v because status showed that it has failed",
			instance.Namespace,
			instance.Name,
		)
		return nil
	}

	if instance.Status.AsyncOpInProgress {
		return c.pollServiceInstanceInternal(instance)
	}

	// If there's no async op in progress, determine whether there is a new
	// generation of the object. If the instance's generation does not match
	// the reconciled generation, then there is a new generation, indicating
	// that changes have been made to the instance's spec. If there is an
	// async op in progress, we need to keep polling, hence do not bail if
	// there is not a new generation.
	//
	// We only do this if the deletion timestamp is nil, because the deletion
	// timestamp changes the object's state in a way that we must reconcile,
	// but does not affect the generation.
	//
	// Note: currently the instance spec is immutable because we do not yet
	// support plan or parameter updates.  This logic is currently meant only
	// to facilitate re-trying provision requests where there was a problem
	// communicating with the broker.  In the future the same logic will
	// result in an instance that requires update being processed by the
	// controller.
	if instance.DeletionTimestamp == nil {
		if instance.Status.ReconciledGeneration == instance.Generation {
			glog.V(4).Infof(
				"Not processing event for ServiceInstance %v/%v because reconciled generation showed there is no work to do",
				instance.Namespace,
				instance.Name,
			)
			return nil
		}
	}

	glog.V(4).Infof("Processing ServiceInstance %v/%v", instance.Namespace, instance.Name)

	if instance.ObjectMeta.DeletionTimestamp != nil {
		return c.reconcileServiceInstanceDelete(instance)
	}

	glog.V(4).Infof("Adding/Updating ServiceInstance %v/%v", instance.Namespace, instance.Name)

	serviceClass, servicePlan, brokerName, brokerClient, err := c.getServiceClassPlanAndServiceBroker(instance)
	if err != nil {
		return err
	}

	// we will definitely update the instance's status - make a deep copy now
	// for use later in this method.
	clone, err := api.Scheme.DeepCopy(instance)
	if err != nil {
		return err
	}
	toUpdate := clone.(*v1alpha1.ServiceInstance)

	var parameters map[string]interface{}
	if instance.Spec.Parameters != nil || instance.Spec.ParametersFrom != nil {
		parameters, err = buildParameters(c.kubeClient, instance.Namespace, instance.Spec.ParametersFrom, instance.Spec.Parameters)
		if err != nil {
			s := fmt.Sprintf("Failed to prepare ServiceInstance parameters\n%s\n %s", instance.Spec.Parameters, err)
			glog.Warning(s)

			setServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionFalse,
				errorWithParameters,
				s,
			)
			c.updateServiceInstanceStatus(toUpdate)

			c.recorder.Event(instance, api.EventTypeWarning, errorWithParameters, s)
			return err
		}
	}

	ns, err := c.kubeClient.Core().Namespaces().Get(instance.Namespace, metav1.GetOptions{})
	if err != nil {
		s := fmt.Sprintf("Failed to get namespace %q during instance create: %s", instance.Namespace, err)
		glog.Info(s)

		setServiceInstanceCondition(
			toUpdate,
			v1alpha1.ServiceInstanceConditionReady,
			v1alpha1.ConditionFalse,
			errorFindingNamespaceServiceInstanceReason,
			"Error finding namespace for instance. "+s,
		)
		c.updateServiceInstanceStatus(toUpdate)

		c.recorder.Event(instance, api.EventTypeWarning, errorFindingNamespaceServiceInstanceReason, s)
		return err
	}

	request := &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        instance.Spec.ExternalID,
		ServiceID:         serviceClass.ExternalID,
		PlanID:            servicePlan.ExternalID,
		Parameters:        parameters,
		OrganizationGUID:  string(ns.UID),
		SpaceGUID:         string(ns.UID),
	}

	// osb client handles whether or not to really send this based
	// on the version of the client.
	request.Context = map[string]interface{}{
		"platform":  brokerapi.ContextProfilePlatformKubernetes,
		"namespace": instance.Namespace,
	}

	if utilfeature.DefaultFeatureGate.Enabled(scfeatures.OriginatingIdentity) {
		originatingIdentity, err := buildOriginatingIdentity(instance.Spec.UserInfo)
		if err != nil {
			s := fmt.Sprintf(`Error building originating identity headers for provisioning ServiceInstance "%v/%v": %v`, instance.Namespace, instance.Name, err)
			glog.Warning(s)

			setServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionFalse,
				errorWithOriginatingIdentity,
				s,
			)
			c.updateServiceInstanceStatus(toUpdate)

			c.recorder.Event(instance, api.EventTypeWarning, errorWithOriginatingIdentity, s)
			return err
		}
		request.OriginatingIdentity = originatingIdentity
	}

	now := metav1.Now()

	glog.V(4).Infof("Provisioning a new ServiceInstance %v/%v of ServiceClass %v at ServiceBroker %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName)
	response, err := brokerClient.ProvisionInstance(request)
	if err != nil {
		// There are two buckets of errors to handle:
		// 1.  Errors that represent a failure response from the broker
		// 2.  All other errors
		if httpErr, ok := osb.IsHTTPError(err); ok {
			// An error from the broker represents a permanent failure and
			// should not be retried; set the Failed condition.
			s := fmt.Sprintf("Error provisioning ServiceInstance \"%s/%s\" of ServiceClass %q at ServiceBroker %q: %s", instance.Namespace, instance.Name, serviceClass.Name, brokerName, httpErr)
			glog.Warning(s)

			setServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionFailed,
				v1alpha1.ConditionTrue,
				"ServiceBrokerReturnedFailure",
				s)
			setServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionFalse,
				errorProvisionCallFailedReason,
				"ServiceBroker returned a failure for provision call; operation will not be retried: "+s)
			toUpdate.Status.OperationStartTime = nil
			toUpdate.Status.ReconciledGeneration = toUpdate.Generation
			err := c.updateServiceInstanceStatus(toUpdate)
			if err != nil {
				return err
			}

			c.recorder.Event(instance, api.EventTypeWarning, errorProvisionCallFailedReason, s)
			return nil
		}

		s := fmt.Sprintf("Error provisioning ServiceInstance \"%s/%s\" of ServiceClass %q at ServiceBroker %q: %s", instance.Namespace, instance.Name, serviceClass.Name, brokerName, err)
		glog.Warning(s)

		setServiceInstanceCondition(
			toUpdate,
			v1alpha1.ServiceInstanceConditionReady,
			v1alpha1.ConditionFalse,
			errorErrorCallingProvisionReason,
			"Provision call failed and will be retried: "+s)
		c.recorder.Event(instance, api.EventTypeWarning, errorErrorCallingProvisionReason, s)

		if instance.Status.OperationStartTime == nil {
			toUpdate.Status.OperationStartTime = &now
		} else if !time.Now().Before(instance.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
			s := fmt.Sprintf(`Stopping reconciliation retries on ServiceInstance "%v/%v" because too much time has elapsed`, instance.Namespace, instance.Name)
			glog.Info(s)
			c.recorder.Event(instance, api.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)
			setServiceInstanceCondition(toUpdate,
				v1alpha1.ServiceInstanceConditionFailed,
				v1alpha1.ConditionTrue,
				errorReconciliationRetryTimeoutReason,
				s)
			toUpdate.Status.OperationStartTime = nil
			toUpdate.Status.ReconciledGeneration = toUpdate.Generation
			c.updateServiceInstanceStatus(toUpdate)
			return nil
		}

		c.updateServiceInstanceStatus(toUpdate)

		return err
	}

	if response.DashboardURL != nil && *response.DashboardURL != "" {
		url := *response.DashboardURL
		toUpdate.Status.DashboardURL = &url
	}

	// ServiceBroker can return either a synchronous or asynchronous
	// response, if the response is StatusAccepted it's an async
	// and we need to add it to the polling queue. ServiceBroker can
	// optionally return 'Operation' that will then need to be
	// passed back to the broker during polling of last_operation.
	if response.Async {
		glog.V(5).Infof("Received asynchronous provisioning response for ServiceInstance %v/%v of ServiceClass %v at ServiceBroker %v: response: %+v", instance.Namespace, instance.Name, serviceClass.Name, brokerName, response)
		if response.OperationKey != nil && *response.OperationKey != "" {
			key := string(*response.OperationKey)
			toUpdate.Status.LastOperation = &key
		}

		// Tag this instance as having an ongoing async operation so we can enforce
		// no other operations against it can start.
		toUpdate.Status.AsyncOpInProgress = true

		toUpdate.Status.OperationStartTime = &now

		setServiceInstanceCondition(
			toUpdate,
			v1alpha1.ServiceInstanceConditionReady,
			v1alpha1.ConditionFalse,
			asyncProvisioningReason,
			asyncProvisioningMessage,
		)
		c.updateServiceInstanceStatus(toUpdate)

		c.recorder.Eventf(instance, api.EventTypeNormal, asyncProvisioningReason, asyncProvisioningMessage)

		if err := c.beginPollingServiceInstance(instance); err != nil {
			return err
		}
	} else {
		glog.V(5).Infof("Successfully provisioned ServiceInstance %v/%v of ServiceClass %v at ServiceBroker %v: response: %+v", instance.Namespace, instance.Name, serviceClass.Name, brokerName, response)

		toUpdate.Status.OperationStartTime = nil

		// Create/Update for Instance has completed successfully, so set
		// Status.ReconciledGeneration to the Generation used.
		toUpdate.Status.ReconciledGeneration = toUpdate.Generation

		// TODO: process response
		setServiceInstanceCondition(
			toUpdate,
			v1alpha1.ServiceInstanceConditionReady,
			v1alpha1.ConditionTrue,
			successProvisionReason,
			successProvisionMessage,
		)
		c.updateServiceInstanceStatus(toUpdate)

		c.recorder.Eventf(instance, api.EventTypeNormal, successProvisionReason, successProvisionMessage)
	}
	return nil
}

func (c *controller) pollServiceInstanceInternal(instance *v1alpha1.ServiceInstance) error {
	glog.V(4).Infof("Processing ServiceInstance %v/%v", instance.Namespace, instance.Name)

	serviceClass, servicePlan, brokerName, brokerClient, err := c.getServiceClassPlanAndServiceBroker(instance)
	if err != nil {
		return err
	}
	return c.pollServiceInstance(serviceClass, servicePlan, brokerName, brokerClient, instance)
}

func (c *controller) pollServiceInstance(serviceClass *v1alpha1.ServiceClass, servicePlan *v1alpha1.ServicePlan, brokerName string, brokerClient osb.Client, instance *v1alpha1.ServiceInstance) error {
	// There are some conditions that are different if we're
	// deleting, this is more readable than checking the
	// timestamps in various places.
	deleting := false
	if instance.DeletionTimestamp != nil {
		deleting = true
	}

	// OperationStartTime must be set because we are polling an in-progress
	// operation. If it is not set, this is a logical error. Let's bail out.
	if instance.Status.OperationStartTime == nil {
		clone, err := api.Scheme.DeepCopy(instance)
		if err != nil {
			return err
		}
		toUpdate := clone.(*v1alpha1.ServiceInstance)
		s := fmt.Sprintf(`Stopping reconciliation retries on ServiceInstance "%v/%v" because the operation start time is not set`, instance.Namespace, instance.Name)
		glog.Info(s)
		c.recorder.Event(instance, api.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)
		setServiceInstanceCondition(toUpdate,
			v1alpha1.ServiceInstanceConditionFailed,
			v1alpha1.ConditionTrue,
			errorReconciliationRetryTimeoutReason,
			s)
		toUpdate.Status.OperationStartTime = nil
		toUpdate.Status.ReconciledGeneration = toUpdate.Generation
		if err := c.updateServiceInstanceStatus(toUpdate); err != nil {
			return err
		}
		return c.finishPollingServiceInstance(instance)
	}

	request := &osb.LastOperationRequest{
		InstanceID: instance.Spec.ExternalID,
		ServiceID:  &serviceClass.ExternalID,
		PlanID:     &servicePlan.ExternalID,
	}
	if instance.Status.LastOperation != nil && *instance.Status.LastOperation != "" {
		key := osb.OperationKey(*instance.Status.LastOperation)
		request.OperationKey = &key
	}

	if utilfeature.DefaultFeatureGate.Enabled(scfeatures.OriginatingIdentity) {
		originatingIdentity, err := buildOriginatingIdentity(instance.Spec.UserInfo)
		if err != nil {
			s := fmt.Sprintf(`Error building originating identity headers for polling last operation of ServiceInstance "%v/%v": %v`, instance.Namespace, instance.Name, err)
			glog.Warning(s)

			c.recorder.Event(instance, api.EventTypeWarning, errorWithOriginatingIdentity, s)
			return err
		}
		request.OriginatingIdentity = originatingIdentity
	}

	glog.V(5).Infof("Polling last operation on ServiceInstance %v/%v", instance.Namespace, instance.Name)

	response, err := brokerClient.PollLastOperation(request)
	if err != nil {
		// If the operation was for delete and we receive a http.StatusGone,
		// this is considered a success as per the spec, so mark as deleted
		// and remove any finalizers.
		if osb.IsGoneError(err) && deleting {
			clone, err := api.Scheme.DeepCopy(instance)
			if err != nil {
				return err
			}
			toUpdate := clone.(*v1alpha1.ServiceInstance)

			toUpdate.Status.AsyncOpInProgress = false
			toUpdate.Status.OperationStartTime = nil
			c.updateServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionFalse,
				successDeprovisionReason,
				successDeprovisionMessage,
			)

			// Clear the finalizer
			if finalizers := sets.NewString(toUpdate.Finalizers...); finalizers.Has(v1alpha1.FinalizerServiceCatalog) {
				finalizers.Delete(v1alpha1.FinalizerServiceCatalog)
				c.updateServiceInstanceFinalizers(toUpdate, finalizers.List())
			}

			c.recorder.Event(instance, api.EventTypeNormal, successDeprovisionReason, successDeprovisionMessage)
			glog.V(5).Infof("Successfully deprovisioned ServiceInstance %v/%v of ServiceClass %v at ServiceBroker %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName)

			err = c.finishPollingServiceInstance(instance)
			if err != nil {
				return err
			}
			return nil
		}

		// We got some kind of error from the broker.  While polling last
		// operation, this represents an invalid response and we should
		// continue polling last operation.
		//
		// The ready condition on the instance should already have
		// condition false; it should be sufficient to create an event for
		// the instance.
		errText := ""
		httpErr, isError := osb.IsHTTPError(err)
		if isError {
			errText = fmt.Sprintf("Status code: %d; ErrorMessage: %q; description: %q", httpErr.StatusCode, httpErr.ErrorMessage, httpErr.Description)
		} else {
			errText = err.Error()
		}

		s := fmt.Sprintf("Error polling last operation for instance %v/%v: %v", instance.Namespace, instance.Name, errText)
		glog.V(4).Info(s)
		c.recorder.Event(instance, api.EventTypeWarning, errorPollingLastOperationReason, s)

		if !time.Now().Before(instance.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
			clone, err := api.Scheme.DeepCopy(instance)
			if err != nil {
				return err
			}
			toUpdate := clone.(*v1alpha1.ServiceInstance)
			s := fmt.Sprintf(`Stopping reconciliation retries on ServiceInstance "%v/%v" because too much time has elapsed`, instance.Namespace, instance.Name)
			glog.Info(s)
			c.recorder.Event(instance, api.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)
			setServiceInstanceCondition(toUpdate,
				v1alpha1.ServiceInstanceConditionFailed,
				v1alpha1.ConditionTrue,
				errorReconciliationRetryTimeoutReason,
				s)
			toUpdate.Status.OperationStartTime = nil
			toUpdate.Status.ReconciledGeneration = toUpdate.Generation
			if err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}
			return c.finishPollingServiceInstance(instance)
		}

		return c.continuePollingServiceInstance(instance)
	}

	glog.V(4).Infof("Poll for %v/%v returned %q : %q", instance.Namespace, instance.Name, response.State, response.Description)

	switch response.State {
	case osb.StateInProgress:
		// if the description is non-nil, then update the instance condition with it
		if response.Description != nil {
			// The way the worker keeps on requeueing is by returning an error, so
			// we need to keep on polling.
			clone, err := api.Scheme.DeepCopy(instance)
			if err != nil {
				return err
			}
			toUpdate := clone.(*v1alpha1.ServiceInstance)
			toUpdate.Status.AsyncOpInProgress = true

			var message string
			var reason string
			if deleting {
				reason = asyncDeprovisioningReason
				message = asyncDeprovisioningMessage
			} else {
				reason = asyncProvisioningReason
				message = asyncProvisioningMessage
			}

			if response.Description != nil {
				message = fmt.Sprintf("%s (%s)", message, *response.Description)
			}
			c.updateServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionFalse,
				reason,
				message,
			)
		}

		if !time.Now().Before(instance.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
			clone, err := api.Scheme.DeepCopy(instance)
			if err != nil {
				return err
			}
			toUpdate := clone.(*v1alpha1.ServiceInstance)
			s := fmt.Sprintf(`Stopping reconciliation retries on ServiceInstance "%v/%v" because too much time has elapsed`, instance.Namespace, instance.Name)
			glog.Info(s)
			c.recorder.Event(instance, api.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)
			setServiceInstanceCondition(toUpdate,
				v1alpha1.ServiceInstanceConditionFailed,
				v1alpha1.ConditionTrue,
				errorReconciliationRetryTimeoutReason,
				s)
			toUpdate.Status.AsyncOpInProgress = false
			toUpdate.Status.OperationStartTime = nil
			toUpdate.Status.ReconciledGeneration = toUpdate.Generation
			if err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}

			return c.finishPollingServiceInstance(instance)
		}

		err = c.continuePollingServiceInstance(instance)
		if err != nil {
			return err
		}
		glog.V(4).Infof("last operation not completed (still in progress) for %v/%v", instance.Namespace, instance.Name)
	case osb.StateSucceeded:
		// Update the instance to reflect that an async operation is no longer
		// in progress.
		clone, err := api.Scheme.DeepCopy(instance)
		if err != nil {
			return err
		}
		toUpdate := clone.(*v1alpha1.ServiceInstance)
		toUpdate.Status.AsyncOpInProgress = false
		toUpdate.Status.OperationStartTime = nil

		// If we were asynchronously deleting a Service Instance, finish
		// the finalizers.
		if deleting {
			err := c.updateServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionFalse,
				successDeprovisionReason,
				successDeprovisionMessage,
			)
			if err != nil {
				return err
			}

			// Clear the finalizer
			if finalizers := sets.NewString(toUpdate.Finalizers...); finalizers.Has(v1alpha1.FinalizerServiceCatalog) {
				finalizers.Delete(v1alpha1.FinalizerServiceCatalog)
				c.updateServiceInstanceFinalizers(toUpdate, finalizers.List())
			}
			c.recorder.Event(instance, api.EventTypeNormal, successDeprovisionReason, successDeprovisionMessage)
			glog.V(5).Infof("Successfully deprovisioned ServiceInstance %v/%v of ServiceClass %v at ServiceBroker %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName)
		} else {
			// Create/Update for InstanceCredential has completed successfully,
			// so set Status.ReconciledGeneration to the Generation used.
			toUpdate.Status.ReconciledGeneration = toUpdate.Generation

			c.updateServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionTrue,
				successProvisionReason,
				successProvisionMessage,
			)
		}

		err = c.finishPollingServiceInstance(instance)
		if err != nil {
			return err
		}
	case osb.StateFailed:
		description := ""
		if response.Description != nil {
			description = *response.Description
		}
		s := fmt.Sprintf("Error deprovisioning ServiceInstance \"%s/%s\" of ServiceClass %q at ServiceBroker %q: %q", instance.Namespace, instance.Name, serviceClass.Name, brokerName, description)

		clone, err := api.Scheme.DeepCopy(instance)
		if err != nil {
			return err
		}
		toUpdate := clone.(*v1alpha1.ServiceInstance)
		toUpdate.Status.AsyncOpInProgress = false
		toUpdate.Status.OperationStartTime = nil
		toUpdate.Status.ReconciledGeneration = toUpdate.Generation

		readyCond := v1alpha1.ConditionFalse
		reason := errorProvisionCallFailedReason
		msg := "Provision call failed: " + s
		if deleting {
			readyCond = v1alpha1.ConditionUnknown
			reason = errorDeprovisionCalledReason
			msg = "Deprovision call failed:" + s
		}
		setServiceInstanceCondition(
			toUpdate,
			v1alpha1.ServiceInstanceConditionReady,
			readyCond,
			reason,
			msg,
		)
		setServiceInstanceCondition(
			toUpdate,
			v1alpha1.ServiceInstanceConditionFailed,
			v1alpha1.ConditionTrue,
			reason,
			msg,
		)
		if err := c.updateServiceInstanceStatus(toUpdate); err != nil {
			return err
		}
		c.recorder.Event(instance, api.EventTypeWarning, errorDeprovisionCalledReason, s)

		err = c.finishPollingServiceInstance(instance)
		if err != nil {
			return err
		}
	default:
		glog.Warningf("Got invalid state in LastOperationResponse: %q", response.State)
		if !time.Now().Before(instance.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
			clone, err := api.Scheme.DeepCopy(instance)
			if err != nil {
				return err
			}
			toUpdate := clone.(*v1alpha1.ServiceInstance)
			s := fmt.Sprintf(`Stopping reconciliation retries on ServiceInstance "%v/%v" because too much time has elapsed`, instance.Namespace, instance.Name)
			glog.Info(s)
			c.recorder.Event(instance, api.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)
			setServiceInstanceCondition(toUpdate,
				v1alpha1.ServiceInstanceConditionFailed,
				v1alpha1.ConditionTrue,
				errorReconciliationRetryTimeoutReason,
				s)
			toUpdate.Status.OperationStartTime = nil
			toUpdate.Status.ReconciledGeneration = toUpdate.Generation
			if err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}
			return c.finishPollingServiceInstance(instance)
		}
		return fmt.Errorf("Got invalid state in LastOperationResponse: %q", response.State)
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

// setServiceInstanceCondition sets a single condition on an Instance's status: if
// the condition already exists in the status, it is mutated; if the condition
// does not already exist in the status, it is added.  Other conditions in the
// status are not altered.  If the condition exists and its status changes,
// the LastTransitionTime field is updated.
//
// Note: objects coming from informers should never be mutated; always pass a
// deep copy as the instance parameter.
func setServiceInstanceCondition(toUpdate *v1alpha1.ServiceInstance,
	conditionType v1alpha1.ServiceInstanceConditionType,
	status v1alpha1.ConditionStatus,
	reason,
	message string) {
	setServiceInstanceConditionInternal(toUpdate, conditionType, status, reason, message, metav1.Now())
}

// setServiceInstanceConditionInternal is setServiceInstanceCondition but allows the time to
// be parameterized for testing.
func setServiceInstanceConditionInternal(toUpdate *v1alpha1.ServiceInstance,
	conditionType v1alpha1.ServiceInstanceConditionType,
	status v1alpha1.ConditionStatus,
	reason,
	message string,
	t metav1.Time) {

	glog.V(5).Infof(`Setting ServiceInstance "%v/%v" condition %q to %v`, toUpdate.Namespace, toUpdate.Name, conditionType, status)

	newCondition := v1alpha1.ServiceInstanceCondition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	}

	if len(toUpdate.Status.Conditions) == 0 {
		glog.V(3).Infof(`Setting lastTransitionTime for ServiceInstance "%v/%v" condition %q to %v`, toUpdate.Namespace, toUpdate.Name, conditionType, t)
		newCondition.LastTransitionTime = t
		toUpdate.Status.Conditions = []v1alpha1.ServiceInstanceCondition{newCondition}
		return
	}

	for i, cond := range toUpdate.Status.Conditions {
		if cond.Type == conditionType {
			if cond.Status != newCondition.Status {
				glog.V(3).Infof(`Found status change for ServiceInstance "%v/%v" condition %q: %q -> %q; setting lastTransitionTime to %v`, toUpdate.Namespace, toUpdate.Name, conditionType, cond.Status, status, t)
				newCondition.LastTransitionTime = t
			} else {
				newCondition.LastTransitionTime = cond.LastTransitionTime
			}

			toUpdate.Status.Conditions[i] = newCondition
			return
		}
	}

	glog.V(3).Infof(`Setting lastTransitionTime for ServiceInstance "%v/%v" condition %q to %v`, toUpdate.Namespace, toUpdate.Name, conditionType, t)
	newCondition.LastTransitionTime = t
	toUpdate.Status.Conditions = append(toUpdate.Status.Conditions, newCondition)
}

// updateServiceInstanceStatus updates the status for the given instance.
//
// Note: objects coming from informers should never be mutated; the instance
// passed to this method should always be a deep copy.
func (c *controller) updateServiceInstanceStatus(toUpdate *v1alpha1.ServiceInstance) error {
	glog.V(4).Infof("Updating status for ServiceInstance %v/%v", toUpdate.Namespace, toUpdate.Name)
	_, err := c.serviceCatalogClient.ServiceInstances(toUpdate.Namespace).UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf("Failed to update status for ServiceInstance %v/%v: %v", toUpdate.Namespace, toUpdate.Name, err)
	}

	return err
}

// updateServiceInstanceCondition updates the given condition for the given Instance
// with the given status, reason, and message.
func (c *controller) updateServiceInstanceCondition(
	instance *v1alpha1.ServiceInstance,
	conditionType v1alpha1.ServiceInstanceConditionType,
	status v1alpha1.ConditionStatus,
	reason,
	message string) error {

	clone, err := api.Scheme.DeepCopy(instance)
	if err != nil {
		return err
	}
	toUpdate := clone.(*v1alpha1.ServiceInstance)

	setServiceInstanceCondition(toUpdate, conditionType, status, reason, message)

	glog.V(4).Infof("Updating %v condition for ServiceInstance %v/%v to %v", conditionType, instance.Namespace, instance.Name, status)
	_, err = c.serviceCatalogClient.ServiceInstances(instance.Namespace).UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf("Failed to update condition %v for ServiceInstance %v/%v to true: %v", conditionType, instance.Namespace, instance.Name, err)
	}

	return err
}

// updateServiceInstanceFinalizers updates the given finalizers for the given ServiceInstanceCredential.
func (c *controller) updateServiceInstanceFinalizers(
	instance *v1alpha1.ServiceInstance,
	finalizers []string) error {

	// Get the latest version of the instance so that we can avoid conflicts
	// (since we have probably just updated the status of the instance and are
	// now removing the last finalizer).
	instance, err := c.serviceCatalogClient.ServiceInstances(instance.Namespace).Get(instance.Name, metav1.GetOptions{})
	if err != nil {
		glog.Errorf("Error getting ServiceInstance %v/%v to finalize: %v", instance.Namespace, instance.Name, err)
	}

	clone, err := api.Scheme.DeepCopy(instance)
	if err != nil {
		return err
	}
	toUpdate := clone.(*v1alpha1.ServiceInstance)

	toUpdate.Finalizers = finalizers

	logContext := fmt.Sprintf("finalizers for ServiceInstance %v/%v to %v",
		instance.Namespace, instance.Name, finalizers)

	glog.V(4).Infof("Updating %v", logContext)
	_, err = c.serviceCatalogClient.ServiceInstances(instance.Namespace).UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf("Error updating %v: %v", logContext, err)
	}
	return err
}

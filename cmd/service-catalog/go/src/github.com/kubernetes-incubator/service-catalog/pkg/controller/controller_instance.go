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
	"net/url"

	"github.com/golang/glog"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	scfeatures "github.com/kubernetes-incubator/service-catalog/pkg/features"
	"github.com/kubernetes-incubator/service-catalog/pkg/pretty"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
)

const (
	successDeprovisionReason       string = "DeprovisionedSuccessfully"
	successDeprovisionMessage      string = "The instance was deprovisioned successfully"
	successUpdateInstanceReason    string = "InstanceUpdatedSuccessfully"
	successUpdateInstanceMessage   string = "The instance was updated successfully"
	successProvisionReason         string = "ProvisionedSuccessfully"
	successProvisionMessage        string = "The instance was provisioned successfully"
	successOrphanMitigationReason  string = "OrphanMitigationSuccessful"
	successOrphanMitigationMessage string = "Orphan mitigation was completed successfully"

	errorWithParameters                        string = "ErrorWithParameters"
	errorProvisionCallFailedReason             string = "ProvisionCallFailed"
	errorErrorCallingProvisionReason           string = "ErrorCallingProvision"
	errorUpdateInstanceCallFailedReason        string = "UpdateInstanceCallFailed"
	errorErrorCallingUpdateInstanceReason      string = "ErrorCallingUpdateInstance"
	errorDeprovisionCalledReason               string = "DeprovisionCallFailed"
	errorDeprovisionBlockedByCredentialsReason string = "DeprovisionBlockedByExistingCredentials"
	errorPollingLastOperationReason            string = "ErrorPollingLastOperation"
	errorWithOriginatingIdentity               string = "Error with Originating Identity"
	errorWithOngoingAsyncOperation             string = "ErrorAsyncOperationInProgress"
	errorWithOngoingAsyncOperationMessage      string = "Another operation for this service instance is in progress. "
	errorNonexistentClusterServiceClassReason  string = "ReferencesNonexistentServiceClass"
	errorNonexistentClusterServiceClassMessage string = "ReferencesNonexistentServiceClass"
	errorNonexistentClusterServicePlanReason   string = "ReferencesNonexistentServicePlan"
	errorNonexistentClusterServiceBrokerReason string = "ReferencesNonexistentBroker"
	errorDeletedClusterServiceClassReason      string = "ReferencesDeletedServiceClass"
	errorDeletedClusterServiceClassMessage     string = "ReferencesDeletedServiceClass"
	errorDeletedClusterServicePlanReason       string = "ReferencesDeletedServicePlan"
	errorDeletedClusterServicePlanMessage      string = "ReferencesDeletedServicePlan"
	errorFindingNamespaceServiceInstanceReason string = "ErrorFindingNamespaceForInstance"
	errorOrphanMitigationFailedReason          string = "OrphanMitigationFailed"
	errorInvalidDeprovisionStatusReason        string = "InvalidDeprovisionStatus"
	errorInvalidDeprovisionStatusMessage       string = "The deprovision status is invalid"
	errorUnknownServicePlanReason              string = "UnknownServicePlan"
	errorUnknownServicePlanMessage             string = "The ServicePlan is not known"

	asyncProvisioningReason                 string = "Provisioning"
	asyncProvisioningMessage                string = "The instance is being provisioned asynchronously"
	asyncUpdatingInstanceReason             string = "UpdatingInstance"
	asyncUpdatingInstanceMessage            string = "The instance is being updated asynchronously"
	asyncDeprovisioningReason               string = "Deprovisioning"
	asyncDeprovisioningMessage              string = "The instance is being deprovisioned asynchronously"
	provisioningInFlightReason              string = "ProvisionRequestInFlight"
	provisioningInFlightMessage             string = "Provision request for ServiceInstance in-flight to Broker"
	instanceUpdatingInFlightReason          string = "UpdateInstanceRequestInFlight"
	instanceUpdatingInFlightMessage         string = "Update request for ServiceInstance in-flight to Broker"
	deprovisioningInFlightReason            string = "DeprovisionRequestInFlight"
	deprovisioningInFlightMessage           string = "Deprovision request for ServiceInstance in-flight to Broker"
	startingInstanceOrphanMitigationReason  string = "StartingInstanceOrphanMitigation"
	startingInstanceOrphanMitigationMessage string = "The instance provision call failed with an ambiguous error; attempting to deprovision the instance in order to mitigate an orphaned resource"
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
	instance := newObj.(*v1beta1.ServiceInstance)
	if !instance.Status.AsyncOpInProgress {
		c.instanceAdd(newObj)
	}
}

func (c *controller) instanceDelete(obj interface{}) {
	instance, ok := obj.(*v1beta1.ServiceInstance)
	if instance == nil || !ok {
		return
	}

	pcb := pretty.NewContextBuilder(pretty.ServiceInstance, instance.Namespace, instance.Name)
	glog.V(4).Info(pcb.Message("Received delete event; no further processing will occur"))
}

// Async operations on instances have a somewhat convoluted flow in order to
// ensure that only a single goroutine works on an instance at any given time.
// The flow is:
//
// 1.  When the controller wants to begin polling the state of an operation on
//     an instance, it calls its beginPollingServiceInstance method (or
//     calls continuePollingServiceInstance, an alias of that method)
// 2.  begin/continuePollingServiceInstance do a rate-limited add to the polling queue
// 3.  the instancePollingQueue calls requeueServiceInstanceForPoll, which adds the instance's
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
func (c *controller) beginPollingServiceInstance(instance *v1beta1.ServiceInstance) error {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(instance)
	if err != nil {
		pcb := pretty.NewContextBuilder(pretty.ServiceInstance, instance.Namespace, instance.Name)
		s := fmt.Sprintf("Couldn't create a key for object %+v: %v", instance, err)
		glog.Errorf(pcb.Message(s))
		return fmt.Errorf(s)
	}

	c.instancePollingQueue.AddRateLimited(key)

	return nil
}

// continuePollingServiceInstance does a rate-limited add of the key for the given
// instance to the controller's instance polling queue.
func (c *controller) continuePollingServiceInstance(instance *v1beta1.ServiceInstance) error {
	return c.beginPollingServiceInstance(instance)
}

// finishPollingServiceInstance removes the instance's key from the controller's instance
// polling queue.
func (c *controller) finishPollingServiceInstance(instance *v1beta1.ServiceInstance) error {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(instance)
	if err != nil {
		pcb := pretty.NewContextBuilder(pretty.ServiceInstance, instance.Namespace, instance.Name)
		s := fmt.Sprintf("Couldn't create a key for object %+v: %v", instance, err)
		glog.Errorf(pcb.Message(s))
		return fmt.Errorf(s)
	}

	c.instancePollingQueue.Forget(key)

	return nil
}

// getReconciliationActionForServiceInstance gets the action the reconciler
// should be taking on the given instance.
func getReconciliationActionForServiceInstance(instance *v1beta1.ServiceInstance) ReconciliationAction {
	switch {
	case instance.Status.AsyncOpInProgress:
		return reconcilePoll
	case instance.ObjectMeta.DeletionTimestamp != nil || instance.Status.OrphanMitigationInProgress:
		return reconcileDelete
	case instance.Status.ReconciledGeneration != 0:
		return reconcileUpdate
	default: // instance.Status.ReconciledGeneration == 0
		return reconcileAdd
	}
}

func (c *controller) reconcileServiceInstanceKey(key string) error {
	// For namespace-scoped resources, SplitMetaNamespaceKey splits the key
	// i.e. "namespace/name" into two separate strings
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	pcb := pretty.NewContextBuilder(pretty.ServiceInstance, namespace, name)
	instance, err := c.instanceLister.ServiceInstances(namespace).Get(name)
	if errors.IsNotFound(err) {
		glog.Info(pcb.Messagef("Not doing work for %v because it has been deleted", key))
		return nil
	}
	if err != nil {
		glog.Errorf(pcb.Messagef("Unable to retrieve %v from store: %v", key, err))
		return err
	}

	return c.reconcileServiceInstance(instance)
}

// reconcileServiceInstance is the control-loop for reconciling Instances. An
// error is returned to indicate that the instance has not been fully
// processed and should be resubmitted at a later time.
func (c *controller) reconcileServiceInstance(instance *v1beta1.ServiceInstance) error {
	reconciliationAction := getReconciliationActionForServiceInstance(instance)
	switch reconciliationAction {
	case reconcileAdd:
		return c.reconcileServiceInstanceAdd(instance)
	case reconcileUpdate:
		return c.reconcileServiceInstanceUpdate(instance)
	case reconcileDelete:
		return c.reconcileServiceInstanceDelete(instance)
	case reconcilePoll:
		return c.pollServiceInstance(instance)
	default:
		pcb := pretty.NewContextBuilder(pretty.ServiceInstance, instance.Namespace, instance.Name)
		return fmt.Errorf(pcb.Messagef("Unknown reconciliation action %v", reconciliationAction))
	}
}

// reconcileServiceInstanceAdd is responsible for handling the provisioning
// of new service instances.
func (c *controller) reconcileServiceInstanceAdd(instance *v1beta1.ServiceInstance) error {
	pcb := pretty.NewContextBuilder(pretty.ServiceInstance, instance.Namespace, instance.Name)

	if isServiceInstanceFailed(instance) {
		glog.V(4).Info(pcb.Message("Not processing event because status showed that it has failed"))
		return nil
	}

	if instance.Status.ReconciledGeneration == instance.Generation {
		glog.V(4).Info(pcb.Message("Not processing event because reconciled generation showed there is no work to do"))
		return nil
	}

	instance = instance.DeepCopy()

	// Update references to ClusterServicePlan / ClusterServiceClass if necessary.
	//
	// TODO(mkibbe): Make this trigger another reconciliation instead of continuing.
	instance, err := c.resolveReferences(instance)
	if err != nil {
		return err
	}

	glog.V(4).Info(pcb.Message("Processing adding event"))

	serviceClass, servicePlan, brokerName, brokerClient, err := c.getClusterServiceClassPlanAndClusterServiceBroker(instance)
	if err != nil {
		return c.handleServiceInstanceReconciliationError(instance, err)
	}

	// Check if the ServiceClass or ServicePlan has been deleted and do not allow
	// creation of new ServiceInstances.
	if err := c.checkForRemovedClassAndPlan(instance, serviceClass, servicePlan); err != nil {
		return c.handleServiceInstanceReconciliationError(instance, err)
	}

	request, inProgressProperties, err := c.prepareProvisionRequest(instance, serviceClass, servicePlan)
	if err != nil {
		return c.handleServiceInstanceReconciliationError(instance, err)
	}

	if instance.Status.CurrentOperation == "" {
		instance, err = c.recordStartOfServiceInstanceOperation(instance, v1beta1.ServiceInstanceOperationProvision, inProgressProperties)
		if err != nil {
			// There has been an update to the instance. Start reconciliation
			// over with a fresh view of the instance.
			return err
		}
	}

	glog.V(4).Info(pcb.Messagef(
		"Provisioning a new ServiceInstance of %s at ClusterServiceBroker %q",
		pretty.ClusterServiceClassName(serviceClass), brokerName,
	))

	response, err := brokerClient.ProvisionInstance(request)
	if err != nil {
		// A failure HTTP response code is treated as a terminal
		// failure. Depending on the specific response, we may also
		// need to initiate orphan mitigation.
		if httpErr, ok := osb.IsHTTPError(err); ok {
			msg := fmt.Sprintf(
				"Error provisioning ServiceInstance of %s at ClusterServiceBroker %q: %s",
				pretty.ClusterServiceClassName(serviceClass), brokerName, httpErr,
			)
			readyCond := newServiceInstanceReadyCondition(v1beta1.ConditionFalse, errorProvisionCallFailedReason, msg)
			failedCond := newServiceInstanceFailedCondition(v1beta1.ConditionTrue, "ClusterServiceBrokerReturnedFailure", msg)
			return c.processProvisionFailure(instance, readyCond, failedCond, shouldStartOrphanMitigation(httpErr.StatusCode))
		}

		reason := errorErrorCallingProvisionReason

		// A timeout error is considered a terminal failure and we
		// should initiate orphan mitigation.
		if urlErr, ok := err.(*url.Error); ok && urlErr.Timeout() {
			msg := fmt.Sprintf("Communication with the ClusterServiceBroker timed out; operation will not be retried: %v", urlErr)
			readyCond := newServiceInstanceReadyCondition(v1beta1.ConditionFalse, reason, msg)
			failedCond := newServiceInstanceFailedCondition(v1beta1.ConditionTrue, reason, msg)
			return c.processProvisionFailure(instance, readyCond, failedCond, true)
		}

		// All other errors should be retried, unless the
		// reconciliation retry time limit has passed.
		msg := fmt.Sprintf("The provision call failed and will be retried: Error communicating with broker for provisioning: %v", err)
		readyCond := newServiceInstanceReadyCondition(v1beta1.ConditionFalse, reason, msg)

		if c.reconciliationRetryDurationExceeded(instance.Status.OperationStartTime) {
			msg := "Stopping reconciliation retries because too much time has elapsed"
			failedCond := newServiceInstanceFailedCondition(v1beta1.ConditionTrue, errorReconciliationRetryTimeoutReason, msg)
			return c.processProvisionFailure(instance, readyCond, failedCond, false)
		}

		return c.processServiceInstanceOperationError(instance, readyCond)
	}

	if response.Async {
		return c.processProvisionAsyncResponse(instance, response)
	}

	return c.processProvisionSuccess(instance, response.DashboardURL)
}

// reconcileServiceInstanceUpdate is responsible for handling updating the plan
// or parameters of a service instance.
func (c *controller) reconcileServiceInstanceUpdate(instance *v1beta1.ServiceInstance) error {
	pcb := pretty.NewContextBuilder(pretty.ServiceInstance, instance.Namespace, instance.Name)

	if isServiceInstanceFailed(instance) {
		glog.V(4).Info(pcb.Message("Not processing event because status showed that it has failed"))
		return nil
	}

	if instance.Status.ReconciledGeneration == instance.Generation {
		glog.V(4).Info(pcb.Message("Not processing event because reconciled generation showed there is no work to do"))
		return nil
	}

	instance = instance.DeepCopy()

	// Update references to ClusterServicePlan / ClusterServiceClass if necessary.
	//
	// TODO(mkibbe): Make this trigger another reconciliation instead of continuing.
	instance, err := c.resolveReferences(instance)
	if err != nil {
		return err
	}

	glog.V(4).Info(pcb.Message("Processing updating event"))

	serviceClass, servicePlan, brokerName, brokerClient, err := c.getClusterServiceClassPlanAndClusterServiceBroker(instance)
	if err != nil {
		return c.handleServiceInstanceReconciliationError(instance, err)
	}

	// Check if the ServiceClass or ServicePlan has been deleted. If so, do
	// not allow plan upgrades, but do allow parameter changes.
	if err := c.checkForRemovedClassAndPlan(instance, serviceClass, servicePlan); err != nil {
		return c.handleServiceInstanceReconciliationError(instance, err)
	}

	request, inProgressProperties, err := c.prepareUpdateInstanceRequest(instance, serviceClass, servicePlan)
	if err != nil {
		return c.handleServiceInstanceReconciliationError(instance, err)
	}

	if instance.Status.CurrentOperation == "" {
		instance, err = c.recordStartOfServiceInstanceOperation(instance, v1beta1.ServiceInstanceOperationUpdate, inProgressProperties)
		if err != nil {
			// There has been an update to the instance. Start reconciliation
			// over with a fresh view of the instance.
			return err
		}
	}

	glog.V(4).Info(pcb.Messagef(
		"Updating ServiceInstance of %s at ClusterServiceBroker %q",
		pretty.ClusterServiceClassName(serviceClass), brokerName,
	))

	response, err := brokerClient.UpdateInstance(request)
	if err != nil {
		if httpErr, ok := osb.IsHTTPError(err); ok {
			msg := fmt.Sprintf("ClusterServiceBroker returned a failure for update call; update will not be retried: %v", httpErr)
			readyCond := newServiceInstanceReadyCondition(v1beta1.ConditionFalse, errorUpdateInstanceCallFailedReason, msg)
			return c.processUpdateServiceInstanceFailure(instance, readyCond)
		}

		reason := errorErrorCallingUpdateInstanceReason

		if urlErr, ok := err.(*url.Error); ok && urlErr.Timeout() {
			msg := fmt.Sprintf("Communication with the ClusterServiceBroker timed out; update will not be retried: %v", urlErr)
			readyCond := newServiceInstanceReadyCondition(v1beta1.ConditionFalse, reason, msg)
			return c.processUpdateServiceInstanceFailure(instance, readyCond)
		}

		msg := fmt.Sprintf("The update call failed and will be retried: Error communicating with broker for updating: %s", err)

		if c.reconciliationRetryDurationExceeded(instance.Status.OperationStartTime) {
			// log and record the real error, but process as a
			// failure with reconciliation retry timeout
			glog.Info(pcb.Message(msg))
			c.recorder.Event(instance, corev1.EventTypeWarning, reason, msg)

			msg = "Stopping reconciliation retries because too much time has elapsed"
			readyCond := newServiceInstanceReadyCondition(v1beta1.ConditionFalse, errorReconciliationRetryTimeoutReason, msg)
			return c.processUpdateServiceInstanceFailure(instance, readyCond)
		}

		readyCond := newServiceInstanceReadyCondition(v1beta1.ConditionFalse, reason, msg)
		return c.processServiceInstanceOperationError(instance, readyCond)
	}

	if response.Async {
		return c.processUpdateServiceInstanceAsyncResponse(instance, response)
	}

	return c.processUpdateServiceInstanceSuccess(instance)
}

// reconcileServiceInstanceDelete is responsible for handling any instance whose
// deletion timestamp is set.
func (c *controller) reconcileServiceInstanceDelete(instance *v1beta1.ServiceInstance) error {
	// nothing to do...
	if instance.DeletionTimestamp == nil && !instance.Status.OrphanMitigationInProgress {
		return nil
	}

	if finalizers := sets.NewString(instance.Finalizers...); !finalizers.Has(v1beta1.FinalizerServiceCatalog) {
		return nil
	}

	pcb := pretty.NewContextBuilder(pretty.ServiceInstance, instance.Namespace, instance.Name)

	// If deprovisioning has already failed, do not do anything more
	if instance.Status.DeprovisionStatus == v1beta1.ServiceInstanceDeprovisionStatusFailed {
		glog.V(4).Info(pcb.Message("Not processing deleting event because deprovisioning has failed"))
		return nil
	}

	glog.V(4).Info(pcb.Message("Processing deleting event"))

	instance = instance.DeepCopy()

	// If the deprovisioning succeeded or is not needed, then no need to
	// make a request to the broker.
	if instance.Status.DeprovisionStatus == v1beta1.ServiceInstanceDeprovisionStatusNotRequired ||
		instance.Status.DeprovisionStatus == v1beta1.ServiceInstanceDeprovisionStatusSucceeded {

		return c.processServiceInstanceGracefulDeletionSuccess(instance)
	}

	// At this point, if the deprovision status is not Required, then it is
	// either an invalid value or there is a logical error in the controller.
	// Set the deprovision status to Failed and bail out.
	if instance.Status.DeprovisionStatus != v1beta1.ServiceInstanceDeprovisionStatusRequired {
		msg := fmt.Sprintf("ServiceInstance has invalid DeprovisionStatus field: %v", instance.Status.DeprovisionStatus)
		readyCond := newServiceInstanceReadyCondition(v1beta1.ConditionUnknown, errorInvalidDeprovisionStatusReason, msg)
		failedCond := newServiceInstanceFailedCondition(v1beta1.ConditionTrue, errorInvalidDeprovisionStatusReason, msg)
		return c.processDeprovisionFailure(instance, readyCond, failedCond)
	}

	// We don't want to delete the instance if there are any bindings associated.
	if err := c.checkServiceInstanceHasExistingBindings(instance); err != nil {
		return c.handleServiceInstanceReconciliationError(instance, err)
	}

	serviceClass, servicePlan, brokerName, brokerClient, err := c.getClusterServiceClassPlanAndClusterServiceBroker(instance)
	if err != nil {
		return c.handleServiceInstanceReconciliationError(instance, err)
	}

	request, err := c.prepareDeprovisionRequest(instance, serviceClass, servicePlan)
	if err != nil {
		return c.handleServiceInstanceReconciliationError(instance, err)
	}

	if instance.DeletionTimestamp == nil {
		if instance.Status.OperationStartTime == nil {
			// if mitigating an orphan, set the operation start time if unset
			now := metav1.Now()
			instance.Status.OperationStartTime = &now
		}
	} else {
		if instance.Status.CurrentOperation != v1beta1.ServiceInstanceOperationDeprovision {
			instance, err = c.recordStartOfServiceInstanceOperation(instance, v1beta1.ServiceInstanceOperationDeprovision, nil)
			if err != nil {
				// There has been an update to the instance. Start reconciliation
				// over with a fresh view of the instance.
				return err
			}
		}
	}

	glog.V(4).Info(pcb.Message("Sending deprovision request to broker"))
	response, err := brokerClient.DeprovisionInstance(request)
	if err != nil {
		msg := fmt.Sprintf(
			`Error deprovisioning, %s at ClusterServiceBroker %q: %v`,
			pretty.ClusterServiceClassName(serviceClass), brokerName, err,
		)
		if httpErr, ok := osb.IsHTTPError(err); ok {
			msg = fmt.Sprintf("Deprovision call failed; received error response from broker: %v", httpErr)
		}

		readyCond := newServiceInstanceReadyCondition(v1beta1.ConditionUnknown, errorDeprovisionCalledReason, msg)

		if c.reconciliationRetryDurationExceeded(instance.Status.OperationStartTime) {
			msg := "Stopping reconciliation retries because too much time has elapsed"
			failedCond := newServiceInstanceFailedCondition(v1beta1.ConditionTrue, errorReconciliationRetryTimeoutReason, msg)
			return c.processDeprovisionFailure(instance, readyCond, failedCond)
		}

		return c.processServiceInstanceOperationError(instance, readyCond)
	}

	if response.Async {
		return c.processDeprovisionAsyncResponse(instance, response)
	}

	return c.processDeprovisionSuccess(instance)
}

func (c *controller) pollServiceInstance(instance *v1beta1.ServiceInstance) error {
	pcb := pretty.NewContextBuilder(pretty.ServiceInstance, instance.Namespace, instance.Name)
	glog.V(4).Info(pcb.Message("Processing"))

	instance = instance.DeepCopy()

	serviceClass, servicePlan, _, brokerClient, err := c.getClusterServiceClassPlanAndClusterServiceBroker(instance)
	if err != nil {
		return c.handleServiceInstanceReconciliationError(instance, err)
	}

	// There are some conditions that are different depending on which
	// operation we're polling for. This is more readable than checking the
	// status in various places.
	mitigatingOrphan := instance.Status.OrphanMitigationInProgress
	provisioning := instance.Status.CurrentOperation == v1beta1.ServiceInstanceOperationProvision && !mitigatingOrphan
	deleting := instance.Status.CurrentOperation == v1beta1.ServiceInstanceOperationDeprovision || mitigatingOrphan

	request, err := c.prepareServiceInstanceLastOperationRequest(instance, serviceClass, servicePlan)
	if err != nil {
		return c.handleServiceInstanceReconciliationError(instance, err)
	}

	glog.V(5).Info(pcb.Message("Polling last operation"))

	response, err := brokerClient.PollLastOperation(request)
	if err != nil {
		// If the operation was for delete and we receive a http.StatusGone,
		// this is considered a success as per the spec
		if osb.IsGoneError(err) && deleting {
			if err := c.processDeprovisionSuccess(instance); err != nil {
				return c.handleServiceInstancePollingError(instance, err)
			}
			return c.finishPollingServiceInstance(instance)
		}

		// We got some kind of error and should continue polling.
		//
		// The instance's Ready condition should already be False, so
		// we just need to record an event.
		s := fmt.Sprintf("Error polling last operation: %v", err)
		glog.V(4).Info(pcb.Message(s))
		c.recorder.Event(instance, corev1.EventTypeWarning, errorPollingLastOperationReason, s)

		if c.reconciliationRetryDurationExceeded(instance.Status.OperationStartTime) {
			return c.processServiceInstancePollingFailureRetryTimeout(instance, nil)
		}

		return c.continuePollingServiceInstance(instance)
	}

	glog.V(4).Info(pcb.Messagef("Poll returned %q: Response description: %v", response.State, response.Description))

	switch response.State {
	case osb.StateInProgress:
		var message string
		var reason string
		switch {
		case deleting:
			reason = asyncDeprovisioningReason
			message = asyncDeprovisioningMessage
		case provisioning:
			reason = asyncProvisioningReason
			message = asyncProvisioningMessage
		default:
			reason = asyncUpdatingInstanceReason
			message = asyncUpdatingInstanceMessage
		}

		if response.Description != nil {
			message = fmt.Sprintf("%s (%s)", message, *response.Description)
		}

		readyCond := newServiceInstanceReadyCondition(v1beta1.ConditionFalse, reason, message)
		if c.reconciliationRetryDurationExceeded(instance.Status.OperationStartTime) {
			return c.processServiceInstancePollingFailureRetryTimeout(instance, readyCond)
		}

		// only need to update the resource if there was a description for the operation provided
		if response.Description != nil {
			c.recorder.Event(instance, corev1.EventTypeNormal, readyCond.Reason, readyCond.Message)

			setServiceInstanceCondition(instance, v1beta1.ServiceInstanceConditionReady, readyCond.Status, readyCond.Reason, readyCond.Message)
			if _, err := c.updateServiceInstanceStatus(instance); err != nil {
				return c.handleServiceInstancePollingError(instance, err)
			}
		}

		glog.V(4).Info(pcb.Message("Last operation not completed (still in progress)"))
		return c.continuePollingServiceInstance(instance)
	case osb.StateSucceeded:
		var err error
		switch {
		case deleting:
			err = c.processDeprovisionSuccess(instance)
		case provisioning:
			err = c.processProvisionSuccess(instance, nil)
		default:
			err = c.processUpdateServiceInstanceSuccess(instance)
		}
		if err != nil {
			return c.handleServiceInstancePollingError(instance, err)
		}
		return c.finishPollingServiceInstance(instance)
	case osb.StateFailed:
		description := "(no description provided)"
		if response.Description != nil {
			description = *response.Description
		}

		var err error
		switch {
		case deleting:
			// For deprovisioning only, we should reattempt even on failure
			//
			// TODO(mkibbe): This is actually broken today, as we
			// will not actually be retrying the deprovision, but
			// instead constantly re-hitting the "last_operation"
			// endpoint with the same operation, which will forever
			// be stuck on failure.
			msg := "Deprovision call failed: " + description
			readyCond := newServiceInstanceReadyCondition(v1beta1.ConditionUnknown, errorDeprovisionCalledReason, msg)

			if c.reconciliationRetryDurationExceeded(instance.Status.OperationStartTime) {
				return c.processServiceInstancePollingFailureRetryTimeout(instance, readyCond)
			}

			c.processServiceInstanceOperationError(instance, readyCond)
			return c.continuePollingServiceInstance(instance)
		case provisioning:
			reason := errorProvisionCallFailedReason
			message := "Provision call failed: " + description
			readyCond := newServiceInstanceReadyCondition(v1beta1.ConditionFalse, reason, message)
			failedCond := newServiceInstanceFailedCondition(v1beta1.ConditionTrue, reason, message)
			err = c.processProvisionFailure(instance, readyCond, failedCond, false)
		default:
			msg := "Update call failed: " + description
			readyCond := newServiceInstanceReadyCondition(v1beta1.ConditionFalse, errorUpdateInstanceCallFailedReason, msg)
			err = c.processUpdateServiceInstanceFailure(instance, readyCond)
		}
		if err != nil {
			return c.handleServiceInstancePollingError(instance, err)
		}

		return c.finishPollingServiceInstance(instance)
	default:
		glog.Warning(pcb.Messagef("Got invalid state in LastOperationResponse: %q", response.State))
		if c.reconciliationRetryDurationExceeded(instance.Status.OperationStartTime) {
			return c.processServiceInstancePollingFailureRetryTimeout(instance, nil)
		}

		err := fmt.Errorf(`Got invalid state in LastOperationResponse: %q`, response.State)
		return c.handleServiceInstancePollingError(instance, err)
	}
}

// isServiceInstanceFailed returns whether the instance has a failed condition with
// status true.
func isServiceInstanceFailed(instance *v1beta1.ServiceInstance) bool {
	for _, condition := range instance.Status.Conditions {
		if condition.Type == v1beta1.ServiceInstanceConditionFailed && condition.Status == v1beta1.ConditionTrue {
			return true
		}
	}

	return false
}

// processServiceInstancePollingFailureRetryTimeout marks the instance as having
// failed polling due to its reconciliation retry duration expiring
func (c *controller) processServiceInstancePollingFailureRetryTimeout(instance *v1beta1.ServiceInstance, readyCond *v1beta1.ServiceInstanceCondition) error {
	mitigatingOrphan := instance.Status.OrphanMitigationInProgress
	provisioning := instance.Status.CurrentOperation == v1beta1.ServiceInstanceOperationProvision && !mitigatingOrphan
	deleting := instance.Status.CurrentOperation == v1beta1.ServiceInstanceOperationDeprovision || mitigatingOrphan

	msg := "Stopping reconciliation retries because too much time has elapsed"
	failedCond := newServiceInstanceFailedCondition(v1beta1.ConditionTrue, errorReconciliationRetryTimeoutReason, msg)

	var err error
	switch {
	case deleting:
		err = c.processDeprovisionFailure(instance, readyCond, failedCond)
	case provisioning:
		// always finish polling instance, as triggering OM will return an error
		c.finishPollingServiceInstance(instance)
		return c.processProvisionFailure(instance, readyCond, failedCond, true)
	default:
		readyCond := newServiceInstanceReadyCondition(v1beta1.ConditionFalse, errorReconciliationRetryTimeoutReason, msg)
		err = c.processUpdateServiceInstanceFailure(instance, readyCond)
	}
	if err != nil {
		return c.handleServiceInstancePollingError(instance, err)
	}

	return c.finishPollingServiceInstance(instance)
}

// resolveReferences checks to see if ClusterServiceClassRef and/or ClusterServicePlanRef are
// nil and if so, will resolve the references and update the instance.
// If either can not be resolved, returns an error and sets the InstanceCondition
// with the appropriate error message.
func (c *controller) resolveReferences(instance *v1beta1.ServiceInstance) (*v1beta1.ServiceInstance, error) {
	if instance.Spec.ClusterServiceClassRef != nil && instance.Spec.ClusterServicePlanRef != nil {
		return instance, nil
	}

	var sc *v1beta1.ClusterServiceClass
	var err error
	if instance.Spec.ClusterServiceClassRef == nil {
		instance, sc, err = c.resolveClusterServiceClassRef(instance)
		if err != nil {
			return nil, err
		}
	}

	if instance.Spec.ClusterServicePlanRef == nil {
		if sc == nil {
			var scErr error
			sc, scErr = c.serviceClassLister.Get(instance.Spec.ClusterServiceClassRef.Name)
			if scErr != nil {
				return nil, fmt.Errorf(`Couldn't find ClusterServiceClass (K8S: %s)": %v`, instance.Spec.ClusterServiceClassRef.Name, scErr.Error())
			}
		}

		instance, err = c.resolveClusterServicePlanRef(instance, sc.Spec.ClusterServiceBrokerName)
		if err != nil {
			return nil, err
		}
	}
	return c.updateServiceInstanceReferences(instance)
}

// resolveClusterServiceClassRef resolves a reference  to a ClusterServiceClass
// and updates the instance.
// If ClusterServiceClass can not be resolved, returns an error, records an
// Event, and sets the InstanceCondition with the appropriate error message.
func (c *controller) resolveClusterServiceClassRef(instance *v1beta1.ServiceInstance) (*v1beta1.ServiceInstance, *v1beta1.ClusterServiceClass, error) {
	pcb := pretty.NewContextBuilder(pretty.ServiceInstance, instance.Namespace, instance.Name)
	var sc *v1beta1.ClusterServiceClass
	if instance.Spec.ClusterServiceClassExternalName != "" {
		glog.V(4).Info(pcb.Messagef("looking up a ClusterServiceClass from externalName: %q", instance.Spec.ClusterServiceClassExternalName))
		listOpts := metav1.ListOptions{FieldSelector: "spec.externalName==" + instance.Spec.ClusterServiceClassExternalName}
		serviceClasses, err := c.serviceCatalogClient.ClusterServiceClasses().List(listOpts)
		if err == nil && len(serviceClasses.Items) == 1 {
			sc = &serviceClasses.Items[0]
			instance.Spec.ClusterServiceClassRef = &v1beta1.ClusterObjectReference{
				Name: sc.Name,
			}
			glog.V(4).Info(pcb.Messagef(
				"resolved ClusterServiceClass with externalName %q to K8S ClusterServiceClass %q",
				instance.Spec.ClusterServiceClassExternalName, sc.Name,
			))
		} else {
			s := fmt.Sprintf(
				"References a non-existent ClusterServiceClass (ExternalName: %q) or there is more than one (found: %d)",
				instance.Spec.ClusterServiceClassExternalName, len(serviceClasses.Items),
			)
			glog.Warning(pcb.Message(s))
			c.updateServiceInstanceCondition(
				instance,
				v1beta1.ServiceInstanceConditionReady,
				v1beta1.ConditionFalse,
				errorNonexistentClusterServiceClassReason,
				"The instance references a ClusterServiceClass that does not exist. "+s,
			)
			c.recorder.Event(instance, corev1.EventTypeWarning, errorNonexistentClusterServiceClassReason, s)
			return nil, nil, fmt.Errorf(s)
		}
	} else if instance.Spec.ClusterServiceClassName != "" {
		glog.V(4).Info(pcb.Messagef("looking up a ClusterServiceClass from K8S Name: %q", instance.Spec.ClusterServiceClassName))

		var err error
		sc, err = c.serviceClassLister.Get(instance.Spec.ClusterServiceClassName)
		if err == nil {
			instance.Spec.ClusterServiceClassRef = &v1beta1.ClusterObjectReference{
				Name: sc.Name,
			}
			glog.V(4).Info(pcb.Messagef(
				"resolved ClusterServiceClass with K8S name %q to ClusterServiceClass with external Name %q",
				instance.Spec.ClusterServiceClassName, sc.Spec.ExternalName,
			))
		} else {
			s := fmt.Sprintf(
				"References a non-existent ClusterServiceClass (K8S: %q)",
				instance.Spec.ClusterServiceClassName,
			)
			glog.Warning(pcb.Message(s))
			c.updateServiceInstanceCondition(
				instance,
				v1beta1.ServiceInstanceConditionReady,
				v1beta1.ConditionFalse,
				errorNonexistentClusterServiceClassReason,
				"The instance references a ClusterServiceClass that does not exist. "+s,
			)
			c.recorder.Event(instance, corev1.EventTypeWarning, errorNonexistentClusterServiceClassReason, s)
			return nil, nil, fmt.Errorf(s)
		}
	} else {
		// ServiceInstance is in invalid state, should not ever happen. check
		return nil, nil, fmt.Errorf("ServiceInstance is in inconsistent state, neither ClusterServiceClassExternalName nor ClusterServiceClassName is set: %+v", instance.Spec)
	}
	return instance, sc, nil
}

// resolveClusterServicePlanRef resolves a reference  to a ClusterServicePlan
// and updates the instance.
// If ClusterServicePlan can not be resolved, returns an error, records an
// Event, and sets the InstanceCondition with the appropriate error message.
func (c *controller) resolveClusterServicePlanRef(instance *v1beta1.ServiceInstance, brokerName string) (*v1beta1.ServiceInstance, error) {
	pcb := pretty.NewContextBuilder(pretty.ServiceInstance, instance.Namespace, instance.Name)
	if instance.Spec.ClusterServicePlanExternalName != "" {
		fieldSet := fields.Set{
			"spec.externalName":                instance.Spec.ClusterServicePlanExternalName,
			"spec.clusterServiceClassRef.name": instance.Spec.ClusterServiceClassRef.Name,
			"spec.clusterServiceBrokerName":    brokerName,
		}
		fieldSelector := fields.SelectorFromSet(fieldSet).String()
		listOpts := metav1.ListOptions{FieldSelector: fieldSelector}
		servicePlans, err := c.serviceCatalogClient.ClusterServicePlans().List(listOpts)
		if err == nil && len(servicePlans.Items) == 1 {
			sp := &servicePlans.Items[0]
			instance.Spec.ClusterServicePlanRef = &v1beta1.ClusterObjectReference{
				Name: sp.Name,
			}
			glog.V(4).Info(pcb.Messagef("resolved ClusterServicePlan (ExternalName: %q) to ClusterServicePlan (K8S: %q)",
				instance.Spec.ClusterServicePlanExternalName, sp.Name,
			))
		} else {
			s := fmt.Sprintf(
				"References a non-existent ClusterServicePlan (K8S: %q ExternalName: %q) on ClusterServiceClass (K8S: %q ExternalName: %q) or there is more than one (found: %d)",
				instance.Spec.ClusterServicePlanName, instance.Spec.ClusterServicePlanExternalName, instance.Spec.ClusterServiceClassRef.Name, instance.Spec.ClusterServiceClassExternalName, len(servicePlans.Items),
			)
			glog.Warning(pcb.Message(s))
			c.updateServiceInstanceCondition(
				instance,
				v1beta1.ServiceInstanceConditionReady,
				v1beta1.ConditionFalse,
				errorNonexistentClusterServicePlanReason,
				"The instance references a ClusterServicePlan that does not exist. "+s,
			)
			c.recorder.Event(instance, corev1.EventTypeWarning, errorNonexistentClusterServicePlanReason, s)
			return nil, fmt.Errorf(s)
		}
	} else if instance.Spec.ClusterServicePlanName != "" {
		sp, err := c.servicePlanLister.Get(instance.Spec.ClusterServicePlanName)
		if err == nil {
			instance.Spec.ClusterServicePlanRef = &v1beta1.ClusterObjectReference{
				Name: sp.Name,
			}
			glog.V(4).Info(pcb.Messagef(
				"resolved ClusterServicePlan with K8S name %q to ClusterServicePlan with external name %q",
				instance.Spec.ClusterServicePlanName, sp.Spec.ExternalName,
			))
		} else {
			s := fmt.Sprintf(
				"References a non-existent ClusterServicePlan with K8S name %q on ClusterServiceClass with K8S name %q",
				instance.Spec.ClusterServicePlanName, instance.Spec.ClusterServiceClassName,
			)
			glog.Warning(pcb.Message(s))
			c.updateServiceInstanceCondition(
				instance,
				v1beta1.ServiceInstanceConditionReady,
				v1beta1.ConditionFalse,
				errorNonexistentClusterServicePlanReason,
				"The instance references a ClusterServicePlan that does not exist. "+s,
			)
			c.recorder.Event(instance, corev1.EventTypeWarning, errorNonexistentClusterServicePlanReason, s)
			return nil, fmt.Errorf(s)
		}
	} else {
		// ServiceInstance is in invalid state, should not ever happen. check
		return nil, fmt.Errorf("ServiceInstance is in inconsistent state, neither ClusterServicePlanExternalName nor ClusterServicePlanName is set: %+v", instance.Spec)
	}
	return instance, nil
}

// newServiceInstanceReadyCondition is a helper function that returns a Ready
// condition with the given status, reason, and message, with its transition
// time set to now.
func newServiceInstanceReadyCondition(status v1beta1.ConditionStatus, reason, message string) *v1beta1.ServiceInstanceCondition {
	return &v1beta1.ServiceInstanceCondition{
		Type:               v1beta1.ServiceInstanceConditionReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
}

// newServiceInstanceFailedCondition is a helper function that returns a Failed
// condition with the given status, reason, and message, with its transition
// time set to now.
func newServiceInstanceFailedCondition(status v1beta1.ConditionStatus, reason, message string) *v1beta1.ServiceInstanceCondition {
	return &v1beta1.ServiceInstanceCondition{
		Type:               v1beta1.ServiceInstanceConditionFailed,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
}

// setServiceInstanceCondition sets a single condition on an Instance's status: if
// the condition already exists in the status, it is mutated; if the condition
// does not already exist in the status, it is added.  Other conditions in the
// status are not altered.  If the condition exists and its status changes,
// the LastTransitionTime field is updated.
//
// Note: objects coming from informers should never be mutated; always pass a
// deep copy as the instance parameter.
func setServiceInstanceCondition(toUpdate *v1beta1.ServiceInstance,
	conditionType v1beta1.ServiceInstanceConditionType,
	status v1beta1.ConditionStatus,
	reason,
	message string) {
	setServiceInstanceConditionInternal(toUpdate, conditionType, status, reason, message, metav1.Now())
}

// setServiceInstanceConditionInternal is setServiceInstanceCondition but allows the time to
// be parameterized for testing.
func setServiceInstanceConditionInternal(toUpdate *v1beta1.ServiceInstance,
	conditionType v1beta1.ServiceInstanceConditionType,
	status v1beta1.ConditionStatus,
	reason,
	message string,
	t metav1.Time) {

	pcb := pretty.NewContextBuilder(pretty.ServiceInstance, toUpdate.Namespace, toUpdate.Name)
	glog.Info(pcb.Message(message))
	glog.V(5).Info(pcb.Messagef(
		"Setting condition %q to %v",
		conditionType, status,
	))

	newCondition := v1beta1.ServiceInstanceCondition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	}

	if len(toUpdate.Status.Conditions) == 0 {
		glog.V(3).Info(pcb.Messagef(
			"Setting lastTransitionTime, condition %q to %v",
			conditionType, t,
		))
		newCondition.LastTransitionTime = t
		toUpdate.Status.Conditions = []v1beta1.ServiceInstanceCondition{newCondition}
		return
	}

	for i, cond := range toUpdate.Status.Conditions {
		if cond.Type == conditionType {
			if cond.Status != newCondition.Status {
				glog.V(3).Info(pcb.Messagef("Found status change, condition %q: %q -> %q; setting lastTransitionTime to %v",
					conditionType, cond.Status, status, t,
				))
				newCondition.LastTransitionTime = t
			} else {
				newCondition.LastTransitionTime = cond.LastTransitionTime
			}

			toUpdate.Status.Conditions[i] = newCondition
			return
		}
	}

	glog.V(3).Info(pcb.Messagef(
		"Setting lastTransitionTime, condition %q to %v",
		conditionType, t,
	))
	newCondition.LastTransitionTime = t
	toUpdate.Status.Conditions = append(toUpdate.Status.Conditions, newCondition)
}

// updateServiceInstanceReferences updates the refs for the given instance.
func (c *controller) updateServiceInstanceReferences(toUpdate *v1beta1.ServiceInstance) (*v1beta1.ServiceInstance, error) {
	pcb := pretty.NewContextBuilder(pretty.ServiceInstance, toUpdate.Namespace, toUpdate.Name)
	glog.V(4).Info(pcb.Message("Updating references"))
	updatedInstance, err := c.serviceCatalogClient.ServiceInstances(toUpdate.Namespace).UpdateReferences(toUpdate)
	if err != nil {
		glog.Errorf(pcb.Messagef("Failed to update references: %v", err))
	}
	return updatedInstance, err
}

// updateServiceInstanceStatus updates the status for the given instance.
//
// Note: objects coming from informers should never be mutated; the instance
// passed to this method should always be a deep copy.
func (c *controller) updateServiceInstanceStatus(toUpdate *v1beta1.ServiceInstance) (*v1beta1.ServiceInstance, error) {
	pcb := pretty.NewContextBuilder(pretty.ServiceInstance, toUpdate.Namespace, toUpdate.Name)
	glog.V(4).Info(pcb.Message("Updating status"))
	updatedInstance, err := c.serviceCatalogClient.ServiceInstances(toUpdate.Namespace).UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf(pcb.Messagef("Failed to update status: %v", err))
	}

	return updatedInstance, err
}

// updateServiceInstanceCondition updates the given condition for the given Instance
// with the given status, reason, and message.
func (c *controller) updateServiceInstanceCondition(
	instance *v1beta1.ServiceInstance,
	conditionType v1beta1.ServiceInstanceConditionType,
	status v1beta1.ConditionStatus,
	reason,
	message string) error {
	pcb := pretty.NewContextBuilder(pretty.ServiceInstance, instance.Namespace, instance.Name)
	toUpdate := instance.DeepCopy()

	setServiceInstanceCondition(toUpdate, conditionType, status, reason, message)

	glog.V(4).Info(pcb.Messagef("Updating %v condition to %v", conditionType, status))
	_, err := c.serviceCatalogClient.ServiceInstances(instance.Namespace).UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf(pcb.Messagef("Failed to update condition %v to true: %v", conditionType, err))
	}

	return err
}

// recordStartOfServiceInstanceOperation updates the instance to indicate that
// there is an operation being performed. If the instance was already
// performing a different operation, that operation is replaced. The Status of
// the instance is recorded in the registry.
// params:
// toUpdate - a modifiable copy of the instance in the registry to update
// operation - operation that is being performed on the instance
// returns:
// 1 - a modifiable copy of the updated instance in the registry; or toUpdate
//     if there was an error
// 2 - any error that occurred
func (c *controller) recordStartOfServiceInstanceOperation(toUpdate *v1beta1.ServiceInstance, operation v1beta1.ServiceInstanceOperation, inProgressProperties *v1beta1.ServiceInstancePropertiesState) (*v1beta1.ServiceInstance, error) {
	currentReconciledGeneration := toUpdate.Status.ReconciledGeneration
	clearServiceInstanceCurrentOperation(toUpdate)

	toUpdate.Status.ReconciledGeneration = currentReconciledGeneration
	toUpdate.Status.CurrentOperation = operation
	now := metav1.Now()
	toUpdate.Status.OperationStartTime = &now
	reason := ""
	message := ""
	switch operation {
	case v1beta1.ServiceInstanceOperationProvision:
		reason = provisioningInFlightReason
		message = provisioningInFlightMessage
		toUpdate.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusRequired
		toUpdate.Status.InProgressProperties = inProgressProperties
	case v1beta1.ServiceInstanceOperationUpdate:
		reason = instanceUpdatingInFlightReason
		message = instanceUpdatingInFlightMessage
		toUpdate.Status.InProgressProperties = inProgressProperties
	case v1beta1.ServiceInstanceOperationDeprovision:
		reason = deprovisioningInFlightReason
		message = deprovisioningInFlightMessage
	}
	setServiceInstanceCondition(
		toUpdate,
		v1beta1.ServiceInstanceConditionReady,
		v1beta1.ConditionFalse,
		reason,
		message,
	)
	return c.updateServiceInstanceStatus(toUpdate)
}

// checkForRemovedClassAndPlan looks at serviceClass and servicePlan and
// if either has been deleted, will block a new instance creation. If
//
func (c *controller) checkForRemovedClassAndPlan(instance *v1beta1.ServiceInstance, serviceClass *v1beta1.ClusterServiceClass, servicePlan *v1beta1.ClusterServicePlan) error {
	classDeleted := serviceClass.Status.RemovedFromBrokerCatalog
	planDeleted := servicePlan.Status.RemovedFromBrokerCatalog

	if !classDeleted && !planDeleted {
		// Neither has been deleted, life's good.
		return nil
	}

	isProvisioning := false
	if instance.Status.ReconciledGeneration == 0 {
		isProvisioning = true
	}

	// Regardless of what's been deleted, you can always update
	// parameters (ie, not change plans)
	if !isProvisioning && instance.Status.ExternalProperties != nil &&
		servicePlan.Spec.ExternalID == instance.Status.ExternalProperties.ClusterServicePlanExternalID {
		// Service Instance has already been provisioned and we're only
		// updating parameters, so let it through.
		return nil
	}

	// At this point we know that plan is being changed
	if planDeleted {
		return &operationError{
			reason:  errorDeletedClusterServicePlanReason,
			message: fmt.Sprintf("%s has been deleted; cannot provision.", pretty.ClusterServicePlanName(servicePlan)),
		}
	}

	return &operationError{
		reason:  errorDeletedClusterServiceClassReason,
		message: fmt.Sprintf("%s has been deleted; cannot provision.", pretty.ClusterServiceClassName(serviceClass)),
	}
}

// clearServiceInstanceCurrentOperation sets the fields of the instance's Status
// to indicate that there is no current operation being performed. The Status
// is *not* recorded in the registry.
func clearServiceInstanceCurrentOperation(toUpdate *v1beta1.ServiceInstance) {
	toUpdate.Status.CurrentOperation = ""
	toUpdate.Status.OperationStartTime = nil
	toUpdate.Status.AsyncOpInProgress = false
	toUpdate.Status.OrphanMitigationInProgress = false
	toUpdate.Status.LastOperation = nil
	toUpdate.Status.InProgressProperties = nil
	toUpdate.Status.ReconciledGeneration = toUpdate.Generation
}

// rollbackReconciledGenerationOnDeletion resets the ReconciledGeneration if a
// deletion was performed while an async provision or update is running.
// TODO: rework saving off current generation as the start of the async
// operation, see PR 1708/Issue 1587.
func rollbackReconciledGenerationOnDeletion(instance *v1beta1.ServiceInstance, currentReconciledGeneration int64) {
	if instance.DeletionTimestamp != nil {
		glog.V(4).Infof("Not updating ReconciledGeneration after async operation because there is a deletion pending.")
		instance.Status.ReconciledGeneration = currentReconciledGeneration
	}
}

// serviceInstanceHasExistingBindings returns true if there are any existing
// bindings associated with the given ServiceInstance.
func (c *controller) checkServiceInstanceHasExistingBindings(instance *v1beta1.ServiceInstance) error {
	bindingLister := c.bindingLister.ServiceBindings(instance.Namespace)

	selector := labels.NewSelector()
	bindingList, err := bindingLister.List(selector)
	if err != nil {
		return err
	}

	for _, binding := range bindingList {
		// Note that as we are potentially looking at a stale binding resource
		// and cannot rely on UnbindStatus == ServiceBindingUnbindStatusNotRequired
		// to filter out binding requests that have yet to be sent to the broker.
		if instance.Name == binding.Spec.ServiceInstanceRef.Name {
			return &operationError{
				reason:  errorDeprovisionBlockedByCredentialsReason,
				message: "All associated ServiceBindings must be removed before this ServiceInstance can be deleted",
			}
		}
	}

	return nil
}

// requestHelper is a helper struct with properties common to multiple request
// types.
type requestHelper struct {
	ns                   *corev1.Namespace
	parameters           map[string]interface{}
	inProgressProperties *v1beta1.ServiceInstancePropertiesState
	originatingIdentity  *osb.OriginatingIdentity
	requestContext       map[string]interface{}
}

// prepareRequestHelper is a helper function that generates a struct with
// properties common to multiple request types.
func (c *controller) prepareRequestHelper(instance *v1beta1.ServiceInstance, serviceClass *v1beta1.ClusterServiceClass, servicePlan *v1beta1.ClusterServicePlan) (*requestHelper, error) {
	rh := &requestHelper{}

	if utilfeature.DefaultFeatureGate.Enabled(scfeatures.OriginatingIdentity) {
		originatingIdentity, err := buildOriginatingIdentity(instance.Spec.UserInfo)
		if err != nil {
			return nil, &operationError{
				reason:  errorWithOriginatingIdentity,
				message: fmt.Sprintf("Error building originating identity headers: %v", err),
			}
		}
		rh.originatingIdentity = originatingIdentity
	}

	reconciliationAction := getReconciliationActionForServiceInstance(instance)
	if reconciliationAction == reconcileDelete || reconciliationAction == reconcilePoll {
		return rh, nil
	}

	// Only prepare namespace, parameters, and context for provision/update
	ns, err := c.kubeClient.CoreV1().Namespaces().Get(instance.Namespace, metav1.GetOptions{})
	if err != nil {
		return nil, &operationError{
			reason:  errorFindingNamespaceServiceInstanceReason,
			message: fmt.Sprintf("Failed to get namespace %q: %s", instance.Namespace, err),
		}
	}
	rh.ns = ns

	parameters, parametersChecksum, rawParametersWithRedaction, err := prepareInProgressPropertyParameters(
		c.kubeClient,
		instance.Namespace,
		instance.Spec.Parameters,
		instance.Spec.ParametersFrom,
	)
	if err != nil {
		return nil, &operationError{
			reason:  errorWithParameters,
			message: err.Error(),
		}
	}
	rh.parameters = parameters

	rh.inProgressProperties = &v1beta1.ServiceInstancePropertiesState{
		ClusterServicePlanExternalName: servicePlan.Spec.ExternalName,
		ClusterServicePlanExternalID:   servicePlan.Spec.ExternalID,
		Parameters:                     rawParametersWithRedaction,
		ParametersChecksum:             parametersChecksum,
		UserInfo:                       instance.Spec.UserInfo,
	}

	// osb client handles whether or not to really send this based
	// on the version of the client.
	rh.requestContext = map[string]interface{}{
		"platform":  ContextProfilePlatformKubernetes,
		"namespace": instance.Namespace,
	}

	return rh, nil
}

// prepareProvisionRequest creates a provision request object to be passed to
// the broker client to provision the given instance.
func (c *controller) prepareProvisionRequest(instance *v1beta1.ServiceInstance, serviceClass *v1beta1.ClusterServiceClass, servicePlan *v1beta1.ClusterServicePlan) (*osb.ProvisionRequest, *v1beta1.ServiceInstancePropertiesState, error) {
	rh, err := c.prepareRequestHelper(instance, serviceClass, servicePlan)
	if err != nil {
		return nil, nil, err
	}

	request := &osb.ProvisionRequest{
		AcceptsIncomplete:   true,
		InstanceID:          instance.Spec.ExternalID,
		ServiceID:           serviceClass.Spec.ExternalID,
		PlanID:              servicePlan.Spec.ExternalID,
		Parameters:          rh.parameters,
		OrganizationGUID:    string(rh.ns.UID),
		SpaceGUID:           string(rh.ns.UID),
		Context:             rh.requestContext,
		OriginatingIdentity: rh.originatingIdentity,
	}

	return request, rh.inProgressProperties, nil
}

// prepareUpdateInstanceRequest creates an update instance request object to be
// passed to the broker client to update the given instance.
func (c *controller) prepareUpdateInstanceRequest(instance *v1beta1.ServiceInstance, serviceClass *v1beta1.ClusterServiceClass, servicePlan *v1beta1.ClusterServicePlan) (*osb.UpdateInstanceRequest, *v1beta1.ServiceInstancePropertiesState, error) {
	rh, err := c.prepareRequestHelper(instance, serviceClass, servicePlan)
	if err != nil {
		return nil, nil, err
	}

	request := &osb.UpdateInstanceRequest{
		AcceptsIncomplete:   true,
		InstanceID:          instance.Spec.ExternalID,
		ServiceID:           serviceClass.Spec.ExternalID,
		Context:             rh.requestContext,
		OriginatingIdentity: rh.originatingIdentity,
	}

	// Only send the plan ID if the plan ID has changed from what the Broker has
	if instance.Status.ExternalProperties == nil ||
		servicePlan.Spec.ExternalID != instance.Status.ExternalProperties.ClusterServicePlanExternalID {
		planID := servicePlan.Spec.ExternalID
		request.PlanID = &planID
	}
	// Only send the parameters if they have changed from what the Broker has
	if instance.Status.ExternalProperties == nil ||
		rh.inProgressProperties.ParametersChecksum != instance.Status.ExternalProperties.ParametersChecksum {
		if rh.parameters != nil {
			request.Parameters = rh.parameters
		} else {
			request.Parameters = make(map[string]interface{})
		}
	}

	return request, rh.inProgressProperties, nil
}

// prepareDeprovisionRequest creates a deprovision request object to be passed
// to the broker client to deprovision the given instance.
func (c *controller) prepareDeprovisionRequest(instance *v1beta1.ServiceInstance, serviceClass *v1beta1.ClusterServiceClass, servicePlan *v1beta1.ClusterServicePlan) (*osb.DeprovisionRequest, error) {
	rh, err := c.prepareRequestHelper(instance, serviceClass, servicePlan)
	if err != nil {
		return nil, err
	}

	var servicePlanExternalID string
	if instance.Status.ExternalProperties != nil {
		servicePlanExternalID = instance.Status.ExternalProperties.ClusterServicePlanExternalID
	} else if servicePlan != nil {
		servicePlanExternalID = servicePlan.Spec.ExternalID
	} else {
		return nil, &operationError{
			reason:  errorUnknownServicePlanReason,
			message: errorUnknownServicePlanMessage,
		}
	}

	request := &osb.DeprovisionRequest{
		InstanceID:          instance.Spec.ExternalID,
		ServiceID:           serviceClass.Spec.ExternalID,
		PlanID:              servicePlanExternalID,
		OriginatingIdentity: rh.originatingIdentity,
		AcceptsIncomplete:   true,
	}

	return request, nil
}

// preparePollServiceInstanceRequest creates a request object to be passed to
// the broker client to query the given instance's last operation endpoint.
func (c *controller) prepareServiceInstanceLastOperationRequest(instance *v1beta1.ServiceInstance, serviceClass *v1beta1.ClusterServiceClass, servicePlan *v1beta1.ClusterServicePlan) (*osb.LastOperationRequest, error) {
	rh, err := c.prepareRequestHelper(instance, serviceClass, servicePlan)
	if err != nil {
		return nil, err
	}

	request := &osb.LastOperationRequest{
		InstanceID:          instance.Spec.ExternalID,
		ServiceID:           &serviceClass.Spec.ExternalID,
		PlanID:              &servicePlan.Spec.ExternalID,
		OriginatingIdentity: rh.originatingIdentity,
	}
	if instance.Status.LastOperation != nil && *instance.Status.LastOperation != "" {
		key := osb.OperationKey(*instance.Status.LastOperation)
		request.OperationKey = &key
	}

	return request, nil
}

// processServiceInstanceGracefulDeletionSuccess handles the logging and
// updating of a ServiceInstance that has successfully finished graceful
// deletion.
func (c *controller) processServiceInstanceGracefulDeletionSuccess(instance *v1beta1.ServiceInstance) error {
	finalizers := sets.NewString(instance.Finalizers...)
	finalizers.Delete(v1beta1.FinalizerServiceCatalog)
	instance.Finalizers = finalizers.List()

	if _, err := c.updateServiceInstanceStatus(instance); err != nil {
		return err
	}

	pcb := pretty.NewContextBuilder(pretty.ServiceInstance, instance.Namespace, instance.Name)
	glog.Info(pcb.Message("Cleared finalizer"))

	return nil
}

// handleServiceInstanceReconciliationError is a helper function that handles
// on error whether the error represents an operation error and should update
// the ServiceInstance resource.
func (c *controller) handleServiceInstanceReconciliationError(instance *v1beta1.ServiceInstance, err error) error {
	if resourceErr, ok := err.(*operationError); ok {
		status := v1beta1.ConditionFalse
		if instance.Status.CurrentOperation == v1beta1.ServiceInstanceOperationDeprovision {
			status = v1beta1.ConditionUnknown
		}
		readyCond := newServiceInstanceReadyCondition(status, resourceErr.reason, resourceErr.message)
		return c.processServiceInstanceOperationError(instance, readyCond)
	}
	return err
}

// processServiceInstanceOperationError handles the logging and updating of
// a ServiceInstance that hit a retryable error during reconciliation.
func (c *controller) processServiceInstanceOperationError(instance *v1beta1.ServiceInstance, readyCond *v1beta1.ServiceInstanceCondition) error {
	setServiceInstanceCondition(instance, v1beta1.ServiceInstanceConditionReady, readyCond.Status, readyCond.Reason, readyCond.Message)
	if _, err := c.updateServiceInstanceStatus(instance); err != nil {
		return err
	}

	c.recorder.Event(instance, corev1.EventTypeWarning, readyCond.Reason, readyCond.Message)

	// The result of this function should be directly returned from the
	// reconciler, so it is necessary to return an error to tell the worker
	// to retry reconciling the resource.
	return fmt.Errorf(readyCond.Message)
}

// processProvisionSuccess handles the logging and updating of a
// ServiceInstance that has successfully been provisioned at the broker.
func (c *controller) processProvisionSuccess(instance *v1beta1.ServiceInstance, dashboardURL *string) error {
	setServiceInstanceDashboardURL(instance, dashboardURL)
	setServiceInstanceCondition(instance, v1beta1.ServiceInstanceConditionReady, v1beta1.ConditionTrue, successProvisionReason, successProvisionMessage)
	instance.Status.ExternalProperties = instance.Status.InProgressProperties
	currentReconciledGeneration := instance.Status.ReconciledGeneration
	clearServiceInstanceCurrentOperation(instance)
	rollbackReconciledGenerationOnDeletion(instance, currentReconciledGeneration)

	if _, err := c.updateServiceInstanceStatus(instance); err != nil {
		return err
	}

	c.recorder.Eventf(instance, corev1.EventTypeNormal, successProvisionReason, successProvisionMessage)
	return nil
}

// processProvisionFailure handles the logging and updating of a
// ServiceInstance that hit a terminal failure during provision reconciliation.
func (c *controller) processProvisionFailure(instance *v1beta1.ServiceInstance, readyCond, failedCond *v1beta1.ServiceInstanceCondition, shouldMitigateOrphan bool) error {
	currentReconciledGeneration := instance.Status.ReconciledGeneration
	if failedCond == nil {
		return fmt.Errorf("failedCond must not be nil")
	}

	if readyCond != nil {
		c.recorder.Event(instance, corev1.EventTypeWarning, readyCond.Reason, readyCond.Message)
		setServiceInstanceCondition(instance, v1beta1.ServiceInstanceConditionReady, readyCond.Status, readyCond.Reason, readyCond.Message)
	}

	c.recorder.Event(instance, corev1.EventTypeWarning, failedCond.Reason, failedCond.Message)
	setServiceInstanceCondition(instance, v1beta1.ServiceInstanceConditionFailed, failedCond.Status, failedCond.Reason, failedCond.Message)

	// Need to vary return error depending on whether the worker should
	// requeue this resource.
	var err error
	if shouldMitigateOrphan {
		readyCond := newServiceInstanceReadyCondition(v1beta1.ConditionFalse, startingInstanceOrphanMitigationReason, startingInstanceOrphanMitigationMessage)
		c.recorder.Event(instance, corev1.EventTypeWarning, readyCond.Reason, readyCond.Message)
		setServiceInstanceCondition(instance, v1beta1.ServiceInstanceConditionReady, readyCond.Status, readyCond.Reason, readyCond.Message)

		instance.Status.OperationStartTime = nil
		instance.Status.AsyncOpInProgress = false
		instance.Status.OrphanMitigationInProgress = true

		err = fmt.Errorf(failedCond.Message)
	} else {
		clearServiceInstanceCurrentOperation(instance)
		rollbackReconciledGenerationOnDeletion(instance, currentReconciledGeneration)
	}

	if _, err := c.updateServiceInstanceStatus(instance); err != nil {
		return err
	}

	return err
}

// processProvisionAsyncResponse handles the logging and updating
// of a ServiceInstance that received an asynchronous response from the broker
// when requesting a provision.
func (c *controller) processProvisionAsyncResponse(instance *v1beta1.ServiceInstance, response *osb.ProvisionResponse) error {
	setServiceInstanceDashboardURL(instance, response.DashboardURL)
	setServiceInstanceLastOperation(instance, response.OperationKey)
	setServiceInstanceCondition(instance, v1beta1.ServiceInstanceConditionReady, v1beta1.ConditionFalse, asyncProvisioningReason, asyncProvisioningMessage)
	instance.Status.AsyncOpInProgress = true

	if _, err := c.updateServiceInstanceStatus(instance); err != nil {
		return err
	}

	c.recorder.Event(instance, corev1.EventTypeNormal, asyncProvisioningReason, asyncProvisioningMessage)
	return c.beginPollingServiceInstance(instance)
}

// processUpdateServiceInstanceSuccess handles the logging and updating of a
// ServiceInstance that has successfully been updated at the broker.
func (c *controller) processUpdateServiceInstanceSuccess(instance *v1beta1.ServiceInstance) error {
	setServiceInstanceCondition(instance, v1beta1.ServiceInstanceConditionReady, v1beta1.ConditionTrue, successUpdateInstanceReason, successUpdateInstanceMessage)
	instance.Status.ExternalProperties = instance.Status.InProgressProperties
	currentReconciledGeneration := instance.Status.ReconciledGeneration
	clearServiceInstanceCurrentOperation(instance)
	rollbackReconciledGenerationOnDeletion(instance, currentReconciledGeneration)

	if _, err := c.updateServiceInstanceStatus(instance); err != nil {
		return err
	}

	c.recorder.Eventf(instance, corev1.EventTypeNormal, successUpdateInstanceReason, successUpdateInstanceMessage)
	return nil
}

// processUpdateServiceInstanceFailure handles the logging and updating of a
// ServiceInstance that hit a terminal failure during update reconciliation.
func (c *controller) processUpdateServiceInstanceFailure(instance *v1beta1.ServiceInstance, readyCond *v1beta1.ServiceInstanceCondition) error {
	c.recorder.Event(instance, corev1.EventTypeWarning, readyCond.Reason, readyCond.Message)
	currentReconciledGeneration := instance.Status.ReconciledGeneration

	setServiceInstanceCondition(instance, v1beta1.ServiceInstanceConditionReady, readyCond.Status, readyCond.Reason, readyCond.Message)
	clearServiceInstanceCurrentOperation(instance)
	rollbackReconciledGenerationOnDeletion(instance, currentReconciledGeneration)

	if _, err := c.updateServiceInstanceStatus(instance); err != nil {
		return err
	}

	return nil
}

// processUpdateServiceInstanceAsyncResponse handles the logging and updating
// of a ServiceInstance that received an asynchronous response from the broker
// when requesting an instance update.
func (c *controller) processUpdateServiceInstanceAsyncResponse(instance *v1beta1.ServiceInstance, response *osb.UpdateInstanceResponse) error {
	setServiceInstanceLastOperation(instance, response.OperationKey)
	setServiceInstanceCondition(instance, v1beta1.ServiceInstanceConditionReady, v1beta1.ConditionFalse, asyncUpdatingInstanceReason, asyncUpdatingInstanceMessage)
	instance.Status.AsyncOpInProgress = true

	if _, err := c.updateServiceInstanceStatus(instance); err != nil {
		return err
	}

	c.recorder.Event(instance, corev1.EventTypeNormal, asyncUpdatingInstanceReason, asyncUpdatingInstanceMessage)
	return c.beginPollingServiceInstance(instance)
}

// processDeprovisionSuccess handles the logging and updating of
// a ServiceInstance that has successfully been deprovisioned at the broker.
func (c *controller) processDeprovisionSuccess(instance *v1beta1.ServiceInstance) error {
	mitigatingOrphan := instance.Status.OrphanMitigationInProgress

	reason := successDeprovisionReason
	msg := successDeprovisionMessage
	if mitigatingOrphan {
		reason = successOrphanMitigationReason
		msg = successOrphanMitigationMessage
	}

	setServiceInstanceCondition(instance, v1beta1.ServiceInstanceConditionReady, v1beta1.ConditionFalse, reason, msg)
	clearServiceInstanceCurrentOperation(instance)
	instance.Status.ExternalProperties = nil
	instance.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusSucceeded

	if mitigatingOrphan {
		if _, err := c.updateServiceInstanceStatus(instance); err != nil {
			return err
		}
	} else {
		// If part of a resource deletion request, follow-through to the
		// graceful deletion handler in order to clear the finalizer.
		if err := c.processServiceInstanceGracefulDeletionSuccess(instance); err != nil {
			return err
		}
	}

	c.recorder.Event(instance, corev1.EventTypeNormal, reason, msg)
	return nil
}

// processDeprovisionFailure handles the logging and updating of a
// ServiceInstance that hit a terminal failure during deprovision
// reconciliation.
func (c *controller) processDeprovisionFailure(instance *v1beta1.ServiceInstance, readyCond, failedCond *v1beta1.ServiceInstanceCondition) error {
	if failedCond == nil {
		return fmt.Errorf("failedCond must not be nil")
	}

	if instance.Status.OrphanMitigationInProgress {
		// replace Ready condition with orphan mitigation-related one.
		msg := "Orphan mitigation failed: " + failedCond.Message
		readyCond := newServiceInstanceReadyCondition(v1beta1.ConditionUnknown, errorOrphanMitigationFailedReason, msg)

		setServiceInstanceCondition(instance, v1beta1.ServiceInstanceConditionReady, readyCond.Status, readyCond.Reason, readyCond.Message)
		c.recorder.Event(instance, corev1.EventTypeWarning, readyCond.Reason, readyCond.Message)
	} else {
		if readyCond != nil {
			setServiceInstanceCondition(instance, v1beta1.ServiceInstanceConditionReady, v1beta1.ConditionUnknown, readyCond.Reason, readyCond.Message)
			c.recorder.Event(instance, corev1.EventTypeWarning, readyCond.Reason, readyCond.Message)
		}

		setServiceInstanceCondition(instance, v1beta1.ServiceInstanceConditionFailed, failedCond.Status, failedCond.Reason, failedCond.Message)
		c.recorder.Event(instance, corev1.EventTypeWarning, failedCond.Reason, failedCond.Message)
	}

	clearServiceInstanceCurrentOperation(instance)
	instance.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusFailed

	if _, err := c.updateServiceInstanceStatus(instance); err != nil {
		return err
	}

	return nil
}

// processDeprovisionAsyncResponse handles the logging and
// updating of a ServiceInstance that received an asynchronous response from
// the broker when requesting a deprovision.
func (c *controller) processDeprovisionAsyncResponse(instance *v1beta1.ServiceInstance, response *osb.DeprovisionResponse) error {
	setServiceInstanceLastOperation(instance, response.OperationKey)
	setServiceInstanceCondition(instance, v1beta1.ServiceInstanceConditionReady, v1beta1.ConditionFalse, asyncDeprovisioningReason, asyncDeprovisioningMessage)
	instance.Status.AsyncOpInProgress = true

	if _, err := c.updateServiceInstanceStatus(instance); err != nil {
		return err
	}

	c.recorder.Event(instance, corev1.EventTypeNormal, asyncDeprovisioningReason, asyncDeprovisioningMessage)
	return c.beginPollingServiceInstance(instance)
}

// handleServiceInstancePollingError is a helper function that handles logic for
// an error returned during reconciliation while polling a service instance.
func (c *controller) handleServiceInstancePollingError(instance *v1beta1.ServiceInstance, err error) error {
	// During polling, an error means we should:
	//	1) log the error
	//	2) attempt to requeue in the polling queue
	//		- if successful, we can return nil to avoid regular queue
	//		- if failure, return err to fall back to regular queue
	pcb := pretty.NewContextBuilder(pretty.ServiceInstance, instance.Namespace, instance.Name)
	glog.V(4).Info(pcb.Messagef("Error during polling: %v", err))
	return c.continuePollingServiceInstance(instance)
}

// setServiceInstanceDashboardURL sets the dashboard URL on the given instance.
func setServiceInstanceDashboardURL(instance *v1beta1.ServiceInstance, dashboardURL *string) {
	if dashboardURL != nil && *dashboardURL != "" {
		url := *dashboardURL
		instance.Status.DashboardURL = &url
	}
}

// setServiceInstanceLastOperation sets the last operation key on the given
// instance.
func setServiceInstanceLastOperation(instance *v1beta1.ServiceInstance, operationKey *osb.OperationKey) {
	if operationKey != nil && *operationKey != "" {
		key := string(*operationKey)
		instance.Status.LastOperation = &key
	}
}

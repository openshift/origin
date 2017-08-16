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

	checksum "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/checksum/versioned/v1alpha1"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	"github.com/kubernetes-incubator/service-catalog/pkg/brokerapi"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/tools/cache"
)

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

// reconcileInstanceDelete is responsible for handling any instance whose deletion timestamp is set.
func (c *controller) reconcileInstanceDelete(instance *v1alpha1.Instance) error {
	// nothing to do...
	if instance.DeletionTimestamp == nil {
		return nil
	}

	finalizerToken := v1alpha1.FinalizerServiceCatalog
	finalizers := sets.NewString(instance.Finalizers...)
	if !finalizers.Has(finalizerToken) {
		return nil
	}

	// if there is no op in progress, and the instance was never provisioned, we can just delete.
	// this can happen if the service class name referenced never existed.
	if !instance.Status.AsyncOpInProgress && instance.Status.Checksum == nil {
		finalizers.Delete(finalizerToken)
		// Clear the finalizer
		return c.updateInstanceFinalizers(instance, finalizers.List())
	}

	// All updates not having a DeletingTimestamp will have been handled above
	// and returned early. If we reach this point, we're dealing with an update
	// that's actually a soft delete-- i.e. we have some finalization to do.
	// Since the potential exists for an instance to have multiple finalizers and
	// since those most be cleared in order, we proceed with the soft delete
	// only if it's "our turn--" i.e. only if the finalizer we care about is at
	// the head of the finalizers list.
	serviceClass, servicePlan, brokerName, brokerClient, err := c.getServiceClassPlanAndBroker(instance)
	if err != nil {
		return err
	}

	glog.V(4).Infof("Finalizing Instance %v/%v", instance.Namespace, instance.Name)

	request := &osb.DeprovisionRequest{
		InstanceID:        instance.Spec.ExternalID,
		ServiceID:         serviceClass.ExternalID,
		PlanID:            servicePlan.ExternalID,
		AcceptsIncomplete: true,
	}

	glog.V(4).Infof("Deprovisioning Instance %v/%v of ServiceClass %v at Broker %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName)
	response, err := brokerClient.DeprovisionInstance(request)
	if err != nil {
		httpErr, isError := osb.IsHTTPError(err)
		if isError {
			s := fmt.Sprintf(
				"Error deprovisioning Instance \"%s/%s\" of ServiceClass %q at Broker %q with status code %d: ErrorMessage: %v, Description: %v",
				instance.Namespace,
				instance.Name,
				serviceClass.Name,
				brokerName,
				httpErr.StatusCode,
				httpErr.ErrorMessage,
				httpErr.Description,
			)
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

		s := fmt.Sprintf(
			"Error deprovisioning Instance \"%s/%s\" of ServiceClass %q at Broker %q: %s",
			instance.Namespace,
			instance.Name,
			serviceClass.Name,
			brokerName,
			err,
		)
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

	if response.Async {
		glog.V(5).Infof("Received asynchronous de-provisioning response for Instance %v/%v of ServiceClass %v at Broker %v: response: %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName, response)
		if response.OperationKey != nil && *response.OperationKey != "" {
			key := string(*response.OperationKey)
			instance.Status.LastOperation = &key
		}

		// Tag this instance as having an ongoing async operation so we can enforce
		// no other operations against it can start.
		instance.Status.AsyncOpInProgress = true

		err := c.updateInstanceCondition(
			instance,
			v1alpha1.InstanceConditionReady,
			v1alpha1.ConditionFalse,
			asyncDeprovisioningReason,
			asyncDeprovisioningMessage,
		)
		if err != nil {
			return err
		}
	} else {
		err := c.updateInstanceCondition(
			instance,
			v1alpha1.InstanceConditionReady,
			v1alpha1.ConditionFalse,
			successDeprovisionReason,
			successDeprovisionMessage,
		)
		if err != nil {
			return err
		}

		// Clear the finalizer
		finalizers.Delete(v1alpha1.FinalizerServiceCatalog)
		if err = c.updateInstanceFinalizers(instance, finalizers.List()); err != nil {
			return err
		}
		c.recorder.Event(instance, api.EventTypeNormal, successDeprovisionReason, successDeprovisionMessage)
		glog.V(5).Infof("Successfully deprovisioned Instance %v/%v of ServiceClass %v at Broker %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName)
	}

	return nil
}

// reconcileInstance is the control-loop for reconciling Instances. An
// error is returned to indicate that the binding has not been fully
// processed and should be resubmitted at a later time.
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
				glog.V(4).Infof(
					"Not processing event for Instance %v/%v because checksum showed there is no work to do",
					instance.Namespace,
					instance.Name,
				)
				return nil
			}
		}
	}

	glog.V(4).Infof("Processing Instance %v/%v", instance.Namespace, instance.Name)

	// if the instance is marked for deletion, handle that first.
	if instance.ObjectMeta.DeletionTimestamp != nil {
		glog.V(4).Infof("Soft-deleting Instance %v/%v", instance.Namespace, instance.Name)
		return c.reconcileInstanceDelete(instance)
	}

	serviceClass, servicePlan, brokerName, brokerClient, err := c.getServiceClassPlanAndBroker(instance)
	if err != nil {
		return err
	}

	if instance.Status.AsyncOpInProgress {
		return c.pollInstance(serviceClass, servicePlan, brokerName, brokerClient, instance)
	}

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

	glog.V(4).Infof("Provisioning a new Instance %v/%v of ServiceClass %v at Broker %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName)
	response, err := brokerClient.ProvisionInstance(request)
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

	if response.DashboardURL != nil && *response.DashboardURL != "" {
		instance.Status.DashboardURL = response.DashboardURL
	}

	// Broker can return either a synchronous or asynchronous
	// response, if the response is StatusAccepted it's an async
	// and we need to add it to the polling queue. Broker can
	// optionally return 'Operation' that will then need to be
	// passed back to the broker during polling of last_operation.
	if response.Async {
		glog.V(5).Infof("Received asynchronous provisioning response for Instance %v/%v of ServiceClass %v at Broker %v: response: %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName, response)
		if response.OperationKey != nil && *response.OperationKey != "" {
			key := string(*response.OperationKey)
			instance.Status.LastOperation = &key
		}

		clone, err := api.Scheme.DeepCopy(instance)
		if err != nil {
			return err
		}
		toUpdate := clone.(*v1alpha1.Instance)

		// Tag this instance as having an ongoing async operation so we can enforce
		// no other operations against it can start.
		toUpdate.Status.AsyncOpInProgress = true

		c.updateInstanceCondition(
			toUpdate,
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

func (c *controller) pollInstanceInternal(instance *v1alpha1.Instance) error {
	glog.V(4).Infof("Processing Instance %v/%v", instance.Namespace, instance.Name)

	serviceClass, servicePlan, brokerName, brokerClient, err := c.getServiceClassPlanAndBroker(instance)
	if err != nil {
		return err
	}
	return c.pollInstance(serviceClass, servicePlan, brokerName, brokerClient, instance)
}

func (c *controller) pollInstance(serviceClass *v1alpha1.ServiceClass, servicePlan *v1alpha1.ServicePlan, brokerName string, brokerClient osb.Client, instance *v1alpha1.Instance) error {
	// There are some conditions that are different if we're
	// deleting, this is more readable than checking the
	// timestamps in various places.
	deleting := false
	if instance.DeletionTimestamp != nil {
		deleting = true
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

	glog.V(5).Infof("Polling last operation on Instance %v/%v", instance.Namespace, instance.Name)

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
			toUpdate := clone.(*v1alpha1.Instance)

			toUpdate.Status.AsyncOpInProgress = false
			c.updateInstanceCondition(
				toUpdate,
				v1alpha1.InstanceConditionReady,
				v1alpha1.ConditionFalse,
				successDeprovisionReason,
				successDeprovisionMessage,
			)

			// Clear the finalizer
			if finalizers := sets.NewString(toUpdate.Finalizers...); finalizers.Has(v1alpha1.FinalizerServiceCatalog) {
				finalizers.Delete(v1alpha1.FinalizerServiceCatalog)
				c.updateInstanceFinalizers(toUpdate, finalizers.List())
			}

			c.recorder.Event(instance, api.EventTypeNormal, successDeprovisionReason, successDeprovisionMessage)
			glog.V(5).Infof("Successfully deprovisioned Instance %v/%v of ServiceClass %v at Broker %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName)
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
		return err
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
			toUpdate := clone.(*v1alpha1.Instance)
			toUpdate.Status.AsyncOpInProgress = true

			var message string
			var reason string
			if deleting {
				reason = asyncDeprovisioningMessage
			} else {
				reason = asyncProvisioningReason
			}
			if response.Description != nil {
				message = fmt.Sprintf("%s (%s)", asyncProvisioningMessage, *response.Description)
			} else {
				message = asyncProvisioningMessage
			}
			c.updateInstanceCondition(
				toUpdate,
				v1alpha1.InstanceConditionReady,
				v1alpha1.ConditionFalse,
				reason,
				message,
			)
		}
		return fmt.Errorf("last operation not completed (still in progress) for %v/%v", instance.Namespace, instance.Name)
	case osb.StateSucceeded:
		// Update the instance to reflect that an async operation is no longer
		// in progress.
		clone, err := api.Scheme.DeepCopy(instance)
		if err != nil {
			return err
		}
		toUpdate := clone.(*v1alpha1.Instance)
		toUpdate.Status.AsyncOpInProgress = false

		// If we were asynchronously deleting a Service Instance, finish
		// the finalizers.
		if deleting {
			err := c.updateInstanceCondition(
				toUpdate,
				v1alpha1.InstanceConditionReady,
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
				c.updateInstanceFinalizers(toUpdate, finalizers.List())
			}
			c.recorder.Event(instance, api.EventTypeNormal, successDeprovisionReason, successDeprovisionMessage)
			glog.V(5).Infof("Successfully deprovisioned Instance %v/%v of ServiceClass %v at Broker %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName)
		} else {
			c.updateInstanceCondition(
				toUpdate,
				v1alpha1.InstanceConditionReady,
				v1alpha1.ConditionTrue,
				successProvisionReason,
				successProvisionMessage,
			)
		}
	case osb.StateFailed:
		description := ""
		if response.Description != nil {
			description = *response.Description
		}
		s := fmt.Sprintf("Error deprovisioning Instance \"%s/%s\" of ServiceClass %q at Broker %q: %q", instance.Namespace, instance.Name, serviceClass.Name, brokerName, description)

		clone, err := api.Scheme.DeepCopy(instance)
		if err != nil {
			return err
		}
		toUpdate := clone.(*v1alpha1.Instance)
		toUpdate.Status.AsyncOpInProgress = false

		cond := v1alpha1.ConditionFalse
		reason := errorProvisionCalledReason
		msg := "Provision call failed: " + s
		if deleting {
			cond = v1alpha1.ConditionUnknown
			reason = errorDeprovisionCalledReason
			msg = "Deprovision call failed:" + s
		}
		c.updateInstanceCondition(
			toUpdate,
			v1alpha1.InstanceConditionReady,
			cond,
			reason,
			msg,
		)
		c.recorder.Event(instance, api.EventTypeWarning, errorDeprovisionCalledReason, s)
	default:
		glog.Warningf("Got invalid state in LastOperationResponse: %q", response.State)
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

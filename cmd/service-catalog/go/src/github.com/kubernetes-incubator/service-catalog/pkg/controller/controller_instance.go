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
	"net/http"
	"net/url"
	"time"

	"github.com/golang/glog"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	"k8s.io/apimachinery/pkg/runtime"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"github.com/kubernetes-incubator/service-catalog/pkg/api"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	scfeatures "github.com/kubernetes-incubator/service-catalog/pkg/features"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	apiv1 "k8s.io/client-go/pkg/api/v1"
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
func (c *controller) reconcileServiceInstanceDelete(instance *v1alpha1.ServiceInstance) error {
	// nothing to do...
	if instance.DeletionTimestamp == nil && !instance.Status.OrphanMitigationInProgress {
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
			c.recorder.Event(instance, apiv1.EventTypeWarning, errorDeprovisionBlockedByCredentialsReason, s)

			setServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionFalse,
				errorDeprovisionBlockedByCredentialsReason,
				"Delete instance failed. "+s)
			if _, err = c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}
			return nil
		}
	}

	finalizerToken := v1alpha1.FinalizerServiceCatalog
	finalizers := sets.NewString(instance.Finalizers...)
	if !finalizers.Has(finalizerToken) {
		return nil
	}

	// If there is no op in progress, and the instance either was never
	// provisioned or was already deprovisioned due to orphan mitigation,
	// we can just clear the finalizer and delete. One possible  scenario
	// is if  the service class name referenced never existed.
	if !instance.Status.AsyncOpInProgress &&
		!instance.Status.OrphanMitigationInProgress &&
		(isServiceInstanceFailed(instance) || instance.Status.ReconciledGeneration == 0) {

		glog.V(5).Infof("Clearing catalog finalizer from ServiceInstance %v/%v", instance.Namespace, instance.Name)
		clone, err := api.Scheme.DeepCopy(instance)
		if err != nil {
			return err
		}
		toUpdate := clone.(*v1alpha1.ServiceInstance)
		// Clear the finalizer
		finalizers.Delete(finalizerToken)
		toUpdate.Finalizers = finalizers.List()
		if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
			return err
		}
		return nil
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

	request := &osb.DeprovisionRequest{
		InstanceID:        instance.Spec.ExternalID,
		ServiceID:         serviceClass.Spec.ExternalID,
		PlanID:            servicePlan.Spec.ExternalID,
		AcceptsIncomplete: true,
	}

	if utilfeature.DefaultFeatureGate.Enabled(scfeatures.OriginatingIdentity) {
		originatingIdentity, err := buildOriginatingIdentity(instance.Spec.UserInfo)
		if err != nil {
			s := fmt.Sprintf(`Error building originating identity headers for deprovisioning ServiceInstance "%v/%v": %v`, instance.Namespace, instance.Name, err)
			glog.Warning(s)
			c.recorder.Event(instance, apiv1.EventTypeWarning, errorWithOriginatingIdentity, s)

			setServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionFalse,
				errorWithOriginatingIdentity,
				s,
			)
			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}

			return err
		}
		request.OriginatingIdentity = originatingIdentity
	}

	if instance.Status.OrphanMitigationInProgress && instance.Status.OperationStartTime == nil {
		now := metav1.Now()
		toUpdate.Status.OperationStartTime = &now
	}

	if toUpdate.Status.CurrentOperation == "" {
		toUpdate, err = c.recordStartOfServiceInstanceOperation(toUpdate, v1alpha1.ServiceInstanceOperationDeprovision)
		if err != nil {
			// There has been an update to the instance. Start reconciliation
			// over with a fresh view of the instance.
			return err
		}
	}

	glog.V(4).Infof("Deprovisioning ServiceInstance %v/%v of ServiceClass %v at ServiceBroker %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName)
	response, err := brokerClient.DeprovisionInstance(request)
	if err != nil {
		if httpErr, ok := osb.IsHTTPError(err); ok {
			s := fmt.Sprintf(
				"Error deprovisioning ServiceInstance \"%s/%s\" of ServiceClass %q at ServiceBroker %q with status code %d: ErrorMessage: %v, Description: %v",
				instance.Namespace,
				instance.Name,
				serviceClass.Spec.ExternalName,
				brokerName,
				httpErr.StatusCode,
				httpErr.ErrorMessage,
				httpErr.Description,
			)
			glog.Warning(s)
			c.recorder.Event(instance, apiv1.EventTypeWarning, errorDeprovisionCalledReason, s)

			setServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionUnknown,
				errorDeprovisionCalledReason,
				"Deprovision call failed. "+s)

			if !instance.Status.OrphanMitigationInProgress {
				// Do not overwrite 'Failed' message if deprovisioning due to orphan
				// mitigation in order to prevent loss of original reason for the
				// orphan.
				setServiceInstanceCondition(
					toUpdate,
					v1alpha1.ServiceInstanceConditionFailed,
					v1alpha1.ConditionTrue,
					errorDeprovisionCalledReason,
					s,
				)
			}

			c.clearServiceInstanceCurrentOperation(toUpdate)
			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}
			return nil
		}

		s := fmt.Sprintf(
			"Error deprovisioning ServiceInstance \"%s/%s\" of ServiceClass %q at ServiceBroker %q: %v",
			instance.Namespace,
			instance.Name,
			serviceClass.Name,
			brokerName,
			err,
		)
		glog.Warning(s)
		c.recorder.Event(instance, apiv1.EventTypeWarning, errorDeprovisionCalledReason, s)

		setServiceInstanceCondition(
			toUpdate,
			v1alpha1.ServiceInstanceConditionReady,
			v1alpha1.ConditionUnknown,
			errorDeprovisionCalledReason,
			"Deprovision call failed. "+s)

		if !time.Now().Before(toUpdate.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
			s := fmt.Sprintf(`Stopping reconciliation retries on ServiceInstance "%v/%v" because too much time has elapsed`, instance.Namespace, instance.Name)
			glog.Info(s)
			c.recorder.Event(instance, apiv1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)

			if !instance.Status.OrphanMitigationInProgress {
				setServiceInstanceCondition(toUpdate,
					v1alpha1.ServiceInstanceConditionFailed,
					v1alpha1.ConditionTrue,
					errorReconciliationRetryTimeoutReason,
					s)
			}

			c.clearServiceInstanceCurrentOperation(toUpdate)
			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}
			return nil
		}

		if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
			return err
		}
		return err
	}

	if response.Async {
		glog.V(5).Infof("Received asynchronous de-provisioning response for ServiceInstance %v/%v of ServiceClass %v at ServiceBroker %v: response: %+v", instance.Namespace, instance.Name, serviceClass.Name, brokerName, response)

		if response.OperationKey != nil && *response.OperationKey != "" {
			key := string(*response.OperationKey)
			toUpdate.Status.LastOperation = &key
		}

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
		if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
			return err
		}

		err = c.beginPollingServiceInstance(instance)
		if err != nil {
			return err
		}

		c.recorder.Eventf(instance, apiv1.EventTypeNormal, asyncDeprovisioningReason, asyncDeprovisioningMessage)

		return nil
	}

	glog.V(5).Infof("Deprovision call to broker succeeded for ServiceInstance %v/%v, finalizing", instance.Namespace, instance.Name)

	c.clearServiceInstanceCurrentOperation(toUpdate)
	toUpdate.Status.ExternalProperties = nil

	setServiceInstanceCondition(
		toUpdate,
		v1alpha1.ServiceInstanceConditionReady,
		v1alpha1.ConditionFalse,
		successDeprovisionReason,
		successDeprovisionMessage,
	)

	if instance.DeletionTimestamp != nil {
		// Clear the finalizer for normal instance deletions
		finalizers.Delete(v1alpha1.FinalizerServiceCatalog)
		toUpdate.Finalizers = finalizers.List()
	}

	if _, err = c.updateServiceInstanceStatus(toUpdate); err != nil {
		return err
	}

	c.recorder.Event(instance, apiv1.EventTypeNormal, successDeprovisionReason, successDeprovisionMessage)
	glog.V(5).Infof("Successfully deprovisioned ServiceInstance %v/%v of ServiceClass %v at ServiceBroker %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName)

	return nil
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
// error is returned to indicate that the instance has not been fully
// processed and should be resubmitted at a later time.
func (c *controller) reconcileServiceInstance(instance *v1alpha1.ServiceInstance) error {
	if instance.Status.AsyncOpInProgress {
		return c.pollServiceInstanceInternal(instance)
	}

	if instance.ObjectMeta.DeletionTimestamp != nil || instance.Status.OrphanMitigationInProgress {
		return c.reconcileServiceInstanceDelete(instance)
	}

	// Currently, we only set a failure condition if the initial provision
	// call fails, so if that condition is set, we only need to remove the
	// finalizer from the instance. We will need to reevaluate this logic as
	// we make any changes to capture permanent failure in new cases.
	if isServiceInstanceFailed(instance) {
		glog.V(4).Infof(
			"Not processing event for ServiceInstance %v/%v because status showed that it has failed",
			instance.Namespace,
			instance.Name,
		)
		return nil
	}

	// If there's no async op in progress, determine whether there is a new
	// generation of the object. If the instance's generation does not match
	// the reconciled generation, then there is a new generation, indicating
	// that changes have been made to the instance's spec. If there is an
	// async op in progress, we need to keep polling, hence do not bail if
	// there is not a new generation.
	//
	// Note: currently the instance spec is immutable because we do not yet
	// support plan or parameter updates.  This logic is currently meant only
	// to facilitate re-trying provision requests where there was a problem
	// communicating with the broker.  In the future the same logic will
	// result in an instance that requires update being processed by the
	// controller.
	if instance.Status.ReconciledGeneration == instance.Generation {
		glog.V(4).Infof(
			"Not processing event for ServiceInstance %v/%v because reconciled generation showed there is no work to do",
			instance.Namespace,
			instance.Name,
		)
		return nil
	}

	// we will definitely update the instance's status - make a deep copy now
	// for use later in this method.
	clone, err := api.Scheme.DeepCopy(instance)
	if err != nil {
		return err
	}
	toUpdate := clone.(*v1alpha1.ServiceInstance)

	// Update references to ServicePlan / ServiceClass if necessary.
	toUpdate, err = c.resolveReferences(toUpdate)
	if err != nil {
		return err
	}

	glog.V(4).Infof("Processing ServiceInstance %v/%v", instance.Namespace, instance.Name)

	glog.V(4).Infof("Adding/Updating ServiceInstance %v/%v", instance.Namespace, instance.Name)

	serviceClass, servicePlan, brokerName, brokerClient, err := c.getServiceClassPlanAndServiceBroker(toUpdate)
	if err != nil {
		return err
	}

	ns, err := c.kubeClient.Core().Namespaces().Get(instance.Namespace, metav1.GetOptions{})
	if err != nil {
		s := fmt.Sprintf("Failed to get namespace %q during instance create: %s", instance.Namespace, err)
		glog.Info(s)
		c.recorder.Event(instance, apiv1.EventTypeWarning, errorFindingNamespaceServiceInstanceReason, s)

		setServiceInstanceCondition(
			toUpdate,
			v1alpha1.ServiceInstanceConditionReady,
			v1alpha1.ConditionFalse,
			errorFindingNamespaceServiceInstanceReason,
			"Error finding namespace for instance. "+s,
		)
		if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
			return err
		}

		return err
	}

	var (
		parameters                 map[string]interface{}
		parametersChecksum         string
		rawParametersWithRedaction *runtime.RawExtension
	)
	if instance.Spec.Parameters != nil || instance.Spec.ParametersFrom != nil {
		var parametersWithSecretsRedacted map[string]interface{}
		parameters, parametersWithSecretsRedacted, err = buildParameters(c.kubeClient, instance.Namespace, instance.Spec.ParametersFrom, instance.Spec.Parameters)
		if err != nil {
			s := fmt.Sprintf("Failed to prepare ServiceInstance parameters\n%s\n %s", instance.Spec.Parameters, err)
			glog.Warning(s)
			c.recorder.Event(instance, apiv1.EventTypeWarning, errorWithParameters, s)

			setServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionFalse,
				errorWithParameters,
				s,
			)
			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}

			return err
		}

		parametersChecksum, err = generateChecksumOfParameters(parameters)
		if err != nil {
			s := fmt.Sprintf(`Failed to generate the parameters checksum to store in the Status of ServiceInstance "%s/%s": %s`, instance.Namespace, instance.Name, err)
			glog.Info(s)
			c.recorder.Eventf(instance, apiv1.EventTypeWarning, errorWithParameters, s)
			setServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionFalse,
				errorWithParameters,
				s)
			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}
			return err
		}

		marshalledParametersWithRedaction, err := MarshalRawParameters(parametersWithSecretsRedacted)
		if err != nil {
			s := fmt.Sprintf(`Failed to marshal the parameters to store in the Status of ServiceInstance "%s/%s": %s`, instance.Namespace, instance.Name, err)
			glog.Info(s)
			c.recorder.Eventf(instance, apiv1.EventTypeWarning, errorWithParameters, s)
			setServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionFalse,
				errorWithParameters,
				s)
			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}
			return err
		}

		rawParametersWithRedaction = &runtime.RawExtension{
			Raw: marshalledParametersWithRedaction,
		}
	}

	toUpdate.Status.InProgressProperties = &v1alpha1.ServiceInstancePropertiesState{
		ExternalServicePlanName: servicePlan.Spec.ExternalName,
		Parameters:              rawParametersWithRedaction,
		ParametersChecksum:      parametersChecksum,
		UserInfo:                instance.Spec.UserInfo,
	}

	request := &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        instance.Spec.ExternalID,
		ServiceID:         serviceClass.Spec.ExternalID,
		PlanID:            servicePlan.Spec.ExternalID,
		Parameters:        parameters,
		OrganizationGUID:  string(ns.UID),
		SpaceGUID:         string(ns.UID),
	}

	// osb client handles whether or not to really send this based
	// on the version of the client.
	request.Context = map[string]interface{}{
		"platform":  ContextProfilePlatformKubernetes,
		"namespace": instance.Namespace,
	}

	if utilfeature.DefaultFeatureGate.Enabled(scfeatures.OriginatingIdentity) {
		originatingIdentity, err := buildOriginatingIdentity(instance.Spec.UserInfo)
		if err != nil {
			s := fmt.Sprintf(`Error building originating identity headers for provisioning ServiceInstance "%v/%v": %v`, instance.Namespace, instance.Name, err)
			glog.Warning(s)
			c.recorder.Event(instance, apiv1.EventTypeWarning, errorWithOriginatingIdentity, s)

			setServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionFalse,
				errorWithOriginatingIdentity,
				s,
			)
			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}

			return err
		}
		request.OriginatingIdentity = originatingIdentity
	}

	if toUpdate.Status.CurrentOperation == "" {
		toUpdate, err = c.recordStartOfServiceInstanceOperation(toUpdate, v1alpha1.ServiceInstanceOperationProvision)
		if err != nil {
			// There has been an update to the instance. Start reconciliation
			// over with a fresh view of the instance.
			return err
		}
	}

	glog.V(4).Infof("Provisioning a new ServiceInstance %v/%v of ServiceClass %v at ServiceBroker %v", instance.Namespace, instance.Name, serviceClass.Spec.ExternalName, brokerName)
	response, err := brokerClient.ProvisionInstance(request)
	if err != nil {
		// There are two buckets of errors to handle:
		// 1.  Errors that represent a failure response from the broker
		// 2.  All other errors
		if httpErr, ok := osb.IsHTTPError(err); ok {
			// An error from the broker represents a permanent failure and
			// should not be retried; set the Failed condition.
			s := fmt.Sprintf("Error provisioning ServiceInstance \"%s/%s\" of ServiceClass %q at ServiceBroker %q: %s", instance.Namespace, instance.Name, serviceClass.Spec.ExternalName, brokerName, httpErr)
			glog.Warning(s)
			c.recorder.Event(instance, apiv1.EventTypeWarning, errorProvisionCallFailedReason, s)

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

			if shouldStartOrphanMitigation(httpErr.StatusCode) {
				setServiceInstanceStartOrphanMitigation(toUpdate)

				if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
					return err
				}

				return httpErr
			}

			c.clearServiceInstanceCurrentOperation(toUpdate)

			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}

			return nil
		}

		s := fmt.Sprintf("Error provisioning ServiceInstance \"%s/%s\" of ServiceClass %q at ServiceBroker %q: %s", instance.Namespace, instance.Name, serviceClass.Spec.ExternalName, brokerName, err)
		glog.Warning(s)
		c.recorder.Event(instance, apiv1.EventTypeWarning, errorErrorCallingProvisionReason, s)

		urlErr, ok := err.(*url.Error)
		if ok && urlErr.Timeout() {
			// Communication to the broker timed out. Treat as terminal failure and
			// begin orphan mitigation.
			setServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionFalse,
				errorErrorCallingProvisionReason,
				"Communication with the ServiceBroker timed out; operation will not be retried: "+s)
			setServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionFailed,
				v1alpha1.ConditionTrue,
				errorErrorCallingProvisionReason,
				"Communication with the ServiceBroker timed out; operation will not be retried: "+s)
			setServiceInstanceStartOrphanMitigation(toUpdate)

			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}

			return err
		}

		setServiceInstanceCondition(
			toUpdate,
			v1alpha1.ServiceInstanceConditionReady,
			v1alpha1.ConditionFalse,
			errorErrorCallingProvisionReason,
			"Provision call failed and will be retried: "+s)

		if !time.Now().Before(toUpdate.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
			s := fmt.Sprintf(`Stopping reconciliation retries on ServiceInstance "%v/%v" because too much time has elapsed`, instance.Namespace, instance.Name)
			glog.Info(s)
			c.recorder.Event(instance, apiv1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)
			setServiceInstanceCondition(toUpdate,
				v1alpha1.ServiceInstanceConditionFailed,
				v1alpha1.ConditionTrue,
				errorReconciliationRetryTimeoutReason,
				s)
			c.clearServiceInstanceCurrentOperation(toUpdate)
			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}
			return nil
		}

		if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
			return err
		}

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
		glog.V(5).Infof("Received asynchronous provisioning response for ServiceInstance %v/%v of ServiceClass %v at ServiceBroker %v: response: %+v", instance.Namespace, instance.Name, serviceClass.Spec.ExternalName, brokerName, response)
		if response.OperationKey != nil && *response.OperationKey != "" {
			key := string(*response.OperationKey)
			toUpdate.Status.LastOperation = &key
		}

		// Tag this instance as having an ongoing async operation so we can enforce
		// no other operations against it can start.
		toUpdate.Status.AsyncOpInProgress = true

		setServiceInstanceCondition(
			toUpdate,
			v1alpha1.ServiceInstanceConditionReady,
			v1alpha1.ConditionFalse,
			asyncProvisioningReason,
			asyncProvisioningMessage,
		)
		if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
			return err
		}

		if err := c.beginPollingServiceInstance(instance); err != nil {
			return err
		}

		c.recorder.Eventf(instance, apiv1.EventTypeNormal, asyncProvisioningReason, asyncProvisioningMessage)
	} else {
		glog.V(5).Infof("Successfully provisioned ServiceInstance %v/%v of ServiceClass %v at ServiceBroker %v: response: %+v", instance.Namespace, instance.Name, serviceClass.Spec.ExternalName, brokerName, response)

		toUpdate.Status.ExternalProperties = toUpdate.Status.InProgressProperties
		c.clearServiceInstanceCurrentOperation(toUpdate)

		// TODO: process response
		setServiceInstanceCondition(
			toUpdate,
			v1alpha1.ServiceInstanceConditionReady,
			v1alpha1.ConditionTrue,
			successProvisionReason,
			successProvisionMessage,
		)
		if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
			return err
		}

		c.recorder.Eventf(instance, apiv1.EventTypeNormal, successProvisionReason, successProvisionMessage)
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
	// There are three possible operations that require polling:
	// 1) Normal asynchronous provision
	// 2) Normal asynchronous deprovision
	// 3) Deprovisioning as part of orphan mitigation
	//
	// There are some conditions that are different depending on which
	// operation we're polling for. This is more readable than checking the
	// status in various places.
	mitigatingOrphan := instance.Status.OrphanMitigationInProgress
	deleting := false
	if instance.Status.CurrentOperation == v1alpha1.ServiceInstanceOperationDeprovision || mitigatingOrphan {
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
		c.recorder.Event(instance, apiv1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)

		if !mitigatingOrphan {
			setServiceInstanceCondition(toUpdate,
				v1alpha1.ServiceInstanceConditionFailed,
				v1alpha1.ConditionTrue,
				errorReconciliationRetryTimeoutReason,
				s)
		}

		if deleting {
			c.clearServiceInstanceCurrentOperation(toUpdate)
		} else {
			setServiceInstanceStartOrphanMitigation(toUpdate)
		}

		if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
			return err
		}
		return c.finishPollingServiceInstance(instance)
	}

	request := &osb.LastOperationRequest{
		InstanceID: instance.Spec.ExternalID,
		ServiceID:  &serviceClass.Spec.ExternalID,
		PlanID:     &servicePlan.Spec.ExternalID,
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
			c.recorder.Event(instance, apiv1.EventTypeWarning, errorWithOriginatingIdentity, s)

			clone, cloneErr := api.Scheme.DeepCopy(instance)
			if cloneErr != nil {
				return cloneErr
			}
			toUpdate := clone.(*v1alpha1.ServiceInstance)
			setServiceInstanceCondition(toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionFalse,
				errorWithOriginatingIdentity,
				s)
			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}
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

			c.clearServiceInstanceCurrentOperation(toUpdate)
			toUpdate.Status.ExternalProperties = nil

			setServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionFalse,
				successDeprovisionReason,
				successDeprovisionMessage,
			)

			if !mitigatingOrphan {
				// Clear the finalizer
				if finalizers := sets.NewString(toUpdate.Finalizers...); finalizers.Has(v1alpha1.FinalizerServiceCatalog) {
					finalizers.Delete(v1alpha1.FinalizerServiceCatalog)
					toUpdate.Finalizers = finalizers.List()
				}
			}

			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}

			c.recorder.Event(instance, apiv1.EventTypeNormal, successDeprovisionReason, successDeprovisionMessage)
			glog.V(5).Infof("Successfully deprovisioned ServiceInstance %v/%v of ServiceClass %v at ServiceBroker %v", instance.Namespace, instance.Name, serviceClass.Spec.ExternalName, brokerName)

			return c.finishPollingServiceInstance(instance)
		}

		// We got some kind of error from the broker.  While polling last
		// operation, this represents an invalid response and we should
		// continue polling last operation.
		//
		// The ready condition on the instance should already have
		// condition false; it should be sufficient to create an event for
		// the instance.
		errText := ""
		if httpErr, ok := osb.IsHTTPError(err); ok {
			errText = fmt.Sprintf("Status code: %d; ErrorMessage: %q; description: %q", httpErr.StatusCode, httpErr.ErrorMessage, httpErr.Description)
		} else {
			errText = err.Error()
		}

		s := fmt.Sprintf("Error polling last operation for instance %v/%v: %v", instance.Namespace, instance.Name, errText)
		glog.V(4).Info(s)
		c.recorder.Event(instance, apiv1.EventTypeWarning, errorPollingLastOperationReason, s)

		if !time.Now().Before(instance.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
			clone, err := api.Scheme.DeepCopy(instance)
			if err != nil {
				return err
			}
			toUpdate := clone.(*v1alpha1.ServiceInstance)
			s := fmt.Sprintf(`Stopping reconciliation retries on ServiceInstance "%v/%v" because too much time has elapsed`, instance.Namespace, instance.Name)
			glog.Info(s)
			c.recorder.Event(instance, apiv1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)

			if !mitigatingOrphan {
				setServiceInstanceCondition(toUpdate,
					v1alpha1.ServiceInstanceConditionFailed,
					v1alpha1.ConditionTrue,
					errorReconciliationRetryTimeoutReason,
					s)
			}

			if deleting {
				c.clearServiceInstanceCurrentOperation(toUpdate)
			} else {
				setServiceInstanceStartOrphanMitigation(toUpdate)
			}

			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}
			return c.finishPollingServiceInstance(instance)
		}

		return c.continuePollingServiceInstance(instance)
	}

	glog.V(4).Infof("Poll for %v/%v returned %q : %q", instance.Namespace, instance.Name, response.State, response.Description)

	switch response.State {
	case osb.StateInProgress:
		var toUpdate *v1alpha1.ServiceInstance

		// if the description is non-nil, then update the instance condition with it
		if response.Description != nil {
			// The way the worker keeps on requeueing is by returning an error, so
			// we need to keep on polling.
			clone, err := api.Scheme.DeepCopy(instance)
			if err != nil {
				return err
			}
			toUpdate = clone.(*v1alpha1.ServiceInstance)
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
			setServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionFalse,
				reason,
				message,
			)
		}

		if !time.Now().Before(instance.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
			if toUpdate == nil {
				clone, err := api.Scheme.DeepCopy(instance)
				if err != nil {
					return err
				}
				toUpdate = clone.(*v1alpha1.ServiceInstance)
			}
			s := fmt.Sprintf(`Stopping reconciliation retries on ServiceInstance "%v/%v" because too much time has elapsed`, instance.Namespace, instance.Name)
			glog.Info(s)
			c.recorder.Event(instance, apiv1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)

			if !mitigatingOrphan {
				setServiceInstanceCondition(toUpdate,
					v1alpha1.ServiceInstanceConditionFailed,
					v1alpha1.ConditionTrue,
					errorReconciliationRetryTimeoutReason,
					s)
			}

			if deleting {
				c.clearServiceInstanceCurrentOperation(toUpdate)
			} else {
				setServiceInstanceStartOrphanMitigation(toUpdate)
			}

			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}

			return c.finishPollingServiceInstance(instance)
		}

		if toUpdate != nil {
			if _, err = c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}
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
		toUpdate.Status.ExternalProperties = toUpdate.Status.InProgressProperties
		c.clearServiceInstanceCurrentOperation(toUpdate)

		// If we were asynchronously deleting a Service Instance, finish
		// the finalizers.
		if deleting {
			setServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionFalse,
				successDeprovisionReason,
				successDeprovisionMessage,
			)

			if !mitigatingOrphan {
				// Clear the finalizer
				if finalizers := sets.NewString(toUpdate.Finalizers...); finalizers.Has(v1alpha1.FinalizerServiceCatalog) {
					finalizers.Delete(v1alpha1.FinalizerServiceCatalog)
					toUpdate.Finalizers = finalizers.List()
				}
			}

			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}

			c.recorder.Event(instance, apiv1.EventTypeNormal, successDeprovisionReason, successDeprovisionMessage)
			glog.V(5).Infof("Successfully deprovisioned ServiceInstance %v/%v of ServiceClass %v at ServiceBroker %v", instance.Namespace, instance.Name, serviceClass.Spec.ExternalName, brokerName)
		} else {
			setServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionTrue,
				successProvisionReason,
				successProvisionMessage,
			)
			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}
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
		s := fmt.Sprintf("Error deprovisioning ServiceInstance \"%s/%s\" of ServiceClass %q at ServiceBroker %q: %q", instance.Namespace, instance.Name, serviceClass.Spec.ExternalName, brokerName, description)
		c.recorder.Event(instance, apiv1.EventTypeWarning, errorDeprovisionCalledReason, s)

		clone, err := api.Scheme.DeepCopy(instance)
		if err != nil {
			return err
		}
		toUpdate := clone.(*v1alpha1.ServiceInstance)
		c.clearServiceInstanceCurrentOperation(toUpdate)

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

		if !mitigatingOrphan {
			setServiceInstanceCondition(
				toUpdate,
				v1alpha1.ServiceInstanceConditionFailed,
				v1alpha1.ConditionTrue,
				reason,
				msg,
			)
		}

		if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
			return err
		}

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
			c.recorder.Event(instance, apiv1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)

			if !mitigatingOrphan {
				setServiceInstanceCondition(toUpdate,
					v1alpha1.ServiceInstanceConditionFailed,
					v1alpha1.ConditionTrue,
					errorReconciliationRetryTimeoutReason,
					s)
			}

			if deleting {
				c.clearServiceInstanceCurrentOperation(toUpdate)
			} else {
				setServiceInstanceStartOrphanMitigation(toUpdate)
			}

			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}
			return c.finishPollingServiceInstance(instance)
		}
		return fmt.Errorf("Got invalid state in LastOperationResponse: %q", response.State)
	}
	return nil
}

// resolveReferences checks to see if ServiceClassRef and/or ServicePlanRef are
// nil and if so, will resolve the references and update the instance.
// If either can not be resolved, returns an error and sets the InstanceCondition
// with the appropriate error message.
func (c *controller) resolveReferences(instance *v1alpha1.ServiceInstance) (*v1alpha1.ServiceInstance, error) {
	if instance.Spec.ServiceClassRef != nil && instance.Spec.ServicePlanRef != nil {
		return instance, nil
	}

	var sc *v1alpha1.ServiceClass

	if instance.Spec.ServiceClassRef == nil {
		glog.V(4).Infof(`ServiceInstance "%s/%s": looking up a ServiceClass from externalName: %q`, instance.Namespace, instance.Name, instance.Spec.ExternalServiceClassName)
		listOpts := metav1.ListOptions{FieldSelector: "spec.externalName==" + instance.Spec.ExternalServiceClassName}
		serviceClasses, err := c.serviceCatalogClient.ServiceClasses().List(listOpts)
		if err == nil && len(serviceClasses.Items) == 1 {
			sc = &serviceClasses.Items[0]
			instance.Spec.ServiceClassRef = &apiv1.ObjectReference{
				Kind:            sc.Kind,
				Name:            sc.Name,
				UID:             sc.UID,
				APIVersion:      sc.APIVersion,
				ResourceVersion: sc.ResourceVersion,
			}
			glog.V(4).Infof(`ServiceInstance "%s/%s": resolved ServiceClass with externalName %q to K8S ServiceClass %q`, instance.Namespace, instance.Name, instance.Spec.ExternalServiceClassName, sc.Name)
		} else {
			s := fmt.Sprintf("ServiceInstance \"%s/%s\" references a non-existent ServiceClass %q", instance.Namespace, instance.Name, instance.Spec.ExternalServiceClassName)
			glog.Warning(s)
			c.updateServiceInstanceCondition(
				instance,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionFalse,
				errorNonexistentServiceClassReason,
				"The instance references a ServiceClass that does not exist. "+s,
			)
			c.recorder.Event(instance, apiv1.EventTypeWarning, errorNonexistentServiceClassReason, s)
			return nil, fmt.Errorf(s)
		}
	}

	if instance.Spec.ServicePlanRef == nil {
		if sc == nil {
			var scErr error
			sc, scErr = c.serviceClassLister.Get(instance.Spec.ServiceClassRef.Name)
			if scErr != nil {
				return nil, fmt.Errorf(`Couldn't find ServiceClass (K8S: %s) associated with Instance "%s/%s": %v`, instance.Spec.ServiceClassRef.Name, instance.Namespace, instance.Name, scErr.Error())
			}
		}

		fieldSet := fields.Set{
			"spec.externalName":         instance.Spec.ExternalServicePlanName,
			"spec.serviceClassRef.name": instance.Spec.ServiceClassRef.Name,
			"spec.serviceBrokerName":    sc.Spec.ServiceBrokerName,
		}
		fieldSelector := fields.SelectorFromSet(fieldSet).String()
		listOpts := metav1.ListOptions{FieldSelector: fieldSelector}
		servicePlans, err := c.serviceCatalogClient.ServicePlans().List(listOpts)
		if err == nil && len(servicePlans.Items) == 1 {
			sp := &servicePlans.Items[0]
			instance.Spec.ServicePlanRef = &apiv1.ObjectReference{
				Kind:            sp.Kind,
				Name:            sp.Name,
				UID:             sp.UID,
				APIVersion:      sp.APIVersion,
				ResourceVersion: sp.ResourceVersion,
			}
			glog.V(4).Infof(`ServiceInstance "%s/%s": resolved ServicePlan with externalName %q to K8S ServicePlan %q`, instance.Namespace, instance.Name, instance.Spec.ExternalServicePlanName, sp.Name)
		} else {
			s := fmt.Sprintf("ServiceInstance \"%s/%s\" references a non-existent ServicePlan %q on ServiceClass %q", instance.Namespace, instance.Name, instance.Spec.ExternalServicePlanName, instance.Spec.ExternalServiceClassName)
			glog.Warning(s)
			c.updateServiceInstanceCondition(
				instance,
				v1alpha1.ServiceInstanceConditionReady,
				v1alpha1.ConditionFalse,
				errorNonexistentServicePlanReason,
				"The instance references a ServicePlan that does not exist. "+s,
			)
			c.recorder.Event(instance, apiv1.EventTypeWarning, errorNonexistentServicePlanReason, s)
			return nil, fmt.Errorf(s)
		}
	}
	return c.updateServiceInstanceReferences(instance)
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

// updateServiceInstanceReferences updates the refs for the given instance.
func (c *controller) updateServiceInstanceReferences(toUpdate *v1alpha1.ServiceInstance) (*v1alpha1.ServiceInstance, error) {
	glog.V(4).Infof("Updating references for ServiceInstance %v/%v", toUpdate.Namespace, toUpdate.Name)
	updatedInstance, err := c.serviceCatalogClient.ServiceInstances(toUpdate.Namespace).UpdateReferences(toUpdate)
	if err != nil {
		glog.Errorf("Failed to update references for ServiceInstance %v/%v: %v", toUpdate.Namespace, toUpdate.Name, err)
	}
	return updatedInstance, err
}

// updateServiceInstanceStatus updates the status for the given instance.
//
// Note: objects coming from informers should never be mutated; the instance
// passed to this method should always be a deep copy.
func (c *controller) updateServiceInstanceStatus(toUpdate *v1alpha1.ServiceInstance) (*v1alpha1.ServiceInstance, error) {
	glog.V(4).Infof("Updating status for ServiceInstance %v/%v", toUpdate.Namespace, toUpdate.Name)
	updatedInstance, err := c.serviceCatalogClient.ServiceInstances(toUpdate.Namespace).UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf("Failed to update status for ServiceInstance %v/%v: %v", toUpdate.Namespace, toUpdate.Name, err)
	}

	return updatedInstance, err
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

// recordStartOfServiceInstanceOperation updates the instance to indicate that
// there is a current operation being performed. The Status of the instance
// is recorded in the registry.
// params:
// toUpdate - a modifiable copy of the instance in the registry to update
// operation - operation that is being performed on the instance
// returns:
// 1 - a modifiable copy of the updated instance in the registry; or toUpdate
//     if there was an error
// 2 - any error that occurred
func (c *controller) recordStartOfServiceInstanceOperation(toUpdate *v1alpha1.ServiceInstance, operation v1alpha1.ServiceInstanceOperation) (*v1alpha1.ServiceInstance, error) {
	toUpdate.Status.CurrentOperation = operation
	now := metav1.Now()
	toUpdate.Status.OperationStartTime = &now
	reason := ""
	message := ""
	switch operation {
	case v1alpha1.ServiceInstanceOperationProvision:
		reason = provisioningInFlightReason
		message = provisioningInFlightMessage
	case v1alpha1.ServiceInstanceOperationDeprovision:
		reason = deprovisioningInFlightReason
		message = deprovisioningInFlightMessage
	}
	setServiceInstanceCondition(
		toUpdate,
		v1alpha1.ServiceInstanceConditionReady,
		v1alpha1.ConditionFalse,
		reason,
		message,
	)
	return c.updateServiceInstanceStatus(toUpdate)
}

// clearServiceInstanceCurrentOperation sets the fields of the instance's Status
// to indicate that there is no current operation being performed. The Status
// is *not* recorded in the registry.
func (c *controller) clearServiceInstanceCurrentOperation(toUpdate *v1alpha1.ServiceInstance) {
	toUpdate.Status.CurrentOperation = ""
	toUpdate.Status.OperationStartTime = nil
	toUpdate.Status.AsyncOpInProgress = false
	toUpdate.Status.OrphanMitigationInProgress = false
	toUpdate.Status.LastOperation = nil
	toUpdate.Status.InProgressProperties = nil
	toUpdate.Status.ReconciledGeneration = toUpdate.Generation
}

// setServiceInstanceStartOrphanMitigation sets the fields of the instance's
// Status to indicate that orphan mitigation is starting. The Status is *not*
// recorded in the registry.
func setServiceInstanceStartOrphanMitigation(toUpdate *v1alpha1.ServiceInstance) {
	toUpdate.Status.OperationStartTime = nil
	toUpdate.Status.AsyncOpInProgress = false
	toUpdate.Status.OrphanMitigationInProgress = true
	toUpdate.Status.InProgressProperties = nil
}

// shouldStartOrphanMitigation returns whether an error with the given status
// code indicates that orphan migitation should start.
func shouldStartOrphanMitigation(statusCode int) bool {
	is2XX := (statusCode >= 200 && statusCode < 300)
	is5XX := (statusCode >= 500 && statusCode < 600)

	return (is2XX && statusCode != http.StatusOK) ||
		statusCode == http.StatusRequestTimeout ||
		is5XX
}

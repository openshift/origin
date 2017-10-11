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
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	scfeatures "github.com/kubernetes-incubator/service-catalog/pkg/features"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
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

	glog.V(4).Infof(`ServiceInstance "%s/%s": Received delete event; no further processing will occur`, instance.Namespace, instance.Name)
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
func (c *controller) beginPollingServiceInstance(instance *v1beta1.ServiceInstance) error {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(instance)
	if err != nil {
		glog.Errorf(`ServiceInstance "%s/%s": Couldn't create a key for object %+v: %v`, instance.Namespace, instance.Name, instance, err)
		return fmt.Errorf("Couldn't create a key for object %+v: %v", instance, err)
	}

	c.pollingQueue.AddRateLimited(key)

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
		glog.Errorf(`ServiceInstance "%s/%s": Couldn't create a key for object %+v: %v`, instance.Namespace, instance.Name, instance, err)
		return fmt.Errorf(`ServiceInstance "%s/%s": Couldn't create a key for object %+v: %v`, instance.Namespace, instance.Name, instance, err)
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
		glog.Infof(`ServiceInstance "%s/%s": Not doing work for %v because it has been deleted`, namespace, name, key)
		return nil
	}
	if err != nil {
		glog.Errorf(`ServiceInstance "%s/%s": Unable to retrieve %v from store: %v`, instance.Namespace, instance.Name, key, err)
		return err
	}

	return c.reconcileServiceInstance(instance)
}

// reconcileServiceInstanceDelete is responsible for handling any instance whose
// deletion timestamp is set.
func (c *controller) reconcileServiceInstanceDelete(instance *v1beta1.ServiceInstance) error {
	// nothing to do...
	if instance.DeletionTimestamp == nil && !instance.Status.OrphanMitigationInProgress {
		return nil
	}

	// Determine if any credentials exist for this instance.  We don't want to
	// delete the instance if there are any associated creds
	credentialsLister := c.bindingLister.ServiceBindings(instance.Namespace)

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
			toUpdate := clone.(*v1beta1.ServiceInstance)

			s := fmt.Sprintf(
				`ServiceInstance "%s/%s": Delete instance blocked by existing ServiceBindings associated with this instance.  All credentials must be removed first`,
				instance.Namespace, instance.Name)
			glog.Warning(s)
			c.recorder.Event(instance, corev1.EventTypeWarning, errorDeprovisionBlockedByCredentialsReason, s)

			setServiceInstanceCondition(
				toUpdate,
				v1beta1.ServiceInstanceConditionReady,
				v1beta1.ConditionFalse,
				errorDeprovisionBlockedByCredentialsReason,
				"Delete instance failed. "+s)
			if _, err = c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}
			return nil
		}
	}

	finalizerToken := v1beta1.FinalizerServiceCatalog
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

		glog.V(5).Infof(`ServiceInstance "%s/%s": Clearing catalog finalizer`, instance.Namespace, instance.Name)
		clone, err := api.Scheme.DeepCopy(instance)
		if err != nil {
			return err
		}
		toUpdate := clone.(*v1beta1.ServiceInstance)
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
	serviceClass, servicePlan, brokerName, brokerClient, err := c.getClusterServiceClassPlanAndClusterServiceBroker(instance)
	if err != nil {
		return err
	}

	// we will definitely update the instance's status - make a deep copy now
	// for use later in this method.
	clone, err := api.Scheme.DeepCopy(instance)
	if err != nil {
		return err
	}
	toUpdate := clone.(*v1beta1.ServiceInstance)

	request := &osb.DeprovisionRequest{
		InstanceID:        instance.Spec.ExternalID,
		ServiceID:         serviceClass.Spec.ExternalID,
		PlanID:            servicePlan.Spec.ExternalID,
		AcceptsIncomplete: true,
	}

	if utilfeature.DefaultFeatureGate.Enabled(scfeatures.OriginatingIdentity) {
		originatingIdentity, err := buildOriginatingIdentity(instance.Spec.UserInfo)
		if err != nil {
			s := fmt.Sprintf(`ServiceInstance "%s/%s": Error building originating identity headers when deprovisioning: %v`, instance.Namespace, instance.Name, err)
			glog.Warning(s)
			c.recorder.Event(instance, corev1.EventTypeWarning, errorWithOriginatingIdentity, s)

			setServiceInstanceCondition(
				toUpdate,
				v1beta1.ServiceInstanceConditionReady,
				v1beta1.ConditionFalse,
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
		toUpdate, err = c.recordStartOfServiceInstanceOperation(toUpdate, v1beta1.ServiceInstanceOperationDeprovision)
		if err != nil {
			// There has been an update to the instance. Start reconciliation
			// over with a fresh view of the instance.
			return err
		}
	}

	glog.V(4).Infof(`ServiceInstance "%s/%s": Deprovisioning`, instance.Namespace, instance.Name)
	response, err := brokerClient.DeprovisionInstance(request)
	if err != nil {
		if httpErr, ok := osb.IsHTTPError(err); ok {
			s := fmt.Sprintf(
				`ServiceInstance "%s/%s": Deprovision call failed; received error response from broker: Status Code: %d, Error Message: %v, Description: %v`,
				instance.Namespace, instance.Name,
				httpErr.StatusCode,
				httpErr.ErrorMessage,
				httpErr.Description,
			)
			glog.Warning(s)
			c.recorder.Event(instance, corev1.EventTypeWarning, errorDeprovisionCalledReason, s)

			setServiceInstanceCondition(
				toUpdate,
				v1beta1.ServiceInstanceConditionReady,
				v1beta1.ConditionUnknown,
				errorDeprovisionCalledReason,
				"Deprovision call failed. "+s)

			if !instance.Status.OrphanMitigationInProgress {
				// Do not overwrite 'Failed' message if deprovisioning due to orphan
				// mitigation in order to prevent loss of original reason for the
				// orphan.
				setServiceInstanceCondition(
					toUpdate,
					v1beta1.ServiceInstanceConditionFailed,
					v1beta1.ConditionTrue,
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
			`ServiceInstance "%s/%s": Error deprovisioning, ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q: %v`,
			instance.Namespace, instance.Name,
			serviceClass.Name, serviceClass.Spec.ExternalName,
			brokerName,
			err,
		)
		glog.Warning(s)
		c.recorder.Event(instance, corev1.EventTypeWarning, errorDeprovisionCalledReason, s)

		setServiceInstanceCondition(
			toUpdate,
			v1beta1.ServiceInstanceConditionReady,
			v1beta1.ConditionUnknown,
			errorDeprovisionCalledReason,
			"Deprovision call failed. "+s)

		if !time.Now().Before(toUpdate.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
			s := fmt.Sprintf(`ServiceInstance "%s/%s": Stopping reconciliation retries because too much time has elapsed`, instance.Namespace, instance.Name)
			glog.Info(s)
			c.recorder.Event(instance, corev1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)

			if !instance.Status.OrphanMitigationInProgress {
				setServiceInstanceCondition(toUpdate,
					v1beta1.ServiceInstanceConditionFailed,
					v1beta1.ConditionTrue,
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
		glog.V(5).Infof(`ServiceInstance "%s/%s": Received asynchronous de-provisioning response, %s at %s: response: %+v`, instance.Namespace, instance.Name, serviceClass.Name, brokerName, response)

		if response.OperationKey != nil && *response.OperationKey != "" {
			key := string(*response.OperationKey)
			toUpdate.Status.LastOperation = &key
		}

		// Tag this instance as having an ongoing async operation so we can enforce
		// no other operations against it can start.
		toUpdate.Status.AsyncOpInProgress = true

		setServiceInstanceCondition(
			toUpdate,
			v1beta1.ServiceInstanceConditionReady,
			v1beta1.ConditionFalse,
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

		c.recorder.Eventf(instance, corev1.EventTypeNormal, asyncDeprovisioningReason, asyncDeprovisioningMessage)

		return nil
	}

	glog.V(5).Infof(`ServiceInstance "%s/%s": Deprovision call to broker succeeded, finalizing`, instance.Namespace, instance.Name)

	c.clearServiceInstanceCurrentOperation(toUpdate)
	toUpdate.Status.ExternalProperties = nil

	setServiceInstanceCondition(
		toUpdate,
		v1beta1.ServiceInstanceConditionReady,
		v1beta1.ConditionFalse,
		successDeprovisionReason,
		successDeprovisionMessage,
	)

	if instance.DeletionTimestamp != nil {
		// Clear the finalizer for normal instance deletions
		finalizers.Delete(v1beta1.FinalizerServiceCatalog)
		toUpdate.Finalizers = finalizers.List()
	}

	if _, err = c.updateServiceInstanceStatus(toUpdate); err != nil {
		return err
	}

	c.recorder.Event(instance, corev1.EventTypeNormal, successDeprovisionReason, successDeprovisionMessage)
	glog.V(5).Infof(`ServiceInstance "%s/%s": Successfully deprovisioned, %s at %s`, instance.Namespace, instance.Name, serviceClass.Name, brokerName)

	return nil
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

// reconcileServiceInstance is the control-loop for reconciling Instances. An
// error is returned to indicate that the instance has not been fully
// processed and should be resubmitted at a later time.
func (c *controller) reconcileServiceInstance(instance *v1beta1.ServiceInstance) error {
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
			`ServiceInstance "%s/%s": Not processing event because status showed that it has failed`,
			instance.Namespace, instance.Name,
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
			`ServiceInstance "%s/%s": Not processing event because reconciled generation showed there is no work to do`,
			instance.Namespace, instance.Name,
		)
		return nil
	}

	// we will definitely update the instance's status - make a deep copy now
	// for use later in this method.
	clone, err := api.Scheme.DeepCopy(instance)
	if err != nil {
		return err
	}
	toUpdate := clone.(*v1beta1.ServiceInstance)

	// Update references to ClusterServicePlan / ClusterServiceClass if necessary.
	toUpdate, err = c.resolveReferences(toUpdate)
	if err != nil {
		return err
	}

	glog.V(4).Infof(`ServiceInstance "%s/%s": Processing adding/updating event`, instance.Namespace, instance.Name)

	serviceClass, servicePlan, brokerName, brokerClient, err := c.getClusterServiceClassPlanAndClusterServiceBroker(toUpdate)
	if err != nil {
		return err
	}

	ns, err := c.kubeClient.Core().Namespaces().Get(instance.Namespace, metav1.GetOptions{})
	if err != nil {
		s := fmt.Sprintf(`ServiceInstance "%s/%s": Failed to get namespace %q during instance create: %s`, instance.Namespace, instance.Name, instance.Namespace, err)
		glog.Info(s)
		c.recorder.Event(instance, corev1.EventTypeWarning, errorFindingNamespaceServiceInstanceReason, s)

		setServiceInstanceCondition(
			toUpdate,
			v1beta1.ServiceInstanceConditionReady,
			v1beta1.ConditionFalse,
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
			s := fmt.Sprintf(`ServiceInstance "%s/%s": Failed to prepare ServiceInstance parameters %s: %s`, instance.Namespace, instance.Name, instance.Spec.Parameters, err)
			glog.Warning(s)
			c.recorder.Event(instance, corev1.EventTypeWarning, errorWithParameters, s)

			setServiceInstanceCondition(
				toUpdate,
				v1beta1.ServiceInstanceConditionReady,
				v1beta1.ConditionFalse,
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
			s := fmt.Sprintf(`ServiceInstance "%s/%s": Failed to generate the parameters checksum to store in Status: %s`, instance.Namespace, instance.Name, err)
			glog.Info(s)
			c.recorder.Eventf(instance, corev1.EventTypeWarning, errorWithParameters, s)
			setServiceInstanceCondition(
				toUpdate,
				v1beta1.ServiceInstanceConditionReady,
				v1beta1.ConditionFalse,
				errorWithParameters,
				s)
			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}
			return err
		}

		marshalledParametersWithRedaction, err := MarshalRawParameters(parametersWithSecretsRedacted)
		if err != nil {
			s := fmt.Sprintf(`ServiceInstance "%s/%s": Failed to marshal the parameters to store in the Status: %s`, instance.Namespace, instance.Name, err)
			glog.Info(s)
			c.recorder.Eventf(instance, corev1.EventTypeWarning, errorWithParameters, s)
			setServiceInstanceCondition(
				toUpdate,
				v1beta1.ServiceInstanceConditionReady,
				v1beta1.ConditionFalse,
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

	toUpdate.Status.InProgressProperties = &v1beta1.ServiceInstancePropertiesState{
		ExternalClusterServicePlanName: servicePlan.Spec.ExternalName,
		Parameters:                     rawParametersWithRedaction,
		ParametersChecksum:             parametersChecksum,
		UserInfo:                       instance.Spec.UserInfo,
	}

	var originatingIdentity *osb.AlphaOriginatingIdentity
	if utilfeature.DefaultFeatureGate.Enabled(scfeatures.OriginatingIdentity) {
		originatingIdentity, err = buildOriginatingIdentity(instance.Spec.UserInfo)
		if err != nil {
			s := fmt.Sprintf(`ServiceInstance "%s/%s": Error building originating identity headers for provisioning: %v`, instance.Namespace, instance.Name, err)
			glog.Warning(s)
			c.recorder.Event(instance, corev1.EventTypeWarning, errorWithOriginatingIdentity, s)

			setServiceInstanceCondition(
				toUpdate,
				v1beta1.ServiceInstanceConditionReady,
				v1beta1.ConditionFalse,
				errorWithOriginatingIdentity,
				s,
			)
			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}

			return err
		}
	}

	var (
		isProvisioning             bool
		provisionRequest           *osb.ProvisionRequest
		updateRequest              *osb.UpdateInstanceRequest
		currentOperation           v1beta1.ServiceInstanceOperation
		provisionOrUpdateText      string
		provisionedOrUpdatedText   string
		provisioningOrUpdatingText string
	)
	if toUpdate.Status.ReconciledGeneration == 0 {
		isProvisioning = true
		// osb client handles whether or not to really send this based
		// on the version of the client.
		requestContext := map[string]interface{}{
			"platform":  ContextProfilePlatformKubernetes,
			"namespace": instance.Namespace,
		}
		provisionRequest = &osb.ProvisionRequest{
			AcceptsIncomplete:   true,
			InstanceID:          instance.Spec.ExternalID,
			ServiceID:           serviceClass.Spec.ExternalID,
			PlanID:              servicePlan.Spec.ExternalID,
			Parameters:          parameters,
			OrganizationGUID:    string(ns.UID),
			SpaceGUID:           string(ns.UID),
			Context:             requestContext,
			OriginatingIdentity: originatingIdentity,
		}
		currentOperation = v1beta1.ServiceInstanceOperationProvision
		provisionOrUpdateText = "provision"
		provisionedOrUpdatedText = "provisioned"
		provisioningOrUpdatingText = "provisioning"
	} else {
		isProvisioning = false
		updateRequest = &osb.UpdateInstanceRequest{
			AcceptsIncomplete:   true,
			InstanceID:          instance.Spec.ExternalID,
			ServiceID:           serviceClass.Spec.ExternalID,
			OriginatingIdentity: originatingIdentity,
		}
		// Only send the plan ID if the plan name has changed from what the Broker has
		if toUpdate.Status.ExternalProperties == nil ||
			toUpdate.Status.InProgressProperties.ExternalClusterServicePlanName != toUpdate.Status.ExternalProperties.ExternalClusterServicePlanName {
			planID := servicePlan.Spec.ExternalID
			updateRequest.PlanID = &planID
		}
		// Only send the parameters if they have changed from what the Broker has
		if toUpdate.Status.ExternalProperties == nil ||
			toUpdate.Status.InProgressProperties.ParametersChecksum != toUpdate.Status.ExternalProperties.ParametersChecksum {
			updateRequest.Parameters = parameters
		}
		currentOperation = v1beta1.ServiceInstanceOperationUpdate
		provisionOrUpdateText = "update"
		provisionedOrUpdatedText = "updated"
		provisioningOrUpdatingText = "updating"
	}

	if toUpdate.Status.CurrentOperation == "" {
		toUpdate, err = c.recordStartOfServiceInstanceOperation(toUpdate, currentOperation)
		if err != nil {
			// There has been an update to the instance. Start reconciliation
			// over with a fresh view of the instance.
			return err
		}
	}

	var provisionResponse *osb.ProvisionResponse
	var updateResponse *osb.UpdateInstanceResponse
	if isProvisioning {
		glog.V(4).Infof(`ServiceInstance "%s/%s": Provisioning a new ServiceInstance of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q`, instance.Namespace, instance.Name, serviceClass.Name, serviceClass.Spec.ExternalName, brokerName)
		provisionResponse, err = brokerClient.ProvisionInstance(provisionRequest)
	} else {
		glog.V(4).Infof(`ServiceInstance "%s/%s": Updating ServiceInstance of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q`, instance.Namespace, instance.Name, serviceClass.Name, serviceClass.Spec.ExternalName, brokerName)
		updateResponse, err = brokerClient.UpdateInstance(updateRequest)
	}
	if err != nil {
		// There are two buckets of errors to handle:
		// 1.  Errors that represent a failure response from the broker
		// 2.  All other errors
		if httpErr, ok := osb.IsHTTPError(err); ok {
			reason := errorProvisionCallFailedReason
			if !isProvisioning {
				reason = errorUpdateInstanceCallFailedReason
			}
			// An error from the broker represents a permanent failure and
			// should not be retried; set the Failed condition.
			s := fmt.Sprintf(`ServiceInstance "%s/%s": Error %v ServiceInstance of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q: %s`, instance.Namespace, instance.Name, provisioningOrUpdatingText, serviceClass.Name, serviceClass.Spec.ExternalName, brokerName, httpErr)
			glog.Warning(s)
			c.recorder.Event(instance, corev1.EventTypeWarning, reason, s)

			setServiceInstanceCondition(
				toUpdate,
				v1beta1.ServiceInstanceConditionFailed,
				v1beta1.ConditionTrue,
				"ClusterServiceBrokerReturnedFailure",
				s)
			setServiceInstanceCondition(
				toUpdate,
				v1beta1.ServiceInstanceConditionReady,
				v1beta1.ConditionFalse,
				reason,
				fmt.Sprintf(`ServiceInstance "%s/%s": ClusterServiceBroker returned a failure for %v call; operation will not be retried: %v`, instance.Namespace, instance.Name, provisionOrUpdateText, s))

			if isProvisioning && shouldStartOrphanMitigation(httpErr.StatusCode) {
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

		reason := errorErrorCallingProvisionReason
		if !isProvisioning {
			reason = errorErrorCallingUpdateInstanceReason
		}
		s := fmt.Sprintf(`ServiceInstance "%s/%s": Error communicating with broker for %q: %s`, instance.Namespace, instance.Name, provisioningOrUpdatingText, err)
		glog.Warning(s)
		c.recorder.Event(instance, corev1.EventTypeWarning, reason, s)

		urlErr, ok := err.(*url.Error)
		if ok && urlErr.Timeout() {
			var (
				reason  string
				message string
			)
			if isProvisioning {
				reason = errorErrorCallingProvisionReason
			} else {
				reason = errorErrorCallingUpdateInstanceReason
			}
			message = "Communication with the ClusterServiceBroker timed out; operation will not be retried: " + s
			// Communication to the broker timed out. Treat as terminal failure and
			// begin orphan mitigation.
			setServiceInstanceCondition(
				toUpdate,
				v1beta1.ServiceInstanceConditionReady,
				v1beta1.ConditionFalse,
				reason,
				message)
			setServiceInstanceCondition(
				toUpdate,
				v1beta1.ServiceInstanceConditionFailed,
				v1beta1.ConditionTrue,
				reason,
				message)

			if isProvisioning {
				setServiceInstanceStartOrphanMitigation(toUpdate)
			} else {
				c.clearServiceInstanceCurrentOperation(toUpdate)
			}

			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}

			return err
		}

		setServiceInstanceCondition(
			toUpdate,
			v1beta1.ServiceInstanceConditionReady,
			v1beta1.ConditionFalse,
			reason,
			fmt.Sprintf(`ServiceInstance "%s/%s": The %v call failed and will be retried: %v`, instance.Namespace, instance.Name, provisionOrUpdateText, s))

		if !time.Now().Before(toUpdate.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
			s := fmt.Sprintf(`ServiceInstance "%s/%s": Stopping reconciliation retries because too much time has elapsed`, instance.Namespace, instance.Name)
			glog.Info(s)
			c.recorder.Event(instance, corev1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)
			setServiceInstanceCondition(toUpdate,
				v1beta1.ServiceInstanceConditionFailed,
				v1beta1.ConditionTrue,
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

	if isProvisioning && provisionResponse.DashboardURL != nil && *provisionResponse.DashboardURL != "" {
		url := *provisionResponse.DashboardURL
		toUpdate.Status.DashboardURL = &url
	}

	// ClusterServiceBroker can return either a synchronous or asynchronous
	// response, if the response is StatusAccepted it's an async
	// and we need to add it to the polling queue. ClusterServiceBroker can
	// optionally return 'Operation' that will then need to be
	// passed back to the broker during polling of last_operation.
	var response interface{}
	async := false
	if isProvisioning {
		response = provisionResponse
		async = provisionResponse.Async
	} else {
		response = updateResponse
		async = updateResponse.Async
	}
	if async {
		glog.V(5).Infof(`ServiceInstance "%s/%s": Received asynchronous %v response for ServiceInstance of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q: response: %+v`, instance.Namespace, instance.Name, provisioningOrUpdatingText, serviceClass.Name, serviceClass.Spec.ExternalName, brokerName, response)

		var operationKey *osb.OperationKey
		if isProvisioning {
			operationKey = provisionResponse.OperationKey
		} else {
			operationKey = updateResponse.OperationKey
		}
		if operationKey != nil && *operationKey != "" {
			key := string(*operationKey)
			toUpdate.Status.LastOperation = &key
		}

		// Tag this instance as having an ongoing async operation so we can enforce
		// no other operations against it can start.
		toUpdate.Status.AsyncOpInProgress = true

		reason := asyncProvisioningReason
		message := asyncProvisioningMessage
		if !isProvisioning {
			reason = asyncUpdatingInstanceReason
			message = asyncUpdatingInstanceMessage
		}
		setServiceInstanceCondition(
			toUpdate,
			v1beta1.ServiceInstanceConditionReady,
			v1beta1.ConditionFalse,
			reason,
			message,
		)
		if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
			return err
		}

		if err := c.beginPollingServiceInstance(instance); err != nil {
			return err
		}

		c.recorder.Eventf(instance, corev1.EventTypeNormal, reason, message)
	} else {
		reason := successProvisionReason
		message := successProvisionMessage
		if !isProvisioning {
			reason = successUpdateInstanceReason
			message = successUpdateInstanceMessage
		}
		glog.V(5).Infof(`ServiceInstance "%s/%s": Successfully %v ServiceInstance of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q: response: %+v`, provisionedOrUpdatedText, instance.Namespace, instance.Name, serviceClass.Name, serviceClass.Spec.ExternalName, brokerName, response)

		toUpdate.Status.ExternalProperties = toUpdate.Status.InProgressProperties
		c.clearServiceInstanceCurrentOperation(toUpdate)

		// TODO: process response
		setServiceInstanceCondition(
			toUpdate,
			v1beta1.ServiceInstanceConditionReady,
			v1beta1.ConditionTrue,
			reason,
			message,
		)
		if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
			return err
		}

		c.recorder.Eventf(instance, corev1.EventTypeNormal, reason, message)
	}
	return nil
}

func (c *controller) pollServiceInstanceInternal(instance *v1beta1.ServiceInstance) error {
	glog.V(4).Infof(`ServiceInstance "%s/%s": Processing`, instance.Namespace, instance.Name)

	serviceClass, servicePlan, brokerName, brokerClient, err := c.getClusterServiceClassPlanAndClusterServiceBroker(instance)
	if err != nil {
		return err
	}
	return c.pollServiceInstance(serviceClass, servicePlan, brokerName, brokerClient, instance)
}

func (c *controller) pollServiceInstance(serviceClass *v1beta1.ClusterServiceClass, servicePlan *v1beta1.ClusterServicePlan, brokerName string, brokerClient osb.Client, instance *v1beta1.ServiceInstance) error {
	// There are three possible operations that require polling:
	// 1) Normal asynchronous provision
	// 2) Normal asynchronous deprovision
	// 3) Deprovisioning as part of orphan mitigation
	//
	// There are some conditions that are different depending on which
	// operation we're polling for. This is more readable than checking the
	// status in various places.
	mitigatingOrphan := instance.Status.OrphanMitigationInProgress
	provisioning := instance.Status.CurrentOperation == v1beta1.ServiceInstanceOperationProvision && !mitigatingOrphan
	deleting := false
	if instance.Status.CurrentOperation == v1beta1.ServiceInstanceOperationDeprovision || mitigatingOrphan {
		deleting = true
	}

	// OperationStartTime must be set because we are polling an in-progress
	// operation. If it is not set, this is a logical error. Let's bail out.
	if instance.Status.OperationStartTime == nil {
		clone, err := api.Scheme.DeepCopy(instance)
		if err != nil {
			return err
		}
		toUpdate := clone.(*v1beta1.ServiceInstance)
		s := fmt.Sprintf(`ServiceInstance "%s/%s": Stopping reconciliation retries because the operation start time is not set`, instance.Namespace, instance.Name)
		glog.Info(s)
		c.recorder.Event(instance, corev1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)

		if !mitigatingOrphan {
			setServiceInstanceCondition(toUpdate,
				v1beta1.ServiceInstanceConditionFailed,
				v1beta1.ConditionTrue,
				errorReconciliationRetryTimeoutReason,
				s)
		}

		if !provisioning {
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
			s := fmt.Sprintf(`ServiceInstance "%s/%s": Error building originating identity headers for polling last operation: %v`, instance.Namespace, instance.Name, err)
			glog.Warning(s)
			c.recorder.Event(instance, corev1.EventTypeWarning, errorWithOriginatingIdentity, s)

			clone, cloneErr := api.Scheme.DeepCopy(instance)
			if cloneErr != nil {
				return cloneErr
			}
			toUpdate := clone.(*v1beta1.ServiceInstance)
			setServiceInstanceCondition(toUpdate,
				v1beta1.ServiceInstanceConditionReady,
				v1beta1.ConditionFalse,
				errorWithOriginatingIdentity,
				s)
			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}
			return err
		}
		request.OriginatingIdentity = originatingIdentity
	}

	glog.V(5).Infof(`ServiceInstance "%s/%s": Polling last operation`, instance.Namespace, instance.Name)

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
			toUpdate := clone.(*v1beta1.ServiceInstance)

			c.clearServiceInstanceCurrentOperation(toUpdate)
			toUpdate.Status.ExternalProperties = nil

			setServiceInstanceCondition(
				toUpdate,
				v1beta1.ServiceInstanceConditionReady,
				v1beta1.ConditionFalse,
				successDeprovisionReason,
				successDeprovisionMessage,
			)

			if !mitigatingOrphan {
				// Clear the finalizer
				if finalizers := sets.NewString(toUpdate.Finalizers...); finalizers.Has(v1beta1.FinalizerServiceCatalog) {
					finalizers.Delete(v1beta1.FinalizerServiceCatalog)
					toUpdate.Finalizers = finalizers.List()
				}
			}

			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}

			c.recorder.Event(instance, corev1.EventTypeNormal, successDeprovisionReason, successDeprovisionMessage)
			glog.V(5).Infof(`ServiceInstance "%s/%s": Successfully deprovisioned ServiceInstance of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q`, instance.Namespace, instance.Name, serviceClass.Name, serviceClass.Spec.ExternalName, brokerName)

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
			errText = fmt.Sprintf(`ServiceInstance "%s/%s": Status code: %d; ErrorMessage: %q; description: %q`, instance.Namespace, instance.Name, httpErr.StatusCode, httpErr.ErrorMessage, httpErr.Description)
		} else {
			errText = err.Error()
		}

		s := fmt.Sprintf(`ServiceInstance "%s/%s": Error polling last operation: %v`, instance.Namespace, instance.Name, errText)
		glog.V(4).Info(s)
		c.recorder.Event(instance, corev1.EventTypeWarning, errorPollingLastOperationReason, s)

		if !time.Now().Before(instance.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
			clone, err := api.Scheme.DeepCopy(instance)
			if err != nil {
				return err
			}
			toUpdate := clone.(*v1beta1.ServiceInstance)
			s := fmt.Sprintf(`ServiceInstance "%s/%s": Stopping reconciliation retries because too much time has elapsed`, instance.Namespace, instance.Name)
			glog.Info(s)
			c.recorder.Event(instance, corev1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)

			if !mitigatingOrphan {
				setServiceInstanceCondition(toUpdate,
					v1beta1.ServiceInstanceConditionFailed,
					v1beta1.ConditionTrue,
					errorReconciliationRetryTimeoutReason,
					s)
			}

			if !provisioning {
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

	glog.V(4).Infof(`ServiceInstance "%s/%s": Poll returned %q : %q`, instance.Namespace, instance.Name, response.State, response.Description)

	switch response.State {
	case osb.StateInProgress:
		var toUpdate *v1beta1.ServiceInstance

		// if the description is non-nil, then update the instance condition with it
		if response.Description != nil {
			// The way the worker keeps on requeueing is by returning an error, so
			// we need to keep on polling.
			clone, err := api.Scheme.DeepCopy(instance)
			if err != nil {
				return err
			}
			toUpdate = clone.(*v1beta1.ServiceInstance)
			toUpdate.Status.AsyncOpInProgress = true

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
			setServiceInstanceCondition(
				toUpdate,
				v1beta1.ServiceInstanceConditionReady,
				v1beta1.ConditionFalse,
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
				toUpdate = clone.(*v1beta1.ServiceInstance)
			}
			s := fmt.Sprintf(`ServiceInstance "%s/%s": Stopping reconciliation retries because too much time has elapsed`, instance.Namespace, instance.Name)
			glog.Info(s)
			c.recorder.Event(instance, corev1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)

			if !mitigatingOrphan {
				setServiceInstanceCondition(toUpdate,
					v1beta1.ServiceInstanceConditionFailed,
					v1beta1.ConditionTrue,
					errorReconciliationRetryTimeoutReason,
					s)
			}

			if !provisioning {
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
		glog.V(4).Infof(`ServiceInstance "%s/%s": last operation not completed (still in progress)`, instance.Namespace, instance.Name)
	case osb.StateSucceeded:
		var (
			readyStatus v1beta1.ConditionStatus
			message     string
			reason      string
			actionText  string
		)
		switch {
		case deleting:
			readyStatus = v1beta1.ConditionFalse
			reason = successDeprovisionReason
			message = successDeprovisionMessage
			actionText = "deprovisioned"
		case provisioning:
			readyStatus = v1beta1.ConditionTrue
			reason = successProvisionReason
			message = successProvisionMessage
			actionText = "provisioned"
		default:
			readyStatus = v1beta1.ConditionTrue
			reason = successUpdateInstanceReason
			message = successUpdateInstanceMessage
			actionText = "updated"
		}

		// Update the instance to reflect that an async operation is no longer
		// in progress.
		clone, err := api.Scheme.DeepCopy(instance)
		if err != nil {
			return err
		}
		toUpdate := clone.(*v1beta1.ServiceInstance)

		toUpdate.Status.ExternalProperties = toUpdate.Status.InProgressProperties
		c.clearServiceInstanceCurrentOperation(toUpdate)

		setServiceInstanceCondition(
			toUpdate,
			v1beta1.ServiceInstanceConditionReady,
			readyStatus,
			reason,
			message,
		)

		if deleting && !mitigatingOrphan {
			// Clear the finalizer
			if finalizers := sets.NewString(toUpdate.Finalizers...); finalizers.Has(v1beta1.FinalizerServiceCatalog) {
				finalizers.Delete(v1beta1.FinalizerServiceCatalog)
				toUpdate.Finalizers = finalizers.List()
			}
		}

		if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
			return err
		}

		c.recorder.Event(instance, corev1.EventTypeNormal, reason, message)
		glog.V(5).Infof(`ServiceInstance "%s/%s": Successfully %v`, instance.Namespace, instance.Name, actionText)

		err = c.finishPollingServiceInstance(instance)
		if err != nil {
			return err
		}
	case osb.StateFailed:
		description := ""
		if response.Description != nil {
			description = *response.Description
		}
		actionText := ""
		switch {
		case deleting:
			actionText = "deprovisioning"
		case provisioning:
			actionText = "provisioning"
		default:
			actionText = "updating"
		}
		s := fmt.Sprintf(`ServiceInstance "%s/%s": Error %s: %q`, instance.Namespace, instance.Name, actionText, description)
		c.recorder.Event(instance, corev1.EventTypeWarning, errorDeprovisionCalledReason, s)

		clone, err := api.Scheme.DeepCopy(instance)
		if err != nil {
			return err
		}
		toUpdate := clone.(*v1beta1.ServiceInstance)
		c.clearServiceInstanceCurrentOperation(toUpdate)

		var (
			readyCond v1beta1.ConditionStatus
			reason    string
			msg       string
		)
		switch {
		case deleting:
			readyCond = v1beta1.ConditionUnknown
			reason = errorDeprovisionCalledReason
			msg = "Deprovision call failed: " + s
		case provisioning:
			readyCond = v1beta1.ConditionFalse
			reason = errorProvisionCallFailedReason
			msg = "Provision call failed: " + s
		default:
			readyCond = v1beta1.ConditionFalse
			reason = errorUpdateInstanceCallFailedReason
			msg = "Update call failed: " + s
		}
		setServiceInstanceCondition(
			toUpdate,
			v1beta1.ServiceInstanceConditionReady,
			readyCond,
			reason,
			msg,
		)

		if !mitigatingOrphan {
			setServiceInstanceCondition(
				toUpdate,
				v1beta1.ServiceInstanceConditionFailed,
				v1beta1.ConditionTrue,
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
		glog.Warningf(`ServiceInstance "%s/%s": Got invalid state in LastOperationResponse: %q`, instance.Namespace, instance.Name, response.State)
		if !time.Now().Before(instance.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
			clone, err := api.Scheme.DeepCopy(instance)
			if err != nil {
				return err
			}
			toUpdate := clone.(*v1beta1.ServiceInstance)
			s := fmt.Sprintf(`ServiceInstance "%s/%s": Stopping reconciliation retries on ServiceInstance because too much time has elapsed`, instance.Namespace, instance.Name)
			glog.Info(s)
			c.recorder.Event(instance, corev1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)

			if !mitigatingOrphan {
				setServiceInstanceCondition(toUpdate,
					v1beta1.ServiceInstanceConditionFailed,
					v1beta1.ConditionTrue,
					errorReconciliationRetryTimeoutReason,
					s)
			}

			if !provisioning {
				c.clearServiceInstanceCurrentOperation(toUpdate)
			} else {
				setServiceInstanceStartOrphanMitigation(toUpdate)
			}

			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}
			return c.finishPollingServiceInstance(instance)
		}
		return fmt.Errorf(`ServiceInstance "%s/%s": Got invalid state in LastOperationResponse: %q`, instance.Namespace, instance.Name, response.State)
	}
	return nil
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

	if instance.Spec.ClusterServiceClassRef == nil {
		glog.V(4).Infof(`ServiceInstance "%s/%s": looking up a ClusterServiceClass from externalName: %q`, instance.Namespace, instance.Name, instance.Spec.ExternalClusterServiceClassName)
		listOpts := metav1.ListOptions{FieldSelector: "spec.externalName==" + instance.Spec.ExternalClusterServiceClassName}
		serviceClasses, err := c.serviceCatalogClient.ClusterServiceClasses().List(listOpts)
		if err == nil && len(serviceClasses.Items) == 1 {
			sc = &serviceClasses.Items[0]
			instance.Spec.ClusterServiceClassRef = &corev1.ObjectReference{
				Kind:            sc.Kind,
				Name:            sc.Name,
				UID:             sc.UID,
				APIVersion:      sc.APIVersion,
				ResourceVersion: sc.ResourceVersion,
			}
			glog.V(4).Infof(`ServiceInstance "%s/%s": resolved ClusterServiceClass with externalName %q to K8S ClusterServiceClass %q`, instance.Namespace, instance.Name, instance.Spec.ExternalClusterServiceClassName, sc.Name)
		} else {
			s := fmt.Sprintf(`ServiceInstance "%s/%s": references a non-existent ClusterServiceClass %q or there is more than one (found: %d)`, instance.Namespace, instance.Name, instance.Spec.ExternalClusterServiceClassName, len(serviceClasses.Items))
			glog.Warning(s)
			c.updateServiceInstanceCondition(
				instance,
				v1beta1.ServiceInstanceConditionReady,
				v1beta1.ConditionFalse,
				errorNonexistentClusterServiceClassReason,
				"The instance references a ClusterServiceClass that does not exist. "+s,
			)
			c.recorder.Event(instance, corev1.EventTypeWarning, errorNonexistentClusterServiceClassReason, s)
			return nil, fmt.Errorf(s)
		}
	}

	if instance.Spec.ClusterServicePlanRef == nil {
		if sc == nil {
			var scErr error
			sc, scErr = c.serviceClassLister.Get(instance.Spec.ClusterServiceClassRef.Name)
			if scErr != nil {
				return nil, fmt.Errorf(`ServiceInstance "%s/%s": Couldn't find ClusterServiceClass (K8S: %s)": %v`, instance.Namespace, instance.Name, instance.Spec.ClusterServiceClassRef.Name, scErr.Error())
			}
		}

		fieldSet := fields.Set{
			"spec.externalName":                instance.Spec.ExternalClusterServicePlanName,
			"spec.clusterServiceClassRef.name": instance.Spec.ClusterServiceClassRef.Name,
			"spec.clusterServiceBrokerName":    sc.Spec.ClusterServiceBrokerName,
		}
		fieldSelector := fields.SelectorFromSet(fieldSet).String()
		listOpts := metav1.ListOptions{FieldSelector: fieldSelector}
		servicePlans, err := c.serviceCatalogClient.ClusterServicePlans().List(listOpts)
		if err == nil && len(servicePlans.Items) == 1 {
			sp := &servicePlans.Items[0]
			instance.Spec.ClusterServicePlanRef = &corev1.ObjectReference{
				Kind:            sp.Kind,
				Name:            sp.Name,
				UID:             sp.UID,
				APIVersion:      sp.APIVersion,
				ResourceVersion: sp.ResourceVersion,
			}
			glog.V(4).Infof(`ServiceInstance "%s/%s": resolved ClusterServicePlan with externalName %q to K8S ClusterServicePlan %q`, instance.Namespace, instance.Name, instance.Spec.ExternalClusterServicePlanName, sp.Name)
		} else {
			s := fmt.Sprintf(`ServiceInstance "%s/%s": references a non-existent ClusterServicePlan %q on ClusterServiceClass %q or there is more than one (found: %d)`, instance.Namespace, instance.Name, instance.Spec.ExternalClusterServicePlanName, instance.Spec.ExternalClusterServiceClassName, len(servicePlans.Items))
			glog.Warning(s)
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

	glog.V(5).Infof(`ServiceInstance "%s/%s": Setting condition %q to %v`, toUpdate.Namespace, toUpdate.Name, conditionType, status)

	newCondition := v1beta1.ServiceInstanceCondition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	}

	if len(toUpdate.Status.Conditions) == 0 {
		glog.V(3).Infof(`ServiceInstance "%s/%s": Setting lastTransitionTime, condition %q to %v`, toUpdate.Namespace, toUpdate.Name, conditionType, t)
		newCondition.LastTransitionTime = t
		toUpdate.Status.Conditions = []v1beta1.ServiceInstanceCondition{newCondition}
		return
	}

	for i, cond := range toUpdate.Status.Conditions {
		if cond.Type == conditionType {
			if cond.Status != newCondition.Status {
				glog.V(3).Infof(`ServiceInstance "%s/%s": Found status change, condition %q: %q -> %q; setting lastTransitionTime to %v`, toUpdate.Namespace, toUpdate.Name, conditionType, cond.Status, status, t)
				newCondition.LastTransitionTime = t
			} else {
				newCondition.LastTransitionTime = cond.LastTransitionTime
			}

			toUpdate.Status.Conditions[i] = newCondition
			return
		}
	}

	glog.V(3).Infof(`ServiceInstance "%s/%s": Setting lastTransitionTime, condition %q to %v`, toUpdate.Namespace, toUpdate.Name, conditionType, t)
	newCondition.LastTransitionTime = t
	toUpdate.Status.Conditions = append(toUpdate.Status.Conditions, newCondition)
}

// updateServiceInstanceReferences updates the refs for the given instance.
func (c *controller) updateServiceInstanceReferences(toUpdate *v1beta1.ServiceInstance) (*v1beta1.ServiceInstance, error) {
	glog.V(4).Infof(`ServiceInstance "%s/%s": Updating references`, toUpdate.Namespace, toUpdate.Name)
	updatedInstance, err := c.serviceCatalogClient.ServiceInstances(toUpdate.Namespace).UpdateReferences(toUpdate)
	if err != nil {
		glog.Errorf(`ServiceInstance "%s/%s": Failed to update references: %v`, toUpdate.Namespace, toUpdate.Name, err)
	}
	return updatedInstance, err
}

// updateServiceInstanceStatus updates the status for the given instance.
//
// Note: objects coming from informers should never be mutated; the instance
// passed to this method should always be a deep copy.
func (c *controller) updateServiceInstanceStatus(toUpdate *v1beta1.ServiceInstance) (*v1beta1.ServiceInstance, error) {
	glog.V(4).Infof(`ServiceInstance "%s/%s": Updating status`, toUpdate.Namespace, toUpdate.Name)
	updatedInstance, err := c.serviceCatalogClient.ServiceInstances(toUpdate.Namespace).UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf(`ServiceInstance "%s/%s": Failed to update status: %v`, toUpdate.Namespace, toUpdate.Name, err)
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

	clone, err := api.Scheme.DeepCopy(instance)
	if err != nil {
		return err
	}
	toUpdate := clone.(*v1beta1.ServiceInstance)

	setServiceInstanceCondition(toUpdate, conditionType, status, reason, message)

	glog.V(4).Infof(`ServiceInstance "%s/%s": Updating %v condition to %v`, instance.Namespace, instance.Name, conditionType, status)
	_, err = c.serviceCatalogClient.ServiceInstances(instance.Namespace).UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf(`ServiceInstance "%s/%s": Failed to update condition %v to true: %v`, instance.Namespace, instance.Name, conditionType, err)
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
func (c *controller) recordStartOfServiceInstanceOperation(toUpdate *v1beta1.ServiceInstance, operation v1beta1.ServiceInstanceOperation) (*v1beta1.ServiceInstance, error) {
	toUpdate.Status.CurrentOperation = operation
	now := metav1.Now()
	toUpdate.Status.OperationStartTime = &now
	reason := ""
	message := ""
	switch operation {
	case v1beta1.ServiceInstanceOperationProvision:
		reason = provisioningInFlightReason
		message = provisioningInFlightMessage
	case v1beta1.ServiceInstanceOperationUpdate:
		reason = instanceUpdatingInFlightReason
		message = instanceUpdatingInFlightMessage
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

// clearServiceInstanceCurrentOperation sets the fields of the instance's Status
// to indicate that there is no current operation being performed. The Status
// is *not* recorded in the registry.
func (c *controller) clearServiceInstanceCurrentOperation(toUpdate *v1beta1.ServiceInstance) {
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
func setServiceInstanceStartOrphanMitigation(toUpdate *v1beta1.ServiceInstance) {
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

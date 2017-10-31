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

var typeSI = "ServiceInstance"

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

	glog.V(4).Infof(
		`%s "%s/%s": Received delete event; no further processing will occur`,
		typeSI, instance.Namespace, instance.Name,
	)
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
		glog.Errorf(
			`%s "%s/%s": Couldn't create a key for object %+v: %v`,
			typeSI, instance.Namespace, instance.Name, instance, err,
		)
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
		glog.Errorf(
			`%s "%s/%s": Couldn't create a key for object %+v: %v`,
			typeSI, instance.Namespace, instance.Name, instance, err,
		)
		return fmt.Errorf(`Couldn't create a key for object %+v: %v`, instance, err)
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
		glog.Infof(
			`%s "%s/%s": Not doing work for %v because it has been deleted`,
			typeSI, namespace, name, key,
		)
		return nil
	}
	if err != nil {
		glog.Errorf(
			`%s "%s/%s": Unable to retrieve %v from store: %v`,
			typeSI, instance.Namespace, instance.Name, key, err,
		)
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

	finalizerToken := v1beta1.FinalizerServiceCatalog
	finalizers := sets.NewString(instance.Finalizers...)
	if !finalizers.Has(finalizerToken) {
		return nil
	}

	// If deprovisioning has failed, do not do anything more
	if instance.Status.DeprovisionStatus == v1beta1.ServiceInstanceDeprovisionStatusFailed {
		glog.V(4).Infof(
			`%s "%s/%s": Not processing deleting event because deprovisioning has failed`,
			typeSI, instance.Namespace, instance.Name,
		)
		return nil
	}

	glog.V(4).Infof(
		`%s "%s/%s": Processing deleting event`,
		typeSI, instance.Namespace, instance.Name,
	)

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

			s := "Delete instance blocked by existing ServiceBindings associated with this instance.  All credentials must be removed first"
			glog.Warningf(
				`%s "%s/%s": %s`,
				typeSI, instance.Namespace, instance.Name, s,
			)
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

	// If the deprovisioning succeeded or is not needed, then clear out the finalizers
	if instance.Status.DeprovisionStatus == v1beta1.ServiceInstanceDeprovisionStatusNotRequired ||
		instance.Status.DeprovisionStatus == v1beta1.ServiceInstanceDeprovisionStatusSucceeded {

		glog.V(5).Infof(
			`%s "%s/%s": Clearing catalog finalizer`,
			typeSI, instance.Namespace, instance.Name,
		)
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

	// At this point, the deprovision status should be Required as the
	// controller is about to send a deprovision request. The other
	// valid values for deprovision status have all been handled above.
	// If the deprovision status is anything other than Required, then either
	// there is an invalid value or there is a logical error in the
	// controller. In either case, the controller needs to bail out, setting
	// the deprovision status to Failed.
	if instance.Status.DeprovisionStatus != v1beta1.ServiceInstanceDeprovisionStatusRequired {
		clone, err := api.Scheme.DeepCopy(instance)
		if err != nil {
			return err
		}
		toUpdate := clone.(*v1beta1.ServiceInstance)
		s := fmt.Sprintf("%s: %s", errorInvalidDeprovisionStatusMessage, instance.Status.DeprovisionStatus)
		glog.Infof(
			`%s "%s/%s": %s`,
			typeSI, instance.Namespace, instance.Name, s,
		)
		c.recorder.Event(instance, corev1.EventTypeWarning, errorInvalidDeprovisionStatusReason, s)

		if instance.Status.OrphanMitigationInProgress {
			setServiceInstanceCondition(
				toUpdate,
				v1beta1.ServiceInstanceConditionReady,
				v1beta1.ConditionUnknown,
				errorInvalidDeprovisionStatusReason,
				"Orphan mitigation failed: "+s)
		} else {
			setServiceInstanceCondition(toUpdate,
				v1beta1.ServiceInstanceConditionFailed,
				v1beta1.ConditionTrue,
				errorInvalidDeprovisionStatusReason,
				s)
		}

		c.clearServiceInstanceCurrentOperation(toUpdate)
		toUpdate.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusFailed

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
			s := fmt.Sprintf("Error building originating identity headers when deprovisioning: %v", err)
			glog.Warningf(
				`%s "%s/%s": %s`,
				typeSI, instance.Namespace, instance.Name, s,
			)
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

	if toUpdate.DeletionTimestamp == nil {
		if toUpdate.Status.OperationStartTime == nil {
			now := metav1.Now()
			toUpdate.Status.OperationStartTime = &now
		}
	} else {
		if toUpdate.Status.CurrentOperation != v1beta1.ServiceInstanceOperationDeprovision {
			// Cancel any pending orphan mitigation since the resource is being deleted
			toUpdate.Status.OrphanMitigationInProgress = false

			toUpdate, err = c.recordStartOfServiceInstanceOperation(toUpdate, v1beta1.ServiceInstanceOperationDeprovision)
			if err != nil {
				// There has been an update to the instance. Start reconciliation
				// over with a fresh view of the instance.
				return err
			}
		}
	}

	glog.V(4).Infof(
		`%s "%s/%s": Deprovisioning`,
		typeSI, instance.Namespace, instance.Name,
	)
	response, err := brokerClient.DeprovisionInstance(request)
	if err != nil {
		if httpErr, ok := osb.IsHTTPError(err); ok {
			s := fmt.Sprintf(
				"Deprovision call failed; received error response from broker: %v",
				httpErr.Error(),
			)
			glog.Warningf(
				`%s "%s/%s": %s`,
				typeSI, instance.Namespace, instance.Name, s,
			)
			c.recorder.Event(instance, corev1.EventTypeWarning, errorDeprovisionCalledReason, s)

			if instance.Status.OrphanMitigationInProgress {
				setServiceInstanceCondition(
					toUpdate,
					v1beta1.ServiceInstanceConditionReady,
					v1beta1.ConditionUnknown,
					errorOrphanMitigationFailedReason,
					"Orphan mitigation deprovision call failed. "+s)
			} else {
				setServiceInstanceCondition(
					toUpdate,
					v1beta1.ServiceInstanceConditionReady,
					v1beta1.ConditionUnknown,
					errorDeprovisionCalledReason,
					"Deprovision call failed. "+s)

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
			toUpdate.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusFailed
			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}
			return nil
		}

		s := fmt.Sprintf(
			`Error deprovisioning, ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q: %v`,
			serviceClass.Name, serviceClass.Spec.ExternalName, brokerName, err,
		)
		glog.Warningf(
			`%s "%s/%s": %s`,
			typeSI, instance.Namespace, instance.Name, s,
		)
		c.recorder.Event(instance, corev1.EventTypeWarning, errorDeprovisionCalledReason, s)

		setServiceInstanceCondition(
			toUpdate,
			v1beta1.ServiceInstanceConditionReady,
			v1beta1.ConditionUnknown,
			errorDeprovisionCalledReason,
			"Deprovision call failed. "+s)

		if !time.Now().Before(toUpdate.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
			s := "Stopping reconciliation retries because too much time has elapsed"
			glog.Infof(
				`%s "%s/%s": %s`,
				typeSI, instance.Namespace, instance.Name, s,
			)
			c.recorder.Event(instance, corev1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)

			if instance.Status.OrphanMitigationInProgress {
				setServiceInstanceCondition(
					toUpdate,
					v1beta1.ServiceInstanceConditionReady,
					v1beta1.ConditionUnknown,
					errorOrphanMitigationFailedReason,
					"Orphan mitigation deprovision call failed. "+s)
			} else {
				setServiceInstanceCondition(toUpdate,
					v1beta1.ServiceInstanceConditionFailed,
					v1beta1.ConditionTrue,
					errorReconciliationRetryTimeoutReason,
					s)
			}

			c.clearServiceInstanceCurrentOperation(toUpdate)
			toUpdate.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusFailed
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
		glog.V(5).Infof(
			`%s "%s/%s": Received asynchronous de-provisioning response, %s at %s: response: %+v`,
			typeSI, instance.Namespace, instance.Name, serviceClass.Name, brokerName, response,
		)

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

	glog.V(5).Infof(
		`%s "%s/%s": Deprovision call to broker succeeded, finalizing`,
		typeSI, instance.Namespace, instance.Name,
	)

	c.clearServiceInstanceCurrentOperation(toUpdate)
	toUpdate.Status.ExternalProperties = nil
	toUpdate.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusSucceeded

	if instance.DeletionTimestamp != nil {
		glog.V(5).Infof(
			`%s "%s/%s": Successfully deprovisioned, %s at %s`,
			typeSI, instance.Namespace, instance.Name, serviceClass.Name, brokerName,
		)
		c.recorder.Event(instance, corev1.EventTypeNormal, successDeprovisionReason, successDeprovisionMessage)

		setServiceInstanceCondition(
			toUpdate,
			v1beta1.ServiceInstanceConditionReady,
			v1beta1.ConditionFalse,
			successDeprovisionReason,
			successDeprovisionMessage,
		)

		// Clear the finalizer for normal instance deletions
		finalizers.Delete(v1beta1.FinalizerServiceCatalog)
		toUpdate.Finalizers = finalizers.List()
	} else {
		// Deprovision due to orphan mitigation successful
		glog.V(5).Infof(
			`%s "%s/%s": %s`,
			typeSI,
			instance.Namespace,
			instance.Name,
			successOrphanMitigationMessage,
		)
		c.recorder.Event(instance, corev1.EventTypeNormal, successOrphanMitigationReason, successOrphanMitigationMessage)

		setServiceInstanceCondition(
			toUpdate,
			v1beta1.ServiceInstanceConditionReady,
			v1beta1.ConditionFalse,
			successOrphanMitigationReason,
			successOrphanMitigationMessage,
		)
	}

	if _, err = c.updateServiceInstanceStatus(toUpdate); err != nil {
		return err
	}

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
		return c.pollServiceInstance(instance)
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
			`%s "%s/%s": Not processing event because status showed that it has failed`,
			typeSI, instance.Namespace, instance.Name,
		)
		return nil
	}

	// If the instance's "metadata.generation" matches its
	// "status.reconciledGeneration", then no new changes have been made to
	// the instance's spec, and we can just return.
	if instance.Status.ReconciledGeneration == instance.Generation {
		glog.V(4).Infof(
			`%s "%s/%s": Not processing event because reconciled generation showed there is no work to do`,
			typeSI, instance.Namespace, instance.Name,
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

	glog.V(4).Infof(
		`%s "%s/%s": Processing adding/updating event`,
		typeSI, instance.Namespace, instance.Name,
	)

	serviceClass, servicePlan, brokerName, brokerClient, err := c.getClusterServiceClassPlanAndClusterServiceBroker(toUpdate)
	if err != nil {
		return err
	}

	// Check if the ServiceClass or ServicePlan has been deleted and do not allow
	// creation of new ServiceInstances or plan upgrades. It's little complicated
	// since we do want to allow parameter changes on an instance whose plan or class
	// has been removed from the broker's catalog.
	// If changes are not allowed, the method will set the appropriate status / record
	// events, so we can just return here on failure.
	err = c.checkForRemovedClassAndPlan(instance, serviceClass, servicePlan)
	if err != nil {
		return err
	}

	ns, err := c.kubeClient.CoreV1().Namespaces().Get(instance.Namespace, metav1.GetOptions{})
	if err != nil {
		s := fmt.Sprintf("Failed to get namespace %q during instance create: %s", instance.Namespace, err)
		glog.Infof(
			`%s "%s/%s": %s`,
			typeSI, instance.Namespace, instance.Name, s,
		)
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
			s := fmt.Sprintf(`Failed to prepare ServiceInstance parameters %s: %s`, instance.Spec.Parameters, err)
			glog.Warningf(
				`%s "%s/%s": %s`,
				typeSI, instance.Namespace, instance.Name, s,
			)
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
			s := fmt.Sprintf("Failed to generate the parameters checksum to store in Status: %s", err)
			glog.Infof(
				`%s "%s/%s": %s`,
				typeSI, instance.Namespace, instance.Name, s,
			)
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
			s := fmt.Sprintf("Failed to marshal the parameters to store in the Status: %s", err)
			glog.Infof(
				`%s "%s/%s": %s`,
				typeSI, instance.Namespace, instance.Name, s,
			)
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
		ClusterServicePlanExternalName: servicePlan.Spec.ExternalName,
		Parameters:                     rawParametersWithRedaction,
		ParametersChecksum:             parametersChecksum,
		UserInfo:                       instance.Spec.UserInfo,
	}

	var originatingIdentity *osb.OriginatingIdentity
	if utilfeature.DefaultFeatureGate.Enabled(scfeatures.OriginatingIdentity) {
		originatingIdentity, err = buildOriginatingIdentity(instance.Spec.UserInfo)
		if err != nil {
			s := fmt.Sprintf("Error building originating identity headers for provisioning: %v", err)
			glog.Warningf(
				`%s "%s/%s": %s`,
				typeSI, instance.Namespace, instance.Name, s,
			)
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
		toUpdate.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusRequired
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
			toUpdate.Status.InProgressProperties.ClusterServicePlanExternalName != toUpdate.Status.ExternalProperties.ClusterServicePlanExternalName {
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
		glog.V(4).Infof(
			`%s "%s/%s": Provisioning a new ServiceInstance of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q`,
			typeSI, instance.Namespace, instance.Name, serviceClass.Name, serviceClass.Spec.ExternalName, brokerName,
		)
		provisionResponse, err = brokerClient.ProvisionInstance(provisionRequest)
	} else {
		glog.V(4).Infof(
			`%s "%s/%s": Updating ServiceInstance of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q`,
			typeSI, instance.Namespace, instance.Name, serviceClass.Name, serviceClass.Spec.ExternalName, brokerName,
		)
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
			s := fmt.Sprintf(
				"Error %v ServiceInstance of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q: %s",
				provisioningOrUpdatingText, serviceClass.Name, serviceClass.Spec.ExternalName, brokerName, httpErr,
			)
			glog.Warningf(
				`%s "%s/%s": %s`,
				typeSI, instance.Namespace, instance.Name, s,
			)
			c.recorder.Event(instance, corev1.EventTypeWarning, reason, s)

			setServiceInstanceCondition(
				toUpdate,
				v1beta1.ServiceInstanceConditionFailed,
				v1beta1.ConditionTrue,
				"ClusterServiceBrokerReturnedFailure",
				s)

			if isProvisioning && shouldStartOrphanMitigation(httpErr.StatusCode) {
				c.setServiceInstanceStartOrphanMitigation(toUpdate)

				if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
					return err
				}

				return httpErr
			}

			setServiceInstanceCondition(
				toUpdate,
				v1beta1.ServiceInstanceConditionReady,
				v1beta1.ConditionFalse,
				reason,
				fmt.Sprintf("ClusterServiceBroker returned a failure for %v call; operation will not be retried: %v", provisionOrUpdateText, s))

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
		s := fmt.Sprintf("Error communicating with broker for %q: %s", provisioningOrUpdatingText, err)
		glog.Warningf(
			`%s "%s/%s": %s`,
			typeSI, instance.Namespace, instance.Name, s,
		)
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
				v1beta1.ServiceInstanceConditionFailed,
				v1beta1.ConditionTrue,
				reason,
				message)

			if isProvisioning {
				c.setServiceInstanceStartOrphanMitigation(toUpdate)
			} else {
				setServiceInstanceCondition(
					toUpdate,
					v1beta1.ServiceInstanceConditionReady,
					v1beta1.ConditionFalse,
					reason,
					message)
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
			fmt.Sprintf("The %v call failed and will be retried: %v", provisionOrUpdateText, s))

		if !time.Now().Before(toUpdate.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
			s := "Stopping reconciliation retries because too much time has elapsed"
			glog.Infof(
				`%s "%s/%s": %s`,
				typeSI, instance.Namespace, instance.Name, s,
			)
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
		glog.V(5).Infof(
			`%s "%s/%s": Received asynchronous %v response for ServiceInstance of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q: response: %+v`,
			typeSI, instance.Namespace, instance.Name, provisioningOrUpdatingText, serviceClass.Name, serviceClass.Spec.ExternalName, brokerName, response,
		)

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
		glog.V(5).Infof(
			`%s "%s/%s": Successfully %v ServiceInstance of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q: response: %+v`,
			typeSI, instance.Namespace, instance.Name, provisionedOrUpdatedText, serviceClass.Name, serviceClass.Spec.ExternalName, brokerName, response,
		)

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

func (c *controller) pollServiceInstance(instance *v1beta1.ServiceInstance) error {
	glog.V(4).Infof(
		`%s "%s/%s": Processing`,
		typeSI, instance.Namespace, instance.Name,
	)

	serviceClass, servicePlan, _, brokerClient, err := c.getClusterServiceClassPlanAndClusterServiceBroker(instance)
	if err != nil {
		return err
	}

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
		s := "Stopping reconciliation retries because the operation start time is not set"
		glog.Infof(
			`%s "%s/%s": %s`,
			typeSI, instance.Namespace, instance.Name, s,
		)
		c.recorder.Event(instance, corev1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)

		if mitigatingOrphan {
			setServiceInstanceCondition(
				toUpdate,
				v1beta1.ServiceInstanceConditionReady,
				v1beta1.ConditionUnknown,
				errorOrphanMitigationFailedReason,
				"Orphan mitigation failed: "+s)
		} else {
			setServiceInstanceCondition(toUpdate,
				v1beta1.ServiceInstanceConditionFailed,
				v1beta1.ConditionTrue,
				errorReconciliationRetryTimeoutReason,
				s)
		}

		if !provisioning {
			c.clearServiceInstanceCurrentOperation(toUpdate)
			toUpdate.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusFailed
		} else {
			c.setServiceInstanceStartOrphanMitigation(toUpdate)
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
			s := fmt.Sprintf("Error building originating identity headers for polling last operation: %v", err)
			glog.Warningf(
				`%s "%s/%s": %s`,
				typeSI, instance.Namespace, instance.Name, s,
			)
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

	glog.V(5).Infof(
		`%s "%s/%s": Polling last operation`,
		typeSI, instance.Namespace, instance.Name,
	)

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

			var (
				reason  string
				message string
			)
			switch {
			case mitigatingOrphan:
				reason = successOrphanMitigationReason
				message = successOrphanMitigationMessage
			default:
				reason = successDeprovisionReason
				message = successDeprovisionMessage
			}

			c.clearServiceInstanceCurrentOperation(toUpdate)
			toUpdate.Status.ExternalProperties = nil
			toUpdate.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusSucceeded

			setServiceInstanceCondition(
				toUpdate,
				v1beta1.ServiceInstanceConditionReady,
				v1beta1.ConditionFalse,
				reason,
				message,
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

			glog.V(5).Infof(
				`%s "%s/%s": %s`,
				typeSI, instance.Namespace, instance.Name, message,
			)

			c.recorder.Event(instance, corev1.EventTypeNormal, reason, message)

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
			errText = httpErr.Error()
		} else {
			errText = err.Error()
		}

		s := fmt.Sprintf("Error polling last operation: %v", errText)
		glog.V(4).Infof(
			`%s "%s/%s": %s`,
			typeSI, instance.Namespace, instance.Name, s,
		)
		c.recorder.Event(instance, corev1.EventTypeWarning, errorPollingLastOperationReason, s)

		if !time.Now().Before(instance.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
			clone, err := api.Scheme.DeepCopy(instance)
			if err != nil {
				return err
			}
			toUpdate := clone.(*v1beta1.ServiceInstance)
			s := "Stopping reconciliation retries because too much time has elapsed"
			glog.Infof(
				`%s "%s/%s": %s`,
				typeSI, instance.Namespace, instance.Name, s,
			)
			c.recorder.Event(instance, corev1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)

			if mitigatingOrphan {
				setServiceInstanceCondition(
					toUpdate,
					v1beta1.ServiceInstanceConditionReady,
					v1beta1.ConditionUnknown,
					errorOrphanMitigationFailedReason,
					"Orphan mitigation failed: "+s)
			} else {
				setServiceInstanceCondition(toUpdate,
					v1beta1.ServiceInstanceConditionFailed,
					v1beta1.ConditionTrue,
					errorReconciliationRetryTimeoutReason,
					s)
			}

			if !provisioning {
				c.clearServiceInstanceCurrentOperation(toUpdate)
			} else {
				c.setServiceInstanceStartOrphanMitigation(toUpdate)
			}

			if deleting {
				toUpdate.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusFailed
			}

			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}
			return c.finishPollingServiceInstance(instance)
		}

		return c.continuePollingServiceInstance(instance)
	}

	glog.V(4).Infof(
		`%s "%s/%s": Poll returned %q : Response description: %v`,
		typeSI, instance.Namespace, instance.Name, response.State, response.Description,
	)

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
			s := "Stopping reconciliation retries because too much time has elapsed"
			glog.Infof(
				`%s "%s/%s": %s`,
				typeSI, instance.Namespace, instance.Name, s,
			)
			c.recorder.Event(instance, corev1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)

			if mitigatingOrphan {
				setServiceInstanceCondition(
					toUpdate,
					v1beta1.ServiceInstanceConditionReady,
					v1beta1.ConditionUnknown,
					errorOrphanMitigationFailedReason,
					"Orphan mitigation failed: "+s)
			} else {
				setServiceInstanceCondition(toUpdate,
					v1beta1.ServiceInstanceConditionFailed,
					v1beta1.ConditionTrue,
					errorReconciliationRetryTimeoutReason,
					s)
			}

			if !provisioning {
				c.clearServiceInstanceCurrentOperation(toUpdate)
			} else {
				c.setServiceInstanceStartOrphanMitigation(toUpdate)
			}

			if deleting {
				toUpdate.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusFailed
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
		glog.V(4).Infof(
			`%s "%s/%s": Last operation not completed (still in progress)`,
			typeSI, instance.Namespace, instance.Name,
		)
	case osb.StateSucceeded:
		var (
			readyStatus v1beta1.ConditionStatus
			message     string
			reason      string
			actionText  string
		)
		switch {
		case mitigatingOrphan:
			readyStatus = v1beta1.ConditionFalse
			reason = successOrphanMitigationReason
			message = successOrphanMitigationMessage
			actionText = "completed orphan mitigation"
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
		if deleting {
			toUpdate.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusSucceeded
		}

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
		glog.V(5).Infof(
			`%s "%s/%s": Successfully %v`,
			typeSI, instance.Namespace, instance.Name, actionText,
		)

		err = c.finishPollingServiceInstance(instance)
		if err != nil {
			return err
		}
	case osb.StateFailed:
		description := "(no description provided)"
		if response.Description != nil {
			description = *response.Description
		}
		var (
			readyCond v1beta1.ConditionStatus
			reason    string
			message   string
		)
		switch {
		case mitigatingOrphan:
			readyCond = v1beta1.ConditionUnknown
			reason = errorOrphanMitigationFailedReason
			message = "Orphan mitigation failed: " + description
		case deleting:
			readyCond = v1beta1.ConditionUnknown
			reason = errorDeprovisionCalledReason
			message = "Deprovision call failed: " + description
		case provisioning:
			readyCond = v1beta1.ConditionFalse
			reason = errorProvisionCallFailedReason
			message = "Provision call failed: " + description
		default:
			readyCond = v1beta1.ConditionFalse
			reason = errorUpdateInstanceCallFailedReason
			message = "Update call failed: " + description
		}

		c.recorder.Event(instance, corev1.EventTypeWarning, reason, message)
		glog.V(5).Infof(
			`%s "%s/%s": %s`,
			typeSI, instance.Namespace, instance.Name, message,
		)

		clone, err := api.Scheme.DeepCopy(instance)
		if err != nil {
			return err
		}
		toUpdate := clone.(*v1beta1.ServiceInstance)

		c.clearServiceInstanceCurrentOperation(toUpdate)
		if deleting {
			toUpdate.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusFailed
		}

		setServiceInstanceCondition(
			toUpdate,
			v1beta1.ServiceInstanceConditionReady,
			readyCond,
			reason,
			message,
		)

		if !mitigatingOrphan {
			setServiceInstanceCondition(
				toUpdate,
				v1beta1.ServiceInstanceConditionFailed,
				v1beta1.ConditionTrue,
				reason,
				message,
			)
		}

		if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
			return err
		}

		return c.finishPollingServiceInstance(instance)
	default:
		glog.Warningf(
			`%s "%s/%s": Got invalid state in LastOperationResponse: %q`,
			typeSI, instance.Namespace, instance.Name, response.State,
		)
		if !time.Now().Before(instance.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
			clone, err := api.Scheme.DeepCopy(instance)
			if err != nil {
				return err
			}
			toUpdate := clone.(*v1beta1.ServiceInstance)
			s := "Stopping reconciliation retries on ServiceInstance because too much time has elapsed"
			glog.Infof(
				`%s "%s/%s": %s`,
				typeSI, instance.Namespace, instance.Name, s,
			)
			c.recorder.Event(instance, corev1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)

			if mitigatingOrphan {
				setServiceInstanceCondition(
					toUpdate,
					v1beta1.ServiceInstanceConditionReady,
					v1beta1.ConditionUnknown,
					errorOrphanMitigationFailedReason,
					"Orphan mitigation failed: "+s)
			} else {
				setServiceInstanceCondition(toUpdate,
					v1beta1.ServiceInstanceConditionFailed,
					v1beta1.ConditionTrue,
					errorReconciliationRetryTimeoutReason,
					s)
			}

			if !provisioning {
				c.clearServiceInstanceCurrentOperation(toUpdate)
			} else {
				c.setServiceInstanceStartOrphanMitigation(toUpdate)
			}

			if deleting {
				toUpdate.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusFailed
			}

			if _, err := c.updateServiceInstanceStatus(toUpdate); err != nil {
				return err
			}
			return c.finishPollingServiceInstance(instance)
		}
		return fmt.Errorf(`Got invalid state in LastOperationResponse: %q`, response.State)
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
				return nil, fmt.Errorf(`%s "%s/%s": Couldn't find ClusterServiceClass (K8S: %s)": %v`, typeSI, instance.Namespace, instance.Name, instance.Spec.ClusterServiceClassRef.Name, scErr.Error())
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
	var sc *v1beta1.ClusterServiceClass
	if instance.Spec.ClusterServiceClassExternalName != "" {
		glog.V(4).Infof(
			`%s "%s/%s": looking up a ClusterServiceClass from externalName: %q`,
			typeSI, instance.Namespace, instance.Name, instance.Spec.ClusterServiceClassExternalName,
		)
		listOpts := metav1.ListOptions{FieldSelector: "spec.externalName==" + instance.Spec.ClusterServiceClassExternalName}
		serviceClasses, err := c.serviceCatalogClient.ClusterServiceClasses().List(listOpts)
		if err == nil && len(serviceClasses.Items) == 1 {
			sc = &serviceClasses.Items[0]
			instance.Spec.ClusterServiceClassRef = &v1beta1.ClusterObjectReference{
				Name: sc.Name,
			}
			glog.V(4).Infof(
				`%s "%s/%s": resolved ClusterServiceClass with externalName %q to K8S ClusterServiceClass %q`,
				typeSI, instance.Namespace, instance.Name, instance.Spec.ClusterServiceClassExternalName, sc.Name,
			)
		} else {
			s := fmt.Sprintf(
				"References a non-existent ClusterServiceClass (ExternalName: %q) or there is more than one (found: %d)",
				instance.Spec.ClusterServiceClassExternalName, len(serviceClasses.Items),
			)
			glog.Warningf(
				`%s "%s/%s": %s`,
				typeSI, instance.Namespace, instance.Name, s,
			)
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
		glog.V(4).Infof(
			`%s "%s/%s": looking up a ClusterServiceClass from K8S Name: %q`,
			typeSI, instance.Namespace, instance.Name, instance.Spec.ClusterServiceClassName,
		)

		var err error
		sc, err = c.serviceClassLister.Get(instance.Spec.ClusterServiceClassName)
		if err == nil {
			instance.Spec.ClusterServiceClassRef = &v1beta1.ClusterObjectReference{
				Name: sc.Name,
			}
			glog.V(4).Infof(
				`%s "%s/%s": resolved ClusterServiceClass with K8S name %q to ClusterServiceClass with external Name %q`,
				typeSI, instance.Namespace, instance.Name, instance.Spec.ClusterServiceClassName, sc.Spec.ExternalName,
			)
		} else {
			s := fmt.Sprintf(
				"References a non-existent ClusterServiceClass (K8S: %q)",
				instance.Spec.ClusterServiceClassName,
			)
			glog.Warningf(
				`%s "%s/%s": %s`,
				typeSI, instance.Namespace, instance.Name, s,
			)
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
			glog.V(4).Infof(
				`%s "%s/%s": resolved ClusterServicePlan (ExternalName: %q) to ClusterServicePlan (K8S: %q)`,
				typeSI, instance.Namespace, instance.Name, instance.Spec.ClusterServicePlanExternalName, sp.Name,
			)
		} else {
			s := fmt.Sprintf(
				"References a non-existent ClusterServicePlan (K8S: %q ExternalName: %q) on ClusterServiceClass (K8S: %q ExternalName: %q) or there is more than one (found: %d)",
				instance.Spec.ClusterServicePlanName, instance.Spec.ClusterServicePlanExternalName, instance.Spec.ClusterServiceClassRef.Name, instance.Spec.ClusterServiceClassExternalName, len(servicePlans.Items),
			)
			glog.Warningf(
				`%s "%s/%s": %s`,
				typeSI, instance.Namespace, instance.Name, s,
			)
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
			glog.V(4).Infof(
				`%s "%s/%s": resolved ClusterServicePlan with K8S name %q to ClusterServicePlan with external name %q`,
				typeSI, instance.Namespace, instance.Name, instance.Spec.ClusterServicePlanName, sp.Spec.ExternalName,
			)
		} else {
			s := fmt.Sprintf(
				"References a non-existent ClusterServicePlan with K8S name %q on ClusterServiceClass with K8S name %q",
				instance.Spec.ClusterServicePlanName, instance.Spec.ClusterServiceClassName,
			)
			glog.Warningf(
				`%s "%s/%s": %s`,
				typeSI, instance.Namespace, instance.Name, s,
			)
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

	glog.V(5).Infof(
		`%s "%s/%s": Setting condition %q to %v`,
		typeSI, toUpdate.Namespace, toUpdate.Name, conditionType, status,
	)

	newCondition := v1beta1.ServiceInstanceCondition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	}

	if len(toUpdate.Status.Conditions) == 0 {
		glog.V(3).Infof(
			`%s "%s/%s": Setting lastTransitionTime, condition %q to %v`,
			typeSI, toUpdate.Namespace, toUpdate.Name, conditionType, t,
		)
		newCondition.LastTransitionTime = t
		toUpdate.Status.Conditions = []v1beta1.ServiceInstanceCondition{newCondition}
		return
	}

	for i, cond := range toUpdate.Status.Conditions {
		if cond.Type == conditionType {
			if cond.Status != newCondition.Status {
				glog.V(3).Infof(
					`%s "%s/%s": Found status change, condition %q: %q -> %q; setting lastTransitionTime to %v`,
					typeSI, toUpdate.Namespace, toUpdate.Name, conditionType, cond.Status, status, t,
				)
				newCondition.LastTransitionTime = t
			} else {
				newCondition.LastTransitionTime = cond.LastTransitionTime
			}

			toUpdate.Status.Conditions[i] = newCondition
			return
		}
	}

	glog.V(3).Infof(
		`%s "%s/%s": Setting lastTransitionTime, condition %q to %v`,
		typeSI, toUpdate.Namespace, toUpdate.Name, conditionType, t,
	)
	newCondition.LastTransitionTime = t
	toUpdate.Status.Conditions = append(toUpdate.Status.Conditions, newCondition)
}

// updateServiceInstanceReferences updates the refs for the given instance.
func (c *controller) updateServiceInstanceReferences(toUpdate *v1beta1.ServiceInstance) (*v1beta1.ServiceInstance, error) {
	glog.V(4).Infof(
		`%s "%s/%s": Updating references`,
		typeSI, toUpdate.Namespace, toUpdate.Name,
	)
	updatedInstance, err := c.serviceCatalogClient.ServiceInstances(toUpdate.Namespace).UpdateReferences(toUpdate)
	if err != nil {
		glog.Errorf(
			`%s "%s/%s": Failed to update references: %v`,
			typeSI, toUpdate.Namespace, toUpdate.Name, err,
		)
	}
	return updatedInstance, err
}

// updateServiceInstanceStatus updates the status for the given instance.
//
// Note: objects coming from informers should never be mutated; the instance
// passed to this method should always be a deep copy.
func (c *controller) updateServiceInstanceStatus(toUpdate *v1beta1.ServiceInstance) (*v1beta1.ServiceInstance, error) {
	glog.V(4).Infof(
		`%s "%s/%s": Updating status`,
		typeSI, toUpdate.Namespace, toUpdate.Name,
	)
	updatedInstance, err := c.serviceCatalogClient.ServiceInstances(toUpdate.Namespace).UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf(
			`%s "%s/%s": Failed to update status: %v`,
			typeSI, toUpdate.Namespace, toUpdate.Name, err,
		)
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

	glog.V(4).Infof(
		`%s "%s/%s": Updating %v condition to %v`,
		typeSI, instance.Namespace, instance.Name, conditionType, status,
	)
	_, err = c.serviceCatalogClient.ServiceInstances(instance.Namespace).UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf(
			`%s "%s/%s": Failed to update condition %v to true: %v`,
			typeSI, instance.Namespace, instance.Name, conditionType, err,
		)
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
func (c *controller) setServiceInstanceStartOrphanMitigation(toUpdate *v1beta1.ServiceInstance) {
	glog.V(5).Infof(
		`%s "%s/%s": %s`,
		typeSI,
		toUpdate.Name,
		toUpdate.Namespace,
		startingInstanceOrphanMitigationMessage,
	)

	c.recorder.Event(
		toUpdate,
		corev1.EventTypeWarning,
		startingInstanceOrphanMitigationReason,
		startingInstanceOrphanMitigationMessage,
	)

	toUpdate.Status.OperationStartTime = nil
	toUpdate.Status.AsyncOpInProgress = false
	toUpdate.Status.OrphanMitigationInProgress = true

	setServiceInstanceCondition(
		toUpdate,
		v1beta1.ServiceInstanceConditionReady,
		v1beta1.ConditionFalse,
		startingInstanceOrphanMitigationReason,
		startingInstanceOrphanMitigationMessage,
	)
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
		servicePlan.Spec.ExternalName == instance.Status.ExternalProperties.ClusterServicePlanExternalName {
		// Service Instance has already been provisioned and we're only
		// updating parameters, so let it through.
		return nil
	}

	// At this point we know that plan is being changed
	if planDeleted {
		s := fmt.Sprintf("Service Plan %q (K8S name: %q) has been deleted, can not provision.", servicePlan.Spec.ExternalName, servicePlan.Name)
		glog.Warningf(
			`%s "%s/%s": %s`,
			typeSI, instance.Namespace, instance.Name, s,
		)
		c.recorder.Event(instance, corev1.EventTypeWarning, errorDeletedClusterServicePlanReason, s)

		setServiceInstanceCondition(
			instance,
			v1beta1.ServiceInstanceConditionReady,
			v1beta1.ConditionFalse,
			errorDeletedClusterServicePlanReason,
			s,
		)
		if _, err := c.updateServiceInstanceStatus(instance); err != nil {
			return err
		}
		return fmt.Errorf(s)
	}

	s := fmt.Sprintf("Service Class %q (K8S name: %q) has been deleted, can not provision.", serviceClass.Spec.ExternalName, serviceClass.Name)
	glog.Warningf(
		`%s "%s/%s": %s`,
		typeSI, instance.Namespace, instance.Name, s,
	)
	c.recorder.Event(instance, corev1.EventTypeWarning, errorDeletedClusterServiceClassReason, s)

	setServiceInstanceCondition(
		instance,
		v1beta1.ServiceInstanceConditionReady,
		v1beta1.ConditionFalse,
		errorDeletedClusterServiceClassReason,
		s,
	)
	if _, err := c.updateServiceInstanceStatus(instance); err != nil {
		return err
	}
	return fmt.Errorf(s)
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

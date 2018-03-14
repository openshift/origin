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
	"net"

	"github.com/golang/glog"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	scfeatures "github.com/kubernetes-incubator/service-catalog/pkg/features"
	"github.com/kubernetes-incubator/service-catalog/pkg/pretty"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
)

const (
	errorNonexistentServiceInstanceReason     string = "ReferencesNonexistentInstance"
	errorBindCallReason                       string = "BindCallFailed"
	errorInjectingBindResultReason            string = "ErrorInjectingBindResult"
	errorEjectingBindReason                   string = "ErrorEjectingServiceBinding"
	errorUnbindCallReason                     string = "UnbindCallFailed"
	errorNonbindableClusterServiceClassReason string = "ErrorNonbindableServiceClass"
	errorServiceInstanceNotReadyReason        string = "ErrorInstanceNotReady"
	errorServiceBindingOrphanMitigation       string = "ServiceBindingNeedsOrphanMitigation"
	errorFetchingBindingFailedReason          string = "FetchingBindingFailed"
	errorAsyncOpTimeoutReason                 string = "AsyncOperationTimeout"

	successInjectedBindResultReason  string = "InjectedBindResult"
	successInjectedBindResultMessage string = "Injected bind result"
	successUnboundReason             string = "UnboundSuccessfully"
	asyncBindingReason               string = "Binding"
	asyncBindingMessage              string = "The binding is being created asynchronously"
	asyncUnbindingReason             string = "Unbinding"
	asyncUnbindingMessage            string = "The binding is being deleted asynchronously"
	bindingInFlightReason            string = "BindingRequestInFlight"
	bindingInFlightMessage           string = "Binding request for ServiceBinding in-flight to Broker"
	unbindingInFlightReason          string = "UnbindingRequestInFlight"
	unbindingInFlightMessage         string = "Unbind request for ServiceBinding in-flight to Broker"
)

// bindingControllerKind contains the schema.GroupVersionKind for this controller type.
var bindingControllerKind = v1beta1.SchemeGroupVersion.WithKind("ServiceBinding")

// ServiceBinding handlers and control-loop

func (c *controller) bindingAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		pcb := pretty.NewContextBuilder(pretty.ServiceBinding, "", "")
		glog.Errorf(pcb.Messagef("Couldn't get key for object %+v: %v", obj, err))
		return
	}
	pcb := pretty.NewContextBuilder(pretty.ServiceBinding, "", key)

	acc, err := meta.Accessor(obj)
	if err != nil {
		glog.Errorf(pcb.Messagef("error creating meta accessor: %v", err))
		return
	}

	glog.V(6).Info(pcb.Messagef(
		"received ADD/UPDATE event for: resourceVersion: %v",
		acc.GetResourceVersion()),
	)

	c.bindingQueue.Add(key)
}

func (c *controller) bindingUpdate(oldObj, newObj interface{}) {
	// Bindings with ongoing asynchronous operations will be manually added
	// to the polling queue by the reconciler. They should be ignored here in
	// order to enforce polling rate-limiting.
	binding := newObj.(*v1beta1.ServiceBinding)
	if !binding.Status.AsyncOpInProgress {
		c.bindingAdd(newObj)
	}
}

func (c *controller) bindingDelete(obj interface{}) {
	binding, ok := obj.(*v1beta1.ServiceBinding)
	if binding == nil || !ok {
		return
	}

	pcb := pretty.NewContextBuilder(pretty.ServiceBinding, binding.Namespace, binding.Name)
	glog.V(4).Info(pcb.Messagef("Received DELETE event; no further processing will occur; resourceVersion %v", binding.ResourceVersion))
}

func (c *controller) reconcileServiceBindingKey(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	pcb := pretty.NewContextBuilder(pretty.ServiceBinding, namespace, name)
	binding, err := c.bindingLister.ServiceBindings(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		glog.Info(pcb.Message("Not doing work because the ServiceBinding has been deleted"))
		return nil
	}
	if err != nil {
		glog.Info(pcb.Messagef("Unable to retrieve store: %v", err))
		return err
	}

	return c.reconcileServiceBinding(binding)
}

func isServiceBindingFailed(binding *v1beta1.ServiceBinding) bool {
	for _, condition := range binding.Status.Conditions {
		if condition.Type == v1beta1.ServiceBindingConditionFailed && condition.Status == v1beta1.ConditionTrue {
			return true
		}
	}
	return false
}

// getReconciliationActionForServiceBinding gets the action the reconciler
// should be taking on the given binding.
func getReconciliationActionForServiceBinding(binding *v1beta1.ServiceBinding) ReconciliationAction {
	switch {
	case binding.Status.AsyncOpInProgress:
		return reconcilePoll
	case binding.ObjectMeta.DeletionTimestamp != nil || binding.Status.OrphanMitigationInProgress:
		return reconcileDelete
	default:
		return reconcileAdd
	}
}

// reconcileServiceBinding is the control-loop for reconciling ServiceBindings.
// An error is returned to indicate that the binding has not been fully
// processed and should be resubmitted at a later time.
func (c *controller) reconcileServiceBinding(binding *v1beta1.ServiceBinding) error {
	pcb := pretty.NewContextBuilder(pretty.ServiceBinding, binding.Namespace, binding.Name)
	glog.V(6).Info(pcb.Messagef(`beginning to process resourceVersion: %v`, binding.ResourceVersion))

	reconciliationAction := getReconciliationActionForServiceBinding(binding)
	switch reconciliationAction {
	case reconcileAdd:
		return c.reconcileServiceBindingAdd(binding)
	case reconcileDelete:
		return c.reconcileServiceBindingDelete(binding)
	case reconcilePoll:
		return c.pollServiceBinding(binding)
	default:
		return fmt.Errorf(pcb.Messagef("Unknown reconciliation action %v", reconciliationAction))
	}
}

// reconcileServiceBindingAdd is responsible for handling the creation of new
// service bindings.
func (c *controller) reconcileServiceBindingAdd(binding *v1beta1.ServiceBinding) error {
	pcb := pretty.NewContextBuilder(pretty.ServiceBinding, binding.Namespace, binding.Name)

	if isServiceBindingFailed(binding) {
		glog.V(4).Info(pcb.Message("not processing event; status showed that it has failed"))
		return nil
	}

	if binding.Status.ReconciledGeneration == binding.Generation {
		glog.V(4).Info(pcb.Message("Not processing event; reconciled generation showed there is no work to do"))
		return nil
	}

	glog.V(4).Info(pcb.Message("Processing"))

	binding = binding.DeepCopy()

	instance, err := c.instanceLister.ServiceInstances(binding.Namespace).Get(binding.Spec.ServiceInstanceRef.Name)
	if err != nil {
		msg := fmt.Sprintf(`References a non-existent %s "%s/%s"`, pretty.ServiceInstance, binding.Namespace, binding.Spec.ServiceInstanceRef.Name)
		readyCond := newServiceBindingReadyCondition(v1beta1.ConditionFalse, errorNonexistentServiceInstanceReason, msg)
		return c.processServiceBindingOperationError(binding, readyCond)
	}

	if instance.Spec.ClusterServiceClassRef == nil || instance.Spec.ClusterServicePlanRef == nil {
		// retry later
		return fmt.Errorf("ClusterServiceClass or ClusterServicePlan references for Instance have not been resolved yet")
	}

	serviceClass, servicePlan, brokerName, brokerClient, err := c.getClusterServiceClassPlanAndClusterServiceBrokerForServiceBinding(instance, binding)
	if err != nil {
		return c.handleServiceBindingReconciliationError(binding, err)
	}

	if !isPlanBindable(serviceClass, servicePlan) {
		msg := fmt.Sprintf(`References a non-bindable %s and Plan (%q) combination`, pretty.ClusterServiceClassName(serviceClass), instance.Spec.ClusterServicePlanExternalName)
		readyCond := newServiceBindingReadyCondition(v1beta1.ConditionFalse, errorNonbindableClusterServiceClassReason, msg)
		failedCond := newServiceBindingFailedCondition(v1beta1.ConditionTrue, errorNonbindableClusterServiceClassReason, msg)
		return c.processBindFailure(binding, readyCond, failedCond, false)
	}

	if !isServiceInstanceReady(instance) {
		msg := fmt.Sprintf(`Binding cannot begin because referenced %s is not ready`, pretty.ServiceInstanceName(instance))
		readyCond := newServiceBindingReadyCondition(v1beta1.ConditionFalse, errorServiceInstanceNotReadyReason, msg)
		return c.processServiceBindingOperationError(binding, readyCond)
	}

	glog.V(4).Info(pcb.Message("Adding/Updating"))

	request, inProgressProperties, err := c.prepareBindRequest(binding, instance, serviceClass, servicePlan)
	if err != nil {
		return c.handleServiceBindingReconciliationError(binding, err)
	}

	if binding.Status.CurrentOperation == "" {
		binding, err = c.recordStartOfServiceBindingOperation(binding, v1beta1.ServiceBindingOperationBind, inProgressProperties)
		if err != nil {
			// There has been an update to the binding. Start reconciliation
			// over with a fresh view of the binding.
			return err
		}
	}

	response, err := brokerClient.Bind(request)
	if err != nil {
		if httpErr, ok := osb.IsHTTPError(err); ok {
			msg := fmt.Sprintf("ServiceBroker returned failure; bind operation will not be retried: %v", err.Error())
			readyCond := newServiceBindingReadyCondition(v1beta1.ConditionFalse, errorBindCallReason, msg)
			failedCond := newServiceBindingFailedCondition(v1beta1.ConditionTrue, "ServiceBindingReturnedFailure", msg)
			return c.processBindFailure(binding, readyCond, failedCond, shouldStartOrphanMitigation(httpErr.StatusCode))
		}

		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			msg := "Communication with the ServiceBroker timed out; Bind operation will not be retried: " + err.Error()
			failedCond := newServiceBindingFailedCondition(v1beta1.ConditionTrue, errorBindCallReason, msg)
			return c.processBindFailure(binding, nil, failedCond, true)
		}

		msg := fmt.Sprintf(
			`Error creating ServiceBinding for %s: %s`,
			pretty.FromServiceInstanceOfClusterServiceClassAtBrokerName(instance, serviceClass, brokerName), err,
		)
		readyCond := newServiceBindingReadyCondition(v1beta1.ConditionFalse, errorBindCallReason, msg)

		if c.reconciliationRetryDurationExceeded(binding.Status.OperationStartTime) {
			msg := "Stopping reconciliation retries, too much time has elapsed"
			failedCond := newServiceBindingFailedCondition(v1beta1.ConditionTrue, errorReconciliationRetryTimeoutReason, msg)
			return c.processBindFailure(binding, readyCond, failedCond, false)
		}

		return c.processServiceBindingOperationError(binding, readyCond)
	}

	if response.Async {
		return c.processBindAsyncResponse(binding, response)
	}

	// Save off the external properties here even if the subsequent
	// credentials injection fails. The Broker has already processed the
	// request, so this is what the Broker knows about the state of the
	// binding.
	binding.Status.ExternalProperties = binding.Status.InProgressProperties

	err = c.injectServiceBinding(binding, response.Credentials)
	if err != nil {
		msg := fmt.Sprintf(`Error injecting bind result: %s`, err)
		readyCond := newServiceBindingReadyCondition(v1beta1.ConditionFalse, errorInjectingBindResultReason, msg)

		if c.reconciliationRetryDurationExceeded(binding.Status.OperationStartTime) {
			msg := "Stopping reconciliation retries, too much time has elapsed"
			failedCond := newServiceBindingFailedCondition(v1beta1.ConditionTrue, errorReconciliationRetryTimeoutReason, msg)
			return c.processBindFailure(binding, readyCond, failedCond, true)
		}

		// TODO: solve scenario where bind request successful, credential injection fails, later reconciliations have non-failing errors
		// with Bind request. After retry duration, reconciler gives up but will not do orphan mitigation.
		return c.processServiceBindingOperationError(binding, readyCond)
	}

	return c.processBindSuccess(binding)
}

func (c *controller) reconcileServiceBindingDelete(binding *v1beta1.ServiceBinding) error {
	var err error
	pcb := pretty.NewContextBuilder(pretty.ServiceBinding, binding.Namespace, binding.Name)

	if binding.DeletionTimestamp == nil && !binding.Status.OrphanMitigationInProgress {
		// nothing to do...
		return nil
	}

	if finalizers := sets.NewString(binding.Finalizers...); !finalizers.Has(v1beta1.FinalizerServiceCatalog) {
		return nil
	}

	// If unbind has failed, do not do anything more
	if binding.Status.UnbindStatus == v1beta1.ServiceBindingUnbindStatusFailed {
		glog.V(4).Info(pcb.Message("Not processing delete event because unbinding has failed"))
		return nil
	}

	glog.V(4).Info(pcb.Message("Processing Delete"))

	binding = binding.DeepCopy()

	// If unbinding succeeded or is not needed, then clear out the finalizers
	if binding.Status.UnbindStatus == v1beta1.ServiceBindingUnbindStatusNotRequired ||
		binding.Status.UnbindStatus == v1beta1.ServiceBindingUnbindStatusSucceeded {

		return c.processServiceBindingGracefulDeletionSuccess(binding)
	}

	if err := c.ejectServiceBinding(binding); err != nil {
		msg := fmt.Sprintf(`Error ejecting binding. Error deleting secret: %s`, err)
		readyCond := newServiceBindingReadyCondition(v1beta1.ConditionFalse, errorEjectingBindReason, msg)
		return c.processServiceBindingOperationError(binding, readyCond)
	}

	if binding.DeletionTimestamp == nil {
		if binding.Status.OperationStartTime == nil {
			now := metav1.Now()
			binding.Status.OperationStartTime = &now
		}
	} else {
		if binding.Status.CurrentOperation != v1beta1.ServiceBindingOperationUnbind {
			binding, err = c.recordStartOfServiceBindingOperation(binding, v1beta1.ServiceBindingOperationUnbind, nil)
			if err != nil {
				// There has been an update to the binding. Start reconciliation
				// over with a fresh view of the binding.
				return err
			}
		}
	}

	instance, err := c.instanceLister.ServiceInstances(binding.Namespace).Get(binding.Spec.ServiceInstanceRef.Name)
	if err != nil {
		msg := fmt.Sprintf(
			`References a non-existent %s "%s/%s"`,
			pretty.ServiceInstance, binding.Namespace, binding.Spec.ServiceInstanceRef.Name,
		)
		readyCond := newServiceBindingReadyCondition(v1beta1.ConditionFalse, errorNonexistentServiceInstanceReason, msg)
		return c.processServiceBindingOperationError(binding, readyCond)
	}

	if instance.Status.AsyncOpInProgress {
		msg := fmt.Sprintf(
			`trying to unbind to %s "%s/%s" that has ongoing asynchronous operation`,
			pretty.ServiceInstance, binding.Namespace, binding.Spec.ServiceInstanceRef.Name,
		)
		readyCond := newServiceBindingReadyCondition(v1beta1.ConditionFalse, errorWithOngoingAsyncOperation, msg)
		return c.processServiceBindingOperationError(binding, readyCond)
	}

	if instance.Spec.ClusterServiceClassRef == nil || instance.Spec.ClusterServicePlanRef == nil {
		// TODO(#1562): ultimately here we need to use logic similar to what is done to determine the plan ID for
		// deprovisioning. We need to allow a ServiceBinding to be deleted, with an unbind request sent to the broker,
		// even if the ServiceInstance has been changed to a non-existent plan.
		return fmt.Errorf("ClusterServiceClass or ClusterServicePlan references for Instance have not been resolved yet")
	}

	serviceClass, servicePlan, brokerName, brokerClient, err := c.getClusterServiceClassPlanAndClusterServiceBrokerForServiceBinding(instance, binding)
	if err != nil {
		return c.handleServiceBindingReconciliationError(binding, err)
	}

	request, err := c.prepareUnbindRequest(binding, instance, serviceClass, servicePlan)
	if err != nil {
		return c.handleServiceBindingReconciliationError(binding, err)
	}

	response, err := brokerClient.Unbind(request)
	if err != nil {
		msg := fmt.Sprintf(
			`Error unbinding from %s: %s`,
			pretty.FromServiceInstanceOfClusterServiceClassAtBrokerName(instance, serviceClass, brokerName), err,
		)
		readyCond := newServiceBindingReadyCondition(v1beta1.ConditionUnknown, errorUnbindCallReason, msg)

		if c.reconciliationRetryDurationExceeded(binding.Status.OperationStartTime) {
			msg := "Stopping reconciliation retries, too much time has elapsed"
			failedCond := newServiceBindingReadyCondition(v1beta1.ConditionTrue, errorReconciliationRetryTimeoutReason, msg)
			return c.processUnbindFailure(binding, readyCond, failedCond)
		}

		return c.processServiceBindingOperationError(binding, readyCond)
	}

	if response.Async {
		return c.processUnbindAsyncResponse(binding, response)
	}

	return c.processUnbindSuccess(binding)
}

// isPlanBindable returns whether the given ClusterServiceClass and ClusterServicePlan
// combination is bindable.  Plans may override the service-level bindable
// attribute, so if the plan provides a value, return that value.  Otherwise,
// return the Bindable field of the ClusterServiceClass.
//
// Note: enforcing that the plan belongs to the given service class is the
// responsibility of the caller.
func isPlanBindable(serviceClass *v1beta1.ClusterServiceClass, plan *v1beta1.ClusterServicePlan) bool {
	if plan.Spec.Bindable != nil {
		return *plan.Spec.Bindable
	}

	return serviceClass.Spec.Bindable
}

func (c *controller) injectServiceBinding(binding *v1beta1.ServiceBinding, credentials map[string]interface{}) error {
	pcb := pretty.NewContextBuilder(pretty.ServiceBinding, binding.Namespace, binding.Name)
	glog.V(5).Info(pcb.Messagef(`Creating/updating Secret "%s/%s" with %d keys`,
		binding.Namespace, binding.Spec.SecretName, len(credentials),
	))

	secretData := make(map[string][]byte)
	for k, v := range credentials {
		var err error
		secretData[k], err = serialize(v)
		if err != nil {
			return fmt.Errorf("Unable to serialize value for credential key %q (value is intentionally not logged): %s", k, err)
		}
	}

	// Creating/updating the Secret
	secretClient := c.kubeClient.CoreV1().Secrets(binding.Namespace)
	existingSecret, err := secretClient.Get(binding.Spec.SecretName, metav1.GetOptions{})
	if err == nil {
		// Update existing secret
		if !metav1.IsControlledBy(existingSecret, binding) {
			controllerRef := metav1.GetControllerOf(existingSecret)
			return fmt.Errorf(`Secret "%s/%s" is not owned by ServiceBinding, controllerRef: %v`, binding.Namespace, existingSecret.Name, controllerRef)
		}
		existingSecret.Data = secretData
		_, err = secretClient.Update(existingSecret)
		if err != nil {
			if apierrors.IsConflict(err) {
				// Conflicting update detected, try again later
				return fmt.Errorf(`Conflicting Secret "%s/%s" update detected`, binding.Namespace, existingSecret.Name)
			}
			return fmt.Errorf(`Unexpected error updating Secret "%s/%s": %v`, binding.Namespace, existingSecret.Name, err)
		}
	} else {
		if !apierrors.IsNotFound(err) {
			// Terminal error
			return fmt.Errorf(`Unexpected error getting Secret "%s/%s": %v`, binding.Namespace, existingSecret.Name, err)
		}
		err = nil

		// Create new secret
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      binding.Spec.SecretName,
				Namespace: binding.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(binding, bindingControllerKind),
				},
			},
			Data: secretData,
		}
		_, err = secretClient.Create(secret)
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				// Concurrent controller has created secret under the same name,
				// Update the secret at the next retry iteration
				return fmt.Errorf(`Conflicting Secret "%s/%s" creation detected`, binding.Namespace, secret.Name)
			}
			// Terminal error
			return fmt.Errorf(`Unexpected error creating Secret "%s/%s": %v`, binding.Namespace, secret.Name, err)
		}
	}

	return err
}

func (c *controller) ejectServiceBinding(binding *v1beta1.ServiceBinding) error {
	var err error
	pcb := pretty.NewContextBuilder(pretty.ServiceBinding, binding.Namespace, binding.Name)
	glog.V(5).Info(pcb.Messagef(`Deleting Secret "%s/%s"`,
		binding.Namespace, binding.Spec.SecretName,
	))
	err = c.kubeClient.CoreV1().Secrets(binding.Namespace).Delete(binding.Spec.SecretName, &metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

// setServiceBindingCondition sets a single condition on a ServiceBinding's
// status: if the condition already exists in the status, it is mutated; if the
// condition does not already exist in the status, it is added. Other
// conditions in the // status are not altered. If the condition exists and its
// status changes, the LastTransitionTime field is updated.

//
// Note: objects coming from informers should never be mutated; always pass a
// deep copy as the binding parameter.
func setServiceBindingCondition(toUpdate *v1beta1.ServiceBinding,
	conditionType v1beta1.ServiceBindingConditionType,
	status v1beta1.ConditionStatus,
	reason, message string) {

	setServiceBindingConditionInternal(toUpdate, conditionType, status, reason, message, metav1.Now())
}

// setServiceBindingConditionInternal is
// setServiceBindingCondition but allows the time to be parameterized
// for testing.
func setServiceBindingConditionInternal(toUpdate *v1beta1.ServiceBinding,
	conditionType v1beta1.ServiceBindingConditionType,
	status v1beta1.ConditionStatus,
	reason, message string,
	t metav1.Time) {
	pcb := pretty.NewContextBuilder(pretty.ServiceBinding, toUpdate.Namespace, toUpdate.Name)
	glog.Info(pcb.Message(message))
	glog.V(5).Info(pcb.Messagef(
		"Setting condition %q to %v",
		conditionType, status,
	))

	newCondition := v1beta1.ServiceBindingCondition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	}

	if len(toUpdate.Status.Conditions) == 0 {
		glog.Info(pcb.Messagef(
			"Setting lastTransitionTime for condition %q to %v",
			conditionType, t,
		))
		newCondition.LastTransitionTime = t
		toUpdate.Status.Conditions = []v1beta1.ServiceBindingCondition{newCondition}
		return
	}
	for i, cond := range toUpdate.Status.Conditions {
		if cond.Type == conditionType {
			if cond.Status != newCondition.Status {
				glog.V(3).Info(pcb.Messagef(
					"Found status change for condition %q: %q -> %q; setting lastTransitionTime to %v",
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

	glog.V(3).Info(
		pcb.Messagef("Setting lastTransitionTime for condition %q to %v",
			conditionType, t,
		))

	newCondition.LastTransitionTime = t
	toUpdate.Status.Conditions = append(toUpdate.Status.Conditions, newCondition)
}

func (c *controller) updateServiceBindingStatus(toUpdate *v1beta1.ServiceBinding) (*v1beta1.ServiceBinding, error) {
	pcb := pretty.NewContextBuilder(pretty.ServiceBinding, toUpdate.Namespace, toUpdate.Name)
	glog.V(4).Info(pcb.Message("Updating status"))
	updatedBinding, err := c.serviceCatalogClient.ServiceBindings(toUpdate.Namespace).UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf(pcb.Messagef("Error updating status: %v", err))
	} else {
		glog.V(6).Info(pcb.Messagef(`Updated status of resourceVersion: %v; got resourceVersion: %v`,
			toUpdate.ResourceVersion, updatedBinding.ResourceVersion),
		)
	}

	return updatedBinding, err
}

// updateServiceBindingCondition updates the given condition for the given ServiceBinding
// with the given status, reason, and message.
func (c *controller) updateServiceBindingCondition(
	binding *v1beta1.ServiceBinding,
	conditionType v1beta1.ServiceBindingConditionType,
	status v1beta1.ConditionStatus,
	reason, message string) error {

	pcb := pretty.NewContextBuilder(pretty.ServiceBinding, binding.Namespace, binding.Name)
	toUpdate := binding.DeepCopy()

	setServiceBindingCondition(toUpdate, conditionType, status, reason, message)

	glog.V(4).Info(pcb.Messagef(
		"Updating %v condition to %v (Reason: %q, Message: %q)",
		conditionType, status, reason, message,
	))
	_, err := c.serviceCatalogClient.ServiceBindings(binding.Namespace).UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf(pcb.Messagef(
			"Error updating %v condition to %v: %v",
			status, err,
		))
	}
	return err
}

// recordStartOfServiceBindingOperation updates the binding to indicate
// that there is a current operation being performed. The Status of the binding
// is recorded in the registry.
// params:
// toUpdate - a modifiable copy of the binding in the registry to update
// operation - operation that is being performed on the binding
// inProgressProperties - the new properties, if any, to apply to the binding
// returns:
// 1 - a modifiable copy of toUpdate; or toUpdate if there was an error
// 2 - any error that occurred
func (c *controller) recordStartOfServiceBindingOperation(
	toUpdate *v1beta1.ServiceBinding, operation v1beta1.ServiceBindingOperation, inProgressProperties *v1beta1.ServiceBindingPropertiesState) (
	*v1beta1.ServiceBinding, error) {

	currentReconciledGeneration := toUpdate.Status.ReconciledGeneration
	clearServiceBindingCurrentOperation(toUpdate)

	toUpdate.Status.ReconciledGeneration = currentReconciledGeneration
	toUpdate.Status.CurrentOperation = operation
	now := metav1.Now()
	toUpdate.Status.OperationStartTime = &now
	toUpdate.Status.InProgressProperties = inProgressProperties
	reason := ""
	message := ""
	switch operation {
	case v1beta1.ServiceBindingOperationBind:
		reason = bindingInFlightReason
		message = bindingInFlightMessage
		toUpdate.Status.UnbindStatus = v1beta1.ServiceBindingUnbindStatusRequired
	case v1beta1.ServiceBindingOperationUnbind:
		reason = unbindingInFlightReason
		message = unbindingInFlightMessage
	}
	setServiceBindingCondition(
		toUpdate,
		v1beta1.ServiceBindingConditionReady,
		v1beta1.ConditionFalse,
		reason,
		message,
	)
	return c.updateServiceBindingStatus(toUpdate)
}

// clearServiceBindingCurrentOperation sets the fields of the binding's
// Status to indicate that there is no current operation being performed. The
// Status is *not* recorded in the registry.
func clearServiceBindingCurrentOperation(toUpdate *v1beta1.ServiceBinding) {
	toUpdate.Status.CurrentOperation = ""
	toUpdate.Status.OperationStartTime = nil
	toUpdate.Status.AsyncOpInProgress = false
	toUpdate.Status.LastOperation = nil
	toUpdate.Status.ReconciledGeneration = toUpdate.Generation
	toUpdate.Status.InProgressProperties = nil
	toUpdate.Status.OrphanMitigationInProgress = false
}

// rollbackBindingReconciledGenerationOnDeletion resets the ReconciledGeneration
// if a deletion was performed while an async bind is running.
// TODO: rework saving off current generation as the start of the async
// operation, see PR 1708/Issue 1587.
func rollbackBindingReconciledGenerationOnDeletion(binding *v1beta1.ServiceBinding, currentReconciledGeneration int64) {
	if binding.DeletionTimestamp != nil {
		glog.V(4).Infof("Not updating ReconciledGeneration after async operation because there is a deletion pending.")
		binding.Status.ReconciledGeneration = currentReconciledGeneration
	}
}

func (c *controller) requeueServiceBindingForPoll(key string) error {
	c.bindingQueue.Add(key)

	return nil
}

// beginPollingServiceBinding does a rate-limited add of the key for the given
// binding to the controller's binding polling queue.
func (c *controller) beginPollingServiceBinding(binding *v1beta1.ServiceBinding) error {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(binding)
	if err != nil {
		glog.Errorf("Couldn't create a key for object %+v: %v", binding, err)
		return fmt.Errorf("Couldn't create a key for object %+v: %v", binding, err)
	}

	c.bindingPollingQueue.AddRateLimited(key)

	return nil
}

// continuePollingServiceBinding does a rate-limited add of the key for the
// given binding to the controller's binding polling queue.
func (c *controller) continuePollingServiceBinding(binding *v1beta1.ServiceBinding) error {
	return c.beginPollingServiceBinding(binding)
}

// finishPollingServiceBinding removes the binding's key from the controller's
// binding polling queue.
func (c *controller) finishPollingServiceBinding(binding *v1beta1.ServiceBinding) error {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(binding)
	if err != nil {
		glog.Errorf("Couldn't create a key for object %+v: %v", binding, err)
		return fmt.Errorf("Couldn't create a key for object %+v: %v", binding, err)
	}

	c.bindingPollingQueue.Forget(key)

	return nil
}

func (c *controller) pollServiceBinding(binding *v1beta1.ServiceBinding) error {
	pcb := pretty.NewContextBuilder(pretty.ServiceBinding, binding.Name, binding.Namespace)
	glog.V(4).Infof(pcb.Message("Processing"))

	binding = binding.DeepCopy()

	instance, err := c.instanceLister.ServiceInstances(binding.Namespace).Get(binding.Spec.ServiceInstanceRef.Name)
	if err != nil {
		msg := fmt.Sprintf(`References a non-existent %s "%s/%s"`, pretty.ServiceInstance, binding.Namespace, binding.Spec.ServiceInstanceRef.Name)
		readyCond := newServiceBindingReadyCondition(v1beta1.ConditionFalse, errorNonexistentServiceInstanceReason, msg)
		return c.processServiceBindingOperationError(binding, readyCond)
	}

	serviceClass, servicePlan, _, brokerClient, err := c.getClusterServiceClassPlanAndClusterServiceBrokerForServiceBinding(instance, binding)
	if err != nil {
		return c.handleServiceBindingReconciliationError(binding, err)
	}

	// There are some conditions that are different if we're
	// deleting or mitigating an orphan; this is more readable than
	// checking the timestamps in various places.
	mitigatingOrphan := binding.Status.OrphanMitigationInProgress
	deleting := binding.Status.CurrentOperation == v1beta1.ServiceBindingOperationUnbind || mitigatingOrphan

	request, err := c.prepareServiceBindingLastOperationRequest(binding, instance, serviceClass, servicePlan)
	if err != nil {
		return c.handleServiceBindingReconciliationError(binding, err)
	}

	glog.V(5).Info(pcb.Message("Polling last operation"))

	response, err := brokerClient.PollBindingLastOperation(request)
	if err != nil {
		// If the operation was for delete and we receive a http.StatusGone,
		// this is considered a success as per the spec.
		if osb.IsGoneError(err) && deleting {
			if err := c.processUnbindSuccess(binding); err != nil {
				return c.handleServiceBindingPollingError(binding, err)
			}
			return c.finishPollingServiceBinding(binding)
		}

		// We got some kind of error and should continue polling as per
		// the spec.
		//
		// The binding's Ready condition should already be False, so we
		// just need to record an event.
		s := fmt.Sprintf("Error polling last operation: %v", err)
		glog.V(4).Info(pcb.Message(s))
		c.recorder.Event(binding, corev1.EventTypeWarning, errorPollingLastOperationReason, s)

		if c.reconciliationRetryDurationExceeded(binding.Status.OperationStartTime) {
			return c.processServiceBindingPollingFailureRetryTimeout(binding, nil)
		}

		return c.continuePollingServiceBinding(binding)
	}

	glog.V(4).Info(pcb.Messagef("Poll returned %q : %q", response.State, response.Description))

	switch response.State {
	case osb.StateInProgress:
		if c.reconciliationRetryDurationExceeded(binding.Status.OperationStartTime) {
			return c.processServiceBindingPollingFailureRetryTimeout(binding, nil)
		}

		// if the description is non-nil, then update the instance condition with it
		if response.Description != nil {
			reason := asyncBindingReason
			message := asyncBindingMessage
			if deleting {
				reason = asyncUnbindingReason
				message = asyncUnbindingMessage
			}

			message = fmt.Sprintf("%s (%s)", message, *response.Description)
			setServiceBindingCondition(binding, v1beta1.ServiceBindingConditionReady, v1beta1.ConditionFalse, reason, message)
			c.recorder.Event(binding, corev1.EventTypeNormal, reason, message)

			if _, err := c.updateServiceBindingStatus(binding); err != nil {
				return err
			}
		}

		glog.V(4).Info(pcb.Message("Last operation not completed (still in progress)"))
		return c.continuePollingServiceBinding(binding)
	case osb.StateSucceeded:
		if deleting {
			if err := c.processUnbindSuccess(binding); err != nil {
				return err
			}
			return c.finishPollingServiceBinding(binding)
		}

		// Update the in progress/external properties, as the changes have been
		// persisted in the broker
		binding.Status.ExternalProperties = binding.Status.InProgressProperties

		getBindingRequest := &osb.GetBindingRequest{
			InstanceID: instance.Spec.ExternalID,
			BindingID:  binding.Spec.ExternalID,
		}

		// TODO(mkibbe): Break this logic out so that GET and inject are retried separately on error
		getBindingResponse, err := brokerClient.GetBinding(getBindingRequest)
		if err != nil {
			reason := errorFetchingBindingFailedReason
			msg := fmt.Sprintf("Could not do a GET on binding resource: %v", err)
			readyCond := newServiceBindingReadyCondition(v1beta1.ConditionFalse, reason, msg)
			failedCond := newServiceBindingFailedCondition(v1beta1.ConditionTrue, reason, msg)

			if err := c.processBindFailure(binding, readyCond, failedCond, true); err != nil {
				return err
			}

			return c.finishPollingServiceBinding(binding)
		}

		if err := c.injectServiceBinding(binding, getBindingResponse.Credentials); err != nil {
			reason := errorInjectingBindResultReason
			msg := fmt.Sprintf("Error injecting bind results: %v", err)

			readyCond := newServiceBindingReadyCondition(v1beta1.ConditionFalse, reason, msg)
			failedCond := newServiceBindingFailedCondition(v1beta1.ConditionTrue, reason, msg)

			if err := c.processBindFailure(binding, readyCond, failedCond, true); err != nil {
				return err
			}

			return c.finishPollingServiceBinding(binding)
		}

		if err := c.processBindSuccess(binding); err != nil {
			return err
		}

		return c.finishPollingServiceBinding(binding)
	case osb.StateFailed:
		description := "(no description provided)"
		if response.Description != nil {
			description = *response.Description
		}

		if !deleting {
			reason := errorBindCallReason
			message := "Bind call failed: " + description
			readyCond := newServiceBindingReadyCondition(v1beta1.ConditionFalse, reason, message)
			failedCond := newServiceBindingFailedCondition(v1beta1.ConditionTrue, reason, message)
			if err := c.processBindFailure(binding, readyCond, failedCond, false); err != nil {
				return c.handleServiceBindingPollingError(binding, err)
			}
			return c.finishPollingServiceBinding(binding)
		}

		msg := "Unbind call failed: " + description
		readyCond := newServiceBindingReadyCondition(v1beta1.ConditionUnknown, errorUnbindCallReason, msg)

		if c.reconciliationRetryDurationExceeded(binding.Status.OperationStartTime) {
			return c.processServiceBindingPollingFailureRetryTimeout(binding, readyCond)
		}

		setServiceBindingCondition(binding, v1beta1.ServiceBindingConditionReady, readyCond.Status, readyCond.Reason, readyCond.Message)
		c.recorder.Event(binding, corev1.EventTypeWarning, errorUnbindCallReason, msg)

		// we must trigger a new unbind attempt entirely (as opposed to
		// retrying querying the failed operation endpoint). Finish
		// polling, and return an error in order to requeue in the
		// standard binding queue.
		binding.Status.AsyncOpInProgress = false
		binding.Status.LastOperation = nil

		if _, err := c.updateServiceBindingStatus(binding); err != nil {
			return err
		}

		c.finishPollingServiceBinding(binding)
		return fmt.Errorf(readyCond.Message)
	default:
		glog.Warning(pcb.Messagef("Got invalid state in LastOperationResponse: %q", response.State))

		if c.reconciliationRetryDurationExceeded(binding.Status.OperationStartTime) {
			return c.processServiceBindingPollingFailureRetryTimeout(binding, nil)
		}

		return c.continuePollingServiceBinding(binding)
	}
}

// processServiceBindingPollingFailureRetryTimeout marks the binding as having
// failed polling due to its reconciliation retry duration expiring
func (c *controller) processServiceBindingPollingFailureRetryTimeout(binding *v1beta1.ServiceBinding, readyCond *v1beta1.ServiceBindingCondition) error {
	mitigatingOrphan := binding.Status.OrphanMitigationInProgress
	deleting := binding.Status.CurrentOperation == v1beta1.ServiceBindingOperationUnbind || mitigatingOrphan

	// if no specific failure provided, just say the operation timed out.
	if readyCond == nil {
		operation := "Bind"
		status := v1beta1.ConditionFalse
		if deleting {
			operation = "Unbind"
			status = v1beta1.ConditionUnknown
		}

		msg := fmt.Sprintf("The asynchronous %v operation timed out and will not be retried", operation)
		readyCond = newServiceBindingReadyCondition(status, errorAsyncOpTimeoutReason, msg)
	}

	msg := "Stopping reconciliation retries because too much time has elapsed"
	failedCond := newServiceBindingFailedCondition(v1beta1.ConditionTrue, errorReconciliationRetryTimeoutReason, msg)

	var err error
	if deleting {
		err = c.processUnbindFailure(binding, readyCond, failedCond)
	} else {
		// always finish polling binding, as triggering OM will return an error
		c.finishPollingServiceBinding(binding)
		return c.processBindFailure(binding, readyCond, failedCond, true)
	}

	if err != nil {
		return c.handleServiceBindingPollingError(binding, err)
	}

	return c.finishPollingServiceBinding(binding)
}

// newServiceBindingReadyCondition is a helper function that returns a Ready
// condition with the given status, reason, and message, with its transition
// time set to now.
func newServiceBindingReadyCondition(status v1beta1.ConditionStatus, reason, message string) *v1beta1.ServiceBindingCondition {
	return &v1beta1.ServiceBindingCondition{
		Type:               v1beta1.ServiceBindingConditionReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
}

// newServiceBindingFailedCondition is a helper function that returns a Failed
// condition with the given status, reason, and message, with its transition
// time set to now.
func newServiceBindingFailedCondition(status v1beta1.ConditionStatus, reason, message string) *v1beta1.ServiceBindingCondition {
	return &v1beta1.ServiceBindingCondition{
		Type:               v1beta1.ServiceBindingConditionFailed,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
}

// setServiceBindingLastOperation sets the last operation key on the given
// binding.
func setServiceBindingLastOperation(binding *v1beta1.ServiceBinding, operationKey *osb.OperationKey) {
	if operationKey != nil && *operationKey != "" {
		key := string(*operationKey)
		binding.Status.LastOperation = &key
	}
}

// prepareBindRequest creates a bind request object to be passed to the broker
// client to create the given binding.
func (c *controller) prepareBindRequest(
	binding *v1beta1.ServiceBinding, instance *v1beta1.ServiceInstance, serviceClass *v1beta1.ClusterServiceClass, servicePlan *v1beta1.ClusterServicePlan) (
	*osb.BindRequest, *v1beta1.ServiceBindingPropertiesState, error) {

	ns, err := c.kubeClient.CoreV1().Namespaces().Get(instance.Namespace, metav1.GetOptions{})
	if err != nil {
		return nil, nil, &operationError{
			reason:  errorFindingNamespaceServiceInstanceReason,
			message: fmt.Sprintf(`Failed to get namespace %q during binding: %s`, instance.Namespace, err),
		}
	}

	parameters, parametersChecksum, rawParametersWithRedaction, err := prepareInProgressPropertyParameters(
		c.kubeClient,
		binding.Namespace,
		binding.Spec.Parameters,
		binding.Spec.ParametersFrom,
	)
	if err != nil {
		return nil, nil, &operationError{
			reason:  errorWithParameters,
			message: err.Error(),
		}
	}

	inProgressProperties := &v1beta1.ServiceBindingPropertiesState{
		Parameters:         rawParametersWithRedaction,
		ParametersChecksum: parametersChecksum,
		UserInfo:           binding.Spec.UserInfo,
	}

	appGUID := string(ns.UID)
	request := &osb.BindRequest{
		BindingID:    binding.Spec.ExternalID,
		InstanceID:   instance.Spec.ExternalID,
		ServiceID:    serviceClass.Spec.ExternalID,
		PlanID:       servicePlan.Spec.ExternalID,
		AppGUID:      &appGUID,
		Parameters:   parameters,
		BindResource: &osb.BindResource{AppGUID: &appGUID},
	}

	// Asynchronous binding operations are currently ALPHA and not
	// enabled by default. To use this feature, you must enable the
	// AsyncBindingOperations feature gate. This may be easily set
	// by setting `asyncBindingOperationsEnabled=true` when
	// deploying the Service Catalog via the Helm charts.
	if serviceClass.Spec.BindingRetrievable &&
		utilfeature.DefaultFeatureGate.Enabled(scfeatures.AsyncBindingOperations) {

		request.AcceptsIncomplete = true
	}

	if utilfeature.DefaultFeatureGate.Enabled(scfeatures.OriginatingIdentity) {
		originatingIdentity, err := buildOriginatingIdentity(binding.Spec.UserInfo)
		if err != nil {
			return nil, nil, &operationError{
				reason:  errorWithOriginatingIdentity,
				message: fmt.Sprintf(`Error building originating identity headers for binding: %v`, err),
			}
		}
		request.OriginatingIdentity = originatingIdentity
	}

	return request, inProgressProperties, nil
}

// prepareUnbindRequest creates an unbind request object to be passed to the
// broker client to delete the given binding.
func (c *controller) prepareUnbindRequest(
	binding *v1beta1.ServiceBinding, instance *v1beta1.ServiceInstance, serviceClass *v1beta1.ClusterServiceClass, servicePlan *v1beta1.ClusterServicePlan) (
	*osb.UnbindRequest, error) {

	request := &osb.UnbindRequest{
		BindingID:  binding.Spec.ExternalID,
		InstanceID: instance.Spec.ExternalID,
		ServiceID:  serviceClass.Spec.ExternalID,
		PlanID:     servicePlan.Spec.ExternalID,
	}

	// Asynchronous binding operations is currently ALPHA and not
	// enabled by default. To use this feature, you must enable the
	// AsyncBindingOperations feature gate. This may be easily set
	// by setting `asyncBindingOperationsEnabled=true` when
	// deploying the Service Catalog via the Helm charts.
	if serviceClass.Spec.BindingRetrievable &&
		utilfeature.DefaultFeatureGate.Enabled(scfeatures.AsyncBindingOperations) {

		request.AcceptsIncomplete = true
	}

	if utilfeature.DefaultFeatureGate.Enabled(scfeatures.OriginatingIdentity) {
		originatingIdentity, err := buildOriginatingIdentity(binding.Spec.UserInfo)
		if err != nil {
			return nil, &operationError{
				reason:  errorWithOriginatingIdentity,
				message: fmt.Sprintf(`Error building originating identity headers for binding: %v`, err),
			}
		}
		request.OriginatingIdentity = originatingIdentity
	}

	return request, nil
}

// prepareServiceBindingLastOperationRequest creates a request object to be
// passed to the broker client to query the given binding's last operation
// endpoint.
func (c *controller) prepareServiceBindingLastOperationRequest(
	binding *v1beta1.ServiceBinding, instance *v1beta1.ServiceInstance, serviceClass *v1beta1.ClusterServiceClass, servicePlan *v1beta1.ClusterServicePlan) (
	*osb.BindingLastOperationRequest, error) {

	request := &osb.BindingLastOperationRequest{
		InstanceID: instance.Spec.ExternalID,
		BindingID:  binding.Spec.ExternalID,
		ServiceID:  &serviceClass.Spec.ExternalID,
		PlanID:     &servicePlan.Spec.ExternalID,
	}
	if binding.Status.LastOperation != nil && *binding.Status.LastOperation != "" {
		key := osb.OperationKey(*binding.Status.LastOperation)
		request.OperationKey = &key
	}

	if utilfeature.DefaultFeatureGate.Enabled(scfeatures.OriginatingIdentity) {
		originatingIdentity, err := buildOriginatingIdentity(binding.Spec.UserInfo)
		if err != nil {
			return nil, &operationError{
				reason:  errorWithOriginatingIdentity,
				message: fmt.Sprintf(`Error building originating identity headers for polling binding last operation: %v`, err),
			}
		}
		request.OriginatingIdentity = originatingIdentity
	}

	return request, nil
}

// processServiceBindingOperationError handles the logging and updating of a
// ServiceBinding that hit a retryable error during reconciliation.
func (c *controller) processServiceBindingOperationError(binding *v1beta1.ServiceBinding, readyCond *v1beta1.ServiceBindingCondition) error {
	c.recorder.Event(binding, corev1.EventTypeWarning, readyCond.Reason, readyCond.Message)
	setServiceBindingCondition(binding, readyCond.Type, readyCond.Status, readyCond.Reason, readyCond.Message)
	if _, err := c.updateServiceBindingStatus(binding); err != nil {
		return err
	}

	return fmt.Errorf(readyCond.Message)
}

// processBindSuccess handles the logging and updating of a ServiceBinding that
// has successfully been created at the broker and has had its credentials
// injected in the cluster.
func (c *controller) processBindSuccess(binding *v1beta1.ServiceBinding) error {
	setServiceBindingCondition(binding, v1beta1.ServiceBindingConditionReady, v1beta1.ConditionTrue, successInjectedBindResultReason, successInjectedBindResultMessage)
	currentReconciledGeneration := binding.Status.ReconciledGeneration
	clearServiceBindingCurrentOperation(binding)
	rollbackBindingReconciledGenerationOnDeletion(binding, currentReconciledGeneration)

	if _, err := c.updateServiceBindingStatus(binding); err != nil {
		return err
	}

	c.recorder.Event(binding, corev1.EventTypeNormal, successInjectedBindResultReason, successInjectedBindResultMessage)
	return nil
}

// processBindFailure handles the logging and updating of a ServiceBinding that
// hit a terminal failure during bind reconciliation.
func (c *controller) processBindFailure(binding *v1beta1.ServiceBinding, readyCond, failedCond *v1beta1.ServiceBindingCondition, shouldMitigateOrphan bool) error {
	currentReconciledGeneration := binding.Status.ReconciledGeneration
	if readyCond != nil {
		c.recorder.Event(binding, corev1.EventTypeWarning, readyCond.Reason, readyCond.Message)
		setServiceBindingCondition(binding, readyCond.Type, readyCond.Status, readyCond.Reason, readyCond.Message)
	}

	c.recorder.Event(binding, corev1.EventTypeWarning, failedCond.Reason, failedCond.Message)
	setServiceBindingCondition(binding, failedCond.Type, failedCond.Status, failedCond.Reason, failedCond.Message)

	if shouldMitigateOrphan {
		msg := "Starting orphan mitigation"
		readyCond := newServiceBindingReadyCondition(v1beta1.ConditionFalse, errorServiceBindingOrphanMitigation, msg)
		setServiceBindingCondition(binding, readyCond.Type, readyCond.Status, readyCond.Reason, readyCond.Message)
		c.recorder.Event(binding, corev1.EventTypeWarning, readyCond.Reason, readyCond.Message)

		binding.Status.OrphanMitigationInProgress = true
		binding.Status.AsyncOpInProgress = false
		binding.Status.OperationStartTime = nil
	} else {
		clearServiceBindingCurrentOperation(binding)
		rollbackBindingReconciledGenerationOnDeletion(binding, currentReconciledGeneration)
	}

	if _, err := c.updateServiceBindingStatus(binding); err != nil {
		return err
	}

	return nil
}

// processBindAsyncResponse handles the logging and updating of a
// ServiceInstance that received an asynchronous response from the broker when
// requesting a bind.
func (c *controller) processBindAsyncResponse(binding *v1beta1.ServiceBinding, response *osb.BindResponse) error {
	setServiceBindingLastOperation(binding, response.OperationKey)
	setServiceBindingCondition(binding, v1beta1.ServiceBindingConditionReady, v1beta1.ConditionFalse, asyncBindingReason, asyncBindingMessage)
	binding.Status.AsyncOpInProgress = true

	if _, err := c.updateServiceBindingStatus(binding); err != nil {
		return err
	}

	c.recorder.Event(binding, corev1.EventTypeNormal, asyncBindingReason, asyncBindingMessage)
	return c.beginPollingServiceBinding(binding)
}

// handleServiceBindingReconciliationError is a helper function that handles on
// error whether the error represents an operation error and should update the
// ServiceBinding resource.
func (c *controller) handleServiceBindingReconciliationError(binding *v1beta1.ServiceBinding, err error) error {
	if resourceErr, ok := err.(*operationError); ok {
		readyCond := newServiceBindingReadyCondition(v1beta1.ConditionFalse, resourceErr.reason, resourceErr.message)
		return c.processServiceBindingOperationError(binding, readyCond)
	}
	return err
}

// processServiceBindingGracefulDeletionSuccess handles the logging and
// updating of a ServiceBinding that has successfully finished graceful
// deletion.
func (c *controller) processServiceBindingGracefulDeletionSuccess(binding *v1beta1.ServiceBinding) error {
	finalizers := sets.NewString(binding.Finalizers...)
	finalizers.Delete(v1beta1.FinalizerServiceCatalog)
	binding.Finalizers = finalizers.List()

	if _, err := c.updateServiceBindingStatus(binding); err != nil {
		return err
	}

	pcb := pretty.NewContextBuilder(pretty.ServiceBinding, binding.Namespace, binding.Name)
	glog.Info(pcb.Message("Cleared finalizer"))

	return nil
}

// processUnbindSuccess handles the logging and updating of a ServiceBinding
// that has successfully been deleted at the broker.
func (c *controller) processUnbindSuccess(binding *v1beta1.ServiceBinding) error {
	mitigatingOrphan := binding.Status.OrphanMitigationInProgress

	reason := successUnboundReason
	msg := "The binding was deleted successfully"
	if mitigatingOrphan {
		reason = successOrphanMitigationReason
		msg = successOrphanMitigationMessage
	}

	setServiceBindingCondition(binding, v1beta1.ServiceBindingConditionReady, v1beta1.ConditionFalse, reason, msg)
	clearServiceBindingCurrentOperation(binding)
	binding.Status.ExternalProperties = nil
	binding.Status.UnbindStatus = v1beta1.ServiceBindingUnbindStatusSucceeded

	if mitigatingOrphan {
		if _, err := c.updateServiceBindingStatus(binding); err != nil {
			return err
		}
	} else {
		// If part of a resource deletion request, follow-through to
		// the graceful deletion handler in order to clear the finalizer.
		if err := c.processServiceBindingGracefulDeletionSuccess(binding); err != nil {
			return err
		}
	}

	c.recorder.Event(binding, corev1.EventTypeNormal, reason, msg)
	return nil
}

// processUnbindFailure handles the logging and updating of a
// ServiceBinding that hit a terminal failure during unbind
// reconciliation.
func (c *controller) processUnbindFailure(binding *v1beta1.ServiceBinding, readyCond, failedCond *v1beta1.ServiceBindingCondition) error {
	if failedCond == nil {
		return fmt.Errorf("failedCond must not be nil")
	}

	if readyCond != nil {
		setServiceBindingCondition(binding, v1beta1.ServiceBindingConditionReady, v1beta1.ConditionUnknown, readyCond.Reason, readyCond.Message)
		c.recorder.Event(binding, corev1.EventTypeWarning, readyCond.Reason, readyCond.Message)
	}

	if binding.Status.OrphanMitigationInProgress {
		// replace Ready condition with orphan mitigation-related one.
		msg := "Orphan mitigation failed: " + failedCond.Message
		readyCond := newServiceBindingReadyCondition(v1beta1.ConditionUnknown, errorOrphanMitigationFailedReason, msg)
		setServiceBindingCondition(binding, v1beta1.ServiceBindingConditionReady, readyCond.Status, readyCond.Reason, readyCond.Message)
		c.recorder.Event(binding, corev1.EventTypeWarning, readyCond.Reason, readyCond.Message)
	} else {
		setServiceBindingCondition(binding, v1beta1.ServiceBindingConditionFailed, failedCond.Status, failedCond.Reason, failedCond.Message)
		c.recorder.Event(binding, corev1.EventTypeWarning, failedCond.Reason, failedCond.Message)
	}

	clearServiceBindingCurrentOperation(binding)
	binding.Status.UnbindStatus = v1beta1.ServiceBindingUnbindStatusFailed

	if _, err := c.updateServiceBindingStatus(binding); err != nil {
		return err
	}

	return nil
}

// processUnbindAsyncResponse handles the logging and updating of a
// ServiceBinding that received an asynchronous response from the broker when
// requesting an unbind.
func (c *controller) processUnbindAsyncResponse(binding *v1beta1.ServiceBinding, response *osb.UnbindResponse) error {
	setServiceBindingLastOperation(binding, response.OperationKey)
	setServiceBindingCondition(binding, v1beta1.ServiceBindingConditionReady, v1beta1.ConditionFalse, asyncUnbindingReason, asyncUnbindingMessage)
	binding.Status.AsyncOpInProgress = true

	if _, err := c.updateServiceBindingStatus(binding); err != nil {
		return err
	}

	c.recorder.Event(binding, corev1.EventTypeNormal, asyncUnbindingReason, asyncUnbindingMessage)
	return c.beginPollingServiceBinding(binding)
}

// handleServiceBindingPollingError is a helper function that handles logic for
// an error returned during reconciliation while polling a service binding.
func (c *controller) handleServiceBindingPollingError(binding *v1beta1.ServiceBinding, err error) error {
	// During polling, an error means we should:
	//	1) log the error
	//	2) attempt to requeue in the polling queue
	//		- if successful, we can return nil to avoid regular queue
	//		- if failure, return err to fall back to regular queue
	pcb := pretty.NewContextBuilder(pretty.ServiceBinding, binding.Namespace, binding.Name)
	glog.V(4).Info(pcb.Messagef("Error during polling: %v", err))
	return c.continuePollingServiceBinding(binding)
}

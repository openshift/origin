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

	"github.com/golang/glog"
	osb "github.com/pmorie/go-open-service-broker-client/v2"

	checksum "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/checksum/versioned/v1alpha1"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/cache"
)

// bindingControllerKind contains the schema.GroupVersionKind for this controller type.
var bindingControllerKind = v1alpha1.SchemeGroupVersion.WithKind("Binding")

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
	if apierrors.IsNotFound(err) {
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

func makeBindingClone(binding *v1alpha1.Binding) (*v1alpha1.Binding, error) {
	clone, err := api.Scheme.DeepCopy(binding)
	if err != nil {
		return nil, err
	}
	return clone.(*v1alpha1.Binding), nil
}

func isBindingFailed(binding *v1alpha1.Binding) bool {
	for _, condition := range binding.Status.Conditions {
		if condition.Type == v1alpha1.BindingConditionFailed && condition.Status == v1alpha1.ConditionTrue {
			return true
		}
	}
	return false
}

// an error is returned to indicate that the binding has not been
// fully processed and should be resubmitted at a later time.
func (c *controller) reconcileBinding(binding *v1alpha1.Binding) error {
	// TODO: this will change once we fully implement orphan mitigation, see:
	// https://github.com/kubernetes-incubator/service-catalog/issues/988
	if isBindingFailed(binding) && binding.ObjectMeta.DeletionTimestamp == nil {
		glog.V(4).Infof(
			"Not processing event for Binding %v/%v because status showed that it has failed",
			binding.Namespace,
			binding.Name,
		)
		return nil
	}

	toUpdate, err := makeBindingClone(binding)
	if err != nil {
		return err
	}

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
			glog.V(4).Infof(
				"Not processing event for Binding %v/%v because checksum showed there is no work to do",
				binding.Namespace,
				binding.Name,
			)
			return nil
		}
	}

	glog.V(4).Infof("Processing Binding %v/%v", binding.Namespace, binding.Name)

	instance, err := c.instanceLister.Instances(binding.Namespace).Get(binding.Spec.InstanceRef.Name)
	if err != nil {
		s := fmt.Sprintf("Binding \"%s/%s\" references a non-existent Instance \"%s/%s\"", binding.Namespace, binding.Name, binding.Namespace, binding.Spec.InstanceRef.Name)
		glog.Warningf(
			"Binding %s/%s references a non-existent instance %s/%s (%s)",
			binding.Namespace,
			binding.Name,
			binding.Namespace,
			binding.Spec.InstanceRef.Name,
			err,
		)
		c.setBindingCondition(
			toUpdate,
			v1alpha1.BindingConditionReady,
			v1alpha1.ConditionFalse,
			errorNonexistentInstanceReason,
			"The binding references an Instance that does not exist. "+s,
		)
		c.updateBindingStatus(toUpdate)
		c.recorder.Event(binding, api.EventTypeWarning, errorNonexistentInstanceReason, s)
		return err
	}

	if instance.Status.AsyncOpInProgress {
		s := fmt.Sprintf(
			"Binding \"%s/%s\" trying to bind to Instance \"%s/%s\" that has ongoing asynchronous operation",
			binding.Namespace,
			binding.Name,
			binding.Namespace,
			binding.Spec.InstanceRef.Name,
		)
		glog.Info(s)
		c.setBindingCondition(
			toUpdate,
			v1alpha1.BindingConditionReady,
			v1alpha1.ConditionFalse,
			errorWithOngoingAsyncOperation,
			errorWithOngoingAsyncOperationMessage,
		)
		c.updateBindingStatus(toUpdate)
		c.recorder.Event(binding, api.EventTypeWarning, errorWithOngoingAsyncOperation, s)
		return fmt.Errorf("Ongoing Asynchronous operation")
	}

	serviceClass, servicePlan, brokerName, brokerClient, err := c.getServiceClassPlanAndBrokerForBinding(instance, binding)
	if err != nil {
		return err
	}

	if !isPlanBindable(serviceClass, servicePlan) {
		s := fmt.Sprintf(
			"Binding \"%s/%s\" references a non-bindable ServiceClass (%q) and Plan (%q) combination",
			binding.Namespace,
			binding.Name,
			instance.Spec.ServiceClassName,
			instance.Spec.PlanName,
		)
		glog.Warning(s)
		c.setBindingCondition(
			toUpdate,
			v1alpha1.BindingConditionReady,
			v1alpha1.ConditionFalse,
			errorNonbindableServiceClassReason,
			s,
		)
		c.updateBindingStatus(toUpdate)
		c.recorder.Event(binding, api.EventTypeWarning, errorNonbindableServiceClassReason, s)
		return nil
	}

	if binding.DeletionTimestamp == nil { // Add or update
		glog.V(4).Infof("Adding/Updating Binding %v/%v", binding.Namespace, binding.Name)

		var parameters map[string]interface{}
		if binding.Spec.Parameters != nil || binding.Spec.ParametersFrom != nil {
			parameters, err = buildParameters(c.kubeClient, binding.Namespace, binding.Spec.ParametersFrom, binding.Spec.Parameters)
			if err != nil {
				s := fmt.Sprintf("Failed to prepare Binding parameters\n%s\n %s", binding.Spec.Parameters, err)
				glog.Warning(s)
				c.setBindingCondition(
					toUpdate,
					v1alpha1.BindingConditionReady,
					v1alpha1.ConditionFalse,
					errorWithParameters,
					s,
				)
				c.updateBindingStatus(toUpdate)
				c.recorder.Event(binding, api.EventTypeWarning, errorWithParameters, s)
				return err
			}
		}

		ns, err := c.kubeClient.Core().Namespaces().Get(instance.Namespace, metav1.GetOptions{})
		if err != nil {
			s := fmt.Sprintf("Failed to get namespace %q during binding: %s", instance.Namespace, err)
			glog.Info(s)
			c.setBindingCondition(
				toUpdate,
				v1alpha1.BindingConditionReady,
				v1alpha1.ConditionFalse,
				errorFindingNamespaceInstanceReason,
				"Error finding namespace for instance. "+s,
			)
			c.updateBindingStatus(toUpdate)
			c.recorder.Eventf(binding, api.EventTypeWarning, errorFindingNamespaceInstanceReason, s)
			return err
		}

		if !isInstanceReady(instance) {
			s := fmt.Sprintf(`Binding cannot begin because referenced instance "%v/%v" is not ready`, instance.Namespace, instance.Name)
			glog.Info(s)
			c.setBindingCondition(
				toUpdate,
				v1alpha1.BindingConditionReady,
				v1alpha1.ConditionFalse,
				errorInstanceNotReadyReason,
				s,
			)
			c.updateBindingStatus(toUpdate)
			c.recorder.Eventf(binding, api.EventTypeWarning, errorInstanceNotReadyReason, s)
			return nil
		}

		appGUID := string(ns.UID)
		request := &osb.BindRequest{
			BindingID:    binding.Spec.ExternalID,
			InstanceID:   instance.Spec.ExternalID,
			ServiceID:    serviceClass.ExternalID,
			PlanID:       servicePlan.ExternalID,
			AppGUID:      &appGUID,
			Parameters:   parameters,
			BindResource: &osb.BindResource{AppGUID: &appGUID},
		}

		response, err := brokerClient.Bind(request)
		if err != nil {
			httpErr, isError := osb.IsHTTPError(err)
			if isError {
				s := fmt.Sprintf("Error creating Binding \"%s/%s\" for Instance \"%s/%s\" of ServiceClass %q at Broker %q, %v",
					binding.Name,
					binding.Namespace,
					instance.Namespace,
					instance.Name,
					serviceClass.Name,
					brokerName,
					httpErr.Error(),
				)
				glog.Warning(s)

				c.setBindingCondition(
					toUpdate,
					v1alpha1.BindingConditionFailed,
					v1alpha1.ConditionTrue,
					"BindingReturnedFailure",
					s,
				)
				c.setBindingCondition(
					toUpdate,
					v1alpha1.BindingConditionReady,
					v1alpha1.ConditionFalse,
					errorBindCallReason,
					"Bind call failed. "+s)
				c.updateBindingStatus(toUpdate)
				c.recorder.Event(binding, api.EventTypeWarning, errorBindCallReason, s)
				return err
			}

			s := fmt.Sprintf("Error creating Binding \"%s/%s\" for Instance \"%s/%s\" of ServiceClass %q at Broker %q: %s", binding.Name, binding.Namespace, instance.Namespace, instance.Name, serviceClass.Name, brokerName, err)
			glog.Warning(s)
			c.setBindingCondition(
				toUpdate,
				v1alpha1.BindingConditionReady,
				v1alpha1.ConditionFalse,
				errorBindCallReason,
				"Bind call failed. "+s)
			c.updateBindingStatus(toUpdate)
			c.recorder.Event(binding, api.EventTypeWarning, errorBindCallReason, s)
			return err
		}

		err = c.injectBinding(binding, response.Credentials)
		if err != nil {
			s := fmt.Sprintf("Error injecting binding results for Binding \"%s/%s\": %s", binding.Namespace, binding.Name, err)
			glog.Warning(s)
			c.setBindingCondition(
				toUpdate,
				v1alpha1.BindingConditionReady,
				v1alpha1.ConditionFalse,
				errorInjectingBindResultReason,
				"Error injecting bind result "+s,
			)
			c.updateBindingStatus(toUpdate)
			c.recorder.Event(binding, api.EventTypeWarning, errorInjectingBindResultReason, s)
			return err
		}
		c.setBindingCondition(
			toUpdate,
			v1alpha1.BindingConditionReady,
			v1alpha1.ConditionTrue,
			successInjectedBindResultReason,
			successInjectedBindResultMessage,
		)
		c.updateBindingStatus(toUpdate)
		c.recorder.Event(binding, api.EventTypeNormal, successInjectedBindResultReason, successInjectedBindResultMessage)

		glog.V(5).Infof("Successfully bound to Instance %v/%v of ServiceClass %v at Broker %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName)

		return nil
	}

	// All updates not having a DeletingTimestamp will have been handled above
	// and returned early. If we reach this point, we're dealing with an update
	// that's actually a soft delete-- i.e. we have some finalization to do.
	if finalizers := sets.NewString(binding.Finalizers...); finalizers.Has(v1alpha1.FinalizerServiceCatalog) {
		glog.V(4).Infof("Finalizing Binding %v/%v", binding.Namespace, binding.Name)
		err = c.ejectBinding(binding)
		if err != nil {
			s := fmt.Sprintf("Error deleting secret: %s", err)
			glog.Warning(s)
			c.setBindingCondition(
				toUpdate,
				v1alpha1.BindingConditionReady,
				v1alpha1.ConditionUnknown,
				errorEjectingBindReason,
				errorEjectingBindMessage+s,
			)
			c.updateBindingStatus(toUpdate)
			c.recorder.Eventf(binding, api.EventTypeWarning, errorEjectingBindReason, "%v %v", errorEjectingBindMessage, s)
			return err
		}

		unbindRequest := &osb.UnbindRequest{
			BindingID:  binding.Spec.ExternalID,
			InstanceID: instance.Spec.ExternalID,
			ServiceID:  serviceClass.ExternalID,
			PlanID:     servicePlan.ExternalID,
		}
		_, err = brokerClient.Unbind(unbindRequest)
		if err != nil {
			httpErr, isError := osb.IsHTTPError(err)
			if isError {
				s := fmt.Sprintf("Error creating Unbinding \"%s/%s\" for Instance \"%s/%s\" of ServiceClass %q at Broker %q, %v",
					binding.Name,
					binding.Namespace,
					instance.Namespace,
					instance.Name,
					serviceClass.Name,
					brokerName,
					httpErr.Error(),
				)
				glog.Warning(s)
				c.setBindingCondition(
					toUpdate,
					v1alpha1.BindingConditionReady,
					v1alpha1.ConditionFalse,
					errorUnbindCallReason,
					"Unbind call failed. "+s)
				c.updateBindingStatus(toUpdate)
				c.recorder.Event(binding, api.EventTypeWarning, errorUnbindCallReason, s)
				return err
			}
			s := fmt.Sprintf(
				"Error unbinding Binding \"%s/%s\" for Instance \"%s/%s\" of ServiceClass %q at Broker %q: %s",
				binding.Name,
				binding.Namespace,
				instance.Namespace,
				instance.Name,
				serviceClass.Name,
				brokerName,
				err,
			)
			glog.Warning(s)
			c.setBindingCondition(
				toUpdate,
				v1alpha1.BindingConditionReady,
				v1alpha1.ConditionFalse,
				errorUnbindCallReason,
				"Unbind call failed. "+s)
			c.updateBindingStatus(toUpdate)
			c.recorder.Event(binding, api.EventTypeWarning, errorUnbindCallReason, s)
			return err
		}

		c.setBindingCondition(
			toUpdate,
			v1alpha1.BindingConditionReady,
			v1alpha1.ConditionFalse,
			successUnboundReason,
			"The binding was deleted successfully",
		)
		c.updateBindingStatus(toUpdate)
		// Clear the finalizer
		finalizers.Delete(v1alpha1.FinalizerServiceCatalog)
		if err = c.updateBindingFinalizers(binding, finalizers.List()); err != nil {
			return err
		}
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

func (c *controller) injectBinding(binding *v1alpha1.Binding, credentials map[string]interface{}) error {
	glog.V(5).Infof("Creating/updating Secret %v/%v", binding.Namespace, binding.Spec.SecretName)

	secretData := make(map[string][]byte)
	for k, v := range credentials {
		var err error
		secretData[k], err = serialize(v)
		if err != nil {
			// Terminal error
			// TODO mark as terminal error once we have the terminal condition
			return fmt.Errorf("Unable to serialize credential value %q: %v; %s",
				k, v, err)
		}
	}

	// Creating/updating the Secret
	secretClient := c.kubeClient.Core().Secrets(binding.Namespace)
	existingSecret, err := secretClient.Get(binding.Spec.SecretName, metav1.GetOptions{})
	if err == nil {
		// Update existing secret
		if !IsControlledBy(existingSecret, binding) {
			controllerRef := GetControllerOf(existingSecret)
			// TODO mark as terminal error once we have the terminal condition
			return fmt.Errorf("Secret '%s' is not owned by Binding, controllerRef: %v",
				existingSecret.Name, controllerRef)
		}
		existingSecret.Data = secretData
		_, err = secretClient.Update(existingSecret)
		if err != nil {
			if apierrors.IsConflict(err) {
				// Conflicting update detected, try again later
				return fmt.Errorf("Conflicting Secret '%s' update detected", existingSecret.Name)
			}
			// Terminal error
			// TODO mark as terminal error once we have the terminal condition
			return fmt.Errorf("Unexpected error in response: %v", err)
		}
	} else {
		if !apierrors.IsNotFound(err) {
			// Terminal error
			// TODO mark as terminal error once we have the terminal condition
			return fmt.Errorf("Unexpected error in response: %v", err)
		}
		err = nil

		// Create new secret
		secret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      binding.Spec.SecretName,
				Namespace: binding.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					*NewControllerRef(binding, bindingControllerKind),
				},
			},
			Data: secretData,
		}
		_, err = secretClient.Create(secret)
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				// Concurrent controller has created secret under the same name,
				// Update the secret at the next retry iteration
				return fmt.Errorf("Conflicting Secret '%s' creation detected", secret.Name)
			}
			// Terminal error
			// TODO mark as terminal error once we have the terminal condition
			return fmt.Errorf("Unexpected error in response: %v", err)
		}
	}

	return err
}

func (c *controller) ejectBinding(binding *v1alpha1.Binding) error {
	var err error

	glog.V(5).Infof("Deleting Secret %v/%v", binding.Namespace, binding.Spec.SecretName)
	err = c.kubeClient.Core().Secrets(binding.Namespace).Delete(binding.Spec.SecretName, &metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (c *controller) setBindingCondition(toUpdate *v1alpha1.Binding,
	conditionType v1alpha1.BindingConditionType,
	status v1alpha1.ConditionStatus,
	reason, message string) {

	setBindingConditionInternal(toUpdate, conditionType, status, reason, message, metav1.Now())
}

func setBindingConditionInternal(toUpdate *v1alpha1.Binding,
	conditionType v1alpha1.BindingConditionType,
	status v1alpha1.ConditionStatus,
	reason, message string,
	t metav1.Time) {

	glog.V(5).Infof("Setting Binding '%v/%v' condition %q to %v", toUpdate.Namespace, toUpdate.Name, conditionType, status)

	newCondition := v1alpha1.BindingCondition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	}

	if len(toUpdate.Status.Conditions) == 0 {
		glog.Infof(`Setting lastTransitionTime for Binding "%v/%v" condition %q to %v`,
			toUpdate.Namespace, toUpdate.Name, conditionType, t)
		newCondition.LastTransitionTime = t
		toUpdate.Status.Conditions = []v1alpha1.BindingCondition{newCondition}
		return
	}
	for i, cond := range toUpdate.Status.Conditions {
		if cond.Type == conditionType {
			if cond.Status != newCondition.Status {
				glog.V(3).Infof(`Found status change for Binding "%v/%v" condition %q: %q -> %q; setting lastTransitionTime to %v`,
					toUpdate.Namespace, toUpdate.Name, conditionType, cond.Status, status, t)
				newCondition.LastTransitionTime = t
			} else {
				newCondition.LastTransitionTime = cond.LastTransitionTime
			}

			toUpdate.Status.Conditions[i] = newCondition
			return
		}
	}

	glog.V(3).Infof("Setting lastTransitionTime for Binding '%v/%v' condition %q to %v",
		toUpdate.Namespace, toUpdate.Name, conditionType, t)

	newCondition.LastTransitionTime = t
	toUpdate.Status.Conditions = append(toUpdate.Status.Conditions, newCondition)
}

func (c *controller) updateBindingStatus(toUpdate *v1alpha1.Binding) error {
	glog.V(4).Infof("Updating status for Binding %v/%v", toUpdate.Namespace, toUpdate.Name)
	_, err := c.serviceCatalogClient.Bindings(toUpdate.Namespace).UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf("Error updating status for Binding %v/%v", toUpdate.Namespace, toUpdate.Name)
	}
	return err
}

// updateBindingCondition updates the given condition for the given Binding
// with the given status, reason, and message.
func (c *controller) updateBindingCondition(
	binding *v1alpha1.Binding,
	conditionType v1alpha1.BindingConditionType,
	status v1alpha1.ConditionStatus,
	reason, message string) error {

	toUpdate, err := makeBindingClone(binding)
	if err != nil {
		return err
	}

	c.setBindingCondition(toUpdate, conditionType, status, reason, message)

	glog.V(4).Infof("Updating %v condition for Binding %v/%v to %v (Reason: %q, Message: %q)",
		conditionType, binding.Namespace, binding.Name, status, reason, message)
	_, err = c.serviceCatalogClient.Bindings(binding.Namespace).UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf("Error updating %v condition for Binding %v/%v to %v: %v", conditionType, binding.Namespace, binding.Name, status, err)
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

	glog.V(4).Infof("Received delete event for Binding %v/%v; no further processing will occur", binding.Namespace, binding.Name)
}

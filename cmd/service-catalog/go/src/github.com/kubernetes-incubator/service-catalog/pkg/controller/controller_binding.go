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
	"k8s.io/apimachinery/pkg/runtime"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"github.com/kubernetes-incubator/service-catalog/pkg/api"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	scfeatures "github.com/kubernetes-incubator/service-catalog/pkg/features"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	apiv1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/cache"
)

// bindingControllerKind contains the schema.GroupVersionKind for this controller type.
var bindingControllerKind = v1alpha1.SchemeGroupVersion.WithKind("ServiceInstanceCredential")

// ServiceInstanceCredential handlers and control-loop

func (c *controller) bindingAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.bindingQueue.Add(key)
}

func (c *controller) reconcileServiceInstanceCredentialKey(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	binding, err := c.bindingLister.ServiceInstanceCredentials(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		glog.Infof("Not doing work for ServiceInstanceCredential %v because it has been deleted", key)
		return nil
	}
	if err != nil {
		glog.Infof("Unable to retrieve ServiceInstanceCredential %v from store: %v", key, err)
		return err
	}

	return c.reconcileServiceInstanceCredential(binding)
}

func (c *controller) bindingUpdate(oldObj, newObj interface{}) {
	c.bindingAdd(newObj)
}

func makeServiceInstanceCredentialClone(binding *v1alpha1.ServiceInstanceCredential) (*v1alpha1.ServiceInstanceCredential, error) {
	clone, err := api.Scheme.DeepCopy(binding)
	if err != nil {
		return nil, err
	}
	return clone.(*v1alpha1.ServiceInstanceCredential), nil
}

func isServiceInstanceCredentialFailed(binding *v1alpha1.ServiceInstanceCredential) bool {
	for _, condition := range binding.Status.Conditions {
		if condition.Type == v1alpha1.ServiceInstanceCredentialConditionFailed && condition.Status == v1alpha1.ConditionTrue {
			return true
		}
	}
	return false
}

// an error is returned to indicate that the binding has not been
// fully processed and should be resubmitted at a later time.
func (c *controller) reconcileServiceInstanceCredential(binding *v1alpha1.ServiceInstanceCredential) error {
	// TODO: this will change once we fully implement orphan mitigation, see:
	// https://github.com/kubernetes-incubator/service-catalog/issues/988
	if isServiceInstanceCredentialFailed(binding) && binding.ObjectMeta.DeletionTimestamp == nil {
		glog.V(4).Infof(
			"Not processing event for ServiceInstanceCredential %v/%v because status showed that it has failed",
			binding.Namespace,
			binding.Name,
		)
		return nil
	}

	toUpdate, err := makeServiceInstanceCredentialClone(binding)
	if err != nil {
		return err
	}

	// Determine whether there is a new generation of the object. If the binding's
	// generation does not match the reconciled generation, then there is a new
	// generation, indicating that changes have been made to the binding's spec.
	//
	// We only do this if the deletion timestamp is nil, because the deletion
	// timestamp changes the object's state in a way that we must reconcile,
	// but does not affect the generation.
	if binding.DeletionTimestamp == nil {
		if binding.Status.ReconciledGeneration == binding.Generation {
			glog.V(4).Infof(
				"Not processing event for ServiceInstanceCredential %v/%v because reconciled generation showed there is no work to do",
				binding.Namespace,
				binding.Name,
			)
			return nil
		}
	}

	glog.V(4).Infof("Processing ServiceInstanceCredential %v/%v", binding.Namespace, binding.Name)

	instance, err := c.instanceLister.ServiceInstances(binding.Namespace).Get(binding.Spec.ServiceInstanceRef.Name)
	if err != nil {
		s := fmt.Sprintf("ServiceInstanceCredential \"%s/%s\" references a non-existent ServiceInstance \"%s/%s\"", binding.Namespace, binding.Name, binding.Namespace, binding.Spec.ServiceInstanceRef.Name)
		glog.Warningf(
			"ServiceInstanceCredential %s/%s references a non-existent instance %s/%s (%s)",
			binding.Namespace,
			binding.Name,
			binding.Namespace,
			binding.Spec.ServiceInstanceRef.Name,
			err,
		)
		c.recorder.Event(binding, apiv1.EventTypeWarning, errorNonexistentServiceInstanceReason, s)
		c.setServiceInstanceCredentialCondition(
			toUpdate,
			v1alpha1.ServiceInstanceCredentialConditionReady,
			v1alpha1.ConditionFalse,
			errorNonexistentServiceInstanceReason,
			"The binding references an ServiceInstance that does not exist. "+s,
		)
		if _, err := c.updateServiceInstanceCredentialStatus(toUpdate); err != nil {
			return err
		}
		return err
	}

	if instance.Status.AsyncOpInProgress {
		s := fmt.Sprintf(
			"ServiceInstanceCredential \"%s/%s\" trying to bind to ServiceInstance \"%s/%s\" that has ongoing asynchronous operation",
			binding.Namespace,
			binding.Name,
			binding.Namespace,
			binding.Spec.ServiceInstanceRef.Name,
		)
		glog.Info(s)
		c.recorder.Event(binding, apiv1.EventTypeWarning, errorWithOngoingAsyncOperation, s)
		c.setServiceInstanceCredentialCondition(
			toUpdate,
			v1alpha1.ServiceInstanceCredentialConditionReady,
			v1alpha1.ConditionFalse,
			errorWithOngoingAsyncOperation,
			errorWithOngoingAsyncOperationMessage,
		)
		if _, err := c.updateServiceInstanceCredentialStatus(toUpdate); err != nil {
			return err
		}
		return fmt.Errorf("Ongoing Asynchronous operation")
	}

	if instance.Spec.ServiceClassRef == nil || instance.Spec.ServicePlanRef == nil {
		// retry later
		return fmt.Errorf("ServiceClass or ServicePlan references for Instance have not been resolved yet")
	}

	serviceClass, servicePlan, brokerName, brokerClient, err := c.getServiceClassPlanAndServiceBrokerForServiceInstanceCredential(instance, binding)
	if err != nil {
		return err // retry later
	}

	if !isPlanBindable(serviceClass, servicePlan) {
		s := fmt.Sprintf(
			"ServiceInstanceCredential \"%s/%s\" references a non-bindable ServiceClass (%q) and Plan (%q) combination",
			binding.Namespace,
			binding.Name,
			instance.Spec.ExternalServiceClassName,
			instance.Spec.ExternalServicePlanName,
		)
		glog.Warning(s)
		c.recorder.Event(binding, apiv1.EventTypeWarning, errorNonbindableServiceClassReason, s)
		c.setServiceInstanceCredentialCondition(
			toUpdate,
			v1alpha1.ServiceInstanceCredentialConditionReady,
			v1alpha1.ConditionFalse,
			errorNonbindableServiceClassReason,
			s,
		)
		if _, err := c.updateServiceInstanceCredentialStatus(toUpdate); err != nil {
			return err
		}
		return nil
	}

	if binding.DeletionTimestamp == nil { // Add or update
		glog.V(4).Infof("Adding/Updating ServiceInstanceCredential %v/%v", binding.Namespace, binding.Name)

		ns, err := c.kubeClient.Core().Namespaces().Get(instance.Namespace, metav1.GetOptions{})
		if err != nil {
			s := fmt.Sprintf("Failed to get namespace %q during binding: %s", instance.Namespace, err)
			glog.Info(s)
			c.recorder.Eventf(binding, apiv1.EventTypeWarning, errorFindingNamespaceServiceInstanceReason, s)
			c.setServiceInstanceCredentialCondition(
				toUpdate,
				v1alpha1.ServiceInstanceCredentialConditionReady,
				v1alpha1.ConditionFalse,
				errorFindingNamespaceServiceInstanceReason,
				"Error finding namespace for instance. "+s,
			)
			if _, err := c.updateServiceInstanceCredentialStatus(toUpdate); err != nil {
				return err
			}
			return err
		}

		if !isServiceInstanceReady(instance) {
			s := fmt.Sprintf(`ServiceInstanceCredential cannot begin because referenced instance "%v/%v" is not ready`, instance.Namespace, instance.Name)
			glog.Info(s)
			c.recorder.Eventf(binding, apiv1.EventTypeWarning, errorServiceInstanceNotReadyReason, s)
			c.setServiceInstanceCredentialCondition(
				toUpdate,
				v1alpha1.ServiceInstanceCredentialConditionReady,
				v1alpha1.ConditionFalse,
				errorServiceInstanceNotReadyReason,
				s,
			)
			if _, err := c.updateServiceInstanceCredentialStatus(toUpdate); err != nil {
				return err
			}
			return nil
		}

		var (
			parameters                 map[string]interface{}
			parametersChecksum         string
			rawParametersWithRedaction *runtime.RawExtension
		)
		if binding.Spec.Parameters != nil || binding.Spec.ParametersFrom != nil {
			var parametersWithSecretsRedacted map[string]interface{}
			parameters, parametersWithSecretsRedacted, err = buildParameters(c.kubeClient, binding.Namespace, binding.Spec.ParametersFrom, binding.Spec.Parameters)
			if err != nil {
				s := fmt.Sprintf("Failed to prepare ServiceInstanceCredential parameters\n%s\n %s", binding.Spec.Parameters, err)
				glog.Warning(s)
				c.recorder.Event(binding, apiv1.EventTypeWarning, errorWithParameters, s)
				c.setServiceInstanceCredentialCondition(
					toUpdate,
					v1alpha1.ServiceInstanceCredentialConditionReady,
					v1alpha1.ConditionFalse,
					errorWithParameters,
					s,
				)
				if _, err := c.updateServiceInstanceCredentialStatus(toUpdate); err != nil {
					return err
				}
				return err
			}

			parametersChecksum, err = generateChecksumOfParameters(parameters)
			if err != nil {
				s := fmt.Sprintf(`Failed to generate the parameters checksum to store in the Status of ServiceInstanceCredential "%s/%s": %s`, binding.Namespace, binding.Name, err)
				glog.Info(s)
				c.recorder.Eventf(binding, apiv1.EventTypeWarning, errorWithParameters, s)
				c.setServiceInstanceCredentialCondition(
					toUpdate,
					v1alpha1.ServiceInstanceCredentialConditionReady,
					v1alpha1.ConditionFalse,
					errorWithParameters,
					s)
				if _, err := c.updateServiceInstanceCredentialStatus(toUpdate); err != nil {
					return err
				}
				return err
			}

			marshalledParametersWithRedaction, err := MarshalRawParameters(parametersWithSecretsRedacted)
			if err != nil {
				s := fmt.Sprintf(`Failed to marshal the parameters to store in the Status of ServiceInstanceCredential "%s/%s": %s`, binding.Namespace, binding.Name, err)
				glog.Info(s)
				c.recorder.Eventf(binding, apiv1.EventTypeWarning, errorWithParameters, s)
				c.setServiceInstanceCredentialCondition(
					toUpdate,
					v1alpha1.ServiceInstanceCredentialConditionReady,
					v1alpha1.ConditionFalse,
					errorWithParameters,
					s)
				if _, err := c.updateServiceInstanceCredentialStatus(toUpdate); err != nil {
					return err
				}
				return err
			}

			rawParametersWithRedaction = &runtime.RawExtension{
				Raw: marshalledParametersWithRedaction,
			}
		}

		toUpdate.Status.InProgressProperties = &v1alpha1.ServiceInstanceCredentialPropertiesState{
			Parameters:         rawParametersWithRedaction,
			ParametersChecksum: parametersChecksum,
			UserInfo:           toUpdate.Spec.UserInfo,
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

		if utilfeature.DefaultFeatureGate.Enabled(scfeatures.OriginatingIdentity) {
			originatingIdentity, err := buildOriginatingIdentity(binding.Spec.UserInfo)
			if err != nil {
				s := fmt.Sprintf(`Error building originating identity headers for binding ServiceInstanceCredential "%v/%v": %v`, binding.Namespace, binding.Name, err)
				glog.Warning(s)
				c.recorder.Event(binding, apiv1.EventTypeWarning, errorWithOriginatingIdentity, s)
				c.setServiceInstanceCredentialCondition(
					toUpdate,
					v1alpha1.ServiceInstanceCredentialConditionReady,
					v1alpha1.ConditionFalse,
					errorWithOriginatingIdentity,
					s,
				)
				if _, err := c.updateServiceInstanceCredentialStatus(toUpdate); err != nil {
					return err
				}
				return err
			}
			request.OriginatingIdentity = originatingIdentity
		}

		if toUpdate.Status.CurrentOperation == "" {
			toUpdate, err = c.recordStartOfServiceInstanceCredentialOperation(toUpdate, v1alpha1.ServiceInstanceCredentialOperationBind)
			if err != nil {
				// There has been an update to the binding. Start reconciliation
				// over with a fresh view of the binding.
				return err
			}
		}

		response, err := brokerClient.Bind(request)
		if err != nil {
			if httpErr, ok := osb.IsHTTPError(err); ok {
				s := fmt.Sprintf("Error creating ServiceInstanceCredential \"%s/%s\" for ServiceInstance \"%s/%s\" of ServiceClass %q at ServiceBroker %q, %v",
					binding.Name,
					binding.Namespace,
					instance.Namespace,
					instance.Name,
					serviceClass.Spec.ExternalName,
					brokerName,
					httpErr.Error(),
				)
				glog.Warning(s)
				c.recorder.Event(binding, apiv1.EventTypeWarning, errorBindCallReason, s)

				c.setServiceInstanceCredentialCondition(
					toUpdate,
					v1alpha1.ServiceInstanceCredentialConditionFailed,
					v1alpha1.ConditionTrue,
					"ServiceInstanceCredentialReturnedFailure",
					s,
				)
				c.setServiceInstanceCredentialCondition(
					toUpdate,
					v1alpha1.ServiceInstanceCredentialConditionReady,
					v1alpha1.ConditionFalse,
					errorBindCallReason,
					"Bind call failed. "+s)
				c.clearServiceInstanceCredentialCurrentOperation(toUpdate)
				if _, err := c.updateServiceInstanceCredentialStatus(toUpdate); err != nil {
					return err
				}
				return nil
			}

			s := fmt.Sprintf("Error creating ServiceInstanceCredential \"%s/%s\" for ServiceInstance \"%s/%s\" of ServiceClass %q at ServiceBroker %q: %s", binding.Name, binding.Namespace, instance.Namespace, instance.Name, serviceClass.Spec.ExternalName, brokerName, err)
			glog.Warning(s)
			c.recorder.Event(binding, apiv1.EventTypeWarning, errorBindCallReason, s)
			c.setServiceInstanceCredentialCondition(
				toUpdate,
				v1alpha1.ServiceInstanceCredentialConditionReady,
				v1alpha1.ConditionFalse,
				errorBindCallReason,
				"Bind call failed. "+s)

			if !time.Now().Before(toUpdate.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
				s := fmt.Sprintf(`Stopping reconciliation retries on ServiceInstanceCredential "%v/%v" because too much time has elapsed`, binding.Namespace, binding.Name)
				glog.Info(s)
				c.recorder.Event(binding, apiv1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)
				c.setServiceInstanceCredentialCondition(toUpdate,
					v1alpha1.ServiceInstanceCredentialConditionFailed,
					v1alpha1.ConditionTrue,
					errorReconciliationRetryTimeoutReason,
					s)
				c.clearServiceInstanceCredentialCurrentOperation(toUpdate)
				if _, err := c.updateServiceInstanceCredentialStatus(toUpdate); err != nil {
					return err
				}
				return nil
			}

			if _, err := c.updateServiceInstanceCredentialStatus(toUpdate); err != nil {
				return err
			}
			return err
		}

		// The Bind request has returned successfully from the Broker. Continue
		// with the sucess case of creating the ServiceInstanceCredential.

		// Save off the external properties here even if the subsequent
		// credentials injection fails. The Broker has already processed the
		// request, so this is what the Broker knows about the state of the
		// binding.
		toUpdate.Status.ExternalProperties = toUpdate.Status.InProgressProperties

		err = c.injectServiceInstanceCredential(binding, response.Credentials)
		if err != nil {
			s := fmt.Sprintf("Error injecting binding results for ServiceInstanceCredential \"%s/%s\": %s", binding.Namespace, binding.Name, err)
			glog.Warning(s)
			c.recorder.Event(binding, apiv1.EventTypeWarning, errorInjectingBindResultReason, s)
			c.setServiceInstanceCredentialCondition(
				toUpdate,
				v1alpha1.ServiceInstanceCredentialConditionReady,
				v1alpha1.ConditionFalse,
				errorInjectingBindResultReason,
				"Error injecting bind result "+s,
			)

			if !time.Now().Before(toUpdate.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
				s := fmt.Sprintf(`Stopping reconciliation retries on ServiceInstanceCredential "%v/%v" because too much time has elapsed`, binding.Namespace, binding.Name)
				glog.Info(s)
				c.recorder.Event(binding, apiv1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)
				c.setServiceInstanceCredentialCondition(toUpdate,
					v1alpha1.ServiceInstanceCredentialConditionFailed,
					v1alpha1.ConditionTrue,
					errorReconciliationRetryTimeoutReason,
					s)
				c.clearServiceInstanceCredentialCurrentOperation(toUpdate)
				if _, err := c.updateServiceInstanceCredentialStatus(toUpdate); err != nil {
					return err
				}

				// TODO: We need to delete the ServiceInstanceCredential from the
				// Broker since the Bind request was successful. This needs to be
				// addressed as part of orphan mitigiation.

				return nil
			}

			if _, err := c.updateServiceInstanceCredentialStatus(toUpdate); err != nil {
				return err
			}
			return err
		}

		c.clearServiceInstanceCredentialCurrentOperation(toUpdate)

		c.setServiceInstanceCredentialCondition(
			toUpdate,
			v1alpha1.ServiceInstanceCredentialConditionReady,
			v1alpha1.ConditionTrue,
			successInjectedBindResultReason,
			successInjectedBindResultMessage,
		)

		if _, err := c.updateServiceInstanceCredentialStatus(toUpdate); err != nil {
			return err
		}

		c.recorder.Event(binding, apiv1.EventTypeNormal, successInjectedBindResultReason, successInjectedBindResultMessage)
		glog.V(5).Infof("Successfully bound to ServiceInstance %v/%v of ServiceClass %v at ServiceBroker %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName)

		return nil
	}

	// All updates not having a DeletingTimestamp will have been handled above
	// and returned early. If we reach this point, we're dealing with an update
	// that's actually a soft delete-- i.e. we have some finalization to do.
	if finalizers := sets.NewString(binding.Finalizers...); finalizers.Has(v1alpha1.FinalizerServiceCatalog) {
		glog.V(4).Infof("Finalizing ServiceInstanceCredential %v/%v", binding.Namespace, binding.Name)
		err = c.ejectServiceInstanceCredential(binding)
		if err != nil {
			s := fmt.Sprintf("Error deleting secret: %s", err)
			glog.Warning(s)
			c.recorder.Eventf(binding, apiv1.EventTypeWarning, errorEjectingBindReason, "%v %v", errorEjectingBindMessage, s)
			c.setServiceInstanceCredentialCondition(
				toUpdate,
				v1alpha1.ServiceInstanceCredentialConditionReady,
				v1alpha1.ConditionUnknown,
				errorEjectingBindReason,
				errorEjectingBindMessage+s,
			)
			if _, err := c.updateServiceInstanceCredentialStatus(toUpdate); err != nil {
				return err
			}
			return err
		}

		unbindRequest := &osb.UnbindRequest{
			BindingID:  binding.Spec.ExternalID,
			InstanceID: instance.Spec.ExternalID,
			ServiceID:  serviceClass.Spec.ExternalID,
			PlanID:     servicePlan.Spec.ExternalID,
		}

		if utilfeature.DefaultFeatureGate.Enabled(scfeatures.OriginatingIdentity) {
			originatingIdentity, err := buildOriginatingIdentity(binding.Spec.UserInfo)
			if err != nil {
				s := fmt.Sprintf(`Error building originating identity headers for unbinding ServiceInstanceCredential "%v/%v": %v`, binding.Namespace, binding.Name, err)
				glog.Warning(s)
				c.recorder.Event(binding, apiv1.EventTypeWarning, errorWithOriginatingIdentity, s)
				c.setServiceInstanceCredentialCondition(
					toUpdate,
					v1alpha1.ServiceInstanceCredentialConditionReady,
					v1alpha1.ConditionFalse,
					errorWithOriginatingIdentity,
					s,
				)
				if _, err := c.updateServiceInstanceCredentialStatus(toUpdate); err != nil {
					return err
				}
				return err
			}
			unbindRequest.OriginatingIdentity = originatingIdentity
		}

		if toUpdate.Status.CurrentOperation == "" {
			toUpdate, err = c.recordStartOfServiceInstanceCredentialOperation(toUpdate, v1alpha1.ServiceInstanceCredentialOperationUnbind)
			if err != nil {
				// There has been an update to the binding. Start reconciliation
				// over with a fresh view of the binding.
				return err
			}
		}

		_, err = brokerClient.Unbind(unbindRequest)
		if err != nil {
			if httpErr, ok := osb.IsHTTPError(err); ok {
				s := fmt.Sprintf("Error unbinding ServiceInstanceCredential \"%s/%s\" for ServiceInstance \"%s/%s\" of ServiceClass %q at ServiceBroker %q: %s",
					binding.Name,
					binding.Namespace,
					instance.Namespace,
					instance.Name,
					serviceClass.Spec.ExternalName,
					brokerName,
					httpErr.Error(),
				)
				glog.Warning(s)
				c.recorder.Event(binding, apiv1.EventTypeWarning, errorUnbindCallReason, s)
				c.setServiceInstanceCredentialCondition(
					toUpdate,
					v1alpha1.ServiceInstanceCredentialConditionReady,
					v1alpha1.ConditionUnknown,
					errorUnbindCallReason,
					"Unbind call failed. "+s)
				c.setServiceInstanceCredentialCondition(
					toUpdate,
					v1alpha1.ServiceInstanceCredentialConditionFailed,
					v1alpha1.ConditionTrue,
					errorUnbindCallReason,
					"Unbind call failed. "+s)
				c.clearServiceInstanceCredentialCurrentOperation(toUpdate)
				if _, err := c.updateServiceInstanceCredentialStatus(toUpdate); err != nil {
					return err
				}
				return nil
			}
			s := fmt.Sprintf(
				"Error unbinding ServiceInstanceCredential \"%s/%s\" for ServiceInstance \"%s/%s\" of ServiceClass %q at ServiceBroker %q: %s",
				binding.Name,
				binding.Namespace,
				instance.Namespace,
				instance.Name,
				serviceClass.Spec.ExternalName,
				brokerName,
				err,
			)
			glog.Warning(s)
			c.recorder.Event(binding, apiv1.EventTypeWarning, errorUnbindCallReason, s)
			c.setServiceInstanceCredentialCondition(
				toUpdate,
				v1alpha1.ServiceInstanceCredentialConditionReady,
				v1alpha1.ConditionUnknown,
				errorUnbindCallReason,
				"Unbind call failed. "+s)

			if !time.Now().Before(toUpdate.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
				s := fmt.Sprintf(`Stopping reconciliation retries on ServiceInstanceCredential "%v/%v" because too much time has elapsed`, binding.Namespace, binding.Name)
				glog.Info(s)
				c.recorder.Event(binding, apiv1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)
				c.setServiceInstanceCredentialCondition(toUpdate,
					v1alpha1.ServiceInstanceCredentialConditionFailed,
					v1alpha1.ConditionTrue,
					errorReconciliationRetryTimeoutReason,
					s)
				c.clearServiceInstanceCredentialCurrentOperation(toUpdate)
				if _, err := c.updateServiceInstanceCredentialStatus(toUpdate); err != nil {
					return err
				}
				return nil
			}

			if _, err := c.updateServiceInstanceCredentialStatus(toUpdate); err != nil {
				return err
			}
			return err
		}

		c.setServiceInstanceCredentialCondition(
			toUpdate,
			v1alpha1.ServiceInstanceCredentialConditionReady,
			v1alpha1.ConditionFalse,
			successUnboundReason,
			"The binding was deleted successfully",
		)
		c.clearServiceInstanceCredentialCurrentOperation(toUpdate)

		toUpdate.Status.ExternalProperties = nil

		// Clear the finalizer
		finalizers.Delete(v1alpha1.FinalizerServiceCatalog)
		toUpdate.Finalizers = finalizers.List()
		if _, err := c.updateServiceInstanceCredentialStatus(toUpdate); err != nil {
			return err
		}

		c.recorder.Event(binding, apiv1.EventTypeNormal, successUnboundReason, "This binding was deleted successfully")
		glog.V(5).Infof("Successfully deleted ServiceInstanceCredential %v/%v of ServiceInstance %v/%v of ServiceClass %v at ServiceBroker %v", binding.Namespace, binding.Name, instance.Namespace, instance.Name, serviceClass.Name, brokerName)
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
	if plan.Spec.Bindable != nil {
		return *plan.Spec.Bindable
	}

	return serviceClass.Spec.Bindable
}

func (c *controller) injectServiceInstanceCredential(binding *v1alpha1.ServiceInstanceCredential, credentials map[string]interface{}) error {
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
			return fmt.Errorf("Secret '%s' is not owned by ServiceInstanceCredential, controllerRef: %v",
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
		secret := &apiv1.Secret{
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

func (c *controller) ejectServiceInstanceCredential(binding *v1alpha1.ServiceInstanceCredential) error {
	var err error

	glog.V(5).Infof("Deleting Secret %v/%v", binding.Namespace, binding.Spec.SecretName)
	err = c.kubeClient.Core().Secrets(binding.Namespace).Delete(binding.Spec.SecretName, &metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

// setServiceInstanceCredentialCondition sets a single condition on a ServiceInstanceCredential's status: if
// the condition already exists in the status, it is mutated; if the condition
// does not already exist in the status, it is added.  Other conditions in the
// status are not altered.  If the condition exists and its status changes,
// the LastTransitionTime field is updated.
//
// Note: objects coming from informers should never be mutated; always pass a
// deep copy as the binding parameter.
func (c *controller) setServiceInstanceCredentialCondition(toUpdate *v1alpha1.ServiceInstanceCredential,
	conditionType v1alpha1.ServiceInstanceCredentialConditionType,
	status v1alpha1.ConditionStatus,
	reason, message string) {

	setServiceInstanceCredentialConditionInternal(toUpdate, conditionType, status, reason, message, metav1.Now())
}

// setServiceInstanceCredentialConditionInternal is
// setServiceInstanceCredentialCondition but allows the time to be parameterized
// for testing.
func setServiceInstanceCredentialConditionInternal(toUpdate *v1alpha1.ServiceInstanceCredential,
	conditionType v1alpha1.ServiceInstanceCredentialConditionType,
	status v1alpha1.ConditionStatus,
	reason, message string,
	t metav1.Time) {

	glog.V(5).Infof("Setting ServiceInstanceCredential '%v/%v' condition %q to %v", toUpdate.Namespace, toUpdate.Name, conditionType, status)

	newCondition := v1alpha1.ServiceInstanceCredentialCondition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	}

	if len(toUpdate.Status.Conditions) == 0 {
		glog.Infof(`Setting lastTransitionTime for ServiceInstanceCredential "%v/%v" condition %q to %v`,
			toUpdate.Namespace, toUpdate.Name, conditionType, t)
		newCondition.LastTransitionTime = t
		toUpdate.Status.Conditions = []v1alpha1.ServiceInstanceCredentialCondition{newCondition}
		return
	}
	for i, cond := range toUpdate.Status.Conditions {
		if cond.Type == conditionType {
			if cond.Status != newCondition.Status {
				glog.V(3).Infof(`Found status change for ServiceInstanceCredential "%v/%v" condition %q: %q -> %q; setting lastTransitionTime to %v`,
					toUpdate.Namespace, toUpdate.Name, conditionType, cond.Status, status, t)
				newCondition.LastTransitionTime = t
			} else {
				newCondition.LastTransitionTime = cond.LastTransitionTime
			}

			toUpdate.Status.Conditions[i] = newCondition
			return
		}
	}

	glog.V(3).Infof("Setting lastTransitionTime for ServiceInstanceCredential '%v/%v' condition %q to %v",
		toUpdate.Namespace, toUpdate.Name, conditionType, t)

	newCondition.LastTransitionTime = t
	toUpdate.Status.Conditions = append(toUpdate.Status.Conditions, newCondition)
}

func (c *controller) updateServiceInstanceCredentialStatus(toUpdate *v1alpha1.ServiceInstanceCredential) (*v1alpha1.ServiceInstanceCredential, error) {
	glog.V(4).Infof("Updating status for ServiceInstanceCredential %v/%v", toUpdate.Namespace, toUpdate.Name)
	updatedBinding, err := c.serviceCatalogClient.ServiceInstanceCredentials(toUpdate.Namespace).UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf("Error updating status for ServiceInstanceCredential %v/%v", toUpdate.Namespace, toUpdate.Name)
	}
	return updatedBinding, err
}

// updateServiceInstanceCredentialCondition updates the given condition for the given ServiceInstanceCredential
// with the given status, reason, and message.
func (c *controller) updateServiceInstanceCredentialCondition(
	binding *v1alpha1.ServiceInstanceCredential,
	conditionType v1alpha1.ServiceInstanceCredentialConditionType,
	status v1alpha1.ConditionStatus,
	reason, message string) error {

	toUpdate, err := makeServiceInstanceCredentialClone(binding)
	if err != nil {
		return err
	}

	c.setServiceInstanceCredentialCondition(toUpdate, conditionType, status, reason, message)

	glog.V(4).Infof("Updating %v condition for ServiceInstanceCredential %v/%v to %v (Reason: %q, Message: %q)",
		conditionType, binding.Namespace, binding.Name, status, reason, message)
	_, err = c.serviceCatalogClient.ServiceInstanceCredentials(binding.Namespace).UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf("Error updating %v condition for ServiceInstanceCredential %v/%v to %v: %v", conditionType, binding.Namespace, binding.Name, status, err)
	}
	return err
}

func (c *controller) bindingDelete(obj interface{}) {
	binding, ok := obj.(*v1alpha1.ServiceInstanceCredential)
	if binding == nil || !ok {
		return
	}

	glog.V(4).Infof("Received delete event for ServiceInstanceCredential %v/%v; no further processing will occur", binding.Namespace, binding.Name)
}

// recordStartOfServiceInstanceCredentialOperation updates the binding to indicate
// that there is a current operation being performed. The Status of the binding
// is recorded in the registry.
// params:
// toUpdate - a modifiable copy of the binding in the registry to update
// operation - operation that is being performed on the binding
// returns:
// 1 - a modifiable copy of toUpdate; or toUpdate if there was an error
// 2 - any error that occurred
func (c *controller) recordStartOfServiceInstanceCredentialOperation(toUpdate *v1alpha1.ServiceInstanceCredential, operation v1alpha1.ServiceInstanceCredentialOperation) (*v1alpha1.ServiceInstanceCredential, error) {
	toUpdate.Status.CurrentOperation = operation
	now := metav1.Now()
	toUpdate.Status.OperationStartTime = &now
	reason := ""
	message := ""
	switch operation {
	case v1alpha1.ServiceInstanceCredentialOperationBind:
		reason = bindingInFlightReason
		message = bindingInFlightMessage
	case v1alpha1.ServiceInstanceCredentialOperationUnbind:
		reason = unbindingInFlightReason
		message = unbindingInFlightMessage
	}
	c.setServiceInstanceCredentialCondition(
		toUpdate,
		v1alpha1.ServiceInstanceCredentialConditionReady,
		v1alpha1.ConditionFalse,
		reason,
		message,
	)
	return c.updateServiceInstanceCredentialStatus(toUpdate)
}

// clearServiceInstanceCredentialCurrentOperation sets the fields of the binding's
// Status to indicate that there is no current operation being performed. The
// Status is *not* recorded in the registry.
func (c *controller) clearServiceInstanceCredentialCurrentOperation(toUpdate *v1alpha1.ServiceInstanceCredential) {
	toUpdate.Status.CurrentOperation = ""
	toUpdate.Status.OperationStartTime = nil
	toUpdate.Status.ReconciledGeneration = toUpdate.Generation
	toUpdate.Status.InProgressProperties = nil
}

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
	"net/http"
	"time"

	"github.com/golang/glog"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	"k8s.io/apimachinery/pkg/runtime"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"github.com/kubernetes-incubator/service-catalog/pkg/api"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	scfeatures "github.com/kubernetes-incubator/service-catalog/pkg/features"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
)

// bindingControllerKind contains the schema.GroupVersionKind for this controller type.
var bindingControllerKind = v1beta1.SchemeGroupVersion.WithKind("ServiceBinding")

// ServiceBinding handlers and control-loop

func (c *controller) bindingAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.bindingQueue.Add(key)
}

func (c *controller) reconcileServiceBindingKey(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	binding, err := c.bindingLister.ServiceBindings(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		glog.Infof("Not doing work for ServiceBinding %v because it has been deleted", key)
		return nil
	}
	if err != nil {
		glog.Infof("Unable to retrieve ServiceBinding %v from store: %v", key, err)
		return err
	}

	return c.reconcileServiceBinding(binding)
}

func (c *controller) bindingUpdate(oldObj, newObj interface{}) {
	c.bindingAdd(newObj)
}

func makeServiceBindingClone(binding *v1beta1.ServiceBinding) (*v1beta1.ServiceBinding, error) {
	clone, err := api.Scheme.DeepCopy(binding)
	if err != nil {
		return nil, err
	}
	return clone.(*v1beta1.ServiceBinding), nil
}

func isServiceBindingFailed(binding *v1beta1.ServiceBinding) bool {
	for _, condition := range binding.Status.Conditions {
		if condition.Type == v1beta1.ServiceBindingConditionFailed && condition.Status == v1beta1.ConditionTrue {
			return true
		}
	}
	return false
}

// setAndUpdateOrphanMitigation is for setting the OrphanMitigationInProgress
// status to true, setting the proper condition statuses, and persisting the
// changes via updateServiceBindingStatus.
func (c *controller) setAndUpdateOrphanMitigation(binding *v1beta1.ServiceBinding, toUpdate *v1beta1.ServiceBinding, instance *v1beta1.ServiceInstance, serviceClass *v1beta1.ClusterServiceClass, brokerName string, errorStr string) error {
	s := fmt.Sprintf("Starting orphan mitgation for ServiceBinding \"%s/%s\" for ServiceInstance \"%s/%s\" of ClusterServiceClass %q at ServiceBroker %q, %v",
		binding.Name,
		binding.Namespace,
		instance.Namespace,
		instance.Name,
		serviceClass.Name,
		brokerName,
		errorStr,
	)
	toUpdate.Status.OrphanMitigationInProgress = true
	toUpdate.Status.OperationStartTime = nil
	toUpdate.Status.InProgressProperties = nil
	glog.V(5).Info(s)

	c.setServiceBindingCondition(
		toUpdate,
		v1beta1.ServiceBindingConditionReady,
		v1beta1.ConditionFalse,
		errorServiceBindingOrphanMitigation,
		s,
	)

	c.recorder.Event(binding, corev1.EventTypeWarning, errorServiceBindingOrphanMitigation, s)
	if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
		return err
	}
	return nil
}

// an error is returned to indicate that the binding has not been
// fully processed and should be resubmitted at a later time.
func (c *controller) reconcileServiceBinding(binding *v1beta1.ServiceBinding) error {
	if isServiceBindingFailed(binding) && binding.ObjectMeta.DeletionTimestamp == nil && !binding.Status.OrphanMitigationInProgress {
		glog.V(4).Infof(
			"Not processing event for ServiceBinding %v/%v because status showed that it has failed",
			binding.Namespace,
			binding.Name,
		)
		return nil
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
				"Not processing event for ServiceBinding %v/%v because reconciled generation showed there is no work to do",
				binding.Namespace,
				binding.Name,
			)
			return nil
		}
	}

	toUpdate, err := makeServiceBindingClone(binding)
	if err != nil {
		return err
	}

	glog.V(4).Infof("Processing ServiceBinding %v/%v", binding.Namespace, binding.Name)

	instance, err := c.instanceLister.ServiceInstances(binding.Namespace).Get(binding.Spec.ServiceInstanceRef.Name)
	if err != nil {
		s := fmt.Sprintf("ServiceBinding \"%s/%s\" references a non-existent ServiceInstance \"%s/%s\"", binding.Namespace, binding.Name, binding.Namespace, binding.Spec.ServiceInstanceRef.Name)
		glog.Warningf(
			"ServiceBinding %s/%s references a non-existent instance %s/%s (%s)",
			binding.Namespace,
			binding.Name,
			binding.Namespace,
			binding.Spec.ServiceInstanceRef.Name,
			err,
		)
		c.recorder.Event(binding, corev1.EventTypeWarning, errorNonexistentServiceInstanceReason, s)
		c.setServiceBindingCondition(
			toUpdate,
			v1beta1.ServiceBindingConditionReady,
			v1beta1.ConditionFalse,
			errorNonexistentServiceInstanceReason,
			"The binding references an ServiceInstance that does not exist. "+s,
		)
		if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
			return err
		}
		return err
	}

	if instance.Status.AsyncOpInProgress {
		s := fmt.Sprintf(
			"ServiceBinding \"%s/%s\" trying to bind to ServiceInstance \"%s/%s\" that has ongoing asynchronous operation",
			binding.Namespace,
			binding.Name,
			binding.Namespace,
			binding.Spec.ServiceInstanceRef.Name,
		)
		glog.Info(s)
		c.recorder.Event(binding, corev1.EventTypeWarning, errorWithOngoingAsyncOperation, s)
		c.setServiceBindingCondition(
			toUpdate,
			v1beta1.ServiceBindingConditionReady,
			v1beta1.ConditionFalse,
			errorWithOngoingAsyncOperation,
			errorWithOngoingAsyncOperationMessage,
		)
		if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
			return err
		}
		return fmt.Errorf("Ongoing Asynchronous operation")
	}

	if instance.Spec.ClusterServiceClassRef == nil || instance.Spec.ClusterServicePlanRef == nil {
		// retry later
		return fmt.Errorf("ClusterServiceClass or ClusterServicePlan references for Instance have not been resolved yet")
	}

	serviceClass, servicePlan, brokerName, brokerClient, err := c.getClusterServiceClassPlanAndClusterServiceBrokerForServiceBinding(instance, binding)
	if err != nil {
		return err // retry later
	}

	if !isPlanBindable(serviceClass, servicePlan) {
		s := fmt.Sprintf(
			"ServiceBinding \"%s/%s\" references a non-bindable ClusterServiceClass (%q) and Plan (%q) combination",
			binding.Namespace,
			binding.Name,
			instance.Spec.ExternalClusterServiceClassName,
			instance.Spec.ExternalClusterServicePlanName,
		)
		glog.Warning(s)
		c.recorder.Event(binding, corev1.EventTypeWarning, errorNonbindableClusterServiceClassReason, s)
		c.setServiceBindingCondition(
			toUpdate,
			v1beta1.ServiceBindingConditionReady,
			v1beta1.ConditionFalse,
			errorNonbindableClusterServiceClassReason,
			s,
		)
		if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
			return err
		}
		return nil
	}

	if binding.DeletionTimestamp == nil && !binding.Status.OrphanMitigationInProgress { // Add or update
		glog.V(4).Infof("Adding/Updating ServiceBinding %v/%v", binding.Namespace, binding.Name)

		ns, err := c.kubeClient.Core().Namespaces().Get(instance.Namespace, metav1.GetOptions{})
		if err != nil {
			s := fmt.Sprintf("Failed to get namespace %q during binding: %s", instance.Namespace, err)
			glog.Info(s)
			c.recorder.Eventf(binding, corev1.EventTypeWarning, errorFindingNamespaceServiceInstanceReason, s)
			c.setServiceBindingCondition(
				toUpdate,
				v1beta1.ServiceBindingConditionReady,
				v1beta1.ConditionFalse,
				errorFindingNamespaceServiceInstanceReason,
				"Error finding namespace for instance. "+s,
			)
			if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
				return err
			}
			return err
		}

		if !isServiceInstanceReady(instance) {
			s := fmt.Sprintf(`ServiceBinding cannot begin because referenced instance "%v/%v" is not ready`, instance.Namespace, instance.Name)
			glog.Info(s)
			c.recorder.Eventf(binding, corev1.EventTypeWarning, errorServiceInstanceNotReadyReason, s)
			c.setServiceBindingCondition(
				toUpdate,
				v1beta1.ServiceBindingConditionReady,
				v1beta1.ConditionFalse,
				errorServiceInstanceNotReadyReason,
				s,
			)
			if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
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
				s := fmt.Sprintf("Failed to prepare ServiceBinding parameters\n%s\n %s", binding.Spec.Parameters, err)
				glog.Warning(s)
				c.recorder.Event(binding, corev1.EventTypeWarning, errorWithParameters, s)
				c.setServiceBindingCondition(
					toUpdate,
					v1beta1.ServiceBindingConditionReady,
					v1beta1.ConditionFalse,
					errorWithParameters,
					s,
				)
				if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
					return err
				}
				return err
			}

			parametersChecksum, err = generateChecksumOfParameters(parameters)
			if err != nil {
				s := fmt.Sprintf(`Failed to generate the parameters checksum to store in the Status of ServiceBinding "%s/%s": %s`, binding.Namespace, binding.Name, err)
				glog.Info(s)
				c.recorder.Eventf(binding, corev1.EventTypeWarning, errorWithParameters, s)
				c.setServiceBindingCondition(
					toUpdate,
					v1beta1.ServiceBindingConditionReady,
					v1beta1.ConditionFalse,
					errorWithParameters,
					s)
				if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
					return err
				}
				return err
			}

			marshalledParametersWithRedaction, err := MarshalRawParameters(parametersWithSecretsRedacted)
			if err != nil {
				s := fmt.Sprintf(`Failed to marshal the parameters to store in the Status of ServiceBinding "%s/%s": %s`, binding.Namespace, binding.Name, err)
				glog.Info(s)
				c.recorder.Eventf(binding, corev1.EventTypeWarning, errorWithParameters, s)
				c.setServiceBindingCondition(
					toUpdate,
					v1beta1.ServiceBindingConditionReady,
					v1beta1.ConditionFalse,
					errorWithParameters,
					s)
				if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
					return err
				}
				return err
			}

			rawParametersWithRedaction = &runtime.RawExtension{
				Raw: marshalledParametersWithRedaction,
			}
		}

		toUpdate.Status.InProgressProperties = &v1beta1.ServiceBindingPropertiesState{
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
				s := fmt.Sprintf(`Error building originating identity headers for binding ServiceBinding "%v/%v": %v`, binding.Namespace, binding.Name, err)
				glog.Warning(s)
				c.recorder.Event(binding, corev1.EventTypeWarning, errorWithOriginatingIdentity, s)
				c.setServiceBindingCondition(
					toUpdate,
					v1beta1.ServiceBindingConditionReady,
					v1beta1.ConditionFalse,
					errorWithOriginatingIdentity,
					s,
				)
				if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
					return err
				}
				return err
			}
			request.OriginatingIdentity = originatingIdentity
		}

		if toUpdate.Status.CurrentOperation == "" {
			toUpdate, err = c.recordStartOfServiceBindingOperation(toUpdate, v1beta1.ServiceBindingOperationBind)
			if err != nil {
				// There has been an update to the binding. Start reconciliation
				// over with a fresh view of the binding.
				return err
			}
		}

		response, err := brokerClient.Bind(request)
		// orphan mitigation: looking for timeout
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			c.setServiceBindingCondition(
				toUpdate,
				v1beta1.ServiceBindingConditionFailed,
				v1beta1.ConditionTrue,
				errorBindCallReason,
				"Communication with the ServiceBroker timed out; Bind operation will not be retried: "+err.Error(),
			)
			return c.setAndUpdateOrphanMitigation(binding, toUpdate, instance, serviceClass, brokerName, netErr.Error())
		} else if err != nil {
			if httpErr, ok := osb.IsHTTPError(err); ok {
				// orphan mitigation: looking for 2xx (excluding 200), 408, 5xx
				if httpErr.StatusCode > 200 && httpErr.StatusCode < 300 ||
					httpErr.StatusCode == http.StatusRequestTimeout ||
					httpErr.StatusCode >= 500 && httpErr.StatusCode < 600 {
					c.setServiceBindingCondition(
						toUpdate,
						v1beta1.ServiceBindingConditionFailed,
						v1beta1.ConditionTrue,
						errorBindCallReason,
						"ServiceBroker returned a failure; Bind operation will not be retried: "+err.Error(),
					)
					return c.setAndUpdateOrphanMitigation(binding, toUpdate, instance, serviceClass, brokerName, httpErr.Error())
				}
				s := fmt.Sprintf("Error creating ServiceBinding \"%s/%s\" for ServiceInstance \"%s/%s\" of ClusterServiceClass %q at ClusterServiceBroker %q, %v",
					binding.Name,
					binding.Namespace,
					instance.Namespace,
					instance.Name,
					serviceClass.Spec.ExternalName,
					brokerName,
					httpErr.Error(),
				)
				glog.Warning(s)
				c.recorder.Event(binding, corev1.EventTypeWarning, errorBindCallReason, s)

				c.setServiceBindingCondition(
					toUpdate,
					v1beta1.ServiceBindingConditionFailed,
					v1beta1.ConditionTrue,
					"ServiceBindingReturnedFailure",
					s,
				)
				c.setServiceBindingCondition(
					toUpdate,
					v1beta1.ServiceBindingConditionReady,
					v1beta1.ConditionFalse,
					errorBindCallReason,
					"Bind call failed. "+s)
				c.clearServiceBindingCurrentOperation(toUpdate)
				if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
					return err
				}
				return nil
			}

			s := fmt.Sprintf("Error creating ServiceBinding \"%s/%s\" for ServiceInstance \"%s/%s\" of ClusterServiceClass %q at ClusterServiceBroker %q: %s", binding.Name, binding.Namespace, instance.Namespace, instance.Name, serviceClass.Spec.ExternalName, brokerName, err)
			glog.Warning(s)
			c.recorder.Event(binding, corev1.EventTypeWarning, errorBindCallReason, s)
			c.setServiceBindingCondition(
				toUpdate,
				v1beta1.ServiceBindingConditionReady,
				v1beta1.ConditionFalse,
				errorBindCallReason,
				"Bind call failed. "+s)

			if !time.Now().Before(toUpdate.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
				s := fmt.Sprintf(`Stopping reconciliation retries on ServiceBinding "%v/%v" because too much time has elapsed`, binding.Namespace, binding.Name)
				glog.Info(s)
				c.recorder.Event(binding, corev1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)
				c.setServiceBindingCondition(toUpdate,
					v1beta1.ServiceBindingConditionFailed,
					v1beta1.ConditionTrue,
					errorReconciliationRetryTimeoutReason,
					s)
				c.clearServiceBindingCurrentOperation(toUpdate)
				if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
					return err
				}
				return nil
			}

			if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
				return err
			}
			return err
		}

		// The Bind request has returned successfully from the Broker. Continue
		// with the success case of creating the ServiceBinding.

		// Save off the external properties here even if the subsequent
		// credentials injection fails. The Broker has already processed the
		// request, so this is what the Broker knows about the state of the
		// binding.
		toUpdate.Status.ExternalProperties = toUpdate.Status.InProgressProperties

		err = c.injectServiceBinding(binding, response.Credentials)
		if err != nil {
			s := fmt.Sprintf("Error injecting binding results for ServiceBinding \"%s/%s\": %s", binding.Namespace, binding.Name, err)
			glog.Warning(s)
			c.recorder.Event(binding, corev1.EventTypeWarning, errorInjectingBindResultReason, s)
			c.setServiceBindingCondition(
				toUpdate,
				v1beta1.ServiceBindingConditionReady,
				v1beta1.ConditionFalse,
				errorInjectingBindResultReason,
				"Error injecting bind result "+s,
			)

			if !time.Now().Before(toUpdate.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
				s := fmt.Sprintf(`Stopping reconciliation retries on ServiceBinding "%v/%v" because too much time has elapsed`, binding.Namespace, binding.Name)
				glog.Info(s)
				c.recorder.Event(binding, corev1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)
				c.setServiceBindingCondition(toUpdate,
					v1beta1.ServiceBindingConditionFailed,
					v1beta1.ConditionTrue,
					errorReconciliationRetryTimeoutReason,
					s)
				return c.setAndUpdateOrphanMitigation(binding, toUpdate, instance, serviceClass, brokerName, "too much time has elapsed")
			}

			if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
				return err
			}
			// TODO: solve scenario where bind request successful, credential injection fails, later reconciliations have non-failing errors
			// with Bind request. After retry duration, reconciler gives up but will not do orphan mitigation.
			return err
		}

		c.clearServiceBindingCurrentOperation(toUpdate)

		c.setServiceBindingCondition(
			toUpdate,
			v1beta1.ServiceBindingConditionReady,
			v1beta1.ConditionTrue,
			successInjectedBindResultReason,
			successInjectedBindResultMessage,
		)

		if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
			return err
		}

		c.recorder.Event(binding, corev1.EventTypeNormal, successInjectedBindResultReason, successInjectedBindResultMessage)
		glog.V(5).Infof("Successfully bound to ServiceInstance %v/%v of ClusterServiceClass %v at ClusterServiceBroker %v", instance.Namespace, instance.Name, serviceClass.Name, brokerName)

		return nil
	}

	// All updates not having a DeletingTimestamp will have been handled above
	// and returned early, except in the case of orphan mitigation. Otherwise,
	// when we reach this point, we're dealing with an update that's actually
	// a soft delete-- i.e. we have some finalization to do.
	if finalizers := sets.NewString(binding.Finalizers...); finalizers.Has(v1beta1.FinalizerServiceCatalog) || binding.Status.OrphanMitigationInProgress {
		err := c.ejectServiceBinding(binding)
		if err != nil {
			s := fmt.Sprintf("Error deleting secret: %s", err)
			glog.Warning(s)
			c.recorder.Eventf(binding, corev1.EventTypeWarning, errorEjectingBindReason, "%v %v", errorEjectingBindMessage, s)
			c.setServiceBindingCondition(
				toUpdate,
				v1beta1.ServiceBindingConditionReady,
				v1beta1.ConditionUnknown,
				errorEjectingBindReason,
				errorEjectingBindMessage+s,
			)
			if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
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
				s := fmt.Sprintf(`Error building originating identity headers for unbinding ServiceBinding "%v/%v": %v`, binding.Namespace, binding.Name, err)
				glog.Warning(s)
				c.recorder.Event(binding, corev1.EventTypeWarning, errorWithOriginatingIdentity, s)
				c.setServiceBindingCondition(
					toUpdate,
					v1beta1.ServiceBindingConditionReady,
					v1beta1.ConditionFalse,
					errorWithOriginatingIdentity,
					s,
				)
				if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
					return err
				}
				return err
			}
			unbindRequest.OriginatingIdentity = originatingIdentity
		}

		if toUpdate.Status.CurrentOperation == "" {
			toUpdate, err = c.recordStartOfServiceBindingOperation(toUpdate, v1beta1.ServiceBindingOperationUnbind)
			if err != nil {
				// There has been an update to the binding. Start reconciliation
				// over with a fresh view of the binding.
				return err
			}
		} else if toUpdate.Status.OrphanMitigationInProgress && toUpdate.Status.OperationStartTime == nil {
			now := metav1.Now()
			toUpdate.Status.OperationStartTime = &now
		}

		_, err = brokerClient.Unbind(unbindRequest)
		if err != nil {
			if httpErr, ok := osb.IsHTTPError(err); ok {
				s := fmt.Sprintf("Error unbinding ServiceBinding \"%s/%s\" for ServiceInstance \"%s/%s\" of ClusterServiceClass %q at ClusterServiceBroker %q: %s",
					binding.Name,
					binding.Namespace,
					instance.Namespace,
					instance.Name,
					serviceClass.Spec.ExternalName,
					brokerName,
					httpErr.Error(),
				)
				glog.Warning(s)
				c.recorder.Event(binding, corev1.EventTypeWarning, errorUnbindCallReason, s)
				c.setServiceBindingCondition(
					toUpdate,
					v1beta1.ServiceBindingConditionReady,
					v1beta1.ConditionUnknown,
					errorUnbindCallReason,
					"Unbind call failed. "+s)
				if !toUpdate.Status.OrphanMitigationInProgress {
					c.setServiceBindingCondition(
						toUpdate,
						v1beta1.ServiceBindingConditionFailed,
						v1beta1.ConditionTrue,
						errorUnbindCallReason,
						"Unbind call failed. "+s)
				}
				c.clearServiceBindingCurrentOperation(toUpdate)
				if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
					return err
				}
				return nil
			}
			s := fmt.Sprintf(
				"Error unbinding ServiceBinding \"%s/%s\" for ServiceInstance \"%s/%s\" of ClusterServiceClass %q at ClusterServiceBroker %q: %s",
				binding.Namespace,
				binding.Name,
				instance.Namespace,
				instance.Name,
				serviceClass.Spec.ExternalName,
				brokerName,
				err,
			)
			glog.Warning(s)
			c.recorder.Event(binding, corev1.EventTypeWarning, errorUnbindCallReason, s)
			c.setServiceBindingCondition(
				toUpdate,
				v1beta1.ServiceBindingConditionReady,
				v1beta1.ConditionUnknown,
				errorUnbindCallReason,
				"Unbind call failed. "+s)

			if !time.Now().Before(toUpdate.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
				s := fmt.Sprintf(`Stopping reconciliation retries on ServiceBinding "%v/%v" because too much time has elapsed`, binding.Namespace, binding.Name)
				glog.Info(s)
				c.recorder.Event(binding, corev1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)
				c.setServiceBindingCondition(toUpdate,
					v1beta1.ServiceBindingConditionFailed,
					v1beta1.ConditionTrue,
					errorReconciliationRetryTimeoutReason,
					s)
				if toUpdate.Status.OrphanMitigationInProgress {
					s := fmt.Sprintf(`Stopping reconciliation retries on ServiceBinding "%v/%v" because too much time has elapsed during orphan mitigation`, binding.Namespace, binding.Name)
					glog.Info(s)
					c.recorder.Event(binding, corev1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)
				} else {
					s := fmt.Sprintf(`Stopping reconciliation retries on ServiceBinding "%v/%v" because too much time has elapsed`, binding.Namespace, binding.Name)
					glog.Info(s)
					c.recorder.Event(binding, corev1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)
					c.setServiceBindingCondition(toUpdate,
						v1beta1.ServiceBindingConditionFailed,
						v1beta1.ConditionTrue,
						errorReconciliationRetryTimeoutReason,
						s)
				}
				c.clearServiceBindingCurrentOperation(toUpdate)
				if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
					return err
				}
				return nil
			}

			if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
				return err
			}
			return err
		}

		if toUpdate.Status.OrphanMitigationInProgress {
			s := fmt.Sprintf(`Orphan mitigation successful for ServiceBinding "%v/%v"`, binding.Namespace, binding.Name)
			c.setServiceBindingCondition(toUpdate,
				v1beta1.ServiceBindingConditionReady,
				v1beta1.ConditionFalse,
				successOrphanMitigationReason,
				s)
		} else {
			c.setServiceBindingCondition(
				toUpdate,
				v1beta1.ServiceBindingConditionReady,
				v1beta1.ConditionFalse,
				successUnboundReason,
				"The binding was deleted successfully",
			)
			// Clear the finalizer
			finalizers.Delete(v1beta1.FinalizerServiceCatalog)
			toUpdate.Finalizers = finalizers.List()
		}

		toUpdate.Status.ExternalProperties = nil
		c.clearServiceBindingCurrentOperation(toUpdate)
		if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
			return err
		}

		c.recorder.Event(binding, corev1.EventTypeNormal, successUnboundReason, "This binding was deleted successfully")
		glog.V(5).Infof("Successfully deleted ServiceBinding %v/%v of ServiceInstance %v/%v of ClusterServiceClass %v at ClusterServiceBroker %v", binding.Namespace, binding.Name, instance.Namespace, instance.Name, serviceClass.Name, brokerName)
	}
	return nil
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
			return fmt.Errorf("Secret '%s' is not owned by ServiceBinding, controllerRef: %v",
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
		secret := &corev1.Secret{
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

func (c *controller) ejectServiceBinding(binding *v1beta1.ServiceBinding) error {
	var err error

	glog.V(5).Infof("Deleting Secret %v/%v", binding.Namespace, binding.Spec.SecretName)
	err = c.kubeClient.Core().Secrets(binding.Namespace).Delete(binding.Spec.SecretName, &metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

// setServiceBindingCondition sets a single condition on a ServiceBinding's status: if
// the condition already exists in the status, it is mutated; if the condition
// does not already exist in the status, it is added.  Other conditions in the
// status are not altered.  If the condition exists and its status changes,
// the LastTransitionTime field is updated.
//
// Note: objects coming from informers should never be mutated; always pass a
// deep copy as the binding parameter.
func (c *controller) setServiceBindingCondition(toUpdate *v1beta1.ServiceBinding,
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

	glog.V(5).Infof("Setting ServiceBinding '%v/%v' condition %q to %v", toUpdate.Namespace, toUpdate.Name, conditionType, status)

	newCondition := v1beta1.ServiceBindingCondition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	}

	if len(toUpdate.Status.Conditions) == 0 {
		glog.Infof(`Setting lastTransitionTime for ServiceBinding "%v/%v" condition %q to %v`,
			toUpdate.Namespace, toUpdate.Name, conditionType, t)
		newCondition.LastTransitionTime = t
		toUpdate.Status.Conditions = []v1beta1.ServiceBindingCondition{newCondition}
		return
	}
	for i, cond := range toUpdate.Status.Conditions {
		if cond.Type == conditionType {
			if cond.Status != newCondition.Status {
				glog.V(3).Infof(`Found status change for ServiceBinding "%v/%v" condition %q: %q -> %q; setting lastTransitionTime to %v`,
					toUpdate.Namespace, toUpdate.Name, conditionType, cond.Status, status, t)
				newCondition.LastTransitionTime = t
			} else {
				newCondition.LastTransitionTime = cond.LastTransitionTime
			}

			toUpdate.Status.Conditions[i] = newCondition
			return
		}
	}

	glog.V(3).Infof("Setting lastTransitionTime for ServiceBinding '%v/%v' condition %q to %v",
		toUpdate.Namespace, toUpdate.Name, conditionType, t)

	newCondition.LastTransitionTime = t
	toUpdate.Status.Conditions = append(toUpdate.Status.Conditions, newCondition)
}

func (c *controller) updateServiceBindingStatus(toUpdate *v1beta1.ServiceBinding) (*v1beta1.ServiceBinding, error) {
	glog.V(4).Infof("Updating status for ServiceBinding %v/%v", toUpdate.Namespace, toUpdate.Name)
	updatedBinding, err := c.serviceCatalogClient.ServiceBindings(toUpdate.Namespace).UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf("Error updating status for ServiceBinding %v/%v", toUpdate.Namespace, toUpdate.Name)
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

	toUpdate, err := makeServiceBindingClone(binding)
	if err != nil {
		return err
	}

	c.setServiceBindingCondition(toUpdate, conditionType, status, reason, message)

	glog.V(4).Infof("Updating %v condition for ServiceBinding %v/%v to %v (Reason: %q, Message: %q)",
		conditionType, binding.Namespace, binding.Name, status, reason, message)
	_, err = c.serviceCatalogClient.ServiceBindings(binding.Namespace).UpdateStatus(toUpdate)
	if err != nil {
		glog.Errorf("Error updating %v condition for ServiceBinding %v/%v to %v: %v", conditionType, binding.Namespace, binding.Name, status, err)
	}
	return err
}

func (c *controller) bindingDelete(obj interface{}) {
	binding, ok := obj.(*v1beta1.ServiceBinding)
	if binding == nil || !ok {
		return
	}

	glog.V(4).Infof("Received delete event for ServiceBinding %v/%v; no further processing will occur", binding.Namespace, binding.Name)
}

// recordStartOfServiceBindingOperation updates the binding to indicate
// that there is a current operation being performed. The Status of the binding
// is recorded in the registry.
// params:
// toUpdate - a modifiable copy of the binding in the registry to update
// operation - operation that is being performed on the binding
// returns:
// 1 - a modifiable copy of toUpdate; or toUpdate if there was an error
// 2 - any error that occurred
func (c *controller) recordStartOfServiceBindingOperation(toUpdate *v1beta1.ServiceBinding, operation v1beta1.ServiceBindingOperation) (*v1beta1.ServiceBinding, error) {
	toUpdate.Status.CurrentOperation = operation
	now := metav1.Now()
	toUpdate.Status.OperationStartTime = &now
	reason := ""
	message := ""
	switch operation {
	case v1beta1.ServiceBindingOperationBind:
		reason = bindingInFlightReason
		message = bindingInFlightMessage
	case v1beta1.ServiceBindingOperationUnbind:
		reason = unbindingInFlightReason
		message = unbindingInFlightMessage
	}
	c.setServiceBindingCondition(
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
func (c *controller) clearServiceBindingCurrentOperation(toUpdate *v1beta1.ServiceBinding) {
	toUpdate.Status.CurrentOperation = ""
	toUpdate.Status.OperationStartTime = nil
	toUpdate.Status.ReconciledGeneration = toUpdate.Generation
	toUpdate.Status.InProgressProperties = nil
	toUpdate.Status.OrphanMitigationInProgress = false
}

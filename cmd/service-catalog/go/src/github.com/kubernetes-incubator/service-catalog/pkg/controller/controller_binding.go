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
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"github.com/kubernetes-incubator/service-catalog/pkg/api"
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
	errorEjectingBindMessage                  string = "Error ejecting binding."
	errorUnbindCallReason                     string = "UnbindCallFailed"
	errorNonbindableClusterServiceClassReason string = "ErrorNonbindableServiceClass"
	errorServiceInstanceNotReadyReason        string = "ErrorInstanceNotReady"
	errorServiceBindingOrphanMitigation       string = "ServiceBindingNeedsOrphanMitigation"
	errorFetchingBindingFailedReason          string = "FetchingBindingFailed"

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

// setAndUpdateServiceBindingStartOrphanMitigation is for setting the
// OrphanMitigationInProgress status to true, setting the proper condition
// statuses, and persisting the changes via updateServiceBindingStatus.
func (c *controller) setAndUpdateServiceBindingStartOrphanMitigation(toUpdate *v1beta1.ServiceBinding) error {
	pcb := pretty.NewContextBuilder(pretty.ServiceBinding, toUpdate.Name, toUpdate.Namespace)
	s := pcb.Message("Starting orphan mitigation")

	toUpdate.Status.OrphanMitigationInProgress = true
	toUpdate.Status.OperationStartTime = nil
	glog.V(5).Info(s)

	setServiceBindingCondition(
		toUpdate,
		v1beta1.ServiceBindingConditionReady,
		v1beta1.ConditionFalse,
		errorServiceBindingOrphanMitigation,
		s,
	)

	c.recorder.Event(toUpdate, corev1.EventTypeWarning, errorServiceBindingOrphanMitigation, s)
	if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
		return err
	}
	return nil
}

// an error is returned to indicate that the binding has not been
// fully processed and should be resubmitted at a later time.
func (c *controller) reconcileServiceBinding(binding *v1beta1.ServiceBinding) error {
	pcb := pretty.NewContextBuilder(pretty.ServiceBinding, binding.Namespace, binding.Name)
	glog.V(6).Info(pcb.Messagef(`beginning to process resourceVersion: %v`, binding.ResourceVersion))

	if binding.Status.AsyncOpInProgress {
		return c.pollServiceBinding(binding)
	}

	if isServiceBindingFailed(binding) && binding.ObjectMeta.DeletionTimestamp == nil && !binding.Status.OrphanMitigationInProgress {
		glog.V(4).Info(pcb.Message("not processing event; status showed that it has failed"))
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
			glog.V(4).Info(pcb.Message("Not processing event; reconciled generation showed there is no work to do"))
			return nil
		}
	}
	if binding.DeletionTimestamp != nil || binding.Status.OrphanMitigationInProgress {
		return c.reconcileServiceBindingDelete(binding)
	}

	glog.V(4).Info(pcb.Message("Processing"))

	toUpdate, err := makeServiceBindingClone(binding)
	if err != nil {
		return err
	}

	instance, err := c.instanceLister.ServiceInstances(binding.Namespace).Get(binding.Spec.ServiceInstanceRef.Name)
	if err != nil {
		s := fmt.Sprintf(
			`References a non-existent %s "%s/%s"`,
			pretty.ServiceInstance, binding.Namespace, binding.Spec.ServiceInstanceRef.Name,
		)
		glog.Warningf(pcb.Messagef("%s (%s)", s, err))
		c.recorder.Event(binding, corev1.EventTypeWarning, errorNonexistentServiceInstanceReason, s)
		setServiceBindingCondition(
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
			`trying to bind to %s "%s/%s" that has ongoing asynchronous operation`,
			pretty.ServiceInstance, binding.Namespace, binding.Spec.ServiceInstanceRef.Name,
		)
		glog.Info(pcb.Message(s))
		c.recorder.Event(binding, corev1.EventTypeWarning, errorWithOngoingAsyncOperation, s)
		setServiceBindingCondition(
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
			`References a non-bindable %s and Plan (%q) combination`,
			pretty.ClusterServiceClassName(serviceClass), instance.Spec.ClusterServicePlanExternalName,
		)
		glog.Warning(pcb.Message(s))
		c.recorder.Event(binding, corev1.EventTypeWarning, errorNonbindableClusterServiceClassReason, s)
		setServiceBindingCondition(
			toUpdate,
			v1beta1.ServiceBindingConditionReady,
			v1beta1.ConditionFalse,
			errorNonbindableClusterServiceClassReason,
			s,
		)
		setServiceBindingCondition(
			toUpdate,
			v1beta1.ServiceBindingConditionFailed,
			v1beta1.ConditionTrue,
			errorNonbindableClusterServiceClassReason,
			s,
		)
		clearServiceBindingCurrentOperation(toUpdate)
		if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
			return err
		}
		return nil
	}

	if binding.DeletionTimestamp == nil && !binding.Status.OrphanMitigationInProgress { // Add or update
		glog.V(4).Info(pcb.Message("Adding/Updating"))

		ns, err := c.kubeClient.CoreV1().Namespaces().Get(instance.Namespace, metav1.GetOptions{})
		if err != nil {
			s := fmt.Sprintf(`Failed to get namespace %q during binding: %s`, instance.Namespace, err)
			glog.Info(pcb.Message(s))
			c.recorder.Eventf(binding, corev1.EventTypeWarning, errorFindingNamespaceServiceInstanceReason, s)
			setServiceBindingCondition(
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
			s := fmt.Sprintf(
				`ServiceBinding cannot begin because referenced %s is not ready`,
				pretty.ServiceInstanceName(instance),
			)
			glog.Info(pcb.Message(s))
			c.recorder.Eventf(binding, corev1.EventTypeWarning, errorServiceInstanceNotReadyReason, s)
			setServiceBindingCondition(
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

		parameters, parametersChecksum, rawParametersWithRedaction, err := prepareInProgressPropertyParameters(
			c.kubeClient,
			binding.Namespace,
			binding.Spec.Parameters,
			binding.Spec.ParametersFrom,
		)
		if err != nil {
			glog.Warning(pcb.Message(err.Error()))
			c.recorder.Event(toUpdate, corev1.EventTypeWarning, errorWithParameters, err.Error())
			setServiceBindingCondition(
				toUpdate,
				v1beta1.ServiceBindingConditionReady,
				v1beta1.ConditionFalse,
				errorWithParameters,
				err.Error(),
			)
			if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
				return err
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
				s := fmt.Sprintf(`Error building originating identity headers for binding: %v`, err)
				glog.Warning(pcb.Message(s))
				c.recorder.Event(binding, corev1.EventTypeWarning, errorWithOriginatingIdentity, s)
				setServiceBindingCondition(
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

		toUpdate.Status.UnbindStatus = v1beta1.ServiceBindingUnbindStatusRequired

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
			setServiceBindingCondition(
				toUpdate,
				v1beta1.ServiceBindingConditionFailed,
				v1beta1.ConditionTrue,
				errorBindCallReason,
				"Communication with the ServiceBroker timed out; Bind operation will not be retried: "+err.Error(),
			)
			return c.setAndUpdateServiceBindingStartOrphanMitigation(toUpdate)
		} else if err != nil {
			if httpErr, ok := osb.IsHTTPError(err); ok {
				// orphan mitigation: looking for 2xx (excluding 200), 408, 5xx
				if httpErr.StatusCode > 200 && httpErr.StatusCode < 300 ||
					httpErr.StatusCode == http.StatusRequestTimeout ||
					httpErr.StatusCode >= 500 && httpErr.StatusCode < 600 {
					setServiceBindingCondition(
						toUpdate,
						v1beta1.ServiceBindingConditionFailed,
						v1beta1.ConditionTrue,
						errorBindCallReason,
						"ServiceBroker returned a failure; Bind operation will not be retried: "+err.Error(),
					)
					return c.setAndUpdateServiceBindingStartOrphanMitigation(toUpdate)
				}
				s := fmt.Sprintf(
					`Error creating ServiceBinding for %s: %v`,
					pretty.FromServiceInstanceOfClusterServiceClassAtBrokerName(instance, serviceClass, brokerName), httpErr.Error(),
				)
				glog.Warning(pcb.Message(s))
				c.recorder.Event(binding, corev1.EventTypeWarning, errorBindCallReason, s)

				setServiceBindingCondition(
					toUpdate,
					v1beta1.ServiceBindingConditionFailed,
					v1beta1.ConditionTrue,
					"ServiceBindingReturnedFailure",
					s,
				)
				setServiceBindingCondition(
					toUpdate,
					v1beta1.ServiceBindingConditionReady,
					v1beta1.ConditionFalse,
					errorBindCallReason,
					"Bind call failed. "+s)
				clearServiceBindingCurrentOperation(toUpdate)
				if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
					return err
				}
				return nil
			}

			s := fmt.Sprintf(
				`Error creating ServiceBinding for %s: %s`,
				pretty.FromServiceInstanceOfClusterServiceClassAtBrokerName(instance, serviceClass, brokerName), err,
			)
			glog.Warning(pcb.Message(s))
			c.recorder.Event(binding, corev1.EventTypeWarning, errorBindCallReason, s)
			setServiceBindingCondition(
				toUpdate,
				v1beta1.ServiceBindingConditionReady,
				v1beta1.ConditionFalse,
				errorBindCallReason,
				"Bind call failed. "+s)

			if !time.Now().Before(toUpdate.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
				s := "Stopping reconciliation retries, too much time has elapsed"
				glog.Info(pcb.Message(s))
				c.recorder.Event(binding, corev1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)
				setServiceBindingCondition(toUpdate,
					v1beta1.ServiceBindingConditionFailed,
					v1beta1.ConditionTrue,
					errorReconciliationRetryTimeoutReason,
					s)
				clearServiceBindingCurrentOperation(toUpdate)
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

		if response.Async {
			glog.Info(pcb.Message("Received asynchronous bind response"))

			if response.OperationKey != nil && *response.OperationKey != "" {
				key := string(*response.OperationKey)
				toUpdate.Status.LastOperation = &key
			}

			toUpdate.Status.AsyncOpInProgress = true

			setServiceBindingCondition(
				toUpdate,
				v1beta1.ServiceBindingConditionReady,
				v1beta1.ConditionFalse,
				asyncBindingReason,
				asyncBindingMessage,
			)

			if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
				return err
			}

			if err := c.beginPollingServiceBinding(binding); err != nil {
				return err
			}

			c.recorder.Eventf(binding, corev1.EventTypeNormal, asyncBindingReason, asyncBindingMessage)

			return nil
		}

		// The Bind request has returned successfully from the Broker. Continue
		// with the success case of creating the ServiceBinding.

		// Save off the external properties here even if the subsequent
		// credentials injection fails. The Broker has already processed the
		// request, so this is what the Broker knows about the state of the
		// binding.
		toUpdate.Status.ExternalProperties = toUpdate.Status.InProgressProperties
		toUpdate.Status.InProgressProperties = nil

		err = c.injectServiceBinding(binding, response.Credentials)
		if err != nil {
			s := fmt.Sprintf(`Error injecting binding results: %s`, err)
			glog.Warning(pcb.Message(s))
			c.recorder.Event(binding, corev1.EventTypeWarning, errorInjectingBindResultReason, s)
			setServiceBindingCondition(
				toUpdate,
				v1beta1.ServiceBindingConditionReady,
				v1beta1.ConditionFalse,
				errorInjectingBindResultReason,
				"Error injecting bind result "+s,
			)

			if !time.Now().Before(toUpdate.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
				s := fmt.Sprint(pcb.Message("Stopping reconciliation retries, too much time has elapsed"))
				glog.Info(pcb.Message(s))
				c.recorder.Event(binding, corev1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)
				setServiceBindingCondition(toUpdate,
					v1beta1.ServiceBindingConditionFailed,
					v1beta1.ConditionTrue,
					errorReconciliationRetryTimeoutReason,
					s)
				return c.setAndUpdateServiceBindingStartOrphanMitigation(toUpdate)
			}

			if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
				return err
			}
			// TODO: solve scenario where bind request successful, credential injection fails, later reconciliations have non-failing errors
			// with Bind request. After retry duration, reconciler gives up but will not do orphan mitigation.
			return err
		}

		clearServiceBindingCurrentOperation(toUpdate)

		setServiceBindingCondition(
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
		glog.V(5).Info(pcb.Messagef(
			`Successfully bound to %s`,
			pretty.FromServiceInstanceOfClusterServiceClassAtBrokerName(instance, serviceClass, brokerName),
		))

		return nil
	}

	return nil
}

func (c *controller) reconcileServiceBindingDelete(binding *v1beta1.ServiceBinding) error {
	// All updates having a DeletingTimestamp will have been handled here.
	// We're dealing with an update that's actually a soft delete-- i.e. we
	// have some finalization to do.

	if binding.DeletionTimestamp == nil && !binding.Status.OrphanMitigationInProgress {
		// nothing to do...
		return nil
	}

	pcb := pretty.NewContextBuilder(pretty.ServiceBinding, binding.Namespace, binding.Name)
	glog.V(4).Info(pcb.Message("Processing Delete"))

	finalizerToken := v1beta1.FinalizerServiceCatalog
	finalizers := sets.NewString(binding.Finalizers...)
	if !finalizers.Has(finalizerToken) {
		return nil
	}

	// If unbind has failed, do not do anything more
	if binding.Status.UnbindStatus == v1beta1.ServiceBindingUnbindStatusFailed {
		glog.V(4).Info(pcb.Message("Not processing delete event because unbinding has failed"))
		return nil
	}

	toUpdate, err := makeServiceBindingClone(binding)
	if err != nil {
		return err
	}

	// If unbinding succeeded or is not needed, then clear out the finalizers
	if binding.Status.UnbindStatus == v1beta1.ServiceBindingUnbindStatusNotRequired ||
		binding.Status.UnbindStatus == v1beta1.ServiceBindingUnbindStatusSucceeded {

		glog.V(5).Info(pcb.Message("Clearing catalog finalizer"))

		// Clear the finalizer
		finalizers.Delete(v1beta1.FinalizerServiceCatalog)
		toUpdate.Finalizers = finalizers.List()

		if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
			return err
		}
		return nil
	}

	err = c.ejectServiceBinding(binding)
	if err != nil {
		s := fmt.Sprintf(`Error deleting secret: %s`, err)
		glog.Warning(pcb.Message(s))
		c.recorder.Eventf(binding, corev1.EventTypeWarning, errorEjectingBindReason, "%v %v", errorEjectingBindMessage, s)
		setServiceBindingCondition(
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

	if toUpdate.DeletionTimestamp == nil {
		if toUpdate.Status.OperationStartTime == nil {
			now := metav1.Now()
			toUpdate.Status.OperationStartTime = &now
		}
	} else {
		if toUpdate.Status.CurrentOperation != v1beta1.ServiceBindingOperationUnbind {
			// Cancel any pending orphan mitigation since the resource is being deleted
			toUpdate.Status.OrphanMitigationInProgress = false

			toUpdate, err = c.recordStartOfServiceBindingOperation(toUpdate, v1beta1.ServiceBindingOperationUnbind)
			if err != nil {
				// There has been an update to the binding. Start reconciliation
				// over with a fresh view of the binding.
				return err
			}
		}
	}

	if binding.Status.UnbindStatus == v1beta1.ServiceBindingUnbindStatusRequired {
		if ok, err := c.serviceBindingRequestUnbinding(binding, toUpdate, pcb); !ok || err != nil {
			return err
		}
	}

	if toUpdate.Status.OrphanMitigationInProgress {
		s := "Orphan mitigation successful"
		setServiceBindingCondition(toUpdate,
			v1beta1.ServiceBindingConditionReady,
			v1beta1.ConditionFalse,
			successOrphanMitigationReason,
			s)
	} else {
		s := "The binding was deleted successfully"
		setServiceBindingCondition(
			toUpdate,
			v1beta1.ServiceBindingConditionReady,
			v1beta1.ConditionFalse,
			successUnboundReason,
			s,
		)
		// Clear the finalizer
		finalizers.Delete(v1beta1.FinalizerServiceCatalog)
		toUpdate.Finalizers = finalizers.List()
	}

	toUpdate.Status.ExternalProperties = nil
	clearServiceBindingCurrentOperation(toUpdate)
	if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
		return err
	}

	c.recorder.Event(binding, corev1.EventTypeNormal, successUnboundReason, "This binding was deleted successfully")
	glog.V(5).Info(pcb.Message("Successfully deleted ServiceBinding"))

	return nil
}

// serviceBindingRequestUnbinding validates and makes the binding request to the broker.
// Returns if reconciliation should continue and any error produced.
func (c *controller) serviceBindingRequestUnbinding(binding *v1beta1.ServiceBinding, toUpdate *v1beta1.ServiceBinding, pcb *pretty.ContextBuilder) (bool, error) {
	glog.V(4).Info(pcb.Message("Going to make request to unbind"))
	instance, err := c.instanceLister.ServiceInstances(binding.Namespace).Get(binding.Spec.ServiceInstanceRef.Name)
	if err != nil {
		s := fmt.Sprintf(
			`References a non-existent %s "%s/%s"`,
			pretty.ServiceInstance, binding.Namespace, binding.Spec.ServiceInstanceRef.Name,
		)
		glog.Warningf(pcb.Messagef("%s (%s)", s, err))
		c.recorder.Event(binding, corev1.EventTypeWarning, errorNonexistentServiceInstanceReason, s)
		setServiceBindingCondition(
			toUpdate,
			v1beta1.ServiceBindingConditionReady,
			v1beta1.ConditionFalse,
			errorNonexistentServiceInstanceReason,
			"The binding references an ServiceInstance that does not exist. "+s,
		)
		if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
			return false, err

		}
		return false, err
	}

	if instance.Status.AsyncOpInProgress {
		s := fmt.Sprintf(
			`trying to unbind to %s "%s/%s" that has ongoing asynchronous operation`,
			pretty.ServiceInstance, binding.Namespace, binding.Spec.ServiceInstanceRef.Name,
		)
		glog.Info(pcb.Message(s))
		c.recorder.Event(binding, corev1.EventTypeWarning, errorWithOngoingAsyncOperation, s)
		setServiceBindingCondition(
			toUpdate,
			v1beta1.ServiceBindingConditionReady,
			v1beta1.ConditionFalse,
			errorWithOngoingAsyncOperation,
			errorWithOngoingAsyncOperationMessage,
		)
		if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
			return false, err
		}
		return false, fmt.Errorf("Ongoing Asynchronous operation")
	}

	if instance.Spec.ClusterServiceClassRef == nil || instance.Spec.ClusterServicePlanRef == nil {
		// TODO(#1562): ultimately here we need to use logic similar to what is done to determine the plan ID for
		// deprovisioning. We need to allow a ServiceBinding to be deleted, with an unbind request sent to the broker,
		// even if the ServiceInstance has been changed to a non-existent plan.
		return false, fmt.Errorf("ClusterServiceClass or ClusterServicePlan references for Instance have not been resolved yet")
	}

	serviceClass, servicePlan, brokerName, brokerClient, err := c.getClusterServiceClassPlanAndClusterServiceBrokerForServiceBinding(instance, binding)
	if err != nil {
		return false, err // retry later
	}

	unbindRequest := &osb.UnbindRequest{
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

		unbindRequest.AcceptsIncomplete = true
	}

	if utilfeature.DefaultFeatureGate.Enabled(scfeatures.OriginatingIdentity) {
		originatingIdentity, err := buildOriginatingIdentity(binding.Spec.UserInfo)
		if err != nil {
			s := fmt.Sprintf(`Error building originating identity headers while unbinding: %v`, err)
			glog.Warning(pcb.Message(s))
			c.recorder.Event(binding, corev1.EventTypeWarning, errorWithOriginatingIdentity, s)
			setServiceBindingCondition(
				toUpdate,
				v1beta1.ServiceBindingConditionReady,
				v1beta1.ConditionFalse,
				errorWithOriginatingIdentity,
				s,
			)
			if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
				return false, err
			}
			return false, err
		}
		unbindRequest.OriginatingIdentity = originatingIdentity
	}

	response, err := brokerClient.Unbind(unbindRequest)
	if err != nil {
		if httpErr, ok := osb.IsHTTPError(err); ok {
			s := fmt.Sprintf(
				`Error unbinding from %s: %s`,
				pretty.FromServiceInstanceOfClusterServiceClassAtBrokerName(instance, serviceClass, brokerName), httpErr.Error(),
			)
			glog.Warning(pcb.Message(s))
			c.recorder.Event(binding, corev1.EventTypeWarning, errorUnbindCallReason, s)
			setServiceBindingCondition(
				toUpdate,
				v1beta1.ServiceBindingConditionReady,
				v1beta1.ConditionUnknown,
				errorUnbindCallReason,
				"Unbind call failed. "+s)
			if !toUpdate.Status.OrphanMitigationInProgress {
				setServiceBindingCondition(
					toUpdate,
					v1beta1.ServiceBindingConditionFailed,
					v1beta1.ConditionTrue,
					errorUnbindCallReason,
					"Unbind call failed. "+s)
			}
			clearServiceBindingCurrentOperation(toUpdate)
			toUpdate.Status.UnbindStatus = v1beta1.ServiceBindingUnbindStatusFailed
			if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
				return false, err
			}
			return false, nil
		}
		s := fmt.Sprintf(
			`Error unbinding from %s: %s`,
			pretty.FromServiceInstanceOfClusterServiceClassAtBrokerName(instance, serviceClass, brokerName), err,
		)
		glog.Warning(pcb.Message(s))
		c.recorder.Event(binding, corev1.EventTypeWarning, errorUnbindCallReason, s)
		setServiceBindingCondition(
			toUpdate,
			v1beta1.ServiceBindingConditionReady,
			v1beta1.ConditionUnknown,
			errorUnbindCallReason,
			"Unbind call failed. "+s)

		if !time.Now().Before(toUpdate.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
			if toUpdate.Status.OrphanMitigationInProgress {
				s := "Stopping reconciliation retries, too much time has elapsed during orphan mitigation"
				glog.Info(pcb.Message(s))
				c.recorder.Event(binding, corev1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)
			} else {
				s := "Stopping reconciliation retries, too much time has elapsed"
				glog.Info(pcb.Message(s))
				c.recorder.Event(binding, corev1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)
				setServiceBindingCondition(toUpdate,
					v1beta1.ServiceBindingConditionFailed,
					v1beta1.ConditionTrue,
					errorReconciliationRetryTimeoutReason,
					s)
			}
			clearServiceBindingCurrentOperation(toUpdate)
			toUpdate.Status.UnbindStatus = v1beta1.ServiceBindingUnbindStatusFailed
			if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
				return false, err
			}
			return false, nil
		}

		if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
			return false, err
		}
		return false, err
	}

	if response.Async {
		glog.Info(pcb.Message("Received asynchronous unbind response"))

		if response.OperationKey != nil && *response.OperationKey != "" {
			key := string(*response.OperationKey)
			toUpdate.Status.LastOperation = &key
		}

		toUpdate.Status.AsyncOpInProgress = true

		setServiceBindingCondition(
			toUpdate,
			v1beta1.ServiceBindingConditionReady,
			v1beta1.ConditionFalse,
			asyncUnbindingReason,
			asyncUnbindingMessage,
		)

		if _, err := c.updateServiceBindingStatus(toUpdate); err != nil {
			return false, err
		}

		if err := c.beginPollingServiceBinding(binding); err != nil {
			return false, err
		}

		c.recorder.Eventf(binding, corev1.EventTypeNormal, asyncUnbindingReason, asyncUnbindingMessage)

		return false, nil
	}
	toUpdate.Status.UnbindStatus = v1beta1.ServiceBindingUnbindStatusSucceeded
	return true, nil
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

	toUpdate, err := makeServiceBindingClone(binding)
	if err != nil {
		return err
	}

	setServiceBindingCondition(toUpdate, conditionType, status, reason, message)

	glog.V(4).Info(pcb.Messagef(
		"Updating %v condition to %v (Reason: %q, Message: %q)",
		conditionType, status, reason, message,
	))
	_, err = c.serviceCatalogClient.ServiceBindings(binding.Namespace).UpdateStatus(toUpdate)
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

	binding, err := makeServiceBindingClone(binding)
	if err != nil {
		return err
	}

	instance, err := c.instanceLister.ServiceInstances(binding.Namespace).Get(binding.Spec.ServiceInstanceRef.Name)
	if err != nil {
		return fmt.Errorf("could not get instance for ServiceBinding %v/%v", binding.Namespace, binding.Name)
	}

	serviceClass, servicePlan, _, brokerClient, err := c.getClusterServiceClassPlanAndClusterServiceBrokerForServiceBinding(instance, binding)
	if err != nil {
		return err
	}

	// There are some conditions that are different if we're
	// deleting or mitigating an orphan; this is more readable than
	// checking the timestamps in various places.
	mitigatingOrphan := binding.Status.OrphanMitigationInProgress
	deleting := false
	if binding.Status.CurrentOperation == v1beta1.ServiceBindingOperationUnbind || mitigatingOrphan {
		deleting = true
	}

	if binding.Status.OperationStartTime == nil {
		s := "Stopping reconciliation retries because the operation start time is not set"
		glog.Info(pcb.Message(s))
		c.recorder.Event(binding, corev1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)

		if mitigatingOrphan {
			setServiceBindingCondition(
				binding,
				v1beta1.ServiceBindingConditionReady,
				v1beta1.ConditionUnknown,
				errorOrphanMitigationFailedReason,
				"Orphan mitigation failed: "+s,
			)
		} else {
			setServiceBindingCondition(
				binding,
				v1beta1.ServiceBindingConditionFailed,
				v1beta1.ConditionTrue,
				errorReconciliationRetryTimeoutReason,
				s,
			)
		}

		if deleting {
			clearServiceBindingCurrentOperation(binding)
			binding.Status.UnbindStatus = v1beta1.ServiceBindingUnbindStatusFailed

			if _, err := c.updateServiceBindingStatus(binding); err != nil {
				return err
			}
		} else {
			if err := c.setAndUpdateServiceBindingStartOrphanMitigation(binding); err != nil {
				return err
			}
		}

		return c.finishPollingServiceBinding(binding)
	}

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
			s := fmt.Sprintf("Error building originating identity headers for polling last operation: %v", err)
			glog.Warningf(pcb.Message(s))
			c.recorder.Event(binding, corev1.EventTypeWarning, errorWithOriginatingIdentity, s)
			setServiceBindingCondition(
				binding,
				v1beta1.ServiceBindingConditionReady,
				v1beta1.ConditionFalse,
				errorWithOriginatingIdentity,
				s,
			)
			if _, err := c.updateServiceBindingStatus(binding); err != nil {
				return err
			}
			return err
		}
		request.OriginatingIdentity = originatingIdentity
	}

	glog.V(5).Info(pcb.Message("Polling last operation"))

	response, err := brokerClient.PollBindingLastOperation(request)
	if err != nil {
		// If the operation was for delete and we receive a http.StatusGone,
		// this is considered a success as per the spec, so mark as deleted
		// and remove any finalizers.
		if osb.IsGoneError(err) && deleting {
			var (
				reason  string
				message string
			)
			switch {
			case mitigatingOrphan:
				reason = successOrphanMitigationReason
				message = successOrphanMitigationMessage
			default:
				reason = successUnboundReason
				message = "The binding was deleted successfully"
			}

			clearServiceBindingCurrentOperation(binding)
			binding.Status.UnbindStatus = v1beta1.ServiceBindingUnbindStatusSucceeded
			binding.Status.ExternalProperties = nil

			setServiceBindingCondition(
				binding,
				v1beta1.ServiceBindingConditionReady,
				v1beta1.ConditionFalse,
				reason,
				message,
			)

			if !mitigatingOrphan {
				// Clear the finalizer
				if finalizers := sets.NewString(binding.Finalizers...); finalizers.Has(v1beta1.FinalizerServiceCatalog) {
					finalizers.Delete(v1beta1.FinalizerServiceCatalog)
					binding.Finalizers = finalizers.List()
				}
			}

			if _, err := c.updateServiceBindingStatus(binding); err != nil {
				return err
			}

			c.recorder.Event(binding, corev1.EventTypeNormal, reason, message)
			glog.V(4).Info(pcb.Message(message))

			return c.finishPollingServiceBinding(binding)
		}

		// We got some kind of error from the broker.  While polling last
		// operation, this represents an invalid response and we should
		// continue polling last operation.
		//
		// The ready condition on the binding should already have
		// condition false; it should be sufficient to create an event for
		// the instance.
		errText := ""
		if httpErr, ok := osb.IsHTTPError(err); ok {
			errText = httpErr.Error()
		} else {
			errText = err.Error()
		}

		s := fmt.Sprintf("Error polling last operation: %v", errText)
		glog.V(4).Info(pcb.Message(s))
		c.recorder.Event(binding, corev1.EventTypeWarning, errorPollingLastOperationReason, s)

		if c.isServiceBindingReconciliationRetryDurationExceeded(binding) {
			return c.reconciliationRetryDurationExceededFinishPollingServiceBinding(binding)
		}

		return c.continuePollingServiceBinding(binding)
	}

	glog.V(4).Info(pcb.Messagef("Poll returned %q : %q", response.State, response.Description))

	switch response.State {
	case osb.StateInProgress:
		// if the description is non-nil, then update the instance condition with it
		if response.Description != nil {
			var (
				message string
				reason  string
			)
			if deleting {
				reason = asyncUnbindingReason
				message = asyncUnbindingMessage
			} else {
				reason = asyncBindingReason
				message = asyncBindingMessage
			}

			message = fmt.Sprintf("%s (%s)", message, *response.Description)

			setServiceBindingCondition(
				binding,
				v1beta1.ServiceBindingConditionReady,
				v1beta1.ConditionFalse,
				reason,
				message,
			)
		}

		if c.isServiceBindingReconciliationRetryDurationExceeded(binding) {
			return c.reconciliationRetryDurationExceededFinishPollingServiceBinding(binding)
		}

		if _, err := c.updateServiceBindingStatus(binding); err != nil {
			return err
		}

		glog.V(4).Info(pcb.Message("Last operation not completed (still in progress)"))

		return c.continuePollingServiceBinding(binding)
	case osb.StateSucceeded:
		if deleting {
			var (
				reason  string
				message string
			)
			switch {
			case mitigatingOrphan:
				reason = successOrphanMitigationReason
				message = successOrphanMitigationMessage
			default:
				reason = successUnboundReason
				message = "The binding was deleted successfully"
			}

			clearServiceBindingCurrentOperation(binding)
			binding.Status.UnbindStatus = v1beta1.ServiceBindingUnbindStatusSucceeded
			binding.Status.ExternalProperties = nil

			setServiceBindingCondition(
				binding,
				v1beta1.ServiceBindingConditionReady,
				v1beta1.ConditionFalse,
				reason,
				message,
			)

			if !mitigatingOrphan {
				// Clear the finalizer
				if finalizers := sets.NewString(binding.Finalizers...); finalizers.Has(v1beta1.FinalizerServiceCatalog) {
					finalizers.Delete(v1beta1.FinalizerServiceCatalog)
					binding.Finalizers = finalizers.List()
				}
			}

			if _, err := c.updateServiceBindingStatus(binding); err != nil {
				return err
			}

			c.recorder.Event(binding, corev1.EventTypeNormal, reason, message)
			glog.V(4).Info(pcb.Message(message))

			return c.finishPollingServiceBinding(binding)
		}

		// Update the in progress/external properties, as the changes have been
		// persisted in the broker
		binding.Status.ExternalProperties = binding.Status.InProgressProperties
		binding.Status.InProgressProperties = nil

		getBindingRequest := &osb.GetBindingRequest{
			InstanceID: instance.Spec.ExternalID,
			BindingID:  binding.Spec.ExternalID,
		}

		getBindingResponse, err := brokerClient.GetBinding(getBindingRequest)
		if err != nil {
			s := fmt.Sprintf("Could not do a GET on binding resource: %v", err)
			glog.Warning(pcb.Message(s))
			c.recorder.Event(binding, corev1.EventTypeWarning, errorFetchingBindingFailedReason, s)

			setServiceBindingCondition(
				binding,
				v1beta1.ServiceBindingConditionFailed,
				v1beta1.ConditionTrue,
				errorFetchingBindingFailedReason,
				s,
			)

			if err := c.setAndUpdateServiceBindingStartOrphanMitigation(binding); err != nil {
				return err
			}

			return c.finishPollingServiceBinding(binding)
		}

		if err := c.injectServiceBinding(binding, getBindingResponse.Credentials); err != nil {
			s := fmt.Sprintf("Error injecting bind results: %v", err)
			glog.Warning(pcb.Message(s))
			c.recorder.Event(binding, corev1.EventTypeWarning, errorInjectingBindResultReason, s)

			setServiceBindingCondition(
				binding,
				v1beta1.ServiceBindingConditionFailed,
				v1beta1.ConditionTrue,
				errorInjectingBindResultReason,
				s,
			)

			if err := c.setAndUpdateServiceBindingStartOrphanMitigation(binding); err != nil {
				return err
			}

			return c.finishPollingServiceBinding(binding)
		}

		glog.V(4).Info(pcb.Message(successInjectedBindResultMessage))
		c.recorder.Event(binding, corev1.EventTypeNormal, successInjectedBindResultReason, successInjectedBindResultMessage)

		setServiceBindingCondition(
			binding,
			v1beta1.ServiceBindingConditionReady,
			v1beta1.ConditionTrue,
			successInjectedBindResultReason,
			successInjectedBindResultMessage,
		)
		clearServiceBindingCurrentOperation(binding)

		if _, err := c.updateServiceBindingStatus(binding); err != nil {
			return err
		}

		return c.finishPollingServiceBinding(binding)
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
			reason = errorUnbindCallReason
			message = "Unbind call failed: " + description
		default:
			readyCond = v1beta1.ConditionFalse
			reason = errorBindCallReason
			message = "Bind call failed: " + description
		}

		glog.Warning(pcb.Message(message))
		c.recorder.Event(binding, corev1.EventTypeWarning, reason, message)

		setServiceBindingCondition(
			binding,
			v1beta1.ServiceBindingConditionReady,
			readyCond,
			reason,
			message,
		)

		if !mitigatingOrphan {
			setServiceBindingCondition(
				binding,
				v1beta1.ServiceBindingConditionFailed,
				v1beta1.ConditionTrue,
				reason,
				message,
			)
		}

		clearServiceBindingCurrentOperation(binding)

		if deleting {
			binding.Status.UnbindStatus = v1beta1.ServiceBindingUnbindStatusFailed
		}

		if _, err := c.updateServiceBindingStatus(binding); err != nil {
			return err
		}

		return c.finishPollingServiceBinding(binding)
	default:
		glog.Warning(pcb.Messagef("Got invalid state in LastOperationResponse: %q", response.State))

		if c.isServiceBindingReconciliationRetryDurationExceeded(binding) {
			return c.reconciliationRetryDurationExceededFinishPollingServiceBinding(binding)
		}

		return c.continuePollingServiceBinding(binding)
	}
}

// reconciliationTimeExpired tests if the current Operation State time has
// elapsed the reconciliationRetryDuration time period
func (c *controller) isServiceBindingReconciliationRetryDurationExceeded(binding *v1beta1.ServiceBinding) bool {
	if time.Now().After(binding.Status.OperationStartTime.Time.Add(c.reconciliationRetryDuration)) {
		return true
	}
	return false
}

// reconciliationRetryDurationExceededFinishPollingServiceBinding marks the
// binding as failed due to the reconciliation retry duration having been
// exceeded.
//
// The binding resource passed will be directly modified, so make sure it is
// not directly from the cache.
func (c *controller) reconciliationRetryDurationExceededFinishPollingServiceBinding(binding *v1beta1.ServiceBinding) error {
	pcb := pretty.NewContextBuilder(pretty.ServiceBinding, binding.Namespace, binding.Name)

	mitigatingOrphan := binding.Status.OrphanMitigationInProgress
	deleting := false
	if binding.Status.CurrentOperation == v1beta1.ServiceBindingOperationUnbind || mitigatingOrphan {
		deleting = true
	}

	s := "Stopping reconciliation retries because too much time has elapsed"
	glog.Infof(pcb.Message(s))
	c.recorder.Event(binding, corev1.EventTypeWarning, errorReconciliationRetryTimeoutReason, s)

	if mitigatingOrphan {
		setServiceBindingCondition(
			binding,
			v1beta1.ServiceBindingConditionReady,
			v1beta1.ConditionUnknown,
			errorOrphanMitigationFailedReason,
			"Orphan mitigation failed: "+s,
		)
	} else {
		setServiceBindingCondition(
			binding,
			v1beta1.ServiceBindingConditionFailed,
			v1beta1.ConditionTrue,
			errorReconciliationRetryTimeoutReason,
			s,
		)
	}

	if deleting {
		clearServiceBindingCurrentOperation(binding)
		binding.Status.UnbindStatus = v1beta1.ServiceBindingUnbindStatusFailed

		if _, err := c.updateServiceBindingStatus(binding); err != nil {
			return err
		}
	} else {
		if err := c.setAndUpdateServiceBindingStartOrphanMitigation(binding); err != nil {
			return err
		}
	}

	return c.finishPollingServiceBinding(binding)
}

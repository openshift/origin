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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	scmeta "github.com/kubernetes-incubator/service-catalog/pkg/api/meta"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	v1beta1informers "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/externalversions/servicecatalog/v1beta1"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	fakeosb "github.com/pmorie/go-open-service-broker-client/v2/fake"
	corev1 "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"github.com/kubernetes-incubator/service-catalog/pkg/api"
	scfeatures "github.com/kubernetes-incubator/service-catalog/pkg/features"
	clientgofake "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
)

// TestReconcileBindingNonExistingInstance tests reconcileBinding to ensure a
// binding fails as expected when an instance to bind to doesn't exist.
func TestReconcileServiceBindingNonExistingServiceInstance(t *testing.T) {
	_, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, _ := newTestController(t, noFakeActions())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testNonExistentClusterServiceClassName},
			ExternalID:         testServiceBindingGUID,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	err := testController.reconcileServiceBinding(binding)
	if err == nil {
		t.Fatal("binding nothere was found and it should not be found")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says it failed because no such instance exists.
	updateAction := actions[0].(clientgotesting.UpdateAction)
	if e, a := "update", updateAction.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on actions[0]; %s", expectedGot(e, a))
	}
	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingErrorBeforeRequest(t, updatedServiceBinding, errorNonexistentServiceInstanceReason, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := warningEventBuilder(errorNonexistentServiceInstanceReason).msgf(
		"References a non-existent ServiceInstance %q",
		"/"+testNonExistentClusterServiceClassName,
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceBindingUnresolvedClusterServiceClassReference
// tests reconcileBinding to ensure a binding fails when a ClusterServiceClassRef has not been resolved.
func TestReconcileServiceBindingUnresolvedClusterServiceClassReference(t *testing.T) {
	_, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	instance := &v1beta1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceName, Namespace: testNamespace},
		Spec: v1beta1.ServiceInstanceSpec{
			PlanReference: v1beta1.PlanReference{
				ClusterServiceClassExternalName: testNonExistentClusterServiceClassName,
				ClusterServicePlanExternalName:  testClusterServicePlanName,
			},
			ExternalID: testServiceInstanceGUID,
		},
	}
	sharedInformers.ServiceInstances().Informer().GetStore().Add(instance)
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	err := testController.reconcileServiceBinding(binding)
	if err == nil {
		t.Fatal("serviceclassref was nil and reconcile should return an error")
	}
	if !strings.Contains(err.Error(), "not been resolved yet") {
		t.Fatalf("Did not get the expected error: %s", expectedGot("not been resolved yet", err))
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	// There are no actions.
	assertNumberOfActions(t, actions, 0)
}

// TestReconcileServiceBindingUnresolvedClusterServicePlanReference
// tests reconcileBinding to ensure a binding fails when a ClusterServiceClassRef has not been resolved.
func TestReconcileServiceBindingUnresolvedClusterServicePlanReference(t *testing.T) {
	_, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	instance := &v1beta1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceName, Namespace: testNamespace},
		Spec: v1beta1.ServiceInstanceSpec{
			PlanReference: v1beta1.PlanReference{
				ClusterServiceClassExternalName: testNonExistentClusterServiceClassName,
				ClusterServicePlanExternalName:  testClusterServicePlanName,
			},
			ExternalID:             testServiceInstanceGUID,
			ClusterServiceClassRef: &v1beta1.ClusterObjectReference{Name: "Some Ref"},
		},
	}
	sharedInformers.ServiceInstances().Informer().GetStore().Add(instance)
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	err := testController.reconcileServiceBinding(binding)
	if err == nil {
		t.Fatal("serviceclass nothere was found and it should not be found")
	}

	if err := checkEventContains(err.Error(), "not been resolved yet"); err != nil {
		t.Fatal(err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	// There are no actions.
	assertNumberOfActions(t, actions, 0)
}

// TestReconcileBindingNonExistingClusterServiceClass tests reconcileBinding to ensure a
// binding fails as expected when a serviceclass does not exist.
func TestReconcileServiceBindingNonExistingClusterServiceClass(t *testing.T) {
	_, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	instance := &v1beta1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceName, Namespace: testNamespace},
		Spec: v1beta1.ServiceInstanceSpec{
			PlanReference: v1beta1.PlanReference{
				ClusterServiceClassExternalName: testNonExistentClusterServiceClassName,
				ClusterServicePlanExternalName:  testClusterServicePlanName,
			},
			ExternalID:             testServiceInstanceGUID,
			ClusterServiceClassRef: &v1beta1.ClusterObjectReference{Name: "nosuchclassid"},
			ClusterServicePlanRef:  &v1beta1.ClusterObjectReference{Name: "nosuchplanid"},
		},
	}
	sharedInformers.ServiceInstances().Informer().GetStore().Add(instance)
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	err := testController.reconcileServiceBinding(binding)
	if err == nil {
		t.Fatal("serviceclass nothere was found and it should not be found")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	// There is one action to update to failed status because there's
	// no such service
	assertNumberOfActions(t, actions, 1)

	// There should be one action that says it failed because no such service class.
	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingReadyFalse(t, updatedServiceBinding, errorNonexistentClusterServiceClassMessage)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := warningEventBuilder(errorNonexistentClusterServiceClassMessage).msgf(
		"References a non-existent ClusterServiceClass (K8S: %q ExternalName: %q)",
		"nosuchclassid", testNonExistentClusterServiceClassName,
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingWithSecretConflict tests reconcileBinding to ensure a
// binding with an existing secret not owned by the bindings fails as expected.
func TestReconcileServiceBindingWithSecretConflict(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: map[string]interface{}{
					"a": "b",
					"c": "d",
				},
			},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)
	// existing Secret with nil controllerRef
	addGetSecretReaction(fakeKubeClient, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceBindingName, Namespace: testNamespace},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
			SecretName:         testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	err := testController.reconcileServiceBinding(binding)
	if err == nil {
		t.Fatalf("a binding should fail to create a secret: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
		AppGUID:    strPtr(testNamespaceGUID),
		BindResource: &osb.BindResource{
			AppGUID: strPtr(testNamespaceGUID),
		},
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingOperationInProgress(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, binding)

	updatedServiceBinding = assertUpdateStatus(t, actions[1], binding).(*v1beta1.ServiceBinding)

	assertServiceBindingReadyFalse(t, updatedServiceBinding, errorInjectingBindResultReason)
	assertServiceBindingCurrentOperation(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind)
	assertServiceBindingOperationStartTimeSet(t, updatedServiceBinding, true)
	assertServiceBindingReconciledGeneration(t, updatedServiceBinding, binding.Status.ReconciledGeneration)
	assertServiceBindingInProgressPropertiesNil(t, updatedServiceBinding)
	// External properties are updated because the bind request with the Broker was successful
	assertServiceBindingExternalPropertiesParameters(t, updatedServiceBinding, nil, "")
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 2)

	// first action is a get on the namespace
	// second action is a get on the secret
	action := kubeActions[1].(clientgotesting.GetAction)
	if e, a := "get", action.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on action; %s", expectedGot(e, a))
	}
	if e, a := "secrets", action.GetResource().Resource; e != a {
		t.Fatalf("Unexpected resource on action; %s", expectedGot(e, a))
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := warningEventBuilder(errorInjectingBindResultReason)

	if err := checkEventPrefixes(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingWithParameters tests reconcileBinding to ensure a
// binding with parameters will be passed to the broker properly.
func TestReconcileServiceBindingWithParameters(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: map[string]interface{}{
					"a": "b",
					"c": "d",
				},
			},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)
	addGetSecretNotFoundReaction(fakeKubeClient)

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Finalizers: []string{v1beta1.FinalizerServiceCatalog},
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
			SecretName:         testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	parameters := bindingParameters{Name: "test-param"}
	parameters.Args = append(parameters.Args, "first-arg")
	parameters.Args = append(parameters.Args, "second-arg")
	b, err := json.Marshal(parameters)
	if err != nil {
		t.Fatalf("Failed to marshal parameters %v : %v", parameters, err)
	}
	binding.Spec.Parameters = &runtime.RawExtension{Raw: b}

	err = testController.reconcileServiceBinding(binding)
	if err != nil {
		t.Fatalf("a valid binding should not fail: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
		AppGUID:    strPtr(testNamespaceGUID),
		Parameters: map[string]interface{}{
			"args": []interface{}{
				"first-arg",
				"second-arg",
			},
			"name": "test-param",
		},
		BindResource: &osb.BindResource{
			AppGUID: strPtr(testNamespaceGUID),
		},
	})

	expectedParameters := map[string]interface{}{
		"args": []interface{}{
			"first-arg",
			"second-arg",
		},
		"name": "test-param",
	}
	expectedParametersChecksum, err := generateChecksumOfParameters(expectedParameters)
	if err != nil {
		t.Fatalf("Failed to generate parameters checksum: %v", err)
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingOperationInProgressWithParameters(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, expectedParameters, expectedParametersChecksum, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	updatedServiceBinding = assertUpdateStatus(t, actions[1], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingOperationSuccessWithParameters(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, expectedParameters, expectedParametersChecksum, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 3)

	// first action is a get on the namespace
	// second action is a get on the secret
	action := kubeActions[2].(clientgotesting.CreateAction)
	if e, a := "create", action.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on action; %s", expectedGot(e, a))
	}
	if e, a := "secrets", action.GetResource().Resource; e != a {
		t.Fatalf("Unexpected resource on action; %s", expectedGot(e, a))
	}
	actionSecret, ok := action.GetObject().(*corev1.Secret)
	if !ok {
		t.Fatal("couldn't convert secret into a corev1.Secret")
	}
	controllerRef := metav1.GetControllerOf(actionSecret)
	if controllerRef == nil || controllerRef.UID != updatedServiceBinding.UID {
		t.Fatalf("Secret is not owned by the ServiceBinding: %v", controllerRef)
	}
	if !metav1.IsControlledBy(actionSecret, updatedServiceBinding) {
		t.Fatal("Secret is not owned by the ServiceBinding")
	}
	if e, a := testServiceBindingSecretName, actionSecret.Name; e != a {
		t.Fatalf("Unexpected name of secret; %s", expectedGot(e, a))
	}
	value, ok := actionSecret.Data["a"]
	if !ok {
		t.Fatal("Didn't find secret key 'a' in created secret")
	}
	if e, a := "b", string(value); e != a {
		t.Fatalf("Unexpected value of key 'a' in created secret; %s", expectedGot(e, a))
	}
	value, ok = actionSecret.Data["c"]
	if !ok {
		t.Fatal("Didn't find secret key 'a' in created secret")
	}
	if e, a := "d", string(value); e != a {
		t.Fatalf("Unexpected value of key 'c' in created secret; %s", expectedGot(e, a))
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := normalEventBuilder(successInjectedBindResultReason).msg(successInjectedBindResultMessage)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingNonbindableClusterServiceClass tests reconcileBinding to ensure a
// binding for an instance that references a non-bindable service class and a
// non-bindable plan fails as expected.
func TestReconcileServiceBindingNonbindableClusterServiceClass(t *testing.T) {
	_, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestNonbindableClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestNonbindableServiceInstance())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlanNonbindable())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	err := testController.reconcileServiceBinding(binding)
	if err != nil {
		t.Fatalf("binding should fail against a non-bindable ClusterServiceClass")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says binding was created
	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingFailedBeforeRequest(t, updatedServiceBinding, errorNonbindableClusterServiceClassReason, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)
	assertServiceBindingReconciledGeneration(t, updatedServiceBinding, binding.Generation)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorNonbindableClusterServiceClassReason).msgf(
		"References a non-bindable ClusterServiceClass (K8S: %q ExternalName: %q) and Plan (%q) combination",
		"UNBINDABLE-SERVICE", "test-unbindable-serviceclass", "test-unbindable-plan",
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingNonbindableClusterServiceClassBindablePlan tests reconcileBinding
// to ensure a binding for an instance that references a non-bindable service
// class and a bindable plan fails as expected.
func TestReconcileServiceBindingNonbindableClusterServiceClassBindablePlan(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: map[string]interface{}{
					"a": "b",
					"c": "d",
				},
			},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)
	addGetSecretNotFoundReaction(fakeKubeClient)

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestNonbindableClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(func() *v1beta1.ServiceInstance {
		i := getTestServiceInstanceNonbindableServiceBindablePlan()
		i.Status = v1beta1.ServiceInstanceStatus{
			Conditions: []v1beta1.ServiceInstanceCondition{
				{
					Type:   v1beta1.ServiceInstanceConditionReady,
					Status: v1beta1.ConditionTrue,
				},
			},
		}
		return i
	}())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Finalizers: []string{v1beta1.FinalizerServiceCatalog},
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
			SecretName:         testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	err := testController.reconcileServiceBinding(binding)
	if err != nil {
		t.Fatalf("A bindable plan overrides the bindability of a service class: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testNonbindableClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
		AppGUID:    strPtr(testNamespaceGUID),
		BindResource: &osb.BindResource{
			AppGUID: strPtr(testNamespaceGUID),
		},
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingOperationInProgress(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	updatedServiceBinding = assertUpdateStatus(t, actions[1], binding)
	assertServiceBindingOperationSuccess(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 3)

	// first action is a get on the namespace
	// second action is a get on the secret
	action := kubeActions[2].(clientgotesting.CreateAction)
	if e, a := "create", action.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on action; %s", expectedGot(e, a))
	}
	if e, a := "secrets", action.GetResource().Resource; e != a {
		t.Fatalf("Unexpected resource on action; %s", expectedGot(e, a))
	}
	actionSecret, ok := action.GetObject().(*corev1.Secret)
	if !ok {
		t.Fatal("couldn't convert secret into a corev1.Secret")
	}
	if e, a := testServiceBindingSecretName, actionSecret.Name; e != a {
		t.Fatalf("Unexpected name of secret; %s", expectedGot(e, a))
	}
	value, ok := actionSecret.Data["a"]
	if !ok {
		t.Fatal("Didn't find secret key 'a' in created secret")
	}
	if e, a := "b", string(value); e != a {
		t.Fatalf("Unexpected value of key 'a' in created secret; %s", expectedGot(e, a))
	}
	value, ok = actionSecret.Data["c"]
	if !ok {
		t.Fatal("Didn't find secret key 'a' in created secret")
	}
	if e, a := "d", string(value); e != a {
		t.Fatalf("Unexpected value of key 'c' in created secret; %s", expectedGot(e, a))
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)
}

// TestReconcileBindingBindableClusterServiceClassNonbindablePlan tests reconcileBinding
// to ensure a binding for an instance that references a bindable service class
// and a non-bindable plan fails as expected.
func TestReconcileServiceBindingBindableClusterServiceClassNonbindablePlan(t *testing.T) {
	_, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceBindableServiceNonbindablePlan())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlanNonbindable())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	err := testController.reconcileServiceBinding(binding)
	if err != nil {
		t.Fatalf("binding against a nonbindable plan should fail")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says binding was created
	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingFailedBeforeRequest(t, updatedServiceBinding, errorNonbindableClusterServiceClassReason, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := warningEventBuilder(errorNonbindableClusterServiceClassReason).msgf(
		"References a non-bindable ClusterServiceClass (K8S: %q ExternalName: %q) and Plan (%q) combination",
		"SCGUID", "test-serviceclass", "test-unbindable-plan",
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingFailsWithInstanceAsyncOngoing tests reconcileBinding
// to ensure a binding that references an instance that has the
// AsyncOpInProgreset flag set to true fails as expected.
func TestReconcileServiceBindingFailsWithServiceInstanceAsyncOngoing(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceAsyncProvisioning(""))
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	err := testController.reconcileServiceBinding(binding)
	if err == nil {
		t.Fatalf("reconcileServiceBinding did not fail with async operation ongoing")
	}

	if err := checkEventContains(err.Error(), "Ongoing Asynchronous"); err != nil {
		t.Fatal(err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says binding was created
	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingErrorBeforeRequest(t, updatedServiceBinding, errorWithOngoingAsyncOperation, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)

	assertNumEvents(t, events, 1)
	if err := checkEventContains(events[0], "has ongoing asynchronous operation"); err != nil {
		t.Fatal(err)
	}
	if err := checkEventContains(events[0], testNamespace+"/"+testServiceInstanceName); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingInstanceNotReady tests reconcileBinding to ensure a
// binding for an instance with a ready condition set to false fails as expected.
func TestReconcileServiceBindingServiceInstanceNotReady(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	addGetNamespaceReaction(fakeKubeClient)

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithRefs())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	err := testController.reconcileServiceBinding(binding)
	if err != nil {
		t.Fatalf("a binding cannot be created against an instance that is not prepared")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says binding was created
	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingErrorBeforeRequest(t, updatedServiceBinding, errorServiceInstanceNotReadyReason, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := warningEventBuilder(errorServiceInstanceNotReadyReason).msgf(
		"ServiceBinding cannot begin because referenced ServiceInstance %q is not ready",
		"test-ns/test-instance",
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingNamespaceError tests reconcileBinding to ensure a binding
// with an invalid namespace fails as expected.
func TestReconcileServiceBindingNamespaceError(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	fakeKubeClient.AddReactor("get", "namespaces", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &corev1.Namespace{}, errors.New("No namespace")
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithRefs())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	err := testController.reconcileServiceBinding(binding)
	if err == nil {
		t.Fatalf("ServiceBindings are namespaced. If we cannot get the namespace we cannot find the binding")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingErrorBeforeRequest(t, updatedServiceBinding, errorFindingNamespaceServiceInstanceReason, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := warningEventBuilder(errorFindingNamespaceServiceInstanceReason).msgf(
		"Failed to get namespace %q during binding: %s",
		"test-ns", "No namespace",
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingDelete tests reconcileBinding to ensure a binding
// deletion works as expected.
func TestReconcileServiceBindingDelete(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UnbindReaction: &fakeosb.UnbindReaction{
			Response: &osb.UnbindResponse{},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithRefs())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testServiceBindingName,
			Namespace:         testNamespace,
			DeletionTimestamp: &metav1.Time{},
			Finalizers:        []string{v1beta1.FinalizerServiceCatalog},
			Generation:        2,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
			SecretName:         testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			ReconciledGeneration: 1,
			ExternalProperties:   &v1beta1.ServiceBindingPropertiesState{},
			UnbindStatus:         v1beta1.ServiceBindingUnbindStatusRequired,
		},
	}

	fakeCatalogClient.AddReactor("get", "servicebindings", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, binding, nil
	})

	err := testController.reconcileServiceBinding(binding)
	if err != nil {
		t.Fatalf("%v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertUnbind(t, brokerActions[0], &osb.UnbindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
	})

	kubeActions := fakeKubeClient.Actions()
	// The action should be deleting the secret
	assertNumberOfActions(t, kubeActions, 1)

	deleteAction := kubeActions[0].(clientgotesting.DeleteActionImpl)
	if e, a := "delete", deleteAction.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on kubeActions[1]; %s", expectedGot(e, a))
	}

	if e, a := binding.Spec.SecretName, deleteAction.Name; e != a {
		t.Fatalf("Unexpected name of secret: %s", expectedGot(e, a))
	}

	actions := fakeCatalogClient.Actions()
	// The actions should be:
	// 0. Updating the current operation
	// 1. Updating the ready condition
	assertNumberOfActions(t, actions, 2)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingOperationInProgress(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	updatedServiceBinding = assertUpdateStatus(t, actions[1], binding)
	assertServiceBindingOperationSuccess(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)

	expectedEvent := normalEventBuilder(successUnboundReason).msg("This binding was deleted successfully")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceBindingDeleteUnresolvedClusterServiceClassReference
// tests reconcileBinding to ensure a binding delete succeeds when a ClusterServiceClassRef
// has not been resolved and no action has accrued for the binding.
func TestReconcileServiceBindingDeleteUnresolvedClusterServiceClassReference(t *testing.T) {
	_, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	instance := &v1beta1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceName, Namespace: testNamespace},
		Spec: v1beta1.ServiceInstanceSpec{
			PlanReference: v1beta1.PlanReference{
				ClusterServiceClassExternalName: testNonExistentClusterServiceClassName,
				ClusterServicePlanExternalName:  testClusterServicePlanName,
			},
			ExternalID: testServiceInstanceGUID,
		},
	}
	sharedInformers.ServiceInstances().Informer().GetStore().Add(instance)
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testServiceBindingName,
			Namespace:         testNamespace,
			DeletionTimestamp: &metav1.Time{},
			Finalizers:        []string{v1beta1.FinalizerServiceCatalog},
			Generation:        1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	err := testController.reconcileServiceBinding(binding)
	if err != nil {
		t.Fatal("should have deleted the binding")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	// The actions should be:
	// 0. Clear the finalizer
	assertNumberOfActions(t, actions, 1)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 0)
}

// TestSetServiceBindingCondition verifies setting a condition on a binding yields
// the results as expected with respect to the changed condition and transition
// time.
func TestSetServiceBindingCondition(t *testing.T) {
	bindingWithCondition := func(condition *v1beta1.ServiceBindingCondition) *v1beta1.ServiceBinding {
		binding := getTestServiceBinding()
		binding.Status = v1beta1.ServiceBindingStatus{
			Conditions:   []v1beta1.ServiceBindingCondition{*condition},
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusRequired,
		}

		return binding
	}

	// The value of the LastTransitionTime field on conditions has to be
	// tested to ensure it is updated correctly.
	//
	// Time basis for all condition changes:
	newTs := metav1.Now()
	oldTs := metav1.NewTime(newTs.Add(-5 * time.Minute))

	// condition is a shortcut method for creating conditions with the 'old' timestamp.
	condition := func(cType v1beta1.ServiceBindingConditionType, status v1beta1.ConditionStatus, s ...string) *v1beta1.ServiceBindingCondition {
		c := &v1beta1.ServiceBindingCondition{
			Type:   cType,
			Status: status,
		}

		if len(s) > 0 {
			c.Reason = s[0]
		}

		if len(s) > 1 {
			c.Message = s[1]
		}

		// This is the expected 'before' timestamp for all conditions under
		// test.
		c.LastTransitionTime = oldTs

		return c
	}

	// shortcut methods for creating conditions of different types

	readyFalse := func() *v1beta1.ServiceBindingCondition {
		return condition(v1beta1.ServiceBindingConditionReady, v1beta1.ConditionFalse, "Reason", "Message")
	}

	readyFalsef := func(reason, message string) *v1beta1.ServiceBindingCondition {
		return condition(v1beta1.ServiceBindingConditionReady, v1beta1.ConditionFalse, reason, message)
	}

	readyTrue := func() *v1beta1.ServiceBindingCondition {
		return condition(v1beta1.ServiceBindingConditionReady, v1beta1.ConditionTrue, "Reason", "Message")
	}

	failedTrue := func() *v1beta1.ServiceBindingCondition {
		return condition(v1beta1.ServiceBindingConditionFailed, v1beta1.ConditionTrue, "Reason", "Message")
	}

	// withNewTs sets the LastTransitionTime to the 'new' basis time and
	// returns it.
	withNewTs := func(c *v1beta1.ServiceBindingCondition) *v1beta1.ServiceBindingCondition {
		c.LastTransitionTime = newTs
		return c
	}

	// this test works by calling setServiceBindingCondition with the input and
	// condition fields of the test case, and ensuring that afterward the
	// input (which is mutated by the setServiceBindingCondition call) is deep-equal
	// to the test case result.
	//
	// take note of where withNewTs is used when declaring the result to
	// indicate that the LastTransitionTime field on a condition should have
	// changed.
	cases := []struct {
		name      string
		input     *v1beta1.ServiceBinding
		condition *v1beta1.ServiceBindingCondition
		result    *v1beta1.ServiceBinding
	}{
		{
			name:      "new ready condition",
			input:     getTestServiceBinding(),
			condition: readyFalse(),
			result:    bindingWithCondition(withNewTs(readyFalse())),
		},
		{
			name:      "not ready -> not ready; no ts update",
			input:     bindingWithCondition(readyFalse()),
			condition: readyFalse(),
			result:    bindingWithCondition(readyFalse()),
		},
		{
			name:      "not ready -> not ready, reason and message change; no ts update",
			input:     bindingWithCondition(readyFalse()),
			condition: readyFalsef("DifferentReason", "DifferentMessage"),
			result:    bindingWithCondition(readyFalsef("DifferentReason", "DifferentMessage")),
		},
		{
			name:      "not ready -> ready",
			input:     bindingWithCondition(readyFalse()),
			condition: readyTrue(),
			result:    bindingWithCondition(withNewTs(readyTrue())),
		},
		{
			name:      "ready -> ready; no ts update",
			input:     bindingWithCondition(readyTrue()),
			condition: readyTrue(),
			result:    bindingWithCondition(readyTrue()),
		},
		{
			name:      "ready -> not ready",
			input:     bindingWithCondition(readyTrue()),
			condition: readyFalse(),
			result:    bindingWithCondition(withNewTs(readyFalse())),
		},
		{
			name:      "not ready -> not ready + failed",
			input:     bindingWithCondition(readyFalse()),
			condition: failedTrue(),
			result: func() *v1beta1.ServiceBinding {
				i := bindingWithCondition(readyFalse())
				i.Status.Conditions = append(i.Status.Conditions, *withNewTs(failedTrue()))
				return i
			}(),
		},
	}

	for _, tc := range cases {
		setServiceBindingConditionInternal(tc.input, tc.condition.Type, tc.condition.Status, tc.condition.Reason, tc.condition.Message, newTs)

		if !reflect.DeepEqual(tc.input, tc.result) {
			t.Errorf("%v: unexpected diff: %v", tc.name, diff.ObjectReflectDiff(tc.input, tc.result))
		}
	}
}

// TestReconcileServiceBindingDeleteFailedServiceBinding tests reconcileServiceBinding to ensure
// a binding with a failed status is deleted properly.
func TestReconcileServiceBindingDeleteFailedServiceBinding(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UnbindReaction: &fakeosb.UnbindReaction{
			Response: &osb.UnbindResponse{},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithRefs())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := getTestServiceBindingWithFailedStatus()
	binding.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	binding.ObjectMeta.Finalizers = []string{v1beta1.FinalizerServiceCatalog}
	binding.Status.ExternalProperties = &v1beta1.ServiceBindingPropertiesState{}
	binding.Status.UnbindStatus = v1beta1.ServiceBindingUnbindStatusRequired

	binding.ObjectMeta.Generation = 2
	binding.Status.ReconciledGeneration = 1

	fakeCatalogClient.AddReactor("get", "servicebindings", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, binding, nil
	})

	err := testController.reconcileServiceBinding(binding)
	if err != nil {
		t.Fatalf("%v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertUnbind(t, brokerActions[0], &osb.UnbindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
	})

	// verify one kube action occurred
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "delete", resourceName: "secrets", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	deleteAction := kubeActions[0].(clientgotesting.DeleteActionImpl)
	if e, a := binding.Spec.SecretName, deleteAction.Name; e != a {
		t.Fatalf("Unexpected name of secret: %s", expectedGot(e, a))
	}

	actions := fakeCatalogClient.Actions()
	// The four actions should be:
	// 0. Updating the current operation
	// 1. Updating the ready condition
	assertNumberOfActions(t, actions, 2)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingOperationInProgress(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	updatedServiceBinding = assertUpdateStatus(t, actions[1], binding)
	assertServiceBindingOperationSuccess(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := normalEventBuilder(successUnboundReason).msg("This binding was deleted successfully")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingWithBrokerError tests reconcileBinding to ensure a
// binding request response that contains a broker error fails as expected.
func TestReconcileServiceBindingWithClusterServiceBrokerError(t *testing.T) {
	_, fakeCatalogClient, _, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: map[string]interface{}{
					"a": "b",
					"c": "d",
				},
			},
			Error: fakeosb.UnexpectedActionError(),
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
			SecretName:         testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	err := testController.reconcileServiceBinding(binding)
	if err == nil {
		t.Fatal("reconcileServiceBinding should have returned an error")
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingOperationInProgress(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	updatedServiceBinding = assertUpdateStatus(t, actions[1], binding)
	assertServiceBindingRequestRetriableError(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, errorBindCallReason, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorBindCallReason).msgf(
		"Error creating ServiceBinding for ServiceInstance %q of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q:",
		"test-ns/test-instance", "SCGUID", "test-serviceclass", "test-broker",
	).msg("Unexpected action")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingWithBrokerHTTPError tests reconcileBindings to ensure a
// binding request response that contains a broker HTTP error fails as expected.
func TestReconcileServiceBindingWithClusterServiceBrokerHTTPError(t *testing.T) {
	_, fakeCatalogClient, _, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: map[string]interface{}{
					"a": "b",
					"c": "d",
				},
			},
			Error: fakeosb.AsyncRequiredError(),
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Finalizers: []string{v1beta1.FinalizerServiceCatalog},
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
			SecretName:         testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	err := testController.reconcileServiceBinding(binding)
	if err != nil {
		t.Fatal("reconcileServiceBinding should not have returned an error")
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingOperationInProgress(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	updatedServiceBinding = assertUpdateStatus(t, actions[1], binding)
	assertServiceBindingRequestFailingError(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, errorBindCallReason, "ServiceBindingReturnedFailure", binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorBindCallReason).msgf(
		"Error creating ServiceBinding for ServiceInstance %q of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q:",
		"test-ns/test-instance", "SCGUID", "test-serviceclass", "test-broker",
	).msg(
		"Status: 422; ErrorMessage: AsyncRequired; Description: This service plan requires client support for asynchronous service operations.; ResponseError: <nil>",
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceBindingWithFailureCondition tests reconcileServiceBinding to ensure
// no processing is done on a binding containing a failed status.
func TestReconcileServiceBindingWithFailureCondition(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := getTestServiceBindingWithFailedStatus()

	if err := testController.reconcileServiceBinding(binding); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 0)

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 0)
}

// TestReconcileServiceBindingWithServiceBindingCallFailure tests reconcileServiceBinding to ensure
// a bind creation failure is handled properly.
func TestReconcileServiceBindingWithServiceBindingCallFailure(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Error: errors.New("fake creation failure"),
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := getTestServiceBinding()

	if err := testController.reconcileServiceBinding(binding); err == nil {
		t.Fatal("ServiceBinding creation should fail")
	}

	// verify one kube action occurred
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "namespaces", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingOperationInProgress(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	updatedServiceBinding = assertUpdateStatus(t, actions[1], binding)
	assertServiceBindingRequestRetriableError(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, errorBindCallReason, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
		AppGUID:    strPtr(""),
		BindResource: &osb.BindResource{
			AppGUID: strPtr(""),
		},
	})

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorBindCallReason).msgf(
		"Error creating ServiceBinding for ServiceInstance %q of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q:",
		"test-ns/test-instance", "SCGUID", "test-serviceclass", "test-broker",
	).msg("fake creation failure")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceBindingWithServiceBindingFailure tests reconcileServiceBinding to ensure
// a binding request that receives an error from the broker is handled properly.
func TestReconcileServiceBindingWithServiceBindingFailure(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Error: osb.HTTPStatusCodeError{
				StatusCode:   http.StatusConflict,
				ErrorMessage: strPtr("ServiceBindingExists"),
				Description:  strPtr("Service binding with the same id, for the same service instance already exists."),
			},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	binding := getTestServiceBinding()

	if err := testController.reconcileServiceBinding(binding); err != nil {
		t.Fatalf("ServiceBinding creation should complete: %v", err)
	}

	// verify one kube action occurred
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "namespaces", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingOperationInProgress(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	updatedServiceBinding = assertUpdateStatus(t, actions[1], binding)
	assertServiceBindingRequestFailingError(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, errorBindCallReason, "ServiceBindingReturnedFailure", binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
		AppGUID:    strPtr(""),
		BindResource: &osb.BindResource{
			AppGUID: strPtr(""),
		},
	})

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorBindCallReason).msgf(
		"Error creating ServiceBinding for ServiceInstance %q of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q:",
		"test-ns/test-instance", "SCGUID", "test-serviceclass", "test-broker",
	).msg(
		"Status: 409; ErrorMessage: ServiceBindingExists; Description: Service binding with the same id, for the same service instance already exists.; ResponseError: <nil>",
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestUpdateBindingCondition tests updateBindingCondition to ensure all status
// condition transitions on a binding work as expected.
//
// The test cases are proving:
// - a binding with no status that has status condition set to false will update
//   the transition time
// - a binding with condition false set to condition false will not update the
//   transition time
// - a binding with condition false set to condition false with a new message and
//   reason will not update the transition time
// - a binding with condition false set to condition true will update the
//   transition time
// - a binding with condition status true set to true will not update the
//   transition time
// - a binding with condition status true set to false will update the transition
//   time
func TestUpdateServiceBindingCondition(t *testing.T) {
	getTestServiceBindingWithStatus := func(status v1beta1.ConditionStatus) *v1beta1.ServiceBinding {
		instance := getTestServiceBinding()
		instance.Status = v1beta1.ServiceBindingStatus{
			Conditions: []v1beta1.ServiceBindingCondition{{
				Type:               v1beta1.ServiceBindingConditionReady,
				Status:             status,
				Message:            "message",
				LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
			}},
		}

		return instance
	}

	// Anonymous struct fields:
	// name: short description of the test
	// input: the binding to test
	// status: condition status to set for binding condition
	// reason: reason to set for binding condition
	// message: message to set for binding condition
	// transitionTimeChanged: toggle for verifying transition time was updated
	cases := []struct {
		name                  string
		input                 *v1beta1.ServiceBinding
		status                v1beta1.ConditionStatus
		reason                string
		message               string
		transitionTimeChanged bool
	}{

		{
			name:                  "initially unset",
			input:                 getTestServiceBinding(),
			status:                v1beta1.ConditionFalse,
			transitionTimeChanged: true,
		},
		{
			name:                  "not ready -> not ready",
			input:                 getTestServiceBindingWithStatus(v1beta1.ConditionFalse),
			status:                v1beta1.ConditionFalse,
			transitionTimeChanged: false,
		},
		{
			name:                  "not ready -> not ready, message and reason change",
			input:                 getTestServiceBindingWithStatus(v1beta1.ConditionFalse),
			status:                v1beta1.ConditionFalse,
			reason:                "foo",
			message:               "bar",
			transitionTimeChanged: false,
		},
		{
			name:                  "not ready -> ready",
			input:                 getTestServiceBindingWithStatus(v1beta1.ConditionFalse),
			status:                v1beta1.ConditionTrue,
			transitionTimeChanged: true,
		},
		{
			name:                  "ready -> ready",
			input:                 getTestServiceBindingWithStatus(v1beta1.ConditionTrue),
			status:                v1beta1.ConditionTrue,
			transitionTimeChanged: false,
		},
		{
			name:                  "ready -> not ready",
			input:                 getTestServiceBindingWithStatus(v1beta1.ConditionTrue),
			status:                v1beta1.ConditionFalse,
			transitionTimeChanged: true,
		},
	}

	for _, tc := range cases {
		_, fakeCatalogClient, _, testController, _ := newTestController(t, noFakeActions())

		clone, err := api.Scheme.DeepCopy(tc.input)
		if err != nil {
			t.Errorf("%v: deep copy failed", tc.name)
			continue
		}
		inputClone := clone.(*v1beta1.ServiceBinding)

		err = testController.updateServiceBindingCondition(tc.input, v1beta1.ServiceBindingConditionReady, tc.status, tc.reason, tc.message)
		if err != nil {
			t.Errorf("%v: error updating broker condition: %v", tc.name, err)
			continue
		}

		if !reflect.DeepEqual(tc.input, inputClone) {
			t.Errorf("%v: updating broker condition mutated input: %s", tc.name, expectedGot(inputClone, tc.input))
			continue
		}

		actions := fakeCatalogClient.Actions()
		if ok := expectNumberOfActions(t, tc.name, actions, 1); !ok {
			continue
		}

		updatedServiceBinding, ok := expectUpdateStatus(t, tc.name, actions[0], tc.input)
		if !ok {
			continue
		}

		updateActionObject, ok := updatedServiceBinding.(*v1beta1.ServiceBinding)
		if !ok {
			t.Errorf("%v: couldn't convert to binding", tc.name)
			continue
		}

		var initialTs metav1.Time
		if len(inputClone.Status.Conditions) != 0 {
			initialTs = inputClone.Status.Conditions[0].LastTransitionTime
		}

		if e, a := 1, len(updateActionObject.Status.Conditions); e != a {
			t.Errorf("%v: %s", tc.name, expectedGot(e, a))
		}

		outputCondition := updateActionObject.Status.Conditions[0]
		newTs := outputCondition.LastTransitionTime

		if tc.transitionTimeChanged && initialTs == newTs {
			t.Errorf("%v: transition time didn't change when it should have", tc.name)
			continue
		} else if !tc.transitionTimeChanged && initialTs != newTs {
			t.Errorf("%v: transition time changed when it shouldn't have", tc.name)
			continue
		}
		if e, a := tc.reason, outputCondition.Reason; e != "" && e != a {
			t.Errorf("%v: condition reasons didn't match; %s", tc.name, expectedGot(e, a))
			continue
		}
		if e, a := tc.message, outputCondition.Message; e != "" && e != a {
			t.Errorf("%v: condition reasons didn't match; %s", tc.name, expectedGot(e, a))
		}
	}
}

// TestReconcileUnbindingWithBrokerError tests reconcileBinding to ensure an
// unbinding request response that contains a broker error fails as expected.
func TestReconcileUnbindingWithClusterServiceBrokerError(t *testing.T) {
	_, fakeCatalogClient, _, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UnbindReaction: &fakeosb.UnbindReaction{
			Response: &osb.UnbindResponse{},
			Error:    fakeosb.UnexpectedActionError(),
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	t1 := metav1.NewTime(time.Now())
	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testServiceBindingName,
			Namespace:         testNamespace,
			DeletionTimestamp: &t1,
			Generation:        1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
			SecretName:         testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			ExternalProperties: &v1beta1.ServiceBindingPropertiesState{},
			UnbindStatus:       v1beta1.ServiceBindingUnbindStatusRequired,
		},
	}
	if err := scmeta.AddFinalizer(binding, v1beta1.FinalizerServiceCatalog); err != nil {
		t.Fatalf("Finalizer error: %v", err)
	}
	if err := testController.reconcileServiceBinding(binding); err == nil {
		t.Fatal("reconcileServiceBinding should have returned an error")
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingOperationInProgress(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	updatedServiceBinding = assertUpdateStatus(t, actions[1], binding)
	assertServiceBindingRequestRetriableError(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, errorUnbindCallReason, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorUnbindCallReason).msgf(
		"Error unbinding from ServiceInstance %q of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q:",
		"test-ns/test-instance", "SCGUID", "test-serviceclass", "test-broker",
	).msg("Unexpected action")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileUnbindingWithClusterServiceBrokerHTTPError tests reconcileBinding to ensure an
// unbinding request response that contains a broker HTTP error fails as
// expected.
func TestReconcileUnbindingWithClusterServiceBrokerHTTPError(t *testing.T) {
	_, fakeCatalogClient, _, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UnbindReaction: &fakeosb.UnbindReaction{
			Response: &osb.UnbindResponse{},
			Error: osb.HTTPStatusCodeError{
				StatusCode: http.StatusGone,
			},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	t1 := metav1.NewTime(time.Now())
	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testServiceBindingName,
			Namespace:         testNamespace,
			DeletionTimestamp: &t1,
			Generation:        1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
			SecretName:         testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			ExternalProperties: &v1beta1.ServiceBindingPropertiesState{},
			UnbindStatus:       v1beta1.ServiceBindingUnbindStatusRequired,
		},
	}
	if err := scmeta.AddFinalizer(binding, v1beta1.FinalizerServiceCatalog); err != nil {
		t.Fatalf("Finalizer error: %v", err)
	}
	if err := testController.reconcileServiceBinding(binding); err != nil {
		t.Fatalf("reconcileServiceBinding should not have returned an error: %v", err)
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingOperationInProgress(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	updatedServiceBinding = assertUpdateStatus(t, actions[1], binding)
	assertServiceBindingRequestFailingError(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, errorUnbindCallReason, errorUnbindCallReason, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorUnbindCallReason).msgf(
		"Error unbinding from ServiceInstance %q of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q:",
		"test-ns/test-instance", "SCGUID", "test-serviceclass", "test-broker",
	).msg("Status: 410; ErrorMessage: <nil>; Description: <nil>; ResponseError: <nil>")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileBindingUsingOriginatingIdentity(t *testing.T) {
	for _, tc := range originatingIdentityTestCases {
		func() {
			if tc.enableOriginatingIdentity {
				utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=true", scfeatures.OriginatingIdentity))
				defer utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=false", scfeatures.OriginatingIdentity))
			}

			fakeKubeClient, _, fakeBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
				BindReaction: &fakeosb.BindReaction{
					Response: &osb.BindResponse{},
				},
			})

			addGetNamespaceReaction(fakeKubeClient)
			addGetSecretNotFoundReaction(fakeKubeClient)

			sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
			sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
			sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
			sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

			binding := getTestServiceBinding()
			if tc.includeUserInfo {
				binding.Spec.UserInfo = testUserInfo
			}

			err := testController.reconcileServiceBinding(binding)
			if err != nil {
				t.Fatalf("%v: a valid binding should not fail: %v", tc.name, err)
			}

			brokerActions := fakeBrokerClient.Actions()
			assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
			actualRequest, ok := brokerActions[0].Request.(*osb.BindRequest)
			if !ok {
				t.Errorf("%v: unexpected request type; expected %T, got %T", tc.name, &osb.BindRequest{}, actualRequest)
				return
			}
			var expectedOriginatingIdentity *osb.OriginatingIdentity
			if tc.expectedOriginatingIdentity {
				expectedOriginatingIdentity = testOriginatingIdentity
			}
			assertOriginatingIdentity(t, expectedOriginatingIdentity, actualRequest.OriginatingIdentity)
		}()
	}
}

func TestReconcileBindingDeleteUsingOriginatingIdentity(t *testing.T) {
	for _, tc := range originatingIdentityTestCases {
		func() {
			if tc.enableOriginatingIdentity {
				err := utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=true", scfeatures.OriginatingIdentity))
				if err != nil {
					t.Fatalf("Failed to enable originating identity feature: %v", err)
				}
				defer utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=false", scfeatures.OriginatingIdentity))
			}

			fakeKubeClient, _, fakeBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
				UnbindReaction: &fakeosb.UnbindReaction{
					Response: &osb.UnbindResponse{},
				},
			})

			addGetNamespaceReaction(fakeKubeClient)
			addGetSecretNotFoundReaction(fakeKubeClient)

			sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
			sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
			sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
			sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

			binding := getTestServiceBinding()
			binding.DeletionTimestamp = &metav1.Time{}
			binding.Finalizers = []string{v1beta1.FinalizerServiceCatalog}
			if tc.includeUserInfo {
				binding.Spec.UserInfo = testUserInfo
			}

			err := testController.reconcileServiceBinding(binding)
			if err != nil {
				t.Fatalf("%v: a valid binding should not fail: %v", tc.name, err)
			}

			brokerActions := fakeBrokerClient.Actions()
			assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
			actualRequest, ok := brokerActions[0].Request.(*osb.UnbindRequest)
			if !ok {
				t.Errorf("%v: unexpected request type; expected %T, got %T", tc.name, &osb.UnbindRequest{}, actualRequest)
				return
			}
			var expectedOriginatingIdentity *osb.OriginatingIdentity
			if tc.expectedOriginatingIdentity {
				expectedOriginatingIdentity = testOriginatingIdentity
			}
			assertOriginatingIdentity(t, expectedOriginatingIdentity, actualRequest.OriginatingIdentity)
		}()
	}
}

// TestReconcileBindingSuccessOnFinalRetry verifies that reconciliation can
// succeed on the last attempt before timing out of the retry loop
func TestReconcileBindingSuccessOnFinalRetry(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: map[string]interface{}{
					"a": "b",
					"c": "d",
				},
			},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)
	addGetSecretNotFoundReaction(fakeKubeClient)

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

	binding := getTestServiceBinding()
	binding.Status.CurrentOperation = v1beta1.ServiceBindingOperationBind
	startTime := metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
	binding.Status.OperationStartTime = &startTime

	if err := testController.reconcileServiceBinding(binding); err != nil {
		t.Fatalf("a valid binding should not fail: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
		AppGUID:    strPtr(testNamespaceGUID),
		BindResource: &osb.BindResource{
			AppGUID: strPtr(testNamespaceGUID),
		},
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingOperationSuccess(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)

	expectedEvent := normalEventBuilder(successInjectedBindResultReason).msg(successInjectedBindResultMessage)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingFailureOnFinalRetry verifies that reconciliation
// completes in the event of an error after the retry duration elapses.
func TestReconcileBindingFailureOnFinalRetry(t *testing.T) {
	_, fakeCatalogClient, _, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: map[string]interface{}{
					"a": "b",
					"c": "d",
				},
			},
			Error: fakeosb.UnexpectedActionError(),
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

	binding := getTestServiceBinding()
	binding.Status.CurrentOperation = v1beta1.ServiceBindingOperationBind
	startTime := metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
	binding.Status.OperationStartTime = &startTime

	if err := testController.reconcileServiceBinding(binding); err != nil {
		t.Fatalf("Should have return no error because the retry duration has elapsed: %v", err)
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingRequestFailingError(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, errorBindCallReason, errorReconciliationRetryTimeoutReason, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)

	expectedEventPrefixes := []string{
		warningEventBuilder(errorBindCallReason).String(),
		warningEventBuilder(errorReconciliationRetryTimeoutReason).String(),
	}
	if err := checkEventPrefixes(events, expectedEventPrefixes); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileBindingWithSecretConflictFailedAfterFinalRetry tests
// reconcileBinding to ensure a binding with an existing secret not owned by the
// bindings is marked as failed after the retry duration elapses.
func TestReconcileBindingWithSecretConflictFailedAfterFinalRetry(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: map[string]interface{}{
					"a": "b",
					"c": "d",
				},
			},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)
	// existing Secret with nil controllerRef
	addGetSecretReaction(fakeKubeClient, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceBindingName, Namespace: testNamespace},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

	startTime := metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
			SecretName:         testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			CurrentOperation:   v1beta1.ServiceBindingOperationBind,
			OperationStartTime: &startTime,
			UnbindStatus:       v1beta1.ServiceBindingUnbindStatusRequired,
		},
	}

	if err := testController.reconcileServiceBinding(binding); err != nil {
		t.Fatalf("reconciliation should complete since the retry duration has elapsed: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
		AppGUID:    strPtr(testNamespaceGUID),
		BindResource: &osb.BindResource{
			AppGUID: strPtr(testNamespaceGUID),
		},
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)

	assertServiceBindingCondition(t, updatedServiceBinding, v1beta1.ServiceBindingConditionReady, v1beta1.ConditionFalse, errorServiceBindingOrphanMitigation)
	assertServiceBindingCondition(t, updatedServiceBinding, v1beta1.ServiceBindingConditionFailed, v1beta1.ConditionTrue, errorReconciliationRetryTimeoutReason)
	assertServiceBindingStartingOrphanMitigation(t, updatedServiceBinding, binding)
	assertServiceBindingInProgressPropertiesNil(t, updatedServiceBinding)
	assertServiceBindingExternalPropertiesParameters(t, updatedServiceBinding, nil, "")

	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 2)

	// first action is a get on the namespace
	// second action is a get on the secret
	action := kubeActions[1].(clientgotesting.GetAction)
	if e, a := "get", action.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on action; %s", expectedGot(e, a))
	}
	if e, a := "secrets", action.GetResource().Resource; e != a {
		t.Fatalf("Unexpected resource on action; %s", expectedGot(e, a))
	}

	events := getRecordedEvents(testController)

	expectedEventPrefixes := []string{
		warningEventBuilder(errorInjectingBindResultReason).String(),
		warningEventBuilder(errorReconciliationRetryTimeoutReason).String(),
		warningEventBuilder(errorServiceBindingOrphanMitigation).String(),
	}
	if err := checkEventPrefixes(events, expectedEventPrefixes); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceBindingWithStatusUpdateError verifies that the
// reconciler returns an error when there is a conflict updating the status of
// the resource. This is an otherwise successful scenario where the update to set
// the in-progress operation fails.
func TestReconcileServiceBindingWithStatusUpdateError(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	addGetNamespaceReaction(fakeKubeClient)
	addGetSecretNotFoundReaction(fakeKubeClient)

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

	binding := getTestServiceBinding()

	fakeCatalogClient.AddReactor("update", "servicebindings", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("update error")
	})

	err := testController.reconcileServiceBinding(binding)
	if err == nil {
		t.Fatalf("expected error from but got none")
	}
	if e, a := "update error", err.Error(); e != a {
		t.Fatalf("unexpected error returned: %s", expectedGot(e, a))
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingOperationInProgress(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 0)
}

// TestReconcileServiceInstanceCredentailWithSecretParameters tests reconciling a
// binding that has parameters obtained from secrets.
func TestReconcileServiceBindingWithSecretParameters(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: map[string]interface{}{
					"a": "b",
					"c": "d",
				},
			},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)

	paramSecret := &corev1.Secret{
		Data: map[string][]byte{
			"param-secret-key": []byte("{\"b\":\"2\"}"),
		},
	}
	fakeKubeClient.AddReactor("get", "secrets", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		switch name := action.(clientgotesting.GetAction).GetName(); name {
		case "param-secret-name":
			return true, paramSecret, nil
		default:
			return true, nil, apierrors.NewNotFound(action.GetResource().GroupResource(), name)
		}
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Finalizers: []string{v1beta1.FinalizerServiceCatalog},
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
			SecretName:         testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	parameters := map[string]interface{}{
		"a": "1",
	}
	b, err := json.Marshal(parameters)
	if err != nil {
		t.Fatalf("Failed to marshal parameters %v : %v", parameters, err)
	}
	binding.Spec.Parameters = &runtime.RawExtension{Raw: b}

	binding.Spec.ParametersFrom = []v1beta1.ParametersFromSource{
		{
			SecretKeyRef: &v1beta1.SecretKeyReference{
				Name: "param-secret-name",
				Key:  "param-secret-key",
			},
		},
	}

	err = testController.reconcileServiceBinding(binding)
	if err != nil {
		t.Fatalf("a valid binding should not fail: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
		AppGUID:    strPtr(testNamespaceGUID),
		Parameters: map[string]interface{}{
			"a": "1",
			"b": "2",
		},
		BindResource: &osb.BindResource{
			AppGUID: strPtr(testNamespaceGUID),
		},
	})

	expectedParameters := map[string]interface{}{
		"a": "1",
		"b": "<redacted>",
	}
	expectedParametersChecksum, err := generateChecksumOfParameters(map[string]interface{}{
		"a": "1",
		"b": "2",
	})
	if err != nil {
		t.Fatalf("Failed to generate parameters checksum: %v", err)
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding)
	assertServiceBindingOperationInProgressWithParameters(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, expectedParameters, expectedParametersChecksum, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	updatedServiceBinding = assertUpdateStatus(t, actions[1], binding)
	assertServiceBindingOperationSuccessWithParameters(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, expectedParameters, expectedParametersChecksum, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 4)

	// first action is a get on the namespace
	// second action is a get on the secret, to build the parameters
	action, ok := kubeActions[1].(clientgotesting.GetAction)
	if !ok {
		t.Fatalf("unexpected type of action: expected a GetAction, got %T", kubeActions[0])
	}
	if e, a := "secrets", action.GetResource().Resource; e != a {
		t.Fatalf("Unexpected resource on action: %s", expectedGot(e, a))
	}
	if e, a := "param-secret-name", action.GetName(); e != a {
		t.Fatalf("Unexpected name of secret fetched: %s", expectedGot(e, a))
	}

	events := getRecordedEvents(testController)

	expectedEvent := normalEventBuilder(successInjectedBindResultReason).msg(successInjectedBindResultMessage)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}

}

// TestReconcileBindingWithSetOrphanMitigation tests
// reconcileServiceBinding to ensure a binding properly initiates
// orphan mitigation in the case of timeout or receiving certain HTTP codes.
func TestReconcileBindingWithSetOrphanMitigation(t *testing.T) {
	// Anonymous struct fields:
	// bindReactionError: the error to return from the bind attempt
	// setOrphanMitigation: flag for whether or not orphan migitation
	//                      should be performed
	cases := []struct {
		name                string
		bindReactionError   error
		setOrphanMitigation bool
		shouldReturnError   bool
	}{
		{
			name:                "timeout error",
			bindReactionError:   testTimeoutError{},
			setOrphanMitigation: false,
			shouldReturnError:   true,
		},
		{
			name: "osb code 200",
			bindReactionError: osb.HTTPStatusCodeError{
				StatusCode: 200,
			},
			setOrphanMitigation: false,
			shouldReturnError:   false,
		},
		{
			name: "osb code 201",
			bindReactionError: osb.HTTPStatusCodeError{
				StatusCode: 201,
			},
			setOrphanMitigation: true,
			shouldReturnError:   false,
		},
		{
			name: "osb code 300",
			bindReactionError: osb.HTTPStatusCodeError{
				StatusCode: 300,
			},
			setOrphanMitigation: false,
			shouldReturnError:   false,
		},
		{
			name: "osb code 400",
			bindReactionError: osb.HTTPStatusCodeError{
				StatusCode: 400,
			},
			setOrphanMitigation: false,
			shouldReturnError:   false,
		},
		{
			name: "osb code 408",
			bindReactionError: osb.HTTPStatusCodeError{
				StatusCode: 408,
			},
			setOrphanMitigation: true,
			shouldReturnError:   false,
		},
		{
			name: "osb code 500",
			bindReactionError: osb.HTTPStatusCodeError{
				StatusCode: 500,
			},
			setOrphanMitigation: true,
			shouldReturnError:   false,
		},
		{
			name: "osb code 501",
			bindReactionError: osb.HTTPStatusCodeError{
				StatusCode: 501,
			},
			setOrphanMitigation: true,
			shouldReturnError:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
				BindReaction: &fakeosb.BindReaction{
					Response: &osb.BindResponse{},
					Error:    tc.bindReactionError,
				},
			})

			addGetNamespaceReaction(fakeKubeClient)
			// existing Secret with nil controllerRef
			addGetSecretReaction(fakeKubeClient, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: testServiceBindingName, Namespace: testNamespace},
			})

			sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
			sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
			sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
			sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

			binding := &v1beta1.ServiceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       testServiceBindingName,
					Namespace:  testNamespace,
					Generation: 1,
				},
				Spec: v1beta1.ServiceBindingSpec{
					ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
					ExternalID:         testServiceBindingGUID,
					SecretName:         testServiceBindingSecretName,
				},
				Status: v1beta1.ServiceBindingStatus{
					UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
				},
			}
			startTime := metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
			binding.Status.OperationStartTime = &startTime

			if err := testController.reconcileServiceBinding(binding); tc.shouldReturnError && err == nil || !tc.shouldReturnError && err != nil {
				t.Fatalf("expected to return %v from reconciliation attempt, got %v", tc.shouldReturnError, err)
			}

			brokerActions := fakeServiceBrokerClient.Actions()
			assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
			assertBind(t, brokerActions[0], &osb.BindRequest{
				BindingID:  testServiceBindingGUID,
				InstanceID: testServiceInstanceGUID,
				ServiceID:  testClusterServiceClassGUID,
				PlanID:     testClusterServicePlanGUID,
				AppGUID:    strPtr(testNamespaceGUID),
				BindResource: &osb.BindResource{
					AppGUID: strPtr(testNamespaceGUID),
				},
			})

			kubeActions := fakeKubeClient.Actions()
			assertNumberOfActions(t, kubeActions, 1)
			action := kubeActions[0].(clientgotesting.GetAction)
			if e, a := "get", action.GetVerb(); e != a {
				t.Fatalf("Unexpected verb on action; %s", expectedGot(e, a))
			}
			if e, a := "namespaces", action.GetResource().Resource; e != a {
				t.Fatalf("Unexpected resource on action; %s", expectedGot(e, a))
			}

			actions := fakeCatalogClient.Actions()
			assertNumberOfActions(t, actions, 2)

			updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)
			assertServiceBindingReadyFalse(t, updatedServiceBinding)

			updatedServiceBinding = assertUpdateStatus(t, actions[1], binding).(*v1beta1.ServiceBinding)

			if tc.setOrphanMitigation {
				assertServiceBindingStartingOrphanMitigation(t, updatedServiceBinding, binding)
			} else {
				assertServiceBindingReadyFalse(t, updatedServiceBinding)
				assertServiceBindingCondition(t, updatedServiceBinding, v1beta1.ServiceBindingConditionReady, v1beta1.ConditionFalse)
				assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, tc.setOrphanMitigation)
				assertServiceBindingExternalPropertiesNil(t, updatedServiceBinding)
			}
		})
	}
}

// TestReconcileBindingWithOrphanMitigationInProgress tests
// reconcileServiceBinding to ensure a binding is properly handled
// once orphan mitigation is underway.
func TestReconcileBindingWithOrphanMitigationInProgress(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UnbindReaction: &fakeosb.UnbindReaction{
			Response: &osb.UnbindResponse{},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)
	// existing Secret with nil controllerRef
	addGetSecretReaction(fakeKubeClient, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceBindingName, Namespace: testNamespace},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Finalizers: []string{v1beta1.FinalizerServiceCatalog},
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
			SecretName:         testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusRequired,
		},
	}
	binding.Status.CurrentOperation = v1beta1.ServiceBindingOperationBind
	binding.Status.OperationStartTime = nil
	binding.Status.OrphanMitigationInProgress = true

	if err := testController.reconcileServiceBinding(binding); err != nil {
		t.Fatalf("reconciliation should complete since the retry duration has elapsed: %v", err)
	}
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 1)
	action := kubeActions[0].(clientgotesting.GetAction)
	if e, a := "delete", action.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on action; %s", expectedGot(e, a))
	}
	if e, a := "secrets", action.GetResource().Resource; e != a {
		t.Fatalf("Unexpected resource on action; %s", expectedGot(e, a))
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertUnbind(t, brokerActions[0], &osb.UnbindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingCondition(t, updatedServiceBinding, v1beta1.ServiceBindingConditionReady, v1beta1.ConditionFalse, "OrphanMitigationSuccessful")
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)
}

// TestReconcileBindingWithOrphanMitigationReconciliationRetryTimeOut tests
// reconcileServiceBinding to ensure a binding is properly handled
// once orphan mitigation is underway, specifically in the failure scenario of a
// time out during orphan mitigation.
func TestReconcileBindingWithOrphanMitigationReconciliationRetryTimeOut(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UnbindReaction: &fakeosb.UnbindReaction{
			Response: &osb.UnbindResponse{},
			Error:    testTimeoutError{},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)
	// existing Secret with nil controllerRef
	addGetSecretReaction(fakeKubeClient, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceBindingName, Namespace: testNamespace},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceBindingName,
			Namespace:  testNamespace,
			Finalizers: []string{v1beta1.FinalizerServiceCatalog},
			Generation: 1,
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
			SecretName:         testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			Conditions: []v1beta1.ServiceBindingCondition{
				{
					Type:   v1beta1.ServiceBindingConditionFailed,
					Status: v1beta1.ConditionTrue,
					Reason: "reason-orphan-mitigation-began",
				},
			},
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusRequired,
		},
	}
	startTime := metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
	binding.Status.CurrentOperation = v1beta1.ServiceBindingOperationBind
	binding.Status.OperationStartTime = &startTime
	binding.Status.OrphanMitigationInProgress = true

	if err := testController.reconcileServiceBinding(binding); err != nil {
		t.Fatalf("reconciliation should complete since the retry duration has elapsed: %v", err)
	}
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 1)
	action := kubeActions[0].(clientgotesting.GetAction)
	if e, a := "delete", action.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on action; %s", expectedGot(e, a))
	}
	if e, a := "secrets", action.GetResource().Resource; e != a {
		t.Fatalf("Unexpected resource on action; %s", expectedGot(e, a))
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertUnbind(t, brokerActions[0], &osb.UnbindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingRequestFailingError(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, errorUnbindCallReason, "reason-orphan-mitigation-began", binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)

	expectedEventPrefixes := []string{
		warningEventBuilder(errorUnbindCallReason).String(),
		warningEventBuilder(errorReconciliationRetryTimeoutReason).String(),
	}

	if err := checkEventPrefixes(events, expectedEventPrefixes); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceBindingDeleteDuringOngoingOperation tests deleting a
// binding that has an on-going operation.
func TestReconcileServiceBindingDeleteDuringOngoingOperation(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UnbindReaction: &fakeosb.UnbindReaction{
			Response: &osb.UnbindResponse{},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithRefs())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	startTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))
	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testServiceBindingName,
			Namespace:         testNamespace,
			DeletionTimestamp: &metav1.Time{},
			Finalizers:        []string{v1beta1.FinalizerServiceCatalog},
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
			SecretName:         testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			CurrentOperation:   v1beta1.ServiceBindingOperationBind,
			OperationStartTime: &startTime,
			UnbindStatus:       v1beta1.ServiceBindingUnbindStatusRequired,
		},
	}

	fakeCatalogClient.AddReactor("get", "servicebindings", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, binding, nil
	})

	timeOfReconciliation := metav1.Now()

	err := testController.reconcileServiceBinding(binding)
	if err != nil {
		t.Fatalf("%v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertUnbind(t, brokerActions[0], &osb.UnbindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
	})

	kubeActions := fakeKubeClient.Actions()
	// The action should be deleting the secret
	assertNumberOfActions(t, kubeActions, 1)

	deleteAction := kubeActions[0].(clientgotesting.DeleteActionImpl)
	if e, a := "delete", deleteAction.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on kubeActions[1]; %s", expectedGot(e, a))
	}

	if e, a := binding.Spec.SecretName, deleteAction.Name; e != a {
		t.Fatalf("Unexpected name of secret: %s", expectedGot(e, a))
	}

	actions := fakeCatalogClient.Actions()
	// The actions should be:
	// 0. Updating the current operation
	// 1. Updating the ready condition
	assertNumberOfActions(t, actions, 2)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingOperationInProgress(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	// Verify that the operation start time was reset to Now
	if updatedServiceBinding.Status.OperationStartTime.Before(&timeOfReconciliation) {
		t.Fatalf(
			"OperationStartTime should not be before the time that the reconciliation started. OperationStartTime=%v. timeOfReconciliation=%v",
			updatedServiceBinding.Status.OperationStartTime,
			timeOfReconciliation,
		)
	}

	updatedServiceBinding = assertUpdateStatus(t, actions[1], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingOperationSuccess(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)

	expectedEvent := normalEventBuilder(successUnboundReason).msg("This binding was deleted successfully")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceBindingDeleteDuringOrphanMitigation tests deleting a
// binding that is undergoing orphan mitigation
func TestReconcileServiceBindingDeleteDuringOrphanMitigation(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UnbindReaction: &fakeosb.UnbindReaction{
			Response: &osb.UnbindResponse{},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithRefs())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	startTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))
	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testServiceBindingName,
			Namespace:         testNamespace,
			DeletionTimestamp: &metav1.Time{},
			Finalizers:        []string{v1beta1.FinalizerServiceCatalog},
		},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         testServiceBindingGUID,
			SecretName:         testServiceBindingSecretName,
		},
		Status: v1beta1.ServiceBindingStatus{
			CurrentOperation:           v1beta1.ServiceBindingOperationBind,
			OperationStartTime:         &startTime,
			OrphanMitigationInProgress: true,
			UnbindStatus:               v1beta1.ServiceBindingUnbindStatusRequired,
		},
	}

	fakeCatalogClient.AddReactor("get", "servicebindings", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, binding, nil
	})

	timeOfReconciliation := metav1.Now()

	err := testController.reconcileServiceBinding(binding)
	if err != nil {
		t.Fatalf("%v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertUnbind(t, brokerActions[0], &osb.UnbindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
	})

	kubeActions := fakeKubeClient.Actions()
	// The action should be deleting the secret
	assertNumberOfActions(t, kubeActions, 1)

	deleteAction := kubeActions[0].(clientgotesting.DeleteActionImpl)
	if e, a := "delete", deleteAction.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on kubeActions[1]; %s", expectedGot(e, a))
	}

	if e, a := binding.Spec.SecretName, deleteAction.Name; e != a {
		t.Fatalf("Unexpected name of secret: %s", expectedGot(e, a))
	}

	actions := fakeCatalogClient.Actions()
	// The actions should be:
	// 0. Updating the current operation
	// 1. Updating the ready condition
	assertNumberOfActions(t, actions, 2)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingOperationInProgress(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	// Verify that the operation start time was reset to Now
	if updatedServiceBinding.Status.OperationStartTime.Before(&timeOfReconciliation) {
		t.Fatalf(
			"OperationStartTime should not be before the time that the reconciliation started. OperationStartTime=%v. timeOfReconciliation=%v",
			updatedServiceBinding.Status.OperationStartTime,
			timeOfReconciliation,
		)
	}

	updatedServiceBinding = assertUpdateStatus(t, actions[1], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingOperationSuccess(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, binding)
	assertServiceBindingOrphanMitigationSet(t, updatedServiceBinding, false)

	events := getRecordedEvents(testController)

	expectedEvent := normalEventBuilder(successUnboundReason).msg("This binding was deleted successfully")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}

}

// TestReconcileServiceBindingAsynchronousBind tests the situation where the
// controller receives an asynchronous bind response back from the broker when
// doing a bind call.
func TestReconcileServiceBindingAsynchronousBind(t *testing.T) {
	key := osb.OperationKey(testOperation)
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Async:        true,
				OperationKey: &key,
			},
		},
	})

	utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=true", scfeatures.AsyncBindingOperations))
	defer utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=false", scfeatures.AsyncBindingOperations))

	addGetNamespaceReaction(fakeKubeClient)
	addGetSecretNotFoundReaction(fakeKubeClient)

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestBindingRetrievableClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

	binding := getTestServiceBinding()
	bindingKey := binding.Namespace + "/" + binding.Name

	if testController.bindingPollingQueue.NumRequeues(bindingKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test binding")
	}

	if err := testController.reconcileServiceBinding(binding); err != nil {
		t.Fatalf("a valid binding should not fail: %v", err)
	}

	if testController.bindingPollingQueue.NumRequeues(bindingKey) != 1 {
		t.Fatalf("Expected polling queue to have a record of seeing test binding once")
	}

	// Broker actions
	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  testServiceBindingGUID,
		InstanceID: testServiceInstanceGUID,
		ServiceID:  testClusterServiceClassGUID,
		PlanID:     testClusterServicePlanGUID,
		AppGUID:    strPtr(testNamespaceGUID),
		BindResource: &osb.BindResource{
			AppGUID: strPtr(testNamespaceGUID),
		},
		AcceptsIncomplete: true,
	})

	// Kube actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 1)
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "namespaces", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	// Service Catalog actions
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingOperationInProgress(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, binding)

	updatedServiceBinding = assertUpdateStatus(t, actions[1], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingAsyncInProgress(t, updatedServiceBinding, v1beta1.ServiceBindingOperationBind, asyncBindingReason, testOperation, binding)

	// Events
	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := corev1.EventTypeNormal + " " + asyncBindingReason + " " + asyncBindingMessage
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event, expected %v got %v", e, a)
	}
}

// TestReconcileServiceBindingAsynchronousUnbind tests the situation where the
// controller receives an asynchronous bind response back from the broker when
// doing an unbind call.
func TestReconcileServiceBindingAsynchronousUnbind(t *testing.T) {
	key := osb.OperationKey(testOperation)
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UnbindReaction: &fakeosb.UnbindReaction{
			Response: &osb.UnbindResponse{
				Async:        true,
				OperationKey: &key,
			},
		},
	})

	utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=true", scfeatures.AsyncBindingOperations))
	defer utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=false", scfeatures.AsyncBindingOperations))

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestBindingRetrievableClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

	binding := getTestServiceBindingUnbinding()
	bindingKey := binding.Namespace + "/" + binding.Name

	fakeCatalogClient.AddReactor("get", "servicebindings", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, binding, nil
	})

	if testController.bindingPollingQueue.NumRequeues(bindingKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test binding")
	}

	if err := testController.reconcileServiceBinding(binding); err != nil {
		t.Fatalf("a valid binding should not fail: %v", err)
	}

	if testController.bindingPollingQueue.NumRequeues(bindingKey) != 1 {
		t.Fatalf("Expected polling queue to have a record of seeing test binding once")
	}

	// Broker actions
	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertUnbind(t, brokerActions[0], &osb.UnbindRequest{
		BindingID:         testServiceBindingGUID,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            testClusterServicePlanGUID,
		AcceptsIncomplete: true,
	})

	// Kube actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 1)
	deleteAction := kubeActions[0].(clientgotesting.DeleteActionImpl)
	if e, a := "delete", deleteAction.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on kubeActions[1]; expected %v, got %v", e, a)
	}

	if e, a := binding.Spec.SecretName, deleteAction.Name; e != a {
		t.Fatalf("Unexpected name of secret: expected %v, got %v", e, a)
	}

	// Service Catalog actions
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceBinding := assertUpdateStatus(t, actions[0], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingOperationInProgress(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, binding)

	updatedServiceBinding = assertUpdateStatus(t, actions[1], binding).(*v1beta1.ServiceBinding)
	assertServiceBindingAsyncInProgress(t, updatedServiceBinding, v1beta1.ServiceBindingOperationUnbind, asyncUnbindingReason, testOperation, binding)

	// Events
	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := corev1.EventTypeNormal + " " + asyncUnbindingReason + " " + asyncUnbindingMessage
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event, expected %v got %v", e, a)
	}
}

func TestPollServiceBinding(t *testing.T) {
	utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=true", scfeatures.AsyncBindingOperations))
	defer utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=false", scfeatures.AsyncBindingOperations))

	goneError := osb.HTTPStatusCodeError{
		StatusCode: http.StatusGone,
	}

	validatePollBindingLastOperationAction := func(t *testing.T, actions []fakeosb.Action) {
		assertNumberOfClusterServiceBrokerActions(t, actions, 1)

		operationKey := osb.OperationKey(testOperation)
		assertPollBindingLastOperation(t, actions[0], &osb.BindingLastOperationRequest{
			InstanceID:   testServiceInstanceGUID,
			BindingID:    testServiceBindingGUID,
			ServiceID:    strPtr(testClusterServiceClassGUID),
			PlanID:       strPtr(testClusterServicePlanGUID),
			OperationKey: &operationKey,
		})
	}

	validatePollBindingLastOperationAndGetBindingActions := func(t *testing.T, actions []fakeosb.Action) {
		assertNumberOfClusterServiceBrokerActions(t, actions, 2)

		operationKey := osb.OperationKey(testOperation)
		assertPollBindingLastOperation(t, actions[0], &osb.BindingLastOperationRequest{
			InstanceID:   testServiceInstanceGUID,
			BindingID:    testServiceBindingGUID,
			ServiceID:    strPtr(testClusterServiceClassGUID),
			PlanID:       strPtr(testClusterServicePlanGUID),
			OperationKey: &operationKey,
		})

		assertGetBinding(t, actions[1], &osb.GetBindingRequest{
			InstanceID: testServiceInstanceGUID,
			BindingID:  testServiceBindingGUID,
		})
	}

	cases := []struct {
		name                      string
		binding                   *v1beta1.ServiceBinding
		pollReaction              *fakeosb.PollBindingLastOperationReaction
		getBindingReaction        *fakeosb.GetBindingReaction
		environmentSetupFunc      func(t *testing.T, fakeKubeClient *clientgofake.Clientset, sharedInformers v1beta1informers.Interface)
		validateBrokerActionsFunc func(t *testing.T, actions []fakeosb.Action)
		validateKubeActionsFunc   func(t *testing.T, actions []clientgotesting.Action)
		validateConditionsFunc    func(t *testing.T, updatedBinding *v1beta1.ServiceBinding, originalBinding *v1beta1.ServiceBinding)
		shouldFinishPolling       bool
		expectedEvents            []string
	}{
		// Bind
		{
			name:    "bind - error",
			binding: getTestServiceBindingAsyncBinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Error: fmt.Errorf("random error"),
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc:    nil, // does not update resources
			shouldFinishPolling:       false,
			expectedEvents:            []string{corev1.EventTypeWarning + " " + errorPollingLastOperationReason + " " + "Error polling last operation: random error"},
		},
		{
			// Special test for 410, as it is treated differently in other operations
			name:    "bind - 410 Gone considered error",
			binding: getTestServiceBindingAsyncBinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Error: goneError,
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc:    nil, // does not update resources
			shouldFinishPolling:       false,
			expectedEvents:            []string{corev1.EventTypeWarning + " " + errorPollingLastOperationReason + " " + "Error polling last operation: " + goneError.Error()},
		},
		{
			name:    "bind - in progress",
			binding: getTestServiceBindingAsyncBinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateInProgress,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc: func(t *testing.T, updatedBinding *v1beta1.ServiceBinding, originalBinding *v1beta1.ServiceBinding) {
				assertServiceBindingAsyncInProgress(t, updatedBinding, v1beta1.ServiceBindingOperationBind, asyncBindingReason, testOperation, originalBinding)
			},
			shouldFinishPolling: false,
			expectedEvents:      []string{},
		},
		{
			name:    "bind - failed",
			binding: getTestServiceBindingAsyncBinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateFailed,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc: func(t *testing.T, updatedBinding *v1beta1.ServiceBinding, originalBinding *v1beta1.ServiceBinding) {
				assertServiceBindingRequestFailingError(
					t,
					updatedBinding,
					v1beta1.ServiceBindingOperationBind,
					errorBindCallReason,
					errorBindCallReason,
					originalBinding,
				)
			},
			shouldFinishPolling: true,
			expectedEvents:      []string{corev1.EventTypeWarning + " " + errorBindCallReason + " " + "Bind call failed: " + lastOperationDescription},
		},
		{
			name:    "bind - invalid state",
			binding: getTestServiceBindingAsyncBinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       "test invalid state",
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc:    nil, // does not update resources
			shouldFinishPolling:       false,
			expectedEvents:            []string{}, // does not record event
		},
		{
			name:    "bind - in progress - retry duration exceeded",
			binding: getTestServiceBindingAsyncBindingRetryDurationExceeded(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateInProgress,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc: func(t *testing.T, updatedBinding *v1beta1.ServiceBinding, originalBinding *v1beta1.ServiceBinding) {
				assertServiceBindingAsyncBindRetryDurationExceeded(t, updatedBinding, originalBinding)
			},
			shouldFinishPolling: true,
			expectedEvents: []string{
				corev1.EventTypeWarning + " " + errorReconciliationRetryTimeoutReason + " " + "Stopping reconciliation retries because too much time has elapsed",
				corev1.EventTypeWarning + " " + errorServiceBindingOrphanMitigation + " " + `ServiceBinding "test-binding/test-ns": Starting orphan mitigation`,
			},
		},
		{
			name:    "bind - invalid state - retry duration exceeded",
			binding: getTestServiceBindingAsyncBindingRetryDurationExceeded(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       "test invalid state",
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc: func(t *testing.T, updatedBinding *v1beta1.ServiceBinding, originalBinding *v1beta1.ServiceBinding) {
				assertServiceBindingAsyncBindRetryDurationExceeded(t, updatedBinding, originalBinding)
			},
			shouldFinishPolling: true,
			expectedEvents: []string{
				corev1.EventTypeWarning + " " + errorReconciliationRetryTimeoutReason + " " + "Stopping reconciliation retries because too much time has elapsed",
				corev1.EventTypeWarning + " " + errorServiceBindingOrphanMitigation + " " + `ServiceBinding "test-binding/test-ns": Starting orphan mitigation`,
			},
		},
		{
			name:    "bind - operation succeeded but GET failed",
			binding: getTestServiceBindingAsyncBinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateSucceeded,
					Description: strPtr(lastOperationDescription),
				},
			},
			getBindingReaction: &fakeosb.GetBindingReaction{
				Error: fmt.Errorf("some error"),
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAndGetBindingActions,
			validateConditionsFunc: func(t *testing.T, updatedBinding *v1beta1.ServiceBinding, originalBinding *v1beta1.ServiceBinding) {
				assertServiceBindingAsyncBindErrorAfterStateSucceeded(t, updatedBinding, errorFetchingBindingFailedReason, originalBinding)
			},
			shouldFinishPolling: true,
			expectedEvents: []string{
				corev1.EventTypeWarning + " " + errorFetchingBindingFailedReason + " " + "Could not do a GET on binding resource: some error",
				corev1.EventTypeWarning + " " + errorServiceBindingOrphanMitigation + " " + `ServiceBinding "test-binding/test-ns": Starting orphan mitigation`,
			},
		},
		{
			name:    "bind - operation succeeded but binding injection failed",
			binding: getTestServiceBindingAsyncBinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateSucceeded,
					Description: strPtr(lastOperationDescription),
				},
			},
			getBindingReaction: &fakeosb.GetBindingReaction{
				Response: &osb.GetBindingResponse{
					Credentials: map[string]interface{}{
						"a": "b",
						"c": "d",
					},
				},
			},
			environmentSetupFunc: func(t *testing.T, fakeKubeClient *clientgofake.Clientset, sharedInformers v1beta1informers.Interface) {
				sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
				sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestBindingRetrievableClusterServiceClass())
				sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
				sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

				addGetNamespaceReaction(fakeKubeClient)
				addGetSecretReaction(fakeKubeClient, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: testServiceBindingName, Namespace: testNamespace},
				})
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAndGetBindingActions,
			validateKubeActionsFunc: func(t *testing.T, actions []clientgotesting.Action) {
				assertNumberOfActions(t, actions, 1)

				action := actions[0].(clientgotesting.GetAction)
				if e, a := "get", action.GetVerb(); e != a {
					t.Fatalf("Unexpected verb on action; %s", expectedGot(e, a))
				}
				if e, a := "secrets", action.GetResource().Resource; e != a {
					t.Fatalf("Unexpected resource on action; %s", expectedGot(e, a))
				}
			},
			validateConditionsFunc: func(t *testing.T, updatedBinding *v1beta1.ServiceBinding, originalBinding *v1beta1.ServiceBinding) {
				assertServiceBindingAsyncBindErrorAfterStateSucceeded(t, updatedBinding, errorInjectingBindResultReason, originalBinding)
			},
			shouldFinishPolling: true, // should not be requeued in polling queue; will drop back to default rate limiting
			expectedEvents: []string{
				corev1.EventTypeWarning + " " + errorInjectingBindResultReason + " " + `Error injecting bind results: Secret "test-ns/test-binding" is not owned by ServiceBinding, controllerRef: nil`,
				corev1.EventTypeWarning + " " + errorServiceBindingOrphanMitigation + " " + `ServiceBinding "test-binding/test-ns": Starting orphan mitigation`,
			},
		},
		{
			name:    "bind - succeeded",
			binding: getTestServiceBindingAsyncBinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateSucceeded,
					Description: strPtr(lastOperationDescription),
				},
			},
			getBindingReaction: &fakeosb.GetBindingReaction{
				Response: &osb.GetBindingResponse{
					Credentials: map[string]interface{}{
						"a": "b",
						"c": "d",
					},
				},
			},
			environmentSetupFunc: func(t *testing.T, fakeKubeClient *clientgofake.Clientset, sharedInformers v1beta1informers.Interface) {
				sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
				sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestBindingRetrievableClusterServiceClass())
				sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
				sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))

				addGetNamespaceReaction(fakeKubeClient)
				addGetSecretNotFoundReaction(fakeKubeClient)
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAndGetBindingActions,
			validateKubeActionsFunc: func(t *testing.T, actions []clientgotesting.Action) {
				assertNumberOfActions(t, actions, 2)

				// GET on Secret
				getAction := actions[0].(clientgotesting.GetAction)
				if e, a := "get", getAction.GetVerb(); e != a {
					t.Fatalf("Unexpected verb on action; %s", expectedGot(e, a))
				}
				if e, a := "secrets", getAction.GetResource().Resource; e != a {
					t.Fatalf("Unexpected resource on action; %s", expectedGot(e, a))
				}

				// CREATE on Secret
				createAction := actions[1].(clientgotesting.CreateAction)
				if e, a := "create", createAction.GetVerb(); e != a {
					t.Fatalf("Unexpected verb on action; %s", expectedGot(e, a))
				}
				if e, a := "secrets", createAction.GetResource().Resource; e != a {
					t.Fatalf("Unexpected resource on action; %s", expectedGot(e, a))
				}
			},
			validateConditionsFunc: func(t *testing.T, updatedBinding *v1beta1.ServiceBinding, originalBinding *v1beta1.ServiceBinding) {
				assertServiceBindingOperationSuccess(t, updatedBinding, v1beta1.ServiceBindingOperationBind, originalBinding)
			},
			shouldFinishPolling: true,
			expectedEvents:      []string{corev1.EventTypeNormal + " " + successInjectedBindResultReason + " " + successInjectedBindResultMessage},
		},
		// Unbind as part of deletion
		{
			name:    "unbind - succeeded",
			binding: getTestServiceBindingAsyncUnbinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateSucceeded,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc: func(t *testing.T, updatedBinding *v1beta1.ServiceBinding, originalBinding *v1beta1.ServiceBinding) {
				assertServiceBindingOperationSuccess(t, updatedBinding, v1beta1.ServiceBindingOperationUnbind, originalBinding)
			},
			shouldFinishPolling: true,
			expectedEvents:      []string{corev1.EventTypeNormal + " " + successUnboundReason + " " + "The binding was deleted successfully"},
		},
		{
			name:    "unbind - 410 Gone considered succeeded",
			binding: getTestServiceBindingAsyncUnbinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Error: osb.HTTPStatusCodeError{
					StatusCode: http.StatusGone,
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc: func(t *testing.T, updatedBinding *v1beta1.ServiceBinding, originalBinding *v1beta1.ServiceBinding) {
				assertServiceBindingOperationSuccess(t, updatedBinding, v1beta1.ServiceBindingOperationUnbind, originalBinding)
			},
			shouldFinishPolling: true,
			expectedEvents:      []string{corev1.EventTypeNormal + " " + successUnboundReason + " " + "The binding was deleted successfully"},
		},
		{
			name:    "unbind - in progress",
			binding: getTestServiceBindingAsyncUnbinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateInProgress,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc: func(t *testing.T, updatedBinding *v1beta1.ServiceBinding, originalBinding *v1beta1.ServiceBinding) {
				assertServiceBindingAsyncInProgress(t, updatedBinding, v1beta1.ServiceBindingOperationUnbind, asyncUnbindingReason, testOperation, originalBinding)
			},
			shouldFinishPolling: false,
			expectedEvents:      []string{},
		},
		{
			name:    "unbind - error",
			binding: getTestServiceBindingAsyncUnbinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Error: fmt.Errorf("random error"),
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc:    nil, // does not update resources
			shouldFinishPolling:       false,
			expectedEvents:            []string{corev1.EventTypeWarning + " " + errorPollingLastOperationReason + " " + "Error polling last operation: random error"},
		},
		{
			name:    "unbind - failed",
			binding: getTestServiceBindingAsyncUnbinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateFailed,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc: func(t *testing.T, updatedBinding *v1beta1.ServiceBinding, originalBinding *v1beta1.ServiceBinding) {
				assertServiceBindingRequestFailingError(
					t,
					updatedBinding,
					v1beta1.ServiceBindingOperationUnbind,
					errorUnbindCallReason,
					errorUnbindCallReason,
					originalBinding,
				)
			},
			shouldFinishPolling: true,
			expectedEvents:      []string{corev1.EventTypeWarning + " " + errorUnbindCallReason + " " + "Unbind call failed: " + lastOperationDescription},
		},
		{
			name:    "unbind - invalid state",
			binding: getTestServiceBindingAsyncUnbinding(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       "test invalid state",
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc:    nil, // does not update resources
			shouldFinishPolling:       false,
			expectedEvents:            []string{}, // does not record event
		},
		{
			name:    "unbind - in progress - retry duration exceeded",
			binding: getTestServiceBindingAsyncUnbindingRetryDurationExceeded(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateInProgress,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc: func(t *testing.T, updatedBinding *v1beta1.ServiceBinding, originalBinding *v1beta1.ServiceBinding) {
				assertServiceBindingAsyncUnbindRetryDurationExceeded(
					t,
					updatedBinding,
					v1beta1.ServiceBindingOperationUnbind,
					asyncUnbindingReason,
					errorReconciliationRetryTimeoutReason,
					originalBinding,
				)
			},
			shouldFinishPolling: true,
			expectedEvents:      []string{corev1.EventTypeWarning + " " + errorReconciliationRetryTimeoutReason + " " + "Stopping reconciliation retries because too much time has elapsed"},
		},
		{
			name:    "unbind - invalid state - retry duration exceeded",
			binding: getTestServiceBindingAsyncUnbindingRetryDurationExceeded(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       "test invalid state",
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc: func(t *testing.T, updatedBinding *v1beta1.ServiceBinding, originalBinding *v1beta1.ServiceBinding) {
				assertServiceBindingAsyncUnbindRetryDurationExceeded(
					t,
					updatedBinding,
					v1beta1.ServiceBindingOperationUnbind,
					"",
					errorReconciliationRetryTimeoutReason,
					originalBinding,
				)
			},
			shouldFinishPolling: true,
			expectedEvents:      []string{corev1.EventTypeWarning + " " + errorReconciliationRetryTimeoutReason + " " + "Stopping reconciliation retries because too much time has elapsed"},
		},
		// Unbind as part of orphan mitigation
		{
			name:    "orphan mitigation - succeeded",
			binding: getTestServiceBindingAsyncOrphanMitigation(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateSucceeded,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc: func(t *testing.T, updatedBinding *v1beta1.ServiceBinding, originalBinding *v1beta1.ServiceBinding) {
				assertServiceBindingOrphanMitigationSuccess(t, updatedBinding, originalBinding)
			},
			shouldFinishPolling: true,
			expectedEvents:      []string{corev1.EventTypeNormal + " " + successOrphanMitigationReason + " " + successOrphanMitigationMessage},
		},
		{
			name:    "orphan mitigation - 410 Gone considered succeeded",
			binding: getTestServiceBindingAsyncOrphanMitigation(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Error: osb.HTTPStatusCodeError{
					StatusCode: http.StatusGone,
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc: func(t *testing.T, updatedBinding *v1beta1.ServiceBinding, originalBinding *v1beta1.ServiceBinding) {
				assertServiceBindingOrphanMitigationSuccess(t, updatedBinding, originalBinding)
			},
			shouldFinishPolling: true,
			expectedEvents:      []string{corev1.EventTypeNormal + " " + successOrphanMitigationReason + " " + successOrphanMitigationMessage},
		},
		{
			name:    "orphan mitigation - in progress",
			binding: getTestServiceBindingAsyncOrphanMitigation(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateInProgress,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc: func(t *testing.T, updatedBinding *v1beta1.ServiceBinding, originalBinding *v1beta1.ServiceBinding) {
				assertServiceBindingAsyncInProgress(t, updatedBinding, v1beta1.ServiceBindingOperationBind, asyncUnbindingReason, testOperation, originalBinding)
			},
			shouldFinishPolling: false,
			expectedEvents:      []string{},
		},
		{
			name:    "orphan mitigation - error",
			binding: getTestServiceBindingAsyncOrphanMitigation(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Error: fmt.Errorf("random error"),
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc:    nil, // does not update resources
			shouldFinishPolling:       false,
			expectedEvents:            []string{corev1.EventTypeWarning + " " + errorPollingLastOperationReason + " " + "Error polling last operation: random error"},
		},
		{
			name:    "orphan mitigation - failed",
			binding: getTestServiceBindingAsyncOrphanMitigation(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateFailed,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc: func(t *testing.T, updatedBinding *v1beta1.ServiceBinding, originalBinding *v1beta1.ServiceBinding) {
				assertServiceBindingOrphanMitigationFailure(t, updatedBinding, originalBinding)
			},
			shouldFinishPolling: true,
			expectedEvents:      []string{corev1.EventTypeWarning + " " + errorOrphanMitigationFailedReason + " " + "Orphan mitigation failed: " + lastOperationDescription},
		},
		{
			name:    "orphan mitigation - invalid state",
			binding: getTestServiceBindingAsyncOrphanMitigation(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       "test invalid state",
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc:    nil, // does not update resources
			shouldFinishPolling:       false,
			expectedEvents:            []string{}, // does not record event
		},
		{
			name:    "orphan mitigation - in progress - retry duration exceeded",
			binding: getTestServiceBindingAsyncOrphanMitigationRetryDurationExceeded(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateInProgress,
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc: func(t *testing.T, updatedBinding *v1beta1.ServiceBinding, originalBinding *v1beta1.ServiceBinding) {
				assertServiceBindingAsyncOrphanMitigationRetryDurationExceeded(t, updatedBinding, originalBinding)
			},
			shouldFinishPolling: true,
			expectedEvents:      []string{corev1.EventTypeWarning + " " + errorReconciliationRetryTimeoutReason + " " + "Stopping reconciliation retries because too much time has elapsed"},
		},
		{
			name:    "orphan mitigation - invalid state - retry duration exceeded",
			binding: getTestServiceBindingAsyncOrphanMitigationRetryDurationExceeded(testOperation),
			pollReaction: &fakeosb.PollBindingLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       "test invalid state",
					Description: strPtr(lastOperationDescription),
				},
			},
			validateBrokerActionsFunc: validatePollBindingLastOperationAction,
			validateConditionsFunc: func(t *testing.T, updatedBinding *v1beta1.ServiceBinding, originalBinding *v1beta1.ServiceBinding) {
				assertServiceBindingAsyncOrphanMitigationRetryDurationExceeded(t, updatedBinding, originalBinding)
			},
			shouldFinishPolling: true,
			expectedEvents:      []string{corev1.EventTypeWarning + " " + errorReconciliationRetryTimeoutReason + " " + "Stopping reconciliation retries because too much time has elapsed"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
				PollBindingLastOperationReaction: tc.pollReaction,
				GetBindingReaction:               tc.getBindingReaction,
			})

			if tc.environmentSetupFunc != nil {
				tc.environmentSetupFunc(t, fakeKubeClient, sharedInformers)
			} else {
				// default
				sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
				sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestBindingRetrievableClusterServiceClass())
				sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
				sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1beta1.ConditionTrue))
			}

			bindingKey := tc.binding.Namespace + "/" + tc.binding.Name

			if err := testController.pollServiceBinding(tc.binding); err != nil {
				t.Fatalf("unexpected error when polling service binding: %v", err)
			}

			if tc.shouldFinishPolling && testController.bindingPollingQueue.NumRequeues(bindingKey) != 0 {
				t.Fatalf("Expected polling queue to not have any record of test binding as polling should have completed")
			} else if !tc.shouldFinishPolling && testController.bindingPollingQueue.NumRequeues(bindingKey) != 1 {
				t.Fatalf("Expected polling queue to have record of seeing test binding once")
			}

			// Broker actions
			brokerActions := fakeServiceBrokerClient.Actions()

			if tc.validateBrokerActionsFunc != nil {
				tc.validateBrokerActionsFunc(t, brokerActions)
			} else {
				assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)
			}

			// Kube actions
			kubeActions := fakeKubeClient.Actions()

			if tc.validateKubeActionsFunc != nil {
				tc.validateKubeActionsFunc(t, kubeActions)
			} else {
				assertNumberOfActions(t, kubeActions, 0)
			}

			// Catalog actions
			actions := fakeCatalogClient.Actions()
			if tc.validateConditionsFunc != nil {
				assertNumberOfActions(t, actions, 1)
				updatedBinding := assertUpdateStatus(t, actions[0], tc.binding).(*v1beta1.ServiceBinding)
				tc.validateConditionsFunc(t, updatedBinding, tc.binding)
			} else {
				assertNumberOfActions(t, actions, 0)
			}

			// Events
			events := getRecordedEvents(testController)
			assertNumEvents(t, events, len(tc.expectedEvents))

			for idx, expectedEvent := range tc.expectedEvents {
				if e, a := expectedEvent, events[idx]; e != a {
					t.Fatalf("Received unexpected event #%v, expected %v got %v", idx, e, a)
				}
			}
		})
	}
}

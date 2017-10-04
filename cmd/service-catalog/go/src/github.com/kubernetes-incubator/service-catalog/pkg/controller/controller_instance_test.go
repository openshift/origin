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
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	osb "github.com/pmorie/go-open-service-broker-client/v2"
	fakeosb "github.com/pmorie/go-open-service-broker-client/v2/fake"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"github.com/kubernetes-incubator/service-catalog/pkg/api"
	scfeatures "github.com/kubernetes-incubator/service-catalog/pkg/features"
	apiv1 "k8s.io/client-go/pkg/api/v1"
	clientgotesting "k8s.io/client-go/testing"
)

const (
	lastOperationDescription = "testdescr"
)

// TestReconcileServiceInstanceNonExistentServiceClass tests that reconcileInstance gets a failure when
// the specified service class is not found
func TestReconcileServiceInstanceNonExistentServiceClass(t *testing.T) {
	_, fakeCatalogClient, fakeServiceBrokerClient, testController, _ := newTestController(t, noFakeActions())

	instance := &v1alpha1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceInstanceName,
			Generation: 1,
		},
		Spec: v1alpha1.ServiceInstanceSpec{
			ExternalServiceClassName: "nothere",
			ExternalServicePlanName:  "nothere",
			ExternalID:               instanceGUID,
		},
	}

	if err := testController.reconcileServiceInstance(instance); err == nil {
		t.Fatal("nothere is a service class that cannot be referenced by the service instance as it does not exist.")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.OneTermEqualSelector("spec.externalName", instance.Spec.ExternalServiceClassName),
	}
	assertList(t, actions[0], &v1alpha1.ServiceClass{}, listRestrictions)

	// There should be an action that says it failed because no such class exists.
	updatedServiceInstance := assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceErrorBeforeRequest(t, updatedServiceInstance, errorNonexistentServiceClassReason, instance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := apiv1.EventTypeWarning + " " + errorNonexistentServiceClassReason + " " + "ServiceInstance \"/test-instance\" references a non-existent ServiceClass \"nothere\""
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v\nExpected: %v", a, e)
	}
}

// TestReconcileServiceInstanceNonExistentServiceBroker tests reconciling an instance whose
// broker does not exist.  This returns an error.
func TestReconcileServiceInstanceNonExistentServiceBroker(t *testing.T) {
	_, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceWithRefs()

	if err := testController.reconcileServiceInstance(instance); err == nil {
		t.Fatal("The broker referenced by the instance exists when it should not.")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says it failed because no such broker exists.
	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceErrorBeforeRequest(t, updatedServiceInstance, errorNonexistentServiceBrokerReason, instance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := apiv1.EventTypeWarning + " " + errorNonexistentServiceBrokerReason + " " + "ServiceInstance \"test-ns/test-instance\" references a non-existent broker \"test-broker\""
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v\nExpected: %v", a, e)
	}
}

// TestReconcileServiceInstanceWithAuthError tests reconcileInstance when Kube Client
// fails to locate the broker authentication secret.
func TestReconcileServiceInstanceWithAuthError(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	broker := getTestServiceBroker()
	broker.Spec.AuthInfo = &v1alpha1.ServiceBrokerAuthInfo{
		Basic: &v1alpha1.BasicAuthConfig{
			SecretRef: &apiv1.ObjectReference{
				Namespace: "does_not_exist",
				Name:      "auth-name",
			},
		},
	}
	sharedInformers.ServiceBrokers().Informer().GetStore().Add(broker)
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceWithRefs()

	fakeKubeClient.AddReactor("get", "secrets", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("no secret defined")
	})

	if err := testController.reconcileServiceInstance(instance); err == nil {
		t.Fatal("There was no secret to be found, but does_not_exist/auth-name was found.")
	}

	// verify that no broker actions occurred
	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 0)

	// verify that one catalog client action occurred
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says it failed fetching auth credentials.
	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceErrorBeforeRequest(t, updatedServiceInstance, errorAuthCredentialsReason, instance)

	// verify one kube action occurred
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "secrets", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	// verify that one event was emitted
	events := getRecordedEvents(testController)
	expectedEvent := apiv1.EventTypeWarning + " " + errorAuthCredentialsReason + " " + "Error getting broker auth credentials for broker \"test-broker\": no secret defined"
	if err := checkEvents(events, []string{expectedEvent}); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceNonExistentServicePlan tests that reconcileInstance
// fails when service class points at a non-existent service plan
func TestReconcileServiceInstanceNonExistentServicePlan(t *testing.T) {
	_, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := &v1alpha1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceInstanceName,
			Generation: 1,
		},
		Spec: v1alpha1.ServiceInstanceSpec{
			ExternalServiceClassName: testServiceClassName,
			ServiceClassRef: &apiv1.ObjectReference{
				Name: serviceClassGUID,
			},
			ExternalServicePlanName: "nothere",
			ExternalID:              instanceGUID,
		},
	}

	if err := testController.reconcileServiceInstance(instance); err == nil {
		t.Fatal("The service plan nothere should not exist to be referenced.")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 0)

	// ensure that there are two actions, one to list plans and an action
	// to set the condition on the instance to indicate that the service
	// plan doesn't exist
	actions := fakeCatalogClient.Actions()

	assertNumberOfActions(t, actions, 2)
	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.ParseSelectorOrDie("spec.externalName=nothere,spec.serviceBrokerName=test-broker,spec.serviceClassRef.name=SCGUID"),
	}
	assertList(t, actions[0], &v1alpha1.ServicePlan{}, listRestrictions)

	updatedServiceInstance := assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceErrorBeforeRequest(t, updatedServiceInstance, errorNonexistentServicePlanReason, instance)

	// check to make sure the only event sent indicated that the instance references a non-existent
	// service plan
	events := getRecordedEvents(testController)
	expectedEvent := apiv1.EventTypeWarning + " " + errorNonexistentServicePlanReason + " " + "ServiceInstance \"/test-instance\" references a non-existent ServicePlan \"nothere\" on ServiceClass \"test-serviceclass\""
	if err := checkEvents(events, []string{expectedEvent}); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceWithParameters tests a simple successful reconciliation
func TestReconcileServiceInstanceWithParameters(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Response: &osb.ProvisionResponse{},
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceWithRefs()

	parameters := instanceParameters{Name: "test-param", Args: make(map[string]string)}
	parameters.Args["first"] = "first-arg"
	parameters.Args["second"] = "second-arg"

	b, err := json.Marshal(parameters)
	if err != nil {
		t.Fatalf("Failed to marshal parameters %v : %v", parameters, err)
	}
	instance.Spec.Parameters = &runtime.RawExtension{Raw: b}

	if err = testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        instanceGUID,
		ServiceID:         serviceClassGUID,
		PlanID:            planGUID,
		Context: map[string]interface{}{
			"platform":  "kubernetes",
			"namespace": "test-ns",
		},
		Parameters: map[string]interface{}{
			"args": map[string]interface{}{
				"first":  "first-arg",
				"second": "second-arg",
			},
			"name": "test-param",
		},
	})

	expectedParameters := map[string]interface{}{
		"args": map[string]interface{}{
			"first":  "first-arg",
			"second": "second-arg",
		},
		"name": "test-param",
	}
	expectedParametersChecksum, err := generateChecksumOfParameters(expectedParameters)
	if err != nil {
		t.Fatalf("Failed to generate parameters checksum: %v", err)
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationInProgressWithParameters(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, testServicePlanName, expectedParameters, expectedParametersChecksum, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceOperationSuccessWithParameters(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, testServicePlanName, expectedParameters, expectedParametersChecksum, instance)

	updateObject, ok := updatedServiceInstance.(*v1alpha1.ServiceInstance)
	if !ok {
		t.Fatalf("couldn't convert to *v1alpha1.ServiceInstance")
	}

	// Verify parameters are what we'd expect them to be, basically name, map with two values in it.
	if len(updateObject.Spec.Parameters.Raw) == 0 {
		t.Fatalf("Parameters was unexpectedly empty")
	}

	// verify no kube resources created
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "namespaces", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := apiv1.EventTypeNormal + " " + successProvisionReason + " " + "The instance was provisioned successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v\nExpected: %v", a, e)
	}
}

// TestReconcileServiceInstanceResolvesReferences tests a simple successful
// reconciliation and making sure that Service[Class|Plan]Ref are resolved
func TestReconcileServiceInstanceResolvesReferences(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Response: &osb.ProvisionResponse{},
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sc := getTestServiceClass()
	sharedInformers.ServiceClasses().Informer().GetStore().Add(sc)
	sp := getTestServicePlan()
	sharedInformers.ServicePlans().Informer().GetStore().Add(sp)

	instance := getTestServiceInstance()

	var scItems []v1alpha1.ServiceClass
	scItems = append(scItems, *sc)
	fakeCatalogClient.AddReactor("list", "serviceclasses", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1alpha1.ServiceClassList{Items: scItems}, nil
	})

	var spItems []v1alpha1.ServicePlan
	spItems = append(spItems, *sp)
	fakeCatalogClient.AddReactor("list", "serviceplans", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1alpha1.ServicePlanList{Items: spItems}, nil
	})

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        instanceGUID,
		ServiceID:         serviceClassGUID,
		PlanID:            planGUID,
		Context: map[string]interface{}{
			"platform":  "kubernetes",
			"namespace": "test-ns",
		},
	})

	// We should get the following actions:
	// list call for ServiceClass
	// list call for ServicePlan
	// setReferences on ServiceInstance
	// updateStatus for inprogress
	// updateStatus for success
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 5)

	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.OneTermEqualSelector("spec.externalName", instance.Spec.ExternalServiceClassName),
	}
	assertList(t, actions[0], &v1alpha1.ServiceClass{}, listRestrictions)

	listRestrictions = clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.ParseSelectorOrDie("spec.externalName=test-plan,spec.serviceBrokerName=test-broker,spec.serviceClassRef.name=SCGUID"),
	}
	assertList(t, actions[1], &v1alpha1.ServicePlan{}, listRestrictions)

	updatedServiceInstance := assertUpdateReference(t, actions[2], instance)

	updateObject, ok := updatedServiceInstance.(*v1alpha1.ServiceInstance)
	if !ok {
		t.Fatalf("couldn't convert to *v1alpha1.ServiceInstance")
	}
	if updateObject.Spec.ServiceClassRef == nil || updateObject.Spec.ServiceClassRef.Name != "SCGUID" {
		t.Fatalf("ServiceClassRef was not resolved correctly during reconcile")
	}
	if updateObject.Spec.ServicePlanRef == nil || updateObject.Spec.ServicePlanRef.Name != "PGUID" {
		t.Fatalf("ServicePlanRef was not resolved correctly during reconcile")
	}

	updatedServiceInstance = assertUpdateStatus(t, actions[3], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, testServicePlanName, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[4], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, testServicePlanName, instance)

	// verify no kube resources created
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "namespaces", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := apiv1.EventTypeNormal + " " + successProvisionReason + " " + "The instance was provisioned successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v\nExpected: %v", a, e)
	}
}

// TestReconcileServiceInstanceResolvesReferences tests a simple successful
// reconciliation and making sure that the ServicePlanRef is correctly
// resolved if the ServiceClassRef is already set.
func TestReconcileServiceInstanceResolvesReferencesServiceClassRefAlreadySet(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Response: &osb.ProvisionResponse{},
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sc := getTestServiceClass()
	sharedInformers.ServiceClasses().Informer().GetStore().Add(sc)
	sp := getTestServicePlan()
	sharedInformers.ServicePlans().Informer().GetStore().Add(sp)

	instance := getTestServiceInstance()
	instance.Spec.ServiceClassRef = &apiv1.ObjectReference{
		Name: serviceClassGUID,
	}

	var scItems []v1alpha1.ServiceClass
	scItems = append(scItems, *sc)
	fakeCatalogClient.AddReactor("list", "serviceclasses", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1alpha1.ServiceClassList{Items: scItems}, nil
	})

	var spItems []v1alpha1.ServicePlan
	spItems = append(spItems, *sp)
	fakeCatalogClient.AddReactor("list", "serviceplans", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1alpha1.ServicePlanList{Items: spItems}, nil
	})

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        instanceGUID,
		ServiceID:         serviceClassGUID,
		PlanID:            planGUID,
		Context: map[string]interface{}{
			"platform":  "kubernetes",
			"namespace": "test-ns",
		},
	})

	// We should get the following actions:
	// list call for ServicePlan
	// setReferences on ServiceInstance
	// updateStatus for inprogress
	// updateStatus for success
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 4)

	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.ParseSelectorOrDie("spec.externalName=test-plan,spec.serviceBrokerName=test-broker,spec.serviceClassRef.name=SCGUID"),
	}
	assertList(t, actions[0], &v1alpha1.ServicePlan{}, listRestrictions)

	updatedServiceInstance := assertUpdateReference(t, actions[1], instance)

	updateObject, ok := updatedServiceInstance.(*v1alpha1.ServiceInstance)
	if !ok {
		t.Fatalf("couldn't convert to *v1alpha1.ServiceInstance")
	}
	if updateObject.Spec.ServiceClassRef == nil || updateObject.Spec.ServiceClassRef.Name != "SCGUID" {
		t.Fatalf("ServiceClassRef was not resolved correctly during reconcile")
	}
	if updateObject.Spec.ServicePlanRef == nil || updateObject.Spec.ServicePlanRef.Name != "PGUID" {
		t.Fatalf("ServicePlanRef was not resolved correctly during reconcile")
	}

	updatedServiceInstance = assertUpdateStatus(t, actions[2], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, testServicePlanName, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[3], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, testServicePlanName, instance)

	// verify no kube resources created
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "namespaces", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := apiv1.EventTypeNormal + " " + successProvisionReason + " " + "The instance was provisioned successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v\nExpected: %v", a, e)
	}
}

// TestReconcileServiceInstanceWithInvalidParameters tests that reconcileInstance
// fails with an error when the service parameters are invalid
func TestReconcileServiceInstanceWithInvalidParameters(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceWithRefs()
	parameters := instanceParameters{Name: "test-param", Args: make(map[string]string)}
	parameters.Args["first"] = "first-arg"
	parameters.Args["second"] = "second-arg"

	b, err := json.Marshal(parameters)
	if err != nil {
		t.Fatalf("Failed to marshal parameters %v : %v", parameters, err)
	}
	// corrupt the byte slice to begin with a '!' instead of an opening JSON bracket '{'
	b[0] = 0x21
	instance.Spec.Parameters = &runtime.RawExtension{Raw: b}

	if err = testController.reconcileServiceInstance(instance); err == nil {
		t.Fatalf("this should fail due to a parse error")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says that the parameters were invalid.
	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceErrorBeforeRequest(t, updatedServiceInstance, errorWithParameters, instance)

	// only action should be a get on the namespace
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 1)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := apiv1.EventTypeWarning + " " + errorWithParameters + " " + "Failed to prepare ServiceInstance parameters"
	if e, a := expectedEvent, events[0]; !strings.Contains(a, e) { // event contains RawExtension, so just compare error message
		t.Fatalf("Received unexpected event: %v\nExpected: %v", a, e)
	}
}

// TestReconcileServiceInstanceWithProvisionCallFailure tests that when the provision
// call to the broker fails, the ready condition becomes false, and the
// failure condition is not set.
func TestReconcileServiceInstanceWithProvisionCallFailure(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Error: errors.New("fake creation failure"),
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceWithRefs()

	if err := testController.reconcileServiceInstance(instance); err == nil {
		t.Fatalf("Should not be able to make the ServiceInstance.")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        instanceGUID,
		ServiceID:         serviceClassGUID,
		PlanID:            planGUID,
		Context: map[string]interface{}{
			"platform":  "kubernetes",
			"namespace": "test-ns",
		},
	})

	// verify no kube resources created
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "namespaces", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, testServicePlanName, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceRequestRetriableError(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, errorErrorCallingProvisionReason, testServicePlanName, instance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := apiv1.EventTypeWarning + " " + errorErrorCallingProvisionReason + " " + "Error provisioning ServiceInstance \"test-ns/test-instance\" of ServiceClass \"test-serviceclass\" at ServiceBroker \"test-broker\": fake creation failure"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v\nExpected: %v", a, e)
	}
}

// TestReconcileServiceInstanceWithProvisionFailure tests that when the provision
// call to the broker fails with an HTTP error, the ready condition becomes
// false, and the failure condition is set.
func TestReconcileServiceInstanceWithProvisionFailure(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Error: osb.HTTPStatusCodeError{
				StatusCode:   http.StatusConflict,
				ErrorMessage: strPtr("OutOfQuota"),
				Description:  strPtr("You're out of quota!"),
			},
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceWithRefs()

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        instanceGUID,
		ServiceID:         serviceClassGUID,
		PlanID:            planGUID,
		Context: map[string]interface{}{
			"platform":  "kubernetes",
			"namespace": "test-ns",
		},
	})

	// verify one kube action occurred
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "namespaces", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, testServicePlanName, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceRequestFailingErrorNoOrphanMitigation(
		t,
		updatedServiceInstance,
		v1alpha1.ServiceInstanceOperationProvision,
		errorProvisionCallFailedReason,
		"ServiceBrokerReturnedFailure",
		instance,
	)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := apiv1.EventTypeWarning + " " + errorProvisionCallFailedReason + " " + "Error provisioning ServiceInstance \"test-ns/test-instance\" of ServiceClass \"test-serviceclass\" at ServiceBroker \"test-broker\": Status: 409; ErrorMessage: OutOfQuota; Description: You're out of quota!; ResponseError: <nil>"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v\nExpected: %v", a, e)
	}
}

// TestReconcileServiceInstance tests synchronously provisioning a new service
func TestReconcileServiceInstance(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Response: &osb.ProvisionResponse{
				DashboardURL: &testDashboardURL,
			},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceWithRefs()

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        instanceGUID,
		ServiceID:         serviceClassGUID,
		PlanID:            planGUID,
		OrganizationGUID:  testNsUID,
		SpaceGUID:         testNsUID,
		Context: map[string]interface{}{
			"platform":  "kubernetes",
			"namespace": "test-ns",
		},
	})

	instanceKey := testNamespace + "/" + testServiceInstanceName

	// Since synchronous operation, must not make it into the polling queue.
	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	// verify no kube resources created.
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "namespaces", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, testServicePlanName, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, testServicePlanName, instance)
	assertServiceInstanceDashboardURL(t, updatedServiceInstance, testDashboardURL)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := apiv1.EventTypeNormal + " " + successProvisionReason + " " + successProvisionMessage
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v\nExpected: %v", a, e)
	}
}

// TestReconcileServiceInstanceAsynchronous tests provisioning a new service where
// the request results in a async response.  Resulting status will indicate
// not ready and polling in progress.
func TestReconcileServiceInstanceAsynchronous(t *testing.T) {
	key := osb.OperationKey(testOperation)
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Response: &osb.ProvisionResponse{
				Async:        true,
				DashboardURL: &testDashboardURL,
				OperationKey: &key,
			},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceWithRefs()
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        instanceGUID,
		ServiceID:         serviceClassGUID,
		PlanID:            planGUID,
		OrganizationGUID:  testNsUID,
		SpaceGUID:         testNsUID,
		Context: map[string]interface{}{
			"platform":  "kubernetes",
			"namespace": testNamespace,
		},
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, testServicePlanName, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceAsyncInProgress(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, testOperation, testServicePlanName, instance)
	assertServiceInstanceDashboardURL(t, updatedServiceInstance, testDashboardURL)

	// verify no kube resources created.
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	if e, a := 1, len(kubeActions); e != a {
		t.Fatalf("Unexpected number of actions: expected %v, got %v", e, a)
	}

	if testController.pollingQueue.NumRequeues(instanceKey) != 1 {
		t.Fatalf("Expected polling queue to have a record of seeing test instance once")
	}
}

// TestReconcileServiceInstanceAsynchronousNoOperation tests an async provision
// scenario.  This differs from TestReconcileServiceInstanceAsynchronous() as
// there is no operation key returned by OSB.
func TestReconcileServiceInstanceAsynchronousNoOperation(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Response: &osb.ProvisionResponse{
				Async:        true,
				DashboardURL: &testDashboardURL,
			},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceWithRefs()
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        instanceGUID,
		ServiceID:         serviceClassGUID,
		PlanID:            planGUID,
		OrganizationGUID:  testNsUID,
		SpaceGUID:         testNsUID,
		Context: map[string]interface{}{
			"platform":  "kubernetes",
			"namespace": "test-ns",
		},
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, testServicePlanName, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceAsyncInProgress(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, "", testServicePlanName, instance)
	assertServiceInstanceDashboardURL(t, updatedServiceInstance, testDashboardURL)

	// verify no kube resources created.
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	if e, a := 1, len(kubeActions); e != a {
		t.Fatalf("Unexpected number of actions: expected %v, got %v", e, a)
	}

	if testController.pollingQueue.NumRequeues(instanceKey) != 1 {
		t.Fatalf("Expected polling queue to have a record of seeing test instance once")
	}
}

// TestReconcileServiceInstanceNamespaceError test reconciling an instance where kube
// client fails to get a namespace to create instance in.
func TestReconcileServiceInstanceNamespaceError(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	fakeKubeClient.AddReactor("get", "namespaces", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &apiv1.Namespace{}, errors.New("No namespace")
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceWithRefs()

	if err := testController.reconcileServiceInstance(instance); err == nil {
		t.Fatalf("There should not be a namespace for the ServiceInstance to be created in.")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 0)

	// verify no kube resources created.
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "namespaces", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceErrorBeforeRequest(t, updatedServiceInstance, errorFindingNamespaceServiceInstanceReason, instance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := apiv1.EventTypeWarning + " " + errorFindingNamespaceServiceInstanceReason + " " + "Failed to get namespace \"test-ns\" during instance create: No namespace"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v\nExpected: %v", a, e)
	}
}

// TestReconcileServiceInstanceDelete tests deleting/deprovisioning an instance
func TestReconcileServiceInstanceDelete(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		DeprovisionReaction: &fakeosb.DeprovisionReaction{
			Response: &osb.DeprovisionResponse{},
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceWithRefs()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{v1alpha1.FinalizerServiceCatalog}
	// we only invoke the broker client to deprovision if we have a reconciled generation set
	// as that implies a previous success.
	instance.Generation = 2
	instance.Status.ReconciledGeneration = 1
	instance.Status.ExternalProperties = &v1alpha1.ServiceInstancePropertiesState{}

	fakeCatalogClient.AddReactor("get", "serviceinstances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, instance, nil
	})

	err := testController.reconcileServiceInstance(instance)
	if err != nil {
		t.Fatalf("This should not fail")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	assertDeprovision(t, brokerActions[0], &osb.DeprovisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        instanceGUID,
		ServiceID:         serviceClassGUID,
		PlanID:            planGUID,
	})

	// Verify no core kube actions occurred
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationDeprovision, testServicePlanName, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationDeprovision, testServicePlanName, instance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := apiv1.EventTypeNormal + " " + successDeprovisionReason + " " + "The instance was deprovisioned successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v\nExpected: %v", a, e)
	}
}

// TestReconcileServiceInstanceDeleteBlockedByCredentials tests
// deleting/deprovisioning an instance that has ServiceInstanceCredentials.
// Instance reconcilation will set the Ready condition to false with a msg
// indicating the delete is blocked until the credentials are removed.
func TestReconcileServiceInstanceDeleteBlockedByCredentials(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		DeprovisionReaction: &fakeosb.DeprovisionReaction{
			Response: &osb.DeprovisionResponse{},
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())
	credentials := getTestServiceInstanceCredential()
	sharedInformers.ServiceInstanceCredentials().Informer().GetStore().Add(credentials)

	instance := getTestServiceInstanceWithRefs()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{v1alpha1.FinalizerServiceCatalog}
	// we only invoke the broker client to deprovision if we have a reconciled generation set
	// as that implies a previous success.
	instance.Generation = 2
	instance.Status.ReconciledGeneration = 1
	instance.Status.ExternalProperties = &v1alpha1.ServiceInstancePropertiesState{}

	fakeCatalogClient.AddReactor("get", "serviceinstances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, instance, nil
	})

	err := testController.reconcileServiceInstance(instance)
	if err != nil {
		t.Fatalf("reconcileServiceInstance() returned an error:  %v", err.Error())
	}

	brokerActions := fakeBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 0)

	// Verify no core kube actions occurred
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceErrorBeforeRequest(t, updatedServiceInstance, "DeprovisionBlockedByExistingCredentials", instance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := apiv1.EventTypeWarning + " " + "DeprovisionBlockedByExistingCredentials Delete instance test-ns/test-instance blocked by existing ServiceInstanceCredentials associated with this instance.  All credentials must be removed first."
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v\nExpected: %v", a, e)
	}

	// delete credentials
	sharedInformers.ServiceInstanceCredentials().Informer().GetStore().Delete(credentials)

	fakeKubeClient.ClearActions()
	fakeCatalogClient.ClearActions()

	// credentials were removed, verify the next reconcilation removes
	// the instance

	err = testController.reconcileServiceInstance(instance)
	if err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions = fakeBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	assertDeprovision(t, brokerActions[0], &osb.DeprovisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        instanceGUID,
		ServiceID:         serviceClassGUID,
		PlanID:            planGUID,
	})

	// Verify no core kube actions occurred
	kubeActions = fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions = fakeCatalogClient.Actions()

	// The actions should be:
	// 0. Updating the current operation
	// 1. Updating the ready condition
	assertNumberOfActions(t, actions, 2)

	updatedServiceInstance = assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationDeprovision, testServicePlanName, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationDeprovision, testServicePlanName, instance)

	events = getRecordedEvents(testController)
	assertNumEvents(t, events, 1)
	expectedEvent = apiv1.EventTypeNormal + " " + successDeprovisionReason + " " + "The instance was deprovisioned successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v\nExpected: %v", a, e)
	}
}

func TestReconcileServiceInstanceDeleteAsynchronous(t *testing.T) {
	key := osb.OperationKey(testOperation)
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		DeprovisionReaction: &fakeosb.DeprovisionReaction{
			Response: &osb.DeprovisionResponse{
				Async:        true,
				OperationKey: &key,
			},
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceWithRefs()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{v1alpha1.FinalizerServiceCatalog}
	// we only invoke the broker client to deprovision if we have a reconciled generation set
	// as that implies a previous success.
	instance.Generation = 2
	instance.Status.ReconciledGeneration = 1
	instance.Status.ExternalProperties = &v1alpha1.ServiceInstancePropertiesState{}

	fakeCatalogClient.AddReactor("get", "serviceinstances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, instance, nil
	})

	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.reconcileServiceInstance(instance)
	if err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	// The item should've been added to the pollingQueue for later processing

	if testController.pollingQueue.NumRequeues(instanceKey) != 1 {
		t.Fatalf("Expected polling queue to have a record of seeing test instance once")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	assertDeprovision(t, brokerActions[0], &osb.DeprovisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        instanceGUID,
		ServiceID:         serviceClassGUID,
		PlanID:            planGUID,
	})

	// Verify no core kube actions occurred
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationDeprovision, testServicePlanName, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceAsyncInProgress(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationDeprovision, testOperation, testServicePlanName, instance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := apiv1.EventTypeNormal + " " + asyncDeprovisioningReason + " " + "The instance is being deprovisioned asynchronously"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v\nExpected: %v", a, e)
	}
}

// TestReconcileServiceInstanceDeleteFailedInstance tests that a failed instance will
// be finalized, but no deprovision request will be sent to the broker.
func TestReconcileServiceInstanceDeleteFailedInstance(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceWithFailedStatus()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{v1alpha1.FinalizerServiceCatalog}
	instance.Status.ExternalProperties = &v1alpha1.ServiceInstancePropertiesState{}

	// we only invoke the broker client to deprovision if we have a reconciled generation set
	// as that implies a previous success.
	instance.Generation = 2
	instance.Status.ReconciledGeneration = 1

	fakeCatalogClient.AddReactor("get", "serviceinstances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, instance, nil
	})

	err := testController.reconcileServiceInstance(instance)
	if err != nil {
		t.Fatalf("Unexpected error from reconcileServiceInstance: %v", err)
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 0)

	// Verify no core kube actions occurred
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertEmptyFinalizers(t, updatedServiceInstance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 0)
}

// TestReconcileServiceInstanceDeleteDoesNotInvokeServiceBroker verfies that if an instance
// is created that is never actually provisioned the instance is able to be
// deleted and is not blocked by any interaction with a broker (since its very
// likely that a broker never actually existed).
func TestReconcileServiceInstanceDeleteDoesNotInvokeServiceBroker(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceWithRefs()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{v1alpha1.FinalizerServiceCatalog}
	instance.Generation = 1
	instance.Status.ReconciledGeneration = 0

	fakeCatalogClient.AddReactor("get", "serviceinstances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, instance, nil
	})

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 0)

	// Verify no core kube actions occurred
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	// The one actions should be:
	// 0. Removing the finalizer
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertEmptyFinalizers(t, updatedServiceInstance)

	// no events because no external deprovision was needed
	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 0)
}

// TestReconcileServiceInstanceWithFailureCondition tests reconciling an instance that
// has a status condition set to failure.
func TestReconcileServiceInstanceWithFailureCondition(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceWithFailedStatus()

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 0)

	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 0)

	// verify no actions on the kube client
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 0)
}

// TestPollServiceInstanceInProgressProvisioningWithOperation tests polling an
// instance that is already in process of provisioning (background/
// asynchronously) and is still in progress (should be re-polled)
func TestPollServiceInstanceInProgressProvisioningWithOperation(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State:       osb.StateInProgress,
				Description: strPtr(lastOperationDescription),
			},
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceAsyncProvisioning(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.pollServiceInstanceInternal(instance)
	if err != nil {
		t.Fatalf("pollServiceInstanceInternal failed: %s", err)
	}

	if testController.pollingQueue.NumRequeues(instanceKey) != 1 {
		t.Fatalf("Expected polling queue to have record of seeing test instance once")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   instanceGUID,
		ServiceID:    strPtr(serviceClassGUID),
		PlanID:       strPtr(planGUID),
		OperationKey: &operationKey,
	})

	// there should have been 1 action to update the status with the last operation description
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceAsyncInProgress(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, testOperation, testServicePlanName, instance)
	assertServiceInstanceConditionHasLastOperationDescription(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, lastOperationDescription)

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)
}

// TestPollServiceInstanceSuccessProvisioningWithOperation tests polling an
// instance that is already in process of provisioning (background/
// asynchronously) and is found to be ready
func TestPollServiceInstanceSuccessProvisioningWithOperation(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State:       osb.StateSucceeded,
				Description: strPtr(lastOperationDescription),
			},
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceAsyncProvisioning(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.pollServiceInstanceInternal(instance)
	if err != nil {
		t.Fatalf("pollServiceInstanceInternal failed: %s", err)
	}

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance as polling should have completed")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   instanceGUID,
		ServiceID:    strPtr(serviceClassGUID),
		PlanID:       strPtr(planGUID),
		OperationKey: &operationKey,
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, testServicePlanName, instance)
}

// TestPollServiceInstanceFailureProvisioningWithOperation tests polling an
// instance where provision was in process asynchronously but has an updated
// status of failed to provision.
func TestPollServiceInstanceFailureProvisioningWithOperation(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State: osb.StateFailed,
			},
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceAsyncProvisioning(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.pollServiceInstanceInternal(instance)
	if err != nil {
		t.Fatalf("pollServiceInstanceInternal failed: %s", err)
	}

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance as polling should have completed")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   instanceGUID,
		ServiceID:    strPtr(serviceClassGUID),
		PlanID:       strPtr(planGUID),
		OperationKey: &operationKey,
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceRequestFailingErrorNoOrphanMitigation(
		t,
		updatedServiceInstance,
		v1alpha1.ServiceInstanceOperationProvision,
		errorProvisionCallFailedReason,
		errorProvisionCallFailedReason,
		instance,
	)
}

// TestPollServiceInstanceInProgressDeprovisioningWithOperationNoFinalizer tests
// polling an instance that was asynchronously being deprovisioned and is still
// in progress.
func TestPollServiceInstanceInProgressDeprovisioningWithOperationNoFinalizer(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State:       osb.StateInProgress,
				Description: strPtr(lastOperationDescription),
			},
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceAsyncDeprovisioning(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.pollServiceInstanceInternal(instance)
	if err != nil {
		t.Fatalf("pollServiceInstanceInternal failed: %s", err)
	}

	if testController.pollingQueue.NumRequeues(instanceKey) != 1 {
		t.Fatalf("Expected polling queue to have record of seeing test instance once")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   instanceGUID,
		ServiceID:    strPtr(serviceClassGUID),
		PlanID:       strPtr(planGUID),
		OperationKey: &operationKey,
	})

	// there should have been 1 action to update the instance status with the last operation
	// description
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceAsyncInProgress(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationDeprovision, testOperation, testServicePlanName, instance)
	assertServiceInstanceConditionHasLastOperationDescription(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationDeprovision, lastOperationDescription)

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)
}

// TestPollServiceInstanceSuccessDeprovisioningWithOperationNoFinalizer tests
// polling an instance that was asynchronously being deprovisioned and its
// current poll status succeeded.  Verify instance is deprovisioned.
func TestPollServiceInstanceSuccessDeprovisioningWithOperationNoFinalizer(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State: osb.StateSucceeded,
			},
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceAsyncDeprovisioning(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.pollServiceInstanceInternal(instance)
	if err != nil {
		t.Fatalf("pollServiceInstanceInternal failed: %s", err)
	}

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance as polling should have completed")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   instanceGUID,
		ServiceID:    strPtr(serviceClassGUID),
		PlanID:       strPtr(planGUID),
		OperationKey: &operationKey,
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationDeprovision, testServicePlanName, instance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)
	expectedEvent := apiv1.EventTypeNormal + " " + successDeprovisionReason + " " + "The instance was deprovisioned successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %vExpected: %v", a, e)
	}
}

// TestPollServiceInstanceFailureDeprovisioningWithOperation tests polling an
// instance that has a async deprovision in progress.  Current poll status is
// failed.  Verify instance state is set to unknown.
func TestPollServiceInstanceFailureDeprovisioningWithOperation(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State: osb.StateFailed,
			},
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceAsyncDeprovisioning(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.pollServiceInstanceInternal(instance)
	if err != nil {
		t.Fatalf("pollServiceInstanceInternal failed: %s", err)
	}

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance as polling should have completed")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   instanceGUID,
		ServiceID:    strPtr(serviceClassGUID),
		PlanID:       strPtr(planGUID),
		OperationKey: &operationKey,
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceRequestFailingErrorNoOrphanMitigation(
		t,
		updatedServiceInstance,
		v1alpha1.ServiceInstanceOperationDeprovision,
		errorDeprovisionCalledReason,
		errorDeprovisionCalledReason,
		instance,
	)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)
	expectedEvent := apiv1.EventTypeWarning + " " + errorDeprovisionCalledReason + " " + "Error deprovisioning ServiceInstance \"test-ns/test-instance\" of ServiceClass \"test-serviceclass\" at ServiceBroker \"test-broker\": \"\""
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v\nExpected: %v", a, e)
	}

}

// TestPollServiceInstanceStatusGoneDeprovisioningWithOperationNoFinalizer test
// polling an instance that has a async deprovision in progress.  Current poll
// status is Gone (which is fine).  Verify successful deprovisioning.
func TestPollServiceInstanceStatusGoneDeprovisioningWithOperationNoFinalizer(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Error: osb.HTTPStatusCodeError{
				StatusCode: http.StatusGone,
			},
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceAsyncDeprovisioning(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.pollServiceInstanceInternal(instance)
	if err != nil {
		t.Fatalf("pollServiceInstanceInternal failed: %s", err)
	}

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance as polling should have completed")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   instanceGUID,
		ServiceID:    strPtr(serviceClassGUID),
		PlanID:       strPtr(planGUID),
		OperationKey: &operationKey,
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationDeprovision, testServicePlanName, instance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)
	expectedEvent := apiv1.EventTypeNormal + " " + successDeprovisionReason + " " + "The instance was deprovisioned successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v\nExpected: %v", a, e)
	}
}

// TestPollServiceInstanceServiceBrokerError simulates polling a broker and getting a
// Forbidden status on the poll.  Test simulates that the ServiceBroker was already
// in the process of being deleted prior to the Forbidden status.
func TestPollServiceInstanceServiceBrokerError(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Error: osb.HTTPStatusCodeError{
				StatusCode: http.StatusForbidden,
			},
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceAsyncDeprovisioning(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.pollServiceInstanceInternal(instance)
	if err != nil {
		t.Fatalf("pollServiceInstanceInternal failed: %v", err)
	}

	if testController.pollingQueue.NumRequeues(instanceKey) != 1 {
		t.Fatalf("Expected polling queue to have record of seeing test instance once")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   instanceGUID,
		ServiceID:    strPtr(serviceClassGUID),
		PlanID:       strPtr(planGUID),
		OperationKey: &operationKey,
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 0)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)
	expectedEvent := apiv1.EventTypeWarning + " " + errorPollingLastOperationReason + " " + "Error polling last operation for instance test-ns/test-instance: Status code: 403; ErrorMessage: %!q(*string=<nil>); description: %!q(*string=<nil>)"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v\nExpected: %v", a, e)
	}
}

// TestPollServiceInstanceSuccessDeprovisioningWithOperationWithFinalizer tests
// polling with instance while it is in deprovisioning state to ensure after
// the poll the service is properly removed
func TestPollServiceInstanceSuccessDeprovisioningWithOperationWithFinalizer(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State: osb.StateSucceeded,
			},
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceAsyncDeprovisioningWithFinalizer(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName

	// updateServiceInstanceFinalizers fetches the latest object.
	fakeCatalogClient.AddReactor("get", "serviceinstances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, instance, nil
	})

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.pollServiceInstanceInternal(instance)
	if err != nil {
		t.Fatalf("pollServiceInstanceInternal failed: %s", err)
	}

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance as polling should have completed")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   instanceGUID,
		ServiceID:    strPtr(serviceClassGUID),
		PlanID:       strPtr(planGUID),
		OperationKey: &operationKey,
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationDeprovision, testServicePlanName, instance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)
	expectedEvent := apiv1.EventTypeNormal + " " + successDeprovisionReason + " " + "The instance was deprovisioned successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v\nExpected: %v", a, e)
	}
}

// TestReconcileServiceInstanceSuccessOnFinalRetry verifies that reconciliation
// can succeed on the last attempt before timing out of the retry loop
func TestReconcileServiceInstanceSuccessOnFinalRetry(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Response: &osb.ProvisionResponse{},
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceWithRefs()
	instance.Status.CurrentOperation = v1alpha1.ServiceInstanceOperationProvision

	startTime := metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
	instance.Status.OperationStartTime = &startTime

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        instanceGUID,
		ServiceID:         serviceClassGUID,
		PlanID:            planGUID,
		Context: map[string]interface{}{
			"platform":  "kubernetes",
			"namespace": "test-ns",
		},
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, testServicePlanName, instance)

	// verify no kube resources created
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "namespaces", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := apiv1.EventTypeNormal + " " + successProvisionReason + " " + "The instance was provisioned successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v\nExpected: %v", a, e)
	}
}

// TestReconcileServiceInstanceFailureOnFinalRetry verifies that reconciliation
// completes in the event of an error after the retry duration elapses.
func TestReconcileServiceInstanceFailureOnFinalRetry(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Error: errors.New("fake creation failure"),
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceWithRefs()
	instance.Status.CurrentOperation = v1alpha1.ServiceInstanceOperationProvision
	startTime := metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
	instance.Status.OperationStartTime = &startTime

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("Should have returned no error because the retry duration has elapsed: %v", err)
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        instanceGUID,
		ServiceID:         serviceClassGUID,
		PlanID:            planGUID,
		Context: map[string]interface{}{
			"platform":  "kubernetes",
			"namespace": "test-ns",
		},
	})

	// verify no kube resources created
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "namespaces", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceRequestFailingErrorNoOrphanMitigation(
		t,
		updatedServiceInstance,
		v1alpha1.ServiceInstanceOperationProvision,
		errorErrorCallingProvisionReason,
		errorReconciliationRetryTimeoutReason,
		instance,
	)

	expectedEventPrefixes := []string{
		apiv1.EventTypeWarning + " " + errorErrorCallingProvisionReason,
		apiv1.EventTypeWarning + " " + errorReconciliationRetryTimeoutReason,
	}
	events := getRecordedEvents(testController)
	assertNumEvents(t, events, len(expectedEventPrefixes))

	for i, e := range expectedEventPrefixes {
		a := events[i]
		if !strings.HasPrefix(a, e) {
			t.Fatalf("Received unexpected event:\n  expected prefix: %v\n  got: %v", e, a)
		}
	}
}

// TestPollServiceInstanceSuccessOnFinalRetry verifies that polling
// can succeed on the last attempt before timing out of the retry loop
func TestPollServiceInstanceSuccessOnFinalRetry(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State:       osb.StateSucceeded,
				Description: strPtr(lastOperationDescription),
			},
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceAsyncProvisioning(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName
	startTime := metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
	instance.Status.OperationStartTime = &startTime

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	if err := testController.pollServiceInstanceInternal(instance); err != nil {
		t.Fatalf("pollServiceInstanceInternal failed: %s", err)
	}

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance as polling should have completed")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   instanceGUID,
		ServiceID:    strPtr(serviceClassGUID),
		PlanID:       strPtr(planGUID),
		OperationKey: &operationKey,
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, testServicePlanName, instance)
}

// TestPollServiceInstanceFailureOnFinalRetry verifies that polling
// completes in the event of an error after the retry duration elapses.
func TestPollServiceInstanceFailureOnFinalRetry(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State:       osb.StateInProgress,
				Description: strPtr(lastOperationDescription),
			},
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceAsyncProvisioning(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName
	startTime := metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
	instance.Status.OperationStartTime = &startTime

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	if err := testController.pollServiceInstanceInternal(instance); err != nil {
		t.Fatalf("Should have return no error because the retry duration has elapsed: %v", err)
	}

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance as polling should have completed")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   instanceGUID,
		ServiceID:    strPtr(serviceClassGUID),
		PlanID:       strPtr(planGUID),
		OperationKey: &operationKey,
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceRequestFailingErrorStartOrphanMitigation(
		t,
		updatedServiceInstance,
		v1alpha1.ServiceInstanceOperationProvision,
		asyncProvisioningReason,
		errorReconciliationRetryTimeoutReason,
		instance,
	)
	assertServiceInstanceConditionHasLastOperationDescription(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, lastOperationDescription)

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)
}

// TestReconcileServiceInstanceWithStatusUpdateError verifies that the reconciler
// returns an error when there is a conflict updating the status of the resource.
// This is an otherwise successful scenario where the update to set the
// in-progress operation fails.
func TestReconcileServiceInstanceWithStatusUpdateError(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	addGetNamespaceReaction(fakeKubeClient)

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceWithRefs()

	fakeCatalogClient.AddReactor("update", "serviceinstances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("update error")
	})

	err := testController.reconcileServiceInstance(instance)
	if err == nil {
		t.Fatalf("expected error from but got none")
	}
	if e, a := "update error", err.Error(); e != a {
		t.Fatalf("unexpected error returned: expected %q, got %q", e, a)
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, testServicePlanName, instance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 0)
}

// TestSetServiceInstanceCondition ensures that with the expected conditions the
// SetServiceInstanceCondition() updates a status properly with the given condition
// The test cases are proving:
// - status with no existing conditions accepts new condition of Ready=False
//   and updates the timestamp
// - status with existing Ready=False condition accepts new condition of
//   Ready=False with no timestamp change
// - status with existing Ready=False condition accepts new condition of
//   Ready=False  with reason & msg change and results with no timestamp change
// - status with existing Ready=False condition accepts new condition of
//   Ready=True  and reflects new timestamp
// - status with existing Ready=True condition accepts new condition of
//   Ready=True with no timestamp change
// - status with existing Ready=True condition accepts new condition of
//   Ready=False and reflects new timestamp
// - status with existing Ready=False condition accepts new condition of
//   Failed=True  and reflects Ready=False, Failed=True, new timestamp
func TestSetServiceInstanceCondition(t *testing.T) {
	instanceWithCondition := func(condition *v1alpha1.ServiceInstanceCondition) *v1alpha1.ServiceInstance {
		instance := getTestServiceInstance()
		instance.Status = v1alpha1.ServiceInstanceStatus{
			Conditions: []v1alpha1.ServiceInstanceCondition{*condition},
		}

		return instance
	}

	// The value of the LastTransitionTime field on conditions has to be
	// tested to ensure it is updated correctly.
	//
	// Time basis for all condition changes:
	newTs := metav1.Now()
	oldTs := metav1.NewTime(newTs.Add(-5 * time.Minute))

	// condition is a shortcut method for creating conditions with the 'old' timestamp.
	condition := func(cType v1alpha1.ServiceInstanceConditionType, status v1alpha1.ConditionStatus, s ...string) *v1alpha1.ServiceInstanceCondition {
		c := &v1alpha1.ServiceInstanceCondition{
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

	readyFalse := func() *v1alpha1.ServiceInstanceCondition {
		return condition(v1alpha1.ServiceInstanceConditionReady, v1alpha1.ConditionFalse, "Reason", "Message")
	}

	readyFalsef := func(reason, message string) *v1alpha1.ServiceInstanceCondition {
		return condition(v1alpha1.ServiceInstanceConditionReady, v1alpha1.ConditionFalse, reason, message)
	}

	readyTrue := func() *v1alpha1.ServiceInstanceCondition {
		return condition(v1alpha1.ServiceInstanceConditionReady, v1alpha1.ConditionTrue, "Reason", "Message")
	}

	failedTrue := func() *v1alpha1.ServiceInstanceCondition {
		return condition(v1alpha1.ServiceInstanceConditionFailed, v1alpha1.ConditionTrue, "Reason", "Message")
	}

	// withNewTs sets the LastTransitionTime to the 'new' basis time and
	// returns it.
	withNewTs := func(c *v1alpha1.ServiceInstanceCondition) *v1alpha1.ServiceInstanceCondition {
		c.LastTransitionTime = newTs
		return c
	}

	// this test works by calling setServiceInstanceCondition with the input and
	// condition fields of the test case, and ensuring that afterward the
	// input (which is mutated by the setServiceInstanceCondition call) is deep-equal
	// to the test case result.
	//
	// take note of where withNewTs is used when declaring the result to
	// indicate that the LastTransitionTime field on a condition should have
	// changed.
	//
	// name: short description of the test
	// input: instance status
	// condition: condition  to set
	// result: expected instance result
	cases := []struct {
		name      string
		input     *v1alpha1.ServiceInstance
		condition *v1alpha1.ServiceInstanceCondition
		result    *v1alpha1.ServiceInstance
	}{
		{
			name:      "new ready condition",
			input:     getTestServiceInstance(),
			condition: readyFalse(),
			result:    instanceWithCondition(withNewTs(readyFalse())),
		},
		{
			name:      "not ready -> not ready; no ts update",
			input:     instanceWithCondition(readyFalse()),
			condition: readyFalse(),
			result:    instanceWithCondition(readyFalse()),
		},
		{
			name:      "not ready -> not ready, reason and message change; no ts update",
			input:     instanceWithCondition(readyFalse()),
			condition: readyFalsef("DifferentReason", "DifferentMessage"),
			result:    instanceWithCondition(readyFalsef("DifferentReason", "DifferentMessage")),
		},
		{
			name:      "not ready -> ready",
			input:     instanceWithCondition(readyFalse()),
			condition: readyTrue(),
			result:    instanceWithCondition(withNewTs(readyTrue())),
		},
		{
			name:      "ready -> ready; no ts update",
			input:     instanceWithCondition(readyTrue()),
			condition: readyTrue(),
			result:    instanceWithCondition(readyTrue()),
		},
		{
			name:      "ready -> not ready",
			input:     instanceWithCondition(readyTrue()),
			condition: readyFalse(),
			result:    instanceWithCondition(withNewTs(readyFalse())),
		},
		{
			name:      "not ready -> not ready + failed",
			input:     instanceWithCondition(readyFalse()),
			condition: failedTrue(),
			result: func() *v1alpha1.ServiceInstance {
				i := instanceWithCondition(readyFalse())
				i.Status.Conditions = append(i.Status.Conditions, *withNewTs(failedTrue()))
				return i
			}(),
		},
	}

	for _, tc := range cases {
		setServiceInstanceConditionInternal(tc.input, tc.condition.Type, tc.condition.Status, tc.condition.Reason, tc.condition.Message, newTs)

		if !reflect.DeepEqual(tc.input, tc.result) {
			t.Errorf("%v: unexpected diff: %v", tc.name, diff.ObjectReflectDiff(tc.input, tc.result))
		}
	}
}

// TestUpdateServiceInstanceCondition ensures that with the expected conditions the
// updateServiceInstanceCondition() results in a correct status & associated
// conditions and the expected client actions are verified test cases prove:
// - initially unset status accepts a Ready=False and results in time change
// - initially Ready=False accepts a Ready=False with new null msg update and results in no time change
// - initially Ready=False accepts a Ready=False update with new reason and msg and results in no time change
// - initially Ready=False accepts a Ready=True update with msg and results in time change
// - initially Ready=True accepts a Ready=True update with msg and results in no time change
// - initially Ready=True accepts a Ready=False update with msg and results in time change
// - initially Ready=True accepts a Ready=False update with new msg and results in time change
func TestUpdateServiceInstanceCondition(t *testing.T) {
	getTestServiceInstanceWithStatus := func(status v1alpha1.ConditionStatus) *v1alpha1.ServiceInstance {
		instance := getTestServiceInstance()
		instance.Status = v1alpha1.ServiceInstanceStatus{
			Conditions: []v1alpha1.ServiceInstanceCondition{{
				Type:               v1alpha1.ServiceInstanceConditionReady,
				Status:             status,
				Message:            "message",
				LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
			}},
		}

		return instance
	}

	// name: short description of the test
	// input: instance status
	// condition: condition  to set
	// reason: reason text
	// message: message text
	// transitionTimeChanged: true/false indicating if the test should result in an updated transition time change
	cases := []struct {
		name                  string
		input                 *v1alpha1.ServiceInstance
		status                v1alpha1.ConditionStatus
		reason                string
		message               string
		transitionTimeChanged bool
	}{

		{
			name:                  "initially unset",
			input:                 getTestServiceInstance(),
			status:                v1alpha1.ConditionFalse,
			message:               "message",
			transitionTimeChanged: true,
		},
		{
			name:                  "not ready -> not ready",
			input:                 getTestServiceInstanceWithStatus(v1alpha1.ConditionFalse),
			status:                v1alpha1.ConditionFalse,
			transitionTimeChanged: false,
		},
		{
			name:                  "not ready -> not ready, reason and message change",
			input:                 getTestServiceInstanceWithStatus(v1alpha1.ConditionFalse),
			status:                v1alpha1.ConditionFalse,
			reason:                "foo",
			message:               "bar",
			transitionTimeChanged: false,
		},
		{
			name:                  "not ready -> ready",
			input:                 getTestServiceInstanceWithStatus(v1alpha1.ConditionFalse),
			status:                v1alpha1.ConditionTrue,
			message:               "message",
			transitionTimeChanged: true,
		},
		{
			name:                  "ready -> ready",
			input:                 getTestServiceInstanceWithStatus(v1alpha1.ConditionTrue),
			status:                v1alpha1.ConditionTrue,
			message:               "message",
			transitionTimeChanged: false,
		},
		{
			name:                  "ready -> not ready",
			input:                 getTestServiceInstanceWithStatus(v1alpha1.ConditionTrue),
			status:                v1alpha1.ConditionFalse,
			message:               "message",
			transitionTimeChanged: true,
		},
		{
			name:                  "message -> message2",
			input:                 getTestServiceInstanceWithStatus(v1alpha1.ConditionTrue),
			status:                v1alpha1.ConditionFalse,
			message:               "message2",
			transitionTimeChanged: true,
		},
	}

	for _, tc := range cases {
		_, fakeCatalogClient, fakeServiceBrokerClient, testController, _ := newTestController(t, noFakeActions())

		clone, err := api.Scheme.DeepCopy(tc.input)
		if err != nil {
			t.Errorf("%v: deep copy failed", tc.name)
			continue
		}
		inputClone := clone.(*v1alpha1.ServiceInstance)

		err = testController.updateServiceInstanceCondition(tc.input, v1alpha1.ServiceInstanceConditionReady, tc.status, tc.reason, tc.message)
		if err != nil {
			t.Errorf("%v: error updating instance condition: %v", tc.name, err)
			continue
		}

		brokerActions := fakeServiceBrokerClient.Actions()
		assertNumberOfServiceBrokerActions(t, brokerActions, 0)

		if !reflect.DeepEqual(tc.input, inputClone) {
			t.Errorf("%v: updating broker condition mutated input: expected %v, got %v", tc.name, inputClone, tc.input)
			continue
		}

		actions := fakeCatalogClient.Actions()
		if ok := expectNumberOfActions(t, tc.name, actions, 1); !ok {
			continue
		}

		updatedServiceInstance, ok := expectUpdateStatus(t, tc.name, actions[0], tc.input)
		if !ok {
			continue
		}

		updateActionObject, ok := updatedServiceInstance.(*v1alpha1.ServiceInstance)
		if !ok {
			t.Errorf("%v: couldn't convert to instance", tc.name)
			continue
		}

		var initialTs metav1.Time
		if len(inputClone.Status.Conditions) != 0 {
			initialTs = inputClone.Status.Conditions[0].LastTransitionTime
		}

		if e, a := 1, len(updateActionObject.Status.Conditions); e != a {
			t.Errorf("%v: expected %v condition(s), got %v", tc.name, e, a)
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
			t.Errorf("%v: condition reasons didn't match; expected %v, got %v", tc.name, e, a)
			continue
		}
		if e, a := tc.message, outputCondition.Message; e != "" && e != a {
			t.Errorf("%v: condition reasons didn't match; expected %v, got %v", tc.name, e, a)
		}
	}
}

func TestReconcileInstanceUsingOriginatingIdentity(t *testing.T) {
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
				ProvisionReaction: &fakeosb.ProvisionReaction{
					Response: &osb.ProvisionResponse{
						DashboardURL: &testDashboardURL,
					},
				},
			})

			addGetNamespaceReaction(fakeKubeClient)

			sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
			sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
			sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

			instance := getTestServiceInstanceWithRefs()
			if tc.includeUserInfo {
				instance.Spec.UserInfo = testUserInfo
			}

			if err := testController.reconcileServiceInstance(instance); err != nil {
				t.Fatalf("This should not fail : %v", err)
			}

			brokerActions := fakeBrokerClient.Actions()
			assertNumberOfServiceBrokerActions(t, brokerActions, 1)
			actualRequest, ok := brokerActions[0].Request.(*osb.ProvisionRequest)
			if !ok {
				t.Errorf("%v: unexpected request type; expected %T, got %T", tc.name, &osb.ProvisionRequest{}, actualRequest)
				return
			}
			var expectedOriginatingIdentity *osb.AlphaOriginatingIdentity
			if tc.expectedOriginatingIdentity {
				expectedOriginatingIdentity = testOriginatingIdentity
			}
			assertOriginatingIdentity(t, expectedOriginatingIdentity, actualRequest.OriginatingIdentity)
		}()
	}
}

func TestReconcileInstanceDeleteUsingOriginatingIdentity(t *testing.T) {
	for _, tc := range originatingIdentityTestCases {
		func() {
			if tc.enableOriginatingIdentity {
				utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=true", scfeatures.OriginatingIdentity))
				defer utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=false", scfeatures.OriginatingIdentity))
			}

			_, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
				DeprovisionReaction: &fakeosb.DeprovisionReaction{
					Response: &osb.DeprovisionResponse{},
				},
			})

			sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
			sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
			sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

			instance := getTestServiceInstanceWithRefs()
			instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
			instance.ObjectMeta.Finalizers = []string{v1alpha1.FinalizerServiceCatalog}
			// we only invoke the broker client to deprovision if we have a
			// ReconciledGeneration set as that implies a previous success.
			instance.Generation = 2
			instance.Status.ReconciledGeneration = 1
			if tc.includeUserInfo {
				instance.Spec.UserInfo = testUserInfo
			}

			fakeCatalogClient.AddReactor("get", "instances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
				return true, instance, nil
			})

			err := testController.reconcileServiceInstance(instance)
			if err != nil {
				t.Fatalf("This should not fail")
			}

			brokerActions := fakeBrokerClient.Actions()
			assertNumberOfServiceBrokerActions(t, brokerActions, 1)
			actualRequest, ok := brokerActions[0].Request.(*osb.DeprovisionRequest)
			if !ok {
				t.Errorf("%v: unexpected request type; expected %T, got %T", tc.name, &osb.DeprovisionRequest{}, actualRequest)
				return
			}
			var expectedOriginatingIdentity *osb.AlphaOriginatingIdentity
			if tc.expectedOriginatingIdentity {
				expectedOriginatingIdentity = testOriginatingIdentity
			}
			assertOriginatingIdentity(t, expectedOriginatingIdentity, actualRequest.OriginatingIdentity)
		}()
	}
}

func TestPollInstanceUsingOriginatingIdentity(t *testing.T) {
	for _, tc := range originatingIdentityTestCases {
		func() {
			if tc.enableOriginatingIdentity {
				utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=true", scfeatures.OriginatingIdentity))
				defer utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=false", scfeatures.OriginatingIdentity))
			}

			_, _, fakeBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
				PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
					Response: &osb.LastOperationResponse{
						State:       osb.StateInProgress,
						Description: strPtr(lastOperationDescription),
					},
				},
			})

			sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
			sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
			sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

			instance := getTestServiceInstanceAsyncProvisioning(testOperation)
			if tc.includeUserInfo {
				instance.Spec.UserInfo = testUserInfo
			}

			err := testController.pollServiceInstanceInternal(instance)
			if err != nil {
				t.Fatalf("Expected pollServiceInstanceInternal to not fail while in progress")
			}

			brokerActions := fakeBrokerClient.Actions()
			assertNumberOfServiceBrokerActions(t, brokerActions, 1)
			actualRequest, ok := brokerActions[0].Request.(*osb.LastOperationRequest)
			if !ok {
				t.Errorf("%v: unexpected request type; expected %T, got %T", tc.name, &osb.LastOperationRequest{}, actualRequest)
				return
			}
			var expectedOriginatingIdentity *osb.AlphaOriginatingIdentity
			if tc.expectedOriginatingIdentity {
				expectedOriginatingIdentity = testOriginatingIdentity
			}
			assertOriginatingIdentity(t, expectedOriginatingIdentity, actualRequest.OriginatingIdentity)
		}()
	}
}

func TestReconcileServiceInstanceWithHTTPStatusCodeErrorOrphanMitigation(t *testing.T) {
	cases := []struct {
		name                     string
		statusCode               int
		triggersOrphanMitigation bool
	}{
		{
			name:                     "Status OK",
			statusCode:               200,
			triggersOrphanMitigation: false,
		},
		{
			name:                     "other 2XX",
			statusCode:               201,
			triggersOrphanMitigation: true,
		},
		{
			name:                     "3XX",
			statusCode:               300,
			triggersOrphanMitigation: false,
		},
		{
			name:                     "408",
			statusCode:               408,
			triggersOrphanMitigation: true,
		},
		{
			name:                     "other 4XX",
			statusCode:               400,
			triggersOrphanMitigation: false,
		},
		{
			name:                     "5XX",
			statusCode:               500,
			triggersOrphanMitigation: true,
		},
	}

	for _, tc := range cases {
		_, fakeCatalogClient, _, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
			ProvisionReaction: &fakeosb.ProvisionReaction{
				Error: osb.HTTPStatusCodeError{
					StatusCode: tc.statusCode,
				},
			},
		})

		sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
		sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
		sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

		instance := getTestServiceInstanceWithRefs()

		err := testController.reconcileServiceInstance(instance)

		// The action should be:
		// 0. Updating the status
		actions := fakeCatalogClient.Actions()
		if ok := expectNumberOfActions(t, tc.name, actions, 2); !ok {
			continue
		}

		updatedObject, ok := expectUpdateStatus(t, tc.name, actions[1], instance)
		if !ok {
			continue
		}
		updatedServiceInstance, _ := updatedObject.(*v1alpha1.ServiceInstance)

		if ok := testServiceInstanceOrphanMitigationInProgress(t, tc.name, errorf, updatedServiceInstance, tc.triggersOrphanMitigation); !ok {
			continue
		}

		if tc.triggersOrphanMitigation && err == nil {
			t.Errorf("%v: Reconciler should return error so that instance is orphan mitigated", tc.name)
			continue
		}

		if !tc.triggersOrphanMitigation && err != nil {
			t.Errorf("%v: Reconciler should treat as terminal condition and not requeue", tc.name)
			continue
		}
	}
}

func TestReconcileServiceInstanceTimeoutTriggersOrphanMitigation(t *testing.T) {
	_, fakeCatalogClient, _, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Error: &url.Error{
				Err: getTestTimeoutError(),
			},
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceWithRefs()

	if err := testController.reconcileServiceInstance(instance); err == nil {
		t.Fatal("Reconciler should return error for timeout so that instance is orphan mitigated")
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedObject := assertUpdateStatus(t, actions[1], instance)
	updatedServiceInstance, ok := updatedObject.(*v1alpha1.ServiceInstance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstance", updatedObject)
	}

	assertServiceInstanceReadyFalse(t, updatedServiceInstance)
	assertServiceInstanceOrphanMitigationInProgressTrue(t, updatedServiceInstance)
}

func TestReconcileServiceInstanceOrphanMitigation(t *testing.T) {
	key := osb.OperationKey(testOperation)
	description := "description"
	// invalidState := "invalid state"

	cases := []struct {
		name                     string
		deprovReaction           *fakeosb.DeprovisionReaction
		pollReaction             *fakeosb.PollLastOperationReaction
		async                    bool
		finishedOrphanMitigation bool
		shouldError              bool
		retryDurationExceeded    bool
	}{
		// Synchronous
		{
			name: "sync - success",
			deprovReaction: &fakeosb.DeprovisionReaction{
				Response: &osb.DeprovisionResponse{},
			},
			finishedOrphanMitigation: true,
		},
		{
			name: "sync - 202 accepted",
			deprovReaction: &fakeosb.DeprovisionReaction{
				Response: &osb.DeprovisionResponse{
					Async:        true,
					OperationKey: &key,
				},
			},
			finishedOrphanMitigation: false,
		},
		{
			name: "sync - http error",
			deprovReaction: &fakeosb.DeprovisionReaction{
				Error: fakeosb.AsyncRequiredError(),
			},
			finishedOrphanMitigation: true,
		},
		{
			name: "sync - other error",
			deprovReaction: &fakeosb.DeprovisionReaction{
				Error: fmt.Errorf("other error"),
			},
			finishedOrphanMitigation: false,
			shouldError:              true,
		},
		{
			name: "sync - other error - retry duration exceeded",
			deprovReaction: &fakeosb.DeprovisionReaction{
				Error: fmt.Errorf("other error"),
			},
			finishedOrphanMitigation: true,
			retryDurationExceeded:    true,
		},
		// Asynchronous (Polling)
		{
			name: "poll - success",
			pollReaction: &fakeosb.PollLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State: osb.StateSucceeded,
				},
			},
			async: true,
			finishedOrphanMitigation: true,
		},
		{
			name: "poll - gone",
			pollReaction: &fakeosb.PollLastOperationReaction{
				Error: osb.HTTPStatusCodeError{
					StatusCode: http.StatusGone,
				},
			},
			async: true,
			finishedOrphanMitigation: true,
		},
		{
			name: "poll - in progress",
			pollReaction: &fakeosb.PollLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State:       osb.StateInProgress,
					Description: &description,
				},
			},
			async: true,
			finishedOrphanMitigation: false,
		},
		{
			name: "poll - failed",
			pollReaction: &fakeosb.PollLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State: osb.StateFailed,
				},
			},
			async: true,
			finishedOrphanMitigation: true,
		},
		// TODO (mkibbe): poll - error
		// TODO (mkibbe): invalid state
		{
			name: "poll - error - retry duration exceeded",
			pollReaction: &fakeosb.PollLastOperationReaction{
				Error: fmt.Errorf("other error"),
			},
			async: true,
			finishedOrphanMitigation: true,
			retryDurationExceeded:    true,
		},
		{
			name: "poll - in progress - retry duration exceeded",
			pollReaction: &fakeosb.PollLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State: osb.StateInProgress,
				},
			},
			async: true,
			finishedOrphanMitigation: true,
			retryDurationExceeded:    true,
		},
		{
			name: "poll - invalid state - retry duration exceeded",
			pollReaction: &fakeosb.PollLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State: "invalid state",
				},
			},
			async: true,
			finishedOrphanMitigation: true,
			retryDurationExceeded:    true,
		},
	}

	for _, tc := range cases {
		_, fakeCatalogClient, _, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
			DeprovisionReaction:       tc.deprovReaction,
			PollLastOperationReaction: tc.pollReaction,
		})

		sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
		sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
		sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

		instance := getTestServiceInstanceWithRefs()
		instance.ObjectMeta.Finalizers = []string{v1alpha1.FinalizerServiceCatalog}
		instance.Status.CurrentOperation = v1alpha1.ServiceInstanceOperationProvision
		instance.Status.OrphanMitigationInProgress = true

		if tc.async {
			instance.Status.AsyncOpInProgress = true
		}

		var startTime metav1.Time
		if tc.retryDurationExceeded {
			startTime = metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
		} else {
			startTime = metav1.NewTime(time.Now())
		}
		instance.Status.OperationStartTime = &startTime

		fakeCatalogClient.AddReactor("get", "serviceinstances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
			return true, instance, nil
		})

		err := testController.reconcileServiceInstance(instance)

		// The action should be:
		// 0. Updating the status
		actions := fakeCatalogClient.Actions()
		if ok := expectNumberOfActions(t, tc.name, actions, 1); !ok {
			continue
		}

		updatedObject, ok := expectUpdateStatus(t, tc.name, actions[0], instance)
		if !ok {
			continue
		}
		updatedServiceInstance, _ := updatedObject.(*v1alpha1.ServiceInstance)

		if ok := testServiceInstanceOrphanMitigationInProgress(t, tc.name, errorf, updatedServiceInstance, !tc.finishedOrphanMitigation); !ok {
			continue
		}

		// validate reconciliation error response
		if tc.shouldError {
			if err == nil {
				t.Errorf("%v: Expected error; this should not be a terminal state", tc.name)
				continue
			}
		} else {
			if err != nil {
				t.Errorf("%v: Unexpected error; this should be a terminal state", tc.name)
				continue
			}
		}

		expectCatalogFinalizerExists(t, tc.name, updatedServiceInstance)
	}
}

// TestReconcileServiceInstanceWithSecretParameters tests reconciling an instance
// that has parameters obtained from secrets.
func TestReconcileServiceInstanceWithSecretParameters(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Response: &osb.ProvisionResponse{},
		},
	})

	paramSecret := &apiv1.Secret{
		Data: map[string][]byte{
			"param-secret-key": []byte("{\"b\":\"2\"}"),
		},
	}
	addGetSecretReaction(fakeKubeClient, paramSecret)

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServicePlans().Informer().GetStore().Add(getTestServicePlan())

	instance := getTestServiceInstanceWithRefs()

	parameters := map[string]interface{}{
		"a": "1",
	}
	b, err := json.Marshal(parameters)
	if err != nil {
		t.Fatalf("Failed to marshal parameters %v : %v", parameters, err)
	}
	instance.Spec.Parameters = &runtime.RawExtension{Raw: b}

	instance.Spec.ParametersFrom = []v1alpha1.ParametersFromSource{
		{
			SecretKeyRef: &v1alpha1.SecretKeyReference{
				Name: "param-secret-name",
				Key:  "param-secret-key",
			},
		},
	}

	if err = testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        instanceGUID,
		ServiceID:         serviceClassGUID,
		PlanID:            planGUID,
		Context: map[string]interface{}{
			"platform":  "kubernetes",
			"namespace": "test-ns",
		},
		Parameters: map[string]interface{}{
			"a": "1",
			"b": "2",
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

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationInProgressWithParameters(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, testServicePlanName, expectedParameters, expectedParametersChecksum, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceOperationSuccessWithParameters(t, updatedServiceInstance, v1alpha1.ServiceInstanceOperationProvision, testServicePlanName, expectedParameters, expectedParametersChecksum, instance)

	updateObject, ok := updatedServiceInstance.(*v1alpha1.ServiceInstance)
	if !ok {
		t.Fatalf("couldn't convert to *v1alpha1.ServiceInstance")
	}

	// Verify parameters are what we'd expect them to be, basically name, map with two values in it.
	if len(updateObject.Spec.Parameters.Raw) == 0 {
		t.Fatalf("Parameters was unexpectedly empty")
	}

	// verify no kube resources created
	// First action is getting the namespace uid
	// Second action is getting the parameter secret
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "namespaces", checkType: checkGetActionType},
		{verb: "get", resourceName: "secrets", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := apiv1.EventTypeNormal + " " + successProvisionReason + " " + "The instance was provisioned successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

// TestResolveReferencesReferencesAlreadySet tests that resolveReferences does
// nothing if references have already been set.
func TestResolveReferencesReferencesAlreadySet(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, _, testController, _ := newTestController(t, noFakeActions())
	instance := getTestServiceInstanceWithRefs()
	updatedInstance, err := testController.resolveReferences(instance)
	if err != nil {
		t.Fatalf("resolveReferences failed unexpectedly: %q", err)
	}
	if e, a := instance, updatedInstance; !reflect.DeepEqual(instance, updatedInstance) {
		t.Fatalf("Instance was modified, expected\n%v\nGot\n%v", e, a)
	}

	// No kube actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	// There should be no actions for catalog
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 0)
}

// TestResolveReferencesNoServiceClass tests that resolveReferences fails
// with the expected failure case when no ServiceClass exists
func TestResolveReferencesNoServiceClass(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, _, testController, _ := newTestController(t, noFakeActions())

	instance := getTestServiceInstance()

	updatedInstance, err := testController.resolveReferences(instance)
	if err == nil {
		t.Fatalf("Should have failed with no service class")
	}

	if e, a := "a non-existent ServiceClass", err.Error(); !strings.Contains(a, e) {
		t.Fatalf("Did not get the expected error message %q got %q", e, a)
	}
	if updatedInstance != nil {
		t.Fatalf("updatedInstance retuend was non-nil: %+v", updatedInstance)
	}

	// We should get the following actions:
	// list call for ServiceClass
	// update service instance condition for failure
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.OneTermEqualSelector("spec.externalName", instance.Spec.ExternalServiceClassName),
	}
	assertList(t, actions[0], &v1alpha1.ServiceClass{}, listRestrictions)

	updatedServiceInstance := assertUpdateStatus(t, actions[1], instance)

	updatedObject, ok := updatedServiceInstance.(*v1alpha1.ServiceInstance)
	if !ok {
		t.Fatalf("couldn't convert to *v1alpha1.ServiceInstance")
	}
	if updatedObject.Spec.ServiceClassRef != nil {
		t.Fatalf("ServiceClassRef was unexpectedly set: %+v", updatedObject)
	}
	if updatedObject.Spec.ServicePlanRef != nil {
		t.Fatalf("ServicePlanRef was unexpectedly set: %+v", updatedObject)
	}

	// verify no kube resources created
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := apiv1.EventTypeWarning + " " + errorNonexistentServiceClassReason + " " + "ServiceInstance" + " \"test-ns/test-instance\" references a non-existent ServiceClass \"test-serviceclass\""
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v\nExpected: %v", a, e)
	}
}

// TestResolveReferencesNoServicePlan tests that resolveReferences fails
// with the expected failure case when no ServicePlan exists
func TestResolveReferencesNoServicePlan(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, _, testController, _ := newTestController(t, noFakeActions())

	instance := getTestServiceInstance()

	sc := getTestServiceClass()
	var scItems []v1alpha1.ServiceClass
	scItems = append(scItems, *sc)
	fakeCatalogClient.AddReactor("list", "serviceclasses", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1alpha1.ServiceClassList{Items: scItems}, nil
	})

	updatedInstance, err := testController.resolveReferences(instance)
	if err == nil {
		t.Fatalf("Should have failed with no service plan")
	}

	if e, a := "a non-existent ServicePlan", err.Error(); !strings.Contains(a, e) {
		t.Fatalf("Did not get the expected error message %q got %q", e, a)
	}

	if updatedInstance != nil {
		t.Fatalf("updatedInstance retuend was non-nil: %+v", updatedInstance)
	}

	// We should get the following actions:
	// list call for ServiceClass
	// list call for ServicePlan
	// update service instance condition for failure
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 3)

	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.OneTermEqualSelector("spec.externalName", instance.Spec.ExternalServiceClassName),
	}
	assertList(t, actions[0], &v1alpha1.ServiceClass{}, listRestrictions)

	listRestrictions = clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.ParseSelectorOrDie("spec.externalName=test-plan,spec.serviceBrokerName=test-broker,spec.serviceClassRef.name=SCGUID"),
	}
	assertList(t, actions[1], &v1alpha1.ServicePlan{}, listRestrictions)

	updatedServiceInstance := assertUpdateStatus(t, actions[2], instance)

	updatedObject, ok := updatedServiceInstance.(*v1alpha1.ServiceInstance)
	if !ok {
		t.Fatalf("couldn't convert to *v1alpha1.ServiceInstance")
	}
	if updatedObject.Spec.ServiceClassRef == nil || updatedObject.Spec.ServiceClassRef.Name != serviceClassGUID {
		t.Fatalf("ServiceClassRef.Name was not set correctly, expected %q got: %+v", serviceClassGUID, updatedObject.Spec.ServiceClassRef.Name)
	}
	if updatedObject.Spec.ServicePlanRef != nil {
		t.Fatalf("ServicePlanRef was unexpectedly set: %+v", updatedObject)
	}

	// verify no kube resources created
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := apiv1.EventTypeWarning + " " + errorNonexistentServicePlanReason + " " + "ServiceInstance" + " \"test-ns/test-instance\" references a non-existent ServicePlan \"test-plan\" on ServiceClass \"test-serviceclass\""
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v\nExpected: %v", a, e)
	}
}

// TestResolveReferences tests that resolveReferences works
// correctly and resolves references.
func TestResolveReferencesWorks(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, _, testController, _ := newTestController(t, noFakeActions())

	instance := getTestServiceInstance()

	sc := getTestServiceClass()
	var scItems []v1alpha1.ServiceClass
	scItems = append(scItems, *sc)
	fakeCatalogClient.AddReactor("list", "serviceclasses", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1alpha1.ServiceClassList{Items: scItems}, nil
	})
	sp := getTestServicePlan()
	var spItems []v1alpha1.ServicePlan
	spItems = append(spItems, *sp)
	fakeCatalogClient.AddReactor("list", "serviceplans", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1alpha1.ServicePlanList{Items: spItems}, nil
	})

	updatedInstance, err := testController.resolveReferences(instance)
	if err != nil {
		t.Fatalf("Should not have failed, but failed with: %q", err)
	}

	if updatedInstance.Spec.ServiceClassRef == nil || updatedInstance.Spec.ServiceClassRef.Name != serviceClassGUID {
		t.Fatalf("Did not find expected ServiceClassRef, expected %q got %+v", serviceClassGUID, updatedInstance.Spec.ServiceClassRef)
	}

	if updatedInstance.Spec.ServicePlanRef == nil || updatedInstance.Spec.ServicePlanRef.Name != planGUID {
		t.Fatalf("Did not find expected ServicePlanRef, expected %q got %+v", planGUID, updatedInstance.Spec.ServicePlanRef.Name)
	}

	// We should get the following actions:
	// list call for ServiceClass
	// list call for ServicePlan
	// updating references
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 3)

	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.OneTermEqualSelector("spec.externalName", instance.Spec.ExternalServiceClassName),
	}
	assertList(t, actions[0], &v1alpha1.ServiceClass{}, listRestrictions)

	listRestrictions = clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.ParseSelectorOrDie("spec.externalName=test-plan,spec.serviceBrokerName=test-broker,spec.serviceClassRef.name=SCGUID"),
	}
	assertList(t, actions[1], &v1alpha1.ServicePlan{}, listRestrictions)

	updatedServiceInstance := assertUpdateReference(t, actions[2], instance)
	updateObject, ok := updatedServiceInstance.(*v1alpha1.ServiceInstance)
	if !ok {
		t.Fatalf("couldn't convert to *v1alpha1.ServiceInstance")
	}
	if updateObject.Spec.ServiceClassRef == nil || updateObject.Spec.ServiceClassRef.Name != serviceClassGUID {
		t.Fatalf("ServiceClassRef was not resolved correctly during reconcile")
	}
	if updateObject.Spec.ServicePlanRef == nil || updateObject.Spec.ServicePlanRef.Name != planGUID {
		t.Fatalf("ServicePlanRef was not resolved correctly during reconcile")
	}

	// verify no kube resources created
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 0)
}

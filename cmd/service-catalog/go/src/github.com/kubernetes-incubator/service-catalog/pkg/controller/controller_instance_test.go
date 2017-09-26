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

	osb "github.com/pmorie/go-open-service-broker-client/v2"
	fakeosb "github.com/pmorie/go-open-service-broker-client/v2/fake"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	scfeatures "github.com/kubernetes-incubator/service-catalog/pkg/features"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
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
			ServiceClassName: "nothere",
			PlanName:         "nothere",
			ExternalID:       instanceGUID,
		},
	}

	if err := testController.reconcileServiceInstance(instance); err == nil {
		t.Fatal("nothere is a service class that cannot be referenced by the service instance as it does not exist.")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says it failed because no such class exists.
	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceReadyFalse(t, updatedServiceInstance, errorNonexistentServiceClassReason)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorNonexistentServiceClassReason + " " + "ServiceInstance \"/test-instance\" references a non-existent ServiceClass \"nothere\""
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

// TestReconcileServiceInstanceNonExistentServiceBroker tests reconciling an instance whose
// broker does not exist.  This returns an error.
func TestReconcileServiceInstanceNonExistentServiceBroker(t *testing.T) {
	_, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	instance := getTestServiceInstance()

	if err := testController.reconcileServiceInstance(instance); err == nil {
		t.Fatal("The broker referenced by the instance exists when it should not.")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says it failed because no such broker exists.
	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceReadyFalse(t, updatedServiceInstance, errorNonexistentServiceBrokerReason)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorNonexistentServiceBrokerReason + " " + "ServiceInstance \"test-ns/test-instance\" references a non-existent broker \"test-broker\""
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

// TestReconcileServiceInstanceWithAuthError tests reconcileInstance when Kube Client
// fails to locate the broker authentication secret.
func TestReconcileServiceInstanceWithAuthError(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	broker := getTestServiceBroker()
	broker.Spec.AuthInfo = &v1alpha1.ServiceBrokerAuthInfo{
		Basic: &v1alpha1.BasicAuthConfig{
			SecretRef: &v1.ObjectReference{
				Namespace: "does_not_exist",
				Name:      "auth-name",
			},
		},
	}
	sharedInformers.ServiceBrokers().Informer().GetStore().Add(broker)
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	instance := getTestServiceInstance()

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
	if err := checkCatalogClientActions(actions, []catalogClientAction{
		{
			verb: "update",
			checkObject: checkServiceInstance(instanceDescription{
				name:             testServiceInstanceName,
				conditionReasons: []string{"ErrorGettingAuthCredentials"},
			}),
			getRuntimeObject: getRuntimeObjectFromUpdateAction,
		},
	}); err != nil {
		t.Fatal(err)
	}

	// verify one kube action occurred
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "secrets", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	// verify that one event was emitted
	events := getRecordedEvents(testController)
	expectedEvent := api.EventTypeWarning + " " + errorAuthCredentialsReason + " " + "Error getting broker auth credentials for broker \"test-broker\": no secret defined"
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

	instance := &v1alpha1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceInstanceName,
			Generation: 1,
		},
		Spec: v1alpha1.ServiceInstanceSpec{
			ServiceClassName: testServiceClassName,
			PlanName:         "nothere",
			ExternalID:       instanceGUID,
		},
	}

	if err := testController.reconcileServiceInstance(instance); err == nil {
		t.Fatal("The service plan nothere should not exist to be referenced.")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 0)

	// ensure that the only action made on the catalog client was to set the condition on the
	// instance to indicate that the service plan doesn't exist
	actions := fakeCatalogClient.Actions()
	if err := checkCatalogClientActions(actions, []catalogClientAction{
		{
			verb:             "update",
			getRuntimeObject: getRuntimeObjectFromUpdateAction,
			checkObject: checkServiceInstance(instanceDescription{
				name:             testServiceInstanceName,
				conditionReasons: []string{errorNonexistentServicePlanReason},
			}),
		},
	}); err != nil {
		t.Fatal(err)
	}

	// check to make sure the only event sent indicated that the instance references a non-existent
	// service plan
	events := getRecordedEvents(testController)
	expectedEvent := api.EventTypeWarning + " " + errorNonexistentServicePlanReason + " " + "ServiceInstance \"/test-instance\" references a non-existent ServicePlan \"nothere\" on ServiceClass \"test-serviceclass\""
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

	instance := getTestServiceInstance()

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

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceReadyTrue(t, updatedServiceInstance)

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

	expectedEvent := api.EventTypeNormal + " " + successProvisionReason + " " + "The instance was provisioned successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

// TestReconcileServiceInstanceWithInvalidParameters tests that reconcileInstance
// fails with an error when the service parameters are invalid
func TestReconcileServiceInstanceWithInvalidParameters(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	instance := getTestServiceInstance()
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

	// verify no kube resources created
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceReadyFalse(t, updatedServiceInstance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorWithParameters + " " + "Failed to prepare ServiceInstance parameters"
	if e, a := expectedEvent, events[0]; !strings.Contains(a, e) { // event contains RawExtension, so just compare error message
		t.Fatalf("Received unexpected event: %v", a)
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

	instance := getTestServiceInstance()

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
	assertNumberOfActions(t, actions, 1)

	updatedObject := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceReadyFalse(t, updatedObject)
	// The ready condition should be the only condition set.
	updatedServiceInstance, ok := updatedObject.(*v1alpha1.ServiceInstance)
	if !ok {
		t.Fatalf("couldn't convert to *v1alpha1.ServiceInstance")
	}
	if l := len(updatedServiceInstance.Status.Conditions); l != 1 {
		t.Fatalf("Expected 1 condition, got %v", l)
	}

	if updatedServiceInstance.Status.Conditions[0].Type != v1alpha1.ServiceInstanceConditionReady && updatedServiceInstance.Status.Conditions[0].Status != v1alpha1.ConditionFalse {
		t.Fatalf("Expected failed condition to be set")
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorErrorCallingProvisionReason + " " + "Error provisioning ServiceInstance \"test-ns/test-instance\" of ServiceClass \"test-serviceclass\" at ServiceBroker \"test-broker\": fake creation failure"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
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

	instance := getTestServiceInstance()

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
	assertNumberOfActions(t, actions, 1)
	updatedObject := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceReadyFalse(t, updatedObject)
	updatedServiceInstance, ok := updatedObject.(*v1alpha1.ServiceInstance)
	if !ok {
		t.Fatalf("couldn't convert to *v1alpha1.ServiceInstance")
	}
	if l := len(updatedServiceInstance.Status.Conditions); l != 2 {
		t.Fatalf("Expected 2 conditions, got %v", l)
	}

	if updatedServiceInstance.Status.Conditions[0].Type != v1alpha1.ServiceInstanceConditionFailed && updatedServiceInstance.Status.Conditions[0].Status != v1alpha1.ConditionTrue {
		t.Fatalf("Expected failed condition to be set")
	}

	// already verified 2nd condition Ready=false above

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorProvisionCallFailedReason + " " + "Error provisioning ServiceInstance \"test-ns/test-instance\" of ServiceClass \"test-serviceclass\" at ServiceBroker \"test-broker\": Status: 409; ErrorMessage: OutOfQuota; Description: You're out of quota!; ResponseError: <nil>"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
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

	instance := getTestServiceInstance()

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
	assertNumberOfActions(t, actions, 1)

	// verify no kube resources created.
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "namespaces", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceReadyTrue(t, updatedServiceInstance)
	assertServiceInstanceDashboardURL(t, updatedServiceInstance, testDashboardURL)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeNormal + " " + successProvisionReason + " " + successProvisionMessage
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
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

	instance := getTestServiceInstance()
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
	assertNumberOfActions(t, actions, 1)
	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceReadyFalse(t, updatedServiceInstance)
	assertAsyncOpInProgressTrue(t, updatedServiceInstance)
	assertServiceInstanceLastOperation(t, updatedServiceInstance, testOperation)
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

	instance := getTestServiceInstance()
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
	assertNumberOfActions(t, actions, 1)
	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceReadyFalse(t, updatedServiceInstance)
	assertAsyncOpInProgressTrue(t, updatedServiceInstance)
	assertServiceInstanceLastOperation(t, updatedServiceInstance, "")

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
		return true, &v1.Namespace{}, errors.New("No namespace")
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	instance := getTestServiceInstance()

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
	assertUpdateStatus(t, actions[0], instance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorFindingNamespaceServiceInstanceReason + " " + "Failed to get namespace \"test-ns\" during instance create: No namespace"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
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

	instance := getTestServiceInstance()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{v1alpha1.FinalizerServiceCatalog}
	// we only invoke the broker client to deprovision if we have a reconciled generation set
	// as that implies a previous success.
	instance.Status.ReconciledGeneration = 1

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
	// The three actions should be:
	// 0. Updating the ready condition
	// 1. Get against the instance
	// 2. Removing the finalizer
	assertNumberOfActions(t, actions, 3)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceReadyFalse(t, updatedServiceInstance)

	assertGet(t, actions[1], instance)
	updatedServiceInstance = assertUpdateStatus(t, actions[2], instance)
	assertEmptyFinalizers(t, updatedServiceInstance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeNormal + " " + successDeprovisionReason + " " + "The instance was deprovisioned successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
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
	credentials := getTestServiceInstanceCredential()
	sharedInformers.ServiceInstanceCredentials().Informer().GetStore().Add(credentials)

	instance := getTestServiceInstance()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{v1alpha1.FinalizerServiceCatalog}
	// we only invoke the broker client to deprovision if we have a reconciled generation set
	// as that implies a previous success.
	instance.Status.ReconciledGeneration = 1

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

	// The action should be Update to Ready = false
	assertNumberOfActions(t, actions, 1)
	expectedMessage := "Delete instance failed. Delete instance test-ns/test-instance blocked by existing ServiceInstanceCredentials associated with this instance.  All credentials must be removed first."
	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceReadyFalse(t, updatedServiceInstance, "DeprovisionCallFailed")
	i, ok := updatedServiceInstance.(*v1alpha1.ServiceInstance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstance", updatedServiceInstance)
	}

	condition := i.Status.Conditions[0]
	if condition.Message != expectedMessage {
		fatalf(t, "unexpected message; expected %v, got %v", expectedMessage, condition.Message)
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + "DeprovisionCallFailed Delete instance test-ns/test-instance blocked by existing ServiceInstanceCredentials associated with this instance.  All credentials must be removed first."
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}

	// delete credentials
	sharedInformers.ServiceInstanceCredentials().Informer().GetStore().Delete(credentials)

	fakeKubeClient.ClearActions()
	fakeCatalogClient.ClearActions()

	// credentials were removed, verify the next reconcilation removes
	// the instance

	err = testController.reconcileServiceInstance(instance)
	if err != nil {
		t.Fatalf("This should not fail")
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

	// The three actions should be:
	// 0. Updating the ready condition
	// 1. Get against the instance
	// 2. Removing the finalizer
	assertNumberOfActions(t, actions, 3)

	updatedServiceInstance = assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceReadyFalse(t, updatedServiceInstance)
	assertGet(t, actions[1], instance)
	updatedServiceInstance = assertUpdateStatus(t, actions[2], instance)
	assertEmptyFinalizers(t, updatedServiceInstance)

	events = getRecordedEvents(testController)
	assertNumEvents(t, events, 1)
	expectedEvent = api.EventTypeNormal + " " + successDeprovisionReason + " " + "The instance was deprovisioned successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
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

	instance := getTestServiceInstance()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{v1alpha1.FinalizerServiceCatalog}
	// we only invoke the broker client to deprovision if we have a reconciled generation set
	// as that implies a previous success.
	instance.Status.ReconciledGeneration = 1

	fakeCatalogClient.AddReactor("get", "serviceinstances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, instance, nil
	})

	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.pollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.reconcileServiceInstance(instance)
	if err != nil {
		t.Fatalf("This should not fail")
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
	// The action should be:
	// 0. Updating the ready condition
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceReadyFalse(t, updatedServiceInstance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeNormal + " " + asyncDeprovisioningReason + " " + "The instance is being deprovisioned asynchronously"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

// TestReconcileServiceInstanceDeleteFailedInstance tests that a failed instance will
// be finalized, but no deprovision request will be sent to the broker.
func TestReconcileServiceInstanceDeleteFailedInstance(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())

	instance := getTestServiceInstanceWithFailedStatus()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{v1alpha1.FinalizerServiceCatalog}

	// we only invoke the broker client to deprovision if we have a reconciled generation set
	// as that implies a previous success.
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
	// The two actions should be:
	// 0. Get against the instance
	// 1. Removing the finalizer
	assertNumberOfActions(t, actions, 2)
	assertGet(t, actions[0], instance)
	updatedServiceInstance := assertUpdateStatus(t, actions[1], instance)
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

	instance := getTestServiceInstance()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{v1alpha1.FinalizerServiceCatalog}

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
	// The three actions should be:
	// 0. Get against the instance
	// 1. Removing the finalizer
	assertNumberOfActions(t, actions, 2)

	assertGet(t, actions[0], instance)
	updatedServiceInstance := assertUpdateStatus(t, actions[1], instance)
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
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID: instanceGUID,
		ServiceID:  strPtr(serviceClassGUID),
		PlanID:     strPtr(planGUID),
	})

	// there should have been 1 action to update the status with the last operation description
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)
	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceReadyFalse(t, updatedServiceInstance)
	assertAsyncOpInProgressTrue(t, updatedServiceInstance)

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
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID: instanceGUID,
		ServiceID:  strPtr(serviceClassGUID),
		PlanID:     strPtr(planGUID),
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	// ServiceInstance should be ready and there no longer is an async operation
	// in place.
	assertServiceInstanceReadyTrue(t, updatedServiceInstance)
	assertAsyncOpInProgressFalse(t, updatedServiceInstance)
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
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID: instanceGUID,
		ServiceID:  strPtr(serviceClassGUID),
		PlanID:     strPtr(planGUID),
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	// ServiceInstance should be not ready and there no longer is an async operation
	// in place.
	assertServiceInstanceReadyFalse(t, updatedServiceInstance)
	assertServiceInstanceCondition(t, updatedServiceInstance, v1alpha1.ServiceInstanceConditionFailed, v1alpha1.ConditionTrue)
	assertAsyncOpInProgressFalse(t, updatedServiceInstance)
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
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID: instanceGUID,
		ServiceID:  strPtr(serviceClassGUID),
		PlanID:     strPtr(planGUID),
	})

	// there should have been 1 action to update the instance status with the last operation
	// description
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)
	action := actions[0]
	updatedServiceInstance := assertUpdateStatus(t, action, instance)
	assertServiceInstanceReadyFalse(t, updatedServiceInstance, asyncDeprovisioningReason)

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
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID: instanceGUID,
		ServiceID:  strPtr(serviceClassGUID),
		PlanID:     strPtr(planGUID),
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	// ServiceInstance should have been deprovisioned
	assertServiceInstanceReadyCondition(t, updatedServiceInstance, v1alpha1.ConditionFalse, successDeprovisionReason)
	assertAsyncOpInProgressFalse(t, updatedServiceInstance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)
	expectedEvent := api.EventTypeNormal + " " + successDeprovisionReason + " " + "The instance was deprovisioned successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
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
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID: instanceGUID,
		ServiceID:  strPtr(serviceClassGUID),
		PlanID:     strPtr(planGUID),
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	// ServiceInstance should be set to unknown since the operation on the broker
	// failed.
	assertServiceInstanceReadyCondition(t, updatedServiceInstance, v1alpha1.ConditionUnknown, errorDeprovisionCalledReason)
	assertServiceInstanceCondition(t, updatedServiceInstance, v1alpha1.ServiceInstanceConditionFailed, v1alpha1.ConditionTrue)
	assertAsyncOpInProgressFalse(t, updatedServiceInstance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)
	expectedEvent := api.EventTypeWarning + " " + errorDeprovisionCalledReason + " " + "Error deprovisioning ServiceInstance \"test-ns/test-instance\" of ServiceClass \"test-serviceclass\" at ServiceBroker \"test-broker\": \"\""
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
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
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID: instanceGUID,
		ServiceID:  strPtr(serviceClassGUID),
		PlanID:     strPtr(planGUID),
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	// ServiceInstance should have been deprovisioned
	assertServiceInstanceReadyCondition(t, updatedServiceInstance, v1alpha1.ConditionFalse, successDeprovisionReason)
	assertAsyncOpInProgressFalse(t, updatedServiceInstance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)
	expectedEvent := api.EventTypeNormal + " " + successDeprovisionReason + " " + "The instance was deprovisioned successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
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
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID: instanceGUID,
		ServiceID:  strPtr(serviceClassGUID),
		PlanID:     strPtr(planGUID),
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 0)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)
	expectedEvent := api.EventTypeWarning + " " + errorPollingLastOperationReason + " " + "Error polling last operation for instance test-ns/test-instance: Status code: 403; ErrorMessage: %!q(*string=<nil>); description: %!q(*string=<nil>)"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
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
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID: instanceGUID,
		ServiceID:  strPtr(serviceClassGUID),
		PlanID:     strPtr(planGUID),
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	// The three actions should be:
	// 0. Updating the ready condition
	// 1. Get against the instance (updateFinalizers calls)
	// 2. Removing the finalizer
	assertNumberOfActions(t, actions, 3)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceReadyCondition(t, updatedServiceInstance, v1alpha1.ConditionFalse, successDeprovisionReason)

	// ServiceInstance should have been deprovisioned
	assertGet(t, actions[1], instance)
	updatedServiceInstance = assertUpdateStatus(t, actions[2], instance)
	assertEmptyFinalizers(t, updatedServiceInstance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)
	expectedEvent := api.EventTypeNormal + " " + successDeprovisionReason + " " + "The instance was deprovisioned successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
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

	instance := getTestServiceInstance()
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
	assertServiceInstanceReadyTrue(t, updatedServiceInstance)
	assertServiceInstanceOperationStartTimeSet(t, updatedServiceInstance, false)

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

	expectedEvent := api.EventTypeNormal + " " + successProvisionReason + " " + "The instance was provisioned successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
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

	instance := getTestServiceInstance()
	startTime := metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
	instance.Status.OperationStartTime = &startTime

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("Should have return no error because the retry duration has elapsed: %v", err)
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

	updatedObject := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceReadyFalse(t, updatedObject)
	assertServiceInstanceCondition(t, updatedObject, v1alpha1.ServiceInstanceConditionFailed, v1alpha1.ConditionTrue, errorReconciliationRetryTimeoutReason)
	assertServiceInstanceOperationStartTimeSet(t, updatedObject, false)

	expectedEventPrefixes := []string{
		api.EventTypeWarning + " " + errorErrorCallingProvisionReason,
		api.EventTypeWarning + " " + errorReconciliationRetryTimeoutReason,
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
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID: instanceGUID,
		ServiceID:  strPtr(serviceClassGUID),
		PlanID:     strPtr(planGUID),
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	// ServiceInstance should be ready and there no longer is an async operation
	// in place.
	assertServiceInstanceReadyTrue(t, updatedServiceInstance)
	assertAsyncOpInProgressFalse(t, updatedServiceInstance)
	assertServiceInstanceOperationStartTimeSet(t, updatedServiceInstance, false)
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
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID: instanceGUID,
		ServiceID:  strPtr(serviceClassGUID),
		PlanID:     strPtr(planGUID),
	})

	// there should have been 2 actions:
	// (1) update the status with the last operation description
	// (2) update the status with the Failed condition
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceReadyFalse(t, updatedServiceInstance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceCondition(t, updatedServiceInstance, v1alpha1.ServiceInstanceConditionFailed, v1alpha1.ConditionTrue, errorReconciliationRetryTimeoutReason)
	assertAsyncOpInProgressFalse(t, updatedServiceInstance)
	assertServiceInstanceOperationStartTimeSet(t, updatedServiceInstance, false)

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)
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

			instance := getTestServiceInstance()
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

			instance := getTestServiceInstance()
			instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
			instance.ObjectMeta.Finalizers = []string{v1alpha1.FinalizerServiceCatalog}
			// we only invoke the broker client to deprovision if we have a
			// ReconciledGeneration set as that implies a previous success.
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

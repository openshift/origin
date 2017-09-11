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
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	scmeta "github.com/kubernetes-incubator/service-catalog/pkg/api/meta"
	checksum "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/checksum/versioned/v1alpha1"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	fakeosb "github.com/pmorie/go-open-service-broker-client/v2/fake"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"

	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	clientgotesting "k8s.io/client-go/testing"
)

// TestReconcileBindingNonExistingInstance tests reconcileBinding to ensure a
// binding fails as expected when an instance to bind to doesn't exist.
func TestReconcileServiceInstanceCredentialNonExistingServiceInstance(t *testing.T) {
	_, fakeCatalogClient, fakeServiceBrokerClient, testController, _ := newTestController(t, noFakeActions())

	binding := &v1alpha1.ServiceInstanceCredential{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceCredentialName},
		Spec: v1alpha1.ServiceInstanceCredentialSpec{
			ServiceInstanceRef: v1.LocalObjectReference{Name: "nothere"},
			ExternalID:         bindingGUID,
		},
	}

	err := testController.reconcileServiceInstanceCredential(binding)
	if err == nil {
		t.Fatal("binding nothere was found and it should not be found")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says it failed because no such instance exists.
	updateAction := actions[0].(clientgotesting.UpdateAction)
	if e, a := "update", updateAction.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on actions[0]; expected %v, got %v", e, a)
	}
	updatedServiceInstanceCredential := assertUpdateStatus(t, actions[0], binding)
	assertServiceInstanceCredentialReadyFalse(t, updatedServiceInstanceCredential, errorNonexistentServiceInstanceReason)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorNonexistentServiceInstanceReason + " " + "ServiceInstanceCredential \"/test-binding\" references a non-existent ServiceInstance \"/nothere\""
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

// TestReconcileBindingNonExistingServiceClass tests reconcileBinding to ensure a
// binding fails as expected when a serviceclass does not exist.
func TestReconcileServiceInstanceCredentialNonExistingServiceClass(t *testing.T) {
	_, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	instance := &v1alpha1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceName, Namespace: testNamespace},
		Spec: v1alpha1.ServiceInstanceSpec{
			ServiceClassName: "nothere",
			PlanName:         testPlanName,
			ExternalID:       instanceGUID,
		},
	}
	sharedInformers.ServiceInstances().Informer().GetStore().Add(instance)

	binding := &v1alpha1.ServiceInstanceCredential{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceCredentialName, Namespace: testNamespace},
		Spec: v1alpha1.ServiceInstanceCredentialSpec{
			ServiceInstanceRef: v1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         bindingGUID,
		},
	}

	err := testController.reconcileServiceInstanceCredential(binding)
	if err == nil {
		t.Fatal("serviceclass nothere was found and it should not be found")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says it failed because no such service class.
	updatedServiceInstanceCredential := assertUpdateStatus(t, actions[0], binding)
	assertServiceInstanceCredentialReadyFalse(t, updatedServiceInstanceCredential, errorNonexistentServiceClassMessage)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorNonexistentServiceClassMessage + " " + "ServiceInstanceCredential \"test-ns/test-binding\" references a non-existent ServiceClass \"nothere\""
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

// TestReconcileBindingWithSecretConflict tests reconcileBinding to ensure a
// binding with an existing secret not owned by the bindings fails as expected.
func TestReconcileServiceInstanceCredentialWithSecretConflict(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
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
	addGetSecretReaction(fakeKubeClient, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceCredentialName, Namespace: testNamespace},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1alpha1.ConditionTrue))

	binding := &v1alpha1.ServiceInstanceCredential{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceCredentialName, Namespace: testNamespace},
		Spec: v1alpha1.ServiceInstanceCredentialSpec{
			ServiceInstanceRef: v1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         bindingGUID,
			SecretName:         testServiceInstanceCredentialSecretName,
		},
	}

	err := testController.reconcileServiceInstanceCredential(binding)
	if err == nil {
		t.Fatalf("a binding should fail to create a secret: %v", err)
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  bindingGUID,
		InstanceID: instanceGUID,
		ServiceID:  serviceClassGUID,
		PlanID:     planGUID,
		AppGUID:    strPtr(testNsUID),
		BindResource: &osb.BindResource{
			AppGUID: strPtr(testNsUID),
		},
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)
	updatedServiceInstanceCredential := assertUpdateStatus(t, actions[0], binding).(*v1alpha1.ServiceInstanceCredential)
	assertServiceInstanceCredentialReadyFalse(t, updatedServiceInstanceCredential)

	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 2)

	// first action is a get on the namespace
	// second action is a get on the secret
	action := kubeActions[1].(clientgotesting.GetAction)
	if e, a := "get", action.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on action; expected %v, got %v", e, a)
	}
	if e, a := "secrets", action.GetResource().Resource; e != a {
		t.Fatalf("Unexpected resource on action; expected %v, got %v", e, a)
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorInjectingBindResultReason
	if e, a := expectedEvent, events[0]; !strings.HasPrefix(a, e) {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

// TestReconcileBindingWithParameters tests reconcileBinding to ensure a
// binding with parameters will be passed to the broker properly.
func TestReconcileServiceInstanceCredentialWithParameters(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
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

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1alpha1.ConditionTrue))

	binding := &v1alpha1.ServiceInstanceCredential{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceCredentialName, Namespace: testNamespace},
		Spec: v1alpha1.ServiceInstanceCredentialSpec{
			ServiceInstanceRef: v1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         bindingGUID,
			SecretName:         testServiceInstanceCredentialSecretName,
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

	err = testController.reconcileServiceInstanceCredential(binding)
	if err != nil {
		t.Fatalf("a valid binding should not fail: %v", err)
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  bindingGUID,
		InstanceID: instanceGUID,
		ServiceID:  serviceClassGUID,
		PlanID:     planGUID,
		AppGUID:    strPtr(testNsUID),
		Parameters: map[string]interface{}{
			"args": []interface{}{
				"first-arg",
				"second-arg",
			},
			"name": "test-param",
		},
		BindResource: &osb.BindResource{
			AppGUID: strPtr(testNsUID),
		},
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)
	updatedServiceInstanceCredential := assertUpdateStatus(t, actions[0], binding).(*v1alpha1.ServiceInstanceCredential)
	assertServiceInstanceCredentialReadyTrue(t, updatedServiceInstanceCredential)

	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 3)

	// first action is a get on the namespace
	// second action is a get on the secret
	action := kubeActions[2].(clientgotesting.CreateAction)
	if e, a := "create", action.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on action; expected %v, got %v", e, a)
	}
	if e, a := "secrets", action.GetResource().Resource; e != a {
		t.Fatalf("Unexpected resource on action; expected %v, got %v", e, a)
	}
	actionSecret, ok := action.GetObject().(*v1.Secret)
	if !ok {
		t.Fatal("couldn't convert secret into a v1.Secret")
	}
	controllerRef := GetControllerOf(actionSecret)
	if controllerRef == nil || controllerRef.UID != updatedServiceInstanceCredential.UID {
		t.Fatalf("Secret is not owned by the ServiceInstanceCredential: %v", controllerRef)
	}
	if !IsControlledBy(actionSecret, updatedServiceInstanceCredential) {
		t.Fatal("Secret is not owned by the ServiceInstanceCredential")
	}
	if e, a := testServiceInstanceCredentialSecretName, actionSecret.Name; e != a {
		t.Fatalf("Unexpected name of secret; expected %v, got %v", e, a)
	}
	value, ok := actionSecret.Data["a"]
	if !ok {
		t.Fatal("Didn't find secret key 'a' in created secret")
	}
	if e, a := "b", string(value); e != a {
		t.Fatalf("Unexpected value of key 'a' in created secret; expected %v got %v", e, a)
	}
	value, ok = actionSecret.Data["c"]
	if !ok {
		t.Fatal("Didn't find secret key 'a' in created secret")
	}
	if e, a := "d", string(value); e != a {
		t.Fatalf("Unexpected value of key 'c' in created secret; expected %v got %v", e, a)
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeNormal + " " + successInjectedBindResultReason + " " + successInjectedBindResultMessage
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

// TestReconcileBindingNonbindableServiceClass tests reconcileBinding to ensure a
// binding for an instance that references a non-bindable service class and a
// non-bindable plan fails as expected.
func TestReconcileServiceInstanceCredentialNonbindableServiceClass(t *testing.T) {
	_, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestNonbindableServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestNonbindableServiceInstance())

	binding := &v1alpha1.ServiceInstanceCredential{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceCredentialName, Namespace: testNamespace},
		Spec: v1alpha1.ServiceInstanceCredentialSpec{
			ServiceInstanceRef: v1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         bindingGUID,
		},
	}

	err := testController.reconcileServiceInstanceCredential(binding)
	if err != nil {
		t.Fatalf("binding should fail against a non-bindable ServiceClass")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says binding was created
	updatedServiceInstanceCredential := assertUpdateStatus(t, actions[0], binding)
	assertServiceInstanceCredentialReadyFalse(t, updatedServiceInstanceCredential, errorNonbindableServiceClassReason)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorNonbindableServiceClassReason + ` ServiceInstanceCredential "test-ns/test-binding" references a non-bindable ServiceClass ("test-unbindable-serviceclass") and Plan ("test-unbindable-plan") combination`
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

// TestReconcileBindingNonbindableServiceClassBindablePlan tests reconcileBinding
// to ensure a binding for an instance that references a non-bindable service
// class and a bindable plan fails as expected.
func TestReconcileServiceInstanceCredentialNonbindableServiceClassBindablePlan(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
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

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestNonbindableServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(func() *v1alpha1.ServiceInstance {
		i := getTestServiceInstanceNonbindableServiceBindablePlan()
		i.Status = v1alpha1.ServiceInstanceStatus{
			Conditions: []v1alpha1.ServiceInstanceCondition{
				{
					Type:   v1alpha1.ServiceInstanceConditionReady,
					Status: v1alpha1.ConditionTrue,
				},
			},
		}
		return i
	}())

	binding := &v1alpha1.ServiceInstanceCredential{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceCredentialName, Namespace: testNamespace},
		Spec: v1alpha1.ServiceInstanceCredentialSpec{
			ServiceInstanceRef: v1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         bindingGUID,
			SecretName:         testServiceInstanceCredentialSecretName,
		},
	}

	err := testController.reconcileServiceInstanceCredential(binding)
	if err != nil {
		t.Fatalf("A bindable plan overrides the bindability of a service class: %v", err)
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  bindingGUID,
		InstanceID: instanceGUID,
		ServiceID:  nonbindableServiceClassGUID,
		PlanID:     planGUID,
		AppGUID:    strPtr(testNsUID),
		BindResource: &osb.BindResource{
			AppGUID: strPtr(testNsUID),
		},
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)
	updatedServiceInstanceCredential := assertUpdateStatus(t, actions[0], binding)
	assertServiceInstanceCredentialReadyTrue(t, updatedServiceInstanceCredential)

	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 3)

	// first action is a get on the namespace
	// second action is a get on the secret
	action := kubeActions[2].(clientgotesting.CreateAction)
	if e, a := "create", action.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on action; expected %v, got %v", e, a)
	}
	if e, a := "secrets", action.GetResource().Resource; e != a {
		t.Fatalf("Unexpected resource on action; expected %v, got %v", e, a)
	}
	actionSecret, ok := action.GetObject().(*v1.Secret)
	if !ok {
		t.Fatal("couldn't convert secret into a v1.Secret")
	}
	if e, a := testServiceInstanceCredentialSecretName, actionSecret.Name; e != a {
		t.Fatalf("Unexpected name of secret; expected %v, got %v", e, a)
	}
	value, ok := actionSecret.Data["a"]
	if !ok {
		t.Fatal("Didn't find secret key 'a' in created secret")
	}
	if e, a := "b", string(value); e != a {
		t.Fatalf("Unexpected value of key 'a' in created secret; expected %v got %v", e, a)
	}
	value, ok = actionSecret.Data["c"]
	if !ok {
		t.Fatal("Didn't find secret key 'a' in created secret")
	}
	if e, a := "d", string(value); e != a {
		t.Fatalf("Unexpected value of key 'c' in created secret; expected %v got %v", e, a)
	}

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)
}

// TestReconcileBindingBindableServiceClassNonbindablePlan tests reconcileBinding
// to ensure a binding for an instance that references a bindable service class
// and a non-bindable plan fails as expected.
func TestReconcileServiceInstanceCredentialBindableServiceClassNonbindablePlan(t *testing.T) {
	_, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceBindableServiceNonbindablePlan())

	binding := &v1alpha1.ServiceInstanceCredential{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceCredentialName, Namespace: testNamespace},
		Spec: v1alpha1.ServiceInstanceCredentialSpec{
			ServiceInstanceRef: v1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         bindingGUID,
		},
	}

	err := testController.reconcileServiceInstanceCredential(binding)
	if err != nil {
		t.Fatalf("binding against a nonbindable plan should fail")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says binding was created
	updatedServiceInstanceCredential := assertUpdateStatus(t, actions[0], binding)
	assertServiceInstanceCredentialReadyFalse(t, updatedServiceInstanceCredential, errorNonbindableServiceClassReason)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorNonbindableServiceClassReason + ` ServiceInstanceCredential "test-ns/test-binding" references a non-bindable ServiceClass ("test-serviceclass") and Plan ("test-unbindable-plan") combination`
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

// TestReconcileBindingFailsWithInstanceAsyncOngoing tests reconcileBinding
// to ensure a binding that references an instance that has the
// AsyncOpInProgreset flag set to true fails as expected.
func TestReconcileServiceInstanceCredentialFailsWithServiceInstanceAsyncOngoing(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceAsyncProvisioning(""))

	binding := &v1alpha1.ServiceInstanceCredential{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceCredentialName, Namespace: testNamespace},
		Spec: v1alpha1.ServiceInstanceCredentialSpec{
			ServiceInstanceRef: v1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         bindingGUID,
		},
	}

	err := testController.reconcileServiceInstanceCredential(binding)
	if err == nil {
		t.Fatalf("reconcileServiceInstanceCredential did not fail with async operation ongoing")
	}

	if !strings.Contains(err.Error(), "Ongoing Asynchronous") {
		t.Fatalf("Did not get the expected error %q : got %q", "Ongoing Asynchronous", err)
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 0)

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says binding was created
	updatedServiceInstanceCredential := assertUpdateStatus(t, actions[0], binding)
	assertServiceInstanceCredentialReadyFalse(t, updatedServiceInstanceCredential)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	if !strings.Contains(events[0], "has ongoing asynchronous operation") {
		t.Fatalf("Did not find expected error %q : got %q", "has ongoing asynchronous operation", events[0])
	}
	if !strings.Contains(events[0], testNamespace+"/"+testServiceInstanceName) {
		t.Fatalf("Did not find expected instance name : got %q", events[0])
	}
	if !strings.Contains(events[0], testNamespace+"/"+testServiceInstanceCredentialName) {
		t.Fatalf("Did not find expected binding name : got %q", events[0])
	}
}

// TestReconcileBindingInstanceNotReady tests reconcileBinding to ensure a
// binding for an instance with a ready condition set to false fails as expected.
func TestReconcileServiceInstanceCredentialServiceInstanceNotReady(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	addGetNamespaceReaction(fakeKubeClient)

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstance())

	binding := &v1alpha1.ServiceInstanceCredential{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceCredentialName, Namespace: testNamespace},
		Spec: v1alpha1.ServiceInstanceCredentialSpec{
			ServiceInstanceRef: v1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         bindingGUID,
		},
	}

	err := testController.reconcileServiceInstanceCredential(binding)
	if err != nil {
		t.Fatalf("a binding cannot be created against an instance that is not prepared")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says binding was created
	updatedServiceInstanceCredential := assertUpdateStatus(t, actions[0], binding)
	assertServiceInstanceCredentialReadyFalse(t, updatedServiceInstanceCredential)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorServiceInstanceNotReadyReason + " " + `ServiceInstanceCredential cannot begin because referenced instance "test-ns/test-instance" is not ready`
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

// TestReconcileBindingNamespaceError tests reconcileBinding to ensure a binding
// with an invalid namespace fails as expected.
func TestReconcileServiceInstanceCredentialNamespaceError(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	fakeKubeClient.AddReactor("get", "namespaces", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1.Namespace{}, errors.New("No namespace")
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstance())

	binding := &v1alpha1.ServiceInstanceCredential{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceCredentialName, Namespace: testNamespace},
		Spec: v1alpha1.ServiceInstanceCredentialSpec{
			ServiceInstanceRef: v1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         bindingGUID,
		},
	}

	err := testController.reconcileServiceInstanceCredential(binding)
	if err == nil {
		t.Fatalf("ServiceInstanceCredentials are namespaced. If we cannot get the namespace we cannot find the binding")
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)
	updatedServiceInstanceCredential := assertUpdateStatus(t, actions[0], binding)
	assertServiceInstanceCredentialReadyFalse(t, updatedServiceInstanceCredential)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorFindingNamespaceServiceInstanceReason + " " + "Failed to get namespace \"test-ns\" during binding: No namespace"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

// TestReconcileBindingDelete tests reconcileBinding to ensure a binding
// deletion works as expected.
func TestReconcileServiceInstanceCredentialDelete(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UnbindReaction: &fakeosb.UnbindReaction{},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstance())

	binding := &v1alpha1.ServiceInstanceCredential{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testServiceInstanceCredentialName,
			Namespace:         testNamespace,
			DeletionTimestamp: &metav1.Time{},
			Finalizers:        []string{v1alpha1.FinalizerServiceCatalog},
		},
		Spec: v1alpha1.ServiceInstanceCredentialSpec{
			ServiceInstanceRef: v1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         bindingGUID,
			SecretName:         testServiceInstanceCredentialSecretName,
		},
	}

	fakeCatalogClient.AddReactor("get", "serviceinstancecredentials", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, binding, nil
	})

	err := testController.reconcileServiceInstanceCredential(binding)
	if err != nil {
		t.Fatalf("%v", err)
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	assertUnbind(t, brokerActions[0], &osb.UnbindRequest{
		BindingID:  bindingGUID,
		InstanceID: instanceGUID,
		ServiceID:  serviceClassGUID,
		PlanID:     planGUID,
	})

	kubeActions := fakeKubeClient.Actions()
	// The action should be deleting the secret
	assertNumberOfActions(t, kubeActions, 1)

	deleteAction := kubeActions[0].(clientgotesting.DeleteActionImpl)
	if e, a := "delete", deleteAction.GetVerb(); e != a {
		t.Fatalf("Unexpected verb on kubeActions[1]; expected %v, got %v", e, a)
	}

	if e, a := binding.Spec.SecretName, deleteAction.Name; e != a {
		t.Fatalf("Unexpected name of secret: expected %v, got %v", e, a)
	}

	actions := fakeCatalogClient.Actions()
	// The three actions should be:
	// 0. Updating the ready condition
	// 1. Get against the binding in question
	// 2. Removing the finalizer
	assertNumberOfActions(t, actions, 3)

	updatedServiceInstanceCredential := assertUpdateStatus(t, actions[0], binding)
	assertServiceInstanceCredentialReadyFalse(t, updatedServiceInstanceCredential)

	assertGet(t, actions[1], binding)

	updatedServiceInstanceCredential = assertUpdateStatus(t, actions[2], binding)
	assertEmptyFinalizers(t, updatedServiceInstanceCredential)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeNormal + " " + successUnboundReason + " " + "This binding was deleted successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

// TestSetServiceInstanceCredentialCondition verifies setting a condition on a binding yields
// the results as expected with respect to the changed condition and transition
// time.
func TestSetServiceInstanceCredentialCondition(t *testing.T) {
	bindingWithCondition := func(condition *v1alpha1.ServiceInstanceCredentialCondition) *v1alpha1.ServiceInstanceCredential {
		binding := getTestServiceInstanceCredential()
		binding.Status = v1alpha1.ServiceInstanceCredentialStatus{
			Conditions: []v1alpha1.ServiceInstanceCredentialCondition{*condition},
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
	condition := func(cType v1alpha1.ServiceInstanceCredentialConditionType, status v1alpha1.ConditionStatus, s ...string) *v1alpha1.ServiceInstanceCredentialCondition {
		c := &v1alpha1.ServiceInstanceCredentialCondition{
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

	readyFalse := func() *v1alpha1.ServiceInstanceCredentialCondition {
		return condition(v1alpha1.ServiceInstanceCredentialConditionReady, v1alpha1.ConditionFalse, "Reason", "Message")
	}

	readyFalsef := func(reason, message string) *v1alpha1.ServiceInstanceCredentialCondition {
		return condition(v1alpha1.ServiceInstanceCredentialConditionReady, v1alpha1.ConditionFalse, reason, message)
	}

	readyTrue := func() *v1alpha1.ServiceInstanceCredentialCondition {
		return condition(v1alpha1.ServiceInstanceCredentialConditionReady, v1alpha1.ConditionTrue, "Reason", "Message")
	}

	failedTrue := func() *v1alpha1.ServiceInstanceCredentialCondition {
		return condition(v1alpha1.ServiceInstanceCredentialConditionFailed, v1alpha1.ConditionTrue, "Reason", "Message")
	}

	// withNewTs sets the LastTransitionTime to the 'new' basis time and
	// returns it.
	withNewTs := func(c *v1alpha1.ServiceInstanceCredentialCondition) *v1alpha1.ServiceInstanceCredentialCondition {
		c.LastTransitionTime = newTs
		return c
	}

	// this test works by calling setServiceInstanceCredentialCondition with the input and
	// condition fields of the test case, and ensuring that afterward the
	// input (which is mutated by the setServiceInstanceCredentialCondition call) is deep-equal
	// to the test case result.
	//
	// take note of where withNewTs is used when declaring the result to
	// indicate that the LastTransitionTime field on a condition should have
	// changed.
	cases := []struct {
		name      string
		input     *v1alpha1.ServiceInstanceCredential
		condition *v1alpha1.ServiceInstanceCredentialCondition
		result    *v1alpha1.ServiceInstanceCredential
	}{
		{
			name:      "new ready condition",
			input:     getTestServiceInstanceCredential(),
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
			result: func() *v1alpha1.ServiceInstanceCredential {
				i := bindingWithCondition(readyFalse())
				i.Status.Conditions = append(i.Status.Conditions, *withNewTs(failedTrue()))
				return i
			}(),
		},
	}

	for _, tc := range cases {
		setServiceInstanceCredentialConditionInternal(tc.input, tc.condition.Type, tc.condition.Status, tc.condition.Reason, tc.condition.Message, newTs)

		if !reflect.DeepEqual(tc.input, tc.result) {
			t.Errorf("%v: unexpected diff: %v", tc.name, diff.ObjectReflectDiff(tc.input, tc.result))
		}
	}
}

// TestReconcileServiceInstanceCredentialDeleteFailedServiceInstanceCredential tests reconcileServiceInstanceCredential to ensure
// a binding with a failed status is deleted properly.
func TestReconcileServiceInstanceCredentialDeleteFailedServiceInstanceCredential(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UnbindReaction: &fakeosb.UnbindReaction{},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstance())

	binding := getTestServiceInstanceCredentialWithFailedStatus()
	binding.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	binding.ObjectMeta.Finalizers = []string{v1alpha1.FinalizerServiceCatalog}

	checksum := checksum.ServiceInstanceCredentialSpecChecksum(binding.Spec)
	binding.Status.Checksum = &checksum

	fakeCatalogClient.AddReactor("get", "serviceinstancecredentials", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, binding, nil
	})

	err := testController.reconcileServiceInstanceCredential(binding)
	if err != nil {
		t.Fatalf("%v", err)
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	assertUnbind(t, brokerActions[0], &osb.UnbindRequest{
		BindingID:  bindingGUID,
		InstanceID: instanceGUID,
		ServiceID:  serviceClassGUID,
		PlanID:     planGUID,
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
		t.Fatalf("Unexpected name of secret: expected %v, got %v", e, a)
	}

	actions := fakeCatalogClient.Actions()
	// The three actions should be:
	// 0. Updating the ready condition
	// 1. Get against the binding in question
	// 2. Removing the finalizer
	assertNumberOfActions(t, actions, 3)

	updatedServiceInstanceCredential := assertUpdateStatus(t, actions[0], binding)
	assertServiceInstanceCredentialReadyFalse(t, updatedServiceInstanceCredential)

	assertGet(t, actions[1], binding)

	updatedServiceInstanceCredential = assertUpdateStatus(t, actions[2], binding)
	assertEmptyFinalizers(t, updatedServiceInstanceCredential)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeNormal + " " + successUnboundReason + " " + "This binding was deleted successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

// TestReconcileBindingWithBrokerError tests reconcileBinding to ensure a
// binding request response that contains a broker error fails as expected.
func TestReconcileServiceInstanceCredentialWithServiceBrokerError(t *testing.T) {
	_, _, _, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
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

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1alpha1.ConditionTrue))

	binding := &v1alpha1.ServiceInstanceCredential{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceCredentialName, Namespace: testNamespace},
		Spec: v1alpha1.ServiceInstanceCredentialSpec{
			ServiceInstanceRef: v1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         bindingGUID,
			SecretName:         testServiceInstanceCredentialSecretName,
		},
	}

	err := testController.reconcileServiceInstanceCredential(binding)
	if err == nil {
		t.Fatal("reconcileServiceInstanceCredential should have returned an error")
	}

	events := getRecordedEvents(testController)
	expectedEvent := api.EventTypeWarning + " " + errorBindCallReason + " " + `Error creating ServiceInstanceCredential "test-binding/test-ns" for ServiceInstance "test-ns/test-instance" of ServiceClass "test-serviceclass" at ServiceBroker "test-broker": Unexpected action`
	if 1 != len(events) {
		t.Fatalf("Did not record expected event, expecting: %v", expectedEvent)
	}
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v, expecting: %v", a, e)
	}
}

// TestReconcileBindingWithBrokerHTTPError tests reconcileBindings to ensure a
// binding request response that contains a broker HTTP error fails as expected.
func TestReconcileServiceInstanceCredentialWithServiceBrokerHTTPError(t *testing.T) {
	_, _, _, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
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

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1alpha1.ConditionTrue))

	binding := &v1alpha1.ServiceInstanceCredential{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceInstanceCredentialName, Namespace: testNamespace},
		Spec: v1alpha1.ServiceInstanceCredentialSpec{
			ServiceInstanceRef: v1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         bindingGUID,
			SecretName:         testServiceInstanceCredentialSecretName,
		},
	}

	err := testController.reconcileServiceInstanceCredential(binding)
	if err == nil {
		t.Fatal("reconcileServiceInstanceCredential should have returned an error")
	}

	events := getRecordedEvents(testController)
	expectedEvent := api.EventTypeWarning + " " + errorBindCallReason + " " + `Error creating ServiceInstanceCredential "test-binding/test-ns" for ServiceInstance "test-ns/test-instance" of ServiceClass "test-serviceclass" at ServiceBroker "test-broker", Status: 422; ErrorMessage: AsyncRequired; Description: This service plan requires client support for asynchronous service operations.; ResponseError: <nil>`
	if 1 != len(events) {
		t.Fatalf("Did not record expected event, expecting: %v", expectedEvent)
	}
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: '%v', expecting: '%v'", a, e)
	}
}

// TestReconcileServiceInstanceCredentialWithFailureCondition tests reconcileServiceInstanceCredential to ensure
// no processing is done on a binding containing a failed status.
func TestReconcileServiceInstanceCredentialWithFailureCondition(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1alpha1.ConditionTrue))

	binding := getTestServiceInstanceCredentialWithFailedStatus()

	if err := testController.reconcileServiceInstanceCredential(binding); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 0)

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 0)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 0)
}

// TestReconcileServiceInstanceCredentialWithServiceInstanceCredentialCallFailure tests reconcileServiceInstanceCredential to ensure
// a bind creation failure is handled properly.
func TestReconcileServiceInstanceCredentialWithServiceInstanceCredentialCallFailure(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Error: errors.New("fake creation failure"),
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1alpha1.ConditionTrue))

	binding := getTestServiceInstanceCredential()

	if err := testController.reconcileServiceInstanceCredential(binding); err == nil {
		t.Fatal("ServiceInstanceCredential creation should fail")
	}

	// verify one kube action occurred
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "namespaces", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  bindingGUID,
		InstanceID: instanceGUID,
		ServiceID:  serviceClassGUID,
		PlanID:     planGUID,
		AppGUID:    strPtr(""),
		BindResource: &osb.BindResource{
			AppGUID: strPtr(""),
		},
	})

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorBindCallReason + " " + "Error creating ServiceInstanceCredential \"test-binding/test-ns\" for ServiceInstance \"test-ns/test-instance\" of ServiceClass \"test-serviceclass\" at ServiceBroker \"test-broker\": fake creation failure"

	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
	}
}

// TestReconcileServiceInstanceCredentialWithServiceInstanceCredentialFailure tests reconcileServiceInstanceCredential to ensure
// a binding request that receives an error from the broker is handled properly.
func TestReconcileServiceInstanceCredentialWithServiceInstanceCredentialFailure(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		BindReaction: &fakeosb.BindReaction{
			Error: osb.HTTPStatusCodeError{
				StatusCode:   http.StatusConflict,
				ErrorMessage: strPtr("ServiceInstanceCredentialExists"),
				Description:  strPtr("Service binding with the same id, for the same service instance already exists."),
			},
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1alpha1.ConditionTrue))

	binding := getTestServiceInstanceCredential()

	if err := testController.reconcileServiceInstanceCredential(binding); err == nil {
		t.Fatal("ServiceInstanceCredential creation should fail")
	}

	// verify one kube action occurred
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "namespaces", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedObject := assertUpdateStatus(t, actions[0], binding)
	assertServiceInstanceCredentialReadyFalse(t, updatedObject)
	updatedServiceInstanceCredential, ok := updatedObject.(*v1alpha1.ServiceInstanceCredential)
	if !ok {
		t.Fatal("Couldn't convert to v1alpha1.ServiceInstanceCredential")
	}
	if num := len(updatedServiceInstanceCredential.Status.Conditions); num != 2 {
		t.Fatalf("Expected two conditions, got %v", num)
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfServiceBrokerActions(t, brokerActions, 1)
	assertBind(t, brokerActions[0], &osb.BindRequest{
		BindingID:  bindingGUID,
		InstanceID: instanceGUID,
		ServiceID:  serviceClassGUID,
		PlanID:     planGUID,
		AppGUID:    strPtr(""),
		BindResource: &osb.BindResource{
			AppGUID: strPtr(""),
		},
	})

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := api.EventTypeWarning + " " + errorBindCallReason + " " + "Error creating ServiceInstanceCredential \"test-binding/test-ns\" for ServiceInstance \"test-ns/test-instance\" of ServiceClass \"test-serviceclass\" at ServiceBroker \"test-broker\", Status: 409; ErrorMessage: ServiceInstanceCredentialExists; Description: Service binding with the same id, for the same service instance already exists.; ResponseError: <nil>"

	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v", a)
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
func TestUpdateServiceInstanceCredentialCondition(t *testing.T) {
	getTestServiceInstanceCredentialWithStatus := func(status v1alpha1.ConditionStatus) *v1alpha1.ServiceInstanceCredential {
		instance := getTestServiceInstanceCredential()
		instance.Status = v1alpha1.ServiceInstanceCredentialStatus{
			Conditions: []v1alpha1.ServiceInstanceCredentialCondition{{
				Type:               v1alpha1.ServiceInstanceCredentialConditionReady,
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
		input                 *v1alpha1.ServiceInstanceCredential
		status                v1alpha1.ConditionStatus
		reason                string
		message               string
		transitionTimeChanged bool
	}{

		{
			name:                  "initially unset",
			input:                 getTestServiceInstanceCredential(),
			status:                v1alpha1.ConditionFalse,
			transitionTimeChanged: true,
		},
		{
			name:                  "not ready -> not ready",
			input:                 getTestServiceInstanceCredentialWithStatus(v1alpha1.ConditionFalse),
			status:                v1alpha1.ConditionFalse,
			transitionTimeChanged: false,
		},
		{
			name:                  "not ready -> not ready, message and reason change",
			input:                 getTestServiceInstanceCredentialWithStatus(v1alpha1.ConditionFalse),
			status:                v1alpha1.ConditionFalse,
			reason:                "foo",
			message:               "bar",
			transitionTimeChanged: false,
		},
		{
			name:                  "not ready -> ready",
			input:                 getTestServiceInstanceCredentialWithStatus(v1alpha1.ConditionFalse),
			status:                v1alpha1.ConditionTrue,
			transitionTimeChanged: true,
		},
		{
			name:                  "ready -> ready",
			input:                 getTestServiceInstanceCredentialWithStatus(v1alpha1.ConditionTrue),
			status:                v1alpha1.ConditionTrue,
			transitionTimeChanged: false,
		},
		{
			name:                  "ready -> not ready",
			input:                 getTestServiceInstanceCredentialWithStatus(v1alpha1.ConditionTrue),
			status:                v1alpha1.ConditionFalse,
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
		inputClone := clone.(*v1alpha1.ServiceInstanceCredential)

		err = testController.updateServiceInstanceCredentialCondition(tc.input, v1alpha1.ServiceInstanceCredentialConditionReady, tc.status, tc.reason, tc.message)
		if err != nil {
			t.Errorf("%v: error updating broker condition: %v", tc.name, err)
			continue
		}

		if !reflect.DeepEqual(tc.input, inputClone) {
			t.Errorf("%v: updating broker condition mutated input: expected %v, got %v", tc.name, inputClone, tc.input)
			continue
		}

		actions := fakeCatalogClient.Actions()
		if ok := expectNumberOfActions(t, tc.name, actions, 1); !ok {
			continue
		}

		updatedServiceInstanceCredential, ok := expectUpdateStatus(t, tc.name, actions[0], tc.input)
		if !ok {
			continue
		}

		updateActionObject, ok := updatedServiceInstanceCredential.(*v1alpha1.ServiceInstanceCredential)
		if !ok {
			t.Errorf("%v: couldn't convert to binding", tc.name)
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

// TestReconcileUnbindingWithBrokerError tests reconcileBinding to ensure an
// unbinding request response that contains a broker error fails as expected.
func TestReconcileUnbindingWithServiceBrokerError(t *testing.T) {
	_, _, _, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UnbindReaction: &fakeosb.UnbindReaction{
			Response: &osb.UnbindResponse{},
			Error:    fakeosb.UnexpectedActionError(),
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1alpha1.ConditionTrue))

	t1 := metav1.NewTime(time.Now())
	binding := &v1alpha1.ServiceInstanceCredential{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testServiceInstanceCredentialName,
			Namespace:         testNamespace,
			DeletionTimestamp: &t1,
		},
		Spec: v1alpha1.ServiceInstanceCredentialSpec{
			ServiceInstanceRef: v1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         bindingGUID,
			SecretName:         testServiceInstanceCredentialSecretName,
		},
	}
	if err := scmeta.AddFinalizer(binding, v1alpha1.FinalizerServiceCatalog); err != nil {
		t.Fatalf("Finalizer error: %v", err)
	}
	if err := testController.reconcileServiceInstanceCredential(binding); err == nil {
		t.Fatal("reconcileServiceInstanceCredential should have returned an error")
	}

	events := getRecordedEvents(testController)
	expectedEvent := api.EventTypeWarning + " " + errorUnbindCallReason + " " + `Error unbinding ServiceInstanceCredential "test-binding/test-ns" for ServiceInstance "test-ns/test-instance" of ServiceClass "test-serviceclass" at ServiceBroker "test-broker": Unexpected action`
	if 1 != len(events) {
		t.Fatalf("Did not record expected event, expecting: %v", expectedEvent)
	}
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v, expecting: %v", a, e)
	}
}

// TestReconcileUnbindingWithServiceBrokerHTTPError tests reconcileBinding to ensure an
// unbinding request response that contains a broker HTTP error fails as
// expected.
func TestReconcileUnbindingWithServiceBrokerHTTPError(t *testing.T) {
	_, _, _, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UnbindReaction: &fakeosb.UnbindReaction{
			Response: &osb.UnbindResponse{},
			Error: osb.HTTPStatusCodeError{
				StatusCode: http.StatusGone,
			},
		},
	})

	sharedInformers.ServiceBrokers().Informer().GetStore().Add(getTestServiceBroker())
	sharedInformers.ServiceClasses().Informer().GetStore().Add(getTestServiceClass())
	sharedInformers.ServiceInstances().Informer().GetStore().Add(getTestServiceInstanceWithStatus(v1alpha1.ConditionTrue))

	t1 := metav1.NewTime(time.Now())
	binding := &v1alpha1.ServiceInstanceCredential{
		ObjectMeta: metav1.ObjectMeta{
			Name:              testServiceInstanceCredentialName,
			Namespace:         testNamespace,
			DeletionTimestamp: &t1,
		},
		Spec: v1alpha1.ServiceInstanceCredentialSpec{
			ServiceInstanceRef: v1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         bindingGUID,
			SecretName:         testServiceInstanceCredentialSecretName,
		},
	}
	if err := scmeta.AddFinalizer(binding, v1alpha1.FinalizerServiceCatalog); err != nil {
		t.Fatalf("Finalizer error: %v", err)
	}
	if err := testController.reconcileServiceInstanceCredential(binding); err == nil {
		t.Fatal("reconcileServiceInstanceCredential should have returned an error")
	}

	events := getRecordedEvents(testController)
	expectedEvent := api.EventTypeWarning + " " + errorUnbindCallReason + " " + `Error creating Unbinding "test-binding/test-ns" for ServiceInstance "test-ns/test-instance" of ServiceClass "test-serviceclass" at ServiceBroker "test-broker", Status: 410; ErrorMessage: <nil>; Description: <nil>; ResponseError: <nil>`
	if 1 != len(events) {
		t.Fatalf("Did not record expected event, expecting: %v", expectedEvent)
	}
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v, expecting: %v", a, e)
	}
}

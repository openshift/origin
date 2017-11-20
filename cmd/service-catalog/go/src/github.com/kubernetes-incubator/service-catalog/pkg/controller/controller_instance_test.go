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

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"github.com/kubernetes-incubator/service-catalog/pkg/api"
	scfeatures "github.com/kubernetes-incubator/service-catalog/pkg/features"
	corev1 "k8s.io/api/core/v1"
	clientgotesting "k8s.io/client-go/testing"
)

const (
	lastOperationDescription = "testdescr"
)

// TestReconcileServiceInstanceNonExistentClusterServiceClass tests that reconcileInstance gets a failure when
// the specified service class is not found
func TestReconcileServiceInstanceNonExistentClusterServiceClass(t *testing.T) {
	_, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, _ := newTestController(t, noFakeActions())

	instance := &v1beta1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceInstanceName,
			Generation: 1,
		},
		Spec: v1beta1.ServiceInstanceSpec{
			PlanReference: v1beta1.PlanReference{
				ClusterServiceClassExternalName: "nothere",
				ClusterServicePlanExternalName:  "nothere",
			},
			ExternalID: testServiceInstanceGUID,
		},
	}

	if err := testController.reconcileServiceInstance(instance); err == nil {
		t.Fatal("nothere is a service class that cannot be referenced by the service instance as it does not exist.")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.OneTermEqualSelector("spec.externalName", instance.Spec.ClusterServiceClassExternalName),
	}
	assertList(t, actions[0], &v1beta1.ClusterServiceClass{}, listRestrictions)

	// There should be an action that says it failed because no such class exists.
	updatedServiceInstance := assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceErrorBeforeRequest(t, updatedServiceInstance, errorNonexistentClusterServiceClassReason, instance)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorNonexistentClusterServiceClassReason).msgf(
		"References a non-existent ClusterServiceClass (ExternalName: %q) or there is more than one (found: %d)",
		"nothere", 0,
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceNonExistentClusterServiceClass tests that reconcileInstance gets a failure when
// the specified service class is not found
func TestReconcileServiceInstanceNonExistentClusterServiceClassWithK8SName(t *testing.T) {
	_, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, _ := newTestController(t, noFakeActions())

	instance := &v1beta1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceInstanceName,
			Generation: 1,
		},
		Spec: v1beta1.ServiceInstanceSpec{
			PlanReference: v1beta1.PlanReference{
				ClusterServiceClassName: "nothereclass",
				ClusterServicePlanName:  "nothereplan",
			},
			ExternalID: testServiceInstanceGUID,
		},
	}

	if err := testController.reconcileServiceInstance(instance); err == nil {
		t.Fatal("nothere is a service class that cannot be referenced by the service instance as it does not exist.")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)
	// There should be an action that says it failed because no such class exists.
	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceErrorBeforeRequest(t, updatedServiceInstance, errorNonexistentClusterServiceClassReason, instance)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorNonexistentClusterServiceClassReason).msgf(
		"References a non-existent ClusterServiceClass (K8S: %q)",
		"nothereclass",
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceNonExistentClusterServiceBroker tests reconciling an instance whose
// broker does not exist.  This returns an error.
func TestReconcileServiceInstanceNonExistentClusterServiceBroker(t *testing.T) {
	_, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithRefs()

	if err := testController.reconcileServiceInstance(instance); err == nil {
		t.Fatal("The broker referenced by the instance exists when it should not.")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says it failed because no such broker exists.
	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceErrorBeforeRequest(t, updatedServiceInstance, errorNonexistentClusterServiceBrokerReason, instance)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorNonexistentClusterServiceBrokerReason).msgf(
		"References a non-existent broker %q",
		"test-broker",
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceWithAuthError tests reconcileInstance when Kube Client
// fails to locate the broker authentication secret.
func TestReconcileServiceInstanceWithAuthError(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	broker := getTestClusterServiceBroker()
	broker.Spec.AuthInfo = &v1beta1.ServiceBrokerAuthInfo{
		Basic: &v1beta1.BasicAuthConfig{
			SecretRef: &v1beta1.ObjectReference{
				Namespace: "does_not_exist",
				Name:      "auth-name",
			},
		},
	}
	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(broker)
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithRefs()

	fakeKubeClient.AddReactor("get", "secrets", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("no secret defined")
	})

	if err := testController.reconcileServiceInstance(instance); err == nil {
		t.Fatal("There was no secret to be found, but does_not_exist/auth-name was found.")
	}

	// verify that no broker actions occurred
	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

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
	expectedEvent := warningEventBuilder(errorAuthCredentialsReason).msgf(
		"Error getting broker auth credentials for broker %q:",
		"test-broker",
	).msg("no secret defined")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceNonExistentClusterServicePlan tests that reconcileInstance
// fails when service class points at a non-existent service plan
func TestReconcileServiceInstanceNonExistentClusterServicePlan(t *testing.T) {
	_, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := &v1beta1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceInstanceName,
			Generation: 1,
		},
		Spec: v1beta1.ServiceInstanceSpec{
			PlanReference: v1beta1.PlanReference{
				ClusterServiceClassExternalName: testClusterServiceClassName,
				ClusterServicePlanExternalName:  "nothere",
			},
			ClusterServiceClassRef: &v1beta1.ClusterObjectReference{
				Name: testClusterServiceClassGUID,
			},
			ExternalID: testServiceInstanceGUID,
		},
	}

	if err := testController.reconcileServiceInstance(instance); err == nil {
		t.Fatal("The service plan nothere should not exist to be referenced.")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	// ensure that there are two actions, one to list plans and an action
	// to set the condition on the instance to indicate that the service
	// plan doesn't exist
	actions := fakeCatalogClient.Actions()

	assertNumberOfActions(t, actions, 2)
	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.ParseSelectorOrDie("spec.externalName=nothere,spec.clusterServiceBrokerName=test-broker,spec.clusterServiceClassRef.name=SCGUID"),
	}
	assertList(t, actions[0], &v1beta1.ClusterServicePlan{}, listRestrictions)

	updatedServiceInstance := assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceErrorBeforeRequest(t, updatedServiceInstance, errorNonexistentClusterServicePlanReason, instance)

	// check to make sure the only event sent indicated that the instance references a non-existent
	// service plan
	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorNonexistentClusterServicePlanReason).msgf(
		`References a non-existent ClusterServicePlan (K8S: %q ExternalName: %q) on ClusterServiceClass (K8S: %q ExternalName: %q) or there is more than one (found: %v)`,
		"", "nothere", "SCGUID", "test-serviceclass", 0,
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceNonExistentClusterServicePlanK8SName tests that reconcileInstance
// fails when service class points at a non-existent service plan and is specified using
// k8s name.
func TestReconcileServiceInstanceNonExistentClusterServicePlanK8SName(t *testing.T) {
	_, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := &v1beta1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceInstanceName,
			Generation: 1,
		},
		Spec: v1beta1.ServiceInstanceSpec{
			PlanReference: v1beta1.PlanReference{
				ClusterServiceClassName: testClusterServiceClassGUID,
				ClusterServicePlanName:  "nothereplan",
			},
			ClusterServiceClassRef: &v1beta1.ClusterObjectReference{
				Name: testClusterServiceClassGUID,
			},
			ExternalID: testServiceInstanceGUID,
		},
	}

	if err := testController.reconcileServiceInstance(instance); err == nil {
		t.Fatal("The service plan nothere should not exist to be referenced.")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	// ensure that there is only one action to to set the condition on the
	// instance to indicate that the service plan doesn't exist
	actions := fakeCatalogClient.Actions()

	assertNumberOfActions(t, actions, 1)
	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceErrorBeforeRequest(t, updatedServiceInstance, errorNonexistentClusterServicePlanReason, instance)

	// check to make sure the only event sent indicated that the instance references a non-existent
	// service plan
	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorNonexistentClusterServicePlanReason).msgf(
		"References a non-existent ClusterServicePlan with K8S name %q on ClusterServiceClass with K8S name %q",
		"nothereplan", testClusterServiceClassGUID,
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceWithParameters tests a simple successful reconciliation
func TestReconcileServiceInstanceWithParameters(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Response: &osb.ProvisionResponse{},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

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

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            testClusterServicePlanGUID,
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
	assertServiceInstanceOperationInProgressWithParameters(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, testClusterServicePlanName, testClusterServicePlanGUID, expectedParameters, expectedParametersChecksum, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceOperationSuccessWithParameters(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, testClusterServicePlanName, testClusterServicePlanGUID, expectedParameters, expectedParametersChecksum, instance)

	updateObject, ok := updatedServiceInstance.(*v1beta1.ServiceInstance)
	if !ok {
		t.Fatalf("couldn't convert to *v1beta1.ServiceInstance")
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

	expectedEvent := normalEventBuilder(successProvisionReason).msg("The instance was provisioned successfully")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceResolvesReferences tests a simple successful
// reconciliation and making sure that Service[Class|Plan]Ref are resolved
func TestReconcileServiceInstanceResolvesReferences(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Response: &osb.ProvisionResponse{},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sc := getTestClusterServiceClass()
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(sc)
	sp := getTestClusterServicePlan()
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(sp)

	instance := getTestServiceInstance()

	var scItems []v1beta1.ClusterServiceClass
	scItems = append(scItems, *sc)
	fakeCatalogClient.AddReactor("list", "clusterserviceclasses", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1beta1.ClusterServiceClassList{Items: scItems}, nil
	})

	var spItems []v1beta1.ClusterServicePlan
	spItems = append(spItems, *sp)
	fakeCatalogClient.AddReactor("list", "clusterserviceplans", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1beta1.ClusterServicePlanList{Items: spItems}, nil
	})

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            testClusterServicePlanGUID,
		Context: map[string]interface{}{
			"platform":  "kubernetes",
			"namespace": "test-ns",
		},
	})

	// We should get the following actions:
	// list call for ClusterServiceClass
	// list call for ClusterServicePlan
	// setReferences on ServiceInstance
	// updateStatus for inprogress
	// updateStatus for success
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 5)

	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.OneTermEqualSelector("spec.externalName", instance.Spec.ClusterServiceClassExternalName),
	}
	assertList(t, actions[0], &v1beta1.ClusterServiceClass{}, listRestrictions)

	listRestrictions = clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.ParseSelectorOrDie("spec.externalName=test-plan,spec.clusterServiceBrokerName=test-broker,spec.clusterServiceClassRef.name=SCGUID"),
	}
	assertList(t, actions[1], &v1beta1.ClusterServicePlan{}, listRestrictions)

	updatedServiceInstance := assertUpdateReference(t, actions[2], instance)

	updateObject, ok := updatedServiceInstance.(*v1beta1.ServiceInstance)
	if !ok {
		t.Fatalf("couldn't convert to *v1beta1.ServiceInstance")
	}
	if updateObject.Spec.ClusterServiceClassRef == nil || updateObject.Spec.ClusterServiceClassRef.Name != "SCGUID" {
		t.Fatalf("ClusterServiceClassRef was not resolved correctly during reconcile")
	}
	if updateObject.Spec.ClusterServicePlanRef == nil || updateObject.Spec.ClusterServicePlanRef.Name != "PGUID" {
		t.Fatalf("ClusterServicePlanRef was not resolved correctly during reconcile")
	}

	updatedServiceInstance = assertUpdateStatus(t, actions[3], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[4], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

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

	expectedEvent := normalEventBuilder(successProvisionReason).msg("The instance was provisioned successfully")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceResolvesReferences tests a simple successful
// reconciliation and making sure that the ClusterServicePlanRef is correctly
// resolved if the ClusterServiceClassRef is already set.
func TestReconcileServiceInstanceResolvesReferencesClusterServiceClassRefAlreadySet(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Response: &osb.ProvisionResponse{},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sc := getTestClusterServiceClass()
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(sc)
	sp := getTestClusterServicePlan()
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(sp)

	instance := getTestServiceInstance()
	instance.Spec.ClusterServiceClassRef = &v1beta1.ClusterObjectReference{
		Name: testClusterServiceClassGUID,
	}

	var scItems []v1beta1.ClusterServiceClass
	scItems = append(scItems, *sc)
	fakeCatalogClient.AddReactor("list", "clusterserviceclasses", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1beta1.ClusterServiceClassList{Items: scItems}, nil
	})

	var spItems []v1beta1.ClusterServicePlan
	spItems = append(spItems, *sp)
	fakeCatalogClient.AddReactor("list", "clusterserviceplans", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1beta1.ClusterServicePlanList{Items: spItems}, nil
	})

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            testClusterServicePlanGUID,
		Context: map[string]interface{}{
			"platform":  "kubernetes",
			"namespace": "test-ns",
		},
	})

	// We should get the following actions:
	// list call for ClusterServicePlan
	// setReferences on ServiceInstance
	// updateStatus for inprogress
	// updateStatus for success
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 4)

	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.ParseSelectorOrDie("spec.externalName=test-plan,spec.clusterServiceBrokerName=test-broker,spec.clusterServiceClassRef.name=SCGUID"),
	}
	assertList(t, actions[0], &v1beta1.ClusterServicePlan{}, listRestrictions)

	updatedServiceInstance := assertUpdateReference(t, actions[1], instance)

	updateObject, ok := updatedServiceInstance.(*v1beta1.ServiceInstance)
	if !ok {
		t.Fatalf("couldn't convert to *v1beta1.ServiceInstance")
	}
	if updateObject.Spec.ClusterServiceClassRef == nil || updateObject.Spec.ClusterServiceClassRef.Name != "SCGUID" {
		t.Fatalf("ClusterServiceClassRef was not resolved correctly during reconcile")
	}
	if updateObject.Spec.ClusterServicePlanRef == nil || updateObject.Spec.ClusterServicePlanRef.Name != "PGUID" {
		t.Fatalf("ClusterServicePlanRef was not resolved correctly during reconcile")
	}

	updatedServiceInstance = assertUpdateStatus(t, actions[2], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[3], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	// verify no kube resources created
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "namespaces", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	events := getRecordedEvents(testController)

	expectedEvent := normalEventBuilder(successProvisionReason).msg("The instance was provisioned successfully")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceWithInvalidParameters tests that reconcileInstance
// fails with an error when the service parameters are invalid
func TestReconcileServiceInstanceWithInvalidParameters(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

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

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// There should only be one action that says that the parameters were invalid.
	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceErrorBeforeRequest(t, updatedServiceInstance, errorWithParameters, instance)

	// only action should be a get on the namespace
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 1)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorWithParameters).msg("failed to prepare parameters")
	if err := checkEventContains(events[0], expectedEvent.String()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceWithProvisionCallFailure tests that when the provision
// call to the broker fails, the ready condition becomes false, and the
// failure condition is not set.
func TestReconcileServiceInstanceWithProvisionCallFailure(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Error: errors.New("fake creation failure"),
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithRefs()

	if err := testController.reconcileServiceInstance(instance); err == nil {
		t.Fatalf("Should not be able to make the ServiceInstance")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            testClusterServicePlanGUID,
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
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceRequestRetriableError(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, errorErrorCallingProvisionReason, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorErrorCallingProvisionReason).msgf(
		"Error communicating with broker for %q:",
		"provisioning",
	).msg("fake creation failure")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceWithProvisionFailure tests that when the
// provision call to the broker fails with an HTTP error, the ready condition
// becomes false, and the failure condition is set.
func TestReconcileServiceInstanceWithProvisionFailure(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Error: osb.HTTPStatusCodeError{
				StatusCode:   http.StatusConflict,
				ErrorMessage: strPtr("OutOfQuota"),
				Description:  strPtr("You're out of quota!"),
			},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithRefs()

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            testClusterServicePlanGUID,
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
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceRequestFailingErrorNoOrphanMitigation(
		t,
		updatedServiceInstance,
		v1beta1.ServiceInstanceOperationProvision,
		errorProvisionCallFailedReason,
		"ClusterServiceBrokerReturnedFailure",
		instance,
	)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorProvisionCallFailedReason).msgf(
		"Error provisioning ServiceInstance of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q:",
		"SCGUID", "test-serviceclass", "test-broker",
	).msgf(
		"Status: %v; ErrorMessage: %s",
		409, "OutOfQuota; Description: You're out of quota!; ResponseError: <nil>",
	)

	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstance tests synchronously provisioning a new service
func TestReconcileServiceInstance(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Response: &osb.ProvisionResponse{
				DashboardURL: &testDashboardURL,
			},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithRefs()

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            testClusterServicePlanGUID,
		OrganizationGUID:  testNamespaceGUID,
		SpaceGUID:         testNamespaceGUID,
		Context: map[string]interface{}{
			"platform":  "kubernetes",
			"namespace": "test-ns",
		},
	})

	instanceKey := testNamespace + "/" + testServiceInstanceName

	// Since synchronous operation, must not make it into the polling queue.
	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
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
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, testClusterServicePlanName, testClusterServicePlanGUID, instance)
	assertServiceInstanceDashboardURL(t, updatedServiceInstance, testDashboardURL)

	events := getRecordedEvents(testController)

	expectedEvent := normalEventBuilder(successProvisionReason).msg(successProvisionMessage)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceFailsWithDeletedPlan tests that a ServiceInstance is not
// created if the ServicePlan specified is marked as RemovedFromCatalog.
func TestReconcileServiceInstanceFailsWithDeletedPlan(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	addGetNamespaceReaction(fakeKubeClient)

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sp := getTestClusterServicePlan()
	sp.Status.RemovedFromBrokerCatalog = true
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(sp)

	instance := getTestServiceInstanceWithRefs()

	if err := testController.reconcileServiceInstance(instance); err == nil {
		t.Fatalf("This should fail")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	instanceKey := testNamespace + "/" + testServiceInstanceName

	// Since synchronous operation, must not make it into the polling queue.
	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// verify no kube actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceReadyFalse(t, updatedServiceInstance, errorDeletedClusterServicePlanReason)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorDeletedClusterServicePlanReason).msgf(
		"Service Plan %q (K8S name: %q) has been deleted, can not provision.",
		"test-plan", "PGUID",
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceFailsWithDeletedClass tests that a ServiceInstance is not
// created if the ServiceClass specified is marked as RemovedFromCatalog.
func TestReconcileServiceInstanceFailsWithDeletedClass(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	addGetNamespaceReaction(fakeKubeClient)

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sc := getTestClusterServiceClass()
	sc.Status.RemovedFromBrokerCatalog = true
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(sc)
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithRefs()

	if err := testController.reconcileServiceInstance(instance); err == nil {
		t.Fatalf("This should have failed")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	instanceKey := testNamespace + "/" + testServiceInstanceName

	// Since synchronous operation, must not make it into the polling queue.
	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	// verify no kube actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceReadyFalse(t, updatedServiceInstance, errorDeletedClusterServiceClassReason)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorDeletedClusterServiceClassReason).msgf(
		"Service Class %q (K8S name: %q) has been deleted, can not provision.",
		"test-serviceclass", "SCGUID",
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstance tests synchronously provisioning a new service
func TestReconcileServiceInstanceSuccessWithK8SNames(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Response: &osb.ProvisionResponse{
				DashboardURL: &testDashboardURL,
			},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceK8SNames()

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            testClusterServicePlanGUID,
		OrganizationGUID:  testNamespaceGUID,
		SpaceGUID:         testNamespaceGUID,
		Context: map[string]interface{}{
			"platform":  "kubernetes",
			"namespace": "test-ns",
		},
	})

	instanceKey := testNamespace + "/" + testServiceInstanceName

	// Since synchronous operation, must not make it into the polling queue.
	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	// verify no kube resources created.
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "namespaces", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	// There are 3 actions, one to update references and update status
	// twice.
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 3)

	updatedServiceInstance := assertUpdateReference(t, actions[0], instance)
	updateObject, ok := updatedServiceInstance.(*v1beta1.ServiceInstance)
	if !ok {
		t.Fatalf("couldn't convert to *v1beta1.ServiceInstance")
	}
	if updateObject.Spec.ClusterServiceClassRef == nil || updateObject.Spec.ClusterServiceClassRef.Name != "SCGUID" {
		t.Fatalf("ClusterServiceClassRef was not resolved correctly during reconcile")
	}
	if updateObject.Spec.ClusterServicePlanRef == nil || updateObject.Spec.ClusterServicePlanRef.Name != "PGUID" {
		t.Fatalf("ClusterServicePlanRef was not resolved correctly during reconcile")
	}

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[2], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, testClusterServicePlanName, testClusterServicePlanGUID, instance)
	assertServiceInstanceDashboardURL(t, updatedServiceInstance, testDashboardURL)

	events := getRecordedEvents(testController)

	expectedEvent := normalEventBuilder(successProvisionReason).msg(successProvisionMessage)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceAsynchronous tests provisioning a new service where
// the request results in a async response.  Resulting status will indicate
// not ready and polling in progress.
func TestReconcileServiceInstanceAsynchronous(t *testing.T) {
	key := osb.OperationKey(testOperation)
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Response: &osb.ProvisionResponse{
				Async:        true,
				DashboardURL: &testDashboardURL,
				OperationKey: &key,
			},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithRefs()
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            testClusterServicePlanGUID,
		OrganizationGUID:  testNamespaceGUID,
		SpaceGUID:         testNamespaceGUID,
		Context: map[string]interface{}{
			"platform":  "kubernetes",
			"namespace": testNamespace,
		},
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceAsyncInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, testOperation, testClusterServicePlanName, testClusterServicePlanGUID, instance)
	assertServiceInstanceDashboardURL(t, updatedServiceInstance, testDashboardURL)

	// verify no kube resources created.
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	if e, a := 1, len(kubeActions); e != a {
		t.Fatalf("Unexpected number of actions: expected %v, got %v", e, a)
	}

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 1 {
		t.Fatalf("Expected polling queue to have a record of seeing test instance once")
	}
}

// TestReconcileServiceInstanceAsynchronousNoOperation tests an async provision
// scenario.  This differs from TestReconcileServiceInstanceAsynchronous() as
// there is no operation key returned by OSB.
func TestReconcileServiceInstanceAsynchronousNoOperation(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Response: &osb.ProvisionResponse{
				Async:        true,
				DashboardURL: &testDashboardURL,
			},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithRefs()
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            testClusterServicePlanGUID,
		OrganizationGUID:  testNamespaceGUID,
		SpaceGUID:         testNamespaceGUID,
		Context: map[string]interface{}{
			"platform":  "kubernetes",
			"namespace": "test-ns",
		},
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceAsyncInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, "", testClusterServicePlanName, testClusterServicePlanGUID, instance)
	assertServiceInstanceDashboardURL(t, updatedServiceInstance, testDashboardURL)

	// verify no kube resources created.
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	if e, a := 1, len(kubeActions); e != a {
		t.Fatalf("Unexpected number of actions: expected %v, got %v", e, a)
	}

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 1 {
		t.Fatalf("Expected polling queue to have a record of seeing test instance once")
	}
}

// TestReconcileServiceInstanceNamespaceError test reconciling an instance where kube
// client fails to get a namespace to create instance in.
func TestReconcileServiceInstanceNamespaceError(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	fakeKubeClient.AddReactor("get", "namespaces", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &corev1.Namespace{}, errors.New("No namespace")
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithRefs()

	if err := testController.reconcileServiceInstance(instance); err == nil {
		t.Fatalf("There should not be a namespace for the ServiceInstance to be created in")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

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

	expectedEvent := warningEventBuilder(errorFindingNamespaceServiceInstanceReason).msgf(
		"Failed to get namespace %q during instance create:",
		"test-ns",
	).msg("No namespace")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceDelete tests deleting/deprovisioning an instance
func TestReconcileServiceInstanceDelete(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		DeprovisionReaction: &fakeosb.DeprovisionReaction{
			Response: &osb.DeprovisionResponse{},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithRefs()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{v1beta1.FinalizerServiceCatalog}
	// we only invoke the broker client to deprovision if we have a reconciled generation set
	// as that implies a previous success.
	instance.Generation = 2
	instance.Status.ReconciledGeneration = 1
	instance.Status.ExternalProperties = &v1beta1.ServiceInstancePropertiesState{
		ClusterServicePlanExternalName: testClusterServicePlanName,
		ClusterServicePlanExternalID:   testClusterServicePlanGUID,
	}
	instance.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusRequired

	fakeCatalogClient.AddReactor("get", "serviceinstances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, instance, nil
	})

	err := testController.reconcileServiceInstance(instance)
	if err != nil {
		t.Fatalf("This should not fail")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertDeprovision(t, brokerActions[0], &osb.DeprovisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            testClusterServicePlanGUID,
	})

	// Verify no core kube actions occurred
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationDeprovision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationDeprovision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	events := getRecordedEvents(testController)

	expectedEvent := normalEventBuilder(successDeprovisionReason).msg("The instance was deprovisioned successfully")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceDeleteBlockedByCredentials tests
// deleting/deprovisioning an instance that has ServiceBindings.
// Instance reconcilation will set the Ready condition to false with a msg
// indicating the delete is blocked until the credentials are removed.
func TestReconcileServiceInstanceDeleteBlockedByCredentials(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		DeprovisionReaction: &fakeosb.DeprovisionReaction{
			Response: &osb.DeprovisionResponse{},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())
	credentials := getTestServiceBinding()
	sharedInformers.ServiceBindings().Informer().GetStore().Add(credentials)

	instance := getTestServiceInstanceWithRefs()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{v1beta1.FinalizerServiceCatalog}
	// we only invoke the broker client to deprovision if we have a reconciled generation set
	// as that implies a previous success.
	instance.Generation = 2
	instance.Status.ReconciledGeneration = 1
	instance.Status.ExternalProperties = &v1beta1.ServiceInstancePropertiesState{
		ClusterServicePlanExternalName: testClusterServicePlanName,
		ClusterServicePlanExternalID:   testClusterServicePlanGUID,
	}
	instance.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusRequired

	fakeCatalogClient.AddReactor("get", "serviceinstances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, instance, nil
	})

	err := testController.reconcileServiceInstance(instance)
	if err != nil {
		t.Fatalf("reconcileServiceInstance() returned an error:  %v", err.Error())
	}

	brokerActions := fakeBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	// Verify no core kube actions occurred
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceErrorBeforeRequest(t, updatedServiceInstance, "DeprovisionBlockedByExistingCredentials", instance)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorDeprovisionBlockedByCredentialsReason).msg(
		"Delete instance blocked by existing ServiceBindings associated with this instance.  All credentials must be removed first",
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}

	// delete credentials
	sharedInformers.ServiceBindings().Informer().GetStore().Delete(credentials)

	fakeKubeClient.ClearActions()
	fakeCatalogClient.ClearActions()

	// credentials were removed, verify the next reconcilation removes
	// the instance

	err = testController.reconcileServiceInstance(instance)
	if err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions = fakeBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertDeprovision(t, brokerActions[0], &osb.DeprovisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            testClusterServicePlanGUID,
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
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationDeprovision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationDeprovision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	events = getRecordedEvents(testController)

	expectedEvent = normalEventBuilder(successDeprovisionReason).msg("The instance was deprovisioned successfully")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileServiceInstanceDeleteAsynchronous(t *testing.T) {
	key := osb.OperationKey(testOperation)
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		DeprovisionReaction: &fakeosb.DeprovisionReaction{
			Response: &osb.DeprovisionResponse{
				Async:        true,
				OperationKey: &key,
			},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithRefs()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{v1beta1.FinalizerServiceCatalog}
	// we only invoke the broker client to deprovision if we have a reconciled generation set
	// as that implies a previous success.
	instance.Generation = 2
	instance.Status.ReconciledGeneration = 1
	instance.Status.ExternalProperties = &v1beta1.ServiceInstancePropertiesState{
		ClusterServicePlanExternalName: testClusterServicePlanName,
		ClusterServicePlanExternalID:   testClusterServicePlanGUID,
	}
	instance.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusRequired

	fakeCatalogClient.AddReactor("get", "serviceinstances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, instance, nil
	})

	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.reconcileServiceInstance(instance)
	if err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	// The item should've been added to the instancePollingQueue for later processing

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 1 {
		t.Fatalf("Expected polling queue to have a record of seeing test instance once")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertDeprovision(t, brokerActions[0], &osb.DeprovisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            testClusterServicePlanGUID,
	})

	// Verify no core kube actions occurred
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationDeprovision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceAsyncInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationDeprovision, testOperation, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	events := getRecordedEvents(testController)

	expectedEvent := normalEventBuilder(asyncDeprovisioningReason).msg("The instance is being deprovisioned asynchronously")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceDeleteFailedProvisionWithoutRequest tests that
// an instance that failed to provision without making a provision request
// will be finalized, but no deprovision request will be sent to the broker.
func TestReconcileServiceInstanceDeleteFailedProvisionWithoutRequest(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithFailedStatus()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{v1beta1.FinalizerServiceCatalog}
	instance.Status.ExternalProperties = &v1beta1.ServiceInstancePropertiesState{}
	instance.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusNotRequired

	instance.Generation = 1
	instance.Status.ReconciledGeneration = 0

	fakeCatalogClient.AddReactor("get", "serviceinstances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, instance, nil
	})

	err := testController.reconcileServiceInstance(instance)
	if err != nil {
		t.Fatalf("Unexpected error from reconcileServiceInstance: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

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

// TestReconcileServiceInstanceDeleteFailedProvisionWithRequest tests that an
// instance that failed to provision but for which a provision request was
// made will have a deprovision request sent to the broker.
func TestReconcileServiceInstanceDeleteFailedProvisionWithRequest(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		DeprovisionReaction: &fakeosb.DeprovisionReaction{
			Response: &osb.DeprovisionResponse{},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithFailedStatus()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{v1beta1.FinalizerServiceCatalog}
	instance.Status.ExternalProperties = nil
	instance.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusRequired

	instance.Generation = 2
	instance.Status.ReconciledGeneration = 1

	fakeCatalogClient.AddReactor("get", "serviceinstances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, instance, nil
	})

	err := testController.reconcileServiceInstance(instance)
	if err != nil {
		t.Fatalf("Unexpected error from reconcileServiceInstance: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertDeprovision(t, brokerActions[0], &osb.DeprovisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            testClusterServicePlanGUID,
	})

	// Verify no core kube actions occurred
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationDeprovision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationDeprovision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	events := getRecordedEvents(testController)

	expectedEvent := normalEventBuilder(successDeprovisionReason).msg("The instance was deprovisioned successfully")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceDeleteWhenAlreadyDeprovisionedSuccessfully
// tests that an instance that has already been deprovisioned will be
// finalized, but no deprovision request will be sent to the broker.
func TestReconcileServiceInstanceDeleteWhenAlreadyDeprovisionedSuccessfully(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithFailedStatus()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{v1beta1.FinalizerServiceCatalog}
	instance.Status.ExternalProperties = nil
	instance.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusSucceeded

	instance.Generation = 2
	instance.Status.ReconciledGeneration = 1

	fakeCatalogClient.AddReactor("get", "serviceinstances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, instance, nil
	})

	err := testController.reconcileServiceInstance(instance)
	if err != nil {
		t.Fatalf("Unexpected error from reconcileServiceInstance: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

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

// TestReconcileServiceInstanceDeleteWhenAlreadyDeprovisionedUnsuccessfully
// tests that an instance that has already had a failed deprovision request
// will not be finalized and no further deprovision request will be sent to
// the broker.
func TestReconcileServiceInstanceDeleteWhenAlreadyDeprovisionedUnsuccessfully(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithFailedStatus()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{v1beta1.FinalizerServiceCatalog}
	instance.Status.ExternalProperties = &v1beta1.ServiceInstancePropertiesState{}
	instance.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusFailed

	instance.Generation = 2
	instance.Status.ReconciledGeneration = 1

	fakeCatalogClient.AddReactor("get", "serviceinstances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, instance, nil
	})

	err := testController.reconcileServiceInstance(instance)
	if err != nil {
		t.Fatalf("Unexpected error from reconcileServiceInstance: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	// Verify no core kube actions occurred
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	// Verify no catalog client actions occurred
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 0)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 0)
}

// TestReconcileServiceInstanceDeleteFailedUpdate tests that an instance
// that failed after having been successfully provisioned will send a
// deprovision request to the broker.
func TestReconcileServiceInstanceDeleteFailedUpdate(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		DeprovisionReaction: &fakeosb.DeprovisionReaction{
			Response: &osb.DeprovisionResponse{},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithRefs()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{v1beta1.FinalizerServiceCatalog}
	instance.Status.ExternalProperties = &v1beta1.ServiceInstancePropertiesState{
		ClusterServicePlanExternalName: testClusterServicePlanName,
		ClusterServicePlanExternalID:   testClusterServicePlanGUID,
	}
	instance.Generation = 2
	instance.Status.ReconciledGeneration = 2
	instance.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusRequired

	fakeCatalogClient.AddReactor("get", "serviceinstances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, instance, nil
	})

	err := testController.reconcileServiceInstance(instance)
	if err != nil {
		t.Fatalf("Unexpected error from reconcileServiceInstance: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertDeprovision(t, brokerActions[0], &osb.DeprovisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            testClusterServicePlanGUID,
	})

	// Verify no core kube actions occurred
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationDeprovision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationDeprovision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	events := getRecordedEvents(testController)

	expectedEvent := normalEventBuilder(successDeprovisionReason).msg("The instance was deprovisioned successfully")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}

	assertEmptyFinalizers(t, updatedServiceInstance)
}

// TestReconcileServiceInstanceDeleteDoesNotInvokeClusterServiceBroker verfies that if an instance
// is created that is never actually provisioned the instance is able to be
// deleted and is not blocked by any interaction with a broker (since its very
// likely that a broker never actually existed).
func TestReconcileServiceInstanceDeleteDoesNotInvokeClusterServiceBroker(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithRefs()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{v1beta1.FinalizerServiceCatalog}
	instance.Generation = 1
	instance.Status.ReconciledGeneration = 0
	instance.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusNotRequired

	fakeCatalogClient.AddReactor("get", "serviceinstances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, instance, nil
	})

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

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
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithFailedStatus()

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
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
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State:       osb.StateInProgress,
				Description: strPtr(lastOperationDescription),
			},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceAsyncProvisioning(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.pollServiceInstance(instance)
	if err != nil {
		t.Fatalf("pollServiceInstance failed: %s", err)
	}

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 1 {
		t.Fatalf("Expected polling queue to have record of seeing test instance once")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   testServiceInstanceGUID,
		ServiceID:    strPtr(testClusterServiceClassGUID),
		PlanID:       strPtr(testClusterServicePlanGUID),
		OperationKey: &operationKey,
	})

	// there should have been 1 action to update the status with the last operation description
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceAsyncInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, testOperation, testClusterServicePlanName, testClusterServicePlanGUID, instance)
	assertServiceInstanceConditionHasLastOperationDescription(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, lastOperationDescription)

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)
}

// TestPollServiceInstanceSuccessProvisioningWithOperation tests polling an
// instance that is already in process of provisioning (background/
// asynchronously) and is found to be ready
func TestPollServiceInstanceSuccessProvisioningWithOperation(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State:       osb.StateSucceeded,
				Description: strPtr(lastOperationDescription),
			},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceAsyncProvisioning(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.pollServiceInstance(instance)
	if err != nil {
		t.Fatalf("pollServiceInstance failed: %s", err)
	}

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance as polling should have completed")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   testServiceInstanceGUID,
		ServiceID:    strPtr(testClusterServiceClassGUID),
		PlanID:       strPtr(testClusterServicePlanGUID),
		OperationKey: &operationKey,
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, testClusterServicePlanName, testClusterServicePlanGUID, instance)
}

// TestPollServiceInstanceFailureProvisioningWithOperation tests polling an
// instance where provision was in process asynchronously but has an updated
// status of failed to provision.
func TestPollServiceInstanceFailureProvisioningWithOperation(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State: osb.StateFailed,
			},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceAsyncProvisioning(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.pollServiceInstance(instance)
	if err != nil {
		t.Fatalf("pollServiceInstance failed: %s", err)
	}

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance as polling should have completed")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   testServiceInstanceGUID,
		ServiceID:    strPtr(testClusterServiceClassGUID),
		PlanID:       strPtr(testClusterServicePlanGUID),
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
		v1beta1.ServiceInstanceOperationProvision,
		errorProvisionCallFailedReason,
		errorProvisionCallFailedReason,
		instance,
	)
}

// TestPollServiceInstanceInProgressDeprovisioningWithOperationNoFinalizer tests
// polling an instance that was asynchronously being deprovisioned and is still
// in progress.
func TestPollServiceInstanceInProgressDeprovisioningWithOperationNoFinalizer(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State:       osb.StateInProgress,
				Description: strPtr(lastOperationDescription),
			},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceAsyncDeprovisioning(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.pollServiceInstance(instance)
	if err != nil {
		t.Fatalf("pollServiceInstance failed: %s", err)
	}

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 1 {
		t.Fatalf("Expected polling queue to have record of seeing test instance once")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   testServiceInstanceGUID,
		ServiceID:    strPtr(testClusterServiceClassGUID),
		PlanID:       strPtr(testClusterServicePlanGUID),
		OperationKey: &operationKey,
	})

	// there should have been 1 action to update the instance status with the last operation
	// description
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceAsyncInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationDeprovision, testOperation, testClusterServicePlanName, testClusterServicePlanGUID, instance)
	assertServiceInstanceConditionHasLastOperationDescription(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationDeprovision, lastOperationDescription)

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)
}

// TestPollServiceInstanceSuccessDeprovisioningWithOperationNoFinalizer tests
// polling an instance that was asynchronously being deprovisioned and its
// current poll status succeeded.  Verify instance is deprovisioned.
func TestPollServiceInstanceSuccessDeprovisioningWithOperationNoFinalizer(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State: osb.StateSucceeded,
			},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceAsyncDeprovisioning(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.pollServiceInstance(instance)
	if err != nil {
		t.Fatalf("pollServiceInstance failed: %s", err)
	}

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance as polling should have completed")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   testServiceInstanceGUID,
		ServiceID:    strPtr(testClusterServiceClassGUID),
		PlanID:       strPtr(testClusterServicePlanGUID),
		OperationKey: &operationKey,
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationDeprovision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	events := getRecordedEvents(testController)

	expectedEvent := normalEventBuilder(successDeprovisionReason).msg("The instance was deprovisioned successfully")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestPollServiceInstanceFailureDeprovisioning tests polling an
// instance that has a async deprovision in progress where the broker responds
// with Failed.
func TestPollServiceInstanceFailureDeprovisioning(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State: osb.StateFailed,
			},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceAsyncDeprovisioning(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.pollServiceInstance(instance)
	if err != nil {
		t.Fatalf("pollServiceInstance failed: %s", err)
	}

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 1 {
		t.Fatalf("Expected polling queue to have record of seeing test instance once")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   testServiceInstanceGUID,
		ServiceID:    strPtr(testClusterServiceClassGUID),
		PlanID:       strPtr(testClusterServicePlanGUID),
		OperationKey: &operationKey,
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceRequestRetriableError(
		t,
		updatedServiceInstance,
		v1beta1.ServiceInstanceOperationDeprovision,
		errorDeprovisionCalledReason,
		"", // plan name
		"", // plan ID
		instance,
	)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorDeprovisionCalledReason).msg("Deprovision call failed: (no description provided)")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestPollServiceInstanceFailureDeprovisioningWithReconciliationTimeout tests
// polling an instance that has a async deprovision in progress where the
// broker responds with Failed and the reconciliation retry duration has been
// exhausted.
func TestPollServiceInstanceFailureDeprovisioningWithReconciliationTimeout(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State: osb.StateFailed,
			},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceAsyncDeprovisioning(testOperation)
	startTime := metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
	instance.Status.OperationStartTime = &startTime
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.pollServiceInstance(instance)
	if err != nil {
		t.Fatalf("pollServiceInstance failed: %s", err)
	}

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance as polling should have completed")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   testServiceInstanceGUID,
		ServiceID:    strPtr(testClusterServiceClassGUID),
		PlanID:       strPtr(testClusterServicePlanGUID),
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
		v1beta1.ServiceInstanceOperationDeprovision,
		errorReconciliationRetryTimeoutReason,
		errorReconciliationRetryTimeoutReason,
		instance,
	)

	events := getRecordedEvents(testController)
	expectedEvents := []string{
		warningEventBuilder(errorDeprovisionCalledReason).msg("Deprovision call failed: (no description provided)").String(),
		warningEventBuilder(errorReconciliationRetryTimeoutReason).msg("Stopping reconciliation retries on ServiceInstance because too much time has elapsed").String(),
	}

	if err := checkEvents(events, expectedEvents); err != nil {
		t.Fatal(err)
	}
}

// TestPollServiceInstanceStatusGoneDeprovisioningWithOperationNoFinalizer test
// polling an instance that has a async deprovision in progress.  Current poll
// status is Gone (which is fine).  Verify successful deprovisioning.
func TestPollServiceInstanceStatusGoneDeprovisioningWithOperationNoFinalizer(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Error: osb.HTTPStatusCodeError{
				StatusCode: http.StatusGone,
			},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceAsyncDeprovisioning(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.pollServiceInstance(instance)
	if err != nil {
		t.Fatalf("pollServiceInstance failed: %s", err)
	}

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance as polling should have completed")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   testServiceInstanceGUID,
		ServiceID:    strPtr(testClusterServiceClassGUID),
		PlanID:       strPtr(testClusterServicePlanGUID),
		OperationKey: &operationKey,
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationDeprovision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	events := getRecordedEvents(testController)

	expectedEvent := normalEventBuilder(successDeprovisionReason).msg("The instance was deprovisioned successfully")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestPollServiceInstanceClusterServiceBrokerError simulates polling a broker and getting a
// Forbidden status on the poll.  Test simulates that the ClusterServiceBroker was already
// in the process of being deleted prior to the Forbidden status.
func TestPollServiceInstanceClusterServiceBrokerError(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Error: osb.HTTPStatusCodeError{
				StatusCode: http.StatusForbidden,
			},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceAsyncDeprovisioning(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.pollServiceInstance(instance)
	if err != nil {
		t.Fatalf("pollServiceInstance failed: %v", err)
	}

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 1 {
		t.Fatalf("Expected polling queue to have record of seeing test instance once")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   testServiceInstanceGUID,
		ServiceID:    strPtr(testClusterServiceClassGUID),
		PlanID:       strPtr(testClusterServicePlanGUID),
		OperationKey: &operationKey,
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 0)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorPollingLastOperationReason).msg(
		"Error polling last operation:",
	).msg("Status: 403; ErrorMessage: <nil>; Description: <nil>; ResponseError: <nil>")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestPollServiceInstanceSuccessDeprovisioningWithOperationWithFinalizer tests
// polling with instance while it is in deprovisioning state to ensure after
// the poll the service is properly removed
func TestPollServiceInstanceSuccessDeprovisioningWithOperationWithFinalizer(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State: osb.StateSucceeded,
			},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceAsyncDeprovisioningWithFinalizer(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName

	// updateServiceInstanceFinalizers fetches the latest object.
	fakeCatalogClient.AddReactor("get", "serviceinstances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, instance, nil
	})

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.pollServiceInstance(instance)
	if err != nil {
		t.Fatalf("pollServiceInstance failed: %s", err)
	}

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance as polling should have completed")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   testServiceInstanceGUID,
		ServiceID:    strPtr(testClusterServiceClassGUID),
		PlanID:       strPtr(testClusterServicePlanGUID),
		OperationKey: &operationKey,
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationDeprovision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	events := getRecordedEvents(testController)

	expectedEvent := normalEventBuilder(successDeprovisionReason).msg("The instance was deprovisioned successfully")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceSuccessOnFinalRetry verifies that reconciliation
// can succeed on the last attempt before timing out of the retry loop
func TestReconcileServiceInstanceSuccessOnFinalRetry(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Response: &osb.ProvisionResponse{},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithRefs()
	instance.Status.CurrentOperation = v1beta1.ServiceInstanceOperationProvision

	startTime := metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
	instance.Status.OperationStartTime = &startTime

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            testClusterServicePlanGUID,
		Context: map[string]interface{}{
			"platform":  "kubernetes",
			"namespace": "test-ns",
		},
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	// verify no kube resources created
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	if err := checkKubeClientActions(kubeActions, []kubeClientAction{
		{verb: "get", resourceName: "namespaces", checkType: checkGetActionType},
	}); err != nil {
		t.Fatal(err)
	}

	events := getRecordedEvents(testController)

	expectedEvent := normalEventBuilder(successProvisionReason).msg("The instance was provisioned successfully")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceFailureOnFinalRetry verifies that reconciliation
// completes in the event of an error after the retry duration elapses.
func TestReconcileServiceInstanceFailureOnFinalRetry(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Error: errors.New("fake creation failure"),
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithRefs()
	instance.Status.CurrentOperation = v1beta1.ServiceInstanceOperationProvision
	startTime := metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
	instance.Status.OperationStartTime = &startTime

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("Should have returned no error because the retry duration has elapsed: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            testClusterServicePlanGUID,
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
		v1beta1.ServiceInstanceOperationProvision,
		errorErrorCallingProvisionReason,
		errorReconciliationRetryTimeoutReason,
		instance,
	)

	events := getRecordedEvents(testController)

	expectedEventPrefixes := []string{
		corev1.EventTypeWarning + " " + errorErrorCallingProvisionReason,
		corev1.EventTypeWarning + " " + errorReconciliationRetryTimeoutReason,
	}

	if err := checkEventPrefixes(events, expectedEventPrefixes); err != nil {
		t.Fatal(err)
	}
}

// TestPollServiceInstanceSuccessOnFinalRetry verifies that polling
// can succeed on the last attempt before timing out of the retry loop
func TestPollServiceInstanceSuccessOnFinalRetry(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State:       osb.StateSucceeded,
				Description: strPtr(lastOperationDescription),
			},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceAsyncProvisioning(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName
	startTime := metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
	instance.Status.OperationStartTime = &startTime

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	if err := testController.pollServiceInstance(instance); err != nil {
		t.Fatalf("pollServiceInstance failed: %s", err)
	}

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance as polling should have completed")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   testServiceInstanceGUID,
		ServiceID:    strPtr(testClusterServiceClassGUID),
		PlanID:       strPtr(testClusterServicePlanGUID),
		OperationKey: &operationKey,
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, testClusterServicePlanName, testClusterServicePlanGUID, instance)
}

// TestPollServiceInstanceFailureOnFinalRetry verifies that polling
// completes in the event of an error after the retry duration elapses.
func TestPollServiceInstanceFailureOnFinalRetry(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State:       osb.StateInProgress,
				Description: strPtr(lastOperationDescription),
			},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceAsyncProvisioning(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName
	startTime := metav1.NewTime(time.Now().Add(-7 * 24 * time.Hour))
	instance.Status.OperationStartTime = &startTime

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	if err := testController.pollServiceInstance(instance); err != nil {
		t.Fatalf("Should have return no error because the retry duration has elapsed: %v", err)
	}

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance as polling should have completed")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   testServiceInstanceGUID,
		ServiceID:    strPtr(testClusterServiceClassGUID),
		PlanID:       strPtr(testClusterServicePlanGUID),
		OperationKey: &operationKey,
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceRequestFailingErrorStartOrphanMitigation(
		t,
		updatedServiceInstance,
		v1beta1.ServiceInstanceOperationProvision,
		startingInstanceOrphanMitigationReason,
		errorReconciliationRetryTimeoutReason,
		instance,
	)

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
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, noFakeActions())

	addGetNamespaceReaction(fakeKubeClient)

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

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

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

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
	instanceWithCondition := func(condition *v1beta1.ServiceInstanceCondition) *v1beta1.ServiceInstance {
		instance := getTestServiceInstance()
		instance.Status.Conditions = []v1beta1.ServiceInstanceCondition{*condition}

		return instance
	}

	// The value of the LastTransitionTime field on conditions has to be
	// tested to ensure it is updated correctly.
	//
	// Time basis for all condition changes:
	newTs := metav1.Now()
	oldTs := metav1.NewTime(newTs.Add(-5 * time.Minute))

	// condition is a shortcut method for creating conditions with the 'old' timestamp.
	condition := func(cType v1beta1.ServiceInstanceConditionType, status v1beta1.ConditionStatus, s ...string) *v1beta1.ServiceInstanceCondition {
		c := &v1beta1.ServiceInstanceCondition{
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

	readyFalse := func() *v1beta1.ServiceInstanceCondition {
		return condition(v1beta1.ServiceInstanceConditionReady, v1beta1.ConditionFalse, "Reason", "Message")
	}

	readyFalsef := func(reason, message string) *v1beta1.ServiceInstanceCondition {
		return condition(v1beta1.ServiceInstanceConditionReady, v1beta1.ConditionFalse, reason, message)
	}

	readyTrue := func() *v1beta1.ServiceInstanceCondition {
		return condition(v1beta1.ServiceInstanceConditionReady, v1beta1.ConditionTrue, "Reason", "Message")
	}

	failedTrue := func() *v1beta1.ServiceInstanceCondition {
		return condition(v1beta1.ServiceInstanceConditionFailed, v1beta1.ConditionTrue, "Reason", "Message")
	}

	// withNewTs sets the LastTransitionTime to the 'new' basis time and
	// returns it.
	withNewTs := func(c *v1beta1.ServiceInstanceCondition) *v1beta1.ServiceInstanceCondition {
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
		input     *v1beta1.ServiceInstance
		condition *v1beta1.ServiceInstanceCondition
		result    *v1beta1.ServiceInstance
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
			result: func() *v1beta1.ServiceInstance {
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
	getTestServiceInstanceWithStatus := func(status v1beta1.ConditionStatus) *v1beta1.ServiceInstance {
		instance := getTestServiceInstance()
		instance.Status = v1beta1.ServiceInstanceStatus{
			Conditions: []v1beta1.ServiceInstanceCondition{{
				Type:               v1beta1.ServiceInstanceConditionReady,
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
		input                 *v1beta1.ServiceInstance
		status                v1beta1.ConditionStatus
		reason                string
		message               string
		transitionTimeChanged bool
	}{

		{
			name:                  "initially unset",
			input:                 getTestServiceInstance(),
			status:                v1beta1.ConditionFalse,
			message:               "message",
			transitionTimeChanged: true,
		},
		{
			name:                  "not ready -> not ready",
			input:                 getTestServiceInstanceWithStatus(v1beta1.ConditionFalse),
			status:                v1beta1.ConditionFalse,
			transitionTimeChanged: false,
		},
		{
			name:                  "not ready -> not ready, reason and message change",
			input:                 getTestServiceInstanceWithStatus(v1beta1.ConditionFalse),
			status:                v1beta1.ConditionFalse,
			reason:                "foo",
			message:               "bar",
			transitionTimeChanged: false,
		},
		{
			name:                  "not ready -> ready",
			input:                 getTestServiceInstanceWithStatus(v1beta1.ConditionFalse),
			status:                v1beta1.ConditionTrue,
			message:               "message",
			transitionTimeChanged: true,
		},
		{
			name:                  "ready -> ready",
			input:                 getTestServiceInstanceWithStatus(v1beta1.ConditionTrue),
			status:                v1beta1.ConditionTrue,
			message:               "message",
			transitionTimeChanged: false,
		},
		{
			name:                  "ready -> not ready",
			input:                 getTestServiceInstanceWithStatus(v1beta1.ConditionTrue),
			status:                v1beta1.ConditionFalse,
			message:               "message",
			transitionTimeChanged: true,
		},
		{
			name:                  "message -> message2",
			input:                 getTestServiceInstanceWithStatus(v1beta1.ConditionTrue),
			status:                v1beta1.ConditionFalse,
			message:               "message2",
			transitionTimeChanged: true,
		},
	}

	for _, tc := range cases {
		_, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, _ := newTestController(t, noFakeActions())

		clone, err := api.Scheme.DeepCopy(tc.input)
		if err != nil {
			t.Errorf("%v: deep copy failed", tc.name)
			continue
		}
		inputClone := clone.(*v1beta1.ServiceInstance)

		err = testController.updateServiceInstanceCondition(tc.input, v1beta1.ServiceInstanceConditionReady, tc.status, tc.reason, tc.message)
		if err != nil {
			t.Errorf("%v: error updating instance condition: %v", tc.name, err)
			continue
		}

		brokerActions := fakeClusterServiceBrokerClient.Actions()
		assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

		if !reflect.DeepEqual(tc.input, inputClone) {
			t.Errorf("%v: updating broker condition mutated input: %s", tc.name, expectedGot(inputClone, tc.input))
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

		updateActionObject, ok := updatedServiceInstance.(*v1beta1.ServiceInstance)
		if !ok {
			t.Errorf("%v: couldn't convert to instance", tc.name)
			continue
		}

		var initialTs metav1.Time
		if len(inputClone.Status.Conditions) != 0 {
			initialTs = inputClone.Status.Conditions[0].LastTransitionTime
		}

		if e, a := 1, len(updateActionObject.Status.Conditions); e != a {
			t.Errorf("%v: condition(s) %s", tc.name, expectedGot(e, a))
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

			sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
			sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
			sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

			instance := getTestServiceInstanceWithRefs()
			if tc.includeUserInfo {
				instance.Spec.UserInfo = testUserInfo
			}

			if err := testController.reconcileServiceInstance(instance); err != nil {
				t.Fatalf("This should not fail : %v", err)
			}

			brokerActions := fakeBrokerClient.Actions()
			assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
			actualRequest, ok := brokerActions[0].Request.(*osb.ProvisionRequest)
			if !ok {
				t.Errorf("%v: unexpected request type; expected %T, got %T", tc.name, &osb.ProvisionRequest{}, actualRequest)
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

			sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
			sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
			sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

			instance := getTestServiceInstanceWithRefs()
			instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
			instance.ObjectMeta.Finalizers = []string{v1beta1.FinalizerServiceCatalog}
			// we only invoke the broker client to deprovision if we have a
			// ReconciledGeneration set as that implies a previous success.
			instance.Generation = 2
			instance.Status.ReconciledGeneration = 1
			instance.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusRequired
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
			assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
			actualRequest, ok := brokerActions[0].Request.(*osb.DeprovisionRequest)
			if !ok {
				t.Errorf("%v: unexpected request type; expected %T, got %T", tc.name, &osb.DeprovisionRequest{}, actualRequest)
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

			sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
			sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
			sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

			instance := getTestServiceInstanceAsyncProvisioning(testOperation)
			if tc.includeUserInfo {
				instance.Spec.UserInfo = testUserInfo
			}

			err := testController.pollServiceInstance(instance)
			if err != nil {
				t.Fatalf("Expected pollServiceInstance to not fail while in progress")
			}

			brokerActions := fakeBrokerClient.Actions()
			assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
			actualRequest, ok := brokerActions[0].Request.(*osb.LastOperationRequest)
			if !ok {
				t.Errorf("%v: unexpected request type; expected %T, got %T", tc.name, &osb.LastOperationRequest{}, actualRequest)
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

		sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
		sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
		sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

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
		updatedServiceInstance, _ := updatedObject.(*v1beta1.ServiceInstance)

		if ok := testServiceInstanceOrphanMitigationInProgress(t, tc.name, errorf, updatedServiceInstance, tc.triggersOrphanMitigation); !ok {
			continue
		}

		if tc.triggersOrphanMitigation {
			// TODO(mkibbe): Rework this to be an expects, not asserts
			assertServiceInstanceStartingOrphanMitigation(t, updatedServiceInstance, instance)
			if err == nil {
				t.Errorf("%v: Reconciler should return error so that instance is orphan mitigated", tc.name)
				continue
			}
		} else {
			if err != nil {
				t.Errorf("%v: Reconciler should treat as terminal condition and not requeue", tc.name)
				continue
			}
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

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithRefs()

	if err := testController.reconcileServiceInstance(instance); err == nil {
		t.Fatal("Reconciler should return error for timeout so that instance is orphan mitigated")
	}

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedObject := assertUpdateStatus(t, actions[1], instance)
	updatedServiceInstance, ok := updatedObject.(*v1beta1.ServiceInstance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1beta1.ServiceInstance", updatedObject)
	}

	assertServiceInstanceReadyCondition(t, updatedServiceInstance, v1beta1.ConditionFalse, startingInstanceOrphanMitigationReason)
	assertServiceInstanceOrphanMitigationInProgressTrue(t, updatedServiceInstance)
}

func TestReconcileServiceInstanceOrphanMitigation(t *testing.T) {
	key := osb.OperationKey(testOperation)
	description := "description"
	// invalidState := "invalid state"

	cases := []struct {
		name                         string
		deprovReaction               *fakeosb.DeprovisionReaction
		pollReaction                 *fakeosb.PollLastOperationReaction
		async                        bool
		finishedOrphanMitigation     bool
		shouldError                  bool
		retryDurationExceeded        bool
		expectedReadyConditionStatus v1beta1.ConditionStatus
		expectedReadyConditionReason string
	}{
		// Synchronous
		{
			name: "sync - success",
			deprovReaction: &fakeosb.DeprovisionReaction{
				Response: &osb.DeprovisionResponse{},
			},
			finishedOrphanMitigation:     true,
			expectedReadyConditionStatus: v1beta1.ConditionFalse,
			expectedReadyConditionReason: successOrphanMitigationReason,
		},
		{
			name: "sync - 202 accepted",
			deprovReaction: &fakeosb.DeprovisionReaction{
				Response: &osb.DeprovisionResponse{
					Async:        true,
					OperationKey: &key,
				},
			},
			finishedOrphanMitigation:     false,
			expectedReadyConditionStatus: v1beta1.ConditionFalse,
			expectedReadyConditionReason: asyncDeprovisioningReason,
		},
		{
			name: "sync - http error",
			deprovReaction: &fakeosb.DeprovisionReaction{
				Error: fakeosb.AsyncRequiredError(),
			},
			finishedOrphanMitigation:     false,
			shouldError:                  true,
			expectedReadyConditionStatus: v1beta1.ConditionUnknown,
			expectedReadyConditionReason: errorDeprovisionCalledReason,
		},
		{
			name: "sync - http error - retry duration exceeded",
			deprovReaction: &fakeosb.DeprovisionReaction{
				Error: fakeosb.AsyncRequiredError(),
			},
			finishedOrphanMitigation:     true,
			retryDurationExceeded:        true,
			expectedReadyConditionStatus: v1beta1.ConditionUnknown,
			expectedReadyConditionReason: errorOrphanMitigationFailedReason,
		},
		{
			name: "sync - other error",
			deprovReaction: &fakeosb.DeprovisionReaction{
				Error: fmt.Errorf("other error"),
			},
			finishedOrphanMitigation:     false,
			shouldError:                  true,
			expectedReadyConditionStatus: v1beta1.ConditionUnknown,
			expectedReadyConditionReason: errorDeprovisionCalledReason,
		},
		{
			name: "sync - other error - retry duration exceeded",
			deprovReaction: &fakeosb.DeprovisionReaction{
				Error: fmt.Errorf("other error"),
			},
			finishedOrphanMitigation:     true,
			retryDurationExceeded:        true,
			expectedReadyConditionStatus: v1beta1.ConditionUnknown,
			expectedReadyConditionReason: errorOrphanMitigationFailedReason,
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
			finishedOrphanMitigation:     true,
			expectedReadyConditionStatus: v1beta1.ConditionFalse,
			expectedReadyConditionReason: successOrphanMitigationReason,
		},
		{
			name: "poll - gone",
			pollReaction: &fakeosb.PollLastOperationReaction{
				Error: osb.HTTPStatusCodeError{
					StatusCode: http.StatusGone,
				},
			},
			async: true,
			finishedOrphanMitigation:     true,
			expectedReadyConditionStatus: v1beta1.ConditionFalse,
			expectedReadyConditionReason: successOrphanMitigationReason,
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
			finishedOrphanMitigation:     false,
			expectedReadyConditionStatus: v1beta1.ConditionFalse,
			expectedReadyConditionReason: asyncDeprovisioningReason,
		},
		{
			name: "poll - failed",
			pollReaction: &fakeosb.PollLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State: osb.StateFailed,
				},
			},
			async: true,
			finishedOrphanMitigation:     true,
			retryDurationExceeded:        true,
			expectedReadyConditionStatus: v1beta1.ConditionUnknown,
			expectedReadyConditionReason: errorOrphanMitigationFailedReason,
		},
		{
			name: "poll - failed - retry duration exceeded",
			pollReaction: &fakeosb.PollLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State: osb.StateFailed,
				},
			},
			async: true,
			finishedOrphanMitigation:     false,
			expectedReadyConditionStatus: v1beta1.ConditionUnknown,
			expectedReadyConditionReason: errorOrphanMitigationFailedReason,
		},
		// TODO (mkibbe): poll - error
		// TODO (mkibbe): invalid state
		{
			name: "poll - error - retry duration exceeded",
			pollReaction: &fakeosb.PollLastOperationReaction{
				Error: fmt.Errorf("other error"),
			},
			async: true,
			finishedOrphanMitigation:     true,
			retryDurationExceeded:        true,
			expectedReadyConditionStatus: v1beta1.ConditionUnknown,
			expectedReadyConditionReason: errorOrphanMitigationFailedReason,
		},
		{
			name: "poll - in progress - retry duration exceeded",
			pollReaction: &fakeosb.PollLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State: osb.StateInProgress,
				},
			},
			async: true,
			finishedOrphanMitigation:     true,
			retryDurationExceeded:        true,
			expectedReadyConditionStatus: v1beta1.ConditionUnknown,
			expectedReadyConditionReason: errorOrphanMitigationFailedReason,
		},
		{
			name: "poll - invalid state - retry duration exceeded",
			pollReaction: &fakeosb.PollLastOperationReaction{
				Response: &osb.LastOperationResponse{
					State: "invalid state",
				},
			},
			async: true,
			finishedOrphanMitigation:     true,
			retryDurationExceeded:        true,
			expectedReadyConditionStatus: v1beta1.ConditionUnknown,
			expectedReadyConditionReason: errorOrphanMitigationFailedReason,
		},
	}

	for _, tc := range cases {
		_, fakeCatalogClient, _, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
			DeprovisionReaction:       tc.deprovReaction,
			PollLastOperationReaction: tc.pollReaction,
		})

		sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
		sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
		sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

		instance := getTestServiceInstanceWithRefs()
		instance.ObjectMeta.Finalizers = []string{v1beta1.FinalizerServiceCatalog}
		instance.Status.CurrentOperation = v1beta1.ServiceInstanceOperationProvision
		instance.Status.OrphanMitigationInProgress = true
		instance.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusRequired

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
		updatedServiceInstance, _ := updatedObject.(*v1beta1.ServiceInstance)

		if ok := testServiceInstanceOrphanMitigationInProgress(t, tc.name, errorf, updatedServiceInstance, !tc.finishedOrphanMitigation); !ok {
			continue
		}

		//TODO(mkibbe): change asserts to expects
		assertServiceInstanceReadyCondition(
			t,
			updatedServiceInstance,
			tc.expectedReadyConditionStatus,
			tc.expectedReadyConditionReason,
		)

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
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Response: &osb.ProvisionResponse{},
		},
	})

	paramSecret := &corev1.Secret{
		Data: map[string][]byte{
			"param-secret-key": []byte("{\"b\":\"2\"}"),
		},
	}
	addGetSecretReaction(fakeKubeClient, paramSecret)

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithRefs()

	parameters := map[string]interface{}{
		"a": "1",
	}
	b, err := json.Marshal(parameters)
	if err != nil {
		t.Fatalf("Failed to marshal parameters %v : %v", parameters, err)
	}
	instance.Spec.Parameters = &runtime.RawExtension{Raw: b}

	instance.Spec.ParametersFrom = []v1beta1.ParametersFromSource{
		{
			SecretKeyRef: &v1beta1.SecretKeyReference{
				Name: "param-secret-name",
				Key:  "param-secret-key",
			},
		},
	}

	if err = testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertProvision(t, brokerActions[0], &osb.ProvisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            testClusterServicePlanGUID,
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
	assertServiceInstanceOperationInProgressWithParameters(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, testClusterServicePlanName, testClusterServicePlanGUID, expectedParameters, expectedParametersChecksum, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceOperationSuccessWithParameters(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationProvision, testClusterServicePlanName, testClusterServicePlanGUID, expectedParameters, expectedParametersChecksum, instance)

	updateObject, ok := updatedServiceInstance.(*v1beta1.ServiceInstance)
	if !ok {
		t.Fatalf("couldn't convert to *v1beta1.ServiceInstance")
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

	expectedEvent := normalEventBuilder(successProvisionReason).msg("The instance was provisioned successfully")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
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

// TestResolveReferencesNoClusterServiceClass tests that resolveReferences fails
// with the expected failure case when no ClusterServiceClass exists
func TestResolveReferencesNoClusterServiceClass(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, _, testController, _ := newTestController(t, noFakeActions())

	instance := getTestServiceInstance()

	updatedInstance, err := testController.resolveReferences(instance)
	if err == nil {
		t.Fatalf("Should have failed with no service class")
	}

	if e, a := "a non-existent ClusterServiceClass", err.Error(); !strings.Contains(a, e) {
		t.Fatalf("Did not get the expected error message %q got %q", e, a)
	}
	if updatedInstance != nil {
		t.Fatalf("updatedInstance retuend was non-nil: %+v", updatedInstance)
	}

	// We should get the following actions:
	// list call for ClusterServiceClass
	// update service instance condition for failure
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.OneTermEqualSelector("spec.externalName", instance.Spec.ClusterServiceClassExternalName),
	}
	assertList(t, actions[0], &v1beta1.ClusterServiceClass{}, listRestrictions)

	updatedServiceInstance := assertUpdateStatus(t, actions[1], instance)

	updatedObject, ok := updatedServiceInstance.(*v1beta1.ServiceInstance)
	if !ok {
		t.Fatalf("couldn't convert to *v1beta1.ServiceInstance")
	}
	if updatedObject.Spec.ClusterServiceClassRef != nil {
		t.Fatalf("ClusterServiceClassRef was unexpectedly set: %+v", updatedObject)
	}
	if updatedObject.Spec.ClusterServicePlanRef != nil {
		t.Fatalf("ClusterServicePlanRef was unexpectedly set: %+v", updatedObject)
	}

	// verify no kube resources created
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorNonexistentClusterServiceClassReason).msg("References a non-existent ClusterServiceClass (ExternalName: \"test-serviceclass\") or there is more than one (found: 0)")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceUpdateParameters tests updating a
// ServiceInstance with new paramaters
func TestReconcileServiceInstanceUpdateParameters(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UpdateInstanceReaction: &fakeosb.UpdateInstanceReaction{
			Response: &osb.UpdateInstanceResponse{},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithRefs()
	instance.Generation = 2
	instance.Status.ReconciledGeneration = 1
	instance.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusRequired

	oldParameters := map[string]interface{}{
		"args": map[string]interface{}{
			"first":  "first-arg",
			"second": "second-arg",
		},
		"name": "test-param",
	}
	oldParametersMarshaled, err := MarshalRawParameters(oldParameters)
	if err != nil {
		t.Fatalf("Failed to marshal parameters: %v", err)
	}
	oldParametersRaw := &runtime.RawExtension{
		Raw: oldParametersMarshaled,
	}

	oldParametersChecksum, err := generateChecksumOfParameters(oldParameters)
	if err != nil {
		t.Fatalf("Failed to generate parameters checksum: %v", err)
	}

	instance.Status.ExternalProperties = &v1beta1.ServiceInstancePropertiesState{
		ClusterServicePlanExternalName: testClusterServicePlanName,
		ClusterServicePlanExternalID:   testClusterServicePlanGUID,
		Parameters:                     oldParametersRaw,
		ParametersChecksum:             oldParametersChecksum,
	}

	parameters := instanceParameters{Name: "test-param", Args: make(map[string]string)}
	parameters.Args["first"] = "first-arg"
	parameters.Args["second"] = "new-second-arg"

	b, err := json.Marshal(parameters)
	if err != nil {
		t.Fatalf("Failed to marshal parameters %v : %v", parameters, err)
	}
	instance.Spec.Parameters = &runtime.RawExtension{Raw: b}

	if err = testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertUpdateInstance(t, brokerActions[0], &osb.UpdateInstanceRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            nil, // no change to plan
		Context: map[string]interface{}{
			"platform":  "kubernetes",
			"namespace": "test-ns",
		},
		Parameters: map[string]interface{}{
			"args": map[string]interface{}{
				"first":  "first-arg",
				"second": "new-second-arg",
			},
			"name": "test-param",
		},
	})

	expectedParameters := map[string]interface{}{
		"args": map[string]interface{}{
			"first":  "first-arg",
			"second": "new-second-arg",
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
	assertServiceInstanceOperationInProgressWithParameters(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationUpdate, testClusterServicePlanName, testClusterServicePlanGUID, expectedParameters, expectedParametersChecksum, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceOperationSuccessWithParameters(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationUpdate, testClusterServicePlanName, testClusterServicePlanGUID, expectedParameters, expectedParametersChecksum, instance)

	updateObject, ok := updatedServiceInstance.(*v1beta1.ServiceInstance)
	if !ok {
		t.Fatalf("couldn't convert to *v1beta1.ServiceInstance")
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

	expectedEvent := normalEventBuilder(successUpdateInstanceReason).msg("The instance was updated successfully")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestResolveReferencesNoClusterServicePlan tests that resolveReferences fails
// with the expected failure case when no ClusterServicePlan exists
func TestResolveReferencesNoClusterServicePlan(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, _, testController, _ := newTestController(t, noFakeActions())

	instance := getTestServiceInstance()

	sc := getTestClusterServiceClass()
	var scItems []v1beta1.ClusterServiceClass
	scItems = append(scItems, *sc)
	fakeCatalogClient.AddReactor("list", "clusterserviceclasses", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1beta1.ClusterServiceClassList{Items: scItems}, nil
	})

	updatedInstance, err := testController.resolveReferences(instance)
	if err == nil {
		t.Fatalf("Should have failed with no service plan")
	}

	if e, a := "a non-existent ClusterServicePlan", err.Error(); !strings.Contains(a, e) {
		t.Fatalf("Did not get the expected error message %q got %q", e, a)
	}

	if updatedInstance != nil {
		t.Fatalf("updatedInstance retuend was non-nil: %+v", updatedInstance)
	}

	// We should get the following actions:
	// list call for ClusterServiceClass
	// list call for ClusterServicePlan
	// update service instance condition for failure
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 3)

	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.OneTermEqualSelector("spec.externalName", instance.Spec.ClusterServiceClassExternalName),
	}
	assertList(t, actions[0], &v1beta1.ClusterServiceClass{}, listRestrictions)

	listRestrictions = clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.ParseSelectorOrDie("spec.externalName=test-plan,spec.clusterServiceBrokerName=test-broker,spec.clusterServiceClassRef.name=SCGUID"),
	}
	assertList(t, actions[1], &v1beta1.ClusterServicePlan{}, listRestrictions)

	updatedServiceInstance := assertUpdateStatus(t, actions[2], instance)

	updatedObject, ok := updatedServiceInstance.(*v1beta1.ServiceInstance)
	if !ok {
		t.Fatalf("couldn't convert to *v1beta1.ServiceInstance")
	}
	if updatedObject.Spec.ClusterServiceClassRef == nil || updatedObject.Spec.ClusterServiceClassRef.Name != testClusterServiceClassGUID {
		t.Fatalf("ClusterServiceClassRef.Name was not set correctly, expected %q got: %+v", testClusterServiceClassGUID, updatedObject.Spec.ClusterServiceClassRef.Name)
	}
	if updatedObject.Spec.ClusterServicePlanRef != nil {
		t.Fatalf("ClusterServicePlanRef was unexpectedly set: %+v", updatedObject)
	}

	// verify no kube resources created
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorNonexistentClusterServicePlanReason).msgf(
		`References a non-existent ClusterServicePlan (K8S: %q ExternalName: %q) on ClusterServiceClass (K8S: %q ExternalName: %q) or there is more than one (found: %v)`,
		"", "test-plan", "SCGUID", "test-serviceclass", 0,
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceUpdatePlan tests updating a
// ServiceInstance with a new plan
func TestReconcileServiceInstanceUpdatePlan(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UpdateInstanceReaction: &fakeosb.UpdateInstanceReaction{
			Response: &osb.UpdateInstanceResponse{},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithRefs()
	instance.Generation = 2
	instance.Status.ReconciledGeneration = 1
	instance.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusRequired

	oldParameters := map[string]interface{}{
		"args": map[string]interface{}{
			"first":  "first-arg",
			"second": "second-arg",
		},
		"name": "test-param",
	}
	oldParametersMarshaled, err := MarshalRawParameters(oldParameters)
	if err != nil {
		t.Fatalf("Failed to marshal parameters: %v", err)
	}
	oldParametersRaw := &runtime.RawExtension{
		Raw: oldParametersMarshaled,
	}

	oldParametersChecksum, err := generateChecksumOfParameters(oldParameters)
	if err != nil {
		t.Fatalf("Failed to generate parameters checksum: %v", err)
	}

	instance.Status.ExternalProperties = &v1beta1.ServiceInstancePropertiesState{
		ClusterServicePlanExternalName: "old-plan-name",
		ClusterServicePlanExternalID:   "old-plan-id",
		Parameters:                     oldParametersRaw,
		ParametersChecksum:             oldParametersChecksum,
	}

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

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	expectedPlanID := testClusterServicePlanGUID
	assertUpdateInstance(t, brokerActions[0], &osb.UpdateInstanceRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            &expectedPlanID,
		Context: map[string]interface{}{
			"platform":  "kubernetes",
			"namespace": "test-ns",
		},
		Parameters: nil, // no change to parameters
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationInProgressWithParameters(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationUpdate, testClusterServicePlanName, testClusterServicePlanGUID, oldParameters, oldParametersChecksum, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceOperationSuccessWithParameters(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationUpdate, testClusterServicePlanName, testClusterServicePlanGUID, oldParameters, oldParametersChecksum, instance)

	updateObject, ok := updatedServiceInstance.(*v1beta1.ServiceInstance)
	if !ok {
		t.Fatalf("couldn't convert to *v1beta1.ServiceInstance")
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

	expectedEvent := normalEventBuilder(successUpdateInstanceReason).msg("The instance was updated successfully")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceWithUpdateCallFailure tests that when the update
// call to the broker fails, the ready condition becomes false, and the
// failure condition is not set.
func TestReconcileServiceInstanceWithUpdateCallFailure(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UpdateInstanceReaction: &fakeosb.UpdateInstanceReaction{
			Error: errors.New("fake update failure"),
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceUpdatingPlan()

	if err := testController.reconcileServiceInstance(instance); err == nil {
		t.Fatalf("Should not be able to make the ServiceInstance.")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	expectedPlanID := testClusterServicePlanGUID
	assertUpdateInstance(t, brokerActions[0], &osb.UpdateInstanceRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            &expectedPlanID,
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
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationUpdate, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceRequestRetriableError(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationUpdate, errorErrorCallingUpdateInstanceReason, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorErrorCallingUpdateInstanceReason).msgf(
		"Error communicating with broker for %q:",
		"updating",
	).msg("fake update failure")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceWithUpdateFailure tests that when the provision
// call to the broker fails with an HTTP error, the ready condition becomes
// false, and the failure condition is set.
func TestReconcileServiceInstanceWithUpdateFailure(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UpdateInstanceReaction: &fakeosb.UpdateInstanceReaction{
			Error: osb.HTTPStatusCodeError{
				StatusCode:   http.StatusConflict,
				ErrorMessage: strPtr("OutOfQuota"),
				Description:  strPtr("You're out of quota!"),
			},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceUpdatingPlan()

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	expectedPlanID := testClusterServicePlanGUID
	assertUpdateInstance(t, brokerActions[0], &osb.UpdateInstanceRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            &expectedPlanID,
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
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationUpdate, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceRequestFailingErrorNoOrphanMitigation(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationUpdate, errorUpdateInstanceCallFailedReason, "ClusterServiceBrokerReturnedFailure", instance)

	events := getRecordedEvents(testController)

	expectedEvent := warningEventBuilder(errorUpdateInstanceCallFailedReason).msgf(
		`Error updating ServiceInstance of ClusterServiceClass (K8S: %q ExternalName: %q) at ClusterServiceBroker %q: Status: %v; ErrorMessage: %s`,
		"SCGUID", "test-serviceclass", "test-broker", 409, `OutOfQuota; Description: You're out of quota!; ResponseError: <nil>`,
	)
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestResolveReferences tests that resolveReferences works
// correctly and resolves references.
func TestResolveReferencesWorks(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, _, testController, _ := newTestController(t, noFakeActions())

	instance := getTestServiceInstance()

	sc := getTestClusterServiceClass()
	var scItems []v1beta1.ClusterServiceClass
	scItems = append(scItems, *sc)
	fakeCatalogClient.AddReactor("list", "clusterserviceclasses", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1beta1.ClusterServiceClassList{Items: scItems}, nil
	})
	sp := getTestClusterServicePlan()
	var spItems []v1beta1.ClusterServicePlan
	spItems = append(spItems, *sp)
	fakeCatalogClient.AddReactor("list", "clusterserviceplans", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1beta1.ClusterServicePlanList{Items: spItems}, nil
	})

	updatedInstance, err := testController.resolveReferences(instance)
	if err != nil {
		t.Fatalf("Should not have failed, but failed with: %q", err)
	}

	if updatedInstance.Spec.ClusterServiceClassRef == nil || updatedInstance.Spec.ClusterServiceClassRef.Name != testClusterServiceClassGUID {
		t.Fatalf("Did not find expected ClusterServiceClassRef, expected %q got %+v", testClusterServiceClassGUID, updatedInstance.Spec.ClusterServiceClassRef)
	}

	if updatedInstance.Spec.ClusterServicePlanRef == nil || updatedInstance.Spec.ClusterServicePlanRef.Name != testClusterServicePlanGUID {
		t.Fatalf("Did not find expected ClusterServicePlanRef, expected %q got %+v", testClusterServicePlanGUID, updatedInstance.Spec.ClusterServicePlanRef.Name)
	}

	// We should get the following actions:
	// list call for ClusterServiceClass
	// list call for ClusterServicePlan
	// updating references
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 3)

	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.OneTermEqualSelector("spec.externalName", instance.Spec.ClusterServiceClassExternalName),
	}
	assertList(t, actions[0], &v1beta1.ClusterServiceClass{}, listRestrictions)

	listRestrictions = clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.ParseSelectorOrDie("spec.externalName=test-plan,spec.clusterServiceBrokerName=test-broker,spec.clusterServiceClassRef.name=SCGUID"),
	}
	assertList(t, actions[1], &v1beta1.ClusterServicePlan{}, listRestrictions)

	updatedServiceInstance := assertUpdateReference(t, actions[2], instance)
	updateObject, ok := updatedServiceInstance.(*v1beta1.ServiceInstance)
	if !ok {
		t.Fatalf("couldn't convert to *v1beta1.ServiceInstance")
	}
	if updateObject.Spec.ClusterServiceClassRef == nil || updateObject.Spec.ClusterServiceClassRef.Name != testClusterServiceClassGUID {
		t.Fatalf("ClusterServiceClassRef was not resolved correctly during reconcile")
	}
	if updateObject.Spec.ClusterServicePlanRef == nil || updateObject.Spec.ClusterServicePlanRef.Name != testClusterServicePlanGUID {
		t.Fatalf("ClusterServicePlanRef was not resolved correctly during reconcile")
	}

	// verify no kube resources created
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 0)
}

// TestResolveReferencesForPlanChange tests that resolveReferences updates the
// ClusterServicePlanRef when the plan is changed.
func TestResolveReferencesForPlanChange(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, _, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())

	instance := getTestServiceInstanceWithRefs()

	newPlanID := "new-plan-id"
	newPlanName := "new-plan-name"

	sp := &v1beta1.ClusterServicePlan{
		ObjectMeta: metav1.ObjectMeta{Name: newPlanID},
		Spec: v1beta1.ClusterServicePlanSpec{
			ExternalID:   newPlanID,
			ExternalName: newPlanName,
			Bindable:     truePtr(),
		},
	}
	var spItems []v1beta1.ClusterServicePlan
	spItems = append(spItems, *sp)
	fakeCatalogClient.AddReactor("list", "clusterserviceplans", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1beta1.ClusterServicePlanList{Items: spItems}, nil
	})

	instance.Spec.ClusterServicePlanExternalName = newPlanName
	instance.Spec.ClusterServicePlanRef = nil

	updatedInstance, err := testController.resolveReferences(instance)
	if err != nil {
		t.Fatalf("Should not have failed, but failed with: %q", err)
	}

	if updatedInstance.Spec.ClusterServiceClassRef == nil || updatedInstance.Spec.ClusterServiceClassRef.Name != testClusterServiceClassGUID {
		t.Fatalf("Did not find expected ClusterServiceClassRef, expected %q got %+v", testClusterServiceClassGUID, updatedInstance.Spec.ClusterServiceClassRef)
	}

	if updatedInstance.Spec.ClusterServicePlanRef == nil || updatedInstance.Spec.ClusterServicePlanRef.Name != newPlanID {
		t.Fatalf("Did not find expected ClusterServicePlanRef, expected %q got %+v", newPlanID, updatedInstance.Spec.ClusterServicePlanRef.Name)
	}

	// We should get the following actions:
	// list call for ClusterServicePlan
	// updating references
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	listRestrictions := clientgotesting.ListRestrictions{
		Labels: labels.Everything(),
		Fields: fields.ParseSelectorOrDie("spec.externalName=new-plan-name,spec.clusterServiceBrokerName=test-broker,spec.clusterServiceClassRef.name=SCGUID"),
	}
	assertList(t, actions[0], &v1beta1.ClusterServicePlan{}, listRestrictions)

	updatedServiceInstance := assertUpdateReference(t, actions[1], instance)
	updateObject, ok := updatedServiceInstance.(*v1beta1.ServiceInstance)
	if !ok {
		t.Fatalf("couldn't convert to *v1beta1.ServiceInstance")
	}
	if updateObject.Spec.ClusterServiceClassRef == nil || updateObject.Spec.ClusterServiceClassRef.Name != testClusterServiceClassGUID {
		t.Fatalf("ClusterServiceClassRef was not resolved correctly during reconcile")
	}
	if updateObject.Spec.ClusterServicePlanRef == nil || updateObject.Spec.ClusterServicePlanRef.Name != newPlanID {
		t.Fatalf("ClusterServicePlanRef was not resolved correctly during reconcile")
	}

	// verify no kube resources created
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 0)
}

// TestResolveReferences tests that resolveReferences works
// correctly and resolves references.
func TestResolveReferencesWorksK8SNames(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, _, testController, sharedInformers := newTestController(t, noFakeActions())

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceK8SNames()

	updatedInstance, err := testController.resolveReferences(instance)
	if err != nil {
		t.Fatalf("Should not have failed, but failed with: %q", err)
	}

	if updatedInstance.Spec.ClusterServiceClassRef == nil || updatedInstance.Spec.ClusterServiceClassRef.Name != testClusterServiceClassGUID {
		t.Fatalf("Did not find expected ClusterServiceClassRef, expected %q got %+v", testClusterServiceClassGUID, updatedInstance.Spec.ClusterServiceClassRef)
	}

	if updatedInstance.Spec.ClusterServicePlanRef == nil || updatedInstance.Spec.ClusterServicePlanRef.Name != testClusterServicePlanGUID {
		t.Fatalf("Did not find expected ClusterServicePlanRef, expected %q got %+v", testClusterServicePlanGUID, updatedInstance.Spec.ClusterServicePlanRef.Name)
	}

	// We should get the following actions:
	// updating references
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateReference(t, actions[0], instance)
	updateObject, ok := updatedServiceInstance.(*v1beta1.ServiceInstance)
	if !ok {
		t.Fatalf("couldn't convert to *v1beta1.ServiceInstance")
	}
	if updateObject.Spec.ClusterServiceClassRef == nil || updateObject.Spec.ClusterServiceClassRef.Name != testClusterServiceClassGUID {
		t.Fatalf("ClusterServiceClassRef was not resolved correctly during reconcile")
	}
	if updateObject.Spec.ClusterServicePlanRef == nil || updateObject.Spec.ClusterServicePlanRef.Name != testClusterServicePlanGUID {
		t.Fatalf("ClusterServicePlanRef was not resolved correctly during reconcile")
	}

	// verify no kube resources created
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 0)
}

// TestReconcileServiceInstanceUpdateAsynchronous tests updating a ServiceInstance
// when the request results in an async response. Resulting status will indicate
// not ready and polling in progress.
func TestReconcileServiceInstanceUpdateAsynchronous(t *testing.T) {
	key := osb.OperationKey(testOperation)
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		UpdateInstanceReaction: &fakeosb.UpdateInstanceReaction{
			Response: &osb.UpdateInstanceResponse{
				Async:        true,
				OperationKey: &key,
			},
		},
	})

	addGetNamespaceReaction(fakeKubeClient)

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithRefs()
	instance.Generation = 2
	instance.Status.ReconciledGeneration = 1
	instance.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusRequired

	instance.Status.ExternalProperties = &v1beta1.ServiceInstancePropertiesState{
		ClusterServicePlanExternalName: "old-plan-name",
		ClusterServicePlanExternalID:   "old-plan-id",
	}

	instanceKey := testNamespace + "/" + testServiceInstanceName
	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	if err := testController.reconcileServiceInstance(instance); err != nil {
		t.Fatalf("This should not fail : %v", err)
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	expectedPlanID := testClusterServicePlanGUID
	assertUpdateInstance(t, brokerActions[0], &osb.UpdateInstanceRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            &expectedPlanID,
		Context: map[string]interface{}{
			"platform":  "kubernetes",
			"namespace": "test-ns",
		},
	})

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationUpdate, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceAsyncInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationUpdate, testOperation, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	// verify no kube resources created.
	// One single action comes from getting namespace uid
	kubeActions := fakeKubeClient.Actions()
	if e, a := 1, len(kubeActions); e != a {
		t.Fatalf("Unexpected number of actions: expected %v, got %v", e, a)
	}

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 1 {
		t.Fatalf("Expected polling queue to have a record of seeing test instance once")
	}
}

// TestPollServiceInstanceAsyncInProgressUpdating tests polling an instance that
// is already in process of updating (background/asynchronously) and is still in
// progress (should be re-polled)
func TestPollServiceInstanceAsyncInProgressUpdating(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State:       osb.StateInProgress,
				Description: strPtr(lastOperationDescription),
			},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceAsyncUpdating(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.pollServiceInstance(instance)
	if err != nil {
		t.Fatalf("pollServiceInstance failed: %s", err)
	}

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 1 {
		t.Fatalf("Expected polling queue to have record of seeing test instance once")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   testServiceInstanceGUID,
		ServiceID:    strPtr(testClusterServiceClassGUID),
		PlanID:       strPtr(testClusterServicePlanGUID),
		OperationKey: &operationKey,
	})

	// there should have been 1 action to update the status with the last operation description
	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceAsyncInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationUpdate, testOperation, testClusterServicePlanName, testClusterServicePlanGUID, instance)
	assertServiceInstanceConditionHasLastOperationDescription(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationUpdate, lastOperationDescription)

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)
}

// TestPollServiceInstanceAsyncSuccessUpdating tests polling an instance that is
// already in process of updating (background/ asynchronously) and is found to be
// ready
func TestPollServiceInstanceAsyncSuccessUpdating(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State:       osb.StateSucceeded,
				Description: strPtr(lastOperationDescription),
			},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceAsyncUpdating(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.pollServiceInstance(instance)
	if err != nil {
		t.Fatalf("pollServiceInstance failed: %s", err)
	}

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance as polling should have completed")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   testServiceInstanceGUID,
		ServiceID:    strPtr(testClusterServiceClassGUID),
		PlanID:       strPtr(testClusterServicePlanGUID),
		OperationKey: &operationKey,
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationUpdate, testClusterServicePlanName, testClusterServicePlanGUID, instance)
}

// TestPollServiceInstanceAsyncFailureUpdating tests polling an instance where
// update was in process asynchronously but has an updated status of failed to
// update.
func TestPollServiceInstanceAsyncFailureUpdating(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State: osb.StateFailed,
			},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceAsyncUpdating(testOperation)
	instanceKey := testNamespace + "/" + testServiceInstanceName

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance")
	}

	err := testController.pollServiceInstance(instance)
	if err != nil {
		t.Fatalf("pollServiceInstance failed: %s", err)
	}

	if testController.instancePollingQueue.NumRequeues(instanceKey) != 0 {
		t.Fatalf("Expected polling queue to not have any record of test instance as polling should have completed")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	operationKey := osb.OperationKey(testOperation)
	assertPollLastOperation(t, brokerActions[0], &osb.LastOperationRequest{
		InstanceID:   testServiceInstanceGUID,
		ServiceID:    strPtr(testClusterServiceClassGUID),
		PlanID:       strPtr(testClusterServicePlanGUID),
		OperationKey: &operationKey,
	})

	// verify no kube resources created.
	// No actions
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 1)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceRequestFailingErrorNoOrphanMitigation(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationUpdate, errorUpdateInstanceCallFailedReason, errorUpdateInstanceCallFailedReason, instance)
}

func TestCheckClassAndPlanForDeletion(t *testing.T) {
	cases := []struct {
		name           string
		instance       *v1beta1.ServiceInstance
		class          *v1beta1.ClusterServiceClass
		plan           *v1beta1.ClusterServicePlan
		success        bool
		expectedReason string
		expectedErrors []string
	}{
		{
			name:     "non-deleted plan and class works",
			instance: getTestServiceInstance(),
			class:    getTestClusterServiceClass(),
			plan:     getTestClusterServicePlan(),
			success:  true,
		},
		{
			name:           "deleted plan fails",
			instance:       getTestServiceInstance(),
			class:          getTestClusterServiceClass(),
			plan:           getTestMarkedAsRemovedClusterServicePlan(),
			success:        false,
			expectedReason: errorDeletedClusterServicePlanReason,
			expectedErrors: []string{"Service Plan", "has been deleted"},
		},
		{
			name:           "deleted class fails",
			instance:       getTestServiceInstance(),
			class:          getTestMarkedAsRemovedClusterServiceClass(),
			plan:           getTestClusterServicePlan(),
			success:        false,
			expectedReason: errorDeletedClusterServiceClassReason,
			expectedErrors: []string{"Service Class", "has been deleted"},
		},
		{
			name:           "deleted plan and class fails",
			instance:       getTestServiceInstance(),
			class:          getTestClusterServiceClass(),
			plan:           getTestMarkedAsRemovedClusterServicePlan(),
			success:        false,
			expectedReason: errorDeletedClusterServicePlanReason,
			expectedErrors: []string{"Service Plan", "has been deleted"},
		},
		{
			name:           "Updating plan fails",
			instance:       getTestServiceInstanceUpdatingPlan(),
			class:          getTestClusterServiceClass(),
			plan:           getTestMarkedAsRemovedClusterServicePlan(),
			success:        false,
			expectedReason: errorDeletedClusterServicePlanReason,
			expectedErrors: []string{"Service Plan", "has been deleted"},
		},
		{
			name:     "Updating parameters works",
			instance: getTestServiceInstanceUpdatingParametersOfDeletedPlan(),
			class:    getTestClusterServiceClass(),
			plan:     getTestMarkedAsRemovedClusterServicePlan(),
			success:  true,
		},
	}

	for _, tc := range cases {
		fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, _ := newTestController(t, noFakeActions())

		err := testController.checkForRemovedClassAndPlan(tc.instance, tc.class, tc.plan)
		if err != nil {
			if tc.success {
				t.Errorf("%q: Unexpected error %v", tc.name, err)
			}
			for _, exp := range tc.expectedErrors {
				if e, a := exp, err.Error(); !strings.Contains(a, e) {
					t.Errorf("%q: Did not find expected error %q : got %q", tc.name, e, a)
				}
			}
		} else if !tc.success {
			t.Errorf("%q: Did not get a failure when expected one", tc.name)
		}

		// no kube or broker actions ever
		assertNumberOfActions(t, fakeKubeClient.Actions(), 0)
		brokerActions := fakeClusterServiceBrokerClient.Actions()
		assertNumberOfClusterServiceBrokerActions(t, brokerActions, 0)

		// If things succeeded, make sure no actions on the catalog client
		// and if things fail, make sure instance status is updated and
		// an event is generated
		actions := fakeCatalogClient.Actions()
		if tc.success {
			assertNumberOfActions(t, actions, 0)
		} else {
			assertNumberOfActions(t, actions, 1)
			assertUpdateStatus(t, actions[0], tc.instance)
			assertServiceInstanceReadyFalse(t, tc.instance, tc.expectedReason)
			events := getRecordedEvents(testController)
			assertNumEvents(t, events, 1)
		}
	}
}

// TestReconcileServiceInstanceDeleteDuringOngoingOperation tests deleting an
// instance that has an on-going operation.
func TestReconcileServiceInstanceDeleteDuringOngoingOperation(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		DeprovisionReaction: &fakeosb.DeprovisionReaction{
			Response: &osb.DeprovisionResponse{},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithRefs()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{v1beta1.FinalizerServiceCatalog}
	instance.Status.CurrentOperation = v1beta1.ServiceInstanceOperationProvision
	startTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))
	instance.Status.OperationStartTime = &startTime

	fakeCatalogClient.AddReactor("get", "serviceinstances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, instance, nil
	})

	timeOfReconciliation := metav1.Now()

	err := testController.reconcileServiceInstance(instance)
	if err != nil {
		t.Fatalf("This should not fail")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertDeprovision(t, brokerActions[0], &osb.DeprovisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            testClusterServicePlanGUID,
	})

	// Verify no core kube actions occurred
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance).(*v1beta1.ServiceInstance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationDeprovision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	// Verify that the operation start time was reset to Now
	if updatedServiceInstance.Status.OperationStartTime.Before(&timeOfReconciliation) {
		t.Fatalf(
			"OperationStartTime should not be before the time that the reconciliation started. OperationStartTime=%v. timeOfReconciliation=%v",
			updatedServiceInstance.Status.OperationStartTime,
			timeOfReconciliation,
		)
	}

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance).(*v1beta1.ServiceInstance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationDeprovision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	events := getRecordedEvents(testController)

	expectedEvent := normalEventBuilder(successDeprovisionReason).msg("The instance was deprovisioned successfully")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

// TestReconcileServiceInstanceDeleteDuringOrphanMitigation tests deleting an
// instance that is undergoing orphan mitigation.
func TestReconcileServiceInstanceDeleteWithOngoingOperation(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		DeprovisionReaction: &fakeosb.DeprovisionReaction{
			Response: &osb.DeprovisionResponse{},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())
	sharedInformers.ClusterServicePlans().Informer().GetStore().Add(getTestClusterServicePlan())

	instance := getTestServiceInstanceWithRefs()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{v1beta1.FinalizerServiceCatalog}
	instance.Status.CurrentOperation = v1beta1.ServiceInstanceOperationProvision
	startTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))
	instance.Status.OperationStartTime = &startTime
	instance.Status.OrphanMitigationInProgress = true

	fakeCatalogClient.AddReactor("get", "serviceinstances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, instance, nil
	})

	timeOfReconciliation := metav1.Now()

	err := testController.reconcileServiceInstance(instance)
	if err != nil {
		t.Fatalf("This should not fail")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertDeprovision(t, brokerActions[0], &osb.DeprovisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            testClusterServicePlanGUID,
	})

	// Verify no core kube actions occurred
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance).(*v1beta1.ServiceInstance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationDeprovision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	// Verify that the operation start time was reset to Now
	if updatedServiceInstance.Status.OperationStartTime.Before(&timeOfReconciliation) {
		t.Fatalf(
			"OperationStartTime should not be before the time that the reconciliation started. OperationStartTime=%v. timeOfReconciliation=%v",
			updatedServiceInstance.Status.OperationStartTime,
			timeOfReconciliation,
		)
	}

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance).(*v1beta1.ServiceInstance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationDeprovision, testClusterServicePlanName, testClusterServicePlanGUID, instance)

	events := getRecordedEvents(testController)
	assertNumEvents(t, events, 1)

	expectedEvent := corev1.EventTypeNormal + " " + successDeprovisionReason + " " + "The instance was deprovisioned successfully"
	if e, a := expectedEvent, events[0]; e != a {
		t.Fatalf("Received unexpected event: %v\nExpected: %v", a, e)
	}
}

// TestReconcileServiceInstanceDeleteWithNonExistentPlan tests deprovisioning
// an instance that has been updated to a non-existent plan.
func TestReconcileServiceInstanceDeleteWithNonExistentPlan(t *testing.T) {
	fakeKubeClient, fakeCatalogClient, fakeClusterServiceBrokerClient, testController, sharedInformers := newTestController(t, fakeosb.FakeClientConfiguration{
		DeprovisionReaction: &fakeosb.DeprovisionReaction{
			Response: &osb.DeprovisionResponse{},
		},
	})

	sharedInformers.ClusterServiceBrokers().Informer().GetStore().Add(getTestClusterServiceBroker())
	sharedInformers.ClusterServiceClasses().Informer().GetStore().Add(getTestClusterServiceClass())

	instance := getTestServiceInstanceWithRefs()
	instance.ObjectMeta.DeletionTimestamp = &metav1.Time{}
	instance.ObjectMeta.Finalizers = []string{v1beta1.FinalizerServiceCatalog}
	// we only invoke the broker client to deprovision if we have a reconciled generation set
	// as that implies a previous success.
	instance.Generation = 2
	instance.Status.ReconciledGeneration = 1
	instance.Status.ExternalProperties = &v1beta1.ServiceInstancePropertiesState{
		ClusterServicePlanExternalName: "old-plan-name",
		ClusterServicePlanExternalID:   "old-plan-id",
	}
	instance.Status.DeprovisionStatus = v1beta1.ServiceInstanceDeprovisionStatusRequired
	instance.Spec.ClusterServicePlanRef = nil

	fakeCatalogClient.AddReactor("get", "serviceinstances", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, instance, nil
	})

	err := testController.reconcileServiceInstance(instance)
	if err != nil {
		t.Fatalf("This should not fail")
	}

	brokerActions := fakeClusterServiceBrokerClient.Actions()
	assertNumberOfClusterServiceBrokerActions(t, brokerActions, 1)
	assertDeprovision(t, brokerActions[0], &osb.DeprovisionRequest{
		AcceptsIncomplete: true,
		InstanceID:        testServiceInstanceGUID,
		ServiceID:         testClusterServiceClassGUID,
		PlanID:            "old-plan-id",
	})

	// Verify no core kube actions occurred
	kubeActions := fakeKubeClient.Actions()
	assertNumberOfActions(t, kubeActions, 0)

	actions := fakeCatalogClient.Actions()
	assertNumberOfActions(t, actions, 2)

	updatedServiceInstance := assertUpdateStatus(t, actions[0], instance)
	assertServiceInstanceOperationInProgress(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationDeprovision, "old-plan-name", "old-plan-id", instance)

	updatedServiceInstance = assertUpdateStatus(t, actions[1], instance)
	assertServiceInstanceOperationSuccess(t, updatedServiceInstance, v1beta1.ServiceInstanceOperationDeprovision, "old-plan-name", "old-plan-id", instance)

	events := getRecordedEvents(testController)

	expectedEvent := normalEventBuilder(successDeprovisionReason).msg("The instance was deprovisioned successfully")
	if err := checkEvents(events, expectedEvent.stringArr()); err != nil {
		t.Fatal(err)
	}
}

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

package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/kubernetes/fake"
	corev1 "k8s.io/api/core/v1"
	restclient "k8s.io/client-go/rest"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"

	// avoid error `servicecatalog/v1beta1 is not enabled`
	_ "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/install"

	osb "github.com/pmorie/go-open-service-broker-client/v2"
	fakeosb "github.com/pmorie/go-open-service-broker-client/v2/fake"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset"
	scinformers "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/externalversions"
	informers "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/externalversions/servicecatalog/v1beta1"
	"github.com/kubernetes-incubator/service-catalog/pkg/controller"
	scfeatures "github.com/kubernetes-incubator/service-catalog/pkg/features"
	"github.com/kubernetes-incubator/service-catalog/pkg/registry/servicecatalog/server"
	"github.com/kubernetes-incubator/service-catalog/test/util"
)

const (
	testNamespace               = "test-namespace"
	testBrokerName              = "test-broker"
	testClusterServiceClassName = "test-service"
	testClusterServiceClassID   = "12345"
	testPlanName                = "test-plan"
	testPlanExternalID          = "34567"
	testInstanceName            = "test-instance"
	testBindingName             = "test-binding"
	testSecretName              = "test-secret"
	testBrokerURL               = "https://example.com"
	testExternalID              = "9737b6ed-ca95-4439-8219-c53fcad118ab"
	testDashboardURL            = "http://test-dashboard.example.com"
	testCreatorUsername         = "create-username"
	testUpdaterUsername         = "update-username"
	testDeleterUsername         = "delete-username"
)

func truePtr() *bool {
	b := true
	return &b
}

// TestBasicFlowsSync tests:
//
// - add Broker
// - verify ClusterServiceClasses added
// - provision Instance
// - update Instance
// - make Binding
// - unbind
// - deprovision
// - delete broker
//
// ...using purely synchronous provision/deprovision.
func TestBasicFlowsSync(t *testing.T) {
	_, catalogClient, _, _, _, _, shutdownServer, shutdownController := newTestController(t, fakeosb.FakeClientConfiguration{
		CatalogReaction: &fakeosb.CatalogReaction{
			Response: getTestCatalogResponse(),
		},
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Response: &osb.ProvisionResponse{
				Async: false,
			},
		},
		UpdateInstanceReaction: &fakeosb.UpdateInstanceReaction{
			Response: &osb.UpdateInstanceResponse{
				Async: false,
			},
		},
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: map[string]interface{}{
					"foo": "bar",
					"baz": "zap",
				},
			},
		},
		UnbindReaction: &fakeosb.UnbindReaction{},
		DeprovisionReaction: &fakeosb.DeprovisionReaction{
			Response: &osb.DeprovisionResponse{
				Async: false,
			},
		},
	})
	defer shutdownController()
	defer shutdownServer()

	client := catalogClient.ServicecatalogV1beta1()

	broker := &v1beta1.ClusterServiceBroker{
		ObjectMeta: metav1.ObjectMeta{Name: testBrokerName},
		Spec: v1beta1.ClusterServiceBrokerSpec{
			URL: testBrokerURL,
		},
	}

	_, err := client.ClusterServiceBrokers().Create(broker)
	if nil != err {
		t.Fatalf("error creating the broker %q (%q)", broker.Name, err)
	}

	err = util.WaitForBrokerCondition(client,
		testBrokerName,
		v1beta1.ServiceBrokerCondition{
			Type:   v1beta1.ServiceBrokerConditionReady,
			Status: v1beta1.ConditionTrue,
		})
	if err != nil {
		t.Fatalf("error waiting for broker to become ready: %v", err)
	}

	err = util.WaitForClusterServiceClassToExist(client, testClusterServiceClassID)
	if nil != err {
		t.Fatalf("error waiting from ClusterServiceClass to exist: %v", err)
	}

	// TODO: find some way to compose scenarios; extract method here for real
	// logic for this test.

	//-----------------

	instance := &v1beta1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: testInstanceName},
		Spec: v1beta1.ServiceInstanceSpec{
			PlanReference: v1beta1.PlanReference{
				ExternalClusterServiceClassName: testClusterServiceClassName,
				ExternalClusterServicePlanName:  testPlanName,
			},
			ExternalID: testExternalID,
		},
	}

	if _, err := client.ServiceInstances(testNamespace).Create(instance); err != nil {
		t.Fatalf("error creating Instance: %v", err)
	}

	if err := util.WaitForInstanceCondition(client, testNamespace, testInstanceName, v1beta1.ServiceInstanceCondition{
		Type:   v1beta1.ServiceInstanceConditionReady,
		Status: v1beta1.ConditionTrue,
	}); err != nil {
		t.Fatalf("error waiting for instance to become ready: %v", err)
	}

	retInst, err := client.ServiceInstances(instance.Namespace).Get(instance.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting instance %s/%s back", instance.Namespace, instance.Name)
	}
	if retInst.Spec.ExternalID != instance.Spec.ExternalID {
		t.Fatalf(
			"returned OSB GUID '%s' doesn't match original '%s'",
			retInst.Spec.ExternalID,
			instance.Spec.ExternalID,
		)
	}

	// Update Instance
	updateRequests := retInst.Spec.UpdateRequests + 1
	retInst.Spec.UpdateRequests = updateRequests
	if _, err := client.ServiceInstances(testNamespace).Update(retInst); err != nil {
		t.Fatalf("error updating Instance: %v", err)
	}

	if err := util.WaitForInstanceReconciledGeneration(client, testNamespace, testInstanceName, retInst.Status.ReconciledGeneration+1); err != nil {
		t.Fatalf("error waiting for instance to reconcile: %v", err)
	}

	retInst, err = client.ServiceInstances(instance.Namespace).Get(instance.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting instance %s/%s back", instance.Namespace, instance.Name)
	}
	if e, a := updateRequests, retInst.Spec.UpdateRequests; e != a {
		t.Fatalf("unexpected updateRequets in instance spec: expected %v, got %v", e, a)
	}

	// Binding test begins here
	//-----------------

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: testBindingName},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: corev1.LocalObjectReference{
				Name: testInstanceName,
			},
		},
	}

	_, err = client.ServiceBindings(testNamespace).Create(binding)
	if err != nil {
		t.Fatalf("error creating Binding: %v", binding)
	}

	err = util.WaitForBindingCondition(client, testNamespace, testBindingName, v1beta1.ServiceBindingCondition{
		Type:   v1beta1.ServiceBindingConditionReady,
		Status: v1beta1.ConditionTrue,
	})
	if err != nil {
		t.Fatalf("error waiting for binding to become ready: %v", err)
	}

	err = client.ServiceBindings(testNamespace).Delete(testBindingName, &metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("binding delete should have been accepted: %v", err)
	}

	err = util.WaitForBindingToNotExist(client, testNamespace, testBindingName)
	if err != nil {
		t.Fatalf("error waiting for binding to not exist: %v", err)
	}

	//-----------------
	// End binding test

	err = client.ServiceInstances(testNamespace).Delete(testInstanceName, &metav1.DeleteOptions{})
	if nil != err {
		t.Fatalf("instance delete should have been accepted: %v", err)
	}

	err = util.WaitForInstanceToNotExist(client, testNamespace, testInstanceName)
	if err != nil {
		t.Fatalf("error waiting for instance to be deleted: %v", err)
	}

	//-----------------
	// End provision test

	// Delete the broker
	err = client.ClusterServiceBrokers().Delete(testBrokerName, &metav1.DeleteOptions{})
	if nil != err {
		t.Fatalf("broker should be deleted (%s)", err)
	}

	err = util.WaitForClusterServiceClassToNotExist(client, testClusterServiceClassName)
	if err != nil {
		t.Fatalf("error waiting for ClusterServiceClass to not exist: %v", err)
	}

	err = util.WaitForBrokerToNotExist(client, testBrokerName)
	if err != nil {
		t.Fatalf("error waiting for Broker to not exist: %v", err)
	}
}

// TestBasicFlowsAsync tests the same flows as TestBasicFlowsSync, using
// asynchronous provision/deprovision.
func TestBasicFlowsAsync(t *testing.T) {
	_, catalogClient, _, _, _, _, shutdownServer, shutdownController := newTestController(t, fakeosb.FakeClientConfiguration{
		CatalogReaction: &fakeosb.CatalogReaction{
			Response: getTestCatalogResponse(),
		},
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Response: &osb.ProvisionResponse{
				Async: true,
			},
		},
		PollLastOperationReaction: &fakeosb.PollLastOperationReaction{
			Response: &osb.LastOperationResponse{
				State: osb.StateSucceeded,
			},
		},
		UpdateInstanceReaction: &fakeosb.UpdateInstanceReaction{
			Response: &osb.UpdateInstanceResponse{
				Async: true,
			},
		},
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: map[string]interface{}{
					"foo": "bar",
					"baz": "zap",
				},
			},
		},
		UnbindReaction: &fakeosb.UnbindReaction{},
		DeprovisionReaction: &fakeosb.DeprovisionReaction{
			Response: &osb.DeprovisionResponse{
				Async: true,
			},
		},
	})
	defer shutdownController()
	defer shutdownServer()

	client := catalogClient.ServicecatalogV1beta1()

	broker := &v1beta1.ClusterServiceBroker{
		ObjectMeta: metav1.ObjectMeta{Name: testBrokerName},
		Spec: v1beta1.ClusterServiceBrokerSpec{
			URL: testBrokerURL,
		},
	}

	_, err := client.ClusterServiceBrokers().Create(broker)
	if nil != err {
		t.Fatalf("error creating the broker %q (%q)", broker.Name, err)
	}

	err = util.WaitForBrokerCondition(client,
		testBrokerName,
		v1beta1.ServiceBrokerCondition{
			Type:   v1beta1.ServiceBrokerConditionReady,
			Status: v1beta1.ConditionTrue,
		})
	if err != nil {
		t.Fatalf("error waiting for broker to become ready: %v", err)
	}

	err = util.WaitForClusterServiceClassToExist(client, testClusterServiceClassID)
	if nil != err {
		t.Fatalf("error waiting from ClusterServiceClass to exist: %v", err)
	}

	// TODO: find some way to compose scenarios; extract method here for real
	// logic for this test.

	//-----------------

	instance := &v1beta1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: testInstanceName},
		Spec: v1beta1.ServiceInstanceSpec{
			PlanReference: v1beta1.PlanReference{
				ExternalClusterServiceClassName: testClusterServiceClassName,
				ExternalClusterServicePlanName:  testPlanName,
			},
			ExternalID: testExternalID,
		},
	}

	if _, err := client.ServiceInstances(testNamespace).Create(instance); err != nil {
		t.Fatalf("error creating Instance: %v", err)
	}

	if err := util.WaitForInstanceCondition(client, testNamespace, testInstanceName, v1beta1.ServiceInstanceCondition{
		Type:   v1beta1.ServiceInstanceConditionReady,
		Status: v1beta1.ConditionTrue,
	}); err != nil {
		t.Fatalf("error waiting for instance to become ready: %v", err)
	}

	retInst, err := client.ServiceInstances(instance.Namespace).Get(instance.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting instance %s/%s back", instance.Namespace, instance.Name)
	}
	if retInst.Spec.ExternalID != instance.Spec.ExternalID {
		t.Fatalf(
			"returned OSB GUID '%s' doesn't match original '%s'",
			retInst.Spec.ExternalID,
			instance.Spec.ExternalID,
		)
	}

	// Update Instance
	updateRequests := retInst.Spec.UpdateRequests + 1
	retInst.Spec.UpdateRequests = updateRequests
	if _, err := client.ServiceInstances(testNamespace).Update(retInst); err != nil {
		t.Fatalf("error updating Instance: %v", err)
	}

	if err := util.WaitForInstanceReconciledGeneration(client, testNamespace, testInstanceName, retInst.Status.ReconciledGeneration+1); err != nil {
		t.Fatalf("error waiting for instance to reconcile: %v", err)
	}

	retInst, err = client.ServiceInstances(instance.Namespace).Get(instance.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting instance %s/%s back", instance.Namespace, instance.Name)
	}
	if e, a := updateRequests, retInst.Spec.UpdateRequests; e != a {
		t.Fatalf("unexpected updateRequets in instance spec: expected %v, got %v", e, a)
	}

	// Binding test begins here
	//-----------------

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: testBindingName},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: corev1.LocalObjectReference{
				Name: testInstanceName,
			},
		},
	}

	_, err = client.ServiceBindings(testNamespace).Create(binding)
	if err != nil {
		t.Fatalf("error creating Binding: %v", binding)
	}

	err = util.WaitForBindingCondition(client, testNamespace, testBindingName, v1beta1.ServiceBindingCondition{
		Type:   v1beta1.ServiceBindingConditionReady,
		Status: v1beta1.ConditionTrue,
	})
	if err != nil {
		t.Fatalf("error waiting for binding to become ready: %v", err)
	}

	err = client.ServiceBindings(testNamespace).Delete(testBindingName, &metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("binding delete should have been accepted: %v", err)
	}

	err = util.WaitForBindingToNotExist(client, testNamespace, testBindingName)
	if err != nil {
		t.Fatalf("error waiting for binding to not exist: %v", err)
	}

	//-----------------
	// End binding test

	err = client.ServiceInstances(testNamespace).Delete(testInstanceName, &metav1.DeleteOptions{})
	if nil != err {
		t.Fatalf("instance delete should have been accepted: %v", err)
	}

	err = util.WaitForInstanceToNotExist(client, testNamespace, testInstanceName)
	if err != nil {
		t.Fatalf("error waiting for instance to be deleted: %v", err)
	}

	//-----------------
	// End provision test

	// Delete the broker
	err = client.ClusterServiceBrokers().Delete(testBrokerName, &metav1.DeleteOptions{})
	if nil != err {
		t.Fatalf("broker should be deleted (%s)", err)
	}

	err = util.WaitForClusterServiceClassToNotExist(client, testClusterServiceClassName)
	if err != nil {
		t.Fatalf("error waiting for ClusterServiceClass to not exist: %v", err)
	}

	err = util.WaitForBrokerToNotExist(client, testBrokerName)
	if err != nil {
		t.Fatalf("error waiting for Broker to not exist: %v", err)
	}
}

// TestProvisionFailure tests that the controller correctly handles errors
// from the broker that indicate the a provision operation failed.
//
// TODO: additional tests for scenarios like this will be needed once we
// implement orphan mitigation.
func TestProvisionFailure(t *testing.T) {
	_, catalogClient, _, _, _, _, shutdownServer, shutdownController := newTestController(t, fakeosb.FakeClientConfiguration{
		CatalogReaction: &fakeosb.CatalogReaction{
			Response: getTestCatalogResponse(),
		},
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Error: osb.HTTPStatusCodeError{
				StatusCode:   http.StatusConflict,
				ErrorMessage: strPtr("OutOfQuota"),
				Description:  strPtr("You're out of quota!"),
			},
		},
		// no DeprovisionReaction is configured, so that the client will
		// return an unexpected call error message if deprovision is called on
		// the broker.
	})
	defer shutdownController()
	defer shutdownServer()

	client := catalogClient.ServicecatalogV1beta1()

	broker := &v1beta1.ClusterServiceBroker{
		ObjectMeta: metav1.ObjectMeta{Name: testBrokerName},
		Spec: v1beta1.ClusterServiceBrokerSpec{
			URL: testBrokerURL,
		},
	}

	_, err := client.ClusterServiceBrokers().Create(broker)
	if nil != err {
		t.Fatalf("error creating the broker %q (%q)", broker.Name, err)
	}

	err = util.WaitForBrokerCondition(client,
		testBrokerName,
		v1beta1.ServiceBrokerCondition{
			Type:   v1beta1.ServiceBrokerConditionReady,
			Status: v1beta1.ConditionTrue,
		})
	if err != nil {
		t.Fatalf("error waiting for broker to become ready: %v", err)
	}

	err = util.WaitForClusterServiceClassToExist(client, testClusterServiceClassID)
	if nil != err {
		t.Fatalf("error waiting from ClusterServiceClass to exist: %v", err)
	}

	// TODO: find some way to compose scenarios; extract method here for real
	// logic for this test.

	//-----------------

	instance := &v1beta1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: testInstanceName},
		Spec: v1beta1.ServiceInstanceSpec{
			PlanReference: v1beta1.PlanReference{
				ExternalClusterServiceClassName: testClusterServiceClassName,
				ExternalClusterServicePlanName:  testPlanName,
			},
			ExternalID: testExternalID,
		},
	}

	if _, err := client.ServiceInstances(testNamespace).Create(instance); err != nil {
		t.Fatalf("error creating Instance: %v", err)
	}

	if err := util.WaitForInstanceCondition(client, testNamespace, testInstanceName, v1beta1.ServiceInstanceCondition{
		Type:   v1beta1.ServiceInstanceConditionFailed,
		Status: v1beta1.ConditionTrue,
	}); err != nil {
		t.Fatalf("error waiting for instance to become failed: %v", err)
	}

	retInst, err := client.ServiceInstances(instance.Namespace).Get(instance.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting instance %s/%s back", instance.Namespace, instance.Name)
	}
	if retInst.Spec.ExternalID != instance.Spec.ExternalID {
		t.Fatalf(
			"returned OSB GUID '%s' doesn't match original '%s'",
			retInst.Spec.ExternalID,
			instance.Spec.ExternalID,
		)
	}

	err = client.ServiceInstances(testNamespace).Delete(testInstanceName, &metav1.DeleteOptions{})
	if nil != err {
		t.Fatalf("instance delete should have been accepted: %v", err)
	}

	err = util.WaitForInstanceToNotExist(client, testNamespace, testInstanceName)
	if err != nil {
		t.Fatalf("error waiting for instance to be deleted: %v", err)
	}

	//-----------------
	// End provision test

	// Delete the broker
	err = client.ClusterServiceBrokers().Delete(testBrokerName, &metav1.DeleteOptions{})
	if nil != err {
		t.Fatalf("broker should be deleted (%s)", err)
	}

	err = util.WaitForClusterServiceClassToNotExist(client, testClusterServiceClassName)
	if err != nil {
		t.Fatalf("error waiting for ClusterServiceClass to not exist: %v", err)
	}

	err = util.WaitForBrokerToNotExist(client, testBrokerName)
	if err != nil {
		t.Fatalf("error waiting for Broker to not exist: %v", err)
	}
}

// TestBindingFailure tests that a binding gets a failure condition when the
// broker returns a failure response for a bind operation.
func TestBindingFailure(t *testing.T) {
	_, fakeCatalogClient, _, _, _, _, shutdownServer, shutdownController := newTestController(t, fakeosb.FakeClientConfiguration{
		CatalogReaction: &fakeosb.CatalogReaction{
			Response: getTestCatalogResponse(),
		},
		BindReaction: &fakeosb.BindReaction{
			Error: osb.HTTPStatusCodeError{
				StatusCode:   http.StatusConflict,
				ErrorMessage: strPtr("ServiceBindingExists"),
				Description:  strPtr("Service binding with the same id, for the same service instance already exists."),
			},
		},
		UnbindReaction: &fakeosb.UnbindReaction{},
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Response: &osb.ProvisionResponse{
				Async: false,
			},
		},
		DeprovisionReaction: &fakeosb.DeprovisionReaction{
			Response: &osb.DeprovisionResponse{
				Async: false,
			},
		},
	})
	defer shutdownController()
	defer shutdownServer()

	client := fakeCatalogClient.ServicecatalogV1beta1()

	broker := &v1beta1.ClusterServiceBroker{
		ObjectMeta: metav1.ObjectMeta{Name: testBrokerName},
		Spec: v1beta1.ClusterServiceBrokerSpec{
			URL: testBrokerURL,
		},
	}

	_, err := client.ClusterServiceBrokers().Create(broker)
	if nil != err {
		t.Fatalf("error creating the broker %q (%q)", broker.Name, err)
	}

	err = util.WaitForBrokerCondition(client,
		testBrokerName,
		v1beta1.ServiceBrokerCondition{
			Type:   v1beta1.ServiceBrokerConditionReady,
			Status: v1beta1.ConditionTrue,
		})
	if err != nil {
		t.Fatalf("error waiting for broker to become ready: %v", err)
	}

	err = util.WaitForClusterServiceClassToExist(client, testClusterServiceClassID)
	if nil != err {
		t.Fatalf("error waiting from ClusterServiceClass to exist: %v", err)
	}

	// TODO: find some way to compose scenarios; extract method here for real
	// logic for this test.

	//-----------------

	instance := &v1beta1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: testInstanceName},
		Spec: v1beta1.ServiceInstanceSpec{
			PlanReference: v1beta1.PlanReference{
				ExternalClusterServiceClassName: testClusterServiceClassName,
				ExternalClusterServicePlanName:  testPlanName,
			},
			ExternalID: testExternalID,
		},
	}

	if _, err := client.ServiceInstances(testNamespace).Create(instance); err != nil {
		t.Fatalf("error creating Instance: %v", err)
	}

	if err := util.WaitForInstanceCondition(client, testNamespace, testInstanceName, v1beta1.ServiceInstanceCondition{
		Type:   v1beta1.ServiceInstanceConditionReady,
		Status: v1beta1.ConditionTrue,
	}); err != nil {
		t.Fatalf("error waiting for instance to become ready: %v", err)
	}

	retInst, err := client.ServiceInstances(instance.Namespace).Get(instance.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting instance %s/%s back", instance.Namespace, instance.Name)
	}
	if retInst.Spec.ExternalID != instance.Spec.ExternalID {
		t.Fatalf(
			"returned OSB GUID '%s' doesn't match original '%s'",
			retInst.Spec.ExternalID,
			instance.Spec.ExternalID,
		)
	}

	// Binding test begins here
	//-----------------

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: testBindingName},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: corev1.LocalObjectReference{
				Name: testInstanceName,
			},
		},
	}

	_, err = client.ServiceBindings(testNamespace).Create(binding)
	if err != nil {
		t.Fatalf("error creating Binding: %v", binding)
	}

	err = util.WaitForBindingCondition(client, testNamespace, testBindingName, v1beta1.ServiceBindingCondition{
		Type:   v1beta1.ServiceBindingConditionFailed,
		Status: v1beta1.ConditionTrue,
	})
	if err != nil {
		t.Fatalf("error waiting for binding to become failed: %v", err)
	}

	err = client.ServiceBindings(testNamespace).Delete(testBindingName, &metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("binding delete should have been accepted: %v", err)
	}

	err = util.WaitForBindingToNotExist(client, testNamespace, testBindingName)
	if err != nil {
		t.Fatalf("error waiting for binding to not exist: %v", err)
	}

	//-----------------
	// End binding test

	err = client.ServiceInstances(testNamespace).Delete(testInstanceName, &metav1.DeleteOptions{})
	if nil != err {
		t.Fatalf("instance delete should have been accepted: %v", err)
	}

	err = util.WaitForInstanceToNotExist(client, testNamespace, testInstanceName)
	if err != nil {
		t.Fatalf("error waiting for instance to be deleted: %v", err)
	}

	//-----------------
	// End provision test

	// Delete the broker
	err = client.ClusterServiceBrokers().Delete(testBrokerName, &metav1.DeleteOptions{})
	if nil != err {
		t.Fatalf("broker should be deleted (%s)", err)
	}

	err = util.WaitForClusterServiceClassToNotExist(client, testClusterServiceClassName)
	if err != nil {
		t.Fatalf("error waiting for ClusterServiceClass to not exist: %v", err)
	}

	err = util.WaitForBrokerToNotExist(client, testBrokerName)
	if err != nil {
		t.Fatalf("error waiting for Broker to not exist: %v", err)
	}
}

// TestBasicFlowsWithOriginatingIdentity test the same flow as TestBasicFlowsSync, with OriginatingIdentity
// feature enabled.
func TestBasicFlowsWithOriginatingIdentity(t *testing.T) {
	// Enable the OriginatingIdentity feature
	utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=true", scfeatures.OriginatingIdentity))
	defer utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=false", scfeatures.OriginatingIdentity))

	_, catalogClient, catalogClientConfig, _, _, _, shutdownServer, shutdownController := newTestController(t, fakeosb.FakeClientConfiguration{
		CatalogReaction: &fakeosb.CatalogReaction{
			Response: &osb.CatalogResponse{
				Services: []osb.Service{
					{
						Name:        testClusterServiceClassName,
						ID:          "12345",
						Description: "a test service",
						Bindable:    true,
						Plans: []osb.Plan{
							{
								Name:        testPlanName,
								Free:        truePtr(),
								ID:          "34567",
								Description: "a test plan",
							},
						},
					},
				},
			},
		},
		ProvisionReaction: &fakeosb.ProvisionReaction{
			Response: &osb.ProvisionResponse{
				Async: false,
			},
		},
		UpdateInstanceReaction: &fakeosb.UpdateInstanceReaction{
			Response: &osb.UpdateInstanceResponse{
				Async: false,
			},
		},
		BindReaction: &fakeosb.BindReaction{
			Response: &osb.BindResponse{
				Credentials: map[string]interface{}{
					"foo": "bar",
					"baz": "zap",
				},
			},
		},
		UnbindReaction: &fakeosb.UnbindReaction{},
		DeprovisionReaction: &fakeosb.DeprovisionReaction{
			Response: &osb.DeprovisionResponse{
				Async: false,
			},
		},
	})
	defer shutdownController()
	defer shutdownServer()

	client := catalogClient.ServicecatalogV1beta1()

	broker := &v1beta1.ClusterServiceBroker{
		ObjectMeta: metav1.ObjectMeta{Name: testBrokerName},
		Spec: v1beta1.ClusterServiceBrokerSpec{
			URL: testBrokerURL,
		},
	}

	_, err := client.ClusterServiceBrokers().Create(broker)
	if nil != err {
		t.Fatalf("error creating the broker %v (%q)", broker, err)
	}

	err = util.WaitForBrokerCondition(client,
		testBrokerName,
		v1beta1.ServiceBrokerCondition{
			Type:   v1beta1.ServiceBrokerConditionReady,
			Status: v1beta1.ConditionTrue,
		})
	if err != nil {
		t.Fatalf("error waiting for broker to become ready: %v", err)
	}

	err = util.WaitForClusterServiceClassToExist(client, testClusterServiceClassID)
	if nil != err {
		t.Fatalf("error waiting from ClusterServiceClass to exist: %v", err)
	}

	// TODO: find some way to compose scenarios; extract method here for real
	// logic for this test.

	//-----------------

	catalogClient, err = changeUsernameForCatalogClient(catalogClient, catalogClientConfig, testCreatorUsername)
	if err != nil {
		t.Fatalf("could not change the username for the catalog client: %v", err)
	}

	client = catalogClient.ServicecatalogV1beta1()

	instance := &v1beta1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: testInstanceName},
		Spec: v1beta1.ServiceInstanceSpec{
			PlanReference: v1beta1.PlanReference{
				ExternalClusterServiceClassName: testClusterServiceClassName,
				ExternalClusterServicePlanName:  testPlanName,
			},
			ExternalID: testExternalID,
		},
	}

	// Create Instance
	if _, err := client.ServiceInstances(testNamespace).Create(instance); err != nil {
		t.Fatalf("error creating Instance: %v", err)
	}

	if err := util.WaitForInstanceCondition(client, testNamespace, testInstanceName, v1beta1.ServiceInstanceCondition{
		Type:   v1beta1.ServiceInstanceConditionReady,
		Status: v1beta1.ConditionTrue,
	}); err != nil {
		t.Fatalf("error waiting for instance to become ready: %v", err)
	}

	retInst, err := client.ServiceInstances(instance.Namespace).Get(instance.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting instance %s/%s back", instance.Namespace, instance.Name)
	}
	if retInst.Spec.UserInfo == nil {
		t.Fatalf("instance spec does not include creating user info")
	}
	if e, a := testCreatorUsername, retInst.Spec.UserInfo.Username; e != a {
		t.Fatalf("unexpected creating user name in instance spec: expected %v, got %v", e, a)
	}

	// Update Instance
	catalogClient, err = changeUsernameForCatalogClient(catalogClient, catalogClientConfig, testUpdaterUsername)
	if err != nil {
		t.Fatalf("could not change the username for the catalog client: %v", err)
	}

	client = catalogClient.ServicecatalogV1beta1()

	retInst.Spec.UpdateRequests = retInst.Spec.UpdateRequests + 1
	if _, err := client.ServiceInstances(testNamespace).Update(retInst); err != nil {
		t.Fatalf("error updating Instance: %v", err)
	}

	if err := util.WaitForInstanceReconciledGeneration(client, testNamespace, testInstanceName, retInst.Status.ReconciledGeneration+1); err != nil {
		t.Fatalf("error waiting for instance to reconcile: %v", err)
	}

	retInst, err = client.ServiceInstances(instance.Namespace).Get(instance.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting instance %s/%s back", instance.Namespace, instance.Name)
	}
	if retInst.Spec.UserInfo == nil {
		t.Fatalf("instance spec does not include creating user info")
	}
	if e, a := testUpdaterUsername, retInst.Spec.UserInfo.Username; e != a {
		t.Fatalf("unexpected updating user name in instance spec: expected %v, got %v", e, a)
	}

	// Binding test begins here
	//-----------------

	// Create ServiceBinding
	catalogClient, err = changeUsernameForCatalogClient(catalogClient, catalogClientConfig, testCreatorUsername)
	if err != nil {
		t.Fatalf("could not change the username for the catalog client: %v", err)
	}

	client = catalogClient.ServicecatalogV1beta1()

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: testBindingName},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: corev1.LocalObjectReference{
				Name: testInstanceName,
			},
		},
	}

	_, err = client.ServiceBindings(testNamespace).Create(binding)
	if err != nil {
		t.Fatalf("error creating Binding: %v", binding)
	}

	err = util.WaitForBindingCondition(client, testNamespace, testBindingName, v1beta1.ServiceBindingCondition{
		Type:   v1beta1.ServiceBindingConditionReady,
		Status: v1beta1.ConditionTrue,
	})
	if err != nil {
		t.Fatalf("error waiting for binding to become ready: %v", err)
	}

	retBinding, err := client.ServiceBindings(testNamespace).Get(testBindingName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting binding %s/%s back", testNamespace, testBindingName)
	}
	if retBinding.Spec.UserInfo == nil {
		t.Fatalf("binding spec does not include creating user info")
	}
	if e, a := testCreatorUsername, retBinding.Spec.UserInfo.Username; e != a {
		t.Fatalf("unexpected creating user name in binding spec: expected %v, got %v", e, a)
	}

	// Delete ServiceBinding
	catalogClient, err = changeUsernameForCatalogClient(catalogClient, catalogClientConfig, testDeleterUsername)
	if err != nil {
		t.Fatalf("could not change the username for the catalog client: %v", err)
	}

	client = catalogClient.ServicecatalogV1beta1()

	deleteGracePeriod := int64(60)
	deleteOptions := &metav1.DeleteOptions{GracePeriodSeconds: &deleteGracePeriod}
	if err := client.ServiceInstances(testNamespace).Delete(instance.Name, deleteOptions); err != nil {
		t.Fatalf("error updating Instance: %v", err)
	}

	retInst, err = client.ServiceInstances(instance.Namespace).Get(instance.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting instance %s/%s back", instance.Namespace, instance.Name)
	}
	if retInst.Spec.UserInfo == nil {
		t.Fatalf("instance spec does not include creating user info")
	}
	if e, a := testDeleterUsername, retInst.Spec.UserInfo.Username; e != a {
		t.Fatalf("unexpected deleting user name in instance spec: expected %v, got %v", e, a)
	}

	//-----------------
	// End binding test

	// Delete Instance
	catalogClient, err = changeUsernameForCatalogClient(catalogClient, catalogClientConfig, testDeleterUsername)
	if err != nil {
		t.Fatalf("could not change the username for the catalog client: %v", err)
	}

	client = catalogClient.ServicecatalogV1beta1()

	if err := client.ServiceInstances(testNamespace).Delete(instance.Name, deleteOptions); err != nil {
		t.Fatalf("error updating Instance: %v", err)
	}

	retInst, err = client.ServiceInstances(instance.Namespace).Get(instance.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting instance %s/%s back", instance.Namespace, instance.Name)
	}
	if retInst.Spec.UserInfo == nil {
		t.Fatalf("instance spec does not include creating user info")
	}
	if e, a := testDeleterUsername, retInst.Spec.UserInfo.Username; e != a {
		t.Fatalf("unexpected deleting user name in instance spec: expected %v, got %v", e, a)
	}
}

// newTestController creates a new test controller injected with fake clients
// and returns:
//
// - a fake kubernetes core api client
// - a fake service catalog api client
// - a fake osb client
// - a test controller
// - the shared informers for the service catalog v1beta1 api
//
// If there is an error, newTestController calls 'Fatal' on the injected
// testing.T.
func newTestController(t *testing.T, config fakeosb.FakeClientConfiguration) (
	*fake.Clientset,
	clientset.Interface,
	*restclient.Config,
	*fakeosb.FakeClient,
	controller.Controller,
	informers.Interface,
	func(),
	func()) {

	// create a fake kube client
	fakeKubeClient := &fake.Clientset{}
	addGetSecretNotFoundReaction(fakeKubeClient)

	// create an sc client and running server
	catalogClient, catalogClientConfig, shutdownServer := getFreshApiserverAndClient(t, server.StorageTypeEtcd.String(), func() runtime.Object {
		return &servicecatalog.ClusterServiceBroker{}
	})

	fakeOSBClient := fakeosb.NewFakeClient(config) // error should always be nil
	brokerClFunc := fakeosb.ReturnFakeClientFunc(fakeOSBClient)

	// create informers
	informerFactory := scinformers.NewSharedInformerFactory(catalogClient, 10*time.Second)
	serviceCatalogSharedInformers := informerFactory.Servicecatalog().V1beta1()

	fakeRecorder := record.NewFakeRecorder(10)

	// create a test controller
	testController, err := controller.NewController(
		fakeKubeClient,
		catalogClient.ServicecatalogV1beta1(),
		serviceCatalogSharedInformers.ClusterServiceBrokers(),
		serviceCatalogSharedInformers.ClusterServiceClasses(),
		serviceCatalogSharedInformers.ServiceInstances(),
		serviceCatalogSharedInformers.ServiceBindings(),
		serviceCatalogSharedInformers.ClusterServicePlans(),
		brokerClFunc,
		24*time.Hour,
		osb.LatestAPIVersion().HeaderValue(),
		fakeRecorder,
		7*24*time.Hour,
	)
	t.Log("controller start")
	if err != nil {
		t.Fatal(err)
	}

	stopCh := make(chan struct{})
	controllerStopped := make(chan struct{})
	go func() {
		testController.Run(1, stopCh)
		controllerStopped <- struct{}{}
	}()
	informerFactory.Start(stopCh)
	t.Log("informers start")

	shutdownController := func() {
		close(stopCh)
		<-controllerStopped
	}

	return fakeKubeClient, catalogClient, catalogClientConfig, fakeOSBClient, testController, serviceCatalogSharedInformers, shutdownServer, shutdownController
}

func changeUsernameForCatalogClient(catalogClient clientset.Interface, catalogClientConfig *restclient.Config, username string) (clientset.Interface, error) {
	catalogClientConfig.Username = username
	var err error
	catalogClient, err = clientset.NewForConfig(catalogClientConfig)
	if nil != err {
		return nil, fmt.Errorf("can't make the client from the config: %v", err)
	}
	return catalogClient, nil
}

func addGetSecretNotFoundReaction(fakeKubeClient *fake.Clientset) {
	fakeKubeClient.AddReactor("get", "secrets", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(action.GetResource().GroupResource(), action.(clientgotesting.GetAction).GetName())
	})
}

func getTestCatalogResponse() *osb.CatalogResponse {
	return &osb.CatalogResponse{
		Services: []osb.Service{
			{
				Name:        testClusterServiceClassName,
				ID:          "12345",
				Description: "a test service",
				Bindable:    true,
				Plans: []osb.Plan{
					{
						Name:        testPlanName,
						Free:        truePtr(),
						ID:          testPlanExternalID,
						Description: "a test plan",
					},
				},
			},
		},
	}
}

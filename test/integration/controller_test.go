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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/record"

	// avoid error `servicecatalog/v1alpha1 is not enabled`
	_ "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/install"
	// avoid error `no kind is registered for the type metav1.ListOptions`
	_ "k8s.io/client-go/pkg/api/install"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	"github.com/kubernetes-incubator/service-catalog/pkg/brokerapi"
	fakebrokerapi "github.com/kubernetes-incubator/service-catalog/pkg/brokerapi/fake"
	"github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset"
	scinformers "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/externalversions"
	informers "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/externalversions/servicecatalog/v1alpha1"
	"github.com/kubernetes-incubator/service-catalog/pkg/controller"
	"github.com/kubernetes-incubator/service-catalog/pkg/registry/servicecatalog/server"
	"github.com/kubernetes-incubator/service-catalog/test/util"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	testNamespace        = "test-namespace"
	testBrokerName       = "test-broker"
	testServiceClassName = "test-service"
	planName             = "test-plan"
	testInstanceName     = "test-instance"
	testBindingName      = "test-binding"
	testSecretName       = "test-secret"
	testExternalID       = "9737b6ed-ca95-4439-8219-c53fcad118ab"
	testDashboardURL     = "http://test-dashboard.example.com"
)

// TestBasicFlows tests:
//
// - add Broker
// - verify ServiceClasses added
// - provision Instance
// - make Binding
// - unbind
// - deprovision
// - delete broker
func TestBasicFlows(t *testing.T) {
	_, catalogClient, fakeBrokerCatalog, _, _, _, _, shutdownServer := newTestController(t)
	defer shutdownServer()

	client := catalogClient.ServicecatalogV1alpha1()

	const (
		testBrokerName       = "test-broker"
		testServiceClassName = "test-service"
		testPlanName         = "test-plan"
	)

	fakeBrokerCatalog.RetCatalog = &brokerapi.Catalog{
		Services: []*brokerapi.Service{
			{
				Name:        testServiceClassName,
				ID:          "12345",
				Description: "a test service",
				Bindable:    true,
				Plans: []brokerapi.ServicePlan{
					{
						Name:        testPlanName,
						Free:        true,
						ID:          "34567",
						Description: "a test plan",
					},
				},
			},
		},
	}
	broker := &v1alpha1.Broker{
		ObjectMeta: metav1.ObjectMeta{Name: testBrokerName},
		Spec: v1alpha1.BrokerSpec{
			URL: "https://example.com",
		},
	}

	_, err := client.Brokers().Create(broker)
	if nil != err {
		t.Fatalf("error creating the broker %q (%q)", broker, err)
	}

	err = util.WaitForBrokerCondition(client,
		testBrokerName,
		v1alpha1.BrokerCondition{
			Type:   v1alpha1.BrokerConditionReady,
			Status: v1alpha1.ConditionTrue,
		})
	if err != nil {
		t.Fatalf("error waiting for broker to become ready: %v", err)
	}

	err = util.WaitForServiceClassToExist(client, testServiceClassName)
	if nil != err {
		t.Fatalf("error waiting from ServiceClass to exist: %v", err)
	}

	// TODO: find some way to compose scenarios; extract method here for real
	// logic for this test.

	//-----------------

	instance := &v1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: testInstanceName},
		Spec: v1alpha1.InstanceSpec{
			ServiceClassName: testServiceClassName,
			PlanName:         testPlanName,
			ExternalID:       testExternalID,
		},
	}

	if _, err := client.Instances(testNamespace).Create(instance); err != nil {
		t.Fatalf("error creating Instance: %v", err)
	}

	if err := util.WaitForInstanceCondition(client, testNamespace, testInstanceName, v1alpha1.InstanceCondition{
		Type:   v1alpha1.InstanceConditionReady,
		Status: v1alpha1.ConditionTrue,
	}); err != nil {
		t.Fatalf("error waiting for instance to become ready: %v", err)
	}

	retInst, err := client.Instances(instance.Namespace).Get(instance.Name, metav1.GetOptions{})
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

	binding := &v1alpha1.Binding{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: testBindingName},
		Spec: v1alpha1.BindingSpec{
			InstanceRef: v1.LocalObjectReference{
				Name: testInstanceName,
			},
			SecretName: testSecretName,
		},
	}

	_, err = client.Bindings(testNamespace).Create(binding)
	if err != nil {
		t.Fatalf("error creating Binding: %v", binding)
	}

	err = util.WaitForBindingCondition(client, testNamespace, testBindingName, v1alpha1.BindingCondition{
		Type:   v1alpha1.BindingConditionReady,
		Status: v1alpha1.ConditionTrue,
	})
	if err != nil {
		t.Fatalf("error waiting for binding to become ready: %v", err)
	}

	err = client.Bindings(testNamespace).Delete(testBindingName, &metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("binding delete should have been accepted: %v", err)
	}

	err = util.WaitForBindingToNotExist(client, testNamespace, testBindingName)
	if err != nil {
		t.Fatalf("error waiting for binding to not exist: %v", err)
	}

	//-----------------
	// End binding test

	err = client.Instances(testNamespace).Delete(testInstanceName, &metav1.DeleteOptions{})
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
	err = client.Brokers().Delete(testBrokerName, &metav1.DeleteOptions{})
	if nil != err {
		t.Fatalf("broker should be deleted (%s)", err)
	}

	err = util.WaitForServiceClassToNotExist(client, testServiceClassName)
	if err != nil {
		t.Fatalf("error waiting for ServiceClass to not exist: %v", err)
	}

	err = util.WaitForBrokerToNotExist(client, testBrokerName)
	if err != nil {
		t.Fatalf("error waiting for Broker to not exist: %v", err)
	}
}

func newTestController(t *testing.T) (
	*fake.Clientset,
	clientset.Interface,
	*fakebrokerapi.CatalogClient,
	*fakebrokerapi.InstanceClient,
	*fakebrokerapi.BindingClient,
	controller.Controller,
	informers.Interface,
	func(),
) {
	// create a fake kube client
	fakeKubeClient := &fake.Clientset{}
	// create an sc client and running server
	catalogClient, shutdownServer := getFreshApiserverAndClient(t, server.StorageTypeEtcd.String(), func() runtime.Object {
		return &servicecatalog.Broker{}
	})

	catalogCl := &fakebrokerapi.CatalogClient{}
	instanceCl := fakebrokerapi.NewInstanceClient()
	bindingCl := fakebrokerapi.NewBindingClient()
	brokerClFunc := fakebrokerapi.NewClientFunc(catalogCl, instanceCl, bindingCl)

	// create informers
	resync := 1 * time.Minute

	informerFactory := scinformers.NewSharedInformerFactory(catalogClient, resync)
	serviceCatalogSharedInformers := informerFactory.Servicecatalog().V1alpha1()

	// create a test controller
	testController, err := controller.NewController(
		fakeKubeClient,
		catalogClient.ServicecatalogV1alpha1(),
		serviceCatalogSharedInformers.Brokers(),
		serviceCatalogSharedInformers.ServiceClasses(),
		serviceCatalogSharedInformers.Instances(),
		serviceCatalogSharedInformers.Bindings(),
		brokerClFunc,
		24*time.Hour,
		true,
		&record.FakeRecorder{},
	)
	t.Log("controller start")
	if err != nil {
		t.Fatal(err)
	}

	stopCh := make(chan struct{})
	go testController.Run(1, stopCh)
	informerFactory.Start(stopCh)
	t.Log("informers start")
	return fakeKubeClient, catalogClient, catalogCl, instanceCl, bindingCl,
		testController, serviceCatalogSharedInformers, shutdownServer
}

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
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/kubernetes-incubator/service-catalog/pkg/registry/servicecatalog/server"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// our versioned types
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	// our versioned client
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	servicecatalogclient "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
)

// Used for testing instance parameters
type ipStruct struct {
	Bar    string            `json:"bar"`
	Values map[string]string `json:"values"`
}

const (
	instanceParameter = `{
    "bar": "barvalue",
    "values": {
      "first" : "firstvalue",
      "second" : "secondvalue"
    }
  }
`
	bindingParameter = `{
    "foo": "bar",
    "baz": [
      "first",
      "second"
    ]
  }
`
)

var storageTypes = []server.StorageType{
	server.StorageTypeEtcd,
}

// Used for testing binding parameters
type bpStruct struct {
	Foo string   `json:"foo"`
	Baz []string `json:"baz"`
}

// TestGroupVersion is trivial.
func TestGroupVersion(t *testing.T) {
	rootTestFunc := func(sType server.StorageType) func(t *testing.T) {
		return func(t *testing.T) {
			client, _, shutdownServer := getFreshApiserverAndClient(t, sType.String(), func() runtime.Object {
				return &servicecatalog.ClusterServiceBroker{}
			})
			defer shutdownServer()
			if err := testGroupVersion(client); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, sType := range storageTypes {
		if !t.Run(sType.String(), rootTestFunc(sType)) {
			t.Errorf("%q test failed", sType)
		}
	}
}

func TestEtcdHealthCheckerSuccess(t *testing.T) {
	serverConfig := NewTestServerConfig()
	serverConfig.storageType = server.StorageTypeEtcd
	_, clientconfig, shutdownServer := withConfigGetFreshApiserverAndClient(t, serverConfig)
	t.Log(clientconfig.Host)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	c := &http.Client{Transport: tr}
	resp, err := c.Get(clientconfig.Host + "/healthz")
	if nil != err {
		t.Fatal("health check endpoint should not have failed", err)
	}

	if http.StatusOK != resp.StatusCode {
		t.Fatal("health check endpoint should have had a 200 status code", resp)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("couldn't read response body", err)
	}
	if strings.Contains(string(body), "healthz check failed") {
		t.Fatal("health check endpoint should not have failed")
	}

	defer shutdownServer()
}

func TestEtcdHealthCheckerFail(t *testing.T) {
	serverConfig := NewTestServerConfig()
	// this server won't exist
	serverConfig.etcdServerList = []string{""}
	serverConfig.storageType = server.StorageTypeEtcd
	_, clientconfig, shutdownServer := withConfigGetFreshApiserverAndClient(t, serverConfig)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	c := &http.Client{Transport: tr}
	resp, err := c.Get(clientconfig.Host + "/healthz")
	if nil != err || http.StatusInternalServerError != resp.StatusCode {
		t.Fatal("health check endpoint should have failed and did not")
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("couldn't read response body", err)
	}
	if !strings.Contains(string(body), "healthz check failed") {
		t.Fatal("health check endpoint should contain a failure message")
	}

	defer shutdownServer()
}

func testGroupVersion(client servicecatalogclient.Interface) error {
	gv := client.Servicecatalog().RESTClient().APIVersion()
	if gv.Group != servicecatalog.GroupName {
		return fmt.Errorf("we should be testing the servicecatalog group, not %s", gv.Group)
	}
	return nil
}

// TestNoName checks that all creates fail for objects that have no
// name given.
func TestNoName(t *testing.T) {
	rootTestFunc := func(sType server.StorageType) func(t *testing.T) {
		return func(t *testing.T) {
			client, _, shutdownServer := getFreshApiserverAndClient(t, sType.String(), func() runtime.Object {
				return &servicecatalog.ClusterServiceBroker{}
			})
			defer shutdownServer()
			if err := testNoName(client); err != nil {
				t.Fatal(err)
			}
		}
	}

	for _, sType := range storageTypes {
		if !t.Run(sType.String(), rootTestFunc(sType)) {
			t.Errorf("%q test failed", sType)
		}
	}
}

func testNoName(client servicecatalogclient.Interface) error {
	scClient := client.Servicecatalog()

	ns := "namespace"

	if br, e := scClient.ClusterServiceBrokers().Create(&v1beta1.ClusterServiceBroker{}); nil == e {
		return fmt.Errorf("needs a name (%s)", br.Name)
	}
	if sc, e := scClient.ClusterServiceClasses().Create(&v1beta1.ClusterServiceClass{}); nil == e {
		return fmt.Errorf("needs a name (%s)", sc.Name)
	}
	if sp, e := scClient.ClusterServicePlans().Create(&v1beta1.ClusterServicePlan{}); nil == e {
		return fmt.Errorf("needs a name (%s)", sp.Name)
	}
	if i, e := scClient.ServiceInstances(ns).Create(&v1beta1.ServiceInstance{}); nil == e {
		return fmt.Errorf("needs a name (%s)", i.Name)
	}
	if bi, e := scClient.ServiceBindings(ns).Create(&v1beta1.ServiceBinding{}); nil == e {
		return fmt.Errorf("needs a name (%s)", bi.Name)
	}
	return nil
}

// TestBrokerClient exercises the Broker client.
func TestBrokerClient(t *testing.T) {
	const name = "test-broker"
	rootTestFunc := func(sType server.StorageType) func(t *testing.T) {
		return func(t *testing.T) {
			client, _, shutdownServer := getFreshApiserverAndClient(t, sType.String(), func() runtime.Object {
				return &servicecatalog.ClusterServiceBroker{}
			})
			defer shutdownServer()
			if err := testBrokerClient(sType, client, name); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, sType := range storageTypes {
		if !t.Run(sType.String(), rootTestFunc(sType)) {
			t.Errorf("%q test failed", sType)
		}
	}
}

func testBrokerClient(sType server.StorageType, client servicecatalogclient.Interface, name string) error {
	brokerClient := client.Servicecatalog().ClusterServiceBrokers()
	broker := &v1beta1.ClusterServiceBroker{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1beta1.ClusterServiceBrokerSpec{
			CommonServiceBrokerSpec: v1beta1.CommonServiceBrokerSpec{
				URL: "https://example.com",
			},
		},
	}

	// start from scratch
	brokers, err := brokerClient.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing brokers (%s)", err)
	}
	if brokers.Items == nil {
		return fmt.Errorf("Items field should not be set to nil")
	}
	if len(brokers.Items) > 0 {
		return fmt.Errorf("brokers should not exist on start, had %v brokers", len(brokers.Items))
	}

	brokerServer, err := brokerClient.Create(broker)
	if nil != err {
		return fmt.Errorf("error creating the broker '%v' (%v)", broker, err)
	}
	if name != brokerServer.Name {
		return fmt.Errorf("didn't get the same broker back from the server \n%+v\n%+v", broker, brokerServer)
	}

	brokers, err = brokerClient.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing brokers (%s)", err)
	}
	if 1 != len(brokers.Items) {
		return fmt.Errorf("should have exactly one broker, had %v brokers", len(brokers.Items))
	}

	brokerServer, err = brokerClient.Get(name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting broker %s (%s)", name, err)
	}
	if name != brokerServer.Name &&
		broker.ResourceVersion == brokerServer.ResourceVersion {
		return fmt.Errorf("didn't get the same broker back from the server \n%+v\n%+v", broker, brokerServer)
	}

	// check that the broker is the same from get and list
	brokerListed := &brokers.Items[0]
	if !reflect.DeepEqual(brokerServer, brokerListed) {
		return fmt.Errorf(
			"Didn't get the same instance from list and get: diff: %v",
			diff.ObjectReflectDiff(brokerServer, brokerListed),
		)
	}

	authSecret := &v1beta1.ObjectReference{
		Namespace: "test-namespace",
		Name:      "test-name",
	}

	brokerServer.Spec.AuthInfo = &v1beta1.ClusterServiceBrokerAuthInfo{
		Basic: &v1beta1.ClusterBasicAuthConfig{
			SecretRef: authSecret,
		},
	}

	brokerUpdated, err := brokerClient.Update(brokerServer)
	if nil != err ||
		"test-namespace" != brokerUpdated.Spec.AuthInfo.Basic.SecretRef.Namespace ||
		"test-name" != brokerUpdated.Spec.AuthInfo.Basic.SecretRef.Name {
		return fmt.Errorf("broker wasn't updated, %v, %v", brokerServer, brokerUpdated)
	}

	readyConditionTrue := v1beta1.ServiceBrokerCondition{
		Type:    v1beta1.ServiceBrokerConditionReady,
		Status:  v1beta1.ConditionTrue,
		Reason:  "ConditionReason",
		Message: "ConditionMessage",
	}
	brokerUpdated.Status = v1beta1.ClusterServiceBrokerStatus{
		Conditions: []v1beta1.ServiceBrokerCondition{
			readyConditionTrue,
		},
	}
	brokerUpdated.Spec.URL = "http://shouldnotupdate.com"

	brokerUpdated2, err := brokerClient.UpdateStatus(brokerUpdated)
	if nil != err || len(brokerUpdated2.Status.Conditions) != 1 {
		return fmt.Errorf("broker status wasn't updated")
	}
	if e, a := readyConditionTrue, brokerUpdated2.Status.Conditions[0]; !reflect.DeepEqual(e, a) {
		return fmt.Errorf("Didn't get matching ready conditions:\nexpected: %v\n\ngot: %v", e, a)
	}
	if e, a := "https://example.com", brokerUpdated2.Spec.URL; e != a {
		return fmt.Errorf("Should not be able to update spec from status subresource")
	}

	readyConditionFalse := v1beta1.ServiceBrokerCondition{
		Type:    v1beta1.ServiceBrokerConditionReady,
		Status:  v1beta1.ConditionFalse,
		Reason:  "ConditionReason",
		Message: "ConditionMessage",
	}
	brokerUpdated2.Status.Conditions[0] = readyConditionFalse

	brokerUpdated3, err := brokerClient.UpdateStatus(brokerUpdated2)
	if nil != err || len(brokerUpdated3.Status.Conditions) != 1 {
		return fmt.Errorf("broker status wasn't updated (%s)", err)
	}

	brokerServer, err = brokerClient.Get(name, metav1.GetOptions{})
	if nil != err ||
		"test-namespace" != brokerServer.Spec.AuthInfo.Basic.SecretRef.Namespace ||
		"test-name" != brokerServer.Spec.AuthInfo.Basic.SecretRef.Name {
		return fmt.Errorf("broker wasn't updated (%v)", brokerServer)
	}
	if e, a := readyConditionFalse, brokerServer.Status.Conditions[0]; !reflect.DeepEqual(e, a) {
		return fmt.Errorf("Didn't get matching ready conditions:\nexpected: %v\n\ngot: %v", e, a)
	}

	err = brokerClient.Delete(name, &metav1.DeleteOptions{})
	if nil != err {
		return fmt.Errorf("broker should be deleted (%s)", err)
	}

	brokerDeleted, err := brokerClient.Get(name, metav1.GetOptions{})
	if nil != err {
		return fmt.Errorf("broker should not be deleted (%v): %v", brokerDeleted, err)
	}

	brokerDeleted.ObjectMeta.Finalizers = nil
	_, err = brokerClient.UpdateStatus(brokerDeleted)
	if nil != err {
		return fmt.Errorf("broker should be deleted (%v): %v", brokerDeleted, err)
	}

	brokerDeleted, err = brokerClient.Get("test-broker", metav1.GetOptions{})
	if nil == err {
		return fmt.Errorf("broker should be deleted (%v)", brokerDeleted)
	}
	return nil
}

// TestClusterServiceClassClient exercises the ClusterServiceClass client.
func TestClusterServiceClassClient(t *testing.T) {
	rootTestFunc := func(sType server.StorageType) func(t *testing.T) {
		return func(t *testing.T) {
			const name = "test-serviceclass"
			client, _, shutdownServer := getFreshApiserverAndClient(t, sType.String(), func() runtime.Object {
				return &servicecatalog.ClusterServiceClass{}
			})
			defer shutdownServer()

			if err := testClusterServiceClassClient(sType, client, name); err != nil {
				t.Fatal(err)
			}
		}
	}
	// TODO: Fix this for CRD.
	// https://github.com/kubernetes-incubator/service-catalog/issues/1256
	//	for _, sType := range storageTypes {
	//		if !t.Run(sType.String(), rootTestFunc(sType)) {
	//			t.Errorf("%q test failed", sType)
	//		}
	//	}
	//	for _, sType := range storageTypes {
	//		if !t.Run(sType.String(), rootTestFunc(sType)) {
	//			t.Errorf("%q test failed", sType)
	//		}
	//	}
	sType := server.StorageTypeEtcd
	if !t.Run(sType.String(), rootTestFunc(sType)) {
		t.Errorf("%q test failed", sType)
	}
}

func testClusterServiceClassClient(sType server.StorageType, client servicecatalogclient.Interface, name string) error {
	serviceClassClient := client.Servicecatalog().ClusterServiceClasses()

	serviceClass := &v1beta1.ClusterServiceClass{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1beta1.ClusterServiceClassSpec{
			ClusterServiceBrokerName: "test-broker",
			CommonServiceClassSpec: v1beta1.CommonServiceClassSpec{
				Bindable:     true,
				ExternalName: name,
				ExternalID:   "b8269ab4-7d2d-456d-8c8b-5aab63b321d1",
				Description:  "test description",
			},
		},
	}

	// start from scratch
	serviceClasses, err := serviceClassClient.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing service classes (%s)", err)
	}
	if serviceClasses.Items == nil {
		return fmt.Errorf("Items field should not be set to nil")
	}
	if len(serviceClasses.Items) > 0 {
		return fmt.Errorf(
			"serviceClasses should not exist on start, had %v serviceClasses",
			len(serviceClasses.Items),
		)
	}

	serviceClassAtServer, err := serviceClassClient.Create(serviceClass)
	if nil != err {
		return fmt.Errorf("error creating the ClusterServiceClass (%v)", serviceClass)
	}
	if name != serviceClassAtServer.Name {
		return fmt.Errorf(
			"didn't get the same ClusterServiceClass back from the server \n%+v\n%+v",
			serviceClass,
			serviceClassAtServer,
		)
	}

	serviceClasses, err = serviceClassClient.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing service classes (%s)", err)
	}
	if 1 != len(serviceClasses.Items) {
		return fmt.Errorf("should have exactly one ClusterServiceClass, had %v ClusterServiceClasses", len(serviceClasses.Items))
	}

	serviceClassAtServer, err = serviceClassClient.Get(name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error listing service classes (%s)", err)
	}
	if serviceClassAtServer.Name != name &&
		serviceClass.ResourceVersion == serviceClassAtServer.ResourceVersion {
		return fmt.Errorf(
			"didn't get the same ClusterServiceClass back from the server \n%+v\n%+v",
			serviceClass,
			serviceClassAtServer,
		)
	}

	// check that the broker is the same from get and list
	serviceClassListed := &serviceClasses.Items[0]
	if !reflect.DeepEqual(serviceClassAtServer, serviceClassListed) {
		return fmt.Errorf(
			"Didn't get the same instance from list and get: diff: %v",
			diff.ObjectReflectDiff(serviceClassAtServer, serviceClassListed),
		)
	}

	serviceClassAtServer.Spec.Bindable = false
	_, err = serviceClassClient.Update(serviceClassAtServer)
	if err != nil {
		return fmt.Errorf("Error updating serviceClass: %v", err)
	}
	updated, err := serviceClassClient.Get(name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Error getting serviceClass: %v", err)
	}
	if updated.Spec.Bindable {
		return errors.New("Failed to update service class")
	}

	// Test status subresource
	updated.Status.RemovedFromBrokerCatalog = true
	updated, err = serviceClassClient.UpdateStatus(updated)
	if err != nil {
		return fmt.Errorf("Error updating serviceClass status: %v", err)
	}
	if !updated.Status.RemovedFromBrokerCatalog {
		return errors.New("Expected status.removedFromBrokerCatalog = true, got false")
	}

	// Ok, let's verify the field selectors
	sc2Name := name + "2"
	sc2ID := "someotheridhere"
	serviceClass2 := &v1beta1.ClusterServiceClass{
		ObjectMeta: metav1.ObjectMeta{Name: sc2Name},
		Spec: v1beta1.ClusterServiceClassSpec{
			ClusterServiceBrokerName: "test-broker",
			CommonServiceClassSpec: v1beta1.CommonServiceClassSpec{
				Bindable:     true,
				ExternalName: sc2Name,
				ExternalID:   sc2ID,
				Description:  "test description 2",
			},
		},
	}
	_, err = serviceClassClient.Create(serviceClass2)
	if nil != err {
		return fmt.Errorf("error creating the ClusterServiceClass (%v) : %s", serviceClass2, err)
	}

	serviceClasses, err = serviceClassClient.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing service classes (%s)", err)
	}
	if 2 != len(serviceClasses.Items) {
		return fmt.Errorf("should have two ClusterServiceClasses, had %v ClusterServiceClasses", len(serviceClasses.Items))
	}

	serviceClasses, err = serviceClassClient.List(metav1.ListOptions{FieldSelector: "spec.externalName==" + sc2Name})
	if err != nil {
		return fmt.Errorf("error listing service classes (%s)", err)
	}
	if 1 != len(serviceClasses.Items) {
		return fmt.Errorf("*should have one ClusterServiceClass, had %v ClusterServiceClassess : %+v", len(serviceClasses.Items), serviceClasses.Items)
	}

	if serviceClasses.Items[0].Spec.ExternalID != sc2ID {
		return fmt.Errorf("should have same externalID: %q, got %q", sc2ID, serviceClasses.Items[0].Spec.ExternalID)
	}

	serviceClasses, err = serviceClassClient.List(metav1.ListOptions{FieldSelector: "spec.externalID==" + "b8269ab4-7d2d-456d-8c8b-5aab63b321d1"})
	if err != nil {
		return fmt.Errorf("error listing service classes (%s)", err)
	}
	if 1 != len(serviceClasses.Items) {
		return fmt.Errorf("**should have one ClusterServiceClass, had %v ClusterServiceClasses : %+v", len(serviceClasses.Items), serviceClasses.Items)
	}

	if serviceClasses.Items[0].Spec.ExternalName != name {
		return fmt.Errorf("should have same externalName: %q, got %q", name, serviceClasses.Items[0].Spec.ExternalName)
	}

	serviceClasses, err = serviceClassClient.List(metav1.ListOptions{FieldSelector: "spec.externalName==" + "crap"})
	if err != nil {
		return fmt.Errorf("error listing service classes (%s)", err)
	}
	if 0 != len(serviceClasses.Items) {
		return fmt.Errorf("should have zero ClusterServiceClasses, had %v ClusterServiceClasses : %+v", len(serviceClasses.Items), serviceClasses.Items)
	}

	serviceClasses, err = serviceClassClient.List(metav1.ListOptions{FieldSelector: "spec.clusterServiceBrokerName=" + "test-broker"})
	if err != nil {
		return fmt.Errorf("error listing service classes (%s)", err)
	}
	if 2 != len(serviceClasses.Items) {
		return fmt.Errorf("should have two ClusterServiceClasses, had %v ClusterServiceClasses", len(serviceClasses.Items))
	}

	err = serviceClassClient.Delete(name, &metav1.DeleteOptions{})
	if nil != err {
		return fmt.Errorf("serviceclass should be deleted (%s)", err)
	}

	serviceClassDeleted, err := serviceClassClient.Get(name, metav1.GetOptions{})
	if nil == err {
		return fmt.Errorf("serviceclass should be deleted (%v)", serviceClassDeleted)
	}

	err = serviceClassClient.Delete(sc2Name, &metav1.DeleteOptions{})
	if nil != err {
		return fmt.Errorf("serviceclass should be deleted (%s)", err)
	}
	return nil
}

// TestClusterServicePlanClient exercises the ClusterServicePlan client.
func TestClusterServicePlanClient(t *testing.T) {
	rootTestFunc := func(sType server.StorageType) func(t *testing.T) {
		return func(t *testing.T) {
			const name = "test-serviceplan"
			client, _, shutdownServer := getFreshApiserverAndClient(t, sType.String(), func() runtime.Object {
				return &servicecatalog.ClusterServicePlan{}
			})
			defer shutdownServer()

			if err := testClusterServicePlanClient(sType, client, name); err != nil {
				t.Fatal(err)
			}
		}
	}
	// TODO: Fix this for CRD.
	// https://github.com/kubernetes-incubator/service-catalog/issues/1256
	//	for _, sType := range storageTypes {
	//		if !t.Run(sType.String(), rootTestFunc(sType)) {
	//			t.Errorf("%q test failed", sType)
	//		}
	//	}
	sType := server.StorageTypeEtcd
	if !t.Run(sType.String(), rootTestFunc(sType)) {
		t.Errorf("%q test failed", sType)
	}
}

func testClusterServicePlanClient(sType server.StorageType, client servicecatalogclient.Interface, name string) error {
	servicePlanClient := client.Servicecatalog().ClusterServicePlans()

	bindable := true
	servicePlan := &v1beta1.ClusterServicePlan{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1beta1.ClusterServicePlanSpec{
			ClusterServiceBrokerName: "test-broker",
			CommonServicePlanSpec: v1beta1.CommonServicePlanSpec{
				Bindable:     &bindable,
				ExternalName: name,
				ExternalID:   "b8269ab4-7d2d-456d-8c8b-5aab63b321d1",
				Description:  "test description",
			},
			ClusterServiceClassRef: v1beta1.ClusterObjectReference{
				Name: "test-serviceclass",
			},
		},
	}

	// start from scratch
	servicePlans, err := servicePlanClient.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing service plans (%s)", err)
	}
	if servicePlans.Items == nil {
		return fmt.Errorf("Items field should not be set to nil")
	}
	if len(servicePlans.Items) > 0 {
		return fmt.Errorf(
			"servicePlans should not exist on start, had %v servicePlans",
			len(servicePlans.Items),
		)
	}

	servicePlanAtServer, err := servicePlanClient.Create(servicePlan)
	if nil != err {
		return fmt.Errorf("error creating the Serviceplan (%v)", servicePlan)
	}
	if name != servicePlanAtServer.Name {
		return fmt.Errorf(
			"didn't get the same ClusterServicePlan back from the server \n%+v\n%+v",
			servicePlan,
			servicePlanAtServer,
		)
	}

	servicePlans, err = servicePlanClient.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing service plans (%s)", err)
	}
	if 1 != len(servicePlans.Items) {
		return fmt.Errorf("should have exactly one ClusterServicePlan, had %v ClusterServicePlans", len(servicePlans.Items))
	}

	servicePlanAtServer, err = servicePlanClient.Get(name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error listing service plans (%s)", err)
	}
	if servicePlanAtServer.Name != name &&
		servicePlan.ResourceVersion == servicePlanAtServer.ResourceVersion {
		return fmt.Errorf(
			"didn't get the same ClusterServicePlan back from the server \n%+v\n%+v",
			servicePlan,
			servicePlanAtServer,
		)
	}

	// check that the plan is the same from get and list
	servicePlanListed := &servicePlans.Items[0]
	if !reflect.DeepEqual(servicePlanAtServer, servicePlanListed) {
		return fmt.Errorf(
			"Didn't get the same instance from list and get: diff: %v",
			diff.ObjectReflectDiff(servicePlanAtServer, servicePlanListed),
		)
	}

	bindable = false
	servicePlanAtServer.Spec.Bindable = &bindable
	_, err = servicePlanClient.Update(servicePlanAtServer)
	if err != nil {
		return fmt.Errorf("Error updating servicePlan: %v", err)
	}
	updated, err := servicePlanClient.Get(name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Error getting servicePlan: %v", err)
	}
	if *updated.Spec.Bindable {
		return errors.New("Failed to update service class")
	}

	// Test status subresource
	updated.Status.RemovedFromBrokerCatalog = true
	updated, err = servicePlanClient.UpdateStatus(updated)
	if err != nil {
		return fmt.Errorf("Error updating servicePlan status: %v", err)
	}
	if !updated.Status.RemovedFromBrokerCatalog {
		return errors.New("Expected status.removedFromBrokerCatalog = true, got false")
	}

	// Verify that field selectors work by listing.
	sp2Name := name + "2"
	sp2ID := "anotheridhere"
	servicePlan2 := &v1beta1.ClusterServicePlan{
		ObjectMeta: metav1.ObjectMeta{Name: sp2Name},
		Spec: v1beta1.ClusterServicePlanSpec{
			ClusterServiceBrokerName: "test-broker",
			CommonServicePlanSpec: v1beta1.CommonServicePlanSpec{
				Bindable:     &bindable,
				ExternalName: sp2Name,
				ExternalID:   sp2ID,
				Description:  "test description 2",
			},
			ClusterServiceClassRef: v1beta1.ClusterObjectReference{
				Name: "test-serviceclass",
			},
		},
	}
	_, err = servicePlanClient.Create(servicePlan2)
	if nil != err {
		return fmt.Errorf("error creating the second Serviceplan (%v)", servicePlan2)
	}

	servicePlans, err = servicePlanClient.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing service plans (%s)", err)
	}
	if 2 != len(servicePlans.Items) {
		return fmt.Errorf("should have two ClusterServicePlans, had %v ClusterServicePlans", len(servicePlans.Items))
	}

	servicePlans, err = servicePlanClient.List(metav1.ListOptions{FieldSelector: "spec.externalName==" + sp2Name})
	if err != nil {
		return fmt.Errorf("error listing service plans (%s)", err)
	}
	if 1 != len(servicePlans.Items) {
		return fmt.Errorf("should have one ClusterServicePlan, had %v ClusterServicePlans : %+v", len(servicePlans.Items), servicePlans.Items)
	}

	if servicePlans.Items[0].Spec.ExternalID != sp2ID {
		return fmt.Errorf("should have same externalID: %q, got %q", sp2ID, servicePlans.Items[0].Spec.ExternalID)
	}

	servicePlans, err = servicePlanClient.List(metav1.ListOptions{FieldSelector: "spec.externalID==" + "b8269ab4-7d2d-456d-8c8b-5aab63b321d1"})
	if err != nil {
		return fmt.Errorf("error listing service plans (%s)", err)
	}
	if 1 != len(servicePlans.Items) {
		return fmt.Errorf("should have one ClusterServicePlan, had %v ClusterServicePlans : %+v", len(servicePlans.Items), servicePlans.Items)
	}

	if servicePlans.Items[0].Spec.ExternalName != name {
		return fmt.Errorf("should have same externalName: %q, got %q", name, servicePlans.Items[0].Spec.ExternalName)
	}

	servicePlans, err = servicePlanClient.List(metav1.ListOptions{FieldSelector: "spec.externalName==" + "crap"})
	if err != nil {
		return fmt.Errorf("error listing service plans (%s)", err)
	}
	if 0 != len(servicePlans.Items) {
		return fmt.Errorf("should have zero ClusterServicePlans, had %v ClusterServicePlans : %+v", len(servicePlans.Items), servicePlans.Items)
	}

	servicePlans, err = servicePlanClient.List(metav1.ListOptions{FieldSelector: "spec.clusterServiceBrokerName=" + "test-broker"})
	if err != nil {
		return fmt.Errorf("error listing service plans (%s)", err)
	}
	if 2 != len(servicePlans.Items) {
		return fmt.Errorf("should have two ClusterServicePlans, had %v ClusterServicePlans : %+v", len(servicePlans.Items), servicePlans.Items)
	}

	err = servicePlanClient.Delete(name, &metav1.DeleteOptions{})
	if nil != err {
		return fmt.Errorf("serviceplan should be deleted (%s)", err)
	}

	servicePlanDeleted, err := servicePlanClient.Get(name, metav1.GetOptions{})
	if nil == err {
		return fmt.Errorf("serviceplan should be deleted (%v)", servicePlanDeleted)
	}

	err = servicePlanClient.Delete(sp2Name, &metav1.DeleteOptions{})
	if nil != err {
		return fmt.Errorf("serviceplan should be deleted (%s)", err)
	}

	return nil
}

// TestInstanceClient exercises the Instance client.
func TestInstanceClient(t *testing.T) {
	rootTestFunc := func(sType server.StorageType) func(t *testing.T) {
		return func(t *testing.T) {
			const name = "test-instance"
			client, _, shutdownServer := getFreshApiserverAndClient(t, sType.String(), func() runtime.Object {
				return &servicecatalog.ServiceInstance{}
			})
			defer shutdownServer()
			if err := testInstanceClient(sType, client, name); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, sType := range storageTypes {
		if !t.Run(sType.String(), rootTestFunc(sType)) {
			t.Errorf("%q test failed", sType)
		}
	}
}

func testInstanceClient(sType server.StorageType, client servicecatalogclient.Interface, name string) error {
	const (
		osbGUID      = "9737b6ed-ca95-4439-8219-c53fcad118ab"
		dashboardURL = "http://test-dashboard.example.com"
	)
	instanceClient := client.Servicecatalog().ServiceInstances("test-namespace")

	instance := &v1beta1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1beta1.ServiceInstanceSpec{
			PlanReference: v1beta1.PlanReference{
				ClusterServiceClassExternalName: "service-class-name",
				ClusterServicePlanExternalName:  "plan-name",
			},
			ClusterServiceClassRef: &v1beta1.ClusterObjectReference{
				Name: "test-serviceclass",
			},
			ClusterServicePlanRef: &v1beta1.ClusterObjectReference{
				Name: "test-serviceplan",
			},
			Parameters: &runtime.RawExtension{Raw: []byte(instanceParameter)},
			ExternalID: osbGUID,
		},
		Status: v1beta1.ServiceInstanceStatus{
			DeprovisionStatus: v1beta1.ServiceInstanceDeprovisionStatusNotRequired,
		},
	}

	// list the instances & expect there to be none
	instances, err := instanceClient.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing instances (%s)", err)
	}
	if instances.Items == nil {
		return fmt.Errorf("Items field should not be set to nil")
	}
	if len(instances.Items) > 0 {
		return fmt.Errorf(
			"instances should not exist on start, had %v instances",
			len(instances.Items),
		)
	}

	instanceServer, err := instanceClient.Create(instance)
	if nil != err {
		return fmt.Errorf("error creating the instance (%#v)", instance)
	}
	if name != instanceServer.Name {
		return fmt.Errorf(
			"didn't get the same instance back from the server \n%+v\n%+v",
			instance,
			instanceServer,
		)
	}

	// list instances again, expect there to be one
	instances, err = instanceClient.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing instances (%s)", err)
	}
	if 1 != len(instances.Items) {
		return fmt.Errorf("should have exactly one instance, had %v instances", len(instances.Items))
	}

	// get the name of the instance that's expected to exist
	instanceServer, err = instanceClient.Get(name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting instance (%s)", err)
	}
	if instanceServer.Name != name &&
		instanceServer.ResourceVersion == instance.ResourceVersion &&
		instanceServer.Spec.ExternalID != osbGUID {
		return fmt.Errorf("didn't get the same instance back from the server \n%+v\n%+v", instance, instanceServer)
	}

	// expect the instance in the list to be the same as the instance just fetched by name
	instanceListed := &instances.Items[0]
	if !reflect.DeepEqual(instanceListed, instanceServer) {
		return fmt.Errorf("Didn't get the same instance from list and get: diff: %v", diff.ObjectReflectDiff(instanceListed, instanceServer))
	}

	// check the parameters of the fetched-by-name instance with what was expected
	parameters := ipStruct{}
	err = json.Unmarshal(instanceServer.Spec.Parameters.Raw, &parameters)
	if err != nil {
		return fmt.Errorf("Couldn't unmarshal returned instance parameters: %v", err)
	}
	if parameters.Bar != "barvalue" {
		return fmt.Errorf("Didn't get back 'barvalue' value for key 'bar' was %+v", parameters)
	}
	if len(parameters.Values) != 2 {
		return fmt.Errorf("Didn't get back 'barvalue' value for key 'bar' was %+v", parameters)
	}
	if parameters.Values["first"] != "firstvalue" {
		return fmt.Errorf("Didn't get back 'firstvalue' value for key 'first' in Values map was %+v", parameters)
	}
	if parameters.Values["second"] != "secondvalue" {
		return fmt.Errorf("Didn't get back 'secondvalue' value for key 'second' in Values map was %+v", parameters)
	}

	// update the instance's conditions, and set the ReconciledGeneration so that
	// spec updates are not rejected
	readyConditionTrue := v1beta1.ServiceInstanceCondition{
		Type:    v1beta1.ServiceInstanceConditionReady,
		Status:  v1beta1.ConditionTrue,
		Reason:  "ConditionReason",
		Message: "ConditionMessage",
	}
	instanceServer.Status = v1beta1.ServiceInstanceStatus{
		ReconciledGeneration: instanceServer.Generation,
		ObservedGeneration:   instanceServer.Generation,
		ProvisionStatus:      v1beta1.ServiceInstanceProvisionStatusProvisioned,
		Conditions:           []v1beta1.ServiceInstanceCondition{readyConditionTrue},
		DeprovisionStatus:    v1beta1.ServiceInstanceDeprovisionStatusNotRequired,
	}

	_, err = instanceClient.UpdateStatus(instanceServer)
	if err != nil {
		return fmt.Errorf("Error updating instance: %v", err)
	}

	// re-fetch the instance by name and check its conditions
	instanceServer, err = instanceClient.Get(name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting instance (%s)", err)
	}
	if e, a := readyConditionTrue, instanceServer.Status.Conditions[0]; !reflect.DeepEqual(e, a) {
		return fmt.Errorf("Didn't get matching ready conditions:\nexpected: %v\n\ngot: %v", e, a)
	}

	// Update the instance references
	classRef := &v1beta1.ClusterObjectReference{Name: "service-class-ref"}
	instanceServer.Spec.ClusterServiceClassRef = classRef
	planRef := &v1beta1.ClusterObjectReference{Name: "service-plan-ref"}
	instanceServer.Spec.ClusterServicePlanRef = planRef
	returnedInstance, err := instanceClient.UpdateReferences(instanceServer)
	if err != nil {
		return fmt.Errorf("Error updating instance references: %v", err)
	}
	oldGeneration := instanceServer.Generation
	// check the returned object we got back from the reference subresource
	if e, a := classRef, returnedInstance.Spec.ClusterServiceClassRef; !reflect.DeepEqual(e, a) {
		return fmt.Errorf("ClusterServiceClassRef was not set correctly, expected: %v got: %v", e, a)
	}
	if e, a := planRef, returnedInstance.Spec.ClusterServicePlanRef; !reflect.DeepEqual(e, a) {
		return fmt.Errorf("ClusterServiceClassRef was not set correctly, expected: %v got: %v", e, a)
	}
	if oldGeneration != returnedInstance.Generation {
		return fmt.Errorf("Generation was changed, expected: %q got: %q", oldGeneration, returnedInstance.Generation)
	}

	// re-fetch the instance by name and check that the service class ref was changed
	instanceServer, err = instanceClient.Get(name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting instance (%s)", err)
	}
	if e, a := classRef, instanceServer.Spec.ClusterServiceClassRef; !reflect.DeepEqual(e, a) {
		return fmt.Errorf("ClusterServiceClassRef was not set correctly, expected: %v got: %v", e, a)
	}
	if e, a := planRef, instanceServer.Spec.ClusterServicePlanRef; !reflect.DeepEqual(e, a) {
		return fmt.Errorf("ClusterServiceClassRef was not set correctly, expected: %v got: %v", e, a)
	}
	if oldGeneration != instanceServer.Generation {
		return fmt.Errorf("Generation was changed, expected: %q got: %q", oldGeneration, instanceServer.Generation)
	}

	// field selector tests
	instances, err = instanceClient.List(metav1.ListOptions{FieldSelector: "spec.clusterServiceClassRef.name=should-return-zero"})
	if err != nil {
		return fmt.Errorf("error listing instances: %v", err)
	}

	if 0 != len(instances.Items) {
		return fmt.Errorf("should have exactly zero instances, had %v instances", len(instances.Items))
	}

	instances, err = instanceClient.List(metav1.ListOptions{FieldSelector: "spec.clusterServiceClassRef.name=service-class-ref"})
	if err != nil {
		return fmt.Errorf("error listing instances: %v", err)
	}

	if 1 != len(instances.Items) {
		return fmt.Errorf("should have exactly one instance, had %v instances", len(instances.Items))
	}

	instances, err = instanceClient.List(metav1.ListOptions{FieldSelector: "spec.clusterServicePlanRef.name=service-plan-ref"})
	if err != nil {
		return fmt.Errorf("error listing instances: %v", err)
	}

	if 1 != len(instances.Items) {
		return fmt.Errorf("should have exactly one instance, had %v instances", len(instances.Items))
	}

	// update the instance's spec
	updateRequests := instanceServer.Spec.UpdateRequests + 1
	expectedGeneration := instanceServer.Generation + 1
	instanceServer.Spec.UpdateRequests = updateRequests

	_, err = instanceClient.Update(instanceServer)
	if err != nil {
		return fmt.Errorf("Error updating instance: %v", err)
	}

	// re-fetch the instance by name and check that spec changes were accepted and
	// caused a bump to the generation
	instanceServer, err = instanceClient.Get(name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting instance (%s)", err)
	}
	if e, a := updateRequests, instanceServer.Spec.UpdateRequests; e != a {
		return fmt.Errorf("unexpected UpdateRequests: expected: %v, got %v", e, a)
	}
	if e, a := expectedGeneration, instanceServer.Generation; e != a {
		return fmt.Errorf("unexpected generation: expected %v, got %v", e, a)
	}

	// delete the instance, set its finalizers to nil, update it, then ensure it is actually
	// deleted
	if err := instanceClient.Delete(name, &metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("instance should be deleted (%s)", err)
	}

	instanceDeleted, err := instanceClient.Get(name, metav1.GetOptions{})
	if nil != err {
		return fmt.Errorf("instance should still exist (%v): %v", instanceDeleted, err)
	}

	instanceDeleted.ObjectMeta.Finalizers = nil
	_, err = instanceClient.UpdateStatus(instanceDeleted)
	if nil != err {
		return fmt.Errorf("error updating status (%v): %v", instanceDeleted, err)
	}

	instanceDeleted, err = instanceClient.Get("test-instance", metav1.GetOptions{})
	if nil == err {
		return fmt.Errorf("instance should be deleted (%#v)", instanceDeleted)
	}
	return nil
}

// TestBindingClient exercises the Binding client.
func TestBindingClient(t *testing.T) {
	rootTestFunc := func(sType server.StorageType) func(t *testing.T) {
		return func(t *testing.T) {
			const name = "test-binding"
			client, _, shutdownServer := getFreshApiserverAndClient(t, sType.String(), func() runtime.Object {
				return &servicecatalog.ServiceBinding{}
			})
			defer shutdownServer()

			if err := testBindingClient(sType, client, name); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, sType := range storageTypes {
		if !t.Run(sType.String(), rootTestFunc(sType)) {
			t.Errorf("%q test failed", sType)
		}

	}
}

func testBindingClient(sType server.StorageType, client servicecatalogclient.Interface, name string) error {
	bindingClient := client.Servicecatalog().ServiceBindings("test-namespace")

	binding := &v1beta1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "test-binding"},
		Spec: v1beta1.ServiceBindingSpec{
			ServiceInstanceRef: v1beta1.LocalObjectReference{
				Name: "bar",
			},
			Parameters: &runtime.RawExtension{Raw: []byte(bindingParameter)},
			ExternalID: "UUID-string",
		},
		Status: v1beta1.ServiceBindingStatus{
			UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
		},
	}

	bindings, err := bindingClient.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing bindings (%s)", err)
	}
	if bindings.Items == nil {
		return fmt.Errorf("Items field should not be set to nil")
	}
	if len(bindings.Items) > 0 {
		return fmt.Errorf("bindings should not exist on start, had %v bindings", len(bindings.Items))
	}

	bindingServer, err := bindingClient.Create(binding)
	if nil != err {
		return fmt.Errorf("error creating the binding: %v\n\n%#v", err, binding)
	}
	if name != bindingServer.Name {
		return fmt.Errorf(
			"didn't get the same binding back from the server \n%+v\n%+v",
			binding,
			bindingServer,
		)
	}
	if bindingServer.Spec.SecretName != "test-binding" {
		return fmt.Errorf(
			"didn't get the right secret name back from the server \n%+v\n%+v",
			"test-binding",
			bindingServer.Spec.SecretName,
		)
	}

	bindings, err = bindingClient.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing bindings (%s)", err)
	}
	if 1 != len(bindings.Items) {
		return fmt.Errorf("should have exactly one binding, had %v bindings", len(bindings.Items))
	}

	bindingServer, err = bindingClient.Get(name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting binding (%s)", err)
	}
	if bindingServer.Name != name &&
		bindingServer.ResourceVersion == binding.ResourceVersion {
		return fmt.Errorf(
			"didn't get the same binding back from the server \n%+v\n%+v",
			binding,
			bindingServer,
		)
	}

	bindingListed := &bindings.Items[0]
	if !reflect.DeepEqual(bindingListed, bindingServer) {
		return fmt.Errorf(
			"Didn't get the same binding from list and get: diff: %v",
			diff.ObjectReflectDiff(bindingListed, bindingServer),
		)
	}

	parameters := bpStruct{}
	err = json.Unmarshal(bindingServer.Spec.Parameters.Raw, &parameters)
	if err != nil {
		return fmt.Errorf("Couldn't unmarshal returned parameters: %v", err)
	}
	if parameters.Foo != "bar" {
		return fmt.Errorf("Didn't get back 'bar' value for key 'foo' was %+v", parameters)
	}
	if len(parameters.Baz) != 2 {
		return fmt.Errorf("Didn't get back two values for 'baz' array in parameters was %+v", parameters)
	}
	foundFirst := false
	foundSecond := false
	for _, val := range parameters.Baz {
		if val == "first" {
			foundFirst = true
		}
		if val == "second" {
			foundSecond = true
		}
	}
	if !foundFirst {
		return fmt.Errorf("Didn't find first value in parameters.baz was %+v", parameters)
	}
	if !foundSecond {
		return fmt.Errorf("Didn't find second value in parameters.baz was %+v", parameters)
	}

	readyConditionTrue := v1beta1.ServiceBindingCondition{
		Type:    v1beta1.ServiceBindingConditionReady,
		Status:  v1beta1.ConditionTrue,
		Reason:  "ConditionReason",
		Message: "ConditionMessage",
	}
	bindingServer.Status = v1beta1.ServiceBindingStatus{
		Conditions:   []v1beta1.ServiceBindingCondition{readyConditionTrue},
		UnbindStatus: v1beta1.ServiceBindingUnbindStatusNotRequired,
	}
	if _, err = bindingClient.UpdateStatus(bindingServer); err != nil {
		return fmt.Errorf("Error updating binding: %v", err)
	}
	bindingServer, err = bindingClient.Get(name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting binding: %v", err)
	}
	if e, a := readyConditionTrue, bindingServer.Status.Conditions[0]; !reflect.DeepEqual(e, a) {
		return fmt.Errorf("Didn't get matching ready conditions:\nexpected: %v\n\ngot: %v", e, a)
	}

	if err = bindingClient.Delete(name, &metav1.DeleteOptions{}); nil != err {
		return fmt.Errorf("binding delete failed (%s)", err)
	}

	bindingDeleted, err := bindingClient.Get(name, metav1.GetOptions{})
	if nil != err {
		return fmt.Errorf("binding should still exist on initial get (%s)", err)
	}

	fmt.Printf("-----\nclientset_test\n\nbinding deleted: %#v\n\n", *bindingDeleted)
	bindingDeleted.ObjectMeta.Finalizers = nil
	if _, err := bindingClient.UpdateStatus(bindingDeleted); err != nil {
		return fmt.Errorf("error updating binding status (%s)", err)
	}

	if bindingDeleted, err := bindingClient.Get(name, metav1.GetOptions{}); err == nil {
		return fmt.Errorf(
			"binding should be deleted after finalizers cleared. got binding %#v",
			*bindingDeleted,
		)
	}
	return nil
}

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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// avoid error `servicecatalog/v1beta1 is not enabled`
	_ "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/install"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"github.com/kubernetes-incubator/service-catalog/test/util"
)

func TestClusterServicePlanRemovedFromCatalogWithoutInstances(t *testing.T) {
	ct := &controllerTest{
		t:      t,
		broker: getTestBroker(),
	}

	ct.run(func(ct *controllerTest) {
		removedPlan := getTestClusterServicePlanRemoved()
		removedPlan, err := ct.client.ClusterServicePlans().Create(removedPlan)
		if err != nil {
			t.Fatalf("error creating ClusterServicePlan: %v", err)
		}

		err = util.WaitForClusterServicePlanToExist(ct.client, testRemovedClusterServicePlanGUID)
		if err != nil {
			t.Fatalf("error waiting for ClusterServicePlan to exist: %v", err)
		}

		t.Log("updating ClusterServiceClass status")
		removedPlan.Status.RemovedFromBrokerCatalog = true
		_, err = ct.client.ClusterServicePlans().UpdateStatus(removedPlan)
		if err != nil {
			t.Fatalf("error marking ClusterServicePlan as removed from catalog: %v", err)
		}

		err = util.WaitForClusterServicePlanToNotExist(ct.client, testRemovedClusterServicePlanGUID)
		if err != nil {
			t.Fatalf("error waiting for remove ClusterServicePlan to not exist: %v", err)
		}
	})
}

const (
	testRemovedClusterServicePlanGUID          = "removed-plan"
	testRemovedClusterServicePlanExternalName  = "removed-plan-name"
	testRemovedClusterServiceClassGUID         = "removed-class"
	testRemovedClusterServiceClassExternalName = "removed-class-name"
)

func getTestClusterServicePlanRemoved() *v1beta1.ClusterServicePlan {
	return &v1beta1.ClusterServicePlan{
		ObjectMeta: metav1.ObjectMeta{Name: testRemovedClusterServicePlanGUID},
		Spec: v1beta1.ClusterServicePlanSpec{
			ClusterServiceBrokerName: testClusterServiceBrokerName,
			ExternalID:               testRemovedClusterServicePlanGUID,
			ExternalName:             testRemovedClusterServicePlanExternalName,
			Description:              "a plan that will be removed",
			Bindable:                 truePtr(),
			ClusterServiceClassRef: v1beta1.ClusterObjectReference{
				Name: testClusterServiceClassGUID,
			},
		},
		Status: v1beta1.ClusterServicePlanStatus{},
	}
}

func TestClusterServiceClassRemovedFromCatalogWithoutInstances(t *testing.T) {
	ct := &controllerTest{
		t:      t,
		broker: getTestBroker(),
	}

	ct.run(func(ct *controllerTest) {
		removedClass := getTestClusterServiceClassRemoved()
		removedClass, err := ct.client.ClusterServiceClasses().Create(removedClass)
		if err != nil {
			t.Fatalf("error creating ClusterServiceClass: %v", err)
		}

		err = util.WaitForClusterServiceClassToExist(ct.client, testRemovedClusterServiceClassGUID)
		if err != nil {
			t.Fatalf("error waiting for ClusterServiceClass to exist: %v", err)
		}

		t.Log("updating ClusterServiceClass status")
		removedClass.Status.RemovedFromBrokerCatalog = true
		_, err = ct.client.ClusterServiceClasses().UpdateStatus(removedClass)
		if err != nil {
			t.Fatalf("error marking ClusterServiceClass as removed from catalog: %v", err)
		}

		err = util.WaitForClusterServiceClassToNotExist(ct.client, testRemovedClusterServiceClassGUID)
		if err != nil {
			t.Fatalf("error waiting for remove ClusterServiceClass to not exist: %v", err)
		}
	})
}

func getTestClusterServiceClassRemoved() *v1beta1.ClusterServiceClass {
	return &v1beta1.ClusterServiceClass{
		ObjectMeta: metav1.ObjectMeta{Name: testRemovedClusterServiceClassGUID},
		Spec: v1beta1.ClusterServiceClassSpec{
			ClusterServiceBrokerName: testClusterServiceBrokerName,
			ExternalID:               testRemovedClusterServiceClassGUID,
			ExternalName:             testRemovedClusterServiceClassExternalName,
			Description:              "a serviceclass that will be removed",
			Bindable:                 true,
		},
		Status: v1beta1.ClusterServiceClassStatus{},
	}
}

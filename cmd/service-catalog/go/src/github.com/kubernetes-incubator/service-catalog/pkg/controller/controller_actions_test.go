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
	"reflect"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	fakeosb "github.com/pmorie/go-open-service-broker-client/v2/fake"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/testing"
)

type kubeClientAction struct {
	verb         string
	resourceName string
	checkType    func(testing.Action) error
}

// checkGetActionType can be used as a param for kubeClientAction.checkType. It's intended
// to ensure an action is a testing.GetAction
func checkGetActionType(a testing.Action) error {
	if _, ok := a.(testing.GetAction); !ok {
		return fmt.Errorf("expected a GetAction, got %s", reflect.TypeOf(a))
	}
	return nil
}

// checkUpdateActionType can be used as a param for kubeClientAction.checkType. It's intended
// to ensure an action is a testing.UpdateAction
func checkUpdateActionType(a testing.Action) error {
	if _, ok := a.(testing.UpdateAction); !ok {
		return fmt.Errorf("expected an UpdateAction, got %s", reflect.TypeOf(a))
	}
	return nil
}

type catalogClientAction struct {
	verb             string
	getRuntimeObject func(testing.Action) (runtime.Object, error)
	checkObject      func(runtime.Object) error
}

// getRuntimeObjectFromUpdate asserts that t is a testing.UpdateAction, then returns the return value
// of calling GetObject on the UpdateAction
func getRuntimeObjectFromUpdateAction(t testing.Action) (runtime.Object, error) {
	up, ok := t.(testing.UpdateAction)
	if !ok {
		return nil, fmt.Errorf("action was not a testing.UpdateAction")
	}
	return up.GetObject(), nil
}

// checkServiceInstance can be used as a param to catalogClientAction.checkObject. It's intended
// to check that a runtime.Object is an instance, and to check some properties of that instance
// including:
//
// - the Name
// - the conditions
func checkServiceInstance(descr instanceDescription) func(runtime.Object) error {
	return func(obj runtime.Object) error {
		inst, ok := obj.(*v1alpha1.ServiceInstance)
		if !ok {
			return fmt.Errorf("expected an instance, got a %s", reflect.TypeOf(obj))
		}
		if inst.Name != descr.name {
			return fmt.Errorf("expected instance name %s, got %s", descr.name, inst.Name)
		}
		if len(descr.conditionReasons) != len(inst.Status.Conditions) {
			return fmt.Errorf(
				"expected %d conditions, got %d",
				len(descr.conditionReasons),
				len(inst.Status.Conditions),
			)
		}
		for i, expectedConditionReason := range descr.conditionReasons {
			actualCondition := inst.Status.Conditions[i]
			if expectedConditionReason != actualCondition.Reason {
				return fmt.Errorf(
					"condition %d: expected condition reason %s, got %s",
					i,
					expectedConditionReason,
					actualCondition.Reason,
				)
			}
		}
		return nil
	}
}

// instanceDescription is the description of an instance that will be checked in the function
// returned by checkServiceInstance
type instanceDescription struct {
	name             string
	conditionReasons []string
}

// checkKubeClientActions is the utility function for checking actions returned by the generic
// kubernetes client
func checkKubeClientActions(actual []testing.Action, expected []kubeClientAction) error {
	if len(actual) != len(expected) {
		return fmt.Errorf("expected %d kube client actions, got %d", len(expected), len(actual))
	}
	for i, actualAction := range actual {
		expectedAction := expected[i]
		if actualAction.GetVerb() != expectedAction.verb {
			return fmt.Errorf(
				"action %d: expected verb '%s', got '%s'",
				i,
				expectedAction.verb,
				actualAction.GetVerb(),
			)
		}
		getAction, ok := actualAction.(testing.GetAction)
		if !ok {
			return fmt.Errorf(
				"action %d: expected a GetAction, got %s",
				i,
				reflect.TypeOf(actualAction),
			)
		}
		if expectedAction.resourceName != getAction.GetResource().Resource {
			return fmt.Errorf(
				"expected resource name '%s', got '%s'",
				expectedAction.resourceName,
				getAction.GetResource().Resource,
			)
		}
	}
	return nil
}

// checkCatalogClientActions is the utility function for checking actions returned by
// the catalog client
func checkCatalogClientActions(actual []testing.Action, expected []catalogClientAction) error {
	if len(actual) != len(expected) {
		return fmt.Errorf("expected %d actions, got %d", len(expected), len(actual))
	}
	for i, actualAction := range actual {
		expectedAction := expected[i]

		if actualAction.GetVerb() != expectedAction.verb {
			return fmt.Errorf("action %d: expected verb %s, got %s", i, expectedAction.verb, actualAction.GetVerb())
		}

		obj, err := expectedAction.getRuntimeObject(actualAction)
		if err != nil {
			return fmt.Errorf("action %d: %s", i, err)
		}
		if err := expectedAction.checkObject(obj); err != nil {
			return fmt.Errorf("action %d: %s", i, err)
		}
	}
	return nil
}

type brokerClientAction struct {
	actionType fakeosb.ActionType
}

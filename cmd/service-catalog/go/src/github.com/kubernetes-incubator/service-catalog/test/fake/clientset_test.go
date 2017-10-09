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

package fake

import (
	"testing"

	clientgotesting "k8s.io/client-go/testing"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	servicecatalogclientset "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/fake"
)

func TestClientsetStoresServiceInstanceClone(t *testing.T) {
	clientset := Clientset{&servicecatalogclientset.Clientset{}}
	instance := &v1alpha1.ServiceInstance{}
	instance.Name = "test-instance"
	returnedInstance, err := clientset.ServicecatalogV1alpha1().ServiceInstances("test-namespace").UpdateStatus(instance)
	if err != nil {
		t.Fatalf("unexpected error from UpdateStatus: %v", err)
	}

	actions := clientset.Actions()
	if e, a := 1, len(actions); e != a {
		t.Fatalf("unexpected number of actions: expected %v, got %v", e, a)
	}
	action := actions[0]

	updateAction, ok := actions[0].(clientgotesting.UpdateAction)
	if !ok {
		t.Fatalf("unexpected action type; failed to convert action %+v to UpdateAction", action)
	}

	storedObject := updateAction.GetObject()
	storedInstance, ok := storedObject.(*v1alpha1.ServiceInstance)
	if !ok {
		t.Fatalf("unexpected object in action; failed to convert action object %+v to ServiceInstance", storedObject)
	}

	if e, a := instance, storedInstance; e == a {
		t.Fatalf("expected stored instance to not be the same object as original instance: original = %v, stored = %v", e, a)
	}
	if e, a := returnedInstance, storedInstance; e == a {
		t.Fatalf("expected stored instance to not be the same object as returned instance, returned = %v, stored = %v", e, a)
	}
	if e, a := instance.Name, storedInstance.Name; e != a {
		t.Fatalf("unexpected name: expected %v, got %v", e, a)
	}
}

func TestClientsetStoresServiceInstanceCredentialClone(t *testing.T) {
	clientset := Clientset{&servicecatalogclientset.Clientset{}}
	binding := &v1alpha1.ServiceInstanceCredential{}
	binding.Name = "test-instance"
	returnedBinding, err := clientset.ServicecatalogV1alpha1().ServiceInstanceCredentials("test-namespace").UpdateStatus(binding)
	if err != nil {
		t.Fatalf("unexpected error from UpdateStatus: %v", err)
	}

	actions := clientset.Actions()
	if e, a := 1, len(actions); e != a {
		t.Fatalf("unexpected number of actions: expected %v, got %v", e, a)
	}
	action := actions[0]

	updateAction, ok := actions[0].(clientgotesting.UpdateAction)
	if !ok {
		t.Fatalf("unexpected action type; failed to convert action %+v to UpdateAction", action)
	}

	storedObject := updateAction.GetObject()
	storedBinding, ok := storedObject.(*v1alpha1.ServiceInstanceCredential)
	if !ok {
		t.Fatalf("unexpected object in action; failed to convert action object %+v to ServiceInstanceCredential", storedObject)
	}

	if e, a := binding, storedBinding; e == a {
		t.Fatalf("expected stored instance to not be the same object as original binding: original = %v, stored = %v", e, a)
	}
	if e, a := returnedBinding, storedBinding; e == a {
		t.Fatalf("expected stored instance to not be the same object as returned binding, returned = %v, stored = %v", e, a)
	}
	if e, a := binding.Name, storedBinding.Name; e != a {
		t.Fatalf("unexpected name: expected %v, got %v", e, a)
	}
}

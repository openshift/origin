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

package meta

import (
	"testing"

	sc "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testFinalizer = "testfinalizer"
)

func TestGetFinalizers(t *testing.T) {
	obj := &sc.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Finalizers: []string{testFinalizer}},
	}
	finalizers, err := GetFinalizers(obj)
	if err != nil {
		t.Fatal(err)
	}
	if len(finalizers) != 1 {
		t.Fatalf("expected 1 finalizer, got %d", len(finalizers))
	}
	if finalizers[0] != testFinalizer {
		t.Fatalf("expected finalizer %s, got %s", testFinalizer, finalizers[0])
	}
}

func TestAddFinalizer(t *testing.T) {
	obj := &sc.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{},
	}
	if err := AddFinalizer(obj, testFinalizer); err != nil {
		t.Fatal(err)
	}
	if len(obj.Finalizers) != 1 {
		t.Fatalf("expected 1 finalizer, got %d", len(obj.Finalizers))
	}
	if obj.Finalizers[0] != testFinalizer {
		t.Fatalf("expected finalizer %s, got %s", testFinalizer, obj.Finalizers[0])
	}
}

func TestRemoveFinalizer(t *testing.T) {
	obj := &sc.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Finalizers: []string{testFinalizer}},
	}
	newFinalizers, err := RemoveFinalizer(obj, testFinalizer+"-noexist")
	if err != nil {
		t.Fatalf("error removing non-existent finalizer (%s)", err)
	}
	if len(newFinalizers) != 1 {
		t.Fatalf("number of returned finalizers wasn't 1")
	}
	if len(obj.Finalizers) != 1 {
		t.Fatalf("finalizer was removed when it shouldn't have been")
	}
	if obj.Finalizers[0] != testFinalizer {
		t.Fatalf("expected finalizer %s, got %s", testFinalizer, obj.Finalizers[0])
	}
	newFinalizers, err = RemoveFinalizer(obj, testFinalizer)
	if err != nil {
		t.Fatalf("error removing existent finalizer (%s)", err)
	}
	if len(newFinalizers) != 0 {
		t.Fatalf("number of returned finalizers wasn't 0")
	}
	if len(obj.Finalizers) != 0 {
		t.Fatalf("expected no finalizers, got %d", len(obj.Finalizers))
	}
}

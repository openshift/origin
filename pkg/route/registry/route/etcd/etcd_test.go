/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

package etcd

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/registry/registrytest"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/tools"

	_ "github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/route/registry/route"
)

func newStorage(t *testing.T, allocator *testAllocator) (*REST, *tools.FakeEtcdClient) {
	etcdStorage, fakeClient := registrytest.NewEtcdStorage(t, "")
	return NewREST(etcdStorage, allocator).Route, fakeClient
}

func validNewRoute(name string) *api.Route {
	return &api.Route{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		Spec: api.RouteSpec{
			To: kapi.ObjectReference{
				Name: "test",
			},
		},
	}
}

func TestCreate(t *testing.T) {
	storage, fakeClient := newStorage(t, &testAllocator{})
	test := registrytest.New(t, fakeClient, storage.Etcd)
	validRoute := validNewRoute("foo")
	test.TestCreate(
		// valid
		validRoute,
		// invalid
		&api.Route{
			ObjectMeta: kapi.ObjectMeta{Name: "_-a123-a_"},
		},
	)
}

type testAllocator struct {
	Hostname string
	Err      error
	Allocate bool
	Generate bool
}

func (a *testAllocator) AllocateRouterShard(*api.Route) (*api.RouterShard, error) {
	a.Allocate = true
	return nil, a.Err
}
func (a *testAllocator) GenerateHostname(*api.Route, *api.RouterShard) string {
	a.Generate = true
	return a.Hostname
}

func TestCreateWithAllocation(t *testing.T) {
	allocator := &testAllocator{Hostname: "bar"}
	storage, _ := newStorage(t, allocator)

	validRoute := validNewRoute("foo")
	obj, err := storage.Create(kapi.NewDefaultContext(), validRoute)
	if err != nil {
		t.Fatalf("unable to create object: %v", err)
	}
	result := obj.(*api.Route)
	if result.Spec.Host != "bar" {
		t.Fatalf("unexpected route: %#v", result)
	}
	if v, ok := result.Annotations[route.HostGeneratedAnnotationKey]; !ok || v != "true" {
		t.Fatalf("unexpected route: %#v", result)
	}
	if !allocator.Allocate || !allocator.Generate {
		t.Fatalf("unexpected allocator: %#v", allocator)
	}
}

func TestUpdate(t *testing.T) {
	storage, fakeClient := newStorage(t, nil)
	test := registrytest.New(t, fakeClient, storage.Etcd)

	test.TestUpdate(
		validNewRoute("foo"),
		// valid update
		func(obj runtime.Object) runtime.Object {
			object := obj.(*api.Route)
			if object.Annotations == nil {
				object.Annotations = map[string]string{}
			}
			object.Annotations["updated"] = "true"
			return object
		},
		// invalid update
		func(obj runtime.Object) runtime.Object {
			object := obj.(*api.Route)
			object.Spec.Path = "invalid/path"
			return object
		},
	)
}

func TestDelete(t *testing.T) {
	storage, fakeClient := newStorage(t, nil)
	test := registrytest.New(t, fakeClient, storage.Etcd)
	test.TestDelete(validNewRoute("foo"))
}

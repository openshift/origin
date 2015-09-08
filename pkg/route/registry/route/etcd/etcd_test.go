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

	"github.com/coreos/go-etcd/etcd"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest/resttest"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
	etcdstorage "k8s.io/kubernetes/pkg/storage/etcd"
	"k8s.io/kubernetes/pkg/tools"
	"k8s.io/kubernetes/pkg/tools/etcdtest"

	_ "github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/route/registry/route"
)

func newHelper(t *testing.T) (*tools.FakeEtcdClient, storage.Interface) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	helper := etcdstorage.NewEtcdStorage(fakeClient, testapi.Codec(), etcdtest.PathPrefix())
	return fakeClient, helper
}

func validNewRoute(name string) *api.Route {
	return &api.Route{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		ServiceName: "test",
	}
}

func TestCreate(t *testing.T) {
	fakeClient, helper := newHelper(t)
	storage := NewREST(helper, nil)
	test := resttest.New(t, storage, fakeClient.SetError)
	validRoute := validNewRoute("foo")
	test.TestCreate(
		// valid
		validRoute,
		// invalid
		&api.Route{
			ObjectMeta: kapi.ObjectMeta{Name: "_-a123-a_"},
		},
		// no service
		&api.Route{
			ObjectMeta: kapi.ObjectMeta{Name: "test"},
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
	_, helper := newHelper(t)
	allocator := &testAllocator{Hostname: "bar"}
	storage := NewREST(helper, allocator)

	validRoute := validNewRoute("foo")
	obj, err := storage.Create(kapi.NewDefaultContext(), validRoute)
	if err != nil {
		t.Fatalf("unable to create object: %v", err)
	}
	result := obj.(*api.Route)
	if result.Host != "bar" {
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
	fakeClient, helper := newHelper(t)
	storage := NewREST(helper, nil)
	test := resttest.New(t, storage, fakeClient.SetError)
	key, err := storage.KeyFunc(test.TestContext(), "foo")
	if err != nil {
		t.Fatal(err)
	}
	key = etcdtest.AddPrefix(key)

	fakeClient.ExpectNotFoundGet(key)
	fakeClient.ChangeIndex = 2
	route := validNewRoute("foo")
	route.Namespace = test.TestNamespace()
	existing := validNewRoute("exists")
	existing.Namespace = test.TestNamespace()
	obj, err := storage.Create(test.TestContext(), existing)
	if err != nil {
		t.Fatalf("unable to create object: %v", err)
	}
	older := obj.(*api.Route)
	older.ResourceVersion = "1"

	test.TestUpdate(
		route,
		existing,
		older,
	)
}

func TestDelete(t *testing.T) {
	fakeClient, helper := newHelper(t)
	storage := NewREST(helper, nil)
	test := resttest.New(t, storage, fakeClient.SetError)

	ctx := kapi.NewDefaultContext()
	validRoute := validNewRoute("test")
	validRoute.Namespace = kapi.NamespaceDefault

	key, _ := storage.KeyFunc(ctx, validRoute.Name)
	key = etcdtest.AddPrefix(key)

	createFn := func() runtime.Object {
		obj := validRoute
		obj.ResourceVersion = "1"
		fakeClient.Data[key] = tools.EtcdResponseWithError{
			R: &etcd.Response{
				Node: &etcd.Node{
					Value:         runtime.EncodeOrDie(testapi.Codec(), obj),
					ModifiedIndex: 1,
				},
			},
		}
		return obj
	}
	gracefulSetFn := func() bool {
		// If the controller is still around after trying to delete either the delete
		// failed, or we're deleting it gracefully.
		if fakeClient.Data[key].R.Node != nil {
			return true
		}
		return false
	}

	test.TestDelete(createFn, gracefulSetFn)
}

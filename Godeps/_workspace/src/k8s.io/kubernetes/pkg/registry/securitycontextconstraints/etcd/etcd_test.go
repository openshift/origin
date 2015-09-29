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

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/registry/registrytest"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/tools"
)

func newStorage(t *testing.T) (*REST, *tools.FakeEtcdClient) {
	etcdStorage, fakeClient := registrytest.NewEtcdStorage(t, "")
	return NewStorage(etcdStorage), fakeClient
}

func validNewSecurityContextConstraints(name string) *api.SecurityContextConstraints {
	return &api.SecurityContextConstraints{
		ObjectMeta: api.ObjectMeta{
			Name: name,
		},
		SELinuxContext: api.SELinuxContextStrategyOptions{
			Type: api.SELinuxStrategyRunAsAny,
		},
		RunAsUser: api.RunAsUserStrategyOptions{
			Type: api.RunAsUserStrategyRunAsAny,
		},
	}
}

func TestCreate(t *testing.T) {
	storage, fakeClient := newStorage(t)
	test := registrytest.New(t, fakeClient, storage.Etcd).ClusterScope()
	scc := validNewSecurityContextConstraints("foo")
	scc.ObjectMeta = api.ObjectMeta{GenerateName: "foo-"}
	test.TestCreate(
		// valid
		scc,
		// invalid
		&api.SecurityContextConstraints{
			ObjectMeta: api.ObjectMeta{Name: "name with spaces"},
		},
	)
}

func TestUpdate(t *testing.T) {
	storage, fakeEtcdClient := newStorage(t)
	test := registrytest.New(t, fakeEtcdClient, storage.Etcd).ClusterScope()
	test.TestUpdate(
		validNewSecurityContextConstraints("foo"),
		// updateFunc
		func(obj runtime.Object) runtime.Object {
			object := obj.(*api.SecurityContextConstraints)
			object.AllowPrivilegedContainer = true
			return object
		},
	)
}

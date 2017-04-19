/*
Copyright 2014 The Kubernetes Authors.

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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	etcdtesting "k8s.io/apiserver/pkg/storage/etcd/testing"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/registry/registrytest"
)

func newStorage(t *testing.T) (*REST, *etcdtesting.EtcdTestServer) {
	etcdStorage, server := registrytest.NewEtcdStorage(t, "")
	restOptions := generic.RESTOptions{
		StorageConfig:           etcdStorage,
		Decorator:               generic.UndecoratedStorage,
		DeleteCollectionWorkers: 1,
		ResourcePrefix:          "securitycontextconstraints",
	}
	return NewREST(restOptions), server
}

func validNewSecurityContextConstraints(name string) *api.SecurityContextConstraints {
	return &api.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		SELinuxContext: api.SELinuxContextStrategyOptions{
			Type: api.SELinuxStrategyRunAsAny,
		},
		RunAsUser: api.RunAsUserStrategyOptions{
			Type: api.RunAsUserStrategyRunAsAny,
		},
		FSGroup: api.FSGroupStrategyOptions{
			Type: api.FSGroupStrategyRunAsAny,
		},
		SupplementalGroups: api.SupplementalGroupsStrategyOptions{
			Type: api.SupplementalGroupsStrategyRunAsAny,
		},
	}
}

func TestCreate(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Store).ClusterScope()
	scc := validNewSecurityContextConstraints("foo")
	scc.ObjectMeta = metav1.ObjectMeta{GenerateName: "foo-"}
	test.TestCreate(
		// valid
		scc,
		// invalid
		&api.SecurityContextConstraints{
			ObjectMeta: metav1.ObjectMeta{Name: "name with spaces"},
		},
	)
}

func TestUpdate(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Store).ClusterScope()
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

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

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest/resttest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/testapi"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/storage"
	etcdstorage "github.com/GoogleCloudPlatform/kubernetes/pkg/storage/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools/etcdtest"
)

func newEtcdStorage(t *testing.T) (*tools.FakeEtcdClient, storage.Interface) {
	fakeEtcdClient := tools.NewFakeEtcdClient(t)
	fakeEtcdClient.TestIndex = true
	etcdStorage := etcdstorage.NewEtcdStorage(fakeEtcdClient, testapi.Codec(), etcdtest.PathPrefix())
	return fakeEtcdClient, etcdStorage
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
	fakeEtcdClient, etcdStorage := newEtcdStorage(t)
	storage := NewStorage(etcdStorage)
	test := resttest.New(t, storage, fakeEtcdClient.SetError).ClusterScope()
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
	fakeEtcdClient, etcdStorage := newEtcdStorage(t)
	storage := NewStorage(etcdStorage)
	test := resttest.New(t, storage, fakeEtcdClient.SetError).ClusterScope()
	key := etcdtest.AddPrefix("securitycontextconstraints/foo")

	fakeEtcdClient.ExpectNotFoundGet(key)
	fakeEtcdClient.ChangeIndex = 2
	scc := validNewSecurityContextConstraints("foo")
	existing := validNewSecurityContextConstraints("exists")
	obj, err := storage.Create(api.NewDefaultContext(), existing)
	if err != nil {
		t.Fatalf("unable to create object: %v", err)
	}
	older := obj.(*api.SecurityContextConstraints)
	older.ResourceVersion = "1"

	test.TestUpdate(
		scc,
		existing,
		older,
	)
}

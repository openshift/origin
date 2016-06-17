package etcd

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/registrytest"
	etcdtesting "k8s.io/kubernetes/pkg/storage/etcd/testing"

	"github.com/openshift/origin/pkg/template/api"
	_ "github.com/openshift/origin/pkg/template/api/install"
	"github.com/openshift/origin/pkg/util/restoptions"
)

func newStorage(t *testing.T) (*REST, *etcdtesting.EtcdTestServer) {
	etcdStorage, server := registrytest.NewEtcdStorage(t, "")
	storage, err := NewREST(restoptions.NewSimpleGetter(etcdStorage))
	if err != nil {
		t.Fatal(err)
	}
	return storage, server
}

func validTemplate() *api.Template {
	return &api.Template{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
	}
}

func TestCreate(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Store)
	valid := validTemplate()
	valid.Name = ""
	valid.GenerateName = "test-"
	test.TestCreate(
		valid,
		// invalid
		&api.Template{},
	)
}

func TestList(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Store)
	test.TestList(
		validTemplate(),
	)
}

func TestGet(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Store)
	test.TestGet(
		validTemplate(),
	)
}

func TestDelete(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Store).ReturnDeletedObject()
	test.TestDelete(
		validTemplate(),
	)
}

func TestWatch(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Store)

	valid := validTemplate()
	valid.Name = "foo"
	valid.Labels = map[string]string{"foo": "bar"}

	test.TestWatch(
		valid,
		// matching labels
		[]labels.Set{{"foo": "bar"}},
		// not matching labels
		[]labels.Set{{"foo": "baz"}},
		// matching fields
		[]fields.Set{
			{"metadata.name": "foo"},
		},
		// not matching fields
		[]fields.Set{
			{"metadata.name": "bar"},
		},
	)
}

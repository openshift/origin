package etcd

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/registrytest"
	etcdtesting "k8s.io/kubernetes/pkg/storage/etcd/testing"

	"github.com/openshift/origin/pkg/build/api"
	_ "github.com/openshift/origin/pkg/build/api/install"
	"github.com/openshift/origin/pkg/build/registry/build"
	"github.com/openshift/origin/pkg/util/restoptions"
)

func newStorage(t *testing.T) (*REST, *etcdtesting.EtcdTestServer) {
	etcdStorage, server := registrytest.NewEtcdStorage(t, "")
	storage, _, err := NewREST(restoptions.NewSimpleGetter(etcdStorage))
	if err != nil {
		t.Fatal(err)
	}
	return storage, server
}

func TestStorage(t *testing.T) {
	storage, _ := newStorage(t)
	build.NewRegistry(storage)
}

func validBuild() *api.Build {
	return &api.Build{
		ObjectMeta: kapi.ObjectMeta{Name: "buildid"},
		Spec: api.BuildSpec{
			CommonSpec: api.CommonSpec{
				Source: api.BuildSource{
					Git: &api.GitBuildSource{
						URI: "http://github.com/my/repository",
					},
				},
				Strategy: api.BuildStrategy{
					DockerStrategy: &api.DockerBuildStrategy{},
				},
				Output: api.BuildOutput{
					To: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "repository/data",
					},
				},
			},
		},
	}
}

func TestCreate(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Store)
	valid := validBuild()
	valid.Name = ""
	valid.GenerateName = "test-"
	test.TestCreate(
		valid,
		// invalid
		&api.Build{},
	)
}

func TestList(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Store)
	test.TestList(
		validBuild(),
	)
}

func TestGet(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Store)
	test.TestGet(
		validBuild(),
	)
}

func TestDelete(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Store)
	test.TestDelete(
		validBuild(),
	)
}

func TestWatch(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Store)

	valid := validBuild()
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

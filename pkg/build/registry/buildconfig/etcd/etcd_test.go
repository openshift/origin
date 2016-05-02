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
	"github.com/openshift/origin/pkg/build/registry/buildconfig"
)

func newStorage(t *testing.T) (*REST, *etcdtesting.EtcdTestServer) {
	etcdStorage, server := registrytest.NewEtcdStorage(t, "")
	storage := NewREST(etcdStorage)
	return storage, server
}

func TestStorage(t *testing.T) {
	storage, _ := newStorage(t)
	buildconfig.NewRegistry(storage)
}

func validBuildConfig() *api.BuildConfig {
	return &api.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "configid"},
		Spec: api.BuildConfigSpec{
			RunPolicy: api.BuildRunPolicySerial,
			BuildSpec: api.BuildSpec{
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
	test := registrytest.New(t, storage.Etcd)
	valid := validBuildConfig()
	valid.Name = ""
	valid.GenerateName = "test-"
	test.TestCreate(
		valid,
		// invalid
		&api.BuildConfig{},
	)
}

func TestList(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Etcd)
	test.TestList(
		validBuildConfig(),
	)
}

func TestGet(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Etcd)
	test.TestGet(
		validBuildConfig(),
	)
}

func TestDelete(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Etcd)
	test.TestDelete(
		validBuildConfig(),
	)
}

func TestWatch(t *testing.T) {
	storage, server := newStorage(t)
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Etcd)

	valid := validBuildConfig()
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

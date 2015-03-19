package etcd

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest/resttest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/template/api"
)

func newHelper(t *testing.T) (*tools.FakeEtcdClient, tools.EtcdHelper) {
	fakeEtcdClient := tools.NewFakeEtcdClient(t)
	fakeEtcdClient.TestIndex = true
	helper := tools.NewEtcdHelper(fakeEtcdClient, latest.Codec)
	return fakeEtcdClient, helper
}

func validNew() *api.Template {
	return &api.Template{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "foo",
			Namespace: kapi.NamespaceDefault,
		},
	}
}

func validChanged() *api.Template {
	template := validNew()
	template.ResourceVersion = "1"
	template.Labels = map[string]string{
		"foo": "bar",
	}
	return template
}

func TestStorage(t *testing.T) {
	_, helper := newHelper(t)
	storage := NewREST(helper)
	var _ apiserver.RESTCreater = storage
	var _ apiserver.RESTLister = storage
	var _ apiserver.RESTDeleter = storage
	var _ apiserver.RESTUpdater = storage
	var _ apiserver.RESTGetter = storage
}

func TestCreate(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	storage := NewREST(helper)
	test := resttest.New(t, storage, fakeEtcdClient.SetError)
	template := validNew()
	template.ObjectMeta = kapi.ObjectMeta{}
	test.TestCreate(
		// valid
		template,
		// invalid
		&api.Template{},
	)
}

package etcd

import (
	"fmt"
	"testing"

	"github.com/coreos/go-etcd/etcd"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
	etcdstorage "k8s.io/kubernetes/pkg/storage/etcd"
	"k8s.io/kubernetes/pkg/tools"
	"k8s.io/kubernetes/pkg/tools/etcdtest"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/image/api"
)

func newHelper(t *testing.T) (*tools.FakeEtcdClient, storage.Interface) {
	fakeEtcdClient := tools.NewFakeEtcdClient(t)
	fakeEtcdClient.TestIndex = true
	helper := etcdstorage.NewEtcdStorage(fakeEtcdClient, latest.Codec, etcdtest.PathPrefix())
	return fakeEtcdClient, helper
}

func validNewDeletion() *api.ImageStreamDeletion {
	return &api.ImageStreamDeletion{
		ObjectMeta: kapi.ObjectMeta{
			Name: "ns:foo",
		},
	}
}

func TestCreate(t *testing.T) {
	_, helper := newHelper(t)
	storage := NewREST(helper)
	deletion := validNewDeletion()
	_, err := storage.Create(kapi.NewContext(), deletion)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetImageStreamDeletionError(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Err = fmt.Errorf("ns:foo")
	storage := NewREST(helper)

	deletion, err := storage.Get(kapi.NewContext(), "deletion")
	if deletion != nil {
		t.Errorf("Unexpected non-nil image stream deletion: %#v", deletion)
	}
	if err != fakeEtcdClient.Err {
		t.Errorf("Expected %#v, got %#v", fakeEtcdClient.Err, err)
	}
}

func TestGetImageStreamDeletionOK(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	storage := NewREST(helper)

	ctx := kapi.NewContext()
	deletionName := "ns:foo"
	key, _ := storage.store.KeyFunc(ctx, deletionName)
	fakeEtcdClient.Set(key, runtime.EncodeOrDie(latest.Codec, &api.ImageStreamDeletion{ObjectMeta: kapi.ObjectMeta{Name: deletionName}}), 0)

	obj, err := storage.Get(kapi.NewContext(), deletionName)
	if obj == nil {
		t.Fatalf("Unexpected nil deletion")
	}
	if err != nil {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}
	deletion := obj.(*api.ImageStreamDeletion)
	if e, a := deletionName, deletion.Name; e != a {
		t.Errorf("Expected %#v, got %#v", e, a)
	}
}

func TestListImageStreamDeletionssError(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Err = fmt.Errorf("ns:foo")
	storage := NewREST(helper)

	deletions, err := storage.List(kapi.NewContext(), nil, nil)
	if err != fakeEtcdClient.Err {
		t.Errorf("Expected %#v, Got %#v", fakeEtcdClient.Err, err)
	}

	if deletions != nil {
		t.Errorf("Unexpected non-nil imageStreamDeletions list: %#v", deletions)
	}
}

func TestListImageStreamDeletionssEmptyList(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.ChangeIndex = 1
	fakeEtcdClient.Data["/imagestreamdeletions"] = tools.EtcdResponseWithError{
		R: &etcd.Response{},
		E: fakeEtcdClient.NewError(tools.EtcdErrorCodeNotFound),
	}
	storage := NewREST(helper)

	deletions, err := storage.List(kapi.NewContext(), labels.Everything(), fields.Everything())
	if err != nil {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}
	if len(deletions.(*api.ImageStreamDeletionList).Items) != 0 {
		t.Errorf("Unexpected non-zero imageStreamDeletions list: %#v", deletions)
	}
	if deletions.(*api.ImageStreamDeletionList).ResourceVersion != "1" {
		t.Errorf("Unexpected resource version: %#v", deletions)
	}
}

func TestListImageStreamDeletionsPopulatedList(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	storage := NewREST(helper)

	fakeEtcdClient.Data["/imagestreamdeletions"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStreamDeletion{ObjectMeta: kapi.ObjectMeta{Name: "ns:foo"}})},
					{Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStreamDeletion{ObjectMeta: kapi.ObjectMeta{Name: "ns:bar"}})},
				},
			},
		},
	}

	list, err := storage.List(kapi.NewContext(), labels.Everything(), fields.Everything())
	if err != nil {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}

	deletions := list.(*api.ImageStreamDeletionList)

	if e, a := 2, len(deletions.Items); e != a {
		t.Errorf("Expected %v, got %v", e, a)
	}
}

func TestCreateImageStreamDeletionOK(t *testing.T) {
	_, helper := newHelper(t)
	storage := NewREST(helper)

	stream := &api.ImageStreamDeletion{ObjectMeta: kapi.ObjectMeta{Name: "ns:foo"}}
	_, err := storage.Create(kapi.NewContext(), stream)
	if err != nil {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}

	actual := &api.ImageStreamDeletion{}
	if err := helper.Get("/imagestreamdeletions/ns:foo", actual, false); err != nil {
		t.Fatalf("unexpected extraction error: %v", err)
	}
	if actual.Name != stream.Name {
		t.Errorf("unexpected stream: %#v", actual)
	}
	if stream.CreationTimestamp.IsZero() {
		t.Error("Unexpected zero CreationTimestamp")
	}
}

func TestCreateRegistryErrorSaving(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Err = fmt.Errorf("foo")
	storage := NewREST(helper)

	_, err := storage.Create(kapi.NewContext(), &api.ImageStreamDeletion{ObjectMeta: kapi.ObjectMeta{Name: "ns:foo"}})
	if err != fakeEtcdClient.Err {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}
}

func TestDeleteImageStreamDeletion(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Data["/imagestreamdeletions/ns:foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value:         runtime.EncodeOrDie(latest.Codec, validNewDeletion()),
				ModifiedIndex: 2,
			},
		},
	}
	storage := NewREST(helper)

	obj, err := storage.Delete(kapi.NewContext(), "ns:foo", nil)
	if err != nil {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}
	status, ok := obj.(*kapi.Status)
	if !ok {
		t.Fatalf("Expected status type, got: %#v", obj)
	}
	if status.Status != kapi.StatusSuccess {
		t.Errorf("Expected status=success, got: %#v", status)
	}
	if len(fakeEtcdClient.DeletedKeys) != 1 {
		t.Errorf("Expected 1 delete, found %#v", fakeEtcdClient.DeletedKeys)
	} else if key := "/imagestreamdeletions/ns:foo"; fakeEtcdClient.DeletedKeys[0] != key {
		t.Errorf("Unexpected key: %s, expected %s", fakeEtcdClient.DeletedKeys[0], key)
	}
}

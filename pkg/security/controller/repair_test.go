package controller

import (
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/security"
	"github.com/openshift/origin/pkg/security/uid"
)

type fakeRange struct {
	Err       error
	Range     *kapi.RangeAllocation
	Updated   *kapi.RangeAllocation
	UpdateErr error
}

func (r *fakeRange) Get() (*kapi.RangeAllocation, error) {
	return r.Range, r.Err
}

func (r *fakeRange) CreateOrUpdate(update *kapi.RangeAllocation) error {
	r.Updated = update
	return r.UpdateErr
}

func TestRepair(t *testing.T) {
	client := &testclient.Fake{}
	client.AddReactor("*", "*", func(a testclient.Action) (bool, runtime.Object, error) {
		list := &kapi.NamespaceList{
			Items: []kapi.Namespace{
				{ObjectMeta: kapi.ObjectMeta{Name: "default"}},
			},
		}
		return true, list, nil
	})

	alloc := &fakeRange{
		Range: &kapi.RangeAllocation{},
	}

	uidr, _ := uid.NewRange(10, 20, 2)
	repair := NewRepair(0*time.Second, client.Namespaces(), uidr, alloc)

	err := repair.RunOnce()
	if err != nil {
		t.Fatal(err)
	}
	if alloc.Updated == nil {
		t.Fatalf("did not store range: %#v", alloc)
	}
	if alloc.Updated.Range != "10-20/2" {
		t.Errorf("didn't store range properly: %#v", alloc.Updated)
	}
	if len(alloc.Updated.Data) != 0 {
		t.Errorf("data wasn't empty: %#v", alloc.Updated)
	}
}

func TestRepairIgnoresMismatch(t *testing.T) {
	client := &testclient.Fake{}
	client.AddReactor("*", "*", func(a testclient.Action) (bool, runtime.Object, error) {
		list := &kapi.NamespaceList{
			Items: []kapi.Namespace{
				{
					ObjectMeta: kapi.ObjectMeta{
						Name:        "default",
						Annotations: map[string]string{security.UIDRangeAnnotation: "1/5"},
					},
				},
			},
		}
		return true, list, nil
	})

	alloc := &fakeRange{
		Range: &kapi.RangeAllocation{},
	}

	uidr, _ := uid.NewRange(10, 20, 2)
	repair := NewRepair(0*time.Second, client.Namespaces(), uidr, alloc)

	err := repair.RunOnce()
	if err != nil {
		t.Fatal(err)
	}
	if alloc.Updated == nil {
		t.Fatalf("did not store range: %#v", alloc)
	}
	if alloc.Updated.Range != "10-20/2" {
		t.Errorf("didn't store range properly: %#v", alloc.Updated)
	}
	if len(alloc.Updated.Data) != 0 {
		t.Errorf("data wasn't empty: %#v", alloc.Updated)
	}
}

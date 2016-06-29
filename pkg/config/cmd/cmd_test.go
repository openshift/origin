package cmd

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"
)

type bulkTester struct {
	meta.RESTMapper

	mapping *meta.RESTMapping
	err     error
	opErr   error

	infos    []runtime.Object
	recorded []runtime.Object
}

func (bt *bulkTester) ResourceSingularizer(resource string) (string, error) {
	return resource, nil
}

func (bt *bulkTester) InfoForObject(obj runtime.Object, preferredGVKs []unversioned.GroupVersionKind) (*resource.Info, error) {
	bt.infos = append(bt.infos, obj)
	return &resource.Info{Object: obj, Mapping: bt.mapping}, bt.err
}

func (bt *bulkTester) Record(info *resource.Info, namespace string, obj runtime.Object) (runtime.Object, error) {
	bt.recorded = append(bt.recorded, obj)
	return obj, bt.opErr
}

func TestBulk(t *testing.T) {
	bt := &bulkTester{
		mapping: &meta.RESTMapping{
			MetadataAccessor: meta.NewAccessor(),
		},
	}
	b := Bulk{Mapper: bt, Op: bt.Record}

	in := &kapi.Pod{}
	if errs := b.Run(&kapi.List{Items: []runtime.Object{in}}, "test_namespace"); len(errs) > 0 {
		t.Fatal(errs)
	}
	if !reflect.DeepEqual(bt.infos, []runtime.Object{in}) {
		t.Fatalf("unexpected: %#v", bt.infos)
	}
	if !reflect.DeepEqual(bt.recorded, []runtime.Object{in}) {
		t.Fatalf("unexpected: %#v", bt.recorded)
	}
}

func TestBulkInfoError(t *testing.T) {
	bt := &bulkTester{
		mapping: &meta.RESTMapping{
			MetadataAccessor: meta.NewAccessor(),
		},
		err: fmt.Errorf("error1"),
	}
	b := Bulk{Mapper: bt, Op: bt.Record}

	in := &kapi.Pod{}
	if errs := b.Run(&kapi.List{Items: []runtime.Object{in}}, "test_namespace"); len(errs) != 1 || errs[0] != bt.err {
		t.Fatal(errs)
	}
	if !reflect.DeepEqual(bt.infos, []runtime.Object{in}) {
		t.Fatalf("unexpected: %#v", bt.infos)
	}
	if !reflect.DeepEqual(bt.recorded, []runtime.Object(nil)) {
		t.Fatalf("unexpected: %#v", bt.recorded)
	}
}

func TestBulkOpError(t *testing.T) {
	bt := &bulkTester{
		mapping: &meta.RESTMapping{
			MetadataAccessor: meta.NewAccessor(),
		},
		opErr: fmt.Errorf("error1"),
	}
	b := Bulk{Mapper: bt, Op: bt.Record}

	in := &kapi.Pod{}
	if errs := b.Run(&kapi.List{Items: []runtime.Object{in}}, "test_namespace"); len(errs) != 1 || errs[0] != bt.opErr {
		t.Fatal(errs)
	}
	if !reflect.DeepEqual(bt.infos, []runtime.Object{in}) {
		t.Fatalf("unexpected: %#v", bt.infos)
	}
	if !reflect.DeepEqual(bt.recorded, []runtime.Object{in}) {
		t.Fatalf("unexpected: %#v", bt.recorded)
	}
}

func TestBulkAction(t *testing.T) {
	bt := &bulkTester{
		mapping: &meta.RESTMapping{
			MetadataAccessor: meta.NewAccessor(),
		},
	}
	out, err := &bytes.Buffer{}, &bytes.Buffer{}
	bulk := Bulk{Mapper: bt, Op: bt.Record}
	b := &BulkAction{Bulk: bulk, Output: "", Out: out, ErrOut: err}
	b2 := b.WithMessage("test1", "test2")

	in := &kapi.Pod{ObjectMeta: kapi.ObjectMeta{Name: "obj1"}}
	if errs := b2.Run(&kapi.List{Items: []runtime.Object{in}}, "test_namespace"); len(errs) != 0 {
		t.Fatal(errs)
	}
	if !reflect.DeepEqual(bt.infos, []runtime.Object{in}) {
		t.Fatalf("unexpected: %#v", bt.infos)
	}
	if !reflect.DeepEqual(bt.recorded, []runtime.Object{in}) {
		t.Fatalf("unexpected: %#v", bt.recorded)
	}
	if out.String() != `--> test1 ...
    "obj1" test2
--> Success
` {
		t.Fatalf("unexpected: %s", out.String())
	}
	if err.String() != `` {
		t.Fatalf("unexpected: %s", err.String())
	}
}

func TestBulkActionCompact(t *testing.T) {
	bt := &bulkTester{
		mapping: &meta.RESTMapping{
			MetadataAccessor: meta.NewAccessor(),
		},
	}
	out, err := &bytes.Buffer{}, &bytes.Buffer{}
	bulk := Bulk{Mapper: bt, Op: bt.Record}
	b := &BulkAction{Bulk: bulk, Output: "", Out: out, ErrOut: err}
	b.Compact()
	b2 := b.WithMessage("test1", "test2")

	in := &kapi.Pod{ObjectMeta: kapi.ObjectMeta{Name: "obj1"}}
	if errs := b2.Run(&kapi.List{Items: []runtime.Object{in}}, "test_namespace"); len(errs) != 0 {
		t.Fatal(errs)
	}
	if !reflect.DeepEqual(bt.infos, []runtime.Object{in}) {
		t.Fatalf("unexpected: %#v", bt.infos)
	}
	if !reflect.DeepEqual(bt.recorded, []runtime.Object{in}) {
		t.Fatalf("unexpected: %#v", bt.recorded)
	}
	if out.String() != `` {
		t.Fatalf("unexpected: %s", out.String())
	}
	if err.String() != `` {
		t.Fatalf("unexpected: %s", err.String())
	}
}

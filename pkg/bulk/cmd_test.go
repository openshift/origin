package bulk

import (
	"bytes"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	_ "k8s.io/kubernetes/pkg/apis/core/install"
)

type bulkTester struct {
	meta.RESTMapper

	mapping *meta.RESTMapping
	err     error
	opErr   error

	recorded []runtime.Object
}

func (bt *bulkTester) ResourceSingularizer(resource string) (string, error) {
	return resource, nil
}

func (bt *bulkTester) Record(obj *unstructured.Unstructured, namespace string) (*unstructured.Unstructured, error) {
	bt.recorded = append(bt.recorded, obj)
	return obj, bt.opErr
}

func TestBulk(t *testing.T) {
	bt := &bulkTester{
		mapping: &meta.RESTMapping{},
	}
	b := Bulk{Scheme: legacyscheme.Scheme, Op: bt.Record}

	in := &kapi.Pod{}
	if errs := b.Run(&kapi.List{Items: []runtime.Object{in}}, "test_namespace"); len(errs) > 0 {
		t.Fatal(errs)
	}
	if len(bt.recorded) != len([]runtime.Object{in}) {
		t.Fatalf("unexpected: %#v", bt.recorded)
	}
}

func TestBulkOpError(t *testing.T) {
	bt := &bulkTester{
		mapping: &meta.RESTMapping{},
		opErr:   fmt.Errorf("error1"),
	}
	b := Bulk{Scheme: legacyscheme.Scheme, Op: bt.Record}

	in := &kapi.Pod{}
	if errs := b.Run(&kapi.List{Items: []runtime.Object{in}}, "test_namespace"); len(errs) != 1 || errs[0] != bt.opErr {
		t.Fatal(errs)
	}
	if len(bt.recorded) != len([]runtime.Object{in}) {
		t.Fatalf("unexpected: %#v", bt.recorded)
	}
}

func TestBulkAction(t *testing.T) {
	bt := &bulkTester{
		mapping: &meta.RESTMapping{},
	}
	out, err := &bytes.Buffer{}, &bytes.Buffer{}
	bulk := Bulk{Scheme: legacyscheme.Scheme, Op: bt.Record}
	b := &BulkAction{Bulk: bulk, Output: "", Out: out, ErrOut: err}
	b2 := b.WithMessage("test1", "test2")

	in := &kapi.Pod{ObjectMeta: metav1.ObjectMeta{Name: "obj1"}}
	if errs := b2.Run(&kapi.List{Items: []runtime.Object{in}}, "test_namespace"); len(errs) != 0 {
		t.Fatal(errs)
	}
	if len(bt.recorded) != len([]runtime.Object{in}) {
		t.Fatalf("unexpected: %#v", bt.recorded)
	}
	if out.String() != `--> test1 ...
    pod "obj1" test2
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
		mapping: &meta.RESTMapping{},
	}
	out, err := &bytes.Buffer{}, &bytes.Buffer{}
	bulk := Bulk{Scheme: legacyscheme.Scheme, Op: bt.Record}
	b := &BulkAction{Bulk: bulk, Output: "", Out: out, ErrOut: err}
	b.Compact()
	b2 := b.WithMessage("test1", "test2")

	in := &kapi.Pod{ObjectMeta: metav1.ObjectMeta{Name: "obj1"}}
	if errs := b2.Run(&kapi.List{Items: []runtime.Object{in}}, "test_namespace"); len(errs) != 0 {
		t.Fatal(errs)
	}
	if len(bt.recorded) != len([]runtime.Object{in}) {
		t.Fatalf("unexpected: %#v", bt.recorded)
	}
	if out.String() != `` {
		t.Fatalf("unexpected: %s", out.String())
	}
	if err.String() != `` {
		t.Fatalf("unexpected: %s", err.String())
	}
}

package resourceapply

import (
	"testing"

	"github.com/davecgh/go-spew/spew"

	"k8s.io/client-go/kubernetes/fake"

	"github.com/openshift/library-go/pkg/operator/events"
)

func TestApplyDirectly(t *testing.T) {
	requiredObj, gvk, err := genericCodec.Decode([]byte(`apiVersion: v1
kind: Namespace
metadata:
  name: openshift-apiserver
  labels:
    openshift.io/run-level: "1"
`), nil, nil)
	t.Log(spew.Sdump(requiredObj))
	t.Log(spew.Sdump(gvk))
	if err != nil {
		t.Fatal(err)
	}
}

func TestApplyDirectlyUnhandledType(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	content := func(name string) ([]byte, error) {
		return []byte(`apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: sample-claim
  labels:
    openshift.io/run-level: "1"
`), nil
	}
	recorder := events.NewInMemoryRecorder("")
	ret := ApplyDirectly(fakeClient, recorder, content, "pvc")
	if ret[0].Error == nil {
		t.Fatal("missing expected error")
	} else if ret[0].Error.Error() != "unhandled type *v1.PersistentVolumeClaim" {
		t.Fatal(ret[0].Error)
	}
}

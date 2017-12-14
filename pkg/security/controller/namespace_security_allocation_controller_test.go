package controller

import (
	"fmt"
	"strings"
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/openshift/origin/pkg/security"
	"github.com/openshift/origin/pkg/security/mcs"
	"github.com/openshift/origin/pkg/security/uid"
	"github.com/openshift/origin/pkg/security/uidallocator"
)

func TestController(t *testing.T) {
	var action clientgotesting.Action
	client := &fake.Clientset{}
	client.AddReactor("*", "*", func(a clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		action = a
		return true, (*v1.Namespace)(nil), nil
	})

	uidr, _ := uid.NewRange(10, 20, 2)
	mcsr, _ := mcs.NewRange("s0:", 10, 2)
	uida := uidallocator.NewInMemory(uidr)
	c := &NamespaceSecurityDefaultsController{
		uidAllocator: uida,
		mcsAllocator: DefaultMCSAllocation(uidr, mcsr, 5),
		client:       client.Core().Namespaces(),
	}

	err := c.allocate(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}})
	if err != nil {
		t.Fatal(err)
	}

	got := action.(clientgotesting.CreateAction).GetObject().(*v1.Namespace)
	if got.Annotations[security.UIDRangeAnnotation] != "10/2" {
		t.Errorf("unexpected uid annotation: %#v", got)
	}
	if got.Annotations[security.SupplementalGroupsAnnotation] != "10/2" {
		t.Errorf("unexpected supplemental group annotation: %#v", got)
	}
	if got.Annotations[security.MCSAnnotation] != "s0:c1,c0" {
		t.Errorf("unexpected mcs annotation: %#v", got)
	}
	if !uida.Has(uid.Block{Start: 10, End: 11}) {
		t.Errorf("did not allocate uid: %#v", uida)
	}
}

func TestControllerError(t *testing.T) {
	testCases := map[string]struct {
		err     func() error
		errFn   func(err error) bool
		reactFn clientgotesting.ReactionFunc
		actions int
	}{
		"not found": {
			err:     func() error { return errors.NewNotFound(kapi.Resource("Namespace"), "test") },
			errFn:   func(err error) bool { return err == nil },
			actions: 1,
		},
		"unknown": {
			err:     func() error { return fmt.Errorf("unknown") },
			errFn:   func(err error) bool { return err.Error() == "unknown" },
			actions: 1,
		},
		"conflict": {
			actions: 1,
			reactFn: func(a clientgotesting.Action) (bool, runtime.Object, error) {
				if a.Matches("get", "namespaces") {
					return true, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}}, nil
				}
				return true, (*v1.Namespace)(nil), errors.NewConflict(kapi.Resource("namespace"), "test", fmt.Errorf("test conflict"))
			},
			errFn: func(err error) bool {
				return err != nil && strings.Contains(err.Error(), "test conflict")
			},
		},
	}

	for s, testCase := range testCases {
		client := &fake.Clientset{}

		if testCase.reactFn == nil {
			testCase.reactFn = func(a clientgotesting.Action) (bool, runtime.Object, error) {
				return true, (*v1.Namespace)(nil), testCase.err()
			}
		}

		client.AddReactor("*", "*", testCase.reactFn)

		uidr, _ := uid.NewRange(10, 19, 2)
		mcsr, _ := mcs.NewRange("s0:", 10, 2)
		uida := uidallocator.NewInMemory(uidr)
		c := &NamespaceSecurityDefaultsController{
			uidAllocator: uida,
			mcsAllocator: DefaultMCSAllocation(uidr, mcsr, 5),
			client:       client.Core().Namespaces(),
		}

		err := c.allocate(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}})
		if !testCase.errFn(err) {
			t.Errorf("%s: unexpected error: %v", s, err)
		}

		if len(client.Actions()) != testCase.actions {
			t.Errorf("%s: expected %d actions: %v", s, testCase.actions, client.Actions())
		}
		if uida.Free() != 5 {
			t.Errorf("%s: should not have allocated uid: %d/%d", s, uida.Free(), uidr.Size())
		}
	}
}

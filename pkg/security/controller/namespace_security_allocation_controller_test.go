package controller

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefakeclient "k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/controller"

	securityv1 "github.com/openshift/api/security/v1"
	securityv1fakeclient "github.com/openshift/client-go/security/clientset/versioned/fake"
	"github.com/openshift/origin/pkg/security"
	"github.com/openshift/origin/pkg/security/mcs"
	"github.com/openshift/origin/pkg/security/uid"
)

func TestController(t *testing.T) {
	kubeclient := kubefakeclient.NewSimpleClientset()
	securityclient := securityv1fakeclient.NewSimpleClientset()
	indexer := cache.NewIndexer(controller.KeyFunc, cache.Indexers{})

	uidr, _ := uid.NewRange(10, 20, 2)
	mcsr, _ := mcs.NewRange("s0:", 10, 2)
	c := &NamespaceSCCAllocationController{
		requiredUIDRange:      uidr,
		mcsAllocator:          DefaultMCSAllocation(uidr, mcsr, 5),
		namespaceClient:       kubeclient.CoreV1().Namespaces(),
		nsLister:              corev1listers.NewNamespaceLister(indexer),
		rangeAllocationClient: securityclient.SecurityV1(),
	}
	err := c.Repair()
	if err != nil {
		t.Fatal(err)
	}
	rangeAllocationActions := securityclient.Actions()
	if len(rangeAllocationActions) != 2 {
		t.Fatalf("expected get, create, got\n%v", spew.Sdump(rangeAllocationActions))
	}
	if action, ok := rangeAllocationActions[0].(clientgotesting.GetAction); !ok {
		t.Fatal(spew.Sdump(action))
	}
	if action, ok := rangeAllocationActions[1].(clientgotesting.CreateAction); !ok {
		t.Fatal(spew.Sdump(action))
	}
	securityclient.ClearActions()

	err = c.allocate(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}})
	if err != nil {
		t.Fatal(err)
	}

	kubeActions := kubeclient.Actions()
	if len(kubeActions) != 1 {
		t.Fatalf("expected update, got\n%v", spew.Sdump(kubeActions))
	}
	createNSAction := kubeActions[0]

	got := createNSAction.(clientgotesting.CreateAction).GetObject().(*v1.Namespace)
	if got.Annotations[security.UIDRangeAnnotation] != "10/2" {
		t.Errorf("unexpected uid annotation: %#v", got)
	}
	if got.Annotations[security.SupplementalGroupsAnnotation] != "10/2" {
		t.Errorf("unexpected supplemental group annotation: %#v", got)
	}
	if got.Annotations[security.MCSAnnotation] != "s0:c1,c0" {
		t.Errorf("unexpected mcs annotation: %#v", got)
	}

	rangeAllocationActions = securityclient.Actions()
	if len(rangeAllocationActions) != 2 {
		t.Fatalf("expected update got\n%v", spew.Sdump(rangeAllocationActions))
	}
	if action, ok := rangeAllocationActions[0].(clientgotesting.GetAction); !ok {
		t.Fatal(spew.Sdump(action))
	}
	actualRange := rangeAllocationActions[1].(clientgotesting.UpdateAction).GetObject().(*securityv1.RangeAllocation)
	actualAllocatedInt := big.NewInt(0).SetBytes(actualRange.Data)
	if actualAllocatedInt.Uint64() != 1 {
		t.Errorf("did not allocate uid: %d", actualAllocatedInt.Uint64())
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
		t.Run(s, func(t *testing.T) {
			if testCase.reactFn == nil {
				testCase.reactFn = func(a clientgotesting.Action) (bool, runtime.Object, error) {
					return true, (*v1.Namespace)(nil), testCase.err()
				}
			}
			kubeclient := kubefakeclient.NewSimpleClientset()
			kubeclient.PrependReactor("*", "*", testCase.reactFn)

			securityclient := securityv1fakeclient.NewSimpleClientset()
			indexer := cache.NewIndexer(controller.KeyFunc, cache.Indexers{})

			uidr, _ := uid.NewRange(10, 19, 2)
			mcsr, _ := mcs.NewRange("s0:", 10, 2)
			c := &NamespaceSCCAllocationController{
				requiredUIDRange:      uidr,
				mcsAllocator:          DefaultMCSAllocation(uidr, mcsr, 5),
				namespaceClient:       kubeclient.CoreV1().Namespaces(),
				nsLister:              corev1listers.NewNamespaceLister(indexer),
				rangeAllocationClient: securityclient.SecurityV1(),
			}

			err := c.Repair()
			if err != nil {
				t.Fatal(err)
			}
			securityclient.ClearActions()

			err = c.allocate(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}})
			if !testCase.errFn(err) {
				t.Fatal(err)
			}

			if len(kubeclient.Actions()) != testCase.actions {
				t.Fatalf("expected %d actions: %v", testCase.actions, kubeclient.Actions())
			}

			rangeActions := securityclient.Actions()
			if len(rangeActions) != 2 {
				t.Fatalf("only take two actions to allocate\n%v", spew.Sdump(rangeActions))
			}
			if err != nil && c.currentUIDRangeAllocation != nil {
				t.Fatal("state wasn't cleared!")
			}
		})
	}
}

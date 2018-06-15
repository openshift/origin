package controller

import (
	"math/big"
	"testing"

	"github.com/davecgh/go-spew/spew"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/controller"

	securityv1 "github.com/openshift/api/security/v1"
	securityv1fakeclient "github.com/openshift/client-go/security/clientset/versioned/fake"
	"github.com/openshift/origin/pkg/security"
	"github.com/openshift/origin/pkg/security/uid"
)

func TestRepair(t *testing.T) {
	securityclient := securityv1fakeclient.NewSimpleClientset()
	indexer := cache.NewIndexer(controller.KeyFunc, cache.Indexers{})
	indexer.Add(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})

	uidr, _ := uid.NewRange(10, 20, 2)
	c := &NamespaceSCCAllocationController{
		requiredUIDRange:      uidr,
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
	action, ok := rangeAllocationActions[1].(clientgotesting.CreateAction)
	if !ok {
		t.Fatal(spew.Sdump(action))
	}
	rangeAllocation := action.GetObject().(*securityv1.RangeAllocation)

	if rangeAllocation.Range != "10-20/2" {
		t.Errorf("didn't store range properly: %#v", rangeAllocation.Range)
	}
	actualAllocatedInt := big.NewInt(0).SetBytes(rangeAllocation.Data)
	if actualAllocatedInt.Uint64() != 0 {
		t.Errorf("data wasn't empty: %#v", actualAllocatedInt.Uint64())
	}
}

func TestRepairIgnoresMismatch(t *testing.T) {
	securityclient := securityv1fakeclient.NewSimpleClientset()
	indexer := cache.NewIndexer(controller.KeyFunc, cache.Indexers{})
	indexer.Add(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:        "default",
		Annotations: map[string]string{security.UIDRangeAnnotation: "1/5"},
	}})

	uidr, _ := uid.NewRange(10, 20, 2)
	c := &NamespaceSCCAllocationController{
		requiredUIDRange:      uidr,
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
	action, ok := rangeAllocationActions[1].(clientgotesting.CreateAction)
	if !ok {
		t.Fatal(spew.Sdump(action))
	}
	rangeAllocation := action.GetObject().(*securityv1.RangeAllocation)

	if rangeAllocation.Range != "10-20/2" {
		t.Errorf("didn't store range properly: %#v", rangeAllocation.Range)
	}
	actualAllocatedInt := big.NewInt(0).SetBytes(rangeAllocation.Data)
	if actualAllocatedInt.Uint64() != 0 {
		t.Errorf("data wasn't empty: %#v", actualAllocatedInt.Uint64())
	}
}

func TestRepairTable(t *testing.T) {
	tests := []struct {
		name                    string
		namespaces              []*corev1.Namespace
		existingRangeAllocation *securityv1.RangeAllocation
		uidRange                string
		expectedRange           string
		expectedData            *big.Int
	}{
		{
			name: "ignore-mismatch",
			namespaces: []*corev1.Namespace{
				{ObjectMeta: metav1.ObjectMeta{
					Name:        "one",
					Annotations: map[string]string{security.UIDRangeAnnotation: "10/5"},
				}},
			},
			uidRange:      "10-20/2",
			expectedRange: "10-20/2",
			expectedData:  big.NewInt(0),
		},
		{
			name: "update-range-string",
			namespaces: []*corev1.Namespace{
				{ObjectMeta: metav1.ObjectMeta{
					Name:        "one",
					Annotations: map[string]string{security.UIDRangeAnnotation: "10/5"},
				}},
				{ObjectMeta: metav1.ObjectMeta{
					Name:        "two",
					Annotations: map[string]string{security.UIDRangeAnnotation: "25/5"},
				}},
			},
			existingRangeAllocation: &securityv1.RangeAllocation{
				ObjectMeta: metav1.ObjectMeta{Name: "default"},
				Range:      "10-20/2",
			},
			uidRange:      "20-40/5",
			expectedRange: "20-40/5",
			expectedData:  big.NewInt(2),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var securityclient *securityv1fakeclient.Clientset
			if test.existingRangeAllocation != nil {
				securityclient = securityv1fakeclient.NewSimpleClientset(test.existingRangeAllocation)
			} else {
				securityclient = securityv1fakeclient.NewSimpleClientset()
			}

			indexer := cache.NewIndexer(controller.KeyFunc, cache.Indexers{})
			for _, ns := range test.namespaces {
				indexer.Add(ns)
			}

			uidr, _ := uid.ParseRange(test.uidRange)
			c := &NamespaceSCCAllocationController{
				requiredUIDRange:      uidr,
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

			var actualRangeAllocation *securityv1.RangeAllocation
			if test.existingRangeAllocation != nil {
				action, ok := rangeAllocationActions[1].(clientgotesting.UpdateAction)
				if !ok {
					t.Fatal(spew.Sdump(action))
				}
				actualRangeAllocation = action.GetObject().(*securityv1.RangeAllocation)

			} else {
				action, ok := rangeAllocationActions[1].(clientgotesting.CreateAction)
				if !ok {
					t.Fatal(spew.Sdump(action))
				}
				actualRangeAllocation = action.GetObject().(*securityv1.RangeAllocation)
			}

			if actualRangeAllocation.Range != test.expectedRange {
				t.Errorf("expected %v, got %v", test.expectedRange, actualRangeAllocation.Range)
			}
			actualAllocatedInt := big.NewInt(0).SetBytes(actualRangeAllocation.Data)
			if actualAllocatedInt.Uint64() != test.expectedData.Uint64() {
				t.Errorf("expected %v, got %v", test.expectedData, actualAllocatedInt.Uint64())
			}
		})
	}

}

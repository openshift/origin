package reaper

import (
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/client/testclient"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	"github.com/openshift/origin/pkg/security/legacyclient"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

var (
	groupsResource              = schema.GroupVersionResource{Group: "", Version: "", Resource: "groups"}
	clusterRoleBindingsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "clusterrolebindings"}
	roleBindingsResource        = schema.GroupVersionResource{Group: "", Version: "", Resource: "rolebindings"}
)

func TestGroupReaper(t *testing.T) {
	tests := []struct {
		name     string
		group    string
		objects  []runtime.Object
		sccs     []runtime.Object
		expected []interface{}
	}{
		{
			name:    "no objects",
			group:   "mygroup",
			objects: []runtime.Object{},
			expected: []interface{}{
				clientgotesting.DeleteActionImpl{ActionImpl: clientgotesting.ActionImpl{Verb: "delete", Resource: groupsResource}, Name: "mygroup"},
			},
		},
		{
			name:  "cluster bindings",
			group: "mygroup",
			objects: []runtime.Object{
				&authorizationapi.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-no-subjects"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{},
				},
				&authorizationapi.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-one-subject"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{{Name: "mygroup", Kind: "Group"}},
				},
				&authorizationapi.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-mismatched-subject"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{{Name: "mygroup"}, {Name: "mygroup", Kind: "User"}, {Name: "mygroup", Kind: "Other"}},
				},
			},
			expected: []interface{}{
				clientgotesting.UpdateActionImpl{ActionImpl: clientgotesting.ActionImpl{Verb: "update", Resource: clusterRoleBindingsResource}, Object: &authorizationapi.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-one-subject"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{},
				}},
				clientgotesting.DeleteActionImpl{ActionImpl: clientgotesting.ActionImpl{Verb: "delete", Resource: groupsResource}, Name: "mygroup"},
			},
		},
		{
			name:  "namespaced bindings",
			group: "mygroup",
			objects: []runtime.Object{
				&authorizationapi.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-no-subjects", Namespace: "ns1"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{},
				},
				&authorizationapi.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-one-subject", Namespace: "ns2"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{{Name: "mygroup", Kind: "Group"}},
				},
				&authorizationapi.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-mismatched-subject", Namespace: "ns3"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{{Name: "mygroup"}, {Name: "mygroup", Kind: "User"}, {Name: "mygroup", Kind: "Other"}},
				},
			},
			expected: []interface{}{
				clientgotesting.UpdateActionImpl{ActionImpl: clientgotesting.ActionImpl{Verb: "update", Resource: roleBindingsResource, Namespace: "ns2"}, Object: &authorizationapi.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-one-subject", Namespace: "ns2"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{},
				}},
				clientgotesting.DeleteActionImpl{ActionImpl: clientgotesting.ActionImpl{Verb: "delete", Resource: groupsResource}, Name: "mygroup"},
			},
		},
		{
			name:  "sccs",
			group: "mygroup",
			sccs: []runtime.Object{
				&securityapi.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{Name: "scc-no-subjects"},
					Groups:     []string{},
				},
				&securityapi.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{Name: "scc-one-subject"},
					Groups:     []string{"mygroup"},
				},
				&securityapi.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{Name: "scc-mismatched-subjects"},
					Users:      []string{"mygroup"},
					Groups:     []string{"mygroup2"},
				},
			},
			expected: []interface{}{
				clientgotesting.UpdateActionImpl{ActionImpl: clientgotesting.ActionImpl{Verb: "update", Resource: schema.GroupVersionResource{Resource: "securitycontextconstraints"}}, Object: &securityapi.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{Name: "scc-one-subject"},
					Groups:     []string{},
				}},
				clientgotesting.DeleteActionImpl{ActionImpl: clientgotesting.ActionImpl{Verb: "delete", Resource: groupsResource}, Name: "mygroup"},
			},
		},
	}

	for _, test := range tests {
		tc := testclient.NewSimpleFake(testclient.OriginObjects(test.objects)...)
		ktc := legacyclient.NewSimpleFake(test.sccs...)

		actual := []interface{}{}
		oreactor := func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			actual = append(actual, action)
			return false, nil, nil
		}
		kreactor := func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			actual = append(actual, action)
			return false, nil, nil
		}

		tc.PrependReactor("update", "*", oreactor)
		tc.PrependReactor("delete", "*", oreactor)
		ktc.Fake.PrependReactor("update", "*", kreactor)
		ktc.Fake.PrependReactor("delete", "*", kreactor)

		reaper := NewGroupReaper(tc, tc, tc, ktc)
		err := reaper.Stop("", test.group, 0, nil)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
		}

		if !reflect.DeepEqual(test.expected, actual) {
			for i, x := range test.expected {
				t.Logf("Expected %d: %s", i, spew.Sprint(x))
			}
			for i, x := range actual {
				t.Logf("Actual %d:   %s", i, spew.Sprint(x))
			}
			t.Errorf("%s: unexpected actions", test.name)
		}
	}
}

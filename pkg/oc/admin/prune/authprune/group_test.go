package authprune

import (
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authfake "github.com/openshift/origin/pkg/authorization/generated/internalclientset/fake"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	securityfake "github.com/openshift/origin/pkg/security/generated/internalclientset/fake"
	userfake "github.com/openshift/origin/pkg/user/generated/internalclientset/fake"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

var (
	groupsResource              = schema.GroupVersionResource{Group: "user.openshift.io", Version: "", Resource: "groups"}
	clusterRoleBindingsResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "", Resource: "clusterrolebindings"}
	roleBindingsResource        = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "", Resource: "rolebindings"}
	sccResource                 = schema.GroupVersionResource{Group: "security.openshift.io", Version: "", Resource: "securitycontextconstraints"}
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
				clientgotesting.UpdateActionImpl{ActionImpl: clientgotesting.ActionImpl{Verb: "update", Resource: sccResource}, Object: &securityapi.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{Name: "scc-one-subject"},
					Groups:     []string{},
				}},
				clientgotesting.DeleteActionImpl{ActionImpl: clientgotesting.ActionImpl{Verb: "delete", Resource: groupsResource}, Name: "mygroup"},
			},
		},
	}

	for _, test := range tests {
		authFake := authfake.NewSimpleClientset(test.objects...)
		userFake := userfake.NewSimpleClientset()
		securityFake := securityfake.NewSimpleClientset(test.sccs...)

		actual := []interface{}{}
		oreactor := func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			actual = append(actual, action)
			return false, nil, nil
		}
		kreactor := func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			actual = append(actual, action)
			return false, nil, nil
		}

		authFake.PrependReactor("update", "*", oreactor)
		userFake.PrependReactor("update", "*", oreactor)
		authFake.PrependReactor("delete", "*", oreactor)
		userFake.PrependReactor("delete", "*", oreactor)
		securityFake.Fake.PrependReactor("update", "*", kreactor)
		securityFake.Fake.PrependReactor("delete", "*", kreactor)

		reaper := NewGroupReaper(userFake, authFake, securityFake.Security().SecurityContextConstraints())
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

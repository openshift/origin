package reaper

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/davecgh/go-spew/spew"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client/testclient"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

func TestGroupReaper(t *testing.T) {
	tests := []struct {
		name     string
		group    string
		objects  []runtime.Object
		expected []interface{}
	}{
		{
			name:    "no objects",
			group:   "mygroup",
			objects: []runtime.Object{},
			expected: []interface{}{
				ktestclient.DeleteActionImpl{ActionImpl: ktestclient.ActionImpl{Verb: "delete", Resource: "groups"}, Name: "mygroup"},
			},
		},
		{
			name:  "cluster bindings",
			group: "mygroup",
			objects: []runtime.Object{
				&authorizationapi.ClusterRoleBinding{
					ObjectMeta: kapi.ObjectMeta{Name: "binding-no-subjects"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{},
				},
				&authorizationapi.ClusterRoleBinding{
					ObjectMeta: kapi.ObjectMeta{Name: "binding-one-subject"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{{Name: "mygroup", Kind: "Group"}},
				},
				&authorizationapi.ClusterRoleBinding{
					ObjectMeta: kapi.ObjectMeta{Name: "binding-mismatched-subject"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{{Name: "mygroup"}, {Name: "mygroup", Kind: "User"}, {Name: "mygroup", Kind: "Other"}},
				},
			},
			expected: []interface{}{
				ktestclient.UpdateActionImpl{ActionImpl: ktestclient.ActionImpl{Verb: "update", Resource: "clusterrolebindings"}, Object: &authorizationapi.ClusterRoleBinding{
					ObjectMeta: kapi.ObjectMeta{Name: "binding-one-subject"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{},
				}},
				ktestclient.DeleteActionImpl{ActionImpl: ktestclient.ActionImpl{Verb: "delete", Resource: "groups"}, Name: "mygroup"},
			},
		},
		{
			name:  "namespaced bindings",
			group: "mygroup",
			objects: []runtime.Object{
				&authorizationapi.RoleBinding{
					ObjectMeta: kapi.ObjectMeta{Name: "binding-no-subjects", Namespace: "ns1"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{},
				},
				&authorizationapi.RoleBinding{
					ObjectMeta: kapi.ObjectMeta{Name: "binding-one-subject", Namespace: "ns2"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{{Name: "mygroup", Kind: "Group"}},
				},
				&authorizationapi.RoleBinding{
					ObjectMeta: kapi.ObjectMeta{Name: "binding-mismatched-subject", Namespace: "ns3"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{{Name: "mygroup"}, {Name: "mygroup", Kind: "User"}, {Name: "mygroup", Kind: "Other"}},
				},
			},
			expected: []interface{}{
				ktestclient.UpdateActionImpl{ActionImpl: ktestclient.ActionImpl{Verb: "update", Resource: "rolebindings", Namespace: "ns2"}, Object: &authorizationapi.RoleBinding{
					ObjectMeta: kapi.ObjectMeta{Name: "binding-one-subject", Namespace: "ns2"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{},
				}},
				ktestclient.DeleteActionImpl{ActionImpl: ktestclient.ActionImpl{Verb: "delete", Resource: "groups"}, Name: "mygroup"},
			},
		},
		{
			name:  "sccs",
			group: "mygroup",
			objects: []runtime.Object{
				&kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{Name: "scc-no-subjects"},
					Groups:     []string{},
				},
				&kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{Name: "scc-one-subject"},
					Groups:     []string{"mygroup"},
				},
				&kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{Name: "scc-mismatched-subjects"},
					Users:      []string{"mygroup"},
					Groups:     []string{"mygroup2"},
				},
			},
			expected: []interface{}{
				ktestclient.UpdateActionImpl{ActionImpl: ktestclient.ActionImpl{Verb: "update", Resource: "securitycontextconstraints"}, Object: &kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{Name: "scc-one-subject"},
					Groups:     []string{},
				}},
				ktestclient.DeleteActionImpl{ActionImpl: ktestclient.ActionImpl{Verb: "delete", Resource: "groups"}, Name: "mygroup"},
			},
		},
	}

	for _, test := range tests {
		tc := testclient.NewSimpleFake(test.objects...)
		ktc := ktestclient.NewSimpleFake(test.objects...)

		actual := []interface{}{}
		reactor := func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
			actual = append(actual, action)
			return false, nil, nil
		}

		tc.PrependReactor("update", "*", reactor)
		tc.PrependReactor("delete", "*", reactor)
		ktc.PrependReactor("update", "*", reactor)
		ktc.PrependReactor("delete", "*", reactor)

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

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
	authenticationapi "github.com/openshift/origin/pkg/user/api"
)

func TestUserReaper(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		objects  []runtime.Object
		expected []interface{}
	}{
		{
			name:    "no objects",
			user:    "bob",
			objects: []runtime.Object{},
			expected: []interface{}{
				ktestclient.DeleteActionImpl{ActionImpl: ktestclient.ActionImpl{Verb: "delete", Resource: "users"}, Name: "bob"},
			},
		},
		{
			name: "cluster bindings",
			user: "bob",
			objects: []runtime.Object{
				&authorizationapi.ClusterRoleBinding{
					ObjectMeta: kapi.ObjectMeta{Name: "binding-no-subjects"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{},
				},
				&authorizationapi.ClusterRoleBinding{
					ObjectMeta: kapi.ObjectMeta{Name: "binding-one-subject"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{{Name: "bob", Kind: "User"}},
				},
				&authorizationapi.ClusterRoleBinding{
					ObjectMeta: kapi.ObjectMeta{Name: "binding-mismatched-subject"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{{Name: "bob"}, {Name: "bob", Kind: "Group"}, {Name: "bob", Kind: "Other"}},
				},
			},
			expected: []interface{}{
				ktestclient.UpdateActionImpl{ActionImpl: ktestclient.ActionImpl{Verb: "update", Resource: "clusterrolebindings"}, Object: &authorizationapi.ClusterRoleBinding{
					ObjectMeta: kapi.ObjectMeta{Name: "binding-one-subject"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{},
				}},
				ktestclient.DeleteActionImpl{ActionImpl: ktestclient.ActionImpl{Verb: "delete", Resource: "users"}, Name: "bob"},
			},
		},
		{
			name: "namespaced bindings",
			user: "bob",
			objects: []runtime.Object{
				&authorizationapi.RoleBinding{
					ObjectMeta: kapi.ObjectMeta{Name: "binding-no-subjects", Namespace: "ns1"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{},
				},
				&authorizationapi.RoleBinding{
					ObjectMeta: kapi.ObjectMeta{Name: "binding-one-subject", Namespace: "ns2"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{{Name: "bob", Kind: "User"}},
				},
				&authorizationapi.RoleBinding{
					ObjectMeta: kapi.ObjectMeta{Name: "binding-mismatched-subject", Namespace: "ns3"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{{Name: "bob"}, {Name: "bob", Kind: "Group"}, {Name: "bob", Kind: "Other"}},
				},
			},
			expected: []interface{}{
				ktestclient.UpdateActionImpl{ActionImpl: ktestclient.ActionImpl{Verb: "update", Resource: "rolebindings", Namespace: "ns2"}, Object: &authorizationapi.RoleBinding{
					ObjectMeta: kapi.ObjectMeta{Name: "binding-one-subject", Namespace: "ns2"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{},
				}},
				ktestclient.DeleteActionImpl{ActionImpl: ktestclient.ActionImpl{Verb: "delete", Resource: "users"}, Name: "bob"},
			},
		},
		{
			name: "sccs",
			user: "bob",
			objects: []runtime.Object{
				&kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{Name: "scc-no-subjects"},
					Users:      []string{},
				},
				&kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{Name: "scc-one-subject"},
					Users:      []string{"bob"},
				},
				&kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{Name: "scc-mismatched-subjects"},
					Users:      []string{"bob2"},
					Groups:     []string{"bob"},
				},
			},
			expected: []interface{}{
				ktestclient.UpdateActionImpl{ActionImpl: ktestclient.ActionImpl{Verb: "update", Resource: "securitycontextconstraints"}, Object: &kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{Name: "scc-one-subject"},
					Users:      []string{},
				}},
				ktestclient.DeleteActionImpl{ActionImpl: ktestclient.ActionImpl{Verb: "delete", Resource: "users"}, Name: "bob"},
			},
		},
		{
			name: "identities",
			user: "bob",
			objects: []runtime.Object{
				&authenticationapi.Identity{
					ObjectMeta: kapi.ObjectMeta{Name: "identity-no-user"},
					User:       kapi.ObjectReference{},
				},
				&authenticationapi.Identity{
					ObjectMeta: kapi.ObjectMeta{Name: "identity-matching-user"},
					User:       kapi.ObjectReference{Name: "bob"},
				},
				&authenticationapi.Identity{
					ObjectMeta: kapi.ObjectMeta{Name: "identity-different-uid"},
					User:       kapi.ObjectReference{Name: "bob", UID: "123"},
				},
				&authenticationapi.Identity{
					ObjectMeta: kapi.ObjectMeta{Name: "identity-different-user"},
					User:       kapi.ObjectReference{Name: "bob2"},
				},
			},
			expected: []interface{}{
				// Make sure identities are not messed with, only the user is removed
				ktestclient.DeleteActionImpl{ActionImpl: ktestclient.ActionImpl{Verb: "delete", Resource: "users"}, Name: "bob"},
			},
		},
		{
			name: "groups",
			user: "bob",
			objects: []runtime.Object{
				&authenticationapi.Group{
					ObjectMeta: kapi.ObjectMeta{Name: "group-no-users"},
					Users:      []string{},
				},
				&authenticationapi.Group{
					ObjectMeta: kapi.ObjectMeta{Name: "group-one-user"},
					Users:      []string{"bob"},
				},
				&authenticationapi.Group{
					ObjectMeta: kapi.ObjectMeta{Name: "group-multiple-users"},
					Users:      []string{"bob2", "bob", "steve"},
				},
				&authenticationapi.Group{
					ObjectMeta: kapi.ObjectMeta{Name: "group-mismatched-users"},
					Users:      []string{"bob2", "steve"},
				},
			},
			expected: []interface{}{
				ktestclient.UpdateActionImpl{ActionImpl: ktestclient.ActionImpl{Verb: "update", Resource: "groups"}, Object: &authenticationapi.Group{
					ObjectMeta: kapi.ObjectMeta{Name: "group-one-user"},
					Users:      []string{},
				}},
				ktestclient.UpdateActionImpl{ActionImpl: ktestclient.ActionImpl{Verb: "update", Resource: "groups"}, Object: &authenticationapi.Group{
					ObjectMeta: kapi.ObjectMeta{Name: "group-multiple-users"},
					Users:      []string{"bob2", "steve"},
				}},
				ktestclient.DeleteActionImpl{ActionImpl: ktestclient.ActionImpl{Verb: "delete", Resource: "users"}, Name: "bob"},
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

		reaper := NewUserReaper(tc, tc, tc, tc, ktc)
		err := reaper.Stop("", test.user, 0, nil)
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

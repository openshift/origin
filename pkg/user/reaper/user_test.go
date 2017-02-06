package reaper

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/davecgh/go-spew/spew"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client/testclient"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	authenticationapi "github.com/openshift/origin/pkg/user/api"
)

var (
	usersResource                     = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "users"}
	securityContextContraintsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "securitycontextconstraints"}
	oAuthClientAuthorizationsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "oauthclientauthorizations"}
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
				core.DeleteActionImpl{ActionImpl: core.ActionImpl{Verb: "delete", Resource: usersResource}, Name: "bob"},
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
				core.UpdateActionImpl{ActionImpl: core.ActionImpl{Verb: "update", Resource: clusterRoleBindingsResource}, Object: &authorizationapi.ClusterRoleBinding{
					ObjectMeta: kapi.ObjectMeta{Name: "binding-one-subject"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{},
				}},
				core.DeleteActionImpl{ActionImpl: core.ActionImpl{Verb: "delete", Resource: usersResource}, Name: "bob"},
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
				core.UpdateActionImpl{ActionImpl: core.ActionImpl{Verb: "update", Resource: roleBindingsResource, Namespace: "ns2"}, Object: &authorizationapi.RoleBinding{
					ObjectMeta: kapi.ObjectMeta{Name: "binding-one-subject", Namespace: "ns2"},
					RoleRef:    kapi.ObjectReference{Name: "role"},
					Subjects:   []kapi.ObjectReference{},
				}},
				core.DeleteActionImpl{ActionImpl: core.ActionImpl{Verb: "delete", Resource: usersResource}, Name: "bob"},
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
				core.UpdateActionImpl{ActionImpl: core.ActionImpl{Verb: "update", Resource: securityContextContraintsResource}, Object: &kapi.SecurityContextConstraints{
					ObjectMeta: kapi.ObjectMeta{Name: "scc-one-subject"},
					Users:      []string{},
				}},
				core.DeleteActionImpl{ActionImpl: core.ActionImpl{Verb: "delete", Resource: usersResource}, Name: "bob"},
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
				core.DeleteActionImpl{ActionImpl: core.ActionImpl{Verb: "delete", Resource: usersResource}, Name: "bob"},
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
				core.UpdateActionImpl{ActionImpl: core.ActionImpl{Verb: "update", Resource: groupsResource}, Object: &authenticationapi.Group{
					ObjectMeta: kapi.ObjectMeta{Name: "group-one-user"},
					Users:      []string{},
				}},
				core.UpdateActionImpl{ActionImpl: core.ActionImpl{Verb: "update", Resource: groupsResource}, Object: &authenticationapi.Group{
					ObjectMeta: kapi.ObjectMeta{Name: "group-multiple-users"},
					Users:      []string{"bob2", "steve"},
				}},
				core.DeleteActionImpl{ActionImpl: core.ActionImpl{Verb: "delete", Resource: usersResource}, Name: "bob"},
			},
		},
		{
			name: "oauth client authorizations",
			user: "bob",
			objects: []runtime.Object{
				&oauthapi.OAuthClientAuthorization{
					ObjectMeta: kapi.ObjectMeta{Name: "other-user"},
					UserName:   "alice",
					UserUID:    "123",
				},
				&oauthapi.OAuthClientAuthorization{
					ObjectMeta: kapi.ObjectMeta{Name: "bob-authorization-1"},
					UserName:   "bob",
					UserUID:    "234",
				},
				&oauthapi.OAuthClientAuthorization{
					ObjectMeta: kapi.ObjectMeta{Name: "bob-authorization-2"},
					UserName:   "bob",
					UserUID:    "345",
				},
			},
			expected: []interface{}{
				core.DeleteActionImpl{ActionImpl: core.ActionImpl{Verb: "delete", Resource: oAuthClientAuthorizationsResource}, Name: "bob-authorization-1"},
				core.DeleteActionImpl{ActionImpl: core.ActionImpl{Verb: "delete", Resource: oAuthClientAuthorizationsResource}, Name: "bob-authorization-2"},
				core.DeleteActionImpl{ActionImpl: core.ActionImpl{Verb: "delete", Resource: usersResource}, Name: "bob"},
			},
		},
	}

	for _, test := range tests {
		tc := testclient.NewSimpleFake(test.objects...)
		ktc := fake.NewSimpleClientset(test.objects...)

		actual := []interface{}{}
		oreactor := func(action core.Action) (handled bool, ret runtime.Object, err error) {
			actual = append(actual, action)
			return false, nil, nil
		}
		kreactor := func(action core.Action) (handled bool, ret runtime.Object, err error) {
			actual = append(actual, action)
			return false, nil, nil
		}

		tc.PrependReactor("update", "*", oreactor)
		tc.PrependReactor("delete", "*", oreactor)
		ktc.PrependReactor("update", "*", kreactor)
		ktc.PrependReactor("delete", "*", kreactor)

		reaper := NewUserReaper(tc, tc, tc, tc, tc, ktc.Core())
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

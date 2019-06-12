package auth

import (
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clienttesting "k8s.io/client-go/testing"

	authv1 "github.com/openshift/api/authorization/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	securityv1 "github.com/openshift/api/security/v1"
	userv1 "github.com/openshift/api/user/v1"
	fakeauthclient "github.com/openshift/client-go/authorization/clientset/versioned/fake"
	fakeauthv1client "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1/fake"
	fakeoauthclient "github.com/openshift/client-go/oauth/clientset/versioned/fake"
	fakeoauthv1client "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1/fake"
	fakesecurityclient "github.com/openshift/client-go/security/clientset/versioned/fake"
	fakesecurityv1client "github.com/openshift/client-go/security/clientset/versioned/typed/security/v1/fake"
	fakeuserclient "github.com/openshift/client-go/user/clientset/versioned/fake"
	fakeuserv1client "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1/fake"
)

var (
	securityContextContraintsResource = schema.GroupVersionResource{Group: "security.openshift.io", Version: "v1", Resource: "securitycontextconstraints"}
	oAuthClientAuthorizationsResource = schema.GroupVersionResource{Group: "oauth.openshift.io", Version: "v1", Resource: "oauthclientauthorizations"}
)

func TestUserReaper(t *testing.T) {
	tests := []struct {
		name         string
		user         string
		authObjects  []runtime.Object
		oauthObjects []runtime.Object
		userObjects  []runtime.Object
		sccs         []runtime.Object
		expected     []interface{}
	}{
		{
			name:     "no objects",
			user:     "bob",
			expected: []interface{}{},
		},
		{
			name: "cluster bindings",
			user: "bob",
			authObjects: []runtime.Object{
				&authv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-no-subjects"},
					RoleRef:    corev1.ObjectReference{Name: "role"},
					Subjects:   []corev1.ObjectReference{},
				},
				&authv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-one-subject"},
					RoleRef:    corev1.ObjectReference{Name: "role"},
					Subjects:   []corev1.ObjectReference{{Name: "bob", Kind: "User"}},
				},
				&authv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-mismatched-subject"},
					RoleRef:    corev1.ObjectReference{Name: "role"},
					Subjects:   []corev1.ObjectReference{{Name: "bob"}, {Name: "bob", Kind: "Group"}, {Name: "bob", Kind: "Other"}},
				},
			},
			expected: []interface{}{
				clienttesting.UpdateActionImpl{ActionImpl: clienttesting.ActionImpl{Verb: "update", Resource: clusterRoleBindingsResource}, Object: &authv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-one-subject"},
					RoleRef:    corev1.ObjectReference{Name: "role"},
					Subjects:   []corev1.ObjectReference{},
				}},
			},
		},
		{
			name: "namespaced bindings",
			user: "bob",
			authObjects: []runtime.Object{
				&authv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-no-subjects", Namespace: "ns1"},
					RoleRef:    corev1.ObjectReference{Name: "role"},
					Subjects:   []corev1.ObjectReference{},
				},
				&authv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-one-subject", Namespace: "ns2"},
					RoleRef:    corev1.ObjectReference{Name: "role"},
					Subjects:   []corev1.ObjectReference{{Name: "bob", Kind: "User"}},
				},
				&authv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-mismatched-subject", Namespace: "ns3"},
					RoleRef:    corev1.ObjectReference{Name: "role"},
					Subjects:   []corev1.ObjectReference{{Name: "bob"}, {Name: "bob", Kind: "Group"}, {Name: "bob", Kind: "Other"}},
				},
			},
			expected: []interface{}{
				clienttesting.UpdateActionImpl{ActionImpl: clienttesting.ActionImpl{Verb: "update", Resource: roleBindingsResource, Namespace: "ns2"}, Object: &authv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-one-subject", Namespace: "ns2"},
					RoleRef:    corev1.ObjectReference{Name: "role"},
					Subjects:   []corev1.ObjectReference{},
				}},
			},
		},
		{
			name: "sccs",
			user: "bob",
			sccs: []runtime.Object{
				&securityv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{Name: "scc-no-subjects"},
					Users:      []string{},
				},
				&securityv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{Name: "scc-one-subject"},
					Users:      []string{"bob"},
				},
				&securityv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{Name: "scc-mismatched-subjects"},
					Users:      []string{"bob2"},
					Groups:     []string{"bob"},
				},
			},
			expected: []interface{}{
				clienttesting.UpdateActionImpl{ActionImpl: clienttesting.ActionImpl{Verb: "update", Resource: securityContextContraintsResource}, Object: &securityv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{Name: "scc-one-subject"},
					Users:      []string{},
				}},
			},
		},
		{
			name: "identities",
			user: "bob",
			userObjects: []runtime.Object{
				&userv1.Identity{
					ObjectMeta: metav1.ObjectMeta{Name: "identity-no-user"},
					User:       corev1.ObjectReference{},
				},
				&userv1.Identity{
					ObjectMeta: metav1.ObjectMeta{Name: "identity-matching-user"},
					User:       corev1.ObjectReference{Name: "bob"},
				},
				&userv1.Identity{
					ObjectMeta: metav1.ObjectMeta{Name: "identity-different-uid"},
					User:       corev1.ObjectReference{Name: "bob", UID: "123"},
				},
				&userv1.Identity{
					ObjectMeta: metav1.ObjectMeta{Name: "identity-different-user"},
					User:       corev1.ObjectReference{Name: "bob2"},
				},
			},
			expected: []interface{}{},
		},
		{
			name: "groups",
			user: "bob",
			userObjects: []runtime.Object{
				&userv1.Group{
					ObjectMeta: metav1.ObjectMeta{Name: "group-no-users"},
					Users:      []string{},
				},
				&userv1.Group{
					ObjectMeta: metav1.ObjectMeta{Name: "group-one-user"},
					Users:      []string{"bob"},
				},
				&userv1.Group{
					ObjectMeta: metav1.ObjectMeta{Name: "group-multiple-users"},
					Users:      []string{"bob2", "bob", "steve"},
				},
				&userv1.Group{
					ObjectMeta: metav1.ObjectMeta{Name: "group-mismatched-users"},
					Users:      []string{"bob2", "steve"},
				},
			},
			expected: []interface{}{
				clienttesting.UpdateActionImpl{ActionImpl: clienttesting.ActionImpl{Verb: "update", Resource: groupsResource}, Object: &userv1.Group{
					ObjectMeta: metav1.ObjectMeta{Name: "group-one-user"},
					Users:      []string{},
				}},
				clienttesting.UpdateActionImpl{ActionImpl: clienttesting.ActionImpl{Verb: "update", Resource: groupsResource}, Object: &userv1.Group{
					ObjectMeta: metav1.ObjectMeta{Name: "group-multiple-users"},
					Users:      []string{"bob2", "steve"},
				}},
			},
		},
		{
			name: "oauth client authorizations",
			user: "bob",
			oauthObjects: []runtime.Object{
				&oauthv1.OAuthClientAuthorization{
					ObjectMeta: metav1.ObjectMeta{Name: "other-user"},
					UserName:   "alice",
					UserUID:    "123",
				},
				&oauthv1.OAuthClientAuthorization{
					ObjectMeta: metav1.ObjectMeta{Name: "bob-authorization-1"},
					UserName:   "bob",
					UserUID:    "234",
				},
				&oauthv1.OAuthClientAuthorization{
					ObjectMeta: metav1.ObjectMeta{Name: "bob-authorization-2"},
					UserName:   "bob",
					UserUID:    "345",
				},
			},
			expected: []interface{}{
				clienttesting.DeleteActionImpl{ActionImpl: clienttesting.ActionImpl{Verb: "delete", Resource: oAuthClientAuthorizationsResource}, Name: "bob-authorization-1"},
				clienttesting.DeleteActionImpl{ActionImpl: clienttesting.ActionImpl{Verb: "delete", Resource: oAuthClientAuthorizationsResource}, Name: "bob-authorization-2"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authFake := &fakeauthv1client.FakeAuthorizationV1{Fake: &(fakeauthclient.NewSimpleClientset(test.authObjects...).Fake)}
			oauthFake := &fakeoauthv1client.FakeOauthV1{Fake: &(fakeoauthclient.NewSimpleClientset(test.oauthObjects...).Fake)}
			userFake := &fakeuserv1client.FakeUserV1{Fake: &(fakeuserclient.NewSimpleClientset(test.userObjects...).Fake)}
			securityFake := &fakesecurityv1client.FakeSecurityV1{Fake: &(fakesecurityclient.NewSimpleClientset(test.sccs...).Fake)}

			actual := []interface{}{}
			oreactor := func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
				actual = append(actual, action)
				return false, nil, nil
			}
			kreactor := func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
				actual = append(actual, action)
				return false, nil, nil
			}

			authFake.PrependReactor("update", "*", oreactor)
			userFake.PrependReactor("update", "*", oreactor)
			oauthFake.PrependReactor("update", "*", oreactor)
			authFake.PrependReactor("delete", "*", oreactor)
			userFake.PrependReactor("delete", "*", oreactor)
			oauthFake.PrependReactor("delete", "*", oreactor)
			securityFake.Fake.PrependReactor("update", "*", kreactor)
			securityFake.Fake.PrependReactor("delete", "*", kreactor)

			err := reapForUser(userFake, authFake, oauthFake, securityFake.SecurityContextConstraints(), test.user, ioutil.Discard)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(test.expected, actual) {
				for i, x := range test.expected {
					t.Logf("Expected %d: %s", i, spew.Sprint(x))
				}
				for i, x := range actual {
					t.Logf("Actual %d:   %s", i, spew.Sprint(x))
				}
				t.Errorf("unexpected actions")
			}
		})
	}
}

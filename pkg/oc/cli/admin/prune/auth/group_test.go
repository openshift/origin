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
	securityv1 "github.com/openshift/api/security/v1"
	fakeauthclient "github.com/openshift/client-go/authorization/clientset/versioned/fake"
	fakeauthv1client "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1/fake"
	fakesecurityclient "github.com/openshift/client-go/security/clientset/versioned/fake"
	fakesecurityv1client "github.com/openshift/client-go/security/clientset/versioned/typed/security/v1/fake"
	fakeuserv1client "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1/fake"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

var (
	groupsResource              = schema.GroupVersionResource{Group: "user.openshift.io", Version: "v1", Resource: "groups"}
	clusterRoleBindingsResource = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "v1", Resource: "clusterrolebindings"}
	roleBindingsResource        = schema.GroupVersionResource{Group: "authorization.openshift.io", Version: "v1", Resource: "rolebindings"}
	sccResource                 = schema.GroupVersionResource{Group: "security.openshift.io", Version: "v1", Resource: "securitycontextconstraints"}
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
			name:     "no objects",
			group:    "mygroup",
			objects:  []runtime.Object{},
			expected: []interface{}{},
		},
		{
			name:  "cluster bindings",
			group: "mygroup",
			objects: []runtime.Object{
				&authv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-no-subjects"},
					RoleRef:    corev1.ObjectReference{Name: "role"},
					Subjects:   []corev1.ObjectReference{},
				},
				&authv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-one-subject"},
					RoleRef:    corev1.ObjectReference{Name: "role"},
					Subjects:   []corev1.ObjectReference{{Name: "mygroup", Kind: "Group"}},
				},
				&authv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-mismatched-subject"},
					RoleRef:    corev1.ObjectReference{Name: "role"},
					Subjects:   []corev1.ObjectReference{{Name: "mygroup"}, {Name: "mygroup", Kind: "User"}, {Name: "mygroup", Kind: "Other"}},
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
			name:  "namespaced bindings",
			group: "mygroup",
			objects: []runtime.Object{
				&authv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-no-subjects", Namespace: "ns1"},
					RoleRef:    corev1.ObjectReference{Name: "role"},
					Subjects:   []corev1.ObjectReference{},
				},
				&authv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-one-subject", Namespace: "ns2"},
					RoleRef:    corev1.ObjectReference{Name: "role"},
					Subjects:   []corev1.ObjectReference{{Name: "mygroup", Kind: "Group"}},
				},
				&authv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-mismatched-subject", Namespace: "ns3"},
					RoleRef:    corev1.ObjectReference{Name: "role"},
					Subjects:   []corev1.ObjectReference{{Name: "mygroup"}, {Name: "mygroup", Kind: "User"}, {Name: "mygroup", Kind: "Other"}},
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
			name:  "sccs",
			group: "mygroup",
			sccs: []runtime.Object{
				&securityv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{Name: "scc-no-subjects"},
					Groups:     []string{},
				},
				&securityv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{Name: "scc-one-subject"},
					Groups:     []string{"mygroup"},
				},
				&securityv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{Name: "scc-mismatched-subjects"},
					Users:      []string{"mygroup"},
					Groups:     []string{"mygroup2"},
				},
			},
			expected: []interface{}{
				clienttesting.UpdateActionImpl{ActionImpl: clienttesting.ActionImpl{Verb: "update", Resource: sccResource}, Object: &securityv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{Name: "scc-one-subject"},
					Groups:     []string{},
				}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authFake := &fakeauthv1client.FakeAuthorizationV1{Fake: &(fakeauthclient.NewSimpleClientset(test.objects...).Fake)}
			userFake := &fakeuserv1client.FakeUserV1{Fake: &clienttesting.Fake{}}
			securityFake := &fakesecurityv1client.FakeSecurityV1{Fake: &(fakesecurityclient.NewSimpleClientset(test.sccs...).Fake)}

			actual := []interface{}{}
			oreactor := func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
				t.Logf("oreactor: %#v", action)
				actual = append(actual, action)
				return false, nil, nil
			}
			kreactor := func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
				t.Logf("kreactor: %#v", action)
				actual = append(actual, action)
				return false, nil, nil
			}

			authFake.PrependReactor("update", "*", oreactor)
			userFake.PrependReactor("update", "*", oreactor)
			authFake.PrependReactor("delete", "*", oreactor)
			userFake.PrependReactor("delete", "*", oreactor)
			securityFake.Fake.PrependReactor("update", "*", kreactor)
			securityFake.Fake.PrependReactor("delete", "*", kreactor)

			err := reapForGroup(authFake, securityFake.SecurityContextConstraints(), test.group, ioutil.Discard)
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
				t.Error("unexpected actions")
			}
		})
	}
}

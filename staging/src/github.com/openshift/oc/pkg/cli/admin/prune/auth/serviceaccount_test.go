package auth

import (
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"

	authv1 "github.com/openshift/api/authorization/v1"
	securityv1 "github.com/openshift/api/security/v1"
	fakeauthclient "github.com/openshift/client-go/authorization/clientset/versioned/fake"
	fakeauthv1client "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1/fake"
	fakesecurityclient "github.com/openshift/client-go/security/clientset/versioned/fake"
	fakesecurityv1client "github.com/openshift/client-go/security/clientset/versioned/typed/security/v1/fake"
)

func TestServiceAccountReaper(t *testing.T) {
	tests := []struct {
		name           string
		serviceaccount string
		namespace      string
		authObjects    []runtime.Object
		sccs           []runtime.Object
		expected       []interface{}
	}{
		{
			name:           "no objects",
			serviceaccount: "foosa",
			namespace:      "foons",
			expected:       []interface{}{},
		},
		{
			name:           "cluster bindings",
			serviceaccount: "foosa",
			namespace:      "foons",
			authObjects: []runtime.Object{
				&authv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-no-subjects"},
					RoleRef:    corev1.ObjectReference{Name: "role"},
					Subjects:   []corev1.ObjectReference{},
				},
				&authv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-one-subject"},
					RoleRef:    corev1.ObjectReference{Name: "role"},
					Subjects:   []corev1.ObjectReference{{Name: "foosa", Kind: "ServiceAccount", Namespace: "foons"}},
				},
				&authv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-mismatched-subject"},
					RoleRef:    corev1.ObjectReference{Name: "role"},
					Subjects:   []corev1.ObjectReference{{Name: "foosa"}, {Name: "foosa", Kind: "Group"}, {Name: "foosa", Kind: "Other"}},
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
			name:           "namespaced bindings",
			serviceaccount: "foosa",
			namespace:      "foons",
			authObjects: []runtime.Object{
				&authv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-no-subjects", Namespace: "ns1"},
					RoleRef:    corev1.ObjectReference{Name: "role"},
					Subjects:   []corev1.ObjectReference{},
				},
				&authv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-one-subject", Namespace: "ns2"},
					RoleRef:    corev1.ObjectReference{Name: "role"},
					Subjects:   []corev1.ObjectReference{{Name: "foosa", Kind: "ServiceAccount", Namespace: "foons"}},
				},
				&authv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "binding-mismatched-subject", Namespace: "ns3"},
					RoleRef:    corev1.ObjectReference{Name: "role"},
					Subjects:   []corev1.ObjectReference{{Name: "foosa"}, {Name: "foosa", Kind: "Group"}, {Name: "foosa", Kind: "Other"}},
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
			name:           "sccs",
			serviceaccount: "foosa",
			namespace:      "foons",
			sccs: []runtime.Object{
				&securityv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{Name: "scc-no-subjects"},
					Users:      []string{},
				},
				&securityv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{Name: "scc-one-subject"},
					Users:      []string{"system:serviceaccount:foons:foosa"},
				},
				&securityv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{Name: "scc-mismatched-subjects"},
					Users:      []string{"foouser"},
					Groups:     []string{"foogroup"},
				},
			},
			expected: []interface{}{
				clienttesting.UpdateActionImpl{ActionImpl: clienttesting.ActionImpl{Verb: "update", Resource: securityContextContraintsResource}, Object: &securityv1.SecurityContextConstraints{
					ObjectMeta: metav1.ObjectMeta{Name: "scc-one-subject"},
					Users:      []string{},
				}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authFake := &fakeauthv1client.FakeAuthorizationV1{Fake: &(fakeauthclient.NewSimpleClientset(test.authObjects...).Fake)}
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
			authFake.PrependReactor("delete", "*", oreactor)
			securityFake.Fake.PrependReactor("update", "*", kreactor)
			securityFake.Fake.PrependReactor("delete", "*", kreactor)

			err := reapForServiceAccount(authFake, securityFake.SecurityContextConstraints(), test.namespace, test.serviceaccount, ioutil.Discard)
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

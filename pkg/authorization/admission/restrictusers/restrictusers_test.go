package restrictusers

import (
	"fmt"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/rbac"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	kadmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"

	authorizationapi "github.com/openshift/api/authorization/v1"
	userapi "github.com/openshift/api/user/v1"
	fakeauthorizationclient "github.com/openshift/client-go/authorization/clientset/versioned/fake"
	fakeuserclient "github.com/openshift/client-go/user/clientset/versioned/fake"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
)

func TestAdmission(t *testing.T) {
	var (
		userAlice = userapi.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "Alice",
				Labels: map[string]string{"foo": "bar"},
			},
		}
		userAliceSubj = rbac.Subject{
			Kind: rbac.UserKind,
			Name: "Alice",
		}

		userBob = userapi.User{
			ObjectMeta: metav1.ObjectMeta{Name: "Bob"},
			Groups:     []string{"group"},
		}
		userBobSubj = rbac.Subject{
			Kind: rbac.UserKind,
			Name: "Bob",
		}

		group = userapi.Group{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "group",
				Labels: map[string]string{"baz": "quux"},
			},
			Users: []string{userBobSubj.Name},
		}
		groupSubj = rbac.Subject{
			Kind: rbac.GroupKind,
			Name: "group",
		}

		serviceaccount = kapi.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "namespace",
				Name:      "serviceaccount",
				Labels:    map[string]string{"xyzzy": "thud"},
			},
		}
		serviceaccountSubj = rbac.Subject{
			Kind:      rbac.ServiceAccountKind,
			Namespace: "namespace",
			Name:      "serviceaccount",
		}
	)

	testCases := []struct {
		name        string
		expectedErr string

		object               runtime.Object
		oldObject            runtime.Object
		kind                 schema.GroupVersionKind
		resource             schema.GroupVersionResource
		namespace            string
		subresource          string
		kubeObjects          []runtime.Object
		authorizationObjects []runtime.Object
		userObjects          []runtime.Object
	}{
		{
			name: "ignore (allow) if subresource is nonempty",
			object: &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []rbac.Subject{userAliceSubj},
			},
			oldObject: &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []rbac.Subject{},
			},
			kind:        rbac.Kind("RoleBinding").WithVersion("version"),
			resource:    rbac.Resource("rolebindings").WithVersion("version"),
			namespace:   "namespace",
			subresource: "subresource",
			kubeObjects: []runtime.Object{
				&kapi.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace",
					},
				},
			},
		},
		{
			name: "ignore (allow) cluster-scoped rolebinding",
			object: &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []rbac.Subject{userAliceSubj},
				RoleRef:  rbac.RoleRef{Name: "name"},
			},
			oldObject: &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []rbac.Subject{},
			},
			kind:        rbac.Kind("RoleBinding").WithVersion("version"),
			resource:    rbac.Resource("rolebindings").WithVersion("version"),
			namespace:   "",
			subresource: "",
			kubeObjects: []runtime.Object{
				&kapi.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace",
					},
				},
			},
		},
		{
			name: "allow if the namespace has no rolebinding restrictions",
			object: &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []rbac.Subject{
					userAliceSubj,
					userBobSubj,
					groupSubj,
					serviceaccountSubj,
				},
				RoleRef: rbac.RoleRef{Name: "name"},
			},
			oldObject: &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []rbac.Subject{},
				RoleRef:  rbac.RoleRef{Name: "name"},
			},
			kind:        rbac.Kind("RoleBinding").WithVersion("version"),
			resource:    rbac.Resource("rolebindings").WithVersion("version"),
			namespace:   "namespace",
			subresource: "",
			kubeObjects: []runtime.Object{
				&kapi.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace",
					},
				},
			},
		},
		{
			name: "allow if any rolebinding with the subject already exists",
			object: &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []rbac.Subject{
					userAliceSubj,
					groupSubj,
					serviceaccountSubj,
				},
				RoleRef: rbac.RoleRef{Name: "name"},
			},
			oldObject: &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []rbac.Subject{
					userAliceSubj,
					groupSubj,
					serviceaccountSubj,
				},
				RoleRef: rbac.RoleRef{Name: "name"},
			},
			kind:        rbac.Kind("RoleBinding").WithVersion("version"),
			resource:    rbac.Resource("rolebindings").WithVersion("version"),
			namespace:   "namespace",
			subresource: "",
			kubeObjects: []runtime.Object{
				&kapi.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace",
					},
				},
			},
			authorizationObjects: []runtime.Object{
				&authorizationapi.RoleBindingRestriction{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bogus-matcher",
						Namespace: "namespace",
					},
					Spec: authorizationapi.RoleBindingRestrictionSpec{
						UserRestriction: &authorizationapi.UserRestriction{},
					},
				},
			},
		},
		{
			name: "allow a user, group, or service account in a rolebinding if a literal matches",
			object: &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []rbac.Subject{
					userAliceSubj,
					serviceaccountSubj,
					groupSubj,
				},
				RoleRef: rbac.RoleRef{Name: "name"},
			},
			oldObject: &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []rbac.Subject{},
				RoleRef:  rbac.RoleRef{Name: "name"},
			},
			kind:        rbac.Kind("RoleBinding").WithVersion("version"),
			resource:    rbac.Resource("rolebindings").WithVersion("version"),
			namespace:   "namespace",
			subresource: "",
			kubeObjects: []runtime.Object{
				&kapi.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace",
					},
				},
			},
			authorizationObjects: []runtime.Object{
				&authorizationapi.RoleBindingRestriction{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "match-users",
						Namespace: "namespace",
					},
					Spec: authorizationapi.RoleBindingRestrictionSpec{
						UserRestriction: &authorizationapi.UserRestriction{
							Users: []string{userAlice.Name},
						},
					},
				},
				&authorizationapi.RoleBindingRestriction{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "match-groups",
						Namespace: "namespace",
					},
					Spec: authorizationapi.RoleBindingRestrictionSpec{
						GroupRestriction: &authorizationapi.GroupRestriction{
							Groups: []string{group.Name},
						},
					},
				},
				&authorizationapi.RoleBindingRestriction{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "match-serviceaccounts",
						Namespace: "namespace",
					},
					Spec: authorizationapi.RoleBindingRestrictionSpec{
						ServiceAccountRestriction: &authorizationapi.ServiceAccountRestriction{
							ServiceAccounts: []authorizationapi.ServiceAccountReference{
								{
									Name:      serviceaccount.Name,
									Namespace: serviceaccount.Namespace,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "prohibit user without a matching user literal",
			expectedErr: fmt.Sprintf("rolebindings to %s %q are not allowed",
				userAliceSubj.Kind, userAliceSubj.Name),
			object: &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []rbac.Subject{
					userAliceSubj,
				},
				RoleRef: rbac.RoleRef{Name: "name"},
			},
			oldObject: &rbac.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []rbac.Subject{},
				RoleRef:  rbac.RoleRef{Name: "name"},
			},
			kind:        rbac.Kind("RoleBinding").WithVersion("version"),
			resource:    rbac.Resource("rolebindings").WithVersion("version"),
			namespace:   "namespace",
			subresource: "",
			kubeObjects: []runtime.Object{
				&kapi.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace",
					},
				},
			},
			authorizationObjects: []runtime.Object{
				&authorizationapi.RoleBindingRestriction{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "match-users-bob",
						Namespace: "namespace",
					},
					Spec: authorizationapi.RoleBindingRestrictionSpec{
						UserRestriction: &authorizationapi.UserRestriction{
							Users: []string{userBobSubj.Name},
						},
					},
				},
			},
			userObjects: []runtime.Object{
				&userAlice,
				&userBob,
			},
		},
	}

	stopCh := make(chan struct{})
	defer close(stopCh)

	for _, tc := range testCases {
		kclientset := fake.NewSimpleClientset(tc.kubeObjects...)
		fakeUserClient := fakeuserclient.NewSimpleClientset(tc.userObjects...)
		fakeAuthorizationClient := fakeauthorizationclient.NewSimpleClientset(tc.authorizationObjects...)

		plugin, err := NewRestrictUsersAdmission()
		if err != nil {
			t.Errorf("unexpected error initializing admission plugin: %v", err)
		}

		plugin.(kadmission.WantsInternalKubeClientSet).SetInternalKubeClientSet(kclientset)
		plugin.(oadmission.WantsOpenshiftInternalAuthorizationClient).SetOpenshiftInternalAuthorizationClient(fakeAuthorizationClient)
		plugin.(oadmission.WantsOpenshiftInternalUserClient).SetOpenshiftInternalUserClient(fakeUserClient)
		plugin.(*restrictUsersAdmission).groupCache = fakeGroupCache{}

		err = admission.ValidateInitialization(plugin)
		if err != nil {
			t.Errorf("unexpected error validating admission plugin: %v", err)
		}

		attributes := admission.NewAttributesRecord(
			tc.object,
			tc.oldObject,
			tc.kind,
			tc.namespace,
			tc.name,
			tc.resource,
			tc.subresource,
			admission.Create,
			&user.DefaultInfo{},
		)

		err = plugin.(admission.MutationInterface).Admit(attributes)
		switch {
		case len(tc.expectedErr) == 0 && err == nil:
		case len(tc.expectedErr) == 0 && err != nil:
			t.Errorf("%s: unexpected error: %v", tc.name, err)
		case len(tc.expectedErr) != 0 && err == nil:
			t.Errorf("%s: missing error: %v", tc.name, tc.expectedErr)
		case len(tc.expectedErr) != 0 && err != nil &&
			!strings.Contains(err.Error(), tc.expectedErr):
			t.Errorf("%s: missing error: expected %v, got %v",
				tc.name, tc.expectedErr, err)
		}
	}
}

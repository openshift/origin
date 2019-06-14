package restrictusers

import (
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/apis/rbac"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	userapi "github.com/openshift/api/user/v1"
	authorizationv1listers "github.com/openshift/client-go/authorization/listers/authorization/v1"
	fakeuserclient "github.com/openshift/client-go/user/clientset/versioned/fake"
)

func newTestAdmission(lister authorizationv1listers.RoleBindingRestrictionLister, kclient *fake.Clientset, userclient *fakeuserclient.Clientset, groupcache GroupCache) admission.Interface {
	return &restrictUsersAdmission{
		Handler:    admission.NewHandler(admission.Create),
		rbrLister:  lister,
		rbrSynced:  func() bool { return true },
		userClient: userclient,
		kubeClient: kclient,
		groupCache: groupcache,
	}
}

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

		serviceaccount = corev1.ServiceAccount{
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

		object      runtime.Object
		oldObject   runtime.Object
		kind        schema.GroupVersionKind
		resource    schema.GroupVersionResource
		namespace   string
		subresource string
		rbrs        []*authorizationv1.RoleBindingRestriction
		kubeObjects []runtime.Object
		userObjects []runtime.Object
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
			rbrs:        []*authorizationv1.RoleBindingRestriction{},
			kubeObjects: []runtime.Object{
				&corev1.Namespace{
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
			rbrs:        []*authorizationv1.RoleBindingRestriction{},
			kubeObjects: []runtime.Object{
				&corev1.Namespace{
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
			rbrs:        []*authorizationv1.RoleBindingRestriction{},
			kubeObjects: []runtime.Object{
				&corev1.Namespace{
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
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace",
					},
				},
			},
			rbrs: []*authorizationv1.RoleBindingRestriction{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bogus-matcher",
						Namespace: "namespace",
					},
					Spec: authorizationv1.RoleBindingRestrictionSpec{
						UserRestriction: &authorizationv1.UserRestriction{},
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
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace",
					},
				},
			},
			rbrs: []*authorizationv1.RoleBindingRestriction{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "match-users",
						Namespace: "namespace",
					},
					Spec: authorizationv1.RoleBindingRestrictionSpec{
						UserRestriction: &authorizationv1.UserRestriction{
							Users: []string{userAlice.Name},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "match-groups",
						Namespace: "namespace",
					},
					Spec: authorizationv1.RoleBindingRestrictionSpec{
						GroupRestriction: &authorizationv1.GroupRestriction{
							Groups: []string{group.Name},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "match-serviceaccounts",
						Namespace: "namespace",
					},
					Spec: authorizationv1.RoleBindingRestrictionSpec{
						ServiceAccountRestriction: &authorizationv1.ServiceAccountRestriction{
							ServiceAccounts: []authorizationv1.ServiceAccountReference{
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
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace",
					},
				},
			},
			rbrs: []*authorizationv1.RoleBindingRestriction{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "match-users-bob",
						Namespace: "namespace",
					},
					Spec: authorizationv1.RoleBindingRestrictionSpec{
						UserRestriction: &authorizationv1.UserRestriction{
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
		indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		for _, rbr := range tc.rbrs {
			indexer.Add(rbr)
		}
		rbrLister := authorizationv1listers.NewRoleBindingRestrictionLister(indexer)
		plugin := newTestAdmission(rbrLister, kclientset, fakeUserClient, fakeGroupCache{})

		err := admission.ValidateInitialization(plugin)
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
			false,
			&user.DefaultInfo{},
		)

		err = plugin.(admission.ValidationInterface).Validate(attributes, nil)
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

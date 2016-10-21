package restrictusers

import (
	"fmt"
	"strings"
	"testing"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/auth/user"
	//kcache "k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	otestclient "github.com/openshift/origin/pkg/client/testclient"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	//"github.com/openshift/origin/pkg/project/cache"
	userapi "github.com/openshift/origin/pkg/user/api"
	usercache "github.com/openshift/origin/pkg/user/cache"
)

func TestAdmission(t *testing.T) {
	var (
		userAlice = userapi.User{
			ObjectMeta: kapi.ObjectMeta{
				Name:   "Alice",
				Labels: map[string]string{"foo": "bar"},
			},
		}
		userAliceRef = kapi.ObjectReference{
			Kind: authorizationapi.UserKind,
			Name: "Alice",
		}

		userBob = userapi.User{
			ObjectMeta: kapi.ObjectMeta{Name: "Bob"},
			Groups:     []string{"group"},
		}
		userBobRef = kapi.ObjectReference{
			Kind: authorizationapi.UserKind,
			Name: "Bob",
		}

		group = userapi.Group{
			ObjectMeta: kapi.ObjectMeta{
				Name:   "group",
				Labels: map[string]string{"baz": "quux"},
			},
			Users: []string{userBobRef.Name},
		}
		groupRef = kapi.ObjectReference{
			Kind: authorizationapi.GroupKind,
			Name: "group",
		}

		serviceaccount = kapi.ServiceAccount{
			ObjectMeta: kapi.ObjectMeta{
				Namespace: "namespace",
				Name:      "serviceaccount",
				Labels:    map[string]string{"xyzzy": "thud"},
			},
		}
		serviceaccountRef = kapi.ObjectReference{
			Kind:      authorizationapi.ServiceAccountKind,
			Namespace: "namespace",
			Name:      "serviceaccount",
		}

		systemuserRef = kapi.ObjectReference{
			Kind: authorizationapi.SystemUserKind,
			Name: "system user",
		}
		systemgroupRef = kapi.ObjectReference{
			Kind: authorizationapi.SystemGroupKind,
			Name: "system group",
		}
	)

	testCases := []struct {
		name        string
		expectedErr string

		object      runtime.Object
		oldObject   runtime.Object
		kind        unversioned.GroupVersionKind
		resource    unversioned.GroupVersionResource
		namespace   string
		subresource string
		objects     []runtime.Object
	}{
		{
			name: "ignore (allow) if subresource is nonempty",
			object: &authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []kapi.ObjectReference{userAliceRef},
			},
			oldObject: &authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []kapi.ObjectReference{},
			},
			kind:        authorizationapi.Kind("RoleBinding").WithVersion("version"),
			resource:    authorizationapi.Resource("rolebindings").WithVersion("version"),
			namespace:   "namespace",
			subresource: "subresource",
			objects: []runtime.Object{
				&kapi.Namespace{
					ObjectMeta: kapi.ObjectMeta{
						Name: "namespace",
					},
				},
			},
		},
		{
			name: "ignore (allow) cluster-scoped rolebinding",
			object: &authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []kapi.ObjectReference{userAliceRef},
				RoleRef:  kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
			},
			oldObject: &authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []kapi.ObjectReference{},
			},
			kind:        authorizationapi.Kind("RoleBinding").WithVersion("version"),
			resource:    authorizationapi.Resource("rolebindings").WithVersion("version"),
			namespace:   "",
			subresource: "",
			objects: []runtime.Object{
				&kapi.Namespace{
					ObjectMeta: kapi.ObjectMeta{
						Name: "namespace",
					},
				},
			},
		},
		{
			name: "allow if the namespace has no rolebinding restrictions",
			object: &authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []kapi.ObjectReference{
					userAliceRef,
					userBobRef,
					groupRef,
					serviceaccountRef,
					systemgroupRef,
					systemuserRef,
				},
				RoleRef: kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
			},
			oldObject: &authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []kapi.ObjectReference{},
				RoleRef:  kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
			},
			kind:        authorizationapi.Kind("RoleBinding").WithVersion("version"),
			resource:    authorizationapi.Resource("rolebindings").WithVersion("version"),
			namespace:   "namespace",
			subresource: "",
			objects: []runtime.Object{
				&kapi.Namespace{
					ObjectMeta: kapi.ObjectMeta{
						Name: "namespace",
					},
				},
			},
		},
		{
			name: "allow if any rolebinding with the subject already exists",
			object: &authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []kapi.ObjectReference{
					userAliceRef,
					groupRef,
					serviceaccountRef,
				},
				RoleRef: kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
			},
			oldObject: &authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []kapi.ObjectReference{
					userAliceRef,
					groupRef,
					serviceaccountRef,
				},
				RoleRef: kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
			},
			kind:        authorizationapi.Kind("RoleBinding").WithVersion("version"),
			resource:    authorizationapi.Resource("rolebindings").WithVersion("version"),
			namespace:   "namespace",
			subresource: "",
			objects: []runtime.Object{
				&kapi.Namespace{
					ObjectMeta: kapi.ObjectMeta{
						Name: "namespace",
					},
				},
				&authorizationapi.RoleBindingRestriction{
					ObjectMeta: kapi.ObjectMeta{
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
			name: "allow a system user, system group, user, group, or service account in a rolebinding if a literal matches",
			object: &authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []kapi.ObjectReference{
					systemuserRef,
					systemgroupRef,
					userAliceRef,
					serviceaccountRef,
					groupRef,
				},
				RoleRef: kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
			},
			oldObject: &authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []kapi.ObjectReference{},
				RoleRef:  kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
			},
			kind:        authorizationapi.Kind("RoleBinding").WithVersion("version"),
			resource:    authorizationapi.Resource("rolebindings").WithVersion("version"),
			namespace:   "namespace",
			subresource: "",
			objects: []runtime.Object{
				&kapi.Namespace{
					ObjectMeta: kapi.ObjectMeta{
						Name: "namespace",
					},
				},
				&authorizationapi.RoleBindingRestriction{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "match-system-users",
						Namespace: "namespace",
					},
					Spec: authorizationapi.RoleBindingRestrictionSpec{
						UserRestriction: &authorizationapi.UserRestriction{
							Users: []string{systemuserRef.Name},
						},
					},
				},
				&authorizationapi.RoleBindingRestriction{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "match-system-groups",
						Namespace: "namespace",
					},
					Spec: authorizationapi.RoleBindingRestrictionSpec{
						GroupRestriction: &authorizationapi.GroupRestriction{
							Groups: []string{systemgroupRef.Name},
						},
					},
				},
				&authorizationapi.RoleBindingRestriction{
					ObjectMeta: kapi.ObjectMeta{
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
					ObjectMeta: kapi.ObjectMeta{
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
					ObjectMeta: kapi.ObjectMeta{
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
				userAliceRef.Kind, userAliceRef.Name),
			object: &authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []kapi.ObjectReference{
					userAliceRef,
				},
				RoleRef: kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
			},
			oldObject: &authorizationapi.RoleBinding{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []kapi.ObjectReference{},
				RoleRef:  kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
			},
			kind:        authorizationapi.Kind("RoleBinding").WithVersion("version"),
			resource:    authorizationapi.Resource("rolebindings").WithVersion("version"),
			namespace:   "namespace",
			subresource: "",
			objects: []runtime.Object{
				&kapi.Namespace{
					ObjectMeta: kapi.ObjectMeta{
						Name: "namespace",
					},
				},
				&userAlice,
				&userBob,
				&authorizationapi.RoleBindingRestriction{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "match-users-bob",
						Namespace: "namespace",
					},
					Spec: authorizationapi.RoleBindingRestrictionSpec{
						UserRestriction: &authorizationapi.UserRestriction{
							Users: []string{userBobRef.Name},
						},
					},
				},
			},
		},
		{
			name: "allow users in a policybinding if a selector matches",
			object: &authorizationapi.PolicyBinding{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "namespace",
					Name:      "policybinding",
				},
				RoleBindings: map[string]*authorizationapi.RoleBinding{
					"rolebinding": {
						ObjectMeta: kapi.ObjectMeta{
							Namespace: "namespace",
							Name:      "rolebinding",
						},
						Subjects: []kapi.ObjectReference{
							userAliceRef,
							userBobRef,
						},
						RoleRef: kapi.ObjectReference{
							Namespace: authorizationapi.PolicyName,
						},
					},
				},
				PolicyRef: kapi.ObjectReference{
					Namespace: authorizationapi.PolicyName,
				},
			},
			oldObject: &authorizationapi.PolicyBinding{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "namespace",
					Name:      "policybinding",
				},
				RoleBindings: map[string]*authorizationapi.RoleBinding{
					"rolebinding": {
						ObjectMeta: kapi.ObjectMeta{
							Namespace: "namespace",
							Name:      "rolebinding",
						},
						Subjects: []kapi.ObjectReference{},
						RoleRef: kapi.ObjectReference{
							Namespace: authorizationapi.PolicyName,
						},
					},
				},
				PolicyRef: kapi.ObjectReference{
					Namespace: authorizationapi.PolicyName,
				},
			},
			kind:        authorizationapi.Kind("PolicyBinding").WithVersion("version"),
			resource:    authorizationapi.Resource("policybindings").WithVersion("version"),
			namespace:   "namespace",
			subresource: "",
			objects: []runtime.Object{
				&kapi.Namespace{
					ObjectMeta: kapi.ObjectMeta{
						Name: "namespace",
					},
				},
				&authorizationapi.Policy{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "namespace",
						Name:      authorizationapi.PolicyName,
					},
					Roles: map[string]*authorizationapi.Role{
						"any": {
							ObjectMeta: kapi.ObjectMeta{
								Namespace: kapi.NamespaceDefault,
								Name:      "any",
							},
						},
					},
				},
				&userAlice,
				&userBob,
				&authorizationapi.RoleBindingRestriction{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "match-users-alice",
						Namespace: "namespace",
					},
					Spec: authorizationapi.RoleBindingRestrictionSpec{
						UserRestriction: &authorizationapi.UserRestriction{
							Selectors: []unversioned.LabelSelector{
								{MatchLabels: map[string]string{"foo": "bar"}},
							},
						},
					},
				},
				&authorizationapi.RoleBindingRestriction{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "match-users-bob",
						Namespace: "namespace",
					},
					Spec: authorizationapi.RoleBindingRestrictionSpec{
						UserRestriction: &authorizationapi.UserRestriction{
							Users: []string{userBob.Name},
						},
					},
				},
			},
		},
		{
			name: "prohibit a user in a policybinding without a matching selector",
			expectedErr: fmt.Sprintf("rolebindings to %s %q are not allowed",
				userBobRef.Kind, userBobRef.Name),
			object: &authorizationapi.PolicyBinding{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "namespace",
					Name:      "policybinding",
				},
				RoleBindings: map[string]*authorizationapi.RoleBinding{
					"rolebinding": {
						ObjectMeta: kapi.ObjectMeta{
							Namespace: "namespace",
							Name:      "rolebinding",
						},
						Subjects: []kapi.ObjectReference{
							userAliceRef,
							userBobRef,
						},
						RoleRef: kapi.ObjectReference{
							Namespace: authorizationapi.PolicyName,
						},
					},
				},
				PolicyRef: kapi.ObjectReference{
					Namespace: authorizationapi.PolicyName,
				},
			},
			oldObject: &authorizationapi.PolicyBinding{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "namespace",
					Name:      "policybinding",
				},
				RoleBindings: map[string]*authorizationapi.RoleBinding{
					"rolebinding": {
						ObjectMeta: kapi.ObjectMeta{
							Namespace: "namespace",
							Name:      "rolebinding",
						},
						Subjects: []kapi.ObjectReference{},
						RoleRef: kapi.ObjectReference{
							Namespace: authorizationapi.PolicyName,
						},
					},
				},
				PolicyRef: kapi.ObjectReference{
					Namespace: authorizationapi.PolicyName,
				},
			},
			kind:        authorizationapi.Kind("PolicyBinding").WithVersion("version"),
			resource:    authorizationapi.Resource("policybindings").WithVersion("version"),
			namespace:   "namespace",
			subresource: "",
			objects: []runtime.Object{
				&kapi.Namespace{
					ObjectMeta: kapi.ObjectMeta{
						Name: "namespace",
					},
				},
				&authorizationapi.Policy{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "namespace",
						Name:      authorizationapi.PolicyName,
					},
					Roles: map[string]*authorizationapi.Role{
						"any": {
							ObjectMeta: kapi.ObjectMeta{
								Namespace: kapi.NamespaceDefault,
								Name:      "any",
							},
						},
					},
				},
				&userAlice,
				&userBob,
				&authorizationapi.RoleBindingRestriction{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "match-users-alice",
						Namespace: "namespace",
					},
					Spec: authorizationapi.RoleBindingRestrictionSpec{
						UserRestriction: &authorizationapi.UserRestriction{
							Selectors: []unversioned.LabelSelector{
								{MatchLabels: map[string]string{"foo": "bar"}},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		kclientset := fake.NewSimpleClientset(tc.objects...)
		oclient := otestclient.NewSimpleFake(tc.objects...)

		plugin, err := NewRestrictUsersAdmission(kclientset)
		if err != nil {
			t.Errorf("unexpected error initializing admission plugin: %v", err)
		}

		plugins := []admission.Interface{plugin}

		plugin.(oadmission.WantsOpenshiftClient).SetOpenshiftClient(oclient)

		groupCache := usercache.NewGroupCache(&groupCache{[]userapi.Group{group}})
		plugin.(oadmission.WantsGroupCache).SetGroupCache(groupCache)
		groupCache.Run()

		err = admission.Validate(plugins)
		if err != nil {
			t.Errorf("unexpected error validating admission plugin: %v", err)
		}

		attributes := admission.NewAttributesRecord(
			tc.object,
			tc.oldObject,
			tc.kind,
			tc.namespace,
			"name",
			tc.resource,
			tc.subresource,
			admission.Create,
			&user.DefaultInfo{},
		)

		err = plugin.Admit(attributes)
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

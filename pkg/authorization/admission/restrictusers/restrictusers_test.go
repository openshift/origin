package restrictusers

import (
	"fmt"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	kapi "k8s.io/kubernetes/pkg/api"
	kadmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"
	//kcache "k8s.io/client-go/tools/cache"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	otestclient "github.com/openshift/origin/pkg/client/testclient"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	//"github.com/openshift/origin/pkg/project/cache"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	usercache "github.com/openshift/origin/pkg/user/cache"
)

func TestAdmission(t *testing.T) {
	var (
		userAlice = userapi.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "Alice",
				Labels: map[string]string{"foo": "bar"},
			},
		}
		userAliceRef = kapi.ObjectReference{
			Kind: authorizationapi.UserKind,
			Name: "Alice",
		}

		userBob = userapi.User{
			ObjectMeta: metav1.ObjectMeta{Name: "Bob"},
			Groups:     []string{"group"},
		}
		userBobRef = kapi.ObjectReference{
			Kind: authorizationapi.UserKind,
			Name: "Bob",
		}

		group = userapi.Group{
			ObjectMeta: metav1.ObjectMeta{
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
			ObjectMeta: metav1.ObjectMeta{
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
		kind        schema.GroupVersionKind
		resource    schema.GroupVersionResource
		namespace   string
		subresource string
		objects     []runtime.Object
	}{
		{
			name: "ignore (allow) if subresource is nonempty",
			object: &authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []kapi.ObjectReference{userAliceRef},
			},
			oldObject: &authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace",
					},
				},
			},
		},
		{
			name: "ignore (allow) cluster-scoped rolebinding",
			object: &authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []kapi.ObjectReference{userAliceRef},
				RoleRef:  kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
			},
			oldObject: &authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace",
					},
				},
			},
		},
		{
			name: "allow if the namespace has no rolebinding restrictions",
			object: &authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
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
				ObjectMeta: metav1.ObjectMeta{
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace",
					},
				},
			},
		},
		{
			name: "allow if any rolebinding with the subject already exists",
			object: &authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
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
				ObjectMeta: metav1.ObjectMeta{
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace",
					},
				},
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
			name: "allow a system user, system group, user, group, or service account in a rolebinding if a literal matches",
			object: &authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
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
				ObjectMeta: metav1.ObjectMeta{
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace",
					},
				},
				&authorizationapi.RoleBindingRestriction{
					ObjectMeta: metav1.ObjectMeta{
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
					ObjectMeta: metav1.ObjectMeta{
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
				userAliceRef.Kind, userAliceRef.Name),
			object: &authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Name:      "rolebinding",
				},
				Subjects: []kapi.ObjectReference{
					userAliceRef,
				},
				RoleRef: kapi.ObjectReference{Namespace: authorizationapi.PolicyName},
			},
			oldObject: &authorizationapi.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace",
					},
				},
				&userAlice,
				&userBob,
				&authorizationapi.RoleBindingRestriction{
					ObjectMeta: metav1.ObjectMeta{
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
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Name:      "policybinding",
				},
				RoleBindings: map[string]*authorizationapi.RoleBinding{
					"rolebinding": {
						ObjectMeta: metav1.ObjectMeta{
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
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Name:      "policybinding",
				},
				RoleBindings: map[string]*authorizationapi.RoleBinding{
					"rolebinding": {
						ObjectMeta: metav1.ObjectMeta{
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace",
					},
				},
				&authorizationapi.Policy{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "namespace",
						Name:      authorizationapi.PolicyName,
					},
					Roles: map[string]*authorizationapi.Role{
						"any": {
							ObjectMeta: metav1.ObjectMeta{
								Namespace: metav1.NamespaceDefault,
								Name:      "any",
							},
						},
					},
				},
				&userAlice,
				&userBob,
				&authorizationapi.RoleBindingRestriction{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "match-users-alice",
						Namespace: "namespace",
					},
					Spec: authorizationapi.RoleBindingRestrictionSpec{
						UserRestriction: &authorizationapi.UserRestriction{
							Selectors: []metav1.LabelSelector{
								{MatchLabels: map[string]string{"foo": "bar"}},
							},
						},
					},
				},
				&authorizationapi.RoleBindingRestriction{
					ObjectMeta: metav1.ObjectMeta{
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
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Name:      "policybinding",
				},
				RoleBindings: map[string]*authorizationapi.RoleBinding{
					"rolebinding": {
						ObjectMeta: metav1.ObjectMeta{
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
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace",
					Name:      "policybinding",
				},
				RoleBindings: map[string]*authorizationapi.RoleBinding{
					"rolebinding": {
						ObjectMeta: metav1.ObjectMeta{
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace",
					},
				},
				&authorizationapi.Policy{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "namespace",
						Name:      authorizationapi.PolicyName,
					},
					Roles: map[string]*authorizationapi.Role{
						"any": {
							ObjectMeta: metav1.ObjectMeta{
								Namespace: metav1.NamespaceDefault,
								Name:      "any",
							},
						},
					},
				},
				&userAlice,
				&userBob,
				&authorizationapi.RoleBindingRestriction{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "match-users-alice",
						Namespace: "namespace",
					},
					Spec: authorizationapi.RoleBindingRestrictionSpec{
						UserRestriction: &authorizationapi.UserRestriction{
							Selectors: []metav1.LabelSelector{
								{MatchLabels: map[string]string{"foo": "bar"}},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		kclientset := fake.NewSimpleClientset(otestclient.UpstreamObjects(tc.objects)...)
		oclient := otestclient.NewSimpleFake(otestclient.OriginObjects(tc.objects)...)

		plugin, err := NewRestrictUsersAdmission()
		if err != nil {
			t.Errorf("unexpected error initializing admission plugin: %v", err)
		}

		plugin.(kadmission.WantsInternalKubeClientSet).SetInternalKubeClientSet(kclientset)
		plugin.(oadmission.WantsOpenshiftClient).SetOpenshiftClient(oclient)

		groupCache := usercache.NewGroupCache(&groupCache{[]userapi.Group{group}})
		plugin.(oadmission.WantsGroupCache).SetGroupCache(groupCache)
		groupCache.Run()

		err = admission.Validate(plugin)
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

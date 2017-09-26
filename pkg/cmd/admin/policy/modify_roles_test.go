package policy

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	fakeauthorizationclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset/fake"
	"github.com/openshift/origin/pkg/oc/admin/policy"
)

func TestModifyNamedClusterRoleBinding(t *testing.T) {
	tests := map[string]struct {
		inputRole                   string
		inputRoleBindingName        string
		inputSubjects               []string
		expectedRoleBindingName     string
		expectedSubjects            []string
		existingClusterRoleBindings *authorizationapi.ClusterRoleBindingList
	}{
		// no name provided - create "edit" for role "edit"
		"create-clusterrolebinding": {
			inputRole: "edit",
			inputSubjects: []string{
				"foo",
			},
			expectedRoleBindingName: "edit",
			expectedSubjects: []string{
				"foo",
			},
			existingClusterRoleBindings: &authorizationapi.ClusterRoleBindingList{
				Items: []authorizationapi.ClusterRoleBinding{},
			},
		},
		// name provided - create "custom" for role "edit"
		"create-named-clusterrolebinding": {
			inputRole:            "edit",
			inputRoleBindingName: "custom",
			inputSubjects: []string{
				"foo",
			},
			expectedRoleBindingName: "custom",
			expectedSubjects: []string{
				"foo",
			},
			existingClusterRoleBindings: &authorizationapi.ClusterRoleBindingList{
				Items: []authorizationapi.ClusterRoleBinding{},
			},
		},
		// name provided - modify "custom"
		"update-named-clusterrolebinding": {
			inputRole:            "edit",
			inputRoleBindingName: "custom",
			inputSubjects: []string{
				"baz",
			},
			expectedRoleBindingName: "custom",
			expectedSubjects: []string{
				"bar",
				"baz",
			},
			existingClusterRoleBindings: &authorizationapi.ClusterRoleBindingList{
				Items: []authorizationapi.ClusterRoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Name: "edit",
					},
					Subjects: []kapi.ObjectReference{{
						Name: "foo",
						Kind: authorizationapi.UserKind,
					}},
					RoleRef: kapi.ObjectReference{
						Name: "edit",
					}}, {
					ObjectMeta: metav1.ObjectMeta{
						Name: "custom",
					},
					Subjects: []kapi.ObjectReference{{
						Name: "bar",
						Kind: authorizationapi.UserKind,
					}},
					RoleRef: kapi.ObjectReference{
						Name: "edit",
					}},
				},
			},
		},
		// no name provided - modify "edit"
		"update-default-clusterrolebinding": {
			inputRole: "edit",
			inputSubjects: []string{
				"baz",
			},
			expectedRoleBindingName: "edit",
			expectedSubjects: []string{
				"foo",
				"baz",
			},
			existingClusterRoleBindings: &authorizationapi.ClusterRoleBindingList{
				Items: []authorizationapi.ClusterRoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Name: "edit",
					},
					Subjects: []kapi.ObjectReference{{
						Name: "foo",
						Kind: authorizationapi.UserKind,
					}},
					RoleRef: kapi.ObjectReference{
						Name: "edit",
					}}, {
					ObjectMeta: metav1.ObjectMeta{
						Name: "custom",
					},
					Subjects: []kapi.ObjectReference{{
						Name: "bar",
						Kind: authorizationapi.UserKind,
					}},
					RoleRef: kapi.ObjectReference{
						Name: "edit",
					}},
				},
			},
		},
	}
	for tcName, tc := range tests {
		// Set up modifier options and run AddRole()
		o := &policy.RoleModificationOptions{
			RoleName:            tc.inputRole,
			RoleBindingName:     tc.inputRoleBindingName,
			Users:               tc.inputSubjects,
			RoleBindingAccessor: policy.NewClusterRoleBindingAccessor(fakeauthorizationclient.NewSimpleClientset(tc.existingClusterRoleBindings).Authorization()),
		}

		addRoleAndCheck(t, o, tcName, tc.expectedRoleBindingName, tc.expectedSubjects)
	}
}

func TestModifyNamedLocalRoleBinding(t *testing.T) {
	tests := map[string]struct {
		inputRole               string
		inputRoleBindingName    string
		inputSubjects           []string
		expectedRoleBindingName string
		expectedSubjects        []string
		existingRoleBindings    *authorizationapi.RoleBindingList
	}{
		// no name provided - create "edit" for role "edit"
		"create-rolebinding": {
			inputRole: "edit",
			inputSubjects: []string{
				"foo",
			},
			expectedRoleBindingName: "edit",
			expectedSubjects: []string{
				"foo",
			},
			existingRoleBindings: &authorizationapi.RoleBindingList{
				Items: []authorizationapi.RoleBinding{},
			},
		},
		// name provided - create "custom" for role "edit"
		"create-named-binding": {
			inputRole:            "edit",
			inputRoleBindingName: "custom",
			inputSubjects: []string{
				"foo",
			},
			expectedRoleBindingName: "custom",
			expectedSubjects: []string{
				"foo",
			},
			existingRoleBindings: &authorizationapi.RoleBindingList{
				Items: []authorizationapi.RoleBinding{},
			},
		},
		// no name provided - modify "edit"
		"update-default-binding": {
			inputRole: "edit",
			inputSubjects: []string{
				"baz",
			},
			expectedRoleBindingName: "edit",
			expectedSubjects: []string{
				"foo",
				"baz",
			},
			existingRoleBindings: &authorizationapi.RoleBindingList{
				Items: []authorizationapi.RoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "edit",
						Namespace: metav1.NamespaceDefault,
					},
					Subjects: []kapi.ObjectReference{{
						Name: "foo",
						Kind: authorizationapi.UserKind,
					}},
					RoleRef: kapi.ObjectReference{
						Name:      "edit",
						Namespace: metav1.NamespaceDefault,
					}}, {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "custom",
						Namespace: metav1.NamespaceDefault,
					},
					Subjects: []kapi.ObjectReference{{
						Name: "bar",
						Kind: authorizationapi.UserKind,
					}},
					RoleRef: kapi.ObjectReference{
						Name:      "edit",
						Namespace: metav1.NamespaceDefault,
					}},
				},
			},
		},
		// name provided - modify "custom"
		"update-named-binding": {
			inputRole:            "edit",
			inputRoleBindingName: "custom",
			inputSubjects: []string{
				"baz",
			},
			expectedRoleBindingName: "custom",
			expectedSubjects: []string{
				"bar",
				"baz",
			},
			existingRoleBindings: &authorizationapi.RoleBindingList{
				Items: []authorizationapi.RoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "edit",
						Namespace: metav1.NamespaceDefault,
					},
					Subjects: []kapi.ObjectReference{{
						Name: "foo",
						Kind: authorizationapi.UserKind,
					}},
					RoleRef: kapi.ObjectReference{
						Name:      "edit",
						Namespace: metav1.NamespaceDefault,
					}}, {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "custom",
						Namespace: metav1.NamespaceDefault,
					},
					Subjects: []kapi.ObjectReference{{
						Name: "bar",
						Kind: authorizationapi.UserKind,
					}},
					RoleRef: kapi.ObjectReference{
						Name:      "edit",
						Namespace: metav1.NamespaceDefault,
					}},
				},
			},
		},
	}
	for tcName, tc := range tests {
		// Set up modifier options and run AddRole()
		o := &policy.RoleModificationOptions{
			RoleName:            tc.inputRole,
			RoleBindingName:     tc.inputRoleBindingName,
			Users:               tc.inputSubjects,
			RoleNamespace:       metav1.NamespaceDefault,
			RoleBindingAccessor: policy.NewLocalRoleBindingAccessor(metav1.NamespaceDefault, fakeauthorizationclient.NewSimpleClientset(tc.existingRoleBindings).Authorization()),
		}

		addRoleAndCheck(t, o, tcName, tc.expectedRoleBindingName, tc.expectedSubjects)
	}
}

func addRoleAndCheck(t *testing.T, o *policy.RoleModificationOptions, tcName, expectedName string, expectedSubjects []string) {
	err := o.AddRole()
	if err != nil {
		t.Errorf("%s: unexpected err %v", tcName, err)
	}

	roleBinding, err := o.RoleBindingAccessor.GetRoleBinding(expectedName)
	if err != nil {
		t.Errorf("%s: err fetching roleBinding %s, %s", tcName, expectedName, err)
	}

	subjects, _ := authorizationapi.StringSubjectsFor(roleBinding.Namespace, roleBinding.Subjects)
	if !reflect.DeepEqual(expectedSubjects, subjects) {
		t.Errorf("%s: err expected users: %v, actual: %v", tcName, expectedSubjects, subjects)
	}
}

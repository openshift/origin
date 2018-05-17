package policy

import (
	"fmt"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/apis/rbac"

	fakeauthorizationclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
)

func TestModifyNamedClusterRoleBinding(t *testing.T) {
	tests := map[string]struct {
		action                      string
		inputRole                   string
		inputRoleBindingName        string
		inputSubjects               []string
		expectedRoleBindingName     string
		expectedSubjects            []rbac.Subject
		existingClusterRoleBindings *rbac.ClusterRoleBindingList
	}{
		// no name provided - create "edit" for role "edit"
		"create-clusterrolebinding": {
			action:    "add",
			inputRole: "edit",
			inputSubjects: []string{
				"foo",
			},
			expectedRoleBindingName: "edit",
			expectedSubjects: []rbac.Subject{{
				APIGroup: rbac.GroupName,
				Name:     "foo",
				Kind:     rbac.UserKind,
			}},
			existingClusterRoleBindings: &rbac.ClusterRoleBindingList{
				Items: []rbac.ClusterRoleBinding{},
			},
		},
		// name provided - create "custom" for role "edit"
		"create-named-clusterrolebinding": {
			action:               "add",
			inputRole:            "edit",
			inputRoleBindingName: "custom",
			inputSubjects: []string{
				"foo",
			},
			expectedRoleBindingName: "custom",
			expectedSubjects: []rbac.Subject{{
				APIGroup: rbac.GroupName,
				Name:     "foo",
				Kind:     rbac.UserKind,
			}},
			existingClusterRoleBindings: &rbac.ClusterRoleBindingList{
				Items: []rbac.ClusterRoleBinding{},
			},
		},
		// name provided - modify "custom"
		"update-named-clusterrolebinding": {
			action:               "add",
			inputRole:            "edit",
			inputRoleBindingName: "custom",
			inputSubjects: []string{
				"baz",
			},
			expectedRoleBindingName: "custom",
			expectedSubjects: []rbac.Subject{{
				APIGroup: rbac.GroupName,
				Name:     "bar",
				Kind:     rbac.UserKind,
			}, {
				APIGroup: rbac.GroupName,
				Name:     "baz",
				Kind:     rbac.UserKind,
			}},
			existingClusterRoleBindings: &rbac.ClusterRoleBindingList{
				Items: []rbac.ClusterRoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Name: "edit",
					},
					Subjects: []rbac.Subject{{
						APIGroup: rbac.GroupName,
						Name:     "foo",
						Kind:     rbac.UserKind,
					}},
					RoleRef: rbac.RoleRef{
						Name: "edit",
						Kind: "ClusterRole",
					}}, {
					ObjectMeta: metav1.ObjectMeta{
						Name: "custom",
					},
					Subjects: []rbac.Subject{{
						APIGroup: rbac.GroupName,
						Name:     "bar",
						Kind:     rbac.UserKind,
					}},
					RoleRef: rbac.RoleRef{
						Name: "edit",
						Kind: "ClusterRole",
					}},
				},
			},
		},
		// name provided - remove from "custom"
		"remove-named-clusterrolebinding": {
			action:               "remove",
			inputRole:            "edit",
			inputRoleBindingName: "custom",
			inputSubjects: []string{
				"baz",
			},
			expectedRoleBindingName: "custom",
			expectedSubjects: []rbac.Subject{{
				APIGroup: rbac.GroupName,
				Name:     "bar",
				Kind:     rbac.UserKind,
			}},
			existingClusterRoleBindings: &rbac.ClusterRoleBindingList{
				Items: []rbac.ClusterRoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Name: "edit",
					},
					Subjects: []rbac.Subject{{
						APIGroup: rbac.GroupName,
						Name:     "foo",
						Kind:     rbac.UserKind,
					}},
					RoleRef: rbac.RoleRef{
						Name: "edit",
						Kind: "ClusterRole",
					}}, {
					ObjectMeta: metav1.ObjectMeta{
						Name: "custom",
					},
					Subjects: []rbac.Subject{{
						APIGroup: rbac.GroupName,
						Name:     "bar",
						Kind:     rbac.UserKind,
					}, {
						APIGroup: rbac.GroupName,
						Name:     "baz",
						Kind:     rbac.UserKind,
					}},
					RoleRef: rbac.RoleRef{
						Name: "edit",
						Kind: "ClusterRole",
					}},
				},
			},
		},
		// no name provided - creates "edit-0"
		"update-default-clusterrolebinding": {
			action:    "add",
			inputRole: "edit",
			inputSubjects: []string{
				"baz",
			},
			expectedRoleBindingName: "edit-0",
			expectedSubjects: []rbac.Subject{{
				APIGroup: rbac.GroupName,
				Name:     "baz",
				Kind:     rbac.UserKind,
			}},
			existingClusterRoleBindings: &rbac.ClusterRoleBindingList{
				Items: []rbac.ClusterRoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Name: "edit",
					},
					Subjects: []rbac.Subject{{
						APIGroup: rbac.GroupName,
						Name:     "foo",
						Kind:     rbac.UserKind,
					}},
					RoleRef: rbac.RoleRef{
						Name: "edit",
						Kind: "ClusterRole",
					}}, {
					ObjectMeta: metav1.ObjectMeta{
						Name: "custom",
					},
					Subjects: []rbac.Subject{{
						APIGroup: rbac.GroupName,
						Name:     "bar",
						Kind:     rbac.UserKind,
					}},
					RoleRef: rbac.RoleRef{
						Name: "edit",
						Kind: "ClusterRole",
					}},
				},
			},
		},
		// no name provided - removes "baz"
		"remove-default-clusterrolebinding": {
			action:    "remove",
			inputRole: "edit",
			inputSubjects: []string{
				"baz",
			},
			expectedRoleBindingName: "edit",
			expectedSubjects: []rbac.Subject{{
				APIGroup: rbac.GroupName,
				Name:     "foo",
				Kind:     rbac.UserKind,
			}},
			existingClusterRoleBindings: &rbac.ClusterRoleBindingList{
				Items: []rbac.ClusterRoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Name: "edit",
					},
					Subjects: []rbac.Subject{{
						APIGroup: rbac.GroupName,
						Name:     "foo",
						Kind:     rbac.UserKind,
					}, {
						APIGroup: rbac.GroupName,
						Name:     "baz",
						Kind:     rbac.UserKind,
					}},
					RoleRef: rbac.RoleRef{
						Name: "edit",
						Kind: "ClusterRole",
					}}, {
					ObjectMeta: metav1.ObjectMeta{
						Name: "custom",
					},
					Subjects: []rbac.Subject{{
						APIGroup: rbac.GroupName,
						Name:     "bar",
						Kind:     rbac.UserKind,
					}},
					RoleRef: rbac.RoleRef{
						Name: "edit",
						Kind: "ClusterRole",
					}},
				},
			},
		},
		// name provided - remove from autoupdate protected
		"remove-from-protected-clusterrolebinding": {
			action:               "remove",
			inputRole:            "edit",
			inputRoleBindingName: "custom",
			inputSubjects: []string{
				"bar",
			},
			expectedRoleBindingName: "custom",
			expectedSubjects:        []rbac.Subject{},
			existingClusterRoleBindings: &rbac.ClusterRoleBindingList{
				Items: []rbac.ClusterRoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{rbac.AutoUpdateAnnotationKey: "false"},
						Name:        "custom",
					},
					Subjects: []rbac.Subject{{
						APIGroup: rbac.GroupName,
						Name:     "bar",
						Kind:     rbac.UserKind,
					}},
					RoleRef: rbac.RoleRef{
						Name: "edit",
						Kind: "ClusterRole",
					}},
				},
			},
		},
	}
	for tcName, tc := range tests {
		// Set up modifier options and run AddRole()
		o := &RoleModificationOptions{
			RoleName:        tc.inputRole,
			RoleKind:        "ClusterRole",
			RoleBindingName: tc.inputRoleBindingName,
			Users:           tc.inputSubjects,
			RbacClient:      fakeauthorizationclient.NewSimpleClientset(tc.existingClusterRoleBindings).Rbac(),
		}

		modifyRoleAndCheck(t, o, tcName, tc.action, tc.expectedRoleBindingName, tc.expectedSubjects)
	}
}

func TestModifyNamedLocalRoleBinding(t *testing.T) {
	tests := map[string]struct {
		action                  string
		inputRole               string
		inputRoleBindingName    string
		inputSubjects           []string
		expectedRoleBindingName string
		expectedSubjects        []rbac.Subject
		existingRoleBindings    *rbac.RoleBindingList
	}{
		// no name provided - create "edit" for role "edit"
		"create-rolebinding": {
			action:    "add",
			inputRole: "edit",
			inputSubjects: []string{
				"foo",
			},
			expectedRoleBindingName: "edit",
			expectedSubjects: []rbac.Subject{{
				APIGroup: rbac.GroupName,
				Name:     "foo",
				Kind:     rbac.UserKind,
			}},
			existingRoleBindings: &rbac.RoleBindingList{
				Items: []rbac.RoleBinding{},
			},
		},
		// name provided - create "custom" for role "edit"
		"create-named-binding": {
			action:               "add",
			inputRole:            "edit",
			inputRoleBindingName: "custom",
			inputSubjects: []string{
				"foo",
			},
			expectedRoleBindingName: "custom",
			expectedSubjects: []rbac.Subject{{
				APIGroup: rbac.GroupName,
				Name:     "foo",
				Kind:     rbac.UserKind,
			}},
			existingRoleBindings: &rbac.RoleBindingList{
				Items: []rbac.RoleBinding{},
			},
		},
		// no name provided - modify "edit"
		"update-default-binding": {
			action:    "add",
			inputRole: "edit",
			inputSubjects: []string{
				"baz",
			},
			expectedRoleBindingName: "edit-0",
			expectedSubjects: []rbac.Subject{{
				APIGroup: rbac.GroupName,
				Name:     "baz",
				Kind:     rbac.UserKind,
			}},
			existingRoleBindings: &rbac.RoleBindingList{
				Items: []rbac.RoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "edit",
						Namespace: metav1.NamespaceDefault,
					},
					Subjects: []rbac.Subject{{
						APIGroup: rbac.GroupName,
						Name:     "foo",
						Kind:     rbac.UserKind,
					}},
					RoleRef: rbac.RoleRef{
						Name: "edit",
						Kind: "Role",
					}}, {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "custom",
						Namespace: metav1.NamespaceDefault,
					},
					Subjects: []rbac.Subject{{
						APIGroup: rbac.GroupName,
						Name:     "bar",
						Kind:     rbac.UserKind,
					}},
					RoleRef: rbac.RoleRef{
						Name: "edit",
						Kind: "Role",
					}},
				},
			},
		},
		// no name provided - remove "bar"
		"remove-default-binding": {
			action:    "remove",
			inputRole: "edit",
			inputSubjects: []string{
				"foo",
			},
			expectedRoleBindingName: "edit",
			expectedSubjects: []rbac.Subject{{
				APIGroup: rbac.GroupName,
				Name:     "baz",
				Kind:     rbac.UserKind,
			}},
			existingRoleBindings: &rbac.RoleBindingList{
				Items: []rbac.RoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "edit",
						Namespace: metav1.NamespaceDefault,
					},
					Subjects: []rbac.Subject{{
						APIGroup: rbac.GroupName,
						Name:     "foo",
						Kind:     rbac.UserKind,
					}, {
						APIGroup: rbac.GroupName,
						Name:     "baz",
						Kind:     rbac.UserKind,
					}},
					RoleRef: rbac.RoleRef{
						Name: "edit",
						Kind: "Role",
					}}, {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "custom",
						Namespace: metav1.NamespaceDefault,
					},
					Subjects: []rbac.Subject{{
						APIGroup: rbac.GroupName,
						Name:     "bar",
						Kind:     rbac.UserKind,
					}},
					RoleRef: rbac.RoleRef{
						Name: "edit",
						Kind: "Role",
					}},
				},
			},
		},
		// name provided - modify "custom"
		"update-named-binding": {
			action:               "add",
			inputRole:            "edit",
			inputRoleBindingName: "custom",
			inputSubjects: []string{
				"baz",
			},
			expectedRoleBindingName: "custom",
			expectedSubjects: []rbac.Subject{{
				APIGroup: rbac.GroupName,
				Name:     "bar",
				Kind:     rbac.UserKind,
			}, {
				APIGroup: rbac.GroupName,
				Name:     "baz",
				Kind:     rbac.UserKind,
			}},
			existingRoleBindings: &rbac.RoleBindingList{
				Items: []rbac.RoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "edit",
						Namespace: metav1.NamespaceDefault,
					},
					Subjects: []rbac.Subject{{
						APIGroup: rbac.GroupName,
						Name:     "foo",
						Kind:     rbac.UserKind,
					}},
					RoleRef: rbac.RoleRef{
						Name: "edit",
						Kind: "Role",
					}}, {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "custom",
						Namespace: metav1.NamespaceDefault,
					},
					Subjects: []rbac.Subject{{
						APIGroup: rbac.GroupName,
						Name:     "bar",
						Kind:     rbac.UserKind,
					}},
					RoleRef: rbac.RoleRef{
						Name: "edit",
						Kind: "Role",
					}},
				},
			},
		},
		// name provided - modify "custom"
		"remove-named-binding": {
			action:               "remove",
			inputRole:            "edit",
			inputRoleBindingName: "custom",
			inputSubjects: []string{
				"baz",
			},
			expectedRoleBindingName: "custom",
			expectedSubjects: []rbac.Subject{{
				APIGroup: rbac.GroupName,
				Name:     "bar",
				Kind:     rbac.UserKind,
			}},
			existingRoleBindings: &rbac.RoleBindingList{
				Items: []rbac.RoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "edit",
						Namespace: metav1.NamespaceDefault,
					},
					Subjects: []rbac.Subject{{
						APIGroup: rbac.GroupName,
						Name:     "foo",
						Kind:     rbac.UserKind,
					}},
					RoleRef: rbac.RoleRef{
						Name: "edit",
						Kind: "Role",
					}}, {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "custom",
						Namespace: metav1.NamespaceDefault,
					},
					Subjects: []rbac.Subject{{
						APIGroup: rbac.GroupName,
						Name:     "bar",
						Kind:     rbac.UserKind,
					}, {
						APIGroup: rbac.GroupName,
						Name:     "baz",
						Kind:     rbac.UserKind,
					}},
					RoleRef: rbac.RoleRef{
						Name: "edit",
						Kind: "Role",
					}},
				},
			},
		},
	}
	for tcName, tc := range tests {
		// Set up modifier options and run AddRole()
		o := &RoleModificationOptions{
			RoleBindingNamespace: metav1.NamespaceDefault,
			RoleBindingName:      tc.inputRoleBindingName,
			RoleKind:             "Role",
			RoleName:             tc.inputRole,
			RbacClient:           fakeauthorizationclient.NewSimpleClientset(tc.existingRoleBindings).Rbac(),
			Users:                tc.inputSubjects,
		}

		modifyRoleAndCheck(t, o, tcName, tc.action, tc.expectedRoleBindingName, tc.expectedSubjects)
	}
}

func modifyRoleAndCheck(t *testing.T, o *RoleModificationOptions, tcName, action string, expectedName string, expectedSubjects []rbac.Subject) {
	var err error
	switch action {
	case "add":
		err = o.AddRole()
	case "remove":
		err = o.RemoveRole()
	default:
		err = fmt.Errorf("Invalid action %s", action)
	}
	if err != nil {
		t.Errorf("%s: unexpected err %v", tcName, err)
	}

	roleBinding, err := getRoleBindingAbstraction(o.RbacClient, expectedName, o.RoleBindingNamespace)
	if err != nil {
		t.Errorf("%s: err fetching roleBinding %s, %s", tcName, expectedName, err)
	}

	if !reflect.DeepEqual(expectedSubjects, roleBinding.Subjects()) {
		t.Errorf("%s: err expected users: %v, actual: %v", tcName, expectedSubjects, roleBinding.Subjects())
	}
}

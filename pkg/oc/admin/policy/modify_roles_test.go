package policy

import (
	"fmt"
	"reflect"
	"testing"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/apis/rbac"
	rbacclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/rbac/internalversion"

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
		expectedRoleBindingList     []string
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
			expectedRoleBindingList: []string{"edit"},
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
			expectedRoleBindingList: []string{"custom"},
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
			expectedRoleBindingList: []string{"custom", "edit"},
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
			expectedRoleBindingList: []string{"custom", "edit"},
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
			expectedRoleBindingList: []string{"custom", "edit", "edit-0"},
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
			expectedRoleBindingList: []string{"custom", "edit"},
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
			expectedRoleBindingList: []string{"custom"},
		},
		// name not provided - do not add duplicate
		"do-not-add-duplicate-clusterrolebinding": {
			action:                  "add",
			inputRole:               "edit",
			inputSubjects:           []string{"foo"},
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
					}},
					RoleRef: rbac.RoleRef{
						Name: "edit",
						Kind: "ClusterRole",
					}},
				},
			},
			expectedRoleBindingList: []string{"edit"},
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

		modifyRoleAndCheck(t, o, tcName, tc.action, tc.expectedRoleBindingName, tc.expectedSubjects, tc.expectedRoleBindingList)
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
		expectedRoleBindingList []string
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
			expectedRoleBindingList: []string{"edit"},
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
			expectedRoleBindingList: []string{"custom"},
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
			expectedRoleBindingList: []string{"custom", "edit", "edit-0"},
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
			expectedRoleBindingList: []string{"custom", "edit"},
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
			expectedRoleBindingList: []string{"custom", "edit"},
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
			expectedRoleBindingList: []string{"custom", "edit"},
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

		modifyRoleAndCheck(t, o, tcName, tc.action, tc.expectedRoleBindingName, tc.expectedSubjects, tc.expectedRoleBindingList)
	}
}

func getRoleBindingAbstractionsList(rbacClient rbacclient.RbacInterface, namespace string) ([]*roleBindingAbstraction, error) {
	ret := make([]*roleBindingAbstraction, 0)
	// see if we can find an existing binding that points to the role in question.
	if len(namespace) > 0 {
		roleBindings, err := rbacClient.RoleBindings(namespace).List(metav1.ListOptions{})
		if err != nil && !kapierrors.IsNotFound(err) {
			return nil, err
		}
		for i := range roleBindings.Items {
			// shallow copy outside of the loop so that we can take its address
			roleBinding := roleBindings.Items[i]
			ret = append(ret, &roleBindingAbstraction{rbacClient: rbacClient, roleBinding: &roleBinding})
		}
	} else {
		clusterRoleBindings, err := rbacClient.ClusterRoleBindings().List(metav1.ListOptions{})
		if err != nil && !kapierrors.IsNotFound(err) {
			return nil, err
		}
		for i := range clusterRoleBindings.Items {
			// shallow copy outside of the loop so that we can take its address
			clusterRoleBinding := clusterRoleBindings.Items[i]
			ret = append(ret, &roleBindingAbstraction{rbacClient: rbacClient, clusterRoleBinding: &clusterRoleBinding})
		}
	}

	return ret, nil
}
func modifyRoleAndCheck(t *testing.T, o *RoleModificationOptions, tcName, action string, expectedName string, expectedSubjects []rbac.Subject, expectedBindings []string) {
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

	roleBindings, err := getRoleBindingAbstractionsList(o.RbacClient, o.RoleBindingNamespace)
	foundBindings := make([]string, len(expectedBindings))
	for _, roleBinding := range roleBindings {
		var foundBinding string
		for i := range expectedBindings {
			if expectedBindings[i] == roleBinding.Name() {
				foundBindings[i] = roleBinding.Name()
				foundBinding = roleBinding.Name()
				break
			}
		}
		if len(foundBinding) == 0 {
			t.Errorf("%s: found unexpected binding %q", tcName, roleBinding.Name())
		}
	}
	if !reflect.DeepEqual(expectedBindings, foundBindings) {
		t.Errorf("%s: err expected bindings: %v, actual: %v", tcName, expectedBindings, foundBindings)
	}
}

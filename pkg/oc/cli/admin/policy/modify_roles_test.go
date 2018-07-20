package policy

import (
	"fmt"
	"reflect"
	"testing"

	userv1 "github.com/openshift/api/user/v1"
	fakeuserclient "github.com/openshift/client-go/user/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	diffutil "k8s.io/apimachinery/pkg/util/diff"
	fakeclient "k8s.io/client-go/kubernetes/fake"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
)

func TestModifyNamedClusterRoleBinding(t *testing.T) {
	tests := map[string]struct {
		action                      string
		inputRole                   string
		inputRoleBindingName        string
		inputSubjects               []string
		expectedRoleBindingName     string
		expectedSubjects            []rbacv1.Subject
		existingClusterRoleBindings *rbacv1.ClusterRoleBindingList
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
			expectedSubjects: []rbacv1.Subject{{
				APIGroup: rbacv1.GroupName,
				Name:     "foo",
				Kind:     rbacv1.UserKind,
			}},
			existingClusterRoleBindings: &rbacv1.ClusterRoleBindingList{
				Items: []rbacv1.ClusterRoleBinding{},
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
			expectedSubjects: []rbacv1.Subject{{
				APIGroup: rbacv1.GroupName,
				Name:     "foo",
				Kind:     rbacv1.UserKind,
			}},
			existingClusterRoleBindings: &rbacv1.ClusterRoleBindingList{
				Items: []rbacv1.ClusterRoleBinding{},
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
			expectedSubjects: []rbacv1.Subject{{
				APIGroup: rbacv1.GroupName,
				Name:     "bar",
				Kind:     rbacv1.UserKind,
			}, {
				APIGroup: rbacv1.GroupName,
				Name:     "baz",
				Kind:     rbacv1.UserKind,
			}},
			existingClusterRoleBindings: &rbacv1.ClusterRoleBindingList{
				Items: []rbacv1.ClusterRoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Name: "edit",
					},
					Subjects: []rbacv1.Subject{{
						APIGroup: rbacv1.GroupName,
						Name:     "foo",
						Kind:     rbacv1.UserKind,
					}},
					RoleRef: rbacv1.RoleRef{
						Name: "edit",
						Kind: "ClusterRole",
					}}, {
					ObjectMeta: metav1.ObjectMeta{
						Name: "custom",
					},
					Subjects: []rbacv1.Subject{{
						APIGroup: rbacv1.GroupName,
						Name:     "bar",
						Kind:     rbacv1.UserKind,
					}},
					RoleRef: rbacv1.RoleRef{
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
			expectedSubjects: []rbacv1.Subject{{
				APIGroup: rbacv1.GroupName,
				Name:     "bar",
				Kind:     rbacv1.UserKind,
			}},
			existingClusterRoleBindings: &rbacv1.ClusterRoleBindingList{
				Items: []rbacv1.ClusterRoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Name: "edit",
					},
					Subjects: []rbacv1.Subject{{
						APIGroup: rbacv1.GroupName,
						Name:     "foo",
						Kind:     rbacv1.UserKind,
					}},
					RoleRef: rbacv1.RoleRef{
						Name: "edit",
						Kind: "ClusterRole",
					}}, {
					ObjectMeta: metav1.ObjectMeta{
						Name: "custom",
					},
					Subjects: []rbacv1.Subject{{
						APIGroup: rbacv1.GroupName,
						Name:     "bar",
						Kind:     rbacv1.UserKind,
					}, {
						APIGroup: rbacv1.GroupName,
						Name:     "baz",
						Kind:     rbacv1.UserKind,
					}},
					RoleRef: rbacv1.RoleRef{
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
			expectedSubjects: []rbacv1.Subject{{
				APIGroup: rbacv1.GroupName,
				Name:     "baz",
				Kind:     rbacv1.UserKind,
			}},
			existingClusterRoleBindings: &rbacv1.ClusterRoleBindingList{
				Items: []rbacv1.ClusterRoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Name: "edit",
					},
					Subjects: []rbacv1.Subject{{
						APIGroup: rbacv1.GroupName,
						Name:     "foo",
						Kind:     rbacv1.UserKind,
					}},
					RoleRef: rbacv1.RoleRef{
						Name: "edit",
						Kind: "ClusterRole",
					}}, {
					ObjectMeta: metav1.ObjectMeta{
						Name: "custom",
					},
					Subjects: []rbacv1.Subject{{
						APIGroup: rbacv1.GroupName,
						Name:     "bar",
						Kind:     rbacv1.UserKind,
					}},
					RoleRef: rbacv1.RoleRef{
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
			expectedSubjects: []rbacv1.Subject{{
				APIGroup: rbacv1.GroupName,
				Name:     "foo",
				Kind:     rbacv1.UserKind,
			}},
			existingClusterRoleBindings: &rbacv1.ClusterRoleBindingList{
				Items: []rbacv1.ClusterRoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Name: "edit",
					},
					Subjects: []rbacv1.Subject{{
						APIGroup: rbacv1.GroupName,
						Name:     "foo",
						Kind:     rbacv1.UserKind,
					}, {
						APIGroup: rbacv1.GroupName,
						Name:     "baz",
						Kind:     rbacv1.UserKind,
					}},
					RoleRef: rbacv1.RoleRef{
						Name: "edit",
						Kind: "ClusterRole",
					}}, {
					ObjectMeta: metav1.ObjectMeta{
						Name: "custom",
					},
					Subjects: []rbacv1.Subject{{
						APIGroup: rbacv1.GroupName,
						Name:     "bar",
						Kind:     rbacv1.UserKind,
					}},
					RoleRef: rbacv1.RoleRef{
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
			expectedSubjects:        []rbacv1.Subject{},
			existingClusterRoleBindings: &rbacv1.ClusterRoleBindingList{
				Items: []rbacv1.ClusterRoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{rbacv1.AutoUpdateAnnotationKey: "false"},
						Name:        "custom",
					},
					Subjects: []rbacv1.Subject{{
						APIGroup: rbacv1.GroupName,
						Name:     "bar",
						Kind:     rbacv1.UserKind,
					}},
					RoleRef: rbacv1.RoleRef{
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
			expectedSubjects: []rbacv1.Subject{{
				APIGroup: rbacv1.GroupName,
				Name:     "foo",
				Kind:     rbacv1.UserKind,
			}},
			existingClusterRoleBindings: &rbacv1.ClusterRoleBindingList{
				Items: []rbacv1.ClusterRoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Name: "edit",
					},
					Subjects: []rbacv1.Subject{{
						APIGroup: rbacv1.GroupName,
						Name:     "foo",
						Kind:     rbacv1.UserKind,
					}},
					RoleRef: rbacv1.RoleRef{
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
			RbacClient:      fakeclient.NewSimpleClientset(tc.existingClusterRoleBindings).Rbac(),
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
		expectedSubjects        []rbacv1.Subject
		existingRoleBindings    *rbacv1.RoleBindingList
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
			expectedSubjects: []rbacv1.Subject{{
				APIGroup: rbacv1.GroupName,
				Name:     "foo",
				Kind:     rbacv1.UserKind,
			}},
			existingRoleBindings: &rbacv1.RoleBindingList{
				Items: []rbacv1.RoleBinding{},
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
			expectedSubjects: []rbacv1.Subject{{
				APIGroup: rbacv1.GroupName,
				Name:     "foo",
				Kind:     rbacv1.UserKind,
			}},
			existingRoleBindings: &rbacv1.RoleBindingList{
				Items: []rbacv1.RoleBinding{},
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
			expectedSubjects: []rbacv1.Subject{{
				APIGroup: rbacv1.GroupName,
				Name:     "baz",
				Kind:     rbacv1.UserKind,
			}},
			existingRoleBindings: &rbacv1.RoleBindingList{
				Items: []rbacv1.RoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "edit",
						Namespace: metav1.NamespaceDefault,
					},
					Subjects: []rbacv1.Subject{{
						APIGroup: rbacv1.GroupName,
						Name:     "foo",
						Kind:     rbacv1.UserKind,
					}},
					RoleRef: rbacv1.RoleRef{
						Name: "edit",
						Kind: "Role",
					}}, {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "custom",
						Namespace: metav1.NamespaceDefault,
					},
					Subjects: []rbacv1.Subject{{
						APIGroup: rbacv1.GroupName,
						Name:     "bar",
						Kind:     rbacv1.UserKind,
					}},
					RoleRef: rbacv1.RoleRef{
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
			expectedSubjects: []rbacv1.Subject{{
				APIGroup: rbacv1.GroupName,
				Name:     "baz",
				Kind:     rbacv1.UserKind,
			}},
			existingRoleBindings: &rbacv1.RoleBindingList{
				Items: []rbacv1.RoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "edit",
						Namespace: metav1.NamespaceDefault,
					},
					Subjects: []rbacv1.Subject{{
						APIGroup: rbacv1.GroupName,
						Name:     "foo",
						Kind:     rbacv1.UserKind,
					}, {
						APIGroup: rbacv1.GroupName,
						Name:     "baz",
						Kind:     rbacv1.UserKind,
					}},
					RoleRef: rbacv1.RoleRef{
						Name: "edit",
						Kind: "Role",
					}}, {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "custom",
						Namespace: metav1.NamespaceDefault,
					},
					Subjects: []rbacv1.Subject{{
						APIGroup: rbacv1.GroupName,
						Name:     "bar",
						Kind:     rbacv1.UserKind,
					}},
					RoleRef: rbacv1.RoleRef{
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
			expectedSubjects: []rbacv1.Subject{{
				APIGroup: rbacv1.GroupName,
				Name:     "bar",
				Kind:     rbacv1.UserKind,
			}, {
				APIGroup: rbacv1.GroupName,
				Name:     "baz",
				Kind:     rbacv1.UserKind,
			}},
			existingRoleBindings: &rbacv1.RoleBindingList{
				Items: []rbacv1.RoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "edit",
						Namespace: metav1.NamespaceDefault,
					},
					Subjects: []rbacv1.Subject{{
						APIGroup: rbacv1.GroupName,
						Name:     "foo",
						Kind:     rbacv1.UserKind,
					}},
					RoleRef: rbacv1.RoleRef{
						Name: "edit",
						Kind: "Role",
					}}, {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "custom",
						Namespace: metav1.NamespaceDefault,
					},
					Subjects: []rbacv1.Subject{{
						APIGroup: rbacv1.GroupName,
						Name:     "bar",
						Kind:     rbacv1.UserKind,
					}},
					RoleRef: rbacv1.RoleRef{
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
			expectedSubjects: []rbacv1.Subject{{
				APIGroup: rbacv1.GroupName,
				Name:     "bar",
				Kind:     rbacv1.UserKind,
			}},
			existingRoleBindings: &rbacv1.RoleBindingList{
				Items: []rbacv1.RoleBinding{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "edit",
						Namespace: metav1.NamespaceDefault,
					},
					Subjects: []rbacv1.Subject{{
						APIGroup: rbacv1.GroupName,
						Name:     "foo",
						Kind:     rbacv1.UserKind,
					}},
					RoleRef: rbacv1.RoleRef{
						Name: "edit",
						Kind: "Role",
					}}, {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "custom",
						Namespace: metav1.NamespaceDefault,
					},
					Subjects: []rbacv1.Subject{{
						APIGroup: rbacv1.GroupName,
						Name:     "bar",
						Kind:     rbacv1.UserKind,
					}, {
						APIGroup: rbacv1.GroupName,
						Name:     "baz",
						Kind:     rbacv1.UserKind,
					}},
					RoleRef: rbacv1.RoleRef{
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
			RbacClient:           fakeclient.NewSimpleClientset(tc.existingRoleBindings).Rbac(),
			Users:                tc.inputSubjects,
		}

		modifyRoleAndCheck(t, o, tcName, tc.action, tc.expectedRoleBindingName, tc.expectedSubjects, tc.expectedRoleBindingList)
	}
}
func TestModifyRoleBindingWarnings(t *testing.T) {
	type clusterState struct {
		roles               *rbacv1.RoleList
		clusterRoles        *rbacv1.ClusterRoleList
		roleBindings        *rbacv1.RoleBindingList
		clusterRoleBindings *rbacv1.ClusterRoleBindingList

		users           *userv1.UserList
		groups          *userv1.GroupList
		serviceAccounts *corev1.ServiceAccountList
	}
	type cmdInputs struct {
		roleName        string
		roleKind        string
		roleBindingName string
		roleNamespace   string
		userNames       []string
		groupNames      []string
		serviceAccounts []rbacv1.Subject
	}
	type cmdOutputs struct {
		warnings []string
	}
	const (
		currentNamespace = "ns-0"

		existingRoleName                  = "existing-role-0"
		existingRoleBindingName           = "existing-rolebinding-0"
		existingNamespacedRoleBindingName = "existing-namespaced-rolebinding-0"
		existingClusterRoleBindingName    = "existing-clusterrolebinding-0"
		existingClusterRoleName           = "existing-clusterrole-0"
		existingUserName                  = "existing-user-0"
		existingGroupName                 = "existing-group-0"
		existingServiceAccountName        = "existing-serviceaccount-0"

		boundRoleName           = "bound-role-0"
		boundUserName           = "bound-user-0"
		boundGroupName          = "bound-group-0"
		boundServiceAccountName = "bound-serviceaccount-0"

		newRoleName               = "tbd-role-0"
		newClusterRoleName        = "tbd-clusterrole-0"
		newRoleBindingName        = "tbd-rolebinding-0"
		newClusterRoleBindingName = "tbd-clusterrolebinding-0"
		newUserName               = "tbd-user-0"
		newGroupName              = "tbd-group-0"
		newServiceAccountName     = "tbd-serviceaccount-0"

		roleNotFoundWarning           = "Warning: role 'tbd-role-0' not found\n"
		clusterRoleNotFoundWarning    = "Warning: role 'tbd-clusterrole-0' not found\n"
		userNotFoundWarning           = "Warning: User 'tbd-user-0' not found\n"
		groupNotFoundWarning          = "Warning: Group 'tbd-group-0' not found\n"
		serviceAccountNotFoundWarning = "Warning: ServiceAccount 'tbd-serviceaccount-0' not found\n"
	)
	var (
		boundSubjects = []rbacv1.Subject{
			{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: boundUserName},
			{APIGroup: rbacv1.GroupName, Kind: rbacv1.GroupKind, Name: boundGroupName},
			{APIGroup: rbacv1.GroupName, Kind: rbacv1.ServiceAccountKind, Name: boundServiceAccountName},
		}
		existingUser = userv1.User{
			ObjectMeta: metav1.ObjectMeta{Name: existingUserName},
		}
		existingGroup = userv1.Group{
			ObjectMeta: metav1.ObjectMeta{Name: existingGroupName},
		}
		existingServiceAccount = corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: existingServiceAccountName, Namespace: currentNamespace},
		}
		existingRole = rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: existingRoleName, Namespace: currentNamespace},
		}
		boundRole = rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: boundRoleName, Namespace: currentNamespace},
		}
		existingClusterRole = rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: existingClusterRoleName},
		}
		existingRoleBinding = rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: existingRoleBindingName, Namespace: currentNamespace},
			Subjects:   boundSubjects,
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Name: existingClusterRoleName, Kind: "ClusterRole"},
		}
		existingNamespacedRoleBinding = rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: existingNamespacedRoleBindingName, Namespace: currentNamespace},
			Subjects:   boundSubjects,
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Name: boundRoleName, Kind: "Role"},
		}
		existingClusterRoleBinding = rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: existingClusterRoleBindingName},
			Subjects:   boundSubjects,
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Name: existingClusterRoleName, Kind: "ClusterRole"},
		}
		defaultInitialState = clusterState{
			roles: &rbacv1.RoleList{
				Items: []rbacv1.Role{existingRole},
			},
			clusterRoles: &rbacv1.ClusterRoleList{
				Items: []rbacv1.ClusterRole{existingClusterRole},
			},
			roleBindings: &rbacv1.RoleBindingList{
				Items: []rbacv1.RoleBinding{existingRoleBinding, existingNamespacedRoleBinding},
			},
			clusterRoleBindings: &rbacv1.ClusterRoleBindingList{
				Items: []rbacv1.ClusterRoleBinding{existingClusterRoleBinding},
			},
			users: &userv1.UserList{
				Items: []userv1.User{existingUser},
			},
			groups: &userv1.GroupList{
				Items: []userv1.Group{existingGroup},
			},
			serviceAccounts: &corev1.ServiceAccountList{
				Items: []corev1.ServiceAccount{existingServiceAccount},
			},
		}
	)
	tests := []struct {
		name    string
		subtest string

		initialState clusterState
		inputs       cmdInputs

		expectedOutputs cmdOutputs
		expectedState   clusterState
	}{
		{
			name:         "add-role-to-user",
			subtest:      "no-warnings-needed",
			initialState: defaultInitialState,
			inputs: cmdInputs{
				roleKind:        "Role",
				roleName:        existingRoleName,
				roleBindingName: newRoleBindingName,
				roleNamespace:   currentNamespace,
				userNames:       []string{existingUserName},
				serviceAccounts: []rbacv1.Subject{},
				groupNames:      []string{},
			},
			expectedOutputs: cmdOutputs{
				warnings: []string{},
			},
			expectedState: clusterState{
				roleBindings: &rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{
						existingRoleBinding,
						existingNamespacedRoleBinding,
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: newRoleBindingName, Namespace: currentNamespace},
							Subjects: []rbacv1.Subject{
								{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: existingUserName}},
							RoleRef: rbacv1.RoleRef{
								APIGroup: rbacv1.GroupName, Kind: "Role", Name: existingRoleName},
						},
					},
				},
				clusterRoleBindings: &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{existingClusterRoleBinding},
				},
			},
		},
		{
			name:         "add-role-to-user",
			subtest:      "role-not-found-warning",
			initialState: defaultInitialState,
			inputs: cmdInputs{
				roleKind:        "Role",
				roleName:        newRoleName,
				roleBindingName: newRoleBindingName,
				roleNamespace:   currentNamespace,
				userNames:       []string{existingUserName},
				serviceAccounts: []rbacv1.Subject{},
				groupNames:      []string{},
			},
			expectedOutputs: cmdOutputs{
				warnings: []string{roleNotFoundWarning},
			},
			expectedState: clusterState{
				roleBindings: &rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{
						existingRoleBinding,
						existingNamespacedRoleBinding,
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: newRoleBindingName, Namespace: currentNamespace},
							Subjects: []rbacv1.Subject{
								{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: existingUserName}},
							RoleRef: rbacv1.RoleRef{
								APIGroup: rbacv1.GroupName, Kind: "Role", Name: newRoleName},
						},
					},
				},
				clusterRoleBindings: &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{existingClusterRoleBinding},
				},
			},
		},
		{
			name:    "add-role-to-user",
			subtest: "user-not-found-warning",
			initialState: clusterState{
				roles: &rbacv1.RoleList{
					Items: []rbacv1.Role{existingRole, boundRole},
				},
				clusterRoles: &rbacv1.ClusterRoleList{
					Items: []rbacv1.ClusterRole{existingClusterRole},
				},
				roleBindings: &rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{existingRoleBinding, existingNamespacedRoleBinding},
				},
				clusterRoleBindings: &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{existingClusterRoleBinding},
				},
				users: &userv1.UserList{
					Items: []userv1.User{existingUser},
				},
				groups: &userv1.GroupList{
					Items: []userv1.Group{existingGroup},
				},
				serviceAccounts: &corev1.ServiceAccountList{
					Items: []corev1.ServiceAccount{existingServiceAccount},
				},
			},
			inputs: cmdInputs{
				roleKind:        "Role",
				roleName:        boundRoleName,
				roleBindingName: existingNamespacedRoleBindingName,
				roleNamespace:   currentNamespace,
				userNames:       []string{boundUserName, newUserName},
				serviceAccounts: []rbacv1.Subject{},
				groupNames:      []string{},
			},
			expectedOutputs: cmdOutputs{
				warnings: []string{userNotFoundWarning},
			},
			expectedState: clusterState{
				roleBindings: &rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{
						existingRoleBinding,
						{
							ObjectMeta: existingNamespacedRoleBinding.ObjectMeta,
							Subjects: append(existingNamespacedRoleBinding.Subjects,
								rbacv1.Subject{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: newUserName}),
							RoleRef: existingNamespacedRoleBinding.RoleRef,
						},
					},
				},
				clusterRoleBindings: &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{existingClusterRoleBinding},
				},
			},
		},
		{
			name:    "add-role-to-user",
			subtest: "serviceaccount-not-found-warning",
			initialState: clusterState{
				roles: &rbacv1.RoleList{
					Items: []rbacv1.Role{existingRole, boundRole},
				},
				clusterRoles: &rbacv1.ClusterRoleList{
					Items: []rbacv1.ClusterRole{existingClusterRole},
				},
				roleBindings: &rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{existingRoleBinding, existingNamespacedRoleBinding},
				},
				clusterRoleBindings: &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{existingClusterRoleBinding},
				},
				users: &userv1.UserList{
					Items: []userv1.User{existingUser},
				},
				groups: &userv1.GroupList{
					Items: []userv1.Group{existingGroup},
				},
				serviceAccounts: &corev1.ServiceAccountList{
					Items: []corev1.ServiceAccount{existingServiceAccount},
				},
			},
			inputs: cmdInputs{
				roleKind:        "Role",
				roleName:        boundRoleName,
				roleBindingName: existingNamespacedRoleBindingName,
				roleNamespace:   currentNamespace,
				userNames:       []string{boundUserName},
				serviceAccounts: []rbacv1.Subject{
					{APIGroup: rbacv1.GroupName, Kind: rbacv1.ServiceAccountKind, Name: newServiceAccountName},
				},
				groupNames: []string{},
			},
			expectedOutputs: cmdOutputs{
				warnings: []string{serviceAccountNotFoundWarning},
			},
			expectedState: clusterState{
				roleBindings: &rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{
						existingRoleBinding,
						{
							ObjectMeta: existingNamespacedRoleBinding.ObjectMeta,
							Subjects: append(existingNamespacedRoleBinding.Subjects,
								rbacv1.Subject{APIGroup: rbacv1.GroupName, Kind: rbacv1.ServiceAccountKind, Name: newServiceAccountName}),
							RoleRef: existingNamespacedRoleBinding.RoleRef,
						},
					},
				},
				clusterRoleBindings: &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{existingClusterRoleBinding},
				},
			},
		},
		{
			name:         "add-role-to-group",
			subtest:      "no-warning-needed",
			initialState: defaultInitialState,
			inputs: cmdInputs{
				roleKind:        "Role",
				roleName:        existingRoleName,
				roleBindingName: newRoleBindingName,
				roleNamespace:   currentNamespace,
				userNames:       []string{},
				serviceAccounts: []rbacv1.Subject{},
				groupNames:      []string{existingGroupName},
			},
			expectedOutputs: cmdOutputs{
				warnings: []string{},
			},
			expectedState: clusterState{
				roleBindings: &rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{
						existingRoleBinding,
						existingNamespacedRoleBinding,
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: newRoleBindingName, Namespace: currentNamespace},
							Subjects: []rbacv1.Subject{
								{APIGroup: rbacv1.GroupName, Kind: rbacv1.GroupKind, Name: existingGroupName}},
							RoleRef: rbacv1.RoleRef{
								APIGroup: rbacv1.GroupName, Kind: "Role", Name: existingRoleName},
						},
					},
				},
				clusterRoleBindings: &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{existingClusterRoleBinding},
				},
			},
		},
		{
			name:    "add-role-to-group",
			subtest: "group-not-found-warning",
			initialState: clusterState{
				roles: &rbacv1.RoleList{
					Items: []rbacv1.Role{existingRole, boundRole},
				},
				clusterRoles: &rbacv1.ClusterRoleList{
					Items: []rbacv1.ClusterRole{existingClusterRole},
				},
				roleBindings: &rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{existingRoleBinding, existingNamespacedRoleBinding},
				},
				clusterRoleBindings: &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{existingClusterRoleBinding},
				},
				users: &userv1.UserList{
					Items: []userv1.User{existingUser},
				},
				groups: &userv1.GroupList{
					Items: []userv1.Group{existingGroup},
				},
				serviceAccounts: &corev1.ServiceAccountList{
					Items: []corev1.ServiceAccount{existingServiceAccount},
				},
			},
			inputs: cmdInputs{
				roleKind:        "Role",
				roleName:        boundRoleName,
				roleBindingName: existingNamespacedRoleBindingName,
				roleNamespace:   currentNamespace,
				userNames:       []string{boundUserName},
				serviceAccounts: []rbacv1.Subject{},
				groupNames:      []string{boundGroupName, newGroupName},
			},
			expectedOutputs: cmdOutputs{
				warnings: []string{groupNotFoundWarning},
			},
			expectedState: clusterState{
				roleBindings: &rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{
						existingRoleBinding,
						{
							ObjectMeta: existingNamespacedRoleBinding.ObjectMeta,
							Subjects: append(existingNamespacedRoleBinding.Subjects,
								rbacv1.Subject{APIGroup: rbacv1.GroupName, Kind: rbacv1.GroupKind, Name: newGroupName}),
							RoleRef: existingNamespacedRoleBinding.RoleRef,
						},
					},
				},
				clusterRoleBindings: &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{existingClusterRoleBinding},
				},
			},
		},
		{
			name:         "add-cluster-role-to-user",
			subtest:      "no-warnings-needed",
			initialState: defaultInitialState,
			inputs: cmdInputs{
				roleKind:        "ClusterRole",
				roleName:        existingClusterRoleName,
				roleBindingName: newClusterRoleBindingName,
				roleNamespace:   "",
				userNames:       []string{existingUserName},
				serviceAccounts: []rbacv1.Subject{},
				groupNames:      []string{},
			},
			expectedOutputs: cmdOutputs{
				warnings: []string{},
			},
			expectedState: clusterState{
				roleBindings: &rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{existingRoleBinding, existingNamespacedRoleBinding},
				},
				clusterRoleBindings: &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{
						existingClusterRoleBinding,
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: newClusterRoleBindingName},
							Subjects: []rbacv1.Subject{
								{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: existingUserName}},
							RoleRef: rbacv1.RoleRef{
								APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: existingClusterRoleName},
						},
					},
				},
			},
		},
		{
			name:         "add-cluster-role-to-user",
			subtest:      "role-not-found-warning",
			initialState: defaultInitialState,
			inputs: cmdInputs{
				roleKind:        "ClusterRole",
				roleName:        newClusterRoleName,
				roleBindingName: newClusterRoleBindingName,
				userNames:       []string{existingUserName},
				serviceAccounts: []rbacv1.Subject{},
				groupNames:      []string{},
			},
			expectedOutputs: cmdOutputs{
				warnings: []string{clusterRoleNotFoundWarning},
			},
			expectedState: clusterState{
				roleBindings: &rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{existingRoleBinding, existingNamespacedRoleBinding},
				},
				clusterRoleBindings: &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{
						existingClusterRoleBinding,
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: newClusterRoleBindingName},
							Subjects: []rbacv1.Subject{
								{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: existingUserName}},
							RoleRef: rbacv1.RoleRef{
								APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: newClusterRoleName},
						},
					},
				},
			},
		},
		{
			name:         "add-cluster-role-to-user",
			subtest:      "user-not-found-warning",
			initialState: defaultInitialState,
			inputs: cmdInputs{
				roleKind:        "ClusterRole",
				roleName:        existingClusterRoleName,
				roleBindingName: existingClusterRoleBindingName,
				userNames:       []string{boundUserName, newUserName},
				serviceAccounts: []rbacv1.Subject{},
				groupNames:      []string{},
			},
			expectedOutputs: cmdOutputs{
				warnings: []string{userNotFoundWarning},
			},
			expectedState: clusterState{
				roleBindings: &rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{existingRoleBinding, existingNamespacedRoleBinding},
				},
				clusterRoleBindings: &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{
						existingClusterRoleBinding,
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: existingClusterRoleBindingName},
							Subjects: append(existingClusterRoleBinding.Subjects,
								rbacv1.Subject{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: newUserName}),
							RoleRef: rbacv1.RoleRef{
								APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: existingClusterRoleName},
						},
					},
				},
			},
		},
		{
			name:         "add-cluster-role-to-user",
			subtest:      "serviceaccount-not-found-warning",
			initialState: defaultInitialState,
			inputs: cmdInputs{
				roleKind:        "ClusterRole",
				roleName:        existingClusterRoleName,
				roleBindingName: existingClusterRoleBindingName,
				userNames:       []string{},
				serviceAccounts: []rbacv1.Subject{
					{APIGroup: rbacv1.GroupName, Kind: rbacv1.ServiceAccountKind, Name: newServiceAccountName},
					{APIGroup: rbacv1.GroupName, Kind: rbacv1.ServiceAccountKind, Name: boundServiceAccountName},
				},
				groupNames: []string{},
			},
			expectedOutputs: cmdOutputs{
				warnings: []string{serviceAccountNotFoundWarning},
			},
			expectedState: clusterState{
				roleBindings: &rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{existingRoleBinding, existingNamespacedRoleBinding},
				},
				clusterRoleBindings: &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: existingClusterRoleBindingName},
							Subjects: append(existingClusterRoleBinding.Subjects,
								rbacv1.Subject{APIGroup: rbacv1.GroupName, Kind: rbacv1.ServiceAccountKind, Name: newServiceAccountName}),
							RoleRef: rbacv1.RoleRef{
								APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: existingClusterRoleName},
						},
					},
				},
			},
		},
		{
			name:         "add-cluster-role-to-group",
			subtest:      "no-warning-needed",
			initialState: defaultInitialState,
			inputs: cmdInputs{
				roleKind:        "ClusterRole",
				roleName:        existingClusterRoleName,
				roleBindingName: newClusterRoleBindingName,
				userNames:       []string{},
				serviceAccounts: []rbacv1.Subject{},
				groupNames:      []string{existingGroupName},
			},
			expectedOutputs: cmdOutputs{
				warnings: []string{},
			},
			expectedState: clusterState{
				roleBindings: &rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{existingRoleBinding, existingNamespacedRoleBinding},
				},
				clusterRoleBindings: &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{
						existingClusterRoleBinding,
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: newClusterRoleBindingName},
							Subjects: []rbacv1.Subject{
								{APIGroup: rbacv1.GroupName, Kind: rbacv1.GroupKind, Name: existingGroupName}},
							RoleRef: rbacv1.RoleRef{
								APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: existingClusterRoleName},
						},
					},
				},
			},
		},
		{
			name:         "add-cluster-role-to-group",
			subtest:      "group-not-found-warning",
			initialState: defaultInitialState,
			inputs: cmdInputs{
				roleKind:        "ClusterRole",
				roleName:        existingClusterRoleName,
				roleBindingName: existingClusterRoleBindingName,
				userNames:       []string{boundUserName},
				serviceAccounts: []rbacv1.Subject{},
				groupNames:      []string{boundGroupName, newGroupName},
			},
			expectedOutputs: cmdOutputs{
				warnings: []string{groupNotFoundWarning},
			},
			expectedState: clusterState{
				roleBindings: &rbacv1.RoleBindingList{
					Items: []rbacv1.RoleBinding{existingRoleBinding, existingNamespacedRoleBinding},
				},
				clusterRoleBindings: &rbacv1.ClusterRoleBindingList{
					Items: []rbacv1.ClusterRoleBinding{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: existingClusterRoleBindingName},
							Subjects: append(existingClusterRoleBinding.Subjects,
								rbacv1.Subject{APIGroup: rbacv1.GroupName, Kind: rbacv1.GroupKind, Name: newGroupName}),
							RoleRef: rbacv1.RoleRef{
								APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: existingClusterRoleName},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		// Set up modifier options and run AddRole()
		t.Run(tt.name+":"+tt.subtest, func(t *testing.T) {
			expectedWarnings := map[string]string{}
			for _, warning := range tt.expectedOutputs.warnings {
				expectedWarnings[warning] = warning
			}
			o := &RoleModificationOptions{
				RoleBindingNamespace: tt.inputs.roleNamespace,
				RoleBindingName:      tt.inputs.roleBindingName,
				RoleKind:             tt.inputs.roleKind,
				RoleName:             tt.inputs.roleName,
				RbacClient:           fakeclient.NewSimpleClientset(tt.initialState.roles, tt.initialState.clusterRoles, tt.initialState.roleBindings, tt.initialState.clusterRoleBindings).Rbac(),
				Users:                tt.inputs.userNames,
				Groups:               tt.inputs.groupNames,
				Subjects:             tt.inputs.serviceAccounts,
				UserClient:           fakeuserclient.NewSimpleClientset(tt.initialState.users, tt.initialState.groups).User(),
				ServiceAccountClient: fakeclient.NewSimpleClientset(tt.initialState.serviceAccounts).Core(),
				PrintErrf: func(format string, args ...interface{}) {
					actualWarning := fmt.Sprintf(format, args...)
					if _, ok := expectedWarnings[actualWarning]; !ok {
						t.Errorf("unexpected warning: '%s'", actualWarning)
					}
					delete(expectedWarnings, actualWarning)
				},
			}
			err := o.AddRole()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			rbs, err := o.RbacClient.RoleBindings(tt.inputs.roleNamespace).List(metav1.ListOptions{})
			if err != nil {
				t.Errorf("unexpected error fetching rolebindings: %v", err)
			}
			expectedRoleBindings := map[string]rbacv1.RoleBinding{}
			if tt.expectedState.roleBindings != nil {
				for _, expected := range tt.expectedState.roleBindings.Items {
					expectedRoleBindings[expected.ObjectMeta.Name] = expected
				}
			}
			for _, found := range rbs.Items {
				expected, ok := expectedRoleBindings[found.ObjectMeta.Name]
				if !ok {
					t.Errorf("unexpected rolebinding: %v", found.ObjectMeta.Name)
				}
				compareResources(t, expected, found)
				delete(expectedRoleBindings, found.ObjectMeta.Name)
			}
			for missing := range expectedRoleBindings {
				t.Errorf("missing rolebinding: %s", missing)
			}

			crbs, err := o.RbacClient.ClusterRoleBindings().List(metav1.ListOptions{})
			if err != nil {
				t.Errorf("unexpected error fetching clusterrolebindings: %v", err)
			}
			expectedClusterRoleBindings := map[string]rbacv1.ClusterRoleBinding{}
			if tt.expectedState.clusterRoleBindings != nil {
				for _, expected := range tt.expectedState.clusterRoleBindings.Items {
					expectedClusterRoleBindings[expected.ObjectMeta.Name] = expected
				}
			}
			for _, found := range crbs.Items {
				expected, ok := expectedClusterRoleBindings[found.ObjectMeta.Name]
				if !ok {
					t.Errorf("unexpected clusterrolebinding: %v", found.ObjectMeta.Name)
				}
				compareResources(t, expected, found)
				delete(expectedClusterRoleBindings, found.ObjectMeta.Name)
			}
			for missing := range expectedClusterRoleBindings {
				t.Errorf("missing clusterrolebinding: %s", missing)
			}
			for warning := range expectedWarnings {
				t.Errorf("missing warning: '%s'", warning)
			}
		})
	}
}

// compareResources compares resource equality then prints a diff for easier debugging
func compareResources(t *testing.T, expected, actual interface{}) {
	if eq := equality.Semantic.DeepEqual(expected, actual); !eq {
		t.Errorf("Resource does not match expected value: %s",
			diffutil.ObjectDiff(expected, actual))
	}
}
func getRoleBindingAbstractionsList(rbacClient rbacv1client.RbacV1Interface, namespace string) ([]*roleBindingAbstraction, error) {
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
func modifyRoleAndCheck(t *testing.T, o *RoleModificationOptions, tcName, action string, expectedName string, expectedSubjects []rbacv1.Subject,
	expectedBindings []string) {
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

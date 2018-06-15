package authprune

import (
	"reflect"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
)

func TestRoleReaper(t *testing.T) {
	tests := []struct {
		name                string
		role                *rbacv1.Role
		bindings            []*rbacv1.RoleBinding
		deletedBindingNames []string
	}{
		{
			name: "no bindings",
			role: &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "role",
				},
			},
		},
		{
			name: "bindings",
			role: &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "role",
					Namespace: "one",
				},
			},
			bindings: []*rbacv1.RoleBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "binding-1",
						Namespace: "one",
					},
					RoleRef: rbacv1.RoleRef{Name: "role", Kind: "Role"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "binding-2",
						Namespace: "one",
					},
					RoleRef: rbacv1.RoleRef{Name: "role2", Kind: "Role"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "binding-3",
						Namespace: "one",
					},
					RoleRef: rbacv1.RoleRef{Name: "role", Kind: "Role"},
				},
			},
			deletedBindingNames: []string{"binding-1", "binding-3"},
		},
		{
			name: "bindings in to cluster scoped ignored",
			role: &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "role",
					Namespace: "one",
				},
			},
			bindings: []*rbacv1.RoleBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "binding-1",
						Namespace: "one",
					},
					RoleRef: rbacv1.RoleRef{Name: "role", Kind: "ClusterRole"},
				},
			},
		},
	}

	for _, test := range tests {
		startingObjects := []runtime.Object{}
		startingObjects = append(startingObjects, test.role)
		for _, binding := range test.bindings {
			startingObjects = append(startingObjects, binding)
		}
		tc := fake.NewSimpleClientset(startingObjects...)

		actualDeletedBindingNames := []string{}
		tc.PrependReactor("delete", "rolebindings", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			actualDeletedBindingNames = append(actualDeletedBindingNames, action.(clientgotesting.DeleteAction).GetName())
			return true, nil, nil
		})

		reaper := NewRoleReaper(tc.RbacV1(), tc.RbacV1())
		err := reaper.Stop(test.role.Namespace, test.role.Name, 0, nil)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
		}

		expected := sets.NewString(test.deletedBindingNames...)
		actuals := sets.NewString(actualDeletedBindingNames...)
		if !reflect.DeepEqual(expected.List(), actuals.List()) {
			t.Errorf("%s: expected %v, got %v", test.name, expected.List(), actuals.List())
		}
	}
}

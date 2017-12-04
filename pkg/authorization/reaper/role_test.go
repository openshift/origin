package reaper

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationfake "github.com/openshift/origin/pkg/authorization/generated/internalclientset/fake"
)

func TestRoleReaper(t *testing.T) {
	tests := []struct {
		name                string
		role                *authorizationapi.Role
		bindings            []*authorizationapi.RoleBinding
		deletedBindingNames []string
	}{
		{
			name: "no bindings",
			role: &authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "role",
				},
			},
		},
		{
			name: "bindings",
			role: &authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "role",
					Namespace: "one",
				},
			},
			bindings: []*authorizationapi.RoleBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "binding-1",
						Namespace: "one",
					},
					RoleRef: kapi.ObjectReference{Name: "role", Namespace: "one"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "binding-2",
						Namespace: "one",
					},
					RoleRef: kapi.ObjectReference{Name: "role2", Namespace: "one"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "binding-3",
						Namespace: "one",
					},
					RoleRef: kapi.ObjectReference{Name: "role", Namespace: "one"},
				},
			},
			deletedBindingNames: []string{"binding-1", "binding-3"},
		},
		{
			name: "bindings in other namespace ignored",
			role: &authorizationapi.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "role",
					Namespace: "one",
				},
			},
			bindings: []*authorizationapi.RoleBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "binding-1",
						Namespace: "one",
					},
					RoleRef: kapi.ObjectReference{Name: "role"},
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
		tc := authorizationfake.NewSimpleClientset(startingObjects...)

		actualDeletedBindingNames := []string{}
		tc.PrependReactor("delete", "rolebindings", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			actualDeletedBindingNames = append(actualDeletedBindingNames, action.(clientgotesting.DeleteAction).GetName())
			return true, nil, nil
		})

		reaper := NewRoleReaper(tc.Authorization(), tc.Authorization())
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

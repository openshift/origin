package reaper

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client/testclient"
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
				ObjectMeta: kapi.ObjectMeta{
					Name: "role",
				},
			},
		},
		{
			name: "bindings",
			role: &authorizationapi.Role{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "role",
					Namespace: "one",
				},
			},
			bindings: []*authorizationapi.RoleBinding{
				{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "binding-1",
						Namespace: "one",
					},
					RoleRef: kapi.ObjectReference{Name: "role", Namespace: "one"},
				},
				{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "binding-2",
						Namespace: "one",
					},
					RoleRef: kapi.ObjectReference{Name: "role2", Namespace: "one"},
				},
				{
					ObjectMeta: kapi.ObjectMeta{
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
				ObjectMeta: kapi.ObjectMeta{
					Name:      "role",
					Namespace: "one",
				},
			},
			bindings: []*authorizationapi.RoleBinding{
				{
					ObjectMeta: kapi.ObjectMeta{
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
		tc := testclient.NewSimpleFake(startingObjects...)

		actualDeletedBindingNames := []string{}
		tc.PrependReactor("delete", "rolebindings", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
			actualDeletedBindingNames = append(actualDeletedBindingNames, action.(ktestclient.DeleteAction).GetName())
			return true, nil, nil
		})

		reaper := NewRoleReaper(tc, tc)
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

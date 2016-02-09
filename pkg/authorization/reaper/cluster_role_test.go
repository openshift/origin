package reaper

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	_ "github.com/openshift/origin/pkg/authorization/api/install"
	"github.com/openshift/origin/pkg/client/testclient"
)

func TestClusterRoleReaper(t *testing.T) {
	tests := []struct {
		name                string
		role                *authorizationapi.ClusterRole
		bindings            []*authorizationapi.ClusterRoleBinding
		deletedBindingNames []string
	}{
		{
			name: "no bindings",
			role: &authorizationapi.ClusterRole{
				ObjectMeta: kapi.ObjectMeta{
					Name: "role",
				},
			},
		},
		{
			name: "bindings",
			role: &authorizationapi.ClusterRole{
				ObjectMeta: kapi.ObjectMeta{
					Name: "role",
				},
			},
			bindings: []*authorizationapi.ClusterRoleBinding{
				{
					ObjectMeta: kapi.ObjectMeta{
						Name: "binding-1",
					},
					RoleRef: kapi.ObjectReference{Name: "role"},
				},
				{
					ObjectMeta: kapi.ObjectMeta{
						Name: "binding-2",
					},
					RoleRef: kapi.ObjectReference{Name: "role2"},
				},
				{
					ObjectMeta: kapi.ObjectMeta{
						Name: "binding-3",
					},
					RoleRef: kapi.ObjectReference{Name: "role"},
				},
			},
			deletedBindingNames: []string{"binding-1", "binding-3"},
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
		tc.PrependReactor("delete", "clusterrolebindings", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
			actualDeletedBindingNames = append(actualDeletedBindingNames, action.(ktestclient.DeleteAction).GetName())
			return true, nil, nil
		})

		reaper := NewClusterRoleReaper(tc, tc, tc)
		err := reaper.Stop("", test.role.Name, 0, nil)
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

func TestClusterRoleReaperAgainstNamespacedBindings(t *testing.T) {
	tests := []struct {
		name                string
		role                *authorizationapi.ClusterRole
		bindings            []*authorizationapi.RoleBinding
		deletedBindingNames []string
	}{
		{
			name: "bindings",
			role: &authorizationapi.ClusterRole{
				ObjectMeta: kapi.ObjectMeta{
					Name: "role",
				},
			},
			bindings: []*authorizationapi.RoleBinding{
				{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "binding-1",
						Namespace: "ns-one",
					},
					RoleRef: kapi.ObjectReference{Name: "role"},
				},
				{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "binding-2",
						Namespace: "ns-one",
					},
					RoleRef: kapi.ObjectReference{Name: "role2"},
				},
				{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "binding-3",
						Namespace: "ns-one",
					},
					RoleRef: kapi.ObjectReference{Name: "role"},
				},
			},
			deletedBindingNames: []string{"binding-1", "binding-3"},
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

		reaper := NewClusterRoleReaper(tc, tc, tc)
		err := reaper.Stop("", test.role.Name, 0, nil)
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

package policy

import (
	"fmt"
	"reflect"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericclioptions/printers"
	clientgotesting "k8s.io/client-go/testing"

	fakekubeclient "k8s.io/client-go/kubernetes/fake"
	fakerbacv1 "k8s.io/client-go/kubernetes/typed/rbac/v1/fake"
	fakerbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1/fake"
)

func TestModifySCC(t *testing.T) {
	tests := map[string]struct {
		startingCRB *rbacv1.ClusterRoleBinding
		subjects    []rbacv1.Subject
		expectedCRB *rbacv1.ClusterRoleBinding
		remove      bool
	}{
		"add-user-to-empty": {
			startingCRB: &rbacv1.ClusterRoleBinding{},
			subjects:    []rbacv1.Subject{{Name: "one", Kind: "User"}, {Name: "two", Kind: "User"}},
			expectedCRB: &rbacv1.ClusterRoleBinding{
				Subjects: []rbacv1.Subject{{Kind: "User", Name: "one"}, {Kind: "User", Name: "two"}},
			},
			remove: false,
		},
		"add-user-to-existing": {
			startingCRB: &rbacv1.ClusterRoleBinding{Subjects: []rbacv1.Subject{{Kind: "User", Name: "one"}}},
			subjects:    []rbacv1.Subject{{Name: "two", Kind: "User"}},
			expectedCRB: &rbacv1.ClusterRoleBinding{
				Subjects: []rbacv1.Subject{{Kind: "User", Name: "one"}, {Kind: "User", Name: "two"}},
			},
			remove: false,
		},
		"add-user-to-existing-with-overlap": {
			startingCRB: &rbacv1.ClusterRoleBinding{Subjects: []rbacv1.Subject{{Kind: "User", Name: "one"}}},
			subjects:    []rbacv1.Subject{{Name: "one", Kind: "User"}, {Name: "two", Kind: "User"}},
			expectedCRB: &rbacv1.ClusterRoleBinding{
				Subjects: []rbacv1.Subject{{Kind: "User", Name: "one"}, {Kind: "User", Name: "two"}},
			},
			remove: false,
		},
		"add-sa-to-empty": {
			startingCRB: &rbacv1.ClusterRoleBinding{},
			subjects:    []rbacv1.Subject{{Namespace: "a", Name: "one", Kind: "ServiceAccount"}, {Namespace: "b", Name: "two", Kind: "ServiceAccount"}},
			expectedCRB: &rbacv1.ClusterRoleBinding{
				Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "a", Name: "one"}, {Kind: "ServiceAccount", Namespace: "b", Name: "two"}},
			},
			remove: false,
		},
		"add-sa-to-existing": {
			startingCRB: &rbacv1.ClusterRoleBinding{Subjects: []rbacv1.Subject{{Kind: "User", Name: "one"}}},
			subjects:    []rbacv1.Subject{{Namespace: "b", Name: "two", Kind: "ServiceAccount"}},
			expectedCRB: &rbacv1.ClusterRoleBinding{
				Subjects: []rbacv1.Subject{{Kind: "User", Name: "one"}, {Kind: "ServiceAccount", Namespace: "b", Name: "two"}},
			},
			remove: false,
		},
		"add-sa-to-existing-with-overlap": {
			startingCRB: &rbacv1.ClusterRoleBinding{Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "a", Name: "one"}}},
			subjects:    []rbacv1.Subject{{Namespace: "a", Name: "one", Kind: "ServiceAccount"}, {Namespace: "b", Name: "two", Kind: "ServiceAccount"}},
			expectedCRB: &rbacv1.ClusterRoleBinding{
				Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "a", Name: "one"}, {Kind: "ServiceAccount", Namespace: "b", Name: "two"}},
			},
			remove: false,
		},

		"add-group-to-empty": {
			startingCRB: &rbacv1.ClusterRoleBinding{},
			subjects:    []rbacv1.Subject{{Name: "one", Kind: "Group"}, {Name: "two", Kind: "Group"}},
			expectedCRB: &rbacv1.ClusterRoleBinding{Subjects: []rbacv1.Subject{{Kind: "Group", Name: "one"}, {Kind: "Group", Name: "two"}}},
			remove:      false,
		},
		"add-group-to-existing": {
			startingCRB: &rbacv1.ClusterRoleBinding{Subjects: []rbacv1.Subject{{Kind: "Group", Name: "one"}}},
			subjects:    []rbacv1.Subject{{Name: "two", Kind: "Group"}},
			expectedCRB: &rbacv1.ClusterRoleBinding{Subjects: []rbacv1.Subject{{Kind: "Group", Name: "one"}, {Kind: "Group", Name: "two"}}},
			remove:      false,
		},
		"add-group-to-existing-with-overlap": {
			startingCRB: &rbacv1.ClusterRoleBinding{Subjects: []rbacv1.Subject{{Kind: "Group", Name: "one"}}},
			subjects:    []rbacv1.Subject{{Name: "one", Kind: "Group"}, {Name: "two", Kind: "Group"}},
			expectedCRB: &rbacv1.ClusterRoleBinding{Subjects: []rbacv1.Subject{{Kind: "Group", Name: "one"}, {Kind: "Group", Name: "two"}}},
			remove:      false,
		},

		"remove-user": {
			startingCRB: &rbacv1.ClusterRoleBinding{
				Subjects: []rbacv1.Subject{{Kind: "User", Name: "one"}, {Kind: "User", Name: "two"}},
			},
			subjects:    []rbacv1.Subject{{Name: "one", Kind: "User"}, {Name: "two", Kind: "User"}},
			expectedCRB: &rbacv1.ClusterRoleBinding{},
			remove:      true,
		},
		"remove-user-from-existing-with-overlap": {
			startingCRB: &rbacv1.ClusterRoleBinding{
				Subjects: []rbacv1.Subject{{Kind: "User", Name: "one"}, {Kind: "User", Name: "two"}},
			},
			subjects:    []rbacv1.Subject{{Name: "two", Kind: "User"}},
			expectedCRB: &rbacv1.ClusterRoleBinding{Subjects: []rbacv1.Subject{{Kind: "User", Name: "one"}}},
			remove:      true,
		},

		"remove-sa": {
			startingCRB: &rbacv1.ClusterRoleBinding{Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "a", Name: "one"}, {Kind: "ServiceAccount", Namespace: "b", Name: "two"}}},
			subjects:    []rbacv1.Subject{{Namespace: "a", Name: "one", Kind: "ServiceAccount"}, {Namespace: "b", Name: "two", Kind: "ServiceAccount"}},
			expectedCRB: &rbacv1.ClusterRoleBinding{},
			remove:      true,
		},
		"remove-sa-from-existing-with-overlap": {
			startingCRB: &rbacv1.ClusterRoleBinding{Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "a", Name: "one"}, {Kind: "ServiceAccount", Namespace: "b", Name: "two"}}},
			subjects:    []rbacv1.Subject{{Namespace: "b", Name: "two", Kind: "ServiceAccount"}},
			expectedCRB: &rbacv1.ClusterRoleBinding{Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Namespace: "a", Name: "one"}}},
			remove:      true,
		},

		"remove-group": {
			startingCRB: &rbacv1.ClusterRoleBinding{Subjects: []rbacv1.Subject{{Kind: "Group", Name: "one"}, {Kind: "Group", Name: "two"}}},
			subjects:    []rbacv1.Subject{{Name: "one", Kind: "Group"}, {Name: "two", Kind: "Group"}},
			expectedCRB: &rbacv1.ClusterRoleBinding{},
			remove:      true,
		},
		"remove-group-from-existing-with-overlap": {
			startingCRB: &rbacv1.ClusterRoleBinding{Subjects: []rbacv1.Subject{{Kind: "Group", Name: "one"}, {Kind: "Group", Name: "two"}}},
			subjects:    []rbacv1.Subject{{Name: "two", Kind: "Group"}},
			expectedCRB: &rbacv1.ClusterRoleBinding{Subjects: []rbacv1.Subject{{Kind: "Group", Name: "one"}}},
			remove:      true,
		},
	}

	for tcName, tc := range tests {
		fakeRbacClient := fakerbacv1.FakeRbacV1{Fake: &(fakekubeclient.NewSimpleClientset().Fake)}
		fakeClient := fakerbacv1client.FakeClusterRoleBindings{Fake: &fakeRbacClient}

		sccName := "foo"
		roleRef := rbacv1.RoleRef{Kind: "ClusterRole", Name: fmt.Sprintf(RBACNamesFmt, sccName)}
		tc.expectedCRB.RoleRef = roleRef
		tc.startingCRB.RoleRef = roleRef

		fakeClient.Fake.PrependReactor("get", "clusterrolebindings", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, tc.startingCRB, nil
		})
		var actualCRB *rbacv1.ClusterRoleBinding
		fakeClient.Fake.PrependReactor("update", "clusterrolebindings", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			actualCRB = action.(clientgotesting.UpdateAction).GetObject().(*rbacv1.ClusterRoleBinding)
			return true, actualCRB, nil
		})
		fakeClient.Fake.PrependReactor("delete", "clusterrolebindings", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			actualCRB = &rbacv1.ClusterRoleBinding{Subjects: []rbacv1.Subject{}}
			return true, actualCRB, nil
		})

		o := &SCCModificationOptions{
			PrintFlags: genericclioptions.NewPrintFlags(""),
			ToPrinter:  func(string) (printers.ResourcePrinter, error) { return printers.NewDiscardingPrinter(), nil },

			SCCName:                 sccName,
			RbacClient:              &fakeRbacClient,
			DefaultSubjectNamespace: "",
			Subjects:                tc.subjects,

			IOStreams: genericclioptions.NewTestIOStreamsDiscard(),
		}

		var err error
		if tc.remove {
			err = o.RemoveSCC()
		} else {
			err = o.AddSCC()
		}
		if err != nil {
			t.Errorf("%s: unexpected err %v", tcName, err)
		}

		shouldUpdate := !reflect.DeepEqual(tc.expectedCRB.Subjects, tc.startingCRB.Subjects)
		if shouldUpdate && actualCRB == nil {
			t.Errorf("'%s': clusterrolebinding should have been updated", tcName)
			continue
		}
		if e, a := tc.expectedCRB.Subjects, actualCRB.Subjects; !reflect.DeepEqual(e, a) {
			if len(e) == 0 && len(a) == 0 {
				continue
			}
			t.Errorf("%s: expected %v, actual %v", tcName, e, a)
		}
	}
}

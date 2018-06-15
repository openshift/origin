package aggregated_logging

import (
	"errors"
	"testing"

	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/rbac"

	authapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/log"
)

type fakeRoleBindingDiagnostic struct {
	fakeDiagnostic
	fakeClusterRoleBindingList authapi.ClusterRoleBindingList
}

func newFakeRoleBindingDiagnostic(t *testing.T) *fakeRoleBindingDiagnostic {
	return &fakeRoleBindingDiagnostic{
		fakeDiagnostic:             *newFakeDiagnostic(t),
		fakeClusterRoleBindingList: authapi.ClusterRoleBindingList{},
	}
}

func (f *fakeRoleBindingDiagnostic) listClusterRoleBindings() (*authapi.ClusterRoleBindingList, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &f.fakeClusterRoleBindingList, nil
}
func (f *fakeRoleBindingDiagnostic) addBinding(name string, namespace string) {
	ref := kapi.ObjectReference{
		Name:      name,
		Kind:      rbac.ServiceAccountKind,
		Namespace: namespace,
	}
	if len(f.fakeClusterRoleBindingList.Items) == 0 {
		f.fakeClusterRoleBindingList.Items = append(f.fakeClusterRoleBindingList.Items, authapi.ClusterRoleBinding{
			RoleRef: kapi.ObjectReference{
				Name: clusterReaderRoleBindingRoleName,
			},
		})
	}
	f.fakeClusterRoleBindingList.Items[0].Subjects = append(f.fakeClusterRoleBindingList.Items[0].Subjects, ref)
}

// Test error when client error
func TestCheckClusterRoleBindingsWhenErrorFromClientRetrievingRoles(t *testing.T) {
	d := newFakeRoleBindingDiagnostic(t)
	d.err = errors.New("client error")

	checkClusterRoleBindings(d, d, fakeProject)

	d.assertMessage("AGL0605", "Exp. an error message if client error retrieving ClusterRoleBindings", log.ErrorLevel)
	d.dumpMessages()
}

func TestCheckClusterRoleBindingsWhenClusterReaderIsNotInProject(t *testing.T) {
	d := newFakeRoleBindingDiagnostic(t)
	d.addBinding("someName", "someRandomProject")
	d.addBinding(fluentdServiceAccountName, fakeProject)

	checkClusterRoleBindings(d, d, fakeProject)

	d.assertNoErrors()
	d.dumpMessages()
}

func TestCheckClusterRoleBindingsWhenUnboundServiceAccounts(t *testing.T) {
	d := newFakeRoleBindingDiagnostic(t)
	d.addBinding(fluentdServiceAccountName, "someRandomProject")

	checkClusterRoleBindings(d, d, fakeProject)

	d.assertMessage("AGL0610", "Exp. an error when the exp service-accounts dont have cluster-reader access", log.ErrorLevel)
	d.dumpMessages()
}

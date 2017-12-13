package aggregated_logging

import (
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/log"
)

type mockServiceAccountDiagnostic struct {
	accounts kapi.ServiceAccountList
	fakeDiagnostic
}

func newMockServiceAccountDiagnostic(t *testing.T) *mockServiceAccountDiagnostic {
	return &mockServiceAccountDiagnostic{
		accounts:       kapi.ServiceAccountList{},
		fakeDiagnostic: *newFakeDiagnostic(t),
	}
}

func (m *mockServiceAccountDiagnostic) serviceAccounts(project string, options metav1.ListOptions) (*kapi.ServiceAccountList, error) {
	if m.err != nil {
		return &m.accounts, m.err
	}
	return &m.accounts, nil
}

func (d *mockServiceAccountDiagnostic) addServiceAccountNamed(name string) {
	meta := metav1.ObjectMeta{Name: name}
	d.accounts.Items = append(d.accounts.Items, kapi.ServiceAccount{ObjectMeta: meta})
}

func TestCheckingServiceAccountsWhenFailedResponseFromClient(t *testing.T) {
	d := newMockServiceAccountDiagnostic(t)
	d.err = errors.New("Some Error")
	checkServiceAccounts(d, d, fakeProject)
	d.assertMessage("AGL0505",
		"Exp an error when unable to retrieve service accounts because of a client error",
		log.ErrorLevel)
}

func TestCheckingServiceAccountsWhenMissingExpectedServiceAccount(t *testing.T) {
	d := newMockServiceAccountDiagnostic(t)
	d.addServiceAccountNamed("foobar")

	checkServiceAccounts(d, d, fakeProject)
	d.assertMessage("AGL0515",
		"Exp an error when an expected service account was not found.",
		log.ErrorLevel)
}

func TestCheckingServiceAccountsIsOk(t *testing.T) {
	d := newMockServiceAccountDiagnostic(t)

	for _, name := range serviceAccountNames.List() {
		d.addServiceAccountNamed(name)
	}

	checkServiceAccounts(d, d, fakeProject)
	d.assertNoErrors()
}

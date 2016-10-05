package aggregated_logging

import (
	"errors"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/diagnostics/log"
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

func (m *mockServiceAccountDiagnostic) serviceAccounts(project string, options kapi.ListOptions) (*kapi.ServiceAccountList, error) {
	if m.err != nil {
		return &m.accounts, m.err
	}
	return &m.accounts, nil
}

func (d *mockServiceAccountDiagnostic) addServiceAccountNamed(name string) {
	meta := kapi.ObjectMeta{Name: name}
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

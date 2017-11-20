package aggregated_logging

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/log"

	securityapi "github.com/openshift/origin/pkg/security/apis/security"
)

type fakeSccDiagnostic struct {
	fakeDiagnostic
	fakeScc securityapi.SecurityContextConstraints
}

func newFakeSccDiagnostic(t *testing.T) *fakeSccDiagnostic {
	return &fakeSccDiagnostic{
		fakeDiagnostic: *newFakeDiagnostic(t),
	}
}

func (f *fakeSccDiagnostic) getScc(name string) (*securityapi.SecurityContextConstraints, error) {
	json, _ := json.Marshal(f.fakeScc)
	f.test.Logf(">> test#getScc(%s), err: %s, scc:%s", name, f.err, string(json))
	if f.err != nil {
		return nil, f.err
	}
	return &f.fakeScc, nil
}

func (f *fakeSccDiagnostic) addSccFor(name string, project string) {
	f.fakeScc.Users = append(f.fakeScc.Users, fmt.Sprintf("system:serviceaccount:%s:%s", project, name))
}

func TestCheckSccWhenClientReturnsError(t *testing.T) {
	d := newFakeSccDiagnostic(t)
	d.err = errors.New("client error")

	checkSccs(d, d, fakeProject)

	d.assertMessage("AGL0705", "Exp error when client returns error getting SCC", log.ErrorLevel)
	d.dumpMessages()
}

func TestCheckSccWhenMissingPrivelegedUsers(t *testing.T) {
	d := newFakeSccDiagnostic(t)

	checkSccs(d, d, fakeProject)

	d.assertMessage("AGL0710", "Exp error when SCC is missing exp service acount", log.ErrorLevel)
	d.dumpMessages()
}

func TestCheckSccWhenEverytingExists(t *testing.T) {
	d := newFakeSccDiagnostic(t)
	d.addSccFor(fluentdServiceAccountName, fakeProject)

	checkSccs(d, d, fakeProject)

	d.assertNoErrors()
	d.dumpMessages()
}

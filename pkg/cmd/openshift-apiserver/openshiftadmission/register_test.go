package openshiftadmission

import (
	"testing"

	"github.com/openshift/origin/pkg/admission/admissionregistrationtesting"
)

func TestAdmissionRegistration(t *testing.T) {
	err := admissionregistrationtesting.AdmissionRegistrationTest(OriginAdmissionPlugins, OpenShiftAdmissionPlugins, DefaultOffPlugins)
	if err != nil {
		t.Fatal(err)
	}
}

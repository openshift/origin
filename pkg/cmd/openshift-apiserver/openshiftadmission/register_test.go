package openshiftadmission

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/pkg/admission/admissionregistrationtesting"
)

func TestAdmissionRegistration(t *testing.T) {
	err := admissionregistrationtesting.AdmissionRegistrationTest(OriginAdmissionPlugins, OpenShiftAdmissionPlugins, sets.String{})
	if err != nil {
		t.Fatal(err)
	}
}

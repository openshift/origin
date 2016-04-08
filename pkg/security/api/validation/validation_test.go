package validation

import (
	"testing"

	//kapi "k8s.io/kubernetes/pkg/api"

	securityapi "github.com/openshift/origin/pkg/security/api"
)

func TestValidatePodSpecReview(t *testing.T) {
	okCases := map[string]securityapi.PodSpecReview{
		"good case": securityapi.PodSpecReview{},
	}
	for k, v := range okCases {
		errs := ValidatePodSpecReview(&v)
		if len(errs) > 0 {
			t.Errorf("%s unexpected error %v", k, errs)
		}
	}

}

func TestValidatePodSpecSelfSubjectReview(t *testing.T) {
	okCases := map[string]securityapi.PodSpecSelfSubjectReview{
		"good case": securityapi.PodSpecSelfSubjectReview{},
	}
	for k, v := range okCases {
		errs := ValidatePodSpecSelfSubjectReview(&v)
		if len(errs) > 0 {
			t.Errorf("%s unexpected error %v", k, errs)
		}
	}
}

func TestValidatePodSpecSubjectReview(t *testing.T) {
	okCases := map[string]securityapi.PodSpecSubjectReview{
		"good case": securityapi.PodSpecSubjectReview{},
	}
	for k, v := range okCases {
		errs := ValidatePodSpecSubjectReview(&v)
		if len(errs) > 0 {
			t.Errorf("%s unexpected error %v", k, errs)
		}
	}
}

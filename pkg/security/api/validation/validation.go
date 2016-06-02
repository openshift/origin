package validation

import (
	"k8s.io/kubernetes/pkg/util/validation/field"

	securityapi "github.com/openshift/origin/pkg/security/api"
)

func ValidatePodSpecReview(podSpecReview *securityapi.PodSpecReview) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}

func ValidatePodSpecSelfSubjectReview(podSpecReview *securityapi.PodSpecSelfSubjectReview) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}

func ValidatePodSpecSubjectReview(podSpecReview *securityapi.PodSpecSubjectReview) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}

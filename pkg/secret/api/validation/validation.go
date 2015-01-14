package validation

import (
	errs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	secretapi "github.com/openshift/origin/pkg/secret/api"
)

// ValidateSecret tests required fields for a Secret.
func ValidateSecret(secret *secretapi.Secret) errs.ValidationErrorList {
	allErrs := errs.ValidationErrorList{}
	if len(secret.Name) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("name", secret.Name))
	}
	if len(secret.Type) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("type", secret.Type))
	}
	if len(secret.Data) == 0 {
		allErrs = append(allErrs, errs.NewFieldRequired("data", secret.Data))
	}
	if !util.IsDNS952Label(secret.Name) {
		allErrs = append(allErrs, errs.NewFieldInvalid("name", secret.Name, "Name must conform to the DNS label format"))
	}
	return allErrs
}

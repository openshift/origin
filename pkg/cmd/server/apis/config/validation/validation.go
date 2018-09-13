package validation

import (
	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation/common"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
)

func ValidateHTTPServingInfo(info config.HTTPServingInfo, fldPath *field.Path) common.ValidationResults {
	validationResults := common.ValidationResults{}

	validationResults.Append(common.ValidateServingInfo(info.ServingInfo, true, fldPath))

	if info.MaxRequestsInFlight < 0 {
		validationResults.AddErrors(field.Invalid(fldPath.Child("maxRequestsInFlight"), info.MaxRequestsInFlight, "must be zero (no limit) or greater"))
	}

	if info.RequestTimeoutSeconds < -1 {
		validationResults.AddErrors(field.Invalid(fldPath.Child("requestTimeoutSeconds"), info.RequestTimeoutSeconds, "must be -1 (no timeout), 0 (default timeout), or greater"))
	}

	return validationResults
}

func ValidateKubeConfig(path string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, common.ValidateFile(path, fldPath)...)
	// TODO: load and parse

	return allErrs
}

func ValidateRemoteConnectionInfo(remoteConnectionInfo config.RemoteConnectionInfo, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(remoteConnectionInfo.URL) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("url"), ""))
	} else {
		_, urlErrs := common.ValidateURL(remoteConnectionInfo.URL, fldPath.Child("url"))
		allErrs = append(allErrs, urlErrs...)
	}

	if len(remoteConnectionInfo.CA) > 0 {
		allErrs = append(allErrs, common.ValidateFile(remoteConnectionInfo.CA, fldPath.Child("ca"))...)
	}

	allErrs = append(allErrs, common.ValidateCertInfo(remoteConnectionInfo.ClientCert, false, fldPath)...)

	return allErrs
}

func ValidatePodManifestConfig(podManifestConfig *config.PodManifestConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// the Path can be a file or a directory
	allErrs = append(allErrs, common.ValidateFile(podManifestConfig.Path, fldPath.Child("path"))...)
	if podManifestConfig.FileCheckIntervalSeconds < 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("fileCheckIntervalSeconds"), podManifestConfig.FileCheckIntervalSeconds, "interval has to be positive"))
	}

	return allErrs
}

func ValidateExtendedArguments(config config.ExtendedArguments, flagFunc func(*pflag.FlagSet), fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// check extended arguments for errors
	for _, err := range cmdflags.Resolve(config, flagFunc) {
		switch t := err.(type) {
		case *field.Error:
			allErrs = append(allErrs, t)
		default:
			allErrs = append(allErrs, field.Invalid(fldPath.Child("????"), config, err.Error()))
		}
	}

	return allErrs
}

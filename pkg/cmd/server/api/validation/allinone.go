package validation

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/cmd/server/api"
)

func ValidateAllInOneConfig(master *api.MasterConfig, node *api.NodeConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, ValidateMasterConfig(master).Prefix("masterConfig")...)

	allErrs = append(allErrs, ValidateNodeConfig(node).Prefix("nodeConfig")...)

	// Validation between the configs

	return allErrs
}

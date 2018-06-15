package validation

import (
	"github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation/common"
)

func ValidateAllInOneConfig(master *config.MasterConfig, node *config.NodeConfig) common.ValidationResults {
	validationResults := common.ValidationResults{}

	validationResults.Append(ValidateMasterConfig(master, nil))

	validationResults.Append(ValidateNodeConfig(node, nil))

	// Validation between the configs

	return validationResults
}

package validation

import "github.com/openshift/origin/pkg/cmd/server/apis/config"

func ValidateAllInOneConfig(master *config.MasterConfig, node *config.NodeConfig) ValidationResults {
	validationResults := ValidationResults{}

	validationResults.Append(ValidateMasterConfig(master, nil))

	validationResults.Append(ValidateNodeConfig(node, nil))

	// Validation between the configs

	return validationResults
}

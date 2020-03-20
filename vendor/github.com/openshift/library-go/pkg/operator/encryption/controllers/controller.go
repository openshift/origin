package controllers

import (
	"github.com/openshift/library-go/pkg/operator/management"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
)

func shouldRunEncryptionController(operatorClient operatorv1helpers.OperatorClient) (bool, error) {
	operatorSpec, _, _, err := operatorClient.GetOperatorState()
	if err != nil {
		return false, err
	}

	return management.IsOperatorManaged(operatorSpec.ManagementState), nil
}

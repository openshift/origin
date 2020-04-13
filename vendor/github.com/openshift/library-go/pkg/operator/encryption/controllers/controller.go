package controllers

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/openshift/library-go/pkg/operator/management"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
)

// Provider abstracts external dependencies and preconditions that need to be dynamic during a downgrade/upgrade
type Provider interface {
	// EncryptedGRs returns resources that need to be encrypted
	EncryptedGRs() []schema.GroupResource

	// ShouldRunEncryptionControllers indicates whether external preconditions are satisfied so that encryption controllers can start synchronizing
	ShouldRunEncryptionControllers() (bool, error)
}

func shouldRunEncryptionController(operatorClient operatorv1helpers.OperatorClient, shouldRunFn func() (bool, error)) (bool, error) {

	if shouldRun, err := shouldRunFn(); !shouldRun || err != nil {
		return false, err
	}

	operatorSpec, _, _, err := operatorClient.GetOperatorState()
	if err != nil {
		return false, err
	}

	return management.IsOperatorManaged(operatorSpec.ManagementState), nil
}

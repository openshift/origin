package operator

import (
	"github.com/spf13/cobra"

	"github.com/openshift/service-serving-cert-signer/pkg/boilerplate/controllercmd"
	"github.com/openshift/service-serving-cert-signer/pkg/operator"
	"github.com/openshift/service-serving-cert-signer/pkg/version"
)

const componentName = "openshift-service-cert-signer-operator"

func NewOperator() *cobra.Command {
	cmd := controllercmd.
		NewControllerCommandConfig(componentName, version.Get()).
		WithStartFunc(operator.RunOperator).
		NewCommand()
	cmd.Use = "operator"
	cmd.Short = "Start the Service Cert Signer Operator"
	return cmd
}

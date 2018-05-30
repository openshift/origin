package webconsole_operator

import (
	"io"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/origin/pkg/version"
)

const (
	RecommendedWebConsoleOperatorName = "openshift-webconsole-operator"
)

type WebConsoleOperatorCommandOptions struct {
	Output io.Writer
}

var longDescription = templates.LongDesc(`
	Install the OpenShift webconsole`)

func NewWebConsoleOperatorCommand(name string) *cobra.Command {
	cmd := controllercmd.
		NewControllerCommandConfig("openshift-web-console-operator", version.Get(), RunWebConsoleOperator).
		NewCommand()
	cmd.Use = name
	cmd.Short = "Install and operate the OpenShift webconsole"

	return cmd
}

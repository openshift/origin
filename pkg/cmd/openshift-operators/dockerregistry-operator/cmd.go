package registry_operator

import (
	"io"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/origin/pkg/version"
)

const (
	RecommendedDockerRegistryOperatorName = "openshift-docker-registry-operator"
)

type DockerRegistryOperatorCommandOptions struct {
	Output io.Writer
}

var longDescription = templates.LongDesc(`
	Install the OpenShift Docker Registry`)

func NewDockerRegistryOperatorCommand(name string) *cobra.Command {
	cmd := controllercmd.
		NewControllerCommandConfig(RecommendedDockerRegistryOperatorName, version.Get(), RunDockerRegistryOperator).
		NewCommand()
	cmd.Use = name
	cmd.Short = "Install and operate the OpenShift Docker Registry"

	return cmd
}

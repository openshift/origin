package openshift

import (
	"fmt"

	kubeversion "github.com/GoogleCloudPlatform/kubernetes/pkg/version"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/client"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	"github.com/openshift/origin/pkg/cmd/infra/builder"
	"github.com/openshift/origin/pkg/cmd/infra/deployer"
	"github.com/openshift/origin/pkg/cmd/infra/router"
	"github.com/openshift/origin/pkg/cmd/kubectl"
	"github.com/openshift/origin/pkg/cmd/server"
	"github.com/openshift/origin/pkg/version"
)

const longDescription = `
OpenShift for Admins

OpenShift helps you build, deploy, and manage your applications. To start an all-in-one server, run:

    $ openshift start &

OpenShift is built around Docker and the Kubernetes cluster container manager.  You must have
Docker installed on this machine to start your server.

Note: This is an alpha release of OpenShift and will change significantly.  See

    https://github.com/openshift/origin

for the latest information on OpenShift.

`

// CommandFor returns the appropriate command for this base name,
// or the global OpenShift command
func CommandFor(basename string) *cobra.Command {
	switch basename {
	case "openshift-router":
		return router.NewCommandRouter(basename)
	case "openshift-deploy":
		return deployer.NewCommandDeployer(basename)
	case "openshift-sti-build":
		return builder.NewCommandSTIBuilder(basename)
	case "openshift-docker-build":
		return builder.NewCommandDockerBuilder(basename)
	case "kubectl":
		return kubectl.NewCommandKubectl(basename)
	case "openshift-rollback":
		return NewCommandRollback()
	default:
		return NewCommandOpenShift()
	}
}

// NewCommandOpenShift creates the standard OpenShift command
func NewCommandOpenShift() *cobra.Command {
	root := &cobra.Command{
		Use:   "openshift",
		Short: "OpenShift helps you build, deploy, and manage your cloud applications",
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}

	root.AddCommand(server.NewCommandStartServer("start"))
	root.AddCommand(client.NewCommandKubecfg("kube"))
	root.AddCommand(kubectl.NewCommandKubectl("kubectl"))
	root.AddCommand(newVersionCommand("version"))

	// infra commands are those that are bundled with the binary but not displayed to end users
	// directly
	infra := &cobra.Command{
		Use: "infra", // Because this command exposes no description, it will not be shown in help
	}
	infra.AddCommand(
		router.NewCommandRouter("router"),
		deployer.NewCommandDeployer("deploy"),
		builder.NewCommandSTIBuilder("sti-build"),
		builder.NewCommandDockerBuilder("docker-build"),
	)
	root.AddCommand(infra)

	flagtypes.GLog(root.PersistentFlags())
	return root
}

// newVersionCommand creates a command for displaying the version of this binary
func newVersionCommand(use string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: "Display version",
		Run: func(c *cobra.Command, args []string) {
			fmt.Printf("openshift %v\n", version.Get())
			fmt.Printf("kubernetes %v\n", kubeversion.Get())
		},
	}
}

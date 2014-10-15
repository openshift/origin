package main

import (
	"fmt"
	"os"

	kubeversion "github.com/GoogleCloudPlatform/kubernetes/pkg/version"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/client"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
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

func main() {
	openshiftCmd := &cobra.Command{
		Use:   "openshift",
		Short: "OpenShift helps you build, deploy, and manage your applications",
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}

	openshiftCmd.AddCommand(server.NewCommandStartServer("start"))
	openshiftCmd.AddCommand(client.NewCommandKubecfg("kube"))
	flagtypes.GLog(openshiftCmd.PersistentFlags())

	// version information
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Display version",
		Run: func(c *cobra.Command, args []string) {
			fmt.Printf("openshift %v\n", version.Get())
			fmt.Printf("kubernetes %v\n", kubeversion.Get())
		},
	}
	openshiftCmd.AddCommand(versionCmd)

	if err := openshiftCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err)
		os.Exit(1)
	}
}

package main

import (
	"fmt"
	"os"

	kubeversion "github.com/GoogleCloudPlatform/kubernetes/pkg/version"
	"github.com/openshift/origin/pkg/cmd/client"
	"github.com/openshift/origin/pkg/cmd/master"
	"github.com/openshift/origin/pkg/cmd/helpers"
	"github.com/openshift/origin/pkg/version"
	"github.com/spf13/cobra"
)

const longDescription = `
OpenShift for Admins

OpenShift helps you build, deploy, and manage your applications. To start an all-in-one server, run:

    $ openshift start &
    $ openshift kube create services -c examples/test-service.json

OpenShift is built around Docker and the Kubernetes container orchestration service.  You must have
Docker installed on this machine to start your server.

Note: This is an alpha release of OpenShift and will change significantly.  See

    https://github.com/openshift/origin

for the latest information on OpenShift.

`
type Globals struct {
	Verbose bool
}

func main() {

	// global flags values
	globals := Globals{}

	// root command
	openshiftCmd := &cobra.Command{
		Use:   "openshift",
		Short: "OpenShift helps you build, deploy, and manage your applications",
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}

	// version information
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Display version",
		Run: func(c *cobra.Command, args []string) {
			major, minor, git := version.Get()
			dockerVersion, dockerBuild := helpers.DockerServerVersion()
			fmt.Printf("openshift version %s.%s, build %s\n", major, minor, git)
			fmt.Printf("kubernetes %v\n", kubeversion.Get())
			fmt.Printf("golang version %v\n", helpers.GoVersion())
			fmt.Printf("docker version %v, build %s\n", dockerVersion, dockerBuild)
		},
	}

	// global flags
	openshiftCmd.PersistentFlags().BoolVarP(&globals.Verbose, "verbose", "v", false, "If provided, print extra information")

	// child commands
	openshiftCmd.AddCommand(master.NewCommandStartAllInOne("start"))
	openshiftCmd.AddCommand(client.NewCommandKubecfg("kube"))
	openshiftCmd.AddCommand(versionCmd)

	if err := openshiftCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err)
		os.Exit(1)
	}
}

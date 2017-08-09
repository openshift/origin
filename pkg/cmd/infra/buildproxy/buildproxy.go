package buildproxy

import (
	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/build/proxy"
)

var (
	buildProxyLong = templates.LongDesc(`
		Start a Docker build proxy

		This command launches a proxy that handles the Docker build API and enforces authorization 
		checks from the client.`)
)

// NewCommandBuildProxy provides a command that runs a Docker build proxy
func NewCommandBuildProxy(name string) *cobra.Command {
	server := &proxy.Server{
		ListenAddrs: []string{"unix:///var/run/openshift-build-proxy"},
		Mode:        "passthrough",
	}
	cmd := &cobra.Command{
		Use:   name,
		Short: "Start a build proxy",
		Long:  buildProxyLong,
		Run: func(c *cobra.Command, args []string) {
			err := server.Start()
			kcmdutil.CheckErr(err)
		},
	}
	cmd.Flags().StringSliceVar(&server.ListenAddrs, "listen", server.ListenAddrs, "One or more unix:// or tcp:// sockets to listen on for build connections.")
	cmd.Flags().StringVar(&server.AllowHost, "hostname", server.AllowHost, "The Docker authorization config to accept authentication requests on. Defaults to a random value.")
	cmd.Flags().StringVar(&server.Mode, "mode", server.Mode, "The backend build implementation to use. Accepts 'imagebuilder' or 'passthrough'.")
	return cmd
}

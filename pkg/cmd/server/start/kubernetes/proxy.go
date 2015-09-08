package kubernetes

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/kubernetes/cmd/kube-proxy/app"
	"k8s.io/kubernetes/pkg/util"
)

const proxyLong = `
Start Kubernetes Proxy

This command launches an instance of the Kubernetes proxy (kube-proxy).`

// NewProxyCommand provides a CLI handler for the 'proxy' command
func NewProxyCommand(name, fullName string, out io.Writer) *cobra.Command {
	s := app.NewProxyServer()

	cmd := &cobra.Command{
		Use:   name,
		Short: "Launch Kubernetes proxy (kube-proxy)",
		Long:  proxyLong,
		Run: func(c *cobra.Command, args []string) {
			startProfiler()

			util.InitLogs()
			defer util.FlushLogs()

			if err := s.Run(pflag.CommandLine.Args()); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}
	cmd.SetOutput(out)

	flags := cmd.Flags()
	//TODO: uncomment after picking up a newer cobra
	//pflag.AddFlagSetToPFlagSet(flag, flags)
	s.AddFlags(flags)

	return cmd
}

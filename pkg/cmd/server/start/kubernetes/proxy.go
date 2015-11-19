package kubernetes

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/kubernetes/cmd/kube-proxy/app"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/util"
)

const proxyLong = `
Start Kubernetes Proxy

This command launches an instance of the Kubernetes proxy (kube-proxy).`

// NewProxyCommand provides a CLI handler for the 'proxy' command
func NewProxyCommand(name, fullName string, out io.Writer) *cobra.Command {
	proxyConfig := app.NewProxyConfig()

	cmd := &cobra.Command{
		Use:   name,
		Short: "Launch Kubernetes proxy (kube-proxy)",
		Long:  proxyLong,
		Run: func(c *cobra.Command, args []string) {
			startProfiler()

			util.InitLogs()
			defer util.FlushLogs()

			s, err := app.NewProxyServerDefault(proxyConfig)
			kcmdutil.CheckErr(err)

			if err := s.Run(pflag.CommandLine.Args()); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}
	cmd.SetOutput(out)

	flags := cmd.Flags()
	flags.SetNormalizeFunc(util.WordSepNormalizeFunc)
	flags.AddGoFlagSet(flag.CommandLine)
	proxyConfig.AddFlags(flags)

	return cmd
}

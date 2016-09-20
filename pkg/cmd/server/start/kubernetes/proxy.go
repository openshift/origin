package kubernetes

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	proxyapp "k8s.io/kubernetes/cmd/kube-proxy/app"
	proxyoptions "k8s.io/kubernetes/cmd/kube-proxy/app/options"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kflag "k8s.io/kubernetes/pkg/util/flag"
	"k8s.io/kubernetes/pkg/util/logs"
)

const proxyLong = `
Start Kubernetes Proxy

This command launches an instance of the Kubernetes proxy (kube-proxy).`

// NewProxyCommand provides a CLI handler for the 'proxy' command
func NewProxyCommand(name, fullName string, out io.Writer) *cobra.Command {
	proxyConfig := proxyoptions.NewProxyConfig()

	cmd := &cobra.Command{
		Use:   name,
		Short: "Launch Kubernetes proxy (kube-proxy)",
		Long:  proxyLong,
		Run: func(c *cobra.Command, args []string) {
			startProfiler()

			logs.InitLogs()
			defer logs.FlushLogs()

			s, err := proxyapp.NewProxyServerDefault(proxyConfig)
			kcmdutil.CheckErr(err)

			if err := s.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}
	cmd.SetOutput(out)

	flags := cmd.Flags()
	flags.SetNormalizeFunc(kflag.WordSepNormalizeFunc)
	proxyConfig.AddFlags(flags)

	return cmd
}

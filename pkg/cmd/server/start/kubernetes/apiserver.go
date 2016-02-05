package kubernetes

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	apiserverapp "k8s.io/kubernetes/cmd/kube-apiserver/app"
	apiserveroptions "k8s.io/kubernetes/cmd/kube-apiserver/app/options"
	"k8s.io/kubernetes/pkg/util"
)

const apiserverLong = `
Start Kubernetes apiserver

This command launches an instance of the Kubernetes apiserver (kube-apiserver).`

// NewAPIServerCommand provides a CLI handler for the 'apiserver' command
func NewAPIServerCommand(name, fullName string, out io.Writer) *cobra.Command {
	apiServerOptions := apiserveroptions.NewAPIServer()

	cmd := &cobra.Command{
		Use:   name,
		Short: "Launch Kubernetes apiserver (kube-apiserver)",
		Long:  apiserverLong,
		Run: func(c *cobra.Command, args []string) {
			startProfiler()

			util.InitLogs()
			defer util.FlushLogs()

			if err := apiserverapp.Run(apiServerOptions); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}
	cmd.SetOutput(out)

	flags := cmd.Flags()
	flags.SetNormalizeFunc(util.WordSepNormalizeFunc)
	flags.AddGoFlagSet(flag.CommandLine)
	apiServerOptions.AddFlags(flags)

	return cmd
}

package kubernetes

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	apiserverapp "k8s.io/kubernetes/cmd/kube-apiserver/app"
	apiserveroptions "k8s.io/kubernetes/cmd/kube-apiserver/app/options"
	kflag "k8s.io/kubernetes/pkg/util/flag"
	"k8s.io/kubernetes/pkg/util/logs"
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

			logs.InitLogs()
			defer logs.FlushLogs()

			if err := apiserverapp.Run(apiServerOptions); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}
	cmd.SetOutput(out)

	flags := cmd.Flags()
	flags.SetNormalizeFunc(kflag.WordSepNormalizeFunc)
	apiServerOptions.AddFlags(flags)

	return cmd
}

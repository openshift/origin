package kubernetes

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	controllerapp "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	controlleroptions "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	"k8s.io/kubernetes/pkg/util"
)

const controllersLong = `
Start Kubernetes controller manager

This command launches an instance of the Kubernetes controller-manager (kube-controller-manager).`

// NewControllersCommand provides a CLI handler for the 'controller-manager' command
func NewControllersCommand(name, fullName string, out io.Writer) *cobra.Command {
	controllerOptions := controlleroptions.NewCMServer()

	cmd := &cobra.Command{
		Use:   name,
		Short: "Launch Kubernetes controller manager (kube-controller-manager)",
		Long:  controllersLong,
		Run: func(c *cobra.Command, args []string) {
			startProfiler()

			util.InitLogs()
			defer util.FlushLogs()

			if err := controllerapp.Run(controllerOptions); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}
	cmd.SetOutput(out)

	flags := cmd.Flags()
	flags.SetNormalizeFunc(util.WordSepNormalizeFunc)
	flags.AddGoFlagSet(flag.CommandLine)
	controllerOptions.AddFlags(flags)

	return cmd
}

package kubernetes

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/GoogleCloudPlatform/kubernetes/cmd/kube-apiserver/app"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

const apiserverLong = `
Start Kubernetes apiserver

This command launches an instance of the Kubernetes apiserver (kube-apiserver).`

// NewAPIServerCommand provides a CLI handler for the 'apiserver' command
func NewAPIServerCommand(name, fullName string, out io.Writer) *cobra.Command {
	s := app.NewAPIServer()

	cmd := &cobra.Command{
		Use:   name,
		Short: "Launch Kubernetes apiserver (kube-apiserver)",
		Long:  apiserverLong,
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

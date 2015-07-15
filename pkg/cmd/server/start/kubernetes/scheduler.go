package kubernetes

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/cmd/kube-scheduler/app"
)

const schedulerLong = `
Start Kubernetes scheduler

This command launches an instance of the Kubernetes controller-manager (kube-controller-manager).`

// NewSchedulerCommand provides a CLI handler for the 'scheduler' command
func NewSchedulerCommand(name, fullName string, out io.Writer) *cobra.Command {
	s := app.NewSchedulerServer()

	cmd := &cobra.Command{
		Use:   name,
		Short: "Launch Kubernetes scheduler (kube-scheduler)",
		Long:  controllersLong,
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

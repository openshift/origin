package kubernetes

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	kflag "k8s.io/kubernetes/pkg/util/flag"
	"k8s.io/kubernetes/pkg/util/logs"
	schedulerapp "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app"
	scheduleroptions "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"
)

const schedulerLong = `
Start Kubernetes scheduler

This command launches an instance of the Kubernetes controller-manager (kube-controller-manager).`

// NewSchedulerCommand provides a CLI handler for the 'scheduler' command
func NewSchedulerCommand(name, fullName string, out io.Writer) *cobra.Command {
	schedulerOptions := scheduleroptions.NewSchedulerServer()

	cmd := &cobra.Command{
		Use:   name,
		Short: "Launch Kubernetes scheduler (kube-scheduler)",
		Long:  controllersLong,
		Run: func(c *cobra.Command, args []string) {
			startProfiler()

			logs.InitLogs()
			defer logs.FlushLogs()

			if err := schedulerapp.Run(schedulerOptions); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}
	cmd.SetOutput(out)

	flags := cmd.Flags()
	flags.SetNormalizeFunc(kflag.WordSepNormalizeFunc)
	schedulerOptions.AddFlags(flags)

	return cmd
}

package kubernetes

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	kubeletapp "k8s.io/kubernetes/cmd/kubelet/app"
	kubeletoptions "k8s.io/kubernetes/cmd/kubelet/app/options"
	kflag "k8s.io/kubernetes/pkg/util/flag"
	"k8s.io/kubernetes/pkg/util/logs"
)

const kubeletLog = `Start Kubelet

This command launches a Kubelet. All options are exposed. Use 'openshift start node' for
starting from a configuration file.`

// NewKubeletCommand provides a CLI handler for the 'kubelet' command
func NewKubeletCommand(name, fullName string, out io.Writer) *cobra.Command {
	kubeletOptions := kubeletoptions.NewKubeletServer()

	cmd := &cobra.Command{
		Use:   name,
		Short: "Launch the Kubelet (kubelet)",
		Long:  kubeletLog,
		Run: func(c *cobra.Command, args []string) {
			startProfiler()

			logs.InitLogs()
			defer logs.FlushLogs()

			if err := kubeletapp.Run(kubeletOptions, nil); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}
	cmd.SetOutput(out)

	flags := cmd.Flags()
	flags.SetNormalizeFunc(kflag.WordSepNormalizeFunc)
	kubeletOptions.AddFlags(flags)

	return cmd
}

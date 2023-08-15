package watch_endpointslice

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

type WatchEndpointSliceFlags struct {
	ConfigFlags            *genericclioptions.ConfigFlags
	RecordedDisruptionFile string

	genericclioptions.IOStreams
}

func NewWatchEndpointSliceFlags(streams genericclioptions.IOStreams) *WatchEndpointSliceFlags {
	return &WatchEndpointSliceFlags{
		ConfigFlags: genericclioptions.NewConfigFlags(false),
		IOStreams:   streams,
	}
}

func NewWatchEndpointSlice(ioStreams genericclioptions.IOStreams) *cobra.Command {
	f := NewWatchEndpointSliceFlags(ioStreams)
	cmd := &cobra.Command{
		Use:   "watch-endpoint-slice",
		Short: "Continuously poll endpoints to check availability",

		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancelFn := context.WithCancel(context.Background())
			defer cancelFn()

			// TODO wire sig-term?

			//validate stuff
			o, err := f.ToOptions()
			if err != nil {
				return err
			}
			return o.Run(ctx)

			return nil
		},
	}

	f.BindOptions(cmd.Flags())

	return cmd
}

func (f *WatchEndpointSliceFlags) BindOptions(flags *pflag.FlagSet) {
	flags.StringVar(&f.RecordedDisruptionFile, "disruption-file", f.RecordedDisruptionFile, "file containing json of disruption.")
	f.ConfigFlags.AddFlags(flags)
}

func (f *WatchEndpointSliceFlags) ToOptions() (*WatchEndpointSliceOptions, error) {
	restConfig, err := f.ConfigFlags.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &WatchEndpointSliceOptions{
		KubeClient:             kubeClient,
		RecordedDisruptionFile: f.RecordedDisruptionFile,
		IOStreams:              f.IOStreams,
	}, nil
}

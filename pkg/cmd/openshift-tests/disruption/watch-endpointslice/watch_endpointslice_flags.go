package watch_endpointslice

import (
	"context"
	"fmt"

	"k8s.io/client-go/kubernetes"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// WatchEndpointSliceFlags is used to run a monitoring process against the provided server as
// a command line interaction.
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
		Short: "Continuously poll endpoints to check availability.",

		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancelFn := context.WithCancel(context.Background())
			defer cancelFn()

			// TOOD wire to sig-term

			if err := f.Validate(); err != nil {
				return err
			}
			o, err := f.ToOptions()
			if err != nil {
				return err
			}
			return o.Run(ctx)
		},
	}

	f.BindOptions(cmd.Flags())

	return cmd
}

func (f *WatchEndpointSliceFlags) BindOptions(flags *pflag.FlagSet) {
	flags.StringVar(&f.RecordedDisruptionFile, "disruption-file", f.RecordedDisruptionFile, "file containing jsonl of disruption.")
	f.ConfigFlags.AddFlags(flags)
}

func (f *WatchEndpointSliceFlags) Validate() error {
	if len(f.RecordedDisruptionFile) == 0 {
		return fmt.Errorf("disruption-file must be specified")
	}
	return nil
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

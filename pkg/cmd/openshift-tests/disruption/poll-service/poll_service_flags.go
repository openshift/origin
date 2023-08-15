package poll_service

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/openshift/origin/pkg/clioptions/iooptions"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	// the env variable for the clusterIP of the service to be polled
	ServiceClusterIPENV = "SERVICE_CLUSTER_IP"
	// the env variable for the cluster IP port to poll
	ServicePortENV = "SERVICE_PORT"
)

// PollServiceFlags is used to run a monitoring process against the provided server as
// a command line interaction.
type PollServiceFlags struct {
	ConfigFlags *genericclioptions.ConfigFlags
	OutputFlags *iooptions.OutputFlags

	genericclioptions.IOStreams
}

func NewPollServiceFlags(streams genericclioptions.IOStreams) *PollServiceFlags {
	return &PollServiceFlags{
		ConfigFlags: genericclioptions.NewConfigFlags(false),
		OutputFlags: iooptions.NewOutputOptions(),
		IOStreams:   streams,
	}
}

func NewPollService(ioStreams genericclioptions.IOStreams) *cobra.Command {
	f := NewPollServiceFlags(ioStreams)
	cmd := &cobra.Command{
		Use:   "poll-service",
		Short: "Continuously poll service to check availability",

		SilenceUsage:  true,
		SilenceErrors: true,
		/*
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx, cancelFn := context.WithCancel(context.Background())
				defer cancelFn()
				abortCh := make(chan os.Signal, 2)
				go func() {
					<-abortCh
					fmt.Fprintf(f.ErrOut, "Interrupted, terminating\n")
					cancelFn()

					sig := <-abortCh
					fmt.Fprintf(f.ErrOut, "Interrupted twice, exiting (%s)\n", sig)
					switch sig {
					case syscall.SIGINT:
						os.Exit(130)
					default:
						os.Exit(0)
					}
				}()
				signal.Notify(abortCh, syscall.SIGINT, syscall.SIGTERM)

				if err := f.Validate(); err != nil {
					return err
				}
				o, err := f.ToOptions()
				if err != nil {
					return err
				}
				return o.Run(ctx)
			},
		*/
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancelFn := context.WithCancel(context.Background())
			abortCh := make(chan os.Signal, 2)
			defer cancelFn()
			go func() {
				<-abortCh
				fmt.Fprintf(f.ErrOut, "Inmterrupted, terminating\n")
				cancelFn()

				sig := <-abortCh
				fmt.Fprintf(f.ErrOut, "Interrupted twice, exiting (%s)\n", sig)
				switch sig {
				case syscall.SIGINT:
					os.Exit(130)
				default:
					os.Exit(0)
				}

			}()
			signal.Notify(abortCh, syscall.SIGINT, syscall.SIGTERM)

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

func (f *PollServiceFlags) BindOptions(flags *pflag.FlagSet) {
	f.ConfigFlags.AddFlags(flags)
	f.OutputFlags.BindFlags(flags)
}

func (f *PollServiceFlags) SetIOStreams(streams genericclioptions.IOStreams) {
	f.IOStreams = streams
}

func (f *PollServiceFlags) Validate() error {
	if len(f.OutputFlags.OutFile) == 0 {
		return fmt.Errorf("output-file must be specified")
	}
	return nil
}

func (f *PollServiceFlags) ToOptions() (*PollServiceOptions, error) {
	originalOutStream := f.IOStreams.Out
	closeFn, err := f.OutputFlags.ConfigureIOStreams(f.IOStreams, f)
	if err != nil {
		return nil, nil
	}

	return &PollServiceOptions{
		OutputFile:      f.OutputFlags.OutFile,
		ClusterIP:       os.Getenv(ServiceClusterIPENV),
		Port:            os.Getenv(ServicePortENV),
		OriginalOutFile: originalOutStream,
		CloseFn:         closeFn,
		IOStreams:       f.IOStreams,
	}, nil

}

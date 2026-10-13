package watch_endpointslice

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/client-go/kubernetes"

	"github.com/openshift/origin/pkg/clioptions/iooptions"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// WatchEndpointSliceFlags is used to run a monitoring process against the provided server as
// a command line interaction.
type WatchEndpointSliceFlags struct {
	ConfigFlags        *genericclioptions.ConfigFlags
	OutputFlags        *iooptions.OutputFlags
	ServiceName        string
	BackendPrefix      string
	Scheme             string
	Path               string
	ExpectedStatusCode int
	MyNodeName         string
	StopConfigMapName  string

	genericclioptions.IOStreams
}

func NewWatchEndpointSliceFlags(streams genericclioptions.IOStreams) *WatchEndpointSliceFlags {
	return &WatchEndpointSliceFlags{
		ConfigFlags: genericclioptions.NewConfigFlags(false),
		OutputFlags: iooptions.NewOutputOptions(),
		Scheme:      "https",
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
	}

	f.BindOptions(cmd.Flags())

	return cmd
}

func (f *WatchEndpointSliceFlags) BindOptions(flags *pflag.FlagSet) {
	flags.StringVar(&f.MyNodeName, "my-node-name", f.MyNodeName, "the name of the node running this pod")
	flags.StringVar(&f.ServiceName, "disruption-target-service-name", f.ServiceName, "the name of the service whose endpoints we want to poll")
	flags.StringVar(&f.BackendPrefix, "disruption-backend-prefix", f.BackendPrefix, "classification of disruption for the disruption summary")
	flags.StringVar(&f.StopConfigMapName, "stop-configmap", f.StopConfigMapName, "the name of the configmap that indicates that this pod should stop all watchers.")
	flags.StringVar(&f.Scheme, "request-scheme", f.Scheme, "http or https")
	flags.StringVar(&f.Path, "request-path", f.Path, "path to request, like /healthz")
	flags.IntVar(&f.ExpectedStatusCode, "expected-status-code", f.ExpectedStatusCode, "status code to expect from the sampler")
	f.ConfigFlags.AddFlags(flags)
	f.OutputFlags.BindFlags(flags)
}

func (f *WatchEndpointSliceFlags) SetIOStreams(streams genericclioptions.IOStreams) {
	f.IOStreams = streams
}

func (f *WatchEndpointSliceFlags) Validate() error {
	if len(f.OutputFlags.OutFile) == 0 {
		return fmt.Errorf("output-file must be specified")
	}
	if len(f.ServiceName) == 0 {
		return fmt.Errorf("disruption-target-label-value must be specified")
	}
	if len(f.BackendPrefix) == 0 {
		return fmt.Errorf("disruption-backend-prefix must be specified")
	}

	return nil
}

func (f *WatchEndpointSliceFlags) ToOptions() (*WatchEndpointSliceOptions, error) {
	originalOutStream := f.IOStreams.Out
	closeFn, err := f.OutputFlags.ConfigureIOStreams(f.IOStreams, f)
	if err != nil {
		return nil, err
	}

	namespace, _, err := f.ConfigFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return nil, err
	}
	if len(namespace) == 0 {
		return nil, fmt.Errorf("namespace must be specified")
	}

	restConfig, err := f.ConfigFlags.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &WatchEndpointSliceOptions{
		KubeClient:         kubeClient,
		Namespace:          namespace,
		OutputFile:         f.OutputFlags.OutFile,
		ServiceName:        f.ServiceName,
		StopConfigMapName:  f.StopConfigMapName,
		Scheme:             f.Scheme,
		Path:               f.Path,
		MyNodeName:         f.MyNodeName,
		BackendPrefix:      f.BackendPrefix,
		ExpectedStatusCode: f.ExpectedStatusCode,
		CloseFn:            closeFn,
		OriginalOutFile:    originalOutStream,
		IOStreams:          f.IOStreams,
	}, nil
}

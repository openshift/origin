package poll_service

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/openshift/origin/pkg/clioptions/iooptions"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

type PollServiceFlags struct {
	ConfigFlags       *genericclioptions.ConfigFlags
	OutputFlags       *iooptions.OutputFlags
	BackendPrefix     string
	MyNodeName        string
	StopConfigMapName string
	ServiceClusterIP  string
	ServicePort       uint16

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
		Short: "Continuously poll service to check endpoints availability",

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

func (f *PollServiceFlags) BindOptions(flags *pflag.FlagSet) {
	flags.StringVar(&f.MyNodeName, "my-node-name", f.MyNodeName, "the name of the node running this pod")
	flags.StringVar(&f.StopConfigMapName, "stop-configmap", f.StopConfigMapName, "the name of the configmap that indicates that this pod should stop all watchers.")
	flags.StringVar(&f.ServiceClusterIP, "service-clusterIP", f.ServiceClusterIP, "the service clusterIP to poll")
	flags.Uint16Var(&f.ServicePort, "service-port", f.ServicePort, "the exposed port on the service to poll")
	flags.StringVar(&f.BackendPrefix, "disruption-backend-prefix", f.BackendPrefix, "classification of disruption for the disruption summery")
	f.ConfigFlags.AddFlags(flags)
	f.OutputFlags.BindFlags(flags)
}

func (f *PollServiceFlags) Validate() error {
	if len(f.OutputFlags.OutFile) == 0 {
		return fmt.Errorf("output-file must be specified")
	}
	if ip := net.ParseIP(f.ServiceClusterIP); ip == nil {
		return fmt.Errorf("service-clusterIP must be a valid IP address")
	}

	if len(f.BackendPrefix) == 0 {
		return fmt.Errorf("must specify disruption-backend-prefix")
	}
	return nil
}

func (f *PollServiceFlags) SetIOStreams(streams genericclioptions.IOStreams) {
	f.IOStreams = streams
}

func (f *PollServiceFlags) ToOptions() (*PollServiceOptions, error) {
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
	return &PollServiceOptions{
		KubeClient:        kubeClient,
		Namespace:         namespace,
		OutputFile:        f.OutputFlags.OutFile,
		BackendPrefix:     f.BackendPrefix,
		ClusterIP:         f.ServiceClusterIP,
		Port:              f.ServicePort,
		StopConfigMapName: f.StopConfigMapName,
		MyNodeName:        f.MyNodeName,
		CloseFn:           closeFn,

		OriginalOutFile: originalOutStream,
		IOStreams:       f.IOStreams,
	}, nil
}

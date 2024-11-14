package run_disruption

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"k8s.io/apimachinery/pkg/fields"

	"github.com/openshift/origin/pkg/clioptions/iooptions"
	"github.com/openshift/origin/pkg/disruption/backend"
	disruptionci "github.com/openshift/origin/pkg/disruption/ci"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/test/extended/util/disruption/controlplane"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	apimachinerywatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/watch"
	"k8s.io/kubectl/pkg/util/templates"
)

type RunAPIDisruptionMonitorFlags struct {
	ConfigFlags *genericclioptions.ConfigFlags
	OutputFlags *iooptions.OutputFlags

	ArtifactDir       string
	LoadBalancerType  string
	Source            string
	StopConfigMapName string

	genericclioptions.IOStreams
}

func NewRunInClusterDisruptionMonitorFlags(ioStreams genericclioptions.IOStreams) *RunAPIDisruptionMonitorFlags {
	return &RunAPIDisruptionMonitorFlags{
		ConfigFlags: genericclioptions.NewConfigFlags(false),
		OutputFlags: iooptions.NewOutputOptions(),
		IOStreams:   ioStreams,
	}
}

func NewRunInClusterDisruptionMonitorCommand(ioStreams genericclioptions.IOStreams) *cobra.Command {
	f := NewRunInClusterDisruptionMonitorFlags(ioStreams)
	cmd := &cobra.Command{
		Use:   "run-disruption",
		Short: "Run API server disruption monitor",
		Long: templates.LongDesc(`
		Run a monitor which pings API servers and writes a file with intervals
		`),

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

	f.AddFlags(cmd.Flags())

	return cmd
}

func (f *RunAPIDisruptionMonitorFlags) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&f.LoadBalancerType, "lb-type", f.LoadBalancerType, "Set load balancer type, available options: internal-lb, service-network, external-lb (default)")
	flags.StringVar(&f.Source, "source-name", f.Source, "Set source identifier")
	flags.StringVar(&f.StopConfigMapName, "stop-configmap", f.StopConfigMapName, "the name of the configmap that indicates that this pod should stop all watchers.")

	f.ConfigFlags.AddFlags(flags)
	f.OutputFlags.BindFlags(flags)
}

func (f *RunAPIDisruptionMonitorFlags) SetIOStreams(streams genericclioptions.IOStreams) {
	f.IOStreams = streams
}

func (f *RunAPIDisruptionMonitorFlags) Validate() error {
	if len(f.OutputFlags.OutFile) == 0 {
		return fmt.Errorf("output-file must be specified")
	}
	if len(f.StopConfigMapName) == 0 {
		return fmt.Errorf("stop-configmap must be specified")
	}

	return nil
}

func (f *RunAPIDisruptionMonitorFlags) ToOptions() (*RunAPIDisruptionMonitorOptions, error) {
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

	return &RunAPIDisruptionMonitorOptions{
		KubeClient:        kubeClient,
		KubeClientConfig:  restConfig,
		OutputFile:        f.OutputFlags.OutFile,
		LoadBalancerType:  f.LoadBalancerType,
		StopConfigMapName: f.StopConfigMapName,
		Namespace:         namespace,
		CloseFn:           closeFn,
		OriginalOutFile:   originalOutStream,
		IOStreams:         f.IOStreams,
	}, nil
}

// RunAPIDisruptionMonitorOptions sets options for api server disruption monitor
type RunAPIDisruptionMonitorOptions struct {
	KubeClient        kubernetes.Interface
	KubeClientConfig  *rest.Config
	OutputFile        string
	LoadBalancerType  string
	Source            string
	StopConfigMapName string
	Namespace         string

	OriginalOutFile io.Writer
	CloseFn         iooptions.CloseFunc
	genericclioptions.IOStreams
}

func (o *RunAPIDisruptionMonitorOptions) Run(ctx context.Context) error {
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	startingContent, err := os.ReadFile(o.OutputFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(startingContent) > 0 {
		// print starting content to the log so that we can simply scrape the log to find all entries at the end.
		o.OriginalOutFile.Write(startingContent)
	}

	lb := backend.ParseStringToLoadBalancerType(o.LoadBalancerType)

	recorder := monitor.WrapWithJSONLRecorder(monitor.NewRecorder(), o.IOStreams.Out, nil)
	samplers, err := controlplane.StartAPIMonitoringUsingNewBackend(ctx, recorder, o.KubeClientConfig, o.KubeClient, lb, o.Source)
	if err != nil {
		return err
	}

	go func(ctx context.Context) {
		defer cancelFn()
		err := o.WaitForStopSignal(ctx)
		if err != nil {
			fmt.Fprintf(o.ErrOut, "failure waiting for stop: %v", err)
		}
	}(ctx)

	<-ctx.Done()

	fmt.Fprintf(o.Out, "waiting for samplers to stop")
	wg := sync.WaitGroup{}
	for i := range samplers {
		wg.Add(1)
		func(sampler disruptionci.Sampler) {
			defer wg.Done()
			sampler.Stop()
		}(samplers[i])
	}
	wg.Wait()
	fmt.Fprintf(o.Out, "samplers stopped")

	return nil
}

func (o *RunAPIDisruptionMonitorOptions) WaitForStopSignal(ctx context.Context) error {
	defer utilruntime.HandleCrash()

	_, err := watch.UntilWithSync(
		ctx,
		cache.NewListWatchFromClient(
			o.KubeClient.CoreV1().RESTClient(), "configmaps", o.Namespace, fields.OneTermEqualSelector("metadata.name", o.StopConfigMapName)),
		&corev1.ConfigMap{},
		nil,
		func(event apimachinerywatch.Event) (bool, error) {
			switch event.Type {
			case apimachinerywatch.Added:
				return true, nil
			case apimachinerywatch.Modified:
				return true, nil
			}
			return false, nil
		},
	)
	return err
}

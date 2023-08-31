package run_disruption

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/openshift/origin/pkg/disruption/backend"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/apiserveravailability"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/test/extended/util/disruption/controlplane"
	"github.com/spf13/cobra"
)

// RunAPIDisruptionMonitorOptions sets options for api server disruption monitor
type RunAPIDisruptionMonitorOptions struct {
	Out, ErrOut io.Writer

	ArtifactDir      string
	LoadBalancerType string
	ExtraMessage     string
}

func NewRunInClusterDisruptionMonitorOptions(ioStreams genericclioptions.IOStreams) *RunAPIDisruptionMonitorOptions {
	return &RunAPIDisruptionMonitorOptions{
		Out:    ioStreams.Out,
		ErrOut: ioStreams.ErrOut,
	}
}

func NewRunInClusterDisruptionMonitorCommand(ioStreams genericclioptions.IOStreams) *cobra.Command {
	disruptionOpt := NewRunInClusterDisruptionMonitorOptions(ioStreams)
	cmd := &cobra.Command{
		Use:   "run-disruption",
		Short: "Run API server disruption monitor",
		Long: templates.LongDesc(`
		Run a monitor which pings API servers and writes a file with intervals
		`),

		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return disruptionOpt.Run()
		},
	}
	cmd.Flags().StringVar(&disruptionOpt.ArtifactDir,
		"artifact-dir", disruptionOpt.ArtifactDir,
		"The directory where monitor events will be stored.")
	cmd.Flags().StringVar(&disruptionOpt.LoadBalancerType,
		"lb-type", disruptionOpt.LoadBalancerType,
		"Set load balancer type, available options: internal-lb, service-network, external-lb (default)")
	cmd.Flags().StringVar(&disruptionOpt.ExtraMessage,
		"extra-message", disruptionOpt.ExtraMessage,
		"Add custom label to disruption event message")
	return cmd
}

func (opt *RunAPIDisruptionMonitorOptions) Run() error {
	restConfig, err := monitor.GetMonitorRESTConfig()
	if err != nil {
		return err
	}

	lb := backend.ParseStringToLoadBalancerType(opt.LoadBalancerType)

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	abortCh := make(chan os.Signal, 2)
	go func() {
		<-abortCh
		fmt.Fprintf(opt.ErrOut, "Interrupted, terminating\n")
		// Give some time to store intervals on disk
		time.Sleep(5 * time.Second)
		cancelFn()
		sig := <-abortCh
		fmt.Fprintf(opt.ErrOut, "Interrupted twice, exiting (%s)\n", sig)
		switch sig {
		case syscall.SIGINT:
			os.Exit(130)
		default:
			os.Exit(0)
		}
	}()
	signal.Notify(abortCh, syscall.SIGINT, syscall.SIGTERM)

	recorder, err := StartAPIAvailability(ctx, restConfig, lb)
	if err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		var last time.Time
		done := false
		for !done {
			select {
			case <-ticker.C:
			case <-ctx.Done():
				done = true
			}
			events := recorder.Intervals(last, time.Time{})
			if len(events) > 0 {
				for _, event := range events {
					if !event.From.Equal(event.To) {
						continue
					}
					fmt.Fprintln(opt.Out, event.String())
				}
				last = events[len(events)-1].From
			}
		}
	}()

	<-ctx.Done()

	// Store intervals to artifact directory
	intervals := recorder.Intervals(time.Time{}, time.Time{})
	if len(opt.ExtraMessage) > 0 {
		fmt.Fprintf(opt.Out, "\nAppending %s to recorded event message\n", opt.ExtraMessage)
		for i, event := range intervals {
			intervals[i].Message = fmt.Sprintf("%s user-provided-message=%s", event.Message, opt.ExtraMessage)
		}
	}

	eventDir := filepath.Join(opt.ArtifactDir, monitorapi.EventDir)
	if err := os.MkdirAll(eventDir, os.ModePerm); err != nil {
		fmt.Printf("Failed to create monitor-events directory, err: %v\n", err)
		return err
	}

	timeSuffix := fmt.Sprintf("_%s", time.Now().UTC().Format("20060102-150405"))
	if err := monitorserialization.EventsToFile(filepath.Join(eventDir, fmt.Sprintf("e2e-events%s.json", timeSuffix)), intervals); err != nil {
		fmt.Printf("Failed to write event data, err: %v\n", err)
		return err
	}
	fmt.Fprintf(opt.Out, "\nEvent data written, exiting\n")

	return nil
}

// StartAPIAvailability monitors just the cluster availability
func StartAPIAvailability(ctx context.Context, restConfig *rest.Config, lb backend.LoadBalancerType) (monitorapi.Recorder, error) {
	recorder := monitor.NewRecorder()

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	if err := controlplane.StartAPIMonitoringUsingNewBackend(ctx, recorder, restConfig, lb); err != nil {
		return nil, err
	}

	// read the state of the cluster apiserver client access issues *before* any test (like upgrade) begins
	intervals, err := apiserveravailability.APIServerAvailabilityIntervalsFromCluster(client, time.Time{}, time.Time{})
	if err != nil {
		klog.Errorf("error reading initial apiserver availability: %v", err)
	}
	recorder.AddIntervals(intervals...)
	return recorder, nil
}

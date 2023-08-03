package ginkgo

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/openshift/origin/pkg/defaultinvariants"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/test/extended/util/disruption/controlplane"
	"github.com/openshift/origin/test/extended/util/disruption/externalservice"
	"github.com/openshift/origin/test/extended/util/disruption/frontends"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
)

type MonitorEventsOptions struct {
	monitor   *monitor.Monitor
	startTime *time.Time
	endTime   *time.Time

	genericclioptions.IOStreams
	storageDir                 string
	clusterStabilityDuringTest defaultinvariants.ClusterStabilityDuringTest
}

func NewMonitorEventsOptions(streams genericclioptions.IOStreams, storageDir string, clusterStabilityDuringTest defaultinvariants.ClusterStabilityDuringTest) *MonitorEventsOptions {
	return &MonitorEventsOptions{
		IOStreams:                  streams,
		storageDir:                 storageDir,
		clusterStabilityDuringTest: clusterStabilityDuringTest,
	}
}

func (o *MonitorEventsOptions) Start(ctx context.Context, restConfig *rest.Config) (monitorapi.Recorder, error) {
	if o.monitor != nil {
		return nil, fmt.Errorf("already started")
	}
	t := time.Now()
	o.startTime = &t

	m := monitor.NewMonitor(
		restConfig,
		o.storageDir,
		[]monitor.StartEventIntervalRecorderFunc{
			controlplane.StartAPIMonitoringUsingNewBackend,
			frontends.StartAllIngressMonitoring,
			externalservice.StartExternalServiceMonitoring,
		},
		defaultinvariants.NewInvariantsFor(o.clusterStabilityDuringTest),
	)
	err := m.Start(ctx)
	if err != nil {
		return nil, err
	}
	o.monitor = m

	return m, nil
}

func (o *MonitorEventsOptions) SetIOStreams(streams genericclioptions.IOStreams) {
	o.IOStreams = streams
}

// Stop mutates the method receiver so you shouldn't call it multiple times.
func (o *MonitorEventsOptions) Stop(ctx context.Context, restConfig *rest.Config, artifactDir string) error {
	if o.monitor == nil {
		return fmt.Errorf("not started")
	}
	if o.endTime != nil {
		return fmt.Errorf("already ended")
	}

	t := time.Now()
	o.endTime = &t

	cleanupContext, cleanupCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cleanupCancel()
	if err := o.monitor.Stop(cleanupContext); err != nil {
		fmt.Fprintf(os.Stderr, "error cleaning up, still reporting as best as possible: %v\n", err)
	}

	return nil
}

func (m *MonitorEventsOptions) SerializeResults(ctx context.Context, junitSuiteName, timeSuffix string) error {
	return m.monitor.SerializeResults(ctx, junitSuiteName, timeSuffix)
}

func (o *MonitorEventsOptions) GetStartTime() *time.Time {
	return o.startTime
}

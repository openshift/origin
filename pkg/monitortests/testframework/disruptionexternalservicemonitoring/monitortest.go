package disruptionexternalservicemonitoring

import (
	"context"
	_ "embed"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/disruptionlibrary"
	"github.com/openshift/origin/pkg/monitortestlibrary/utility"

	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

const (
	newConnectionTestName    = "[sig-trt] disruption/ci-cluster-network-liveness connection/new should be available throughout the test"
	reusedConnectionTestName = "[sig-trt] disruption/ci-cluster-network-liveness connection/reused should be available throughout the test"

	externalServiceURL = "http://static.redhat.com/test/rhel-networkmanager.txt"
)

type availability struct {
	disruptionChecker  *disruptionlibrary.Availability
	notSupportedReason error
	suppressJunit      bool
	tcpdumpHook        *backenddisruption.TcpdumpSamplerHook
}

func NewAvailabilityInvariant() monitortestframework.MonitorTest {
	return &availability{}
}

func NewRecordAvailabilityOnly() monitortestframework.MonitorTest {
	return &availability{
		suppressJunit: true,
	}
}

func (w *availability) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *availability) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	tcpdumpHook := utility.CreateTcpdumpHookIfEnabled()
	// Store reference to tcpdump hook for cleanup in CollectData
	w.tcpdumpHook = tcpdumpHook

	var samplerHooks []backenddisruption.SamplerHook
	if tcpdumpHook != nil {
		samplerHooks = append(samplerHooks, tcpdumpHook)
	}

	newConnectionDisruptionSampler := backenddisruption.NewSimpleBackendFromOpenshiftTests(
		externalServiceURL,
		"ci-cluster-network-liveness-new-connections",
		"",
		monitorapi.NewConnectionType).WithSamplerHooks(samplerHooks)

	reusedConnectionDisruptionSampler := backenddisruption.NewSimpleBackendFromOpenshiftTests(
		externalServiceURL,
		"ci-cluster-network-liveness-reused-connections",
		"",
		monitorapi.ReusedConnectionType).WithSamplerHooks(samplerHooks)

	w.disruptionChecker = disruptionlibrary.NewAvailabilityInvariant(
		newConnectionTestName, reusedConnectionTestName,
		newConnectionDisruptionSampler, reusedConnectionDisruptionSampler,
	)
	if err := w.disruptionChecker.StartCollection(ctx, adminRESTConfig, recorder); err != nil {
		return err
	}

	return nil
}

func (w *availability) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if w.notSupportedReason != nil {
		return nil, nil, w.notSupportedReason
	}

	// Stop tcpdump collection as the monitoring test is terminating
	utility.StopTcpdumpCollection(w.tcpdumpHook)

	return w.disruptionChecker.CollectData(ctx)
}

func (w *availability) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, w.notSupportedReason
}

func (w *availability) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	if w.suppressJunit {
		return nil, nil
	}

	return nil, w.notSupportedReason
}

func (w *availability) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	if w.notSupportedReason != nil {
		return w.notSupportedReason
	}

	// Move tcpdump pcap file to storage directory
	utility.MoveTcpdumpToStorage(w.tcpdumpHook, storageDir)

	return nil
}

func (w *availability) Cleanup(ctx context.Context) error {
	return w.notSupportedReason
}

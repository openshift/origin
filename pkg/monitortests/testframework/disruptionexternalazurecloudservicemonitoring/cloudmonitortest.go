package disruptionexternalazurecloudservicemonitoring

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/disruptionlibrary"
	"github.com/openshift/origin/pkg/monitortestlibrary/utility"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/sirupsen/logrus"

	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"k8s.io/client-go/kubernetes"
)

const (
	newCloudConnectionTestName    = "[sig-trt] disruption/azure-network-liveness connection/new should be available throughout the test"
	reusedCloudConnectionTestName = "[sig-trt] disruption/azure-network-liveness connection/reused should be available throughout the test"

	// Load balancer URL
	externalServiceURL = "http://20.127.186.25/health"
)

type cloudAvailability struct {
	disruptionChecker  *disruptionlibrary.Availability
	notSupportedReason error
	suppressJunit      bool
	tcpdumpHook        *backenddisruption.TcpdumpSamplerHook
}

func NewCloudAvailabilityInvariant() monitortestframework.MonitorTest {
	var notSupportedReason error

	return &cloudAvailability{
		suppressJunit:      true,
		notSupportedReason: notSupportedReason,
	}
}

func NewRecordCloudAvailabilityOnly() monitortestframework.MonitorTest {
	return &cloudAvailability{
		suppressJunit: true,
	}
}

func (w *cloudAvailability) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *cloudAvailability) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	{
		kubeClient, err := kubernetes.NewForConfig(adminRESTConfig)
		if err != nil {
			return err
		}
		isMicroShift, err := exutil.IsMicroShiftCluster(kubeClient)
		if err != nil {
			return fmt.Errorf("unable to determine if cluster is MicroShift: %v", err)
		}
		if isMicroShift {
			w.notSupportedReason = &monitortestframework.NotSupportedError{
				Reason: "platform MicroShift not supported",
			}
			return w.notSupportedReason
		}
	}
	// Proxy jobs may require a whitelist we don't want to deal with:
	clusterState, err := clusterdiscovery.DiscoverClusterState(adminRESTConfig)
	if err != nil {
		logrus.WithError(err).Error("error loading cluster state")
		return err
	}
	clusterConfig, err := clusterdiscovery.LoadConfig(clusterState)
	if err != nil {
		logrus.WithError(err).Error("error loading cluster config")
		return err
	}
	if clusterConfig.IsProxied {
		w.notSupportedReason = &monitortestframework.NotSupportedError{Reason: "azure-network-liveness disruption monitor is disabled when HTTP_PROXY is in use"}
		return w.notSupportedReason
	}

	tcpdumpHook := utility.CreateTcpdumpHookIfEnabled()
	// Store reference to tcpdump hook for cleanup in CollectData
	w.tcpdumpHook = tcpdumpHook

	var samplerHooks []backenddisruption.SamplerHook
	if tcpdumpHook != nil {
		samplerHooks = append(samplerHooks, tcpdumpHook)
	}

	newConnectionDisruptionSampler := backenddisruption.NewSimpleBackendFromOpenshiftTests(
		externalServiceURL,
		"azure-network-liveness-new-connections",
		"",
		monitorapi.NewConnectionType).WithSamplerHooks(samplerHooks)

	reusedConnectionDisruptionSampler := backenddisruption.NewSimpleBackendFromOpenshiftTests(
		externalServiceURL,
		"azure-network-liveness-reused-connections",
		"",
		monitorapi.ReusedConnectionType).WithSamplerHooks(samplerHooks)

	w.disruptionChecker = disruptionlibrary.NewAvailabilityInvariant(
		newCloudConnectionTestName, reusedCloudConnectionTestName,
		newConnectionDisruptionSampler, reusedConnectionDisruptionSampler,
	)
	if err := w.disruptionChecker.StartCollection(ctx, adminRESTConfig, recorder); err != nil {
		return err
	}

	return nil
}

func (w *cloudAvailability) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if w.notSupportedReason != nil {
		return nil, nil, w.notSupportedReason
	}

	// Stop tcpdump collection as the monitoring test is terminating
	utility.StopTcpdumpCollection(w.tcpdumpHook)

	return w.disruptionChecker.CollectData(ctx)
}

func (w *cloudAvailability) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, w.notSupportedReason
}

func (w *cloudAvailability) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	if w.suppressJunit {
		return nil, nil
	}

	return nil, w.notSupportedReason
}

func (w *cloudAvailability) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	if w.notSupportedReason != nil {
		return w.notSupportedReason
	}

	// Move tcpdump pcap file to storage directory
	utility.MoveTcpdumpToStorage(w.tcpdumpHook, storageDir)

	return nil
}

func (w *cloudAvailability) Cleanup(ctx context.Context) error {
	return w.notSupportedReason
}

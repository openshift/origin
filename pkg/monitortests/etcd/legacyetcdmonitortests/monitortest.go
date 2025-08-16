package legacyetcdmonitortests

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type legacyMonitorTests struct {
	adminRESTConfig    *rest.Config
	notSupportedReason error
	startTime          time.Time
}

func NewLegacyTests() monitortestframework.MonitorTest {
	return &legacyMonitorTests{}
}

func (w *legacyMonitorTests) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *legacyMonitorTests) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	w.startTime = time.Now()

	kubeClient, err := kubernetes.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return err
	}

	isMicroShift, err := exutil.IsMicroShiftCluster(kubeClient)
	if err != nil {
		return fmt.Errorf("unable to determine if cluster is MicroShift: %v", err)
	}
	if isMicroShift {
		// All tests in this monitor relate to either the etcd operator or the etcd pods
		// which MicroShift does not run. Etcd is embedded into MicroShift systemd unit
		// and runs in a separate binary as a host process.
		w.notSupportedReason = &monitortestframework.NotSupportedError{
			Reason: "platform MicroShift not supported",
		}
		return w.notSupportedReason
	}

	return nil
}

func (w *legacyMonitorTests) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, w.notSupportedReason
}

func (w *legacyMonitorTests) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, w.notSupportedReason
}

func (w *legacyMonitorTests) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	if w.notSupportedReason != nil {
		return nil, w.notSupportedReason
	}
	junits := []*junitapi.JUnitTestCase{}
	junits = append(junits, testRequiredInstallerResourcesMissing(finalIntervals)...)
	junits = append(junits, testEtcdShouldNotLogSlowFdataSyncs(finalIntervals)...)
	junits = append(junits, testEtcdShouldNotLogDroppedRaftMessages(finalIntervals)...)
	junits = append(junits, testOperatorStatusChanged(finalIntervals)...)
	junits = append(junits, testEtcdDoesNotLogExcessiveTookTooLongMessages(finalIntervals, w.startTime)...)
	junits = append(junits, testEtcdDoesNotLogExcessiveOverloadedNetworkMessages(finalIntervals, w.startTime)...)

	return junits, nil
}

func (*legacyMonitorTests) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*legacyMonitorTests) Cleanup(ctx context.Context) error {
	return nil
}

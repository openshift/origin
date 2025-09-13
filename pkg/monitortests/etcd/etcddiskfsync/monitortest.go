package etcddiskfsync

import (
	"context"
	"fmt"
	"time"

	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	utilmetrics "github.com/openshift/library-go/test/library/metrics"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortests/metrics"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/sirupsen/logrus"

	prometheustypes "github.com/prometheus/common/model"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	MonitorName = "etcd-disk-fsync"
)

// NewMonitorTest returns a monitor test that scrapes the client metrics
// ”etcd_disk_wal_fsync_duration_seconds_bucket" and generates intervals when
// we see unusual/elevated values.
//
// histogram_quantile(0.99, irate(etcd_disk_wal_fsync_duration_seconds_bucket{job="etcd"}[5m])) > 0.5
func NewMonitorTest() monitortestframework.MonitorTest {
	return &monitorTest{}
}

type etcdFsyncMonitor struct {
	query    metrics.QueryRunner
	analyzer metrics.SeriesAnalyzer
	callback *etcdFsyncCallback
}

type monitorTest struct {
	monitor            *etcdFsyncMonitor
	notSupportedReason error
}

func (test *monitorTest) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	kubeClient, err := kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}
	isMicroShift, err := exutil.IsMicroShiftCluster(kubeClient)
	if err != nil {
		return fmt.Errorf("unable to determine if cluster is MicroShift: %v", err)
	}
	if isMicroShift {
		test.notSupportedReason = &monitortestframework.NotSupportedError{
			Reason: "platform MicroShift not supported",
		}
		return test.notSupportedReason
	}
	routeClient, err := routeclient.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	client, err := utilmetrics.NewPrometheusClient(ctx, kubeClient, routeClient)
	if err != nil {
		return err
	}

	test.monitor = &etcdFsyncMonitor{
		query: &metrics.PrometheusQueryRunner{
			Client:      client,
			QueryString: `histogram_quantile(0.99, irate(etcd_disk_wal_fsync_duration_seconds_bucket{job="etcd"}[5m])) > 0.5`,
			Step:        15 * time.Second,
		},
		analyzer: metrics.RateSeriesAnalyzer{},
		callback: &etcdFsyncCallback{},
	}

	logrus.Infof("monitor[%s]: monitor initialized", MonitorName)
	return nil
}

func (test *monitorTest) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if test.notSupportedReason != nil {
		return nil, nil, test.notSupportedReason
	}

	m := test.monitor
	if m == nil {
		return monitorapi.Intervals{}, nil, fmt.Errorf("monitor test is not initialized")
	}

	if err := m.analyzer.Analyze(ctx, m.query, beginning, end, m.callback); err != nil {
		return monitorapi.Intervals{}, nil, err
	}
	return m.callback.intervals, nil, nil
}

func (test *monitorTest) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, test.notSupportedReason
}

func (test *monitorTest) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, test.notSupportedReason
}

func (test *monitorTest) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return test.notSupportedReason
}

func (test *monitorTest) Cleanup(ctx context.Context) error {
	return test.notSupportedReason
}

type etcdFsyncCallback struct {
	locator   monitorapi.Locator
	intervals monitorapi.Intervals
}

func (b *etcdFsyncCallback) Name() string { return MonitorName }

func (b *etcdFsyncCallback) StartSeries(metric prometheustypes.Metric) {
	b.locator = monitorapi.NewLocator().WithEtcdDiskFsyncMetric(metric)
}

func (b *etcdFsyncCallback) EndSeries() { b.locator = monitorapi.Locator{} }

func (b *etcdFsyncCallback) NewInterval(metric prometheustypes.Metric, start, end *prometheustypes.SamplePair) {
	startTime := start.Timestamp.Time()
	endTime := end.Timestamp.Time()

	interval := monitorapi.NewInterval(monitorapi.SourceEtcdDiskFsyncMonitor, monitorapi.Error).
		Locator(b.locator).
		Message(monitorapi.NewMessage().
			HumanMessage("high etcd_disk_wal_fsync_duration_seconds observed for pod").
			Reason(monitorapi.EtcdHighDiskFsyncReason)).
		Display().
		Build(startTime, endTime)
	b.intervals = append(b.intervals, interval)
}

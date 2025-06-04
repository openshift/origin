package apiunreachablefromclientmetrics

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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"

	prometheustypes "github.com/prometheus/common/model"
)

const (
	MonitorName = "api-unreachable-from-client-metrics"
)

// NewMonitorTest returns a monitor test that scrapes the client metrics
// 'rest_client_requests_total' and generates api unreachable intervals from the
// perspectives of a client, it runs the following query:
//
//	sum(rate(rest_client_requests_total{code="<error>"}[1m])) by(host)
//
// The query returns rate of network error seen over time, grouped by host.
// The 'host' refers to the URL used to contact the kube-apiserver, typically these are:
// a) external load balancer
// b) internal load balancer
// c) service network
// d) localhost
//
// What components generate these metrics?
// Any component that uses client-go and component-base to talk to the
// kube-apiserver will generate these metrics:
//
// a) kubelet, kcm, scheduler generate these metrics using the internal load balancer
// b) some of the operators we have use client-go/component-base, these operators
// generate these metrics over service network
// c) kube-apiserver uses local loopback to talk to itself
//
// How do we interpret the interval?
// The intervals are scraped from metrics, so they don't have the same granularity
// as other intervals, since:
// a) in OpenShift, metrics are scraped every 30s
// b) for rate to be calculated, we need at lease two samples
// Given these constraints, the minimum duration of an interval is at least 1m.
//
// If an api unreachable interval overlaps with an apiserver shutdown window,
// it is typically indicative of network issues at the load balancer layer.
// Since the intervals are grouped by host, we can also narrow it down to a
// particular host, for example, we have seen cases where connections over
// internal load balancer to be faulty at times while the service network
// operated just fine.
// This monitor test can shed some lights in these situations.
func NewMonitorTest() monitortestframework.MonitorTest {
	return &monitorTest{}
}

type apiUnreachableMonitor struct {
	query    metrics.QueryRunner
	analyzer metrics.SeriesAnalyzer
	callback *apiUnreachableCallback
}

type monitorTest struct {
	monitor            *apiUnreachableMonitor
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

	kubeSvc, err := kubeClient.CoreV1().Services(metav1.NamespaceDefault).Get(ctx, "kubernetes", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to retrieve cluster IP from kubernetes.default.svc - %v", err)
	}

	test.monitor = &apiUnreachableMonitor{
		query: &metrics.PrometheusQueryRunner{
			Client:      client,
			QueryString: `sum(rate(rest_client_requests_total{code="<error>"}[1m])) by(host)`,
			Step:        time.Minute,
		},
		analyzer: metrics.RateSeriesAnalyzer{},
		callback: &apiUnreachableCallback{
			serviceNetworkIP: kubeSvc.Spec.ClusterIP,
		},
	}

	framework.Logf("monitor[%s]: monitor initialized, service-network-ip: %s", MonitorName, kubeSvc.Spec.ClusterIP)
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

// callback passed to the metric analyzer so we can construct the api unreachable intervals
type apiUnreachableCallback struct {
	serviceNetworkIP string
	locator          monitorapi.Locator
	intervals        monitorapi.Intervals
}

func (b *apiUnreachableCallback) Name() string { return MonitorName }
func (b *apiUnreachableCallback) StartSeries(metric prometheustypes.Metric) {
	b.locator = monitorapi.NewLocator().WithAPIUnreachableFromClient(metric, b.serviceNetworkIP)
}
func (b *apiUnreachableCallback) EndSeries() { b.locator = monitorapi.Locator{} }

func (b *apiUnreachableCallback) NewInterval(metric prometheustypes.Metric, start, end *prometheustypes.SamplePair) {
	startTime := start.Timestamp.Time()
	endTime := end.Timestamp.Time()
	if start == end {
		// an api unreachable interval with one sample
		// TODO: approximate the interval to [t-30s ... t+30s] for now
		startTime = start.Timestamp.Time().Add(-30 * time.Second)
		endTime = end.Timestamp.Time().Add(30 * time.Second)
	}

	interval := monitorapi.NewInterval(monitorapi.SourceAPIUnreachableFromClient, monitorapi.Error).
		Locator(b.locator).
		Message(monitorapi.NewMessage().
			HumanMessage(fmt.Sprintf("client observed API error(s), host: %s, duration: %s", string(metric["host"]), endTime.Sub(startTime))).
			Reason(monitorapi.APIUnreachableFromClientMetrics)).
		Display().
		Build(startTime, endTime)
	b.intervals = append(b.intervals, interval)
}

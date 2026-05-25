package leaderchanges

import (
	"context"
	"fmt"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/library-go/test/library/metrics"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/allowedalerts"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const testName = "[sig-etcd] etcd leader changes are not excessive"

type leaderChangesTest struct {
	adminRESTConfig    *rest.Config
	notSupportedReason error
	startTime          time.Time
}

func NewLeaderChangesTest() monitortestframework.MonitorTest {
	return &leaderChangesTest{}
}

func (w *leaderChangesTest) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *leaderChangesTest) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
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
		w.notSupportedReason = &monitortestframework.NotSupportedError{
			Reason: "platform MicroShift not supported",
		}
		return w.notSupportedReason
	}

	return nil
}

func (w *leaderChangesTest) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if w.notSupportedReason != nil {
		return nil, nil, w.notSupportedReason
	}

	kubeClient, err := kubernetes.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating kube client: %v", err)
	}

	configClient, err := configv1client.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating config client: %v", err)
	}

	infra, err := configClient.ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("error getting infrastructure: %v", err)
	}

	etcdNamespace := "openshift-etcd"
	if infra.Status.ControlPlaneTopology == configv1.ExternalTopologyMode {
		etcdNamespace = "clusters-.*"
	}

	routeClient, err := routeclient.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating route client: %v", err)
	}

	prometheusClient, err := metrics.NewPrometheusClient(ctx, kubeClient, routeClient)
	if err != nil {
		return nil, nil, fmt.Errorf("error creating prometheus client: %v", err)
	}

	testDuration := fmt.Sprintf("%ds", int(time.Since(w.startTime).Seconds()))
	query := fmt.Sprintf(`max(max by (pod,job) (increase(etcd_server_leader_changes_seen_total{namespace=~"%s"}[%s])))`, etcdNamespace, testDuration)

	result, _, err := prometheusClient.Query(ctx, query, time.Now())
	if err != nil {
		return nil, nil, fmt.Errorf("prometheus query failed: %v", err)
	}

	vec, ok := result.(model.Vector)
	if !ok {
		return nil, failJunit(fmt.Sprintf("expected Prometheus query to return a vector, got %s instead", result.Type())), nil
	}

	if len(vec) == 0 {
		return nil, failJunit("expected Prometheus query to return at least one item, got 0 instead"), nil
	}

	numberOfRevisions, err := allowedalerts.GetEstimatedNumberOfRevisionsForEtcdOperator(ctx, kubeClient, time.Since(w.startTime))
	if err != nil {
		return nil, nil, fmt.Errorf("error estimating etcd operator revisions: %v", err)
	}

	allowedLeaderChanges := numberOfRevisions * 3
	leaderChanges := vec[0].Value

	if int(leaderChanges) > allowedLeaderChanges {
		return nil, failJunit(fmt.Sprintf(
			"observed %s leader changes (expected at most %d) in %s: Leader changes are a result of stopping the etcd leader process or from latency (disk or network), review etcd performance metrics",
			leaderChanges, allowedLeaderChanges, testDuration,
		)), nil
	}

	return nil, []*junitapi.JUnitTestCase{{Name: testName}}, nil
}

func (w *leaderChangesTest) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, w.notSupportedReason
}

func (w *leaderChangesTest) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, w.notSupportedReason
}

func (w *leaderChangesTest) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*leaderChangesTest) Cleanup(ctx context.Context) error {
	return nil
}

func failJunit(message string) []*junitapi.JUnitTestCase {
	return []*junitapi.JUnitTestCase{
		{
			Name: testName,
			FailureOutput: &junitapi.FailureOutput{
				Output: message,
			},
		},
	}
}

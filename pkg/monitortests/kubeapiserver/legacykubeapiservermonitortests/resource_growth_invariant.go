package legacykubeapiservermonitortests

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	routeclient "github.com/openshift/client-go/route/clientset/versioned"

	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	v1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/library-go/test/library/metrics"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	helper "github.com/openshift/origin/test/extended/util/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	// allowedResourceGrowth is the multiplier we'll allow before failing the test. (currently 40%)
	allowedResourceGrowth = 1.4

	secretTestName    = "[sig-trt] Secret count should not have grown significantly during upgrade"
	configMapTestName = "[sig-trt] ConfigMap count should not have grown significantly during upgrade"
)

type resourceGrowthTests struct {
	adminRESTConfig *rest.Config
	// PreUpgradeResourceCounts stores a map of resource type to a count of the number of
	// resources of that type in the entire cluster, gathered prior to launching the upgrade.
	preUpgradeResourceCounts map[string]int
}

func NewResourceGrowthTests() monitortestframework.MonitorTest {
	return &resourceGrowthTests{
		preUpgradeResourceCounts: map[string]int{},
	}
}

func (w *resourceGrowthTests) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig

	kubeClient, err := kubernetes.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return err
	}
	// Store resource counts we're interested in monitoring from before upgrade to after.
	// Used to test for excessive resource growth during upgrade in the invariants.
	secrets, err := kubeClient.CoreV1().Secrets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	w.preUpgradeResourceCounts["secrets"] = len(secrets.Items)

	configMaps, err := kubeClient.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	w.preUpgradeResourceCounts["configmaps"] = len(configMaps.Items)
	return nil
}

func (w *resourceGrowthTests) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *resourceGrowthTests) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (*resourceGrowthTests) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *resourceGrowthTests) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	junits := []*junitapi.JUnitTestCase{}
	errs := []error{}

	localJunits, err := w.comparePostUpgradeResourceCountFromMetrics(ctx, secretTestName, "secrets")
	errs = append(errs, err)
	junits = append(junits, localJunits...)

	localJunits, err = w.comparePostUpgradeResourceCountFromMetrics(ctx, configMapTestName, "configmaps")
	errs = append(errs, err)
	junits = append(junits, localJunits...)

	return junits, utilerrors.NewAggregate(errs)
}

func (*resourceGrowthTests) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*resourceGrowthTests) Cleanup(ctx context.Context) error {
	return nil
}

// comparePostUpgradeResourceCountFromMetrics tests that some counts for certain resources we're most interested
// in potentially leaking do not increase substantially during upgrade.
// This is in response to a bug discovered where operators were leaking Secrets and ultimately taking down clusters.
// The counts have to be recorded before upgrade which is done in test/e2e/upgrade/upgrade.go, stored in a package
// variable, and then read here in invariant. This is to work around the problems where our normal ginko tests are
// all run in separate processes by themselves.
//
// Values for comparison are a guess at what would have caught the leak we saw. (growth of about 60% during upgrade)
//
// resource should be all lowercase and plural such as "secrets".
func (w *resourceGrowthTests) comparePostUpgradeResourceCountFromMetrics(ctx context.Context, testName, resource string) ([]*junitapi.JUnitTestCase, error) {
	preUpgradeCount := w.preUpgradeResourceCounts[resource]

	// if the check on clusterversion returns a junit, we are done. most likely there was a problem
	// getting the cv or possibly the upgrade completion time was null because the rollback is still in
	// progress and the test has given up.
	cv, err := w.getAndCheckClusterVersion()
	if err != nil {
		return nil, err
	}
	upgradeCompletion := cv.Status.History[0].CompletionTime

	prometheusClient, err := w.newPrometheusClient(ctx)
	if err != nil {
		return nil, err
	}

	// Use prometheus to get the resource count at the moment we recorded upgrade complete. We can't do this
	// for the starting count as prometheus metrics seem to get wiped during the upgrade. We also don't want to
	// just list the resources right now, as we don't know what other tests might have created since.
	e2e.Logf("querying metrics at: %s", upgradeCompletion.Time.UTC().Format(time.RFC3339))
	resourceCountPromQuery := fmt.Sprintf(`cluster:usage:resources:sum{resource="%s"}`, resource)
	promResultsCompletion, err := helper.RunQueryAtTime(ctx, prometheusClient,
		resourceCountPromQuery, upgradeCompletion.Time)
	if err != nil {
		return []*junitapi.JUnitTestCase{
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: "Error getting resource count from Prometheus at upgrade completion time: " + err.Error(),
				},
			},
		}, nil

	}

	if len(promResultsCompletion.Data.Result) == 0 {
		return []*junitapi.JUnitTestCase{
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: "Post-upgrade resource count metric data has no Result",
				},
			},
		}, nil
	}
	completedCount := int(promResultsCompletion.Data.Result[0].Value)

	// Ensure that a resource count did not grow more than allowed:
	maxAllowedCount := int(float64(preUpgradeCount) * allowedResourceGrowth)
	output := fmt.Sprintf("%s count grew from %d to %d during upgrade (max allowed=%d). This test is experimental and may need adjusting in some cases.",
		resource, preUpgradeCount, completedCount, maxAllowedCount)

	if completedCount > maxAllowedCount {
		return []*junitapi.JUnitTestCase{
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: output,
				},
			},
		}, nil
	}
	return []*junitapi.JUnitTestCase{
		{Name: testName},
	}, nil

}

func (w *resourceGrowthTests) newPrometheusClient(ctx context.Context) (prometheusv1.API, error) {
	kubeClient, err := kubernetes.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return nil, err
	}
	routeClient, err := routeclient.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return nil, err
	}
	prometheusClient, err := metrics.NewPrometheusClient(ctx, kubeClient, routeClient)
	if err != nil {
		return nil, err
	}
	return prometheusClient, nil
}

func (w *resourceGrowthTests) getAndCheckClusterVersion() (*v1.ClusterVersion, error) {
	configClient, err := configclient.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return nil, err
	}

	cv, err := configClient.ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})

	if err != nil {
		return nil, fmt.Errorf("error getting ClusterVersion: %w", err)
	}
	if len(cv.Status.History) == 0 {
		return nil, fmt.Errorf("clusterVersion.Status has no History")
	}

	// In the case that cv completionTime is nil meaning the version change (upgrade or
	// rollback is still in progress), flake this case. The problem is likely a bigger
	// problem and the job will fail for that. Returning a failure and fake success, so it
	// will be marked as a flake.
	if cv.Status.History[0].CompletionTime == nil {
		return nil, fmt.Errorf("clusterVersion.completionTime is nil")
	}

	return cv, nil
}

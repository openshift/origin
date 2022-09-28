package synthetictests

import (
	"context"
	"fmt"
	v1 "github.com/openshift/api/config/v1"
	"time"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/openshift/origin/test/e2e/upgrade"
	exutil "github.com/openshift/origin/test/extended/util"
	helper "github.com/openshift/origin/test/extended/util/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	// allowedResourceGrowth is the multiplier we'll allow before failing the test. (currently 40%)
	allowedResourceGrowth = 1.4
)

func testNoExcessiveSecretGrowthDuringUpgrade() []*junitapi.JUnitTestCase {
	const testName = "[sig-trt] Secret count should not have grown significantly during upgrade"
	return comparePostUpgradeResourceCountFromMetrics(testName, "secrets")

}

func testNoExcessiveConfigMapGrowthDuringUpgrade() []*junitapi.JUnitTestCase {
	const testName = "[sig-trt] ConfigMap count should not have grown significantly during upgrade"
	return comparePostUpgradeResourceCountFromMetrics(testName, "configmaps")

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
func comparePostUpgradeResourceCountFromMetrics(testName, resource string) []*junitapi.JUnitTestCase {
	oc := exutil.NewCLI("resource-growth-test")
	ctx := context.Background()

	preUpgradeCount := upgrade.PreUpgradeResourceCounts[resource]
	e2e.Logf("found %d %s prior to upgrade", preUpgradeCount, resource)

	// if the check on clusterversion returns a junit, we are done. most likely there was a problem
	// getting the cv or possibly the upgrade completion time was null because the rollback is still in
	// progress and the test has given up.
	cv, junit := getAndCheckClusterVersion(testName, oc)
	if junit != nil {
		return junit
	}
	upgradeCompletion := cv.Status.History[0].CompletionTime

	// Use prometheus to get the resource count at the moment we recorded upgrade complete. We can't do this
	// for the starting count as prometheus metrics seem to get wiped during the upgrade. We also don't want to
	// just list the resources right now, as we don't know what other tests might have created since.
	e2e.Logf("querying metrics at: %s", upgradeCompletion.Time.UTC().Format(time.RFC3339))
	resourceCountPromQuery := fmt.Sprintf(`cluster:usage:resources:sum{resource="%s"}`, resource)
	promResultsCompletion, err := helper.RunQueryAtTime(ctx, oc.NewPrometheusClient(ctx),
		resourceCountPromQuery, upgradeCompletion.Time)
	if err != nil {
		return []*junitapi.JUnitTestCase{
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: "Error getting resource count from Prometheus at upgrade completion time: " + err.Error(),
				},
			},
		}

	}
	e2e.Logf("got %d metrics after upgrade", len(promResultsCompletion.Data.Result))
	if len(promResultsCompletion.Data.Result) == 0 {
		return []*junitapi.JUnitTestCase{
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: "Post-upgrade resource count metric data has no Result",
				},
			},
		}
	}
	completedCount := int(promResultsCompletion.Data.Result[0].Value)
	e2e.Logf("found %d %s after upgrade", completedCount, resource)

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
		}
	}
	return []*junitapi.JUnitTestCase{
		{Name: testName},
	}

}

func getAndCheckClusterVersion(testName string, oc *exutil.CLI) (*v1.ClusterVersion, []*junitapi.JUnitTestCase) {
	cv, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(context.Background(), "version",
		metav1.GetOptions{})

	if err != nil {
		return cv, []*junitapi.JUnitTestCase{
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: "Error getting ClusterVersion: " + err.Error(),
				},
			},
		}

	}
	if len(cv.Status.History) == 0 {
		return cv, []*junitapi.JUnitTestCase{
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: "ClusterVersion.Status has no History",
				},
			},
		}
	}

	// In the case that cv completionTime is nil meaning the version change (upgrade or
	// rollback is still in progress), flake this case. The problem is likely a bigger
	// problem and the job will fail for that. Returning a failure and fake success, so it
	// will be marked as a flake.
	if cv.Status.History[0].CompletionTime == nil {
		return cv, []*junitapi.JUnitTestCase{
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: "ClusterVersion.completionTime is nil",
				},
			},
			{
				Name: testName,
			},
		}
	}

	return cv, nil
}

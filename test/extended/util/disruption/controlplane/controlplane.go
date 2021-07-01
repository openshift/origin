package controlplane

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/rest"

	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"

	"github.com/blang/semver"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/disruption"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewKubeAvailableWithNewConnectionsTest tests that the Kubernetes control plane remains available during and after a cluster upgrade.
func NewKubeAvailableWithNewConnectionsTest() upgrades.Test {
	return &availableTest{
		testName:        "[sig-api-machinery] Kubernetes APIs remain available for new connections",
		name:            "kubernetes-api-available-new-connections",
		startMonitoring: monitor.StartKubeAPIMonitoringWithNewConnections,
	}
}

// NewOpenShiftAvailableNewConnectionsTest tests that the OpenShift APIs remains available during and after a cluster upgrade.
func NewOpenShiftAvailableNewConnectionsTest() upgrades.Test {
	return &availableTest{
		testName:        "[sig-api-machinery] OpenShift APIs remain available for new connections",
		name:            "openshift-api-available-new-connections",
		startMonitoring: monitor.StartOpenShiftAPIMonitoringWithNewConnections,
	}
}

// NewOAuthAvailableNewConnectionsTest tests that the OAuth APIs remains available during and after a cluster upgrade.
func NewOAuthAvailableNewConnectionsTest() upgrades.Test {
	return &availableTest{
		testName:        "[sig-api-machinery] OAuth APIs remain available for new connections",
		name:            "oauth-api-available-new-connections",
		startMonitoring: monitor.StartOAuthAPIMonitoringWithNewConnections,
	}
}

// NewKubeAvailableWithConnectionReuseTest tests that the Kubernetes control plane remains available during and after a cluster upgrade.
func NewKubeAvailableWithConnectionReuseTest() upgrades.Test {
	return &availableTest{
		testName:        "[sig-api-machinery] Kubernetes APIs remain available with reused connections",
		name:            "kubernetes-api-available-reused-connections",
		startMonitoring: monitor.StartKubeAPIMonitoringWithConnectionReuse,
	}
}

// NewOpenShiftAvailableTest tests that the OpenShift APIs remains available during and after a cluster upgrade.
func NewOpenShiftAvailableWithConnectionReuseTest() upgrades.Test {
	return &availableTest{
		testName:        "[sig-api-machinery] OpenShift APIs remain available with reused connections",
		name:            "openshift-api-available-reused-connections",
		startMonitoring: monitor.StartOpenShiftAPIMonitoringWithConnectionReuse,
	}
}

// NewOauthAvailableTest tests that the OAuth APIs remains available during and after a cluster upgrade.
func NewOAuthAvailableWithConnectionReuseTest() upgrades.Test {
	return &availableTest{
		testName:        "[sig-api-machinery] OAuth APIs remain available with reused connections",
		name:            "oauth-api-available-reused-connections",
		startMonitoring: monitor.StartOAuthAPIMonitoringWithConnectionReuse,
	}
}

type availableTest struct {
	// testName is the name to show in unit
	testName string
	// name helps distinguish which API server in particular is unavailable.
	name            string
	startMonitoring starter
}

type starter func(ctx context.Context, m *monitor.Monitor, clusterConfig *rest.Config, timeout time.Duration) error

func (t availableTest) Name() string { return t.name }
func (t availableTest) DisplayName() string {
	return t.testName
}

// Setup does nothing
func (t *availableTest) Setup(f *framework.Framework) {
}

// Test runs a connectivity check to the core APIs.
func (t *availableTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	config, err := framework.LoadConfig()
	framework.ExpectNoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	m := monitor.NewMonitorWithInterval(time.Second)
	err = t.startMonitoring(ctx, m, config, 15*time.Second)
	framework.ExpectNoError(err, "unable to monitor API")

	start := time.Now()
	m.StartSampling(ctx)

	// wait to ensure API is still up after the test ends
	<-done
	time.Sleep(15 * time.Second)
	cancel()
	end := time.Now()

	// starting from 4.8, enforce the requirement that control plane remains available
	hasAllFixes, err := util.AllClusterVersionsAreGTE(semver.Version{Major: 4, Minor: 8}, config)
	if err != nil {
		framework.Logf("Cannot require full control plane availability, some versions could not be checked: %v", err)
	}

	toleratedDisruption := 0.08
	switch {
	case framework.ProviderIs("azure"), framework.ProviderIs("aws"), framework.ProviderIs("gce"):
		if hasAllFixes {
			framework.Logf("Cluster contains no versions older than 4.8, tolerating no disruption")
			toleratedDisruption = 0
		}
	}
	disruption.ExpectNoDisruption(f, toleratedDisruption, end.Sub(start), m.Intervals(time.Time{}, time.Time{}), fmt.Sprintf("API %q was unreachable during disruption (AWS has a known issue: https://bugzilla.redhat.com/show_bug.cgi?id=1943804)", t.name))
}

// Teardown cleans up any remaining resources.
func (t *availableTest) Teardown(f *framework.Framework) {
}

// allClusterVersionsAreGTE returns true if all historical versions on the cluster version are
// at or newer than the provided semver, ignoring prerelease status. If no versions are found
// an error is returned.
func allClusterVersionsAreGTE(version semver.Version, config *rest.Config) (bool, error) {
	c, err := configv1client.NewForConfig(config)
	if err != nil {
		return false, err
	}
	cv, err := c.ConfigV1().ClusterVersions().Get(context.TODO(), "version", metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	if len(cv.Status.History) == 0 {
		return false, fmt.Errorf("no versions in cluster version history")
	}
	for _, v := range cv.Status.History {
		ver, err := semver.Parse(v.Version)
		if err != nil {
			return false, err
		}

		// ignore prerelease version matching here
		ver.Pre = nil
		ver.Build = nil

		if ver.LT(version) {
			return false, nil
		}
	}
	return true, nil
}

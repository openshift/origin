package controlplane

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/rest"

	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/test/extended/util/disruption"
)

// NewKubeAvailableTest tests that the Kubernetes control plane remains available during and after a cluster upgrade.
func NewKubeAvailableTest() upgrades.Test {
	return &kubeAvailableTest{availableTest{name: "kubernetes-api-available"}}
}

type kubeAvailableTest struct {
	availableTest
}

func (t kubeAvailableTest) Name() string { return t.availableTest.name }
func (kubeAvailableTest) DisplayName() string {
	return "[sig-api-machinery] Kubernetes APIs remain available"
}
func (t *kubeAvailableTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	t.availableTest.test(f, done, upgrade, monitor.StartKubeAPIMonitoring)
}

// NewOpenShiftAvailableTest tests that the OpenShift APIs remains available during and after a cluster upgrade.
func NewOpenShiftAvailableTest() upgrades.Test {
	return &openShiftAvailableTest{availableTest{name: "openshift-api-available"}}
}

type openShiftAvailableTest struct {
	availableTest
}

func (t openShiftAvailableTest) Name() string { return t.availableTest.name }
func (openShiftAvailableTest) DisplayName() string {
	return "[sig-api-machinery] OpenShift APIs remain available"
}
func (t *openShiftAvailableTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	t.availableTest.test(f, done, upgrade, monitor.StartOpenShiftAPIMonitoring)
}

// NewOauthAvailableTest tests that the OAuth APIs remains available during and after a cluster upgrade.
func NewOAuthAvailableTest() upgrades.Test {
	return &oauthAvailableTest{availableTest{name: "oauth-api-available"}}
}

// oauthAvailableTest tests that the OAuth APIs remains available during and after a cluster upgrade.
type oauthAvailableTest struct {
	availableTest
}

func (t oauthAvailableTest) Name() string { return t.availableTest.name }
func (oauthAvailableTest) DisplayName() string {
	return "[sig-api-machinery] OAuth APIs remain available"
}
func (t *oauthAvailableTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	t.availableTest.test(f, done, upgrade, monitor.StartOAuthAPIMonitoring)
}

type availableTest struct {
	// name helps distinguish which API server in particular is unavailable.
	name string
}

type starter func(ctx context.Context, m *monitor.Monitor, clusterConfig *rest.Config, timeout time.Duration) error

// Setup does nothing
func (t *availableTest) Setup(f *framework.Framework) {
}

// Test runs a connectivity check to the core APIs.
func (t *availableTest) test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType, startMonitoring starter) {
	config, err := framework.LoadConfig()
	framework.ExpectNoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	m := monitor.NewMonitorWithInterval(time.Second)
	err = startMonitoring(ctx, m, config, 15*time.Second)
	framework.ExpectNoError(err, "unable to monitor API")

	start := time.Now()
	m.StartSampling(ctx)

	// wait to ensure API is still up after the test ends
	<-done
	time.Sleep(15 * time.Second)
	cancel()
	end := time.Now()

	disruption.ExpectNoDisruption(f, 0.08, end.Sub(start), m.Events(time.Time{}, time.Time{}), fmt.Sprintf("API %q was unreachable during disruption", t.name))
}

// Teardown cleans up any remaining resources.
func (t *availableTest) Teardown(f *framework.Framework) {
}

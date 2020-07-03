package alert

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"

	exutil "github.com/openshift/origin/test/extended/util"
	helper "github.com/openshift/origin/test/extended/util/prometheus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

const (
	// Delay after upgrade is complete before checking for critical alerts
	alertCheckSleepMinutes = 5
	alertCheckSleep        = alertCheckSleepMinutes * time.Minute

	// Previous period in which to check for critical alerts
	alertPeriodCheckMinutes = 1
)

// UpgradeTest runs post-upgrade after alertCheckSleep delay and tests if any critical alerts are firing.
type UpgradeTest struct {
	url         string
	bearerToken string
	oc          *exutil.CLI
}

func (UpgradeTest) Name() string { return "check-for-critical-alerts" }
func (UpgradeTest) DisplayName() string {
	return "Check if critical alerts are firing after upgrade success"
}

// Setup creates parameters to query Prometheus
func (t *UpgradeTest) Setup(f *framework.Framework) {
	g.By("Setting up post-upgrade alert test")

	url, bearerToken, oc, ok := helper.ExpectPrometheus(f)
	if !ok {
		framework.Failf("Prometheus could not be located on this cluster, failing test %s", t.Name())
	}
	t.url = url
	t.bearerToken = bearerToken
	t.oc = oc
	framework.Logf("Post-upgrade alert test setup complete")
}

// Test checks if any critical alerts are firing.
func (t *UpgradeTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	g.By("Checking for critical alerts")

	// Recover current test if it fails so test suite can complete
	defer g.GinkgoRecover()

	// Block until upgrade is done
	g.By("Waiting for upgrade to finish before checking for critical alerts")
	<-done

	_, cancel := context.WithCancel(context.Background())

	// Additonal delay after upgrade completion
	g.By("Waiting before checking for critical alerts")
	time.Sleep(alertCheckSleep)
	cancel()

	if helper.TestUnsupportedAllowVersionSkew() {
		framework.Skipf("Test is disabled to allow cluster components to have different versions, and skewed versions trigger multiple other alerts")
	}
	t.oc.SetupProject()
	ns := t.oc.Namespace()
	execPod := exutil.CreateCentosExecPodOrFail(t.oc.AdminKubeClient(), ns, "execpod", nil)
	defer func() {
		t.oc.AdminKubeClient().CoreV1().Pods(ns).Delete(execPod.Name, metav1.NewDeleteOptions(1))
	}()

	// Query to check if Prometheus has been up and running for entire post-upgrade
	// period by verifying Watchdog alert has been in firing state
	watchdogQuery := fmt.Sprintf(`count_over_time(ALERTS{alertstate="firing",alertname="Watchdog", severity="none"}[%dm])`, alertCheckSleepMinutes)

	// Query to check for any critical severity alerts that have occurred within the last alertPeriodCheckMinutes.
	criticalAlertQuery := fmt.Sprintf(`count_over_time(ALERTS{alertname!~"Watchdog|AlertmanagerReceiversNotConfigured|KubeAPILatencyHigh",alertstate="firing",severity="critical"}[%dm]) >= 1`, alertPeriodCheckMinutes)

	tests := map[string]bool{
		watchdogQuery:      true,
		criticalAlertQuery: false,
	}

	helper.RunQueries(tests, t.oc, ns, execPod.Name, t.url, t.bearerToken)

	framework.Logf("No crtical alerts firing post-upgrade")
}

// Teardown cleans up any remaining resources.
func (t *UpgradeTest) Teardown(f *framework.Framework) {
	// rely on the namespace deletion to clean up everything
}

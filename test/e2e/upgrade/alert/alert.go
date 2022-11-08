package alert

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/alerts"
	exutil "github.com/openshift/origin/test/extended/util"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

// UpgradeTest runs verifies invariants regarding what alerts are allowed to fire
// during the upgrade process.
type UpgradeTest struct {
	oc               *exutil.CLI
	prometheusClient prometheusv1.API
	configClient     configclient.Interface
}

func (UpgradeTest) Name() string { return "check-for-alerts" }
func (UpgradeTest) DisplayName() string {
	return "[sig-arch] Check if alerts are firing during or after upgrade success"
}

// Setup creates parameters to query Prometheus
func (t *UpgradeTest) Setup(f *framework.Framework) {
	g.By("Setting up upgrade alert test")

	oc := exutil.NewCLIWithFramework(f)
	t.oc = oc
	t.prometheusClient = oc.NewPrometheusClient(context.TODO())
	t.configClient = oc.AdminConfigClient()
	framework.Logf("Post-upgrade alert test setup complete")
}

// Test checks if alerts are firing at various points during upgrade.
// An alert firing during an upgrade is a high severity bug - it either points to a real issue in
// a dependency, or a failure of the component, and therefore must be fixed.
func (t *UpgradeTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	g.By("Checking for alerts")
	startTime := time.Now()

	// Block until upgrade is done
	g.By("Waiting for upgrade to finish before checking for alerts")
	<-done

	// Additonal delay after upgrade completion to allow pending alerts to settle
	g.By("Waiting before checking for alerts")
	time.Sleep(1 * time.Minute)

	testDuration := time.Now().Sub(startTime).Round(time.Second)

	alerts.CheckAlerts(alerts.AllowedAlertsDuringUpgrade, t.prometheusClient, t.configClient, testDuration, f)
}

// Teardown cleans up any remaining resources.
func (t *UpgradeTest) Teardown(f *framework.Framework) {
	// rely on the namespace deletion to clean up everything
}

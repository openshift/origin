package allowedalerts

import (
	"context"
	"fmt"
	"strings"

	exutil "github.com/openshift/origin/test/extended/util"

	o "github.com/onsi/gomega"
	helper "github.com/openshift/origin/test/extended/util/prometheus"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
)

type watchdogAlertTest struct {
}

func newWatchdogAlert() *watchdogAlertTest {
	return &watchdogAlertTest{}
}

func (a *watchdogAlertTest) toTest() AlertTest {
	return a
}

func (a *watchdogAlertTest) TestNamePrefix() string {
	return "[bz-monitoring][Late] Alerts"
}

func (a *watchdogAlertTest) LateTestNameSuffix() string {
	return "alert/Watchdog must have no gaps or changes"
}

func (a *watchdogAlertTest) InvariantTestName() string {
	return "[bz-monitoring][invariant] alert/Watchdog must have no gaps or changes"
}

func (a *watchdogAlertTest) AlertName() string {
	return "Watchdog"
}

func (a *watchdogAlertTest) AlertState() AlertState {
	return AlertInfo
}

func (a *watchdogAlertTest) TestAlert(ctx context.Context, prometheusClient prometheusv1.API, restConfig *rest.Config) error {
	testDuration := exutil.DurationSinceStartInSeconds().String()

	// Invariant: The watchdog alert should be firing continuously during the whole upgrade via the thanos
	// querier (which should have no gaps when it queries the individual stores). Allow zero or one changes
	// to the presence of this series (zero if data is preserved over upgrade, one if data is lost on upgrade).
	// This would not catch the alert stopping firing, but we catch that in other places and tests.
	watchdogQuery := fmt.Sprintf(`changes((max((ALERTS{alertstate="firing",alertname="Watchdog",severity="none"}) or (absent(ALERTS{alertstate="firing",alertname="Watchdog",severity="none"})*0)))[%s:1s]) > 1`, testDuration)
	result, err := helper.RunQuery(ctx, prometheusClient, watchdogQuery)
	if err != nil {
		return fmt.Errorf("unable to check watchdog alert over the window: %v", err)
	}
	o.Expect(err).NotTo(o.HaveOccurred())
	if len(result.Data.Result) > 0 {
		framework.Failf("Watchdog alert had %s changes during the run, which may be a sign of a Prometheus outage in violation of the prometheus query SLO of 100%% uptime", result.Data.Result[0].Value)
	}

	return nil
}

func IsWatchdogAlert(eventInterval monitorapi.EventInterval) bool {
	return eventInterval.Locator == "alert/Watchdog ns/openshift-monitoring"
}

func isSNOUpgradeTest(ctx context.Context, restConfig *rest.Config) (bool, error) {
	configClient, err := configclient.NewForConfig(restConfig)
	if err != nil {
		return false, fmt.Errorf("Failed to establish API server connection")
	}
	infra, err := configClient.ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("Failed to find Infra config")
	}
	// Exit early if its not SNO
	if infra.Status.ControlPlaneTopology != configv1.SingleReplicaTopologyMode {
		return false, nil
	}

	clusterVersion, err := configClient.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("Failed to find cluster version")
	}
	// Make sure this cluster is being upgraded
	return len(clusterVersion.Status.History) > 1, nil
}

func (a *watchdogAlertTest) InvariantCheck(ctx context.Context, restConfig *rest.Config, alertIntervals monitorapi.Intervals, _ monitorapi.ResourcesMap) ([]*junitapi.JUnitTestCase, error) {

	// Skip this test when SNO is being upgraded
	isSNOUpgrade, err := isSNOUpgradeTest(ctx, restConfig)
	if err != nil {
		return []*junitapi.JUnitTestCase{
			{
				Name: a.InvariantTestName(),
				FailureOutput: &junitapi.FailureOutput{
					Output: err.Error(),
				},
				SystemOut: err.Error(),
			},
		}, nil
	}
	if isSNOUpgrade {
		return []*junitapi.JUnitTestCase{}, nil
	}

	watchdogIntervals := alertIntervals.Filter(IsWatchdogAlert)
	describe := watchdogIntervals.Strings()

	switch len(watchdogIntervals) {
	case 1:
		return []*junitapi.JUnitTestCase{
			{
				Name: a.InvariantTestName(),
			},
		}, nil
	case 0:
		return []*junitapi.JUnitTestCase{
			{
				Name: a.InvariantTestName(),
				FailureOutput: &junitapi.FailureOutput{
					Output: "Watchdog alert not found",
				},
				SystemOut: "Watchdog alert not found",
			},
		}, nil
	default:
		message := fmt.Sprintf("Watchdog alert had %v changes during the run, which may be a sign of a Prometheus outage in violation of the prometheus query SLO of 100%% uptime\n\n%s", len(alertIntervals), strings.Join(describe, "\n"))
		return []*junitapi.JUnitTestCase{
			{
				Name: a.InvariantTestName(),
				FailureOutput: &junitapi.FailureOutput{
					Output: message,
				},
				SystemOut: message,
			},
		}, nil

	}
}

package prometheus

import (
	"context"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	helper "github.com/openshift/origin/test/extended/util/prometheus"
	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

// MetricsAvailableAfterUpgradeTest tests that metrics from before an upgrade
// are also available after the upgrade.
type MetricsAvailableAfterUpgradeTest struct {
	executionTimestamp model.Time
}

func (t *MetricsAvailableAfterUpgradeTest) Name() string {
	return "prometheus-metrics-available-after-upgrade"
}

func (t *MetricsAvailableAfterUpgradeTest) DisplayName() string {
	return "[sig-instrumentation] Prometheus metrics should be available after an upgrade"
}

func (t *MetricsAvailableAfterUpgradeTest) Setup(f *e2e.Framework) {
	oc := exutil.NewCLIWithFramework(f)
	queryUrl, _, bearerToken, ok := helper.LocatePrometheus(oc)
	if !ok {
		e2e.Failf("Prometheus could not be located on this cluster, failing test %s", t.Name())
	}

	ns := oc.SetupNamespace()
	execPod := exutil.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod")

	defer func() {
		oc.AdminKubeClient().CoreV1().Pods(ns).Delete(context.Background(), execPod.Name, *metav1.NewDeleteOptions(1))
	}()

	g.By("getting the prometheus_build_info metric before the upgrade")
	preUpgradeQuery := `prometheus_build_info{pod="prometheus-k8s-0"}`
	preUpgradeResponse, err := helper.RunQuery(preUpgradeQuery, ns, execPod.Name, queryUrl, bearerToken)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(preUpgradeResponse.Data.Result).NotTo(o.BeEmpty())

	t.executionTimestamp = preUpgradeResponse.Data.Result[0].Timestamp
}

func (t *MetricsAvailableAfterUpgradeTest) Test(f *e2e.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	<-done

	oc := exutil.NewCLIWithFramework(f)
	queryUrl, _, bearerToken, ok := helper.LocatePrometheus(oc)
	if !ok {
		e2e.Failf("Prometheus could not be located on this cluster, failing test %s", t.Name())
	}

	ns := oc.SetupNamespace()
	execPod := exutil.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod")

	defer func() {
		oc.AdminKubeClient().CoreV1().Pods(ns).Delete(context.Background(), execPod.Name, *metav1.NewDeleteOptions(1))
	}()

	g.By("verifying that the timeseries is queryable at the same timestamp after the upgrade")
	postUpgradeQuery := `prometheus_build_info{pod="prometheus-k8s-0"}`
	postUpgradeResponse, err := helper.RunQueryAtTime(postUpgradeQuery, ns, execPod.Name, queryUrl, bearerToken, t.executionTimestamp)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(postUpgradeResponse.Data.Result).NotTo(o.BeEmpty())
}

func (t MetricsAvailableAfterUpgradeTest) Teardown(f *e2e.Framework) {
	return
}

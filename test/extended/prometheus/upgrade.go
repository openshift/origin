package prometheus

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	yaml "gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"

	exutil "github.com/openshift/origin/test/extended/util"
	helper "github.com/openshift/origin/test/extended/util/prometheus"
)

// MetricsAvailableAfterUpgradeTest tests that metrics from before an upgrade
// are also available after the upgrade.
type MetricsAvailableAfterUpgradeTest struct {
	executionTimestamp time.Time
}

func (t *MetricsAvailableAfterUpgradeTest) Name() string {
	return "prometheus-metrics-available-after-upgrade"
}

func (t *MetricsAvailableAfterUpgradeTest) DisplayName() string {
	return "[sig-instrumentation] Prometheus metrics should be available after an upgrade"
}

func (t *MetricsAvailableAfterUpgradeTest) Setup(ctx context.Context, f *e2e.Framework) {
	oc := exutil.NewCLIWithFramework(f)

	g.By("getting the prometheus_build_info metric before the upgrade")
	preUpgradeQuery := `prometheus_build_info{pod="prometheus-k8s-0"}`
	// Retry a few times in case prometheus-k8s-0 hasn't been scraped yet.
	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, 1*time.Minute, true, func(context.Context) (bool, error) {
		preUpgradeResponse, err := helper.RunQuery(ctx, oc.NewPrometheusClient(ctx), preUpgradeQuery)
		if err != nil {
			return false, err
		}

		if len(preUpgradeResponse.Data.Result) == 0 {
			return false, nil
		}

		t.executionTimestamp = preUpgradeResponse.Data.Result[0].Timestamp.Time()
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func (t *MetricsAvailableAfterUpgradeTest) Test(ctx context.Context, f *e2e.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	<-done

	oc := exutil.NewCLIWithFramework(f)

	g.By("verifying that the timeseries is queryable at the same timestamp after the upgrade")
	postUpgradeQuery := `prometheus_build_info{pod="prometheus-k8s-0"}`
	postUpgradeResponse, err := helper.RunQueryAtTime(ctx, oc.NewPrometheusClient(ctx), postUpgradeQuery, t.executionTimestamp)

	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(postUpgradeResponse.Data.Result).NotTo(o.BeEmpty())
}

func (t MetricsAvailableAfterUpgradeTest) Teardown(ctx context.Context, f *e2e.Framework) {
	return
}

func (t *MetricsAvailableAfterUpgradeTest) Skip(_ upgrades.UpgradeContext) bool {
	cfg, err := exutil.GetClientConfig(exutil.KubeConfigPath())
	if err != nil {
		return false
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return false
	}

	return !isPersistentStorageEnabled(client)
}

func isPersistentStorageEnabled(kubeClient kubernetes.Interface) bool {
	cmClient := kubeClient.CoreV1().ConfigMaps("openshift-monitoring")
	config, err := cmClient.Get(context.TODO(), "cluster-monitoring-config", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return false
	}

	var configData map[string]map[string]interface{}
	err = yaml.Unmarshal([]byte(config.Data["config.yaml"]), &configData)
	if err != nil {
		return false
	}

	_, found := configData["prometheusK8s"]["volumeClaimTemplate"]
	return found
}

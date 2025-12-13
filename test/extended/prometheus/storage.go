package prometheus

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	helper "github.com/openshift/origin/test/extended/util/prometheus"
	"k8s.io/apimachinery/pkg/api/errors"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	// storage_operation_duration_seconds_bucket with the mount operations that are considered acceptable.
	expectedMountTimeSeconds = 120
	// storage_operation_duration_seconds_bucket with the attach operations that are considered acceptable.
	expectedAttachTimeSeconds = 120
)

var _ = g.Describe("[sig-storage][Late] Metrics", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithoutNamespace("prometheus")

		url, token string
	)

	g.BeforeEach(func(ctx g.SpecContext) {
		_, err := helper.PrometheusServiceURL(ctx, oc)
		if errors.IsNotFound(err) {
			g.Skip("Prometheus could not be located on this cluster, skipping prometheus test")
		}
		o.Expect(err).NotTo(o.HaveOccurred(), "Verify prometheus service exists")
		url, err = helper.ThanosQuerierRouteURL(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "Get public url of thanos querier")
		token, err = helper.RequestPrometheusServiceAccountAPIToken(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "Request prometheus service account API token")
	})
	g.It("should report short attach times", g.Label("Size:M"), func(ctx g.SpecContext) {
		checkOperation(ctx, oc, url, token, "kube-controller-manager", "volume_attach", expectedAttachTimeSeconds)
	})
	g.It("should report short mount times", g.Label("Size:M"), func(ctx g.SpecContext) {
		checkOperation(ctx, oc, url, token, "kubelet", "volume_mount", expectedMountTimeSeconds)
	})
})

func checkOperation(ctx context.Context, oc *exutil.CLI, url string, bearerToken string, component string, name string, threshold int) {
	plugins := []string{"kubernetes.io/azure-disk", "kubernetes.io/aws-ebs", "kubernetes.io/gce-pd", "kubernetes.io/cinder", "kubernetes.io/vsphere-volume"}

	// we only consider series sent since the beginning of the test
	testDuration := exutil.DurationSinceStartInSeconds().String()

	tests := map[string]bool{}
	// Check "[total nr. of ops] - [nr. of ops < threshold] > 0' and expect failure (all ops should be < threshold).
	// Using sum(max(...)) to sum all kubelets / controller-managers
	// Adding a comment to make the failure more readable.
	e2e.Logf("Checking that Operation %s time of plugin %s should be <= %d seconds", name, plugins, threshold)
	queryTemplate := `
# Operation %[4]s time of plugin %[1]s should be < %[2]d seconds
  sum(max_over_time(storage_operation_duration_seconds_bucket{job="%[3]s",le="+Inf",operation_name="%[4]s",volume_plugin="%[1]s"}[%[5]s]))
- sum(max_over_time(storage_operation_duration_seconds_bucket{job="%[3]s",le="%[2]d",operation_name="%[4]s",volume_plugin="%[1]s"}[%[5]s]))
> 0`
	for _, plugin := range plugins {
		query := fmt.Sprintf(queryTemplate, plugin, threshold, component, name, testDuration)
		// Expect failure of the query (the result should be 0, all ops are expected to take < threshold)
		tests[query] = false
	}

	err := helper.RunQueries(ctx, oc.NewPrometheusClient(ctx), tests, oc)
	if err != nil {
		result.Flakef("Operation %s of plugin %s took more than %d seconds: %s", name, plugins, threshold, err)
	}
}

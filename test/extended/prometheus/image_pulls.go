package prometheus

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	helper "github.com/openshift/origin/test/extended/util/prometheus"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

// ImagePullsAreFast asserts that image pulls complete quickly.
type ImagePullsAreFast struct {
	threshold time.Duration
}

func (t *ImagePullsAreFast) Name() string {
	return "image-pulls-are-fast"
}

func (t *ImagePullsAreFast) DisplayName() string {
	return "[sig-node][Late] Image pulls are fast"
}

func (t *ImagePullsAreFast) Setup(_ context.Context, _ *framework.Framework) {
	if t.threshold.Seconds() == 0 {
		t.threshold = 2 * time.Minute
	}
	return
}

func (t *ImagePullsAreFast) Test(ctx context.Context, f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	<-done

	g.By(fmt.Sprintf("verifying that pull durations do not exceed %s", t.threshold))

	oc := exutil.NewCLIWithFramework(f)

	// we only consider samples since the beginning of the test
	testDuration := exutil.DurationSinceStartInSeconds().String()

	// checking with second-granularity over testDuration, count
	// the number of times that the largest CRI-O pull was slower
	// than threshold seconds.
	query := fmt.Sprintf(`
count_over_time((
  max(container_runtime_crio_operations_latency_microseconds{operation_type="PullImage"})
)[%[1]s:1s]) > %d * 1e6
`, testDuration, int(t.threshold.Seconds()))
	response, err := helper.RunQuery(ctx, oc.NewPrometheusClient(ctx), query)
	o.Expect(err).NotTo(o.HaveOccurred(), "unable to retrieve CRI-O pull latencies")

	if len(response.Data.Result) == 0 {
		return
	} else if len(response.Data.Result) > 1 {
		framework.Failf("expected a single series, got %d: %v", len(response.Data.Result), response.Data.Result)
	}

	series := response.Data.Result[0]
	result.Flakef("PullImage operations over %s for %s seconds", t.threshold, series.Value)
}

func (t ImagePullsAreFast) Teardown(_ context.Context, _ *framework.Framework) {
	return
}

func (t *ImagePullsAreFast) Skip(_ upgrades.UpgradeContext) bool {
	return false
}

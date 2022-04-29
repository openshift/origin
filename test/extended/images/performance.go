package images

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	helper "github.com/openshift/origin/test/extended/util/prometheus"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[bz-Image Registry][sig-imageregistry][Late] Image pulls", func() {
	oc := exutil.NewCLIWithoutNamespace("image-pull-perf")

	g.It("should be fast", func() {
		ctx := context.TODO()
		threshold := 2 * time.Minute

		// we only consider samples since the beginning of the test
		testDuration := exutil.DurationSinceStartInSeconds().String()

		// checking with second-granularity over testDuration, count
		// the number of times that the largest CRI-O pull was slower
		// than threshold seconds.
		query := fmt.Sprintf(`
count_over_time((
  max(container_runtime_crio_operations_latency_microseconds{operation_type="PullImage"})
)[%[1]s:1s]) > %d * 1e6
`, testDuration, int(threshold.Seconds()))
		response, err := helper.RunQuery(ctx, oc.NewPrometheusClient(ctx), query)
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to retrieve CRI-O pull latencies")

		if len(response.Data.Result) == 0 {
			return
		} else if len(response.Data.Result) > 1 {
			framework.Failf("expected a single series, got %d: %v", len(response.Data.Result), response.Data.Result)
		}

		series := response.Data.Result[0]
		result.Flakef("PullImage operations over %s for %s seconds", threshold, series.Value)
	})
})

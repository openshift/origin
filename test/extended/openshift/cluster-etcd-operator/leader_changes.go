package etcd

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/prometheus/common/model"

	"github.com/openshift/origin/test/extended/prometheus/client"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-etcd] etcd", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("etcd-leader-change").AsAdmin()
	g.It("leader changes are not excessive [Late]", func() {
		prometheus, err := client.NewE2EPrometheusRouterClient(oc)
		o.Expect(err).ToNot(o.HaveOccurred())

		// we only consider series sent since the beginning of the test
		testDuration := exutil.DurationSinceStartInSeconds().String()

		g.By("Examining the number of etcd leadership changes over the run")
		result, _, err := prometheus.Query(context.Background(), fmt.Sprintf("max(max by (pod,job) (increase(etcd_server_leader_changes_seen_total[%s])))", testDuration), time.Now())
		o.Expect(err).ToNot(o.HaveOccurred())

		vec, ok := result.(model.Vector)
		if !ok {
			o.Expect(fmt.Errorf("expecting Prometheus query to return a vector, got %s instead", vec.Type())).ToNot(o.HaveOccurred())
		}

		if len(vec) == 0 {
			o.Expect(fmt.Errorf("expecting Prometheus query to return at least one item, got 0 instead")).ToNot(o.HaveOccurred())
		}

		leaderChanges := vec[0].Value
		if leaderChanges != 0 {
			o.Expect(fmt.Errorf("Observed %s leader changes (expected 0) in %s: Leader changes are a result of stopping the etcd leader process or from latency (disk or network), review etcd performance metrics", leaderChanges, testDuration)).ToNot(o.HaveOccurred())
		}
	})
})

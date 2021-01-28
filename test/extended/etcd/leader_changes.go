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
	oc := exutil.NewCLI("etcd-leader-change").AsAdmin()
	g.It("leader changes are not excessive [Late]", func() {
		prometheus, err := client.NewE2EPrometheusRouterClient(oc)
		o.Expect(err).ToNot(o.HaveOccurred())

		// we only consider series sent since the beginning of the test
		testDuration := exutil.DurationSinceStartInSeconds().String()

		g.By("Examining the number of etcd leadership changes over the run")
		result, _, err := prometheus.Query(context.Background(), fmt.Sprintf("increase((max(max by (job) (etcd_server_leader_changes_seen_total)) or 0*absent(etcd_server_leader_changes_seen_total))[%s:15s])", testDuration), time.Now())
		o.Expect(err).ToNot(o.HaveOccurred())
		leaderChanges := result.(model.Vector)[0].Value
		if leaderChanges != 0 {
			o.Expect(fmt.Errorf("Observed %s leader changes in %s: Leader changes are a result of stopping the etcd leader process or from latency (disk or network), review etcd performance metrics", leaderChanges, testDuration)).ToNot(o.HaveOccurred())
		}
	})
})

package etcd

import (
	"context"
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
	g.It("[Late] leader changes are not excessive", func() {
		prometheus, err := client.NewE2EPrometheusRouterClient(oc)
		o.Expect(err).ToNot(o.HaveOccurred())
		g.By("Examining the rate of increase in the number of etcd leadership changes for last five minutes")
		result, _, err := prometheus.Query(context.Background(), "max by (job) (etcd_server_leader_changes_seen_total)", time.Now())
		o.Expect(err).ToNot(o.HaveOccurred())
		o.Expect(result.(model.Vector)[0].Value).To(o.BeNumerically("==", 2))
	})
})

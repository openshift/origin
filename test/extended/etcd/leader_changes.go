package etcd

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitortestlibrary/allowedalerts"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/prometheus/common/model"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/prometheus/client"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-etcd] etcd", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("etcd-leader-change").AsAdmin()

	var earlyTimeStamp time.Time
	g.It("record the start revision of the etcd-operator [Early]", g.Label("Size:S"), func() {
		earlyTimeStamp = time.Now()
	})

	g.It("leader changes are not excessive [Late]", g.Label("Size:S"), func(ctx g.SpecContext) {
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			oc = exutil.NewHypershiftManagementCLI("default").AsAdmin().WithoutNamespace()
		}

		prometheus, err := client.NewE2EPrometheusRouterClient(ctx, oc)
		o.Expect(err).ToNot(o.HaveOccurred())

		// we only consider series sent since the beginning of the test
		testDuration := exutil.DurationSinceStartInSeconds().String()

		g.By("Examining the number of etcd leadership changes over the run")
		etcdNamespace := "openshift-etcd"
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			etcdNamespace = "clusters-.*"
		}
		result, _, err := prometheus.Query(context.Background(), fmt.Sprintf(`max(max by (pod,job) (increase(etcd_server_leader_changes_seen_total{namespace=~"%s"}[%s])))`, etcdNamespace, testDuration), time.Now())
		o.Expect(err).ToNot(o.HaveOccurred())

		vec, ok := result.(model.Vector)
		if !ok {
			o.Expect(fmt.Errorf("expecting Prometheus query to return a vector, got %s instead", vec.Type())).ToNot(o.HaveOccurred())
		}

		if len(vec) == 0 {
			o.Expect(fmt.Errorf("expecting Prometheus query to return at least one item, got 0 instead")).ToNot(o.HaveOccurred())
		}

		// calculate the number of etcd rollouts during the tests
		// based on that calculate the number of allowed leader elections
		// we allow max 3 elections per revision (we assume there are 3 master machines at most)
		numberOfRevisions, err := allowedalerts.GetEstimatedNumberOfRevisionsForEtcdOperator(context.TODO(), oc.KubeClient(), time.Now().Sub(earlyTimeStamp))
		o.Expect(err).ToNot(o.HaveOccurred())

		allowedNumberOfRevisions := numberOfRevisions * 3
		leaderChanges := vec[0].Value
		if int(leaderChanges) > allowedNumberOfRevisions {
			o.Expect(fmt.Errorf("observed %s leader changes (expected %v) in %s: Leader changes are a result of stopping the etcd leader process or from latency (disk or network), review etcd performance metrics", leaderChanges, allowedNumberOfRevisions, testDuration)).ToNot(o.HaveOccurred())
		}
	})
})

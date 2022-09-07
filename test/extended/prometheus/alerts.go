package prometheus

import (
	"context"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/openshift/origin/pkg/synthetictests/allowedalerts"
	exutil "github.com/openshift/origin/test/extended/util"
)

func init() {
	alertTests := allowedalerts.AllAlertTests(context.TODO(), nil, 0)
	for i := range alertTests {
		alertTest := alertTests[i]

		// These tests make use of Prometheus metrics, which are not present in the absence of cluster-monitoring-operator, the owner for
		// the api groups tagged here.
		var _ = g.Describe("[sig-arch]"+alertTest.TestNamePrefix()+" [apigroup:monitoring.coreos.com]", func() {
			defer g.GinkgoRecover()
			var (
				oc = exutil.NewCLIWithoutNamespace("prometheus")
			)

			g.It(alertTest.LateTestNameSuffix(), func() {
				err := alertTest.TestAlert(context.TODO(), oc.NewPrometheusClient(context.TODO()), oc.AdminConfig())
				o.Expect(err).NotTo(o.HaveOccurred(), "unable to check watchdog alert over test window")

			})
		})
	}
}

package prometheus

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/openshift/origin/pkg/synthetictests/allowedalerts"
	exutil "github.com/openshift/origin/test/extended/util"
)

func init() {
	alertTests := allowedalerts.AllAlertTests(context.TODO(), nil, 0)
	for i := range alertTests {
		alertTest := alertTests[i]

		var _ = g.Describe("[sig-arch]"+alertTest.TestNamePrefix(), func() {
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

package synthetictests

import (
	"context"

	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo"

	"github.com/openshift/origin/pkg/synthetictests/allowedalerts"
)

func testAlerts(events monitorapi.Intervals, restConfig *rest.Config) []*ginkgo.JUnitTestCase {
	ret := []*ginkgo.JUnitTestCase{}

	alertTests := allowedalerts.AllAlertTests()
	for i := range alertTests {
		alertTest := alertTests[i]

		junit, err := alertTest.InvariantCheck(context.TODO(), restConfig, events)
		if err != nil {
			ret = append(ret, &ginkgo.JUnitTestCase{
				Name: alertTest.InvariantTestName(),
				FailureOutput: &ginkgo.FailureOutput{
					Output: err.Error(),
				},
				SystemOut: err.Error(),
			})
		}
		ret = append(ret, junit...)
	}

	return ret
}

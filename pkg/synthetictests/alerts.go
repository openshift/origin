package synthetictests

import (
	"context"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/synthetictests/allowedalerts"
)

func testAlerts(events monitorapi.Intervals, restConfig *rest.Config) []*junitapi.JUnitTestCase {
	ret := []*junitapi.JUnitTestCase{}

	alertTests := allowedalerts.AllAlertTests(context.TODO(), restConfig)
	for i := range alertTests {
		alertTest := alertTests[i]

		junit, err := alertTest.InvariantCheck(context.TODO(), restConfig, events)
		if err != nil {
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: alertTest.InvariantTestName(),
				FailureOutput: &junitapi.FailureOutput{
					Output: err.Error(),
				},
				SystemOut: err.Error(),
			})
		}
		ret = append(ret, junit...)
	}

	return ret
}

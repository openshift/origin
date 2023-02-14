package synthetictests

import (
	"context"
	"time"

	"github.com/openshift/origin/pkg/synthetictests/platformidentification"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"

	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/synthetictests/allowedalerts"
)

func testAlerts(events monitorapi.Intervals, restConfig *rest.Config,
	duration time.Duration, recordedResource *monitorapi.ResourcesMap) []*junitapi.JUnitTestCase {

	jobType, err := platformidentification.GetJobType(context.TODO(), restConfig)
	if err != nil {
		// TODO: technically this should fail all tests...
		framework.Logf("ERROR: unable to determine job type for alert testing, abandoning all alert tests: %v", err)
	}

	var etcdAllowance allowedalerts.AlertTestAllowanceCalculator
	etcdAllowance = allowedalerts.DefaultAllowances
	// if we have a clientConfig,  use it.
	if restConfig != nil {
		kubeClient, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			panic(err)
		}
		etcdAllowance, err = allowedalerts.NewAllowedWhenEtcdRevisionChange(context.TODO(),
			kubeClient, duration)
		if err != nil {
			panic(err)
		}
	}

	ret := runAlertTests(jobType, etcdAllowance, events, recordedResource)
	return ret
}

func runAlertTests(jobType *platformidentification.JobType,
	etcdAllowance allowedalerts.AlertTestAllowanceCalculator,
	events monitorapi.Intervals,
	recordedResource *monitorapi.ResourcesMap) []*junitapi.JUnitTestCase {

	ret := []*junitapi.JUnitTestCase{}
	alertTests := allowedalerts.AllAlertTests(jobType, etcdAllowance)
	for i := range alertTests {
		alertTest := alertTests[i]

		junit, err := alertTest.InvariantCheck(events, *recordedResource)
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

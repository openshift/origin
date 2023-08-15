package disruptionlibrary

import (
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortestlibrary/allowedbackenddisruption"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/disruption"
	"github.com/sirupsen/logrus"
	"k8s.io/kubernetes/test/e2e/framework"
)

func TestServerAvailability(
	owner, locator string,
	events monitorapi.Intervals,
	jobRunDuration time.Duration,
	jobType *platformidentification.JobType) []*junitapi.JUnitTestCase {
	logger := logrus.WithField("owner", owner).WithField("locator", locator)
	testName := fmt.Sprintf("[%s] %s should be available throughout the test", owner, locator)

	// Lookup allowed disruption based on historical data:
	locatorParts := monitorapi.LocatorParts(locator)
	disruptionName := monitorapi.DisruptionFrom(locatorParts)
	connType := monitorapi.DisruptionConnectionTypeFrom(locatorParts)
	backendName := fmt.Sprintf("%s-%s-connections", disruptionName, connType)
	if jobType == nil {
		// check for MicroShift cluster
		kubeClient, err := framework.LoadClientset(true)
		if err != nil {
			panic(err)
		}
		isMicroShift, err := exutil.IsMicroShiftCluster(kubeClient)
		if err != nil {
			panic(err)
		}
		if isMicroShift {
			return []*junitapi.JUnitTestCase{}
		}
		return []*junitapi.JUnitTestCase{
			{
				Name:     testName,
				Duration: jobRunDuration.Seconds(),
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("error in platform identification"),
				},
			},
		}
	}
	logger.Infof("testing server availability for: %+v", *jobType)

	allowedDisruption, disruptionDetails, err :=
		allowedbackenddisruption.GetAllowedDisruption(backendName, *jobType)
	if err != nil {
		return []*junitapi.JUnitTestCase{
			{
				Name:     testName,
				Duration: jobRunDuration.Seconds(),
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("error in getting allowed disruption: %s", err),
				},
			},
		}
	}

	// Check if we got an empty result, which signals we did not have historical data for this NURP and thus
	// do not want to run the test.
	if allowedDisruption == nil {
		// An empty StatisticalDuration implies we did not find any data and thus do not want to run the disruption
		// test. We'll mark it as a flake and explain why so we can find these tests should anyone need to look.
		return []*junitapi.JUnitTestCase{
			{
				Name:     testName,
				Duration: jobRunDuration.Seconds(),
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("skipping test due to no historical disruption data: %s", disruptionDetails),
				},
			},
			{
				Name: testName,
			},
		}
	}

	roundedAllowedDisruption := allowedDisruption.Round(time.Second)
	if allowedDisruption.Milliseconds() == disruption.DefaultAllowedDisruption {
		// don't round if we're using the default value so we can find this.
		roundedAllowedDisruption = *allowedDisruption
	}
	framework.Logf("allowedDisruption for backend %s: %s, details: %q",
		backendName, roundedAllowedDisruption, disruptionDetails)

	observedDisruption, disruptionMsgs, _ := monitorapi.BackendDisruptionSeconds(locator, events)

	resultsStr := fmt.Sprintf(
		"%s was unreachable during disruption testing for at least %s of %s (maxAllowed=%s):\n\n%s",
		backendName, observedDisruption, jobRunDuration.Round(time.Second), roundedAllowedDisruption, disruptionDetails)
	successTest := &junitapi.JUnitTestCase{
		Name:     testName,
		Duration: jobRunDuration.Seconds(),
	}
	if observedDisruption > roundedAllowedDisruption {
		test := &junitapi.JUnitTestCase{
			Name:     testName,
			Duration: jobRunDuration.Seconds(),
			FailureOutput: &junitapi.FailureOutput{
				Output: resultsStr,
			},
			SystemOut: strings.Join(disruptionMsgs, "\n"),
		}
		retVal := []*junitapi.JUnitTestCase{test}
		if isExcludedDisruptionBackend(backendName) {
			// Flake failures to allow us to track the disruptions without failing a payload.
			retVal = append(retVal, &junitapi.JUnitTestCase{
				Name: testName,
			})
		}
		return retVal
	} else {
		successTest.SystemOut = resultsStr
		return []*junitapi.JUnitTestCase{successTest}
	}
}

// isExcludedDisruptionBackend returns true if any of the given backends are in the
// disruption backend name.  Essentially, we want to test these disruption backends
// but flake them for now to avoid failing payloads.
func isExcludedDisruptionBackend(name string) bool {
	excludedNames := []string{
		"ci-cluster-network-liveness",
		"kube-api-http1-external-lb",
		"kube-api-http2-external-lb",
		"openshift-api-http2-external-lb",
	}

	for _, excludedName := range excludedNames {
		if strings.Contains(name, excludedName) {
			return true
		}
	}
	return false
}

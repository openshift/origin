package allowedalerts

import (
	_ "embed"
	"sync"

	"github.com/openshift/origin/pkg/monitortestlibrary/historicaldata"
)

// queryResults contains point in time results for the current aggregated query from above.
// Hardcoding this does several things.
//  1. it ensures that a degradation over time will be caught because this doesn't slip over time
//  2. it allows external people (recall we ship this binary) without access to ocp credentials to have a sense of what is normal on their platform
//  3. it gives a spot to wire in a dynamic look *if* someone desired to do so and made it conditional to avoid breaking
//     1 and 2
//
//go:embed query_results.json
var queryResults []byte

var (
	readResults    sync.Once
	historicalData *historicaldata.AlertBestMatcher
)

func GetHistoricalData() *historicaldata.AlertBestMatcher {
	readResults.Do(
		func() {
			var err error
			historicalData, err = historicaldata.NewAlertMatcher(queryResults)
			if err != nil {
				panic(err)
			}
		})

	return historicalData
}

// AllowedAlertNames is a list of alerts we do not test against.
var AllowedAlertNames = []string{
	"Watchdog",
	"AlertmanagerReceiversNotConfigured",
	"PrometheusRemoteWriteDesiredShards",
	"KubeJobFailingSRE", // https://issues.redhat.com/browse/OCPBUGS-55635

	// indicates a problem in the external Telemeter service, presently very common, does not impact our ability to e2e test:
	"TelemeterClientFailures",
	"CDIDefaultStorageClassDegraded", // Installing openshift virt with RWX storage fire an alarm, that is not relevant for most of the tests.
	"VirtHandlerRESTErrorsHigh",      // https://issues.redhat.com/browse/CNV-50418
	"VirtControllerRESTErrorsHigh",   // https://issues.redhat.com/browse/CNV-50418
}

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

func getCurrentResults() *historicaldata.AlertBestMatcher {
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

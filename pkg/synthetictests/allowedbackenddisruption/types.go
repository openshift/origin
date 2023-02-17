package allowedbackenddisruption

import (
	"bytes"
	_ "embed"
	"sync"

	"github.com/openshift/origin/pkg/synthetictests/historicaldata"
)

const (
	p95ViewQuery = `
SELECT
	BackendName,
	Release,
	FromRelease,
	Platform,
	Architecture,
	Network,
	Topology,
	ANY_VALUE(P95) AS P95,
	ANY_VALUE(P99) AS P99,
	FROM (
		SELECT
			Jobs.Release,
			Jobs.FromRelease,
			Jobs.Platform,
			Jobs.Network,
			Jobs.Topology,
			BackendName,
			PERCENTILE_CONT(BackendDisruption.DisruptionSeconds, 0.95) OVER(PARTITION BY BackendDisruption.BackendName, Jobs.Network, Jobs.Platform, Jobs.Release, Jobs.FromRelease, Jobs.Topology) AS P95,
			PERCENTILE_CONT(BackendDisruption.DisruptionSeconds, 0.99) OVER(PARTITION BY BackendDisruption.BackendName, Jobs.Network, Jobs.Platform, Jobs.Release, Jobs.FromRelease, Jobs.Topology) AS P99,
		FROM
			openshift-ci-data-analysis.ci_data.BackendDisruption as BackendDisruption
		INNER JOIN
			openshift-ci-data-analysis.ci_data.BackendDisruption_JobRuns as JobRuns on JobRuns.Name = BackendDisruption.JobRunName
		INNER JOIN
			openshift-ci-data-analysis.ci_data.Jobs as Jobs on Jobs.JobName = JobRuns.JobName
		WHERE
			JobRuns.StartTime > TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 21 DAY)
	)
	GROUP BY
		BackendName, Release, FromRelease, Platform, Network, Topology
`

	p95Query = `
SELECT * FROM openshift-ci-data-analysis.ci_data.BackendDisruption_Unified_LastWeek_P95 
order by 
 BackendName, Release, FromRelease, Topology, Platform, Network
`
)

//go:embed query_results.json
var queryResults []byte

var (
	readResults    sync.Once
	historicalData historicaldata.BestMatcher
)

// if data is missing for a particular jobtype combination, this is the value returned.  Choose a unique value that will
// be easily searchable across large numbers of job runs.  I like e.
const defaultReturn = 2.718

func getCurrentResults() historicaldata.BestMatcher {
	readResults.Do(
		func() {
			var err error
			genericBytes := bytes.ReplaceAll(queryResults, []byte(`    "BackendName": "`), []byte(`    "Name": "`))
			historicalData, err = historicaldata.NewMatcher(genericBytes, defaultReturn)
			if err != nil {
				panic(err)
			}
		})

	return historicalData
}

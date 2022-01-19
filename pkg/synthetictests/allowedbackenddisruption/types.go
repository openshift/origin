package allowedbackenddisruption

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"strconv"
	"sync"
)

const (
	p95ViewQuery = `
SELECT
	BackendName,
	Release,
	FromRelease,
	Platform,
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
	readResults       sync.Once
	percentilesAsList []LastWeekPercentiles
	percentilesAsMap  = map[LastWeekPercentileKey]LastWeekPercentiles{}
)

type LastWeekPercentiles struct {
	LastWeekPercentileKey `json:",inline"`
	P95                   float64
	P99                   float64
}

type LastWeekPercentileKey struct {
	BackendName string
	Release     string
	FromRelease string
	Platform    string
	Network     string
	Topology    string
}

func getCurrentResults() ([]LastWeekPercentiles, map[LastWeekPercentileKey]LastWeekPercentiles) {
	readResults.Do(
		func() {
			inFile := bytes.NewBuffer(queryResults)
			jsonDecoder := json.NewDecoder(inFile)

			type DecodingLastWeekPercentile struct {
				LastWeekPercentileKey `json:",inline"`
				P95                   string
				P99                   string
			}
			decodingPercentilesList := []DecodingLastWeekPercentile{}

			if err := jsonDecoder.Decode(&decodingPercentilesList); err != nil {
				panic(err)
			}

			for _, currDecoded := range decodingPercentilesList {
				p95, err := strconv.ParseFloat(currDecoded.P95, 64)
				if err != nil {
					panic(err)
				}
				p99, err := strconv.ParseFloat(currDecoded.P99, 64)
				if err != nil {
					panic(err)
				}
				curr := LastWeekPercentiles{
					LastWeekPercentileKey: currDecoded.LastWeekPercentileKey,
					P95:                   p95,
					P99:                   p99,
				}
				percentilesAsList = append(percentilesAsList, curr)
				percentilesAsMap[curr.LastWeekPercentileKey] = curr
			}
		})

	return percentilesAsList, percentilesAsMap
}

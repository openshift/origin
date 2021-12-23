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
	ANY_VALUE(P95) AS P95,
FROM (
	SELECT
		Jobs.Release,
		Jobs.FromRelease,
		Jobs.Platform,
		Jobs.Network,
		BackendName,
		PERCENTILE_CONT(BackendDisruption.DisruptionSeconds, 0.95) OVER(PARTITION BY BackendDisruption.BackendName, Jobs.Network, Jobs.Platform, Jobs.Release, Jobs.FromRelease) AS P95,
	FROM
		openshift-ci-data-analysis.ci_data.BackendDisruption as BackendDisruption
	INNER JOIN
		openshift-ci-data-analysis.ci_data.BackendDisruption_JobRuns as JobRuns on JobRuns.Name = BackendDisruption.JobRunName
	INNER JOIN
		openshift-ci-data-analysis.ci_data.Jobs as Jobs on Jobs.JobName = JobRuns.JobName
	WHERE
		JobRuns.StartTime BETWEEN TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 10 DAY)
	AND
		TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 3 DAY)
)
GROUP BY
	BackendName, Release, FromRelease, Platform, Network
`

	p95Query = `
SELECT * FROM openshift-ci-data-analysis.ci_data.BackendDisruption_Unified_LastWeek_P95 
order by 
 BackendName, Release, FromRelease, Platform, Network`
)

//go:embed query_results.json
var queryResults []byte

var (
	readResults sync.Once
	p95AsList   []LastWeekP95
	p95AsMap    = map[LastWeekP95Key]LastWeekP95{}
)

type LastWeekP95 struct {
	LastWeekP95Key `json:",inline"`
	P95            float64
}

type LastWeekP95Key struct {
	BackendName string
	Release     string
	FromRelease string
	Platform    string
	Network     string
}

func getCurrentResults() ([]LastWeekP95, map[LastWeekP95Key]LastWeekP95) {
	readResults.Do(
		func() {
			inFile := bytes.NewBuffer(queryResults)
			jsonDecoder := json.NewDecoder(inFile)

			type DecodingLastWeekP95 struct {
				LastWeekP95Key `json:",inline"`
				P95            string
			}
			decodingP95AsList := []DecodingLastWeekP95{}

			if err := jsonDecoder.Decode(&decodingP95AsList); err != nil {
				panic(err)
			}

			for _, currDecoded := range decodingP95AsList {
				p95, err := strconv.ParseFloat(currDecoded.P95, 64)
				if err != nil {
					panic(err)
				}
				curr := LastWeekP95{
					LastWeekP95Key: currDecoded.LastWeekP95Key,
					P95:            p95,
				}
				p95AsList = append(p95AsList, curr)
				p95AsMap[curr.LastWeekP95Key] = curr
			}
		})

	return p95AsList, p95AsMap
}

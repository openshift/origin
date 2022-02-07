package allowedalerts

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"strconv"
	"sync"

	"github.com/openshift/origin/pkg/synthetictests/platformidentification"
)

const (
	// p95Query produces the query_results.json.  Take this query and run it against bigquery, then export the results
	// as json and place them query_results.json.
	// This query produces the p95 and p99 alert firing seconds for the named alerts on a per platform, release, topology,
	// network type basis.
	p95Query = `
SELECT * FROM openshift-ci-data-analysis.ci_data.Alerts_Unified_LastWeek_P95
where
  alertName = "etcdMembersDown" or 
  alertName = "etcdGRPCRequestsSlow" or 
  alertName = "etcdHighNumberOfFailedGRPCRequests" or 
  alertName = "etcdMemberCommunicationSlow" or 
  alertName = "etcdNoLeader" or 
  alertName = "etcdHighFsyncDurations" or 
  alertName = "etcdHighCommitDurations" or 
  alertName = "etcdInsufficientMembers" or 
  alertName = "etcdHighNumberOfLeaderChanges" or 
  alertName = "KubeAPIErrorBudgetBurn" or 
  alertName = "KubeClientErrors" or 
  alertName = "KubePersistentVolumeErrors" or 
  alertName = "MCDDrainError" or 
  alertName = "PrometheusOperatorWatchErrors"
order by 
 AlertName, Release, FromRelease, Topology, Platform, Network
`
)

// queryResults contains point in time results for the current aggregated query from above.
// Hardcoding this does several things.
//  1. it ensures that a degradation over time will be caught because this doesn't slip over time
//  2. it allows external people (recall we ship this binary) without access to ocp credentials to have a sense of what is normal on their platform
//  3. it gives a spot to wire in a dynamic look *if* someone desired to do so and made it conditional to avoid breaking
//     1 and 2
//go:embed query_results.json
var queryResults []byte

var (
	readResults       sync.Once
	percentilesAsList []LastWeekPercentiles
	percentilesAsMap  = map[LastWeekPercentileKey]LastWeekPercentiles{}
)

type LastWeekPercentiles struct {
	LastWeekPercentileKey `json:",inline"`
	Percentiles           `json:",inline"`
}

type Percentiles struct {
	P95 float64
	P99 float64
}

type LastWeekPercentileKey struct {
	AlertName                      string
	platformidentification.JobType `json:",inline"`
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
					Percentiles: Percentiles{
						P95: p95,
						P99: p99,
					},
				}
				percentilesAsList = append(percentilesAsList, curr)
				percentilesAsMap[curr.LastWeekPercentileKey] = curr
			}
		})

	return percentilesAsList, percentilesAsMap
}

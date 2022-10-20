package allowedalerts

import (
	"bytes"
	_ "embed"
	"sync"

	"github.com/openshift/origin/pkg/synthetictests/historicaldata"
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
  alertName = "PrometheusOperatorWatchErrors" or
  alertName = "VSphereOpenshiftNodeHealthFail"
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
//
//go:embed query_results.json
var queryResults []byte

var (
	readResults    sync.Once
	historicalData historicaldata.BestMatcher
)

// if data is missing for a particular jobtype combination, this is the value returned.  Choose a unique value that will
// be easily searchable across large numbers of job runs.  I like pi.
const defaultReturn = 3.141

func getCurrentResults() historicaldata.BestMatcher {
	readResults.Do(
		func() {
			var err error
			genericBytes := bytes.ReplaceAll(queryResults, []byte(`    "AlertName": "`), []byte(`    "Name": "`))
			historicalData, err = historicaldata.NewMatcher(genericBytes, defaultReturn)
			if err != nil {
				panic(err)
			}
		})

	return historicalData
}

// Package allowednodeprovisioning holds the historical baseline used to detect node provisioning
// latency regressions.
//
// query_results.json is populated by an automated weekly job that runs provisioningP99ViewQuery
// against BigQuery and commits the results. Each entry looks like:
//
//	{
//	  "BackendName": "MachineToReadySeconds/worker/true",
//	  "Release": "4.22", "FromRelease": "", "Platform": "aws",
//	  "Architecture": "amd64", "Network": "OVNKubernetes", "Topology": "ha",
//	  "P95": "301.200", "P99": "412.900", "JobRuns": 250
//	}
//
// BackendName encodes "<Segment>/<MachineRole>/<CreatedDuringRun>" so the existing
// historicaldata.DisruptionBestMatcher can be reused unchanged. The matcher requires at least
// defaultMinJobRuns (100) runs and falls back to previous releases; when no usable match exists
// the caller must skip rather than fail.
package allowednodeprovisioning

import (
	_ "embed"
	"sync"

	"github.com/openshift/origin/pkg/monitortestlibrary/historicaldata"
)

// provisioningP99ViewQuery computes per-run medians (matching the monitor's own comparison) and
// then the P95/P99 of those medians over the last 21 days. It is documented here for the external
// job that refreshes query_results.json; it is not executed by this binary.
//
// The join path is the one proven for autodl tables in this repo. The alternative
// ci_data.JobRuns/ci_data.Jobs join used by the disruption baseline is for a non-autodl table and
// is not relied on here.
const provisioningP99ViewQuery = `
WITH per_machine AS (
  SELECT
    L.JobRunName,
    L.MachineRole,
    L.CreatedDuringRun,
    J.Release,
    J.FromRelease,
    J.Platform,
    J.Architecture,
    J.Network,
    J.Topology,
    L.MachineToInstanceSeconds,
    L.InstanceToNodeSeconds,
    L.NodeToReadySeconds,
    L.MachineToReadySeconds,
    L.MachineToNodeSeconds
  FROM openshift-ci-data-analysis.ci_data_autodl.node_provisioning_latency AS L
  INNER JOIN openshift-gce-devel.ci_analysis_us.jobs AS R
    ON R.prowjob_build_id = L.JobRunName
  INNER JOIN openshift-ci-data-analysis.ci_data.JobsWithVariants AS J
    ON J.JobName = R.prowjob_job_name
  WHERE L.PartitionTime > TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 21 DAY)
),
unpivoted AS (
  SELECT JobRunName, MachineRole, CreatedDuringRun, Release, FromRelease, Platform,
         Architecture, Network, Topology,
         'MachineToInstanceSeconds' AS Segment, MachineToInstanceSeconds AS DurationSeconds
  FROM per_machine WHERE MachineToInstanceSeconds IS NOT NULL
  UNION ALL
  SELECT JobRunName, MachineRole, CreatedDuringRun, Release, FromRelease, Platform,
         Architecture, Network, Topology,
         'InstanceToNodeSeconds', InstanceToNodeSeconds
  FROM per_machine WHERE InstanceToNodeSeconds IS NOT NULL
  UNION ALL
  SELECT JobRunName, MachineRole, CreatedDuringRun, Release, FromRelease, Platform,
         Architecture, Network, Topology,
         'NodeToReadySeconds', NodeToReadySeconds
  FROM per_machine WHERE NodeToReadySeconds IS NOT NULL
  UNION ALL
  SELECT JobRunName, MachineRole, CreatedDuringRun, Release, FromRelease, Platform,
         Architecture, Network, Topology,
         'MachineToReadySeconds', MachineToReadySeconds
  FROM per_machine WHERE MachineToReadySeconds IS NOT NULL
  UNION ALL
  SELECT JobRunName, MachineRole, CreatedDuringRun, Release, FromRelease, Platform,
         Architecture, Network, Topology,
         'MachineToNodeSeconds', MachineToNodeSeconds
  FROM per_machine WHERE MachineToNodeSeconds IS NOT NULL
),
per_run AS (
  SELECT
    JobRunName, Segment, MachineRole, CreatedDuringRun,
    Release, FromRelease, Platform, Architecture, Network, Topology,
    PERCENTILE_CONT(DurationSeconds, 0.5) OVER (
      PARTITION BY JobRunName, Segment, MachineRole, CreatedDuringRun
    ) AS MedianDurationSeconds
  FROM unpivoted
),
per_run_distinct AS (
  SELECT DISTINCT
    JobRunName, Segment, MachineRole, CreatedDuringRun,
    Release, FromRelease, Platform, Architecture, Network, Topology,
    MedianDurationSeconds
  FROM per_run
),
percentiles AS (
  SELECT
    *,
    PERCENTILE_CONT(MedianDurationSeconds, 0.95) OVER (
      PARTITION BY Segment, MachineRole, CreatedDuringRun,
                   Release, FromRelease, Platform, Architecture, Network, Topology
    ) AS P95,
    PERCENTILE_CONT(MedianDurationSeconds, 0.99) OVER (
      PARTITION BY Segment, MachineRole, CreatedDuringRun,
                   Release, FromRelease, Platform, Architecture, Network, Topology
    ) AS P99
  FROM per_run_distinct
)
SELECT
  CONCAT(Segment, '/', MachineRole, '/', CreatedDuringRun) AS BackendName,
  Release, FromRelease, Platform, Architecture, Network, Topology,
  FORMAT('%.3f', ANY_VALUE(P95)) AS P95,
  FORMAT('%.3f', ANY_VALUE(P99)) AS P99,
  COUNT(*) AS JobRuns
FROM percentiles
GROUP BY BackendName, Release, FromRelease, Platform, Architecture, Network, Topology
ORDER BY BackendName, Release, FromRelease, Topology, Platform, Network, Architecture
`

//go:embed query_results.json
var queryResults []byte

var (
	readResults    sync.Once
	historicalData *historicaldata.DisruptionBestMatcher
)

func GetCurrentResults() *historicaldata.DisruptionBestMatcher {
	readResults.Do(func() {
		var err error
		historicalData, err = historicaldata.NewDisruptionMatcher(queryResults)
		if err != nil {
			panic(err)
		}
	})

	return historicalData
}

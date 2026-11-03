package allowednodeprovisioning

import (
	"fmt"
	"strconv"
	"time"

	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
)

// DataKeyName returns the key used in query_results.json for a segment, role and run cohort.
// It must stay in sync with the CONCAT in provisioningP99ViewQuery.
func DataKeyName(segment, role string, createdDuringRun bool) string {
	return fmt.Sprintf("%s/%s/%s", segment, role, strconv.FormatBool(createdDuringRun))
}

// GetAllowedProvisioningLatency returns the historical P99 for the best matching job type, or nil
// when there is no match or too few historical runs. A nil result means the caller must skip the
// test rather than fail it.
func GetAllowedProvisioningLatency(segment, role string, createdDuringRun bool, jobType platformidentification.JobType) (*time.Duration, string, error) {
	// The matcher's previous-release fallback panics on an unparseable release, and an empty
	// release means we cannot identify this job's baseline at all.
	if jobType.Platform == "" || jobType.Release == "" {
		return nil, "job type is missing platform or release", nil
	}
	return GetCurrentResults().BestMatchP99(DataKeyName(segment, role, createdDuringRun), jobType)
}

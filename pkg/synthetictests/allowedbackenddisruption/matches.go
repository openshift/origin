package allowedbackenddisruption

import (
	"time"

	"github.com/openshift/origin/pkg/synthetictests/platformidentification"
)

// GetAllowedDisruption uses the backend and information about the cluster to choose the best historical p95 to operate against.
// We enforce "don't get worse" for disruption by watching the aggregate data in CI over many runs.
func GetAllowedDisruption(backendName string, jobType platformidentification.JobType) (*time.Duration, string, error) {
	return getCurrentResults().BestMatchP99(backendName, jobType)
}

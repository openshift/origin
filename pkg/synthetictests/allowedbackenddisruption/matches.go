package allowedbackenddisruption

import (
	"time"

	"github.com/openshift/origin/pkg/synthetictests/platformidentification"
)

// GetAllowedDisruption uses the backend and information about the cluster to choose the best historical p95 to operate against.
// We enforce "don't get worse" for disruption by watching the aggregate data in CI over many runs.
func GetAllowedDisruption(backendName string, jobType platformidentification.JobType) (*time.Duration, string, error) {
	t := 6 * time.Hour
	return &t, "disruption checking is disabled for prior releases because we lack a good baseline", nil
}

package allowedbackenddisruption

import (
	"time"

	"github.com/openshift/origin/pkg/synthetictests/platformidentification"
	"github.com/openshift/origin/test/extended/util/disruption/externalservice"
)

const (

	// allowedExternalDisruption is the amount of time we'll allow to flake in a CI run.
	// At present we do not have confidence that we can consistently hit an external service
	// and we do not know where the issue lies yet. Allowing 10 minutes means this test will
	// effectively never fail, but will flake if we experience ANY disruption. We can use this
	// to gather data, and correlate with real disruption in graphs.
	allowedExternalDisruption = 600 * time.Second
)

// GetAllowedDisruption uses the backend and information about the cluster to choose the best historical p95 to operate against.
// We enforce "don't get worse" for disruption by watching the aggregate data in CI over many runs.
func GetAllowedDisruption(backendName string, jobType platformidentification.JobType) (*time.Duration, string, error) {
	// Special case for the liveness probe to external service:
	if backendName == externalservice.LivenessProbeBackend {
		aed := allowedExternalDisruption
		return &aed, "forgiving limit for disruption to an external service", nil
	}
	return getCurrentResults().BestMatchP99(backendName, jobType)
}

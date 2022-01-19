package allowedbackenddisruption

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/synthetictests/platformidentification"
	"k8s.io/client-go/rest"
)

// GetAllowedDisruption uses the backend and information about the cluster to choose the best historical p95 to operate against.
// We enforce "don't get worse" for disruption by watching the aggregate data in CI over many runs.
func GetAllowedDisruption(ctx context.Context, backendName string, clientConfig *rest.Config) (*time.Duration, error) {
	jobType, err := platformidentification.GetJobType(ctx, clientConfig)
	if err != nil {
		return nil, err
	}

	return GetClosestP99Value(backendName, jobType.Release, jobType.FromRelease, jobType.Platform, jobType.Network, jobType.Topology), nil
}

func GetClosestP99Value(backendName, release, fromRelease, platform, networkType, topology string) *time.Duration {
	exactMatchKey := LastWeekPercentileKey{
		BackendName: backendName,
		Release:     release,
		FromRelease: fromRelease,
		Platform:    platform,
		Network:     networkType,
		Topology:    topology,
	}
	_, percentileAsMap := getCurrentResults()

	// chose so we can find them easily in the log
	defaultSeconds, err := time.ParseDuration("2.718s")
	if err != nil {
		panic(err)
	}

	if percentiles, ok := percentileAsMap[exactMatchKey]; ok {
		ret, err := time.ParseDuration(fmt.Sprintf("%2fs", percentiles.P99))
		if err != nil {
			return &defaultSeconds
		}
		return &ret
	}

	return &defaultSeconds
}

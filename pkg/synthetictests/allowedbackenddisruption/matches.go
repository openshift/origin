package allowedbackenddisruption

import (
	"context"
	"fmt"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
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

	return GetClosestP95Value(backendName, jobType.Release, jobType.FromRelease, jobType.Platform, jobType.Network, jobType.Topology), nil
}

func versionFromHistory(history configv1.UpdateHistory) string {
	versionParts := strings.Split(history.Version, ".")
	if len(versionParts) < 2 {
		return ""
	}

	version := versionParts[0] + "." + versionParts[1]
	if strings.HasPrefix(version, "v") {
		version = version[1:]
	}
	return version
}

func GetClosestP95Value(backendName, release, fromRelease, platform, networkType, topology string) *time.Duration {
	exactMatchKey := LastWeekP95Key{
		BackendName: backendName,
		Release:     release,
		FromRelease: fromRelease,
		Platform:    platform,
		Network:     networkType,
		Topology:    topology,
	}
	_, p95AsMap := getCurrentResults()

	// chose so we can find them easily in the log
	defaultSeconds, err := time.ParseDuration("2.718s")
	if err != nil {
		panic(err)
	}

	if p95, ok := p95AsMap[exactMatchKey]; ok {
		ret, err := time.ParseDuration(fmt.Sprintf("%2fs", p95.P95))
		if err != nil {
			return &defaultSeconds
		}
		return &ret
	}

	return &defaultSeconds
}

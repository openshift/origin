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
func GetAllowedDisruption(ctx context.Context, backendName string, clientConfig *rest.Config) (*time.Duration, string, error) {
	jobType, err := platformidentification.GetJobType(ctx, clientConfig)
	if err != nil {
		return nil, "", err
	}

	return GetClosestP99Value(backendName, *jobType),
		fmt.Sprintf("jobType=%#v", jobType),
		nil
}

func GetClosestP99Value(backendName string, jobType platformidentification.JobType) *time.Duration {
	exactMatchKey := LastWeekPercentileKey{
		BackendName: backendName,
		JobType:     jobType,
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

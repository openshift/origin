package allowedalerts

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/synthetictests/historicaldata"
	"github.com/stretchr/testify/assert"

	"github.com/openshift/origin/pkg/synthetictests/platformidentification"
)

func TestGetClosestP95Value(t *testing.T) {

	mustDuration := func(durationString string) *time.Duration {
		ret, err := time.ParseDuration(durationString)
		if err != nil {
			panic(err)
		}
		return &ret
	}

	historicalData := []historicaldata.StatisticalData{
		{
			DataKey: historicaldata.DataKey{
				Name: "etcdGRPCRequestsSlow",
				JobType: platformidentification.JobType{
					Release:      "4.12",
					FromRelease:  "4.12",
					Platform:     "gcp",
					Architecture: "amd64",
					Network:      "sdn",
					Topology:     "ha",
				},
			},
			P95: 4.8,
			P99: 7.9,
		},
		{
			DataKey: historicaldata.DataKey{
				Name: "etcdGRPCRequestsSlow",
				JobType: platformidentification.JobType{
					Release:      "4.12",
					FromRelease:  "4.12",
					Platform:     "aws",
					Architecture: "amd64",
					Network:      "sdn",
					Topology:     "ha",
				},
			},
			P95: 50.827,
			P99: 120.458,
		},
	}

	// Convert our slice of statistical data to a map on datakey to match what the matcher needs.
	// This allows us to define our dest data without duplicating the DataKey struct.
	historicalDataMap := map[historicaldata.DataKey]historicaldata.StatisticalData{}
	for _, hd := range historicalData {
		historicalDataMap[hd.DataKey] = hd
	}

	tests := []struct {
		name             string
		alertName        string
		jobType          platformidentification.JobType
		expectedDuration *time.Duration
	}{
		{
			name:      "direct match",
			alertName: "etcdGRPCRequestsSlow",
			jobType: platformidentification.JobType{
				Release:      "4.12",
				FromRelease:  "4.12",
				Platform:     "gcp",
				Architecture: "amd64",
				Network:      "sdn",
				Topology:     "ha",
			},
			expectedDuration: mustDuration("7.9s"),
		},
		{
			name:      "choose different arch",
			alertName: "etcdGRPCRequestsSlow",
			jobType: platformidentification.JobType{
				Release:      "4.12",
				FromRelease:  "4.12",
				Platform:     "aws",
				Architecture: "not-real",
				Network:      "sdn",
				Topology:     "ha",
			},
			expectedDuration: mustDuration("120.458s"),
		},
		{
			name:      "missing",
			alertName: "notARealAlert",
			jobType: platformidentification.JobType{
				Release:      "4.10",
				FromRelease:  "4.10",
				Platform:     "azure",
				Architecture: "amd64",
				Topology:     "missing",
			},
			expectedDuration: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := historicaldata.NewMatcherWithHistoricalData(historicalDataMap, 3.141)
			actualDuration, _, actualErr := matcher.BestMatchP99(tt.alertName, tt.jobType)
			assert.Nil(t, actualErr)
			assert.EqualValues(t, tt.expectedDuration, actualDuration, "unexpected duration")
		})
	}
}

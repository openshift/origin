package allowedalerts

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/synthetictests/historicaldata"
	"github.com/stretchr/testify/assert"

	"github.com/openshift/origin/pkg/synthetictests/platformidentification"
)

func TestGetClosestP99Value(t *testing.T) {

	mustDuration := func(durationString string) *time.Duration {
		ret, err := time.ParseDuration(durationString)
		if err != nil {
			panic(err)
		}
		return &ret
	}

	historicalData := []historicaldata.AlertStatisticalData{
		{
			AlertDataKey: historicaldata.AlertDataKey{
				AlertName:      "etcdGRPCRequestsSlow",
				AlertNamespace: "",
				AlertLevel:     "warning",
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
			AlertDataKey: historicaldata.AlertDataKey{
				AlertName:      "etcdGRPCRequestsSlow",
				AlertNamespace: "",
				AlertLevel:     "warning",
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
	historicalDataMap := map[historicaldata.AlertDataKey]historicaldata.AlertStatisticalData{}
	for _, hd := range historicalData {
		historicalDataMap[hd.AlertDataKey] = hd
	}

	tests := []struct {
		name             string
		key              historicaldata.AlertDataKey
		expectedDuration *time.Duration
	}{
		{
			name: "direct match",
			key: historicaldata.AlertDataKey{
				AlertName:      "etcdGRPCRequestsSlow",
				AlertNamespace: "",
				AlertLevel:     "warning",
				JobType: platformidentification.JobType{
					Release:      "4.12",
					FromRelease:  "4.12",
					Platform:     "gcp",
					Architecture: "amd64",
					Network:      "sdn",
					Topology:     "ha",
				},
			},
			expectedDuration: mustDuration("7.9s"),
		},
		{
			name: "choose different arch",
			key: historicaldata.AlertDataKey{
				AlertName:      "etcdGRPCRequestsSlow",
				AlertNamespace: "",
				AlertLevel:     "warning",
				JobType: platformidentification.JobType{
					Release:      "4.12",
					FromRelease:  "4.12",
					Platform:     "aws",
					Architecture: "not-real",
					Network:      "sdn",
					Topology:     "ha",
				},
			},
			expectedDuration: mustDuration("120.458s"),
		},
		{
			name: "missing",
			key: historicaldata.AlertDataKey{
				AlertName:      "notARealAlert",
				AlertNamespace: "",
				AlertLevel:     "warning",
				JobType: platformidentification.JobType{
					Release:      "4.10",
					FromRelease:  "4.10",
					Platform:     "azure",
					Architecture: "amd64",
					Topology:     "missing",
				},
			},
			expectedDuration: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := historicaldata.NewAlertMatcherWithHistoricalData(historicalDataMap)
			actualDuration, _, actualErr := matcher.BestMatchP99(tt.key)
			assert.Nil(t, actualErr)
			assert.EqualValues(t, tt.expectedDuration, actualDuration, "unexpected duration")
		})
	}
}

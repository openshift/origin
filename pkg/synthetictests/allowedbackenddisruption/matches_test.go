package allowedbackenddisruption

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/synthetictests/historicaldata"
	"github.com/openshift/origin/pkg/synthetictests/platformidentification"
	"github.com/stretchr/testify/assert"
)

func TestGetClosestP95Value(t *testing.T) {

	mustDuration := func(durationString string) *time.Duration {
		ret, err := time.ParseDuration(durationString)
		if err != nil {
			panic(err)
		}
		return &ret
	}
	type args struct {
		backendName string
		jobType     platformidentification.JobType
	}

	historicalData := []historicaldata.StatisticalData{
		{
			DataKey: historicaldata.DataKey{
				Name: "kube-api-new-connections",
				JobType: platformidentification.JobType{
					Release:      "4.12",
					FromRelease:  "4.11",
					Platform:     "aws",
					Architecture: "amd64",
					Network:      "ovn",
					Topology:     "ha",
				},
			},
			P95: 1.578,
			P99: 2.987,
		},
		{
			DataKey: historicaldata.DataKey{
				Name: "kube-api-new-connections",
				JobType: platformidentification.JobType{
					Release:      "4.12",
					FromRelease:  "4.12",
					Platform:     "aws",
					Architecture: "amd64",
					Network:      "sdn",
					Topology:     "single",
				},
			},
			P95: 50.827,
			P99: 120.458,
		},
		{
			DataKey: historicaldata.DataKey{
				Name: "openshift-api-new-connections",
				JobType: platformidentification.JobType{
					Release:      "4.11",
					FromRelease:  "4.11",
					Platform:     "aws",
					Architecture: "amd64",
					Network:      "sdn",
					Topology:     "single",
				},
			},
			P95: 49.419,
			P99: 70.381,
		},
		{
			DataKey: historicaldata.DataKey{
				Name: "oauth-api-new-connections",
				JobType: platformidentification.JobType{
					Release:      "4.12",
					FromRelease:  "4.11",
					Platform:     "aws",
					Architecture: "amd64",
					Network:      "sdn",
					Topology:     "single",
				},
			},
			P95: 20.714,
			P99: 35.917,
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
		args             args
		expectedDuration *time.Duration
		expectedErr      error
	}{

		// Be sure to use unique values here for expectedDuration to ensure we got the correct fallback.
		{
			name: "direct match",
			args: args{
				backendName: "kube-api-new-connections",
				jobType: platformidentification.JobType{
					Release:      "4.12",
					FromRelease:  "4.11",
					Platform:     "aws",
					Architecture: "amd64",
					Network:      "ovn",
					Topology:     "ha",
				},
			},
			expectedDuration: mustDuration("2.987s"),
		},
		{
			name: "fuzzy match fallback to minor upgrade",
			args: args{
				backendName: "kube-api-new-connections",
				jobType: platformidentification.JobType{
					Release:      "4.12",
					FromRelease:  "4.12",
					Platform:     "aws",
					Architecture: "amd64",
					Network:      "ovn",
					Topology:     "ha",
				},
			},
			expectedDuration: mustDuration("2.987s"),
		},
		{
			name: "fuzzy match single ovn fallback to sdn",
			args: args{
				backendName: "kube-api-new-connections",
				jobType: platformidentification.JobType{
					Release:      "4.12",
					FromRelease:  "4.12",
					Platform:     "aws",
					Architecture: "amd64",
					Network:      "ovn", // we only defined sdn above, should fall back to it's value
					Topology:     "single",
				},
			},
			expectedDuration: mustDuration("120.458s"),
		},
		{
			name: "fuzzy match single ovn fallback to sdn previous",
			args: args{
				// switching to openshift-api backend, defined above we only have from 4.11->4.11 and sdn
				backendName: "openshift-api-new-connections",
				jobType: platformidentification.JobType{
					Release:      "4.12",
					FromRelease:  "4.12",
					Platform:     "aws",
					Network:      "ovn",
					Architecture: "amd64",
					Topology:     "single",
				},
			},
			expectedDuration: mustDuration("70.381s"),
		},
		{
			name: "fuzzy match single ovn fallback to sdn previous micro",
			args: args{
				// For oauth backend ovn single don't have 4.12 minor, but we have 4.11->4.12 sdn above:
				backendName: "oauth-api-new-connections",
				jobType: platformidentification.JobType{
					Release:      "4.12",
					FromRelease:  "4.11",
					Platform:     "aws",
					Network:      "ovn",
					Architecture: "amd64",
					Topology:     "single",
				},
			},
			expectedDuration: mustDuration("35.917s"),
		},
		{
			name: "no exact or fuzzy match",
			args: args{
				backendName: "kube-api-reused-connections",
				jobType: platformidentification.JobType{
					Release:      "4.12",
					FromRelease:  "4.12",
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
			matcher := historicaldata.NewMatcherWithHistoricalData(historicalDataMap, 3.141)
			actualDuration, _, actualErr := matcher.BestMatchP99(tt.args.backendName, tt.args.jobType)
			assert.EqualValues(t, tt.expectedDuration, actualDuration, "unexpected duration")
			assert.Equal(t, tt.expectedErr, actualErr, "unexpected error")
		})
	}
}

package allowedalerts

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitortestlibrary/historicaldata"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"

	"github.com/stretchr/testify/assert"
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
			P95:     4.8,
			P99:     7.9,
			JobRuns: 1000,
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
			P95:     50.827,
			P99:     120.458,
			JobRuns: 1000,
		},
		{
			AlertDataKey: historicaldata.AlertDataKey{
				AlertName:      "etcdGRPCRequestsSlow",
				AlertNamespace: "",
				AlertLevel:     "warning",
				JobType: platformidentification.JobType{
					Release:      "4.13",
					FromRelease:  "4.12",
					Platform:     "azure",
					Architecture: "amd64",
					Network:      "sdn",
					Topology:     "ha",
				},
			},
			P95:     20.100,
			P99:     24.938,
			JobRuns: 10, // should get ignored, not min 100 results
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
		{
			name: "skip if insufficient data",
			key: historicaldata.AlertDataKey{
				AlertName:      "etcdGRPCRequestsSlow",
				AlertNamespace: "",
				AlertLevel:     "warning",
				JobType: platformidentification.JobType{
					Release:      "4.13",
					FromRelease:  "4.12",
					Platform:     "azure",
					Network:      "sdn",
					Architecture: "amd64",
					Topology:     "ha",
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

// TestAlertDataFileParsing uses the actual query_results.json data file we populate weekly
// from bigquery and commit into origin. Test ensures we can parse it and the data looks sane.
func TestAlertDataFileParsing(t *testing.T) {

	alertMatcher := getCurrentResults()

	// The list of known alerts that goes into this file is composed of everything we've ever
	// seen fire in that release. As such it can change from one release to the next as alerts
	// are added or removed, or just don't happen to fire.
	//
	// To test here we're really just looking for *something* to indicate we have valid data.

	var dataOver100Runs int
	var foundAWSOVN bool
	var foundAzureOVN bool
	var foundGCPOVN bool
	var foundMetalOVN bool

	releasesInQueryResults := map[string]bool{}
	var currentRelease string // track the one release we find

	for _, v := range alertMatcher.HistoricalData {
		if v.JobRuns > 100 {
			dataOver100Runs++
		}
		releasesInQueryResults[v.Release] = true
		currentRelease = v.Release

		if v.Platform == "aws" && v.Network == "ovn" && v.Architecture == "amd64" {
			foundAWSOVN = true
		}
		if v.Platform == "azure" && v.Network == "ovn" && v.Architecture == "amd64" {
			foundAzureOVN = true
		}
		if v.Platform == "gcp" && v.Network == "ovn" && v.Architecture == "amd64" {
			foundGCPOVN = true
		}
		if v.Platform == "metal" && v.Network == "ovn" && v.Architecture == "amd64" {
			foundMetalOVN = true
		}
	}
	currentRelease = historicaldata.CurrentReleaseFromMap(releasesInQueryResults)
	assert.Greater(t, dataOver100Runs, 5,
		"expected at least 5 entries in query_results.json to have over 100 runs")
	assert.True(t, foundAWSOVN, "no aws ovn job data in query_results.json")
	assert.True(t, foundGCPOVN, "no gcp ovn job data in query_results.json")
	assert.True(t, foundAzureOVN, "no azure ovn job data in query_results.json")
	assert.True(t, foundMetalOVN, "no metal ovn job data in query_results.json")
	assert.Equal(t, 2, len(releasesInQueryResults),
		"expected only one Release in query_results.json")

	// Check that we get a real value for something we know should be there for every release. This alert
	// always fires throughout the entire CI run:
	expectedKey := historicaldata.AlertDataKey{
		AlertName:      "AlertmanagerReceiversNotConfigured",
		AlertNamespace: "openshift-monitoring",
		AlertLevel:     "Warning",
		JobType: platformidentification.JobType{
			Release:      currentRelease,
			FromRelease:  currentRelease,
			Platform:     "aws",
			Architecture: "amd64",
			Network:      "ovn",
			Topology:     "ha",
		},
	}
	hd, msg, err := alertMatcher.BestMatchDuration(expectedKey)
	assert.True(t, hd.P99 > 5*time.Minute, "AlertmanagerReceiversNotConfigured data not present for aws amd64 ovn ha")
	assert.Equal(t, "", msg)
	assert.NoError(t, err)

}

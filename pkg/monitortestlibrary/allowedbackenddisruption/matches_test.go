package allowedbackenddisruption

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitortestlibrary/historicaldata"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"

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

	historicalData := []historicaldata.DisruptionStatisticalData{
		{
			DataKey: historicaldata.DataKey{
				BackendName: "kube-api-new-connections",
				JobType: platformidentification.JobType{
					Release:      "4.12",
					FromRelease:  "4.11",
					Platform:     "aws",
					Architecture: "amd64",
					Network:      "ovn",
					Topology:     "ha",
				},
			},
			P95:     1.578,
			P99:     2.987,
			JobRuns: 1000,
		},
		{
			DataKey: historicaldata.DataKey{
				BackendName: "kube-api-new-connections",
				JobType: platformidentification.JobType{
					Release:      "4.12",
					FromRelease:  "4.12",
					Platform:     "aws",
					Architecture: "amd64",
					Network:      "sdn",
					Topology:     "single",
				},
			},
			P95:     50.827,
			P99:     120.458,
			JobRuns: 1000,
		},
		{
			DataKey: historicaldata.DataKey{
				BackendName: "openshift-api-new-connections",
				JobType: platformidentification.JobType{
					Release:      "4.11",
					FromRelease:  "4.11",
					Platform:     "aws",
					Architecture: "amd64",
					Network:      "sdn",
					Topology:     "single",
				},
			},
			P95:     49.419,
			P99:     70.381,
			JobRuns: 1000,
		},
		{
			DataKey: historicaldata.DataKey{
				BackendName: "oauth-api-new-connections",
				JobType: platformidentification.JobType{
					Release:      "4.12",
					FromRelease:  "4.11",
					Platform:     "aws",
					Architecture: "amd64",
					Network:      "sdn",
					Topology:     "single",
				},
			},
			P95:     20.714,
			P99:     35.917,
			JobRuns: 1000,
		},
		{
			DataKey: historicaldata.DataKey{
				BackendName: "image-registry-new-connections",
				JobType: platformidentification.JobType{
					Release:      "4.13",
					FromRelease:  "4.12",
					Platform:     "aws",
					Architecture: "amd64",
					Network:      "sdn",
					Topology:     "single",
				},
			},
			P95:     20.714,
			P99:     35.917,
			JobRuns: 99, // not enough to count
		},
	}

	// Convert our slice of statistical data to a map on datakey to match what the matcher needs.
	// This allows us to define our dest data without duplicating the DataKey struct.
	historicalDataMap := map[historicaldata.DataKey]historicaldata.DisruptionStatisticalData{}
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
		{
			name: "direct match but insufficient job runs",
			args: args{
				backendName: "image-registry-new-connections",
				jobType: platformidentification.JobType{
					Release:      "4.13",
					FromRelease:  "4.12",
					Platform:     "aws",
					Architecture: "amd64",
					Network:      "sdn",
					Topology:     "single",
				},
			},
			expectedDuration: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := historicaldata.NewDisruptionMatcherWithHistoricalData(historicalDataMap)
			actualDuration, _, actualErr := matcher.BestMatchP99(tt.args.backendName, tt.args.jobType)
			assert.EqualValues(t, tt.expectedDuration, actualDuration, "unexpected duration")
			assert.Equal(t, tt.expectedErr, actualErr, "unexpected error")
		})
	}
}

// TestDisruptionDataFileParsing uses the actual query_results.json data file we populate weekly
// from bigquery and commit into origin. Test ensures we can parse it and the data looks sane.
func TestDisruptionDataFileParsing(t *testing.T) {

	disruptionMatcher := getCurrentResults()

	var dataOver100Runs int
	var foundAWSOVN bool
	var foundAzureOVN bool
	var foundGCPOVN bool
	var foundMetalOVN bool

	releasesInQueryResults := map[string]bool{}
	var currentRelease string // track the one release we find

	for _, v := range disruptionMatcher.HistoricalData {
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

	// Check that we get a real value for something we know should be there for every release.
	jobType := platformidentification.JobType{
		Release:      currentRelease,
		FromRelease:  currentRelease,
		Platform:     "aws",
		Architecture: "amd64",
		Network:      "ovn",
		Topology:     "ha",
	}

	_, msg, err := disruptionMatcher.BestMatchDuration("kube-api-new-connections", jobType)
	// We can't really check a value here as it could very likely be 0,
	// so instead we'll make sure we didn't get a msg complaining about no match:
	assert.Equal(t, "", msg, "BestMatchDuration reported a problem finding data for kube-api-new-connections aws amd64 ovn ha")
	assert.NoError(t, err)
}

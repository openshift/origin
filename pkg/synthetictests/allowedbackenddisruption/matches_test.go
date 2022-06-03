package allowedbackenddisruption

import (
	"reflect"
	"testing"
	"time"

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
	type args struct {
		backendName string
		jobType     platformidentification.JobType
	}
	tests := []struct {
		name             string
		args             args
		expectedDuration *time.Duration
		expectedDetails  string
		expectedErr      error
	}{
		{
			name: "test-that-failed-in-ci",
			args: args{
				backendName: "ingress-to-oauth-server-reused-connections",
				jobType: platformidentification.JobType{
					Release:      "4.10",
					FromRelease:  "4.10",
					Platform:     "gcp",
					Architecture: "amd64",
					Network:      "sdn",
					Topology:     "ha",
				},
			},
			expectedDuration: mustDuration("14.03s"),
		},
		{
			name: "fuzzy-match",
			args: args{
				backendName: "ingress-to-oauth-server-reused-connections",
				jobType: platformidentification.JobType{
					Release:      "4.11",
					FromRelease:  "4.11",
					Platform:     "azure",
					Architecture: "amd64",
					Network:      "sdn",
					Topology:     "ha",
				},
			},
			expectedDuration: mustDuration("2.3s"),
			expectedDetails:  `(no exact match for historicaldata.DataKey{Name:"ingress-to-oauth-server-reused-connections", JobType:platformidentification.JobType{Release:"4.11", FromRelease:"4.11", Platform:"azure", Architecture:"amd64", Network:"sdn", Topology:"ha"}}, fell back to historicaldata.DataKey{Name:"ingress-to-oauth-server-reused-connections", JobType:platformidentification.JobType{Release:"4.11", FromRelease:"4.10", Platform:"azure", Architecture:"amd64", Network:"sdn", Topology:"ha"}})`,
		},
		{
			name: "fuzzy-match-single-ovn-on-sdn",
			args: args{
				backendName: "image-registry-reused-connections",
				jobType: platformidentification.JobType{
					Release:      "4.10",
					FromRelease:  "4.10",
					Platform:     "aws",
					Network:      "ovn",
					Architecture: "amd64",
					Topology:     "single",
				},
			},
			expectedDuration: mustDuration("3m54.16s"),
			expectedDetails:  `(no exact match for historicaldata.DataKey{Name:"image-registry-reused-connections", JobType:platformidentification.JobType{Release:"4.10", FromRelease:"4.10", Platform:"aws", Architecture:"amd64", Network:"ovn", Topology:"single"}}, fell back to historicaldata.DataKey{Name:"image-registry-reused-connections", JobType:platformidentification.JobType{Release:"4.10", FromRelease:"4.10", Platform:"aws", Architecture:"amd64", Network:"sdn", Topology:"single"}})`,
		},
		{
			name: "fuzzy-match-single-ovn-on-sdn-previous",
			args: args{
				backendName: "image-registry-new-connections",
				jobType: platformidentification.JobType{
					Release:      "4.11",
					FromRelease:  "4.11",
					Platform:     "aws",
					Network:      "ovn",
					Architecture: "amd64",
					Topology:     "single",
				},
			},
			expectedDuration: mustDuration("3m54.28s"),
			expectedDetails:  `(no exact match for historicaldata.DataKey{Name:"image-registry-new-connections", JobType:platformidentification.JobType{Release:"4.11", FromRelease:"4.11", Platform:"aws", Architecture:"amd64", Network:"ovn", Topology:"single"}}, fell back to historicaldata.DataKey{Name:"image-registry-new-connections", JobType:platformidentification.JobType{Release:"4.10", FromRelease:"4.10", Platform:"aws", Architecture:"amd64", Network:"sdn", Topology:"single"}})`,
		},
		{
			name: "fuzzy-match-single-ovn-on-sdn-previous-micro-release",
			args: args{
				backendName: "ingress-to-console-new-connections",
				jobType: platformidentification.JobType{
					Release:      "4.11",
					FromRelease:  "4.10",
					Platform:     "aws",
					Network:      "ovn",
					Architecture: "amd64",
					Topology:     "single",
				},
			},
			expectedDuration: mustDuration("560.0s"),
			expectedDetails:  `(no exact match for historicaldata.DataKey{Name:"ingress-to-console-new-connections", JobType:platformidentification.JobType{Release:"4.11", FromRelease:"4.10", Platform:"aws", Architecture:"amd64", Network:"ovn", Topology:"single"}}, fell back to historicaldata.DataKey{Name:"ingress-to-console-new-connections", JobType:platformidentification.JobType{Release:"4.10", FromRelease:"4.10", Platform:"aws", Architecture:"amd64", Network:"sdn", Topology:"single"}})`,
		},
		{
			name: "missing",
			args: args{
				backendName: "kube-api-reused-connections",
				jobType: platformidentification.JobType{
					Release:      "4.10",
					FromRelease:  "4.10",
					Platform:     "azure",
					Architecture: "amd64",
					Topology:     "missing",
				},
			},
			expectedDuration: mustDuration("2.718s"),
			expectedDetails:  `(no exact or fuzzy match for jobType=platformidentification.JobType{Release:"4.10", FromRelease:"4.10", Platform:"azure", Architecture:"amd64", Network:"", Topology:"missing"})`,
		},
		// request is azure 4.11 from 4.10 without network, falls back to 4.10 from 4.10 sdn
		{
			name: "fuzzy-match-single-image-registry-new-connections",
			args: args{
				backendName: "image-registry-new-connections",
				jobType: platformidentification.JobType{
					Release:      "4.11",
					FromRelease:  "4.10",
					Platform:     "azure",
					Architecture: "amd64",
					Topology:     "single",
				},
			},
			expectedDuration: mustDuration("250.76s"),
			expectedDetails:  `(no exact match for historicaldata.DataKey{Name:"image-registry-new-connections", JobType:platformidentification.JobType{Release:"4.11", FromRelease:"4.10", Platform:"azure", Architecture:"amd64", Network:"", Topology:"single"}}, fell back to historicaldata.DataKey{Name:"image-registry-new-connections", JobType:platformidentification.JobType{Release:"4.10", FromRelease:"4.10", Platform:"azure", Architecture:"amd64", Network:"sdn", Topology:"single"}})`,
		},
		// request is azure 4.11 from 4.10 sdn, falls back to 4.11 from 4.11 sdn
		{
			name: "fuzzy-match-single-image-registry-new-connections",
			args: args{
				backendName: "image-registry-new-connections",
				jobType: platformidentification.JobType{
					Release:      "4.11",
					FromRelease:  "4.10",
					Platform:     "azure",
					Network:      "sdn",
					Architecture: "amd64",
					Topology:     "single",
				},
			},
			expectedDuration: mustDuration("246.9s"),
			expectedDetails:  `(no exact match for historicaldata.DataKey{Name:"image-registry-new-connections", JobType:platformidentification.JobType{Release:"4.11", FromRelease:"4.10", Platform:"azure", Architecture:"amd64", Network:"sdn", Topology:"single"}}, fell back to historicaldata.DataKey{Name:"image-registry-new-connections", JobType:platformidentification.JobType{Release:"4.11", FromRelease:"4.11", Platform:"azure", Architecture:"amd64", Network:"sdn", Topology:"single"}})`,
		},
		// request is azure 4.11 from 4.10 ovn, falls back to 4.10 from 4.10 sdn
		{
			name: "fuzzy-match-single-image-registry-new-connections",
			args: args{
				backendName: "image-registry-new-connections",
				jobType: platformidentification.JobType{
					Release:      "4.11",
					FromRelease:  "4.10",
					Platform:     "azure",
					Network:      "ovn",
					Architecture: "amd64",
					Topology:     "single",
				},
			},
			expectedDuration: mustDuration("250.76s"),
			expectedDetails:  `(no exact match for historicaldata.DataKey{Name:"image-registry-new-connections", JobType:platformidentification.JobType{Release:"4.11", FromRelease:"4.10", Platform:"azure", Architecture:"amd64", Network:"ovn", Topology:"single"}}, fell back to historicaldata.DataKey{Name:"image-registry-new-connections", JobType:platformidentification.JobType{Release:"4.10", FromRelease:"4.10", Platform:"azure", Architecture:"amd64", Network:"sdn", Topology:"single"}})`,
		},
		// request is aws 4.11 from 4.10 ovn, falls back to 4.10 from 4.10 sdn
		{
			name: "fuzzy-match-single-image-registry-new-connections",
			args: args{
				backendName: "image-registry-new-connections",
				jobType: platformidentification.JobType{
					Release:      "4.11",
					FromRelease:  "4.10",
					Platform:     "aws",
					Network:      "ovn",
					Architecture: "amd64",
					Topology:     "single",
				},
			},
			expectedDuration: mustDuration("234.28s"),
			expectedDetails:  `(no exact match for historicaldata.DataKey{Name:"image-registry-new-connections", JobType:platformidentification.JobType{Release:"4.11", FromRelease:"4.10", Platform:"aws", Architecture:"amd64", Network:"ovn", Topology:"single"}}, fell back to historicaldata.DataKey{Name:"image-registry-new-connections", JobType:platformidentification.JobType{Release:"4.10", FromRelease:"4.10", Platform:"aws", Architecture:"amd64", Network:"sdn", Topology:"single"}})`,
		},
		// request is for aws 4.11 from 4.10 sdn, falls back to 4.10 from 4.10 sdn
		{
			name: "fuzzy-match-single-image-registry-new-connections",
			args: args{
				backendName: "image-registry-new-connections",
				jobType: platformidentification.JobType{
					Release:      "4.11",
					FromRelease:  "4.10",
					Platform:     "aws",
					Network:      "sdn",
					Architecture: "amd64",
					Topology:     "single",
				},
			},
			expectedDuration: mustDuration("234.28s"),
			expectedDetails:  `(no exact match for historicaldata.DataKey{Name:"image-registry-new-connections", JobType:platformidentification.JobType{Release:"4.11", FromRelease:"4.10", Platform:"aws", Architecture:"amd64", Network:"sdn", Topology:"single"}}, fell back to historicaldata.DataKey{Name:"image-registry-new-connections", JobType:platformidentification.JobType{Release:"4.10", FromRelease:"4.10", Platform:"aws", Architecture:"amd64", Network:"sdn", Topology:"single"}})`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualDuration, actualDetails, actualErr := GetAllowedDisruption(tt.args.backendName, tt.args.jobType)
			if got, want := actualDuration, tt.expectedDuration; !reflect.DeepEqual(got, want) {
				t.Errorf("GetClosestP99Value() = %v, want %v", got, want)
			}
			if got, want := actualDetails, tt.expectedDetails; !reflect.DeepEqual(got, want) {
				t.Errorf("GetClosestP99Value() = %v, want %v", got, want)
			}
			if got, want := actualErr, tt.expectedErr; !reflect.DeepEqual(got, want) {
				t.Errorf("GetClosestP99Value() = %v, want %v", got, want)
			}
		})
	}
}

func TestSingleGetClosestP95Value(t *testing.T) {

	// isolate a single case for debug / validation
	// change as needed

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
	tests := []struct {
		name             string
		args             args
		expectedDuration *time.Duration
		expectedDetails  string
		expectedErr      error
	}{

		{
			name: "fuzzy-match-single-image-registry-new-connections",
			args: args{
				backendName: "image-registry-new-connections",
				jobType: platformidentification.JobType{
					Release:      "4.11",
					FromRelease:  "4.10",
					Platform:     "aws",
					Network:      "sdn",
					Architecture: "amd64",
					Topology:     "single",
				},
			},
			expectedDuration: mustDuration("234.28s"),
			expectedDetails:  `(no exact match for historicaldata.DataKey{Name:"image-registry-new-connections", JobType:platformidentification.JobType{Release:"4.11", FromRelease:"4.10", Platform:"aws", Architecture:"amd64", Network:"sdn", Topology:"single"}}, fell back to historicaldata.DataKey{Name:"image-registry-new-connections", JobType:platformidentification.JobType{Release:"4.10", FromRelease:"4.10", Platform:"aws", Architecture:"amd64", Network:"sdn", Topology:"single"}})`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualDuration, actualDetails, actualErr := GetAllowedDisruption(tt.args.backendName, tt.args.jobType)
			if got, want := actualDuration, tt.expectedDuration; !reflect.DeepEqual(got, want) {
				t.Errorf("GetClosestP99Value() = %v, want %v", got, want)
			}
			if got, want := actualDetails, tt.expectedDetails; !reflect.DeepEqual(got, want) {
				t.Errorf("GetClosestP99Value() = %v, want %v", got, want)
			}
			if got, want := actualErr, tt.expectedErr; !reflect.DeepEqual(got, want) {
				t.Errorf("GetClosestP99Value() = %v, want %v", got, want)
			}
		})
	}
}

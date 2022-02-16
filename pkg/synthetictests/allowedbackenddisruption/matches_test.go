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
			expectedDuration: mustDuration("4s"),
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
			expectedDuration: mustDuration("12.87s"),
			expectedDetails:  `(no exact match for historicaldata.DataKey{Name:"ingress-to-oauth-server-reused-connections", JobType:platformidentification.JobType{Release:"4.11", FromRelease:"4.11", Platform:"azure", Network:"sdn", Topology:"ha"}}, fell back to historicaldata.DataKey{Name:"ingress-to-oauth-server-reused-connections", JobType:platformidentification.JobType{Release:"4.11", FromRelease:"4.10", Platform:"azure", Network:"sdn", Topology:"ha"}})`,
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
			expectedDetails:  `(no exact or fuzzy match for jobType=platformidentification.JobType{Release:"4.10", FromRelease:"4.10", Platform:"azure", Network:"", Topology:"missing"})`,
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

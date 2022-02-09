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
					Release:     "4.10",
					FromRelease: "4.10",
					Platform:    "gcp",
					Network:     "sdn",
					Topology:    "ha",
				},
			},
			expectedDuration: mustDuration("4s"),
		},
		{
			name: "missing",
			args: args{
				backendName: "kube-api-reused-connections",
				jobType: platformidentification.JobType{
					Release:     "4.10",
					FromRelease: "4.10",
					Platform:    "azure",
					Topology:    "missing",
				},
			},
			expectedDuration: mustDuration("2.718s"),
			expectedDetails:  `jobType=platformidentification.JobType{Release:"4.10", FromRelease:"4.10", Platform:"azure", Network:"", Topology:"missing"}`,
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

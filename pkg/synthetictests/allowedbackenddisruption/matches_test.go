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
		name string
		args args
		want *time.Duration
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
			want: mustDuration("4s"),
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
			want: mustDuration("2.718s"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetClosestP99Value(tt.args.backendName, tt.args.jobType); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetClosestP99Value() = %v, want %v", got, tt.want)
			}
		})
	}
}

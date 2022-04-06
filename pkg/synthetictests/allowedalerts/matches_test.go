package allowedalerts

import (
	"reflect"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/synthetictests/historicaldata"

	"github.com/openshift/origin/pkg/synthetictests/platformidentification"
)

func TestGetClosestP95Value(t *testing.T) {

	mustDuration := func(durationString string) time.Duration {
		ret, err := time.ParseDuration(durationString)
		if err != nil {
			panic(err)
		}
		return ret
	}
	tests := []struct {
		name      string
		alertName string
		jobType   platformidentification.JobType
		want      historicaldata.StatisticalDuration
	}{
		{
			name:      "test-that-failed-in-ci",
			alertName: "etcdGRPCRequestsSlow",
			jobType: platformidentification.JobType{
				Release:      "4.10",
				FromRelease:  "4.10",
				Platform:     "gcp",
				Architecture: "amd64",
				Network:      "sdn",
				Topology:     "ha",
			},
			want: historicaldata.StatisticalDuration{
				DataKey: historicaldata.DataKey{
					Name: "etcdGRPCRequestsSlow",
					JobType: platformidentification.JobType{
						Release:      "4.10",
						FromRelease:  "4.10",
						Platform:     "gcp",
						Architecture: "amd64",
						Network:      "sdn",
						Topology:     "ha",
					},
				},

				P95: mustDuration("3s"),
				P99: mustDuration("4s"),
			},
		},
		{
			name:      "choose-different-arch",
			alertName: "etcdGRPCRequestsSlow",
			jobType: platformidentification.JobType{
				Release:      "4.10",
				FromRelease:  "4.10",
				Platform:     "gcp",
				Architecture: "not-real",
				Network:      "sdn",
				Topology:     "ha",
			},
			want: historicaldata.StatisticalDuration{
				DataKey: historicaldata.DataKey{
					Name: "etcdGRPCRequestsSlow",
					JobType: platformidentification.JobType{
						Release:      "4.10",
						FromRelease:  "4.10",
						Platform:     "gcp",
						Architecture: "amd64",
						Network:      "sdn",
						Topology:     "ha",
					},
				},

				P95: mustDuration("3s"),
				P99: mustDuration("4s"),
			},
		},
		{
			name:      "missing",
			alertName: "ingress-to-oauth-server-reused-connections",
			jobType: platformidentification.JobType{
				Release:      "4.10",
				FromRelease:  "4.10",
				Platform:     "azure",
				Architecture: "amd64",
				Topology:     "missing",
			},
			want: historicaldata.StatisticalDuration{
				DataKey: historicaldata.DataKey{
					Name: "ingress-to-oauth-server-reused-connections",
					JobType: platformidentification.JobType{
						Release:      "4.10",
						FromRelease:  "4.10",
						Platform:     "azure",
						Architecture: "amd64",
						Topology:     "missing",
					},
				},

				P95: mustDuration("3.141s"),
				P99: mustDuration("3.141s"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _, _ := getClosestPercentilesValues(tt.alertName, tt.jobType); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetClosestP99Value() = %v, want %v", got, tt.want)
			}
		})
	}
}

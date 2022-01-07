package allowedbackenddisruption

import (
	"reflect"
	"testing"
	"time"
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
		release     string
		fromRelease string
		platform    string
		networkType string
		topology    string
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
				release:     "4.10",
				fromRelease: "4.10",
				platform:    "gcp",
				networkType: "sdn",
				topology:    "ha",
			},
			want: mustDuration("2s"),
		},
		{
			name: "missing",
			args: args{
				backendName: "kube-api-reused-connections",
				release:     "4.10",
				fromRelease: "4.10",
				platform:    "azure",
				topology:    "missing",
			},
			want: mustDuration("2.718s"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetClosestP95Value(tt.args.backendName, tt.args.release, tt.args.fromRelease, tt.args.platform, tt.args.networkType, tt.args.topology); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetClosestP95Value() = %v, want %v", got, tt.want)
			}
		})
	}
}

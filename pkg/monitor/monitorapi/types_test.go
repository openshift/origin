package monitorapi

import (
	"testing"
	"time"
)

func TestIntervals_Duration(t *testing.T) {
	start1 := time.Now()
	start2 := start1.Add(1578 * time.Second)
	start3 := start2.Add(999 * time.Millisecond)
	start4 := start3.Add(26 * time.Second)
	start5 := start4.Add(1 * time.Second)
	start6 := start5.Add(29 * time.Second)
	start7 := start6.Add(999 * time.Millisecond)
	start8 := start7.Add(2030 * time.Second)

	type args struct {
		minDuration time.Duration
	}
	tests := []struct {
		name      string
		intervals Intervals
		args      args
		want      time.Duration
	}{
		{
			name: "about-three-seconds",
			intervals: Intervals{
				Interval{
					Condition: Condition{
						Level:   Info,
						Locator: "disruption/oauth-api connection/new disruption/oauth-api connection/new",
						Message: "started responding to GET requests over new connections",
					},
					From: start1,
					To:   start2,
				},
				Interval{
					Condition: Condition{
						Level:   Error,
						Locator: "disruption/oauth-api connection/new disruption/oauth-api connection/new",
						Message: "stopped responding to GET requests over new connections: Get \"https://api.ci-op-n37nl0in-c1303.ci2.azure.devcluster.openshift.com:6443/apis/oauth.openshift.io/v1/oauthclients\": context deadline exceeded",
					},
					From: start2,
					To:   start3,
				},
				Interval{
					Condition: Condition{
						Level:   Info,
						Locator: "disruption/oauth-api connection/new disruption/oauth-api connection/new",
						Message: "started responding to GET requests over new connections",
					},
					From: start3,
					To:   start4,
				},
				Interval{
					Condition: Condition{
						Level:   Error,
						Locator: "disruption/oauth-api connection/new disruption/oauth-api connection/new",
						Message: "stopped responding to GET requests over new connections: Get \"https://api.ci-op-n37nl0in-c1303.ci2.azure.devcluster.openshift.com:6443/apis/oauth.openshift.io/v1/oauthclients\": context deadline exceeded",
					},
					From: start4,
					To:   start5,
				},
				Interval{
					Condition: Condition{
						Level:   Info,
						Locator: "disruption/oauth-api connection/new disruption/oauth-api connection/new",
						Message: "started responding to GET requests over new connections",
					},
					From: start5,
					To:   start6,
				},
				Interval{
					Condition: Condition{
						Level:   Error,
						Locator: "disruption/oauth-api connection/new disruption/oauth-api connection/new",
						Message: "stopped responding to GET requests over new connections: Get \"https://api.ci-op-n37nl0in-c1303.ci2.azure.devcluster.openshift.com:6443/apis/oauth.openshift.io/v1/oauthclients\": context deadline exceeded",
					},
					From: start6,
					To:   start7,
				},
				Interval{
					Condition: Condition{
						Level:   Info,
						Locator: "disruption/oauth-api connection/new disruption/oauth-api connection/new",
						Message: "started responding to GET requests over new connections",
					},
					From: start7,
					To:   start8,
				},
			},
			args: args{
				minDuration: 1 * time.Second,
			},
			want: 3 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errorEvents := tt.intervals.Filter(IsErrorEvent)
			if got := errorEvents.Duration(tt.args.minDuration); got != tt.want {
				t.Errorf("Duration() = %v, want %v", got, tt.want)
			}
		})
	}
}

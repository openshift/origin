package intervalcreation

import (
	_ "embed"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
)

//go:embed render_test_01_skip_e2e.json
var skipE2e []byte

func TestBelongsInKubeAPIServer(t *testing.T) {
	inputIntervals, err := monitorserialization.EventsFromJSON(skipE2e)
	if err != nil {
		t.Fatal(err)
	}

	for _, event := range inputIntervals {
		t.Logf("%#v", event)

		if actual := isPlatformPodEvent(event); actual {
			t.Error("expected false")
		}
		if actual := BelongsInKubeAPIServer(event); actual {
			t.Error("expected false")
		}
	}
}

func TestIsPathologicalEvent(t *testing.T) {
	type args struct {
		eventInterval monitorapi.EventInterval
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"test true",
			args{
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: "ns/openshift-oauth-apiserver pod/apiserver-6b7664f788-cs9fs node/ci-op-65yws62g-aa502-59ww5-master-1",
						Message: "reason/ProbeError Readiness probe error: Get \"https://10.128.0.59:8443/readyz\": dial tcp 10.128.0.59:8443: connect: connection refused\nbody: \n (19 times)",
					},
					From: time.Now(),
					To:   time.Now().Add(time.Second),
				},
			},
			true,
		},
		{
			"test false",
			args{
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: "ns/openshift-oauth-apiserver pod/apiserver-6b7664f788-cs9fs node/ci-op-65yws62g-aa502-59ww5-master-1",
						Message: "reason/ProbeError Readiness probe error: Get \"https://10.128.0.59:8443/readyz\": dial tcp 10.128.0.59:8443: connect: connection refused",
					},
					From: time.Now(),
					To:   time.Now().Add(time.Second),
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPathologicalEvent(tt.args.eventInterval); got != tt.want {
				t.Errorf("IsPathologicalEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

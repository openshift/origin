package intervalcreation

import (
	"reflect"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
)

func TestIntervalsFromEvents_NodeChanges(t *testing.T) {
	intervals, err := monitorserialization.EventsFromFile("testdata/node.json")
	if err != nil {
		t.Fatal(err)
	}
	changes := IntervalsFromEvents_NodeChanges(intervals, nil, time.Time{}, time.Now())
	out, _ := monitorserialization.EventsIntervalsToJSON(changes)
	if len(changes) != 3 {
		t.Fatalf("unexpected changes: %s", string(out))
	}
	if changes[0].Message != "reason/NodeUpdate phase/Drain roles/worker drained node" {
		t.Errorf("unexpected event: %s", string(out))
	}
	if changes[1].Message != "reason/NodeUpdate phase/OperatingSystemUpdate roles/worker updated operating system" {
		t.Errorf("unexpected event: %s", string(out))
	}
	if changes[2].Message != "reason/NodeUpdate phase/Reboot roles/worker rebooted and kubelet started" {
		t.Errorf("unexpected event: %s", string(out))
	}
}

func Test_probeProblemToContainerReference(t *testing.T) {
	type args struct {
		logLine string
	}
	tests := []struct {
		name string
		args args
		want monitorapi.ContainerReference
	}{
		{
			name: "simple failure",
			args: args{
				logLine: `Jul 05 17:47:52.807876 ci-op-lxqqvl5x-d3bee-gl4hp-master-0 hyperkube[1495]: I0606 17:47:52.807876    1599 prober.go:121] "Probe failed" probeType="Readiness" pod="openshift-authentication/oauth-openshift-77f7b95df5-r4xf7" podUID=1af660b3-ac3a-4182-86eb-2f74725d8415 containerName="oauth-openshift" probeResult=failure output="Get \"https://10.129.0.12:6443/healthz\": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)"`,
			},
			want: monitorapi.ContainerReference{
				Pod: monitorapi.PodReference{
					NamespacedReference: monitorapi.NamespacedReference{
						Namespace: "openshift-authentication",
						Name:      "oauth-openshift-77f7b95df5-r4xf7",
						UID:       "1af660b3-ac3a-4182-86eb-2f74725d8415",
					},
				},
				ContainerName: "oauth-openshift",
			},
		},
		{
			name: "simple error",
			args: args{
				logLine: `Jul 05 17:43:12.908344 ci-op-lxqqvl5x-d3bee-gl4hp-master-0 hyperkube[1495]: E0606 17:43:12.908344    1500 prober.go:118] "Probe errored" err="rpc error: code = NotFound desc = container is not created or running: checking if PID of 645437acbb2ca429c04d5a2628924e2e10d44c681c824dddc7c82ffa30a936be is running failed: container process not found" probeType="Readiness" pod="openshift-marketplace/redhat-operators-4jpg4" podUID=0bac4741-a3bd-483c-b119-e97663d64024 containerName="registry-server"`,
			},
			want: monitorapi.ContainerReference{
				Pod: monitorapi.PodReference{
					NamespacedReference: monitorapi.NamespacedReference{
						Namespace: "openshift-marketplace",
						Name:      "redhat-operators-4jpg4",
						UID:       "0bac4741-a3bd-483c-b119-e97663d64024",
					},
				},
				ContainerName: "registry-server",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := probeProblemToContainerReference(tt.args.logLine); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("probeProblemToContainerReference() = %v, want %v", got, tt.want)
			}
		})
	}
}

func mustTime(in string) time.Time {
	ret, err := time.Parse("02 Jan 2006 15:04:05.999999999 MST", in)
	if err != nil {
		panic(err)
	}
	return ret
}

func Test_messageTime(t *testing.T) {
	type args struct {
		logLine string
	}
	tests := []struct {
		name string
		args args
		want time.Time
	}{
		{
			name: "simple failure",
			args: args{
				logLine: `Jul 05 17:47:52.807876 ci-op-lxqqvl5x-d3bee-gl4hp-master-0 hyperkube[1495]: I0606 17:47:52.807876    1599 prober.go:121] "Probe failed" probeType="Readiness" pod="openshift-authentication/oauth-openshift-77f7b95df5-r4xf7" podUID=1af660b3-ac3a-4182-86eb-2f74725d8415 containerName="oauth-openshift" probeResult=failure output="Get \"https://10.129.0.12:6443/healthz\": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)"`,
			},
			want: mustTime("05 Jul 2022 17:47:52.807876 UTC"),
		},
		{
			name: "simple error",
			args: args{
				logLine: `Jul 05 17:43:12.908344 ci-op-lxqqvl5x-d3bee-gl4hp-master-0 hyperkube[1495]: E0606 17:43:12.908344    1500 prober.go:118] "Probe errored" err="rpc error: code = NotFound desc = container is not created or running: checking if PID of 645437acbb2ca429c04d5a2628924e2e10d44c681c824dddc7c82ffa30a936be is running failed: container process not found" probeType="Readiness" pod="openshift-marketplace/redhat-operators-4jpg4" podUID=0bac4741-a3bd-483c-b119-e97663d64024 containerName="registry-server"`,
			},
			want: mustTime("05 Jul 2022 17:43:12.908344 UTC"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := kubeletLogTime(tt.args.logLine); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("kubeletLogTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_readinessFailure(t *testing.T) {
	type args struct {
		logLine string
	}
	tests := []struct {
		name string
		args args
		want monitorapi.Intervals
	}{
		{
			name: "simple failure",
			args: args{
				logLine: `Jul 05 17:47:52.807876 ci-op-lxqqvl5x-d3bee-gl4hp-master-0 hyperkube[1495]: I0606 17:47:52.807876    1599 prober.go:121] "Probe failed" probeType="Readiness" pod="openshift-authentication/oauth-openshift-77f7b95df5-r4xf7" podUID=1af660b3-ac3a-4182-86eb-2f74725d8415 containerName="oauth-openshift" probeResult=failure output="Get \"https://10.129.0.12:6443/healthz\": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)"`,
			},
			want: monitorapi.Intervals{
				{
					Condition: monitorapi.Condition{
						Level:   0,
						Locator: `ns/openshift-authentication pod/oauth-openshift-77f7b95df5-r4xf7 uid/1af660b3-ac3a-4182-86eb-2f74725d8415 container/oauth-openshift`,
						Message: `reason/ReadinessFailed Get "https://10.129.0.12:6443/healthz": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)`,
					},
					From: mustTime("05 Jul 2022 17:47:52.807876 UTC"),
					To:   mustTime("05 Jul 2022 17:47:52.807876 UTC"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := readinessFailure(tt.args.logLine); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readinessFailure() = %#v, \nwant %#v", got, tt.want)
			}
		})
	}
}

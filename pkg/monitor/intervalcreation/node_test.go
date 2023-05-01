package intervalcreation

import (
	"fmt"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/stretchr/testify/assert"

	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
)

func TestMonitorApiIntervals(t *testing.T) {

	testcase := []struct {
		name    string
		logLine string
		want    monitorapi.EventInterval
	}{
		{
			name:    "status",
			logLine: `Sep 27 08:59:59.857303 ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s kubenswrapper[2397]: I0927 08:59:59.850662    2397 status_manager.go:667] "Failed to get status for pod" podUID=a1947638-25c2-4fd8-b3c8-4dbaa666bc61 pod="openshift-monitoring/prometheus-k8s-0" err="Get \"https://api-int.ci-op-747jjqn3-b3af3.ci2.azure.devcluster.openshift.com:6443/api/v1/namespaces/openshift-monitoring/pods/prometheus-k8s-0\": http2: client connection lost"`,
			want: monitorapi.EventInterval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: "ns/openshift-monitoring pod/prometheus-k8s-0 uid/a1947638-25c2-4fd8-b3c8-4dbaa666bc61 container/",
					Message: "reason/HttpClientConnectionLost Get \"https://api-int.ci-op-747jjqn3-b3af3.ci2.azure.devcluster.openshift.com:6443/api/v1/namespaces/openshift-monitoring/pods/prometheus-k8s-0\": http2: client connection lost",
				},
				From: systemdJournalLogTime("Sep 27 08:59:59.857303"),
				To:   systemdJournalLogTime("Sep 27 08:59:59.857303"),
			},
		},
		{
			name:    "reflector",
			logLine: `Sep 27 08:59:59.853216 ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s kubenswrapper[2397]: W0927 08:59:59.849136    2397 reflector.go:347] object-"openshift-monitoring"/"prometheus-adapter-7m6srg4dfreoi": watch of *v1.Secret ended with: an error on the server ("unable to decode an event from the watch stream: http2: client connection lost") has prevented the request from succeeding`,
			want: monitorapi.EventInterval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: "ns/openshift-monitoring pod/prometheus-adapter-7m6srg4dfreoi uid/ container/",
					Message: "reason/HttpClientConnectionLost unable to decode an event from the watch stream: http2: client connection lost",
				},
				From: systemdJournalLogTime("Sep 27 08:59:59.853216"),
				To:   systemdJournalLogTime("Sep 27 08:59:59.853216"),
			},
		},
		{
			name:    "kubelet",
			logLine: `Sep 27 08:59:59.853216 ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s kubenswrapper[2397]: E0927 08:59:59.849143    2397 kubelet_node_status.go:487] "Error updating node status, will retry" err="error getting node \"ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s\": Get \"https://api-int.ci-op-747jjqn3-b3af3.ci2.azure.devcluster.openshift.com:6443/api/v1/nodes/ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s?timeout=10s\": http2: client connection lost"`,
			want: monitorapi.EventInterval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: "node/ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s",
					Message: "reason/HttpClientConnectionLost error getting node \"ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s\": Get \"https://api-int.ci-op-747jjqn3-b3af3.ci2.azure.devcluster.openshift.com:6443/api/v1/nodes/ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s?timeout=10s\": http2: client connection lost",
				},
				From: systemdJournalLogTime("Sep 27 08:59:59.853216"),
				To:   systemdJournalLogTime("Sep 27 08:59:59.853216"),
			},
		},
		{
			name:    "simple failure",
			logLine: `Jul 05 17:47:52.807876 ci-op-lxqqvl5x-d3bee-gl4hp-master-0 hyperkube[1495]: I0606 17:47:52.807876    1599 prober.go:121] "Probe failed" probeType="Readiness" pod="openshift-authentication/oauth-openshift-77f7b95df5-r4xf7" podUID=1af660b3-ac3a-4182-86eb-2f74725d8415 containerName="oauth-openshift" probeResult=failure output="Get \"https://10.129.0.12:6443/healthz\": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)"`,
			want: monitorapi.EventInterval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: "ns/openshift-authentication pod/oauth-openshift-77f7b95df5-r4xf7 uid/1af660b3-ac3a-4182-86eb-2f74725d8415 container/oauth-openshift",
					Message: "reason/ReadinessFailed Get \"https://10.129.0.12:6443/healthz\": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)",
				},
				From: systemdJournalLogTime("Jul 05 17:47:52.807876"),
				To:   systemdJournalLogTime("Jul 05 17:47:52.807876"),
			},
		},
		{
			name:    "simple error",
			logLine: `Jul 05 17:43:12.908344 ci-op-lxqqvl5x-d3bee-gl4hp-master-0 hyperkube[1495]: E0606 17:43:12.908344    1500 prober.go:118] "Probe errored" err="rpc error: code = NotFound desc = container is not created or running: checking if PID of 645437acbb2ca429c04d5a2628924e2e10d44c681c824dddc7c82ffa30a936be is running failed: container process not found" probeType="Readiness" pod="openshift-marketplace/redhat-operators-4jpg4" podUID=0bac4741-a3bd-483c-b119-e97663d64024 containerName="registry-server"`,
			want: monitorapi.EventInterval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: "ns/openshift-marketplace pod/redhat-operators-4jpg4 uid/0bac4741-a3bd-483c-b119-e97663d64024 container/registry-server",
					Message: "reason/ReadinessErrored rpc error: code = NotFound desc = container is not created or running: checking if PID of 645437acbb2ca429c04d5a2628924e2e10d44c681c824dddc7c82ffa30a936be is running failed: container process not found",
				},
				From: systemdJournalLogTime("Jul 05 17:43:12.908344"),
				To:   systemdJournalLogTime("Jul 05 17:43:12.908344"),
			},
		},
		{
			name:    "signature error",
			logLine: `Feb 01 05:37:45.731611 ci-op-vyccmv3h-4ef92-xs5k5-master-0 kubenswrapper[2213]: E0201 05:37:45.730879 2213 pod_workers.go:965] "Error syncing pod, skipping" err="failed to \"StartContainer\" for \"oauth-proxy\" with ErrImagePull: \"rpc error: code = Unknown desc = copying system image from manifest list: reading signatures: parsing signature https://registry.redhat.io/containers/sigstore/openshift4/ose-oauth-proxy@sha256=f968922564c3eea1c69d6bbe529d8970784d6cae8935afaf674d9fa7c0f72ea3/signature-9: unrecognized signature format, starting with binary 0x3c\"" pod="openshift-e2e-loki/loki-promtail-plm74" podUID=59b26cbf-3421-407c-98ee-986b5a091ef4`,
			want: monitorapi.EventInterval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: "ns/openshift-e2e-loki pod/loki-promtail-plm74 uid/59b26cbf-3421-407c-98ee-986b5a091ef4 container/oauth-proxy",
					Message: "reason/ErrImagePull UnrecognizedSignatureFormat",
				},
				From: systemdJournalLogTime("Feb 01 05:37:45.731611"),
				To:   systemdJournalLogTime("Feb 01 05:37:45.731611"),
			},
		},
	}

	logString := ""
	for i := range testcase {
		logString += testcase[i].logLine + "\n"
	}

	intervals := eventsFromKubeletLogs("testName", []byte(logString))

	assert.NotNil(t, intervals, "Invalid intervals")
	assert.Equal(t, intervals.Len(), len(testcase), "Mismatched interval count")

	for i := range intervals {
		assert.Equalf(t, intervals[i], testcase[i].want, "Interval compare for %s = %v, want %v", testcase[i].name, intervals[i], testcase[i])
	}
}

func TestRegexToContainerReference(t *testing.T) {
	type args struct {
		logLine                 string
		containerReferenceMatch *regexp.Regexp
	}
	tests := []struct {
		name string
		args args
		want monitorapi.ContainerReference
	}{
		{
			name: "statusManager http connection failure",
			args: args{
				logLine:                 `Sep 27 08:59:59.857303 ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s kubenswrapper[2397]: I0927 08:59:59.850662    2397 status_manager.go:667] "Failed to get status for pod" podUID=a1947638-25c2-4fd8-b3c8-4dbaa666bc61 pod="openshift-monitoring/prometheus-k8s-0" err="Get \"https://api-int.ci-op-747jjqn3-b3af3.ci2.azure.devcluster.openshift.com:6443/api/v1/namespaces/openshift-monitoring/pods/prometheus-k8s-0\": http2: client connection lost"`,
				containerReferenceMatch: statusRefRegex,
			},
			want: monitorapi.ContainerReference{
				Pod: monitorapi.PodReference{
					NamespacedReference: monitorapi.NamespacedReference{
						Namespace: "openshift-monitoring",
						Name:      "prometheus-k8s-0",
						UID:       "a1947638-25c2-4fd8-b3c8-4dbaa666bc61",
					},
				},
				ContainerName: "",
			},
		},
		{
			name: "reflector http connection failure",
			args: args{
				logLine:                 `Sep 27 08:59:59.853216 ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s kubenswrapper[2397]: W0927 08:59:59.849136    2397 reflector.go:347] object-"openshift-monitoring"/"prometheus-adapter-7m6srg4dfreoi": watch of *v1.Secret ended with: an error on the server ("unable to decode an event from the watch stream: http2: client connection lost") has prevented the request from succeeding`,
				containerReferenceMatch: reflectorRefRegex,
			},
			want: monitorapi.ContainerReference{
				Pod: monitorapi.PodReference{
					NamespacedReference: monitorapi.NamespacedReference{
						Namespace: "openshift-monitoring",
						Name:      "prometheus-adapter-7m6srg4dfreoi",
						UID:       "",
					},
				},
				ContainerName: "",
			},
		},
		{
			name: "simple failure",
			args: args{
				logLine:                 `Jul 05 17:47:52.807876 ci-op-lxqqvl5x-d3bee-gl4hp-master-0 hyperkube[1495]: I0606 17:47:52.807876    1599 prober.go:121] "Probe failed" probeType="Readiness" pod="openshift-authentication/oauth-openshift-77f7b95df5-r4xf7" podUID=1af660b3-ac3a-4182-86eb-2f74725d8415 containerName="oauth-openshift" probeResult=failure output="Get \"https://10.129.0.12:6443/healthz\": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)"`,
				containerReferenceMatch: containerRefRegex,
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
				logLine:                 `Jul 05 17:43:12.908344 ci-op-lxqqvl5x-d3bee-gl4hp-master-0 hyperkube[1495]: E0606 17:43:12.908344    1500 prober.go:118] "Probe errored" err="rpc error: code = NotFound desc = container is not created or running: checking if PID of 645437acbb2ca429c04d5a2628924e2e10d44c681c824dddc7c82ffa30a936be is running failed: container process not found" probeType="Readiness" pod="openshift-marketplace/redhat-operators-4jpg4" podUID=0bac4741-a3bd-483c-b119-e97663d64024 containerName="registry-server"`,
				containerReferenceMatch: containerRefRegex,
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
			if got := regexToContainerReference(tt.args.logLine, tt.args.containerReferenceMatch); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("regexToContainerReference() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
			name: "kubelet_node failure",
			args: args{
				logLine: `Sep 27 08:59:59.853216 ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s kubenswrapper[2397]: E0927 08:59:59.849143    2397 kubelet_node_status.go:487] "Error updating node status, will retry" err="error getting node \"ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s\": Get \"https://api-int.ci-op-747jjqn3-b3af3.ci2.azure.devcluster.openshift.com:6443/api/v1/nodes/ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s?timeout=10s\": http2: client connection lost"`,
			},
			want: mustTime(fmt.Sprintf("27 Sep %d 08:59:59.853216 UTC", time.Now().Year())),
		},
		{
			name: "reflector failure",
			args: args{
				logLine: `Sep 27 08:59:59.853216 ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s kubenswrapper[2397]: W0927 08:59:59.849136    2397 reflector.go:347] object-"openshift-monitoring"/"prometheus-adapter-7m6srg4dfreoi": watch of *v1.Secret ended with: an error on the server ("unable to decode an event from the watch stream: http2: client connection lost") has prevented the request from succeeding`,
			},
			want: mustTime(fmt.Sprintf("27 Sep %d 08:59:59.853216 UTC", time.Now().Year())),
		},
		{
			name: "status failure",
			args: args{
				logLine: `Sep 27 08:59:59.857303 ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s kubenswrapper[2397]: I0927 08:59:59.850662    2397 status_manager.go:667] "Failed to get status for pod" podUID=a1947638-25c2-4fd8-b3c8-4dbaa666bc61 pod="openshift-monitoring/prometheus-k8s-0" err="Get \"https://api-int.ci-op-747jjqn3-b3af3.ci2.azure.devcluster.openshift.com:6443/api/v1/namespaces/openshift-monitoring/pods/prometheus-k8s-0\": http2: client connection lost"`,
			},
			want: mustTime(fmt.Sprintf("27 Sep %d 08:59:59.857303 UTC", time.Now().Year())),
		},
		{
			name: "simple failure",
			args: args{
				logLine: `Jul 05 17:47:52.807876 ci-op-lxqqvl5x-d3bee-gl4hp-master-0 hyperkube[1495]: I0606 17:47:52.807876    1599 prober.go:121] "Probe failed" probeType="Readiness" pod="openshift-authentication/oauth-openshift-77f7b95df5-r4xf7" podUID=1af660b3-ac3a-4182-86eb-2f74725d8415 containerName="oauth-openshift" probeResult=failure output="Get \"https://10.129.0.12:6443/healthz\": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)"`,
			},
			want: mustTime(fmt.Sprintf("05 Jul %d 17:47:52.807876 UTC", time.Now().Year())),
		},
		{
			name: "simple error",
			args: args{
				logLine: `Jul 05 17:43:12.908344 ci-op-lxqqvl5x-d3bee-gl4hp-master-0 hyperkube[1495]: E0606 17:43:12.908344    1500 prober.go:118] "Probe errored" err="rpc error: code = NotFound desc = container is not created or running: checking if PID of 645437acbb2ca429c04d5a2628924e2e10d44c681c824dddc7c82ffa30a936be is running failed: container process not found" probeType="Readiness" pod="openshift-marketplace/redhat-operators-4jpg4" podUID=0bac4741-a3bd-483c-b119-e97663d64024 containerName="registry-server"`,
			},
			want: mustTime(fmt.Sprintf("05 Jul %d 17:43:12.908344 UTC", time.Now().Year())),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := systemdJournalLogTime(tt.args.logLine); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("systemdJournalLogTime() = %v, want %v", got, tt.want)
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
					From: mustTime(fmt.Sprintf("05 Jul %d 17:47:52.807876 UTC", time.Now().Year())),
					To:   mustTime(fmt.Sprintf("05 Jul %d 17:47:52.807876 UTC", time.Now().Year())),
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

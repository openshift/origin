package kubeletlogcollector

import (
	"fmt"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/stretchr/testify/assert"
)

func TestMonitorApiIntervals(t *testing.T) {

	testcase := []struct {
		name          string
		logLine       string
		generatorFunc func(nodeName string, logBytes []byte) monitorapi.Intervals
		want          monitorapi.Interval
	}{
		{
			name:          "status",
			logLine:       `Sep 27 08:59:59.857303 ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s kubenswrapper[2397]: I0927 08:59:59.850662    2397 status_manager.go:667] "Failed to get status for pod" podUID=a1947638-25c2-4fd8-b3c8-4dbaa666bc61 pod="openshift-monitoring/prometheus-k8s-0" err="Get \"https://api-int.ci-op-747jjqn3-b3af3.ci2.azure.devcluster.openshift.com:6443/api/v1/namespaces/openshift-monitoring/pods/prometheus-k8s-0\": http2: client connection lost"`,
			generatorFunc: eventsFromKubeletLogs,
			want: monitorapi.Interval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: "ns/openshift-monitoring pod/prometheus-k8s-0 uid/a1947638-25c2-4fd8-b3c8-4dbaa666bc61 container/",
					Message: "reason/HttpClientConnectionLost ",
					StructuredLocator: monitorapi.Locator{
						Type: monitorapi.LocatorTypeContainer,
						Keys: map[monitorapi.LocatorKey]string{
							"namespace": "openshift-monitoring",
							"pod":       "prometheus-k8s-0",
							"uid":       "a1947638-25c2-4fd8-b3c8-4dbaa666bc61",
							"container": "",
						},
					},
					StructuredMessage: monitorapi.Message{
						Reason:       "HttpClientConnectionLost",
						HumanMessage: "Get \"https://api-int.ci-op-747jjqn3-b3af3.ci2.azure.devcluster.openshift.com:6443/api/v1/namespaces/openshift-monitoring/pods/prometheus-k8s-0\": http2: client connection lost",
						Annotations: map[monitorapi.AnnotationKey]string{
							monitorapi.AnnotationReason: "HttpClientConnectionLost",
							monitorapi.AnnotationNode:   "testName",
						},
					},
				},
				From: systemdJournalLogTime("Sep 27 08:59:59.857303"),
				To:   systemdJournalLogTime("Sep 27 08:59:59.857303"),
			},
		},
		{
			name:          "reflector",
			logLine:       `Sep 27 08:59:59.853216 ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s kubenswrapper[2397]: W0927 08:59:59.849136    2397 reflector.go:347] object-"openshift-monitoring"/"prometheus-adapter-7m6srg4dfreoi": watch of *v1.Secret ended with: an error on the server ("unable to decode an event from the watch stream: http2: client connection lost") has prevented the request from succeeding`,
			generatorFunc: eventsFromKubeletLogs,
			want: monitorapi.Interval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: "ns/openshift-monitoring pod/prometheus-adapter-7m6srg4dfreoi uid/ container/",
					Message: "reason/HttpClientConnectionLost unable to decode an event from the watch stream: http2: client connection lost",
					StructuredLocator: monitorapi.Locator{
						Type: monitorapi.LocatorTypeContainer,
						Keys: map[monitorapi.LocatorKey]string{
							"namespace": "openshift-monitoring",
							"pod":       "prometheus-adapter-7m6srg4dfreoi",
							"uid":       "",
							"container": "",
						},
					},
					StructuredMessage: monitorapi.Message{
						Reason:       "HttpClientConnectionLost",
						HumanMessage: "unable to decode an event from the watch stream: http2: client connection lost",
						Annotations: map[monitorapi.AnnotationKey]string{
							monitorapi.AnnotationReason: "HttpClientConnectionLost",
							monitorapi.AnnotationNode:   "testName",
						},
					},
				},
				From: systemdJournalLogTime("Sep 27 08:59:59.853216"),
				To:   systemdJournalLogTime("Sep 27 08:59:59.853216"),
			},
		},
		{
			name:          "kubelet",
			logLine:       `Sep 27 08:59:59.853216 ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s kubenswrapper[2397]: E0927 08:59:59.849143    2397 kubelet_node_status.go:487] "Error updating node status, will retry" err="error getting node \"ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s\": Get \"https://api-int.ci-op-747jjqn3-b3af3.ci2.azure.devcluster.openshift.com:6443/api/v1/nodes/ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s?timeout=10s\": http2: client connection lost"`,
			generatorFunc: eventsFromKubeletLogs,
			want: monitorapi.Interval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: "node/ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s",
					Message: "reason/HttpClientConnectionLost error getting node \"ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s\": Get \"https://api-int.ci-op-747jjqn3-b3af3.ci2.azure.devcluster.openshift.com:6443/api/v1/nodes/ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s?timeout=10s\": http2: client connection lost",
					StructuredLocator: monitorapi.Locator{
						Type: monitorapi.LocatorTypeNode,
						Keys: map[monitorapi.LocatorKey]string{
							"node": "testName",
						},
					},
					StructuredMessage: monitorapi.Message{
						Reason:       "HttpClientConnectionLost",
						HumanMessage: "error getting node \"ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s\": Get \"https://api-int.ci-op-747jjqn3-b3af3.ci2.azure.devcluster.openshift.com:6443/api/v1/nodes/ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s?timeout=10s\": http2: client connection lost",
						Annotations: map[monitorapi.AnnotationKey]string{
							monitorapi.AnnotationReason: "HttpClientConnectionLost",
							monitorapi.AnnotationNode:   "testName",
						},
					},
				},
				From: systemdJournalLogTime("Sep 27 08:59:59.853216"),
				To:   systemdJournalLogTime("Sep 27 08:59:59.853216"),
			},
		},
		{
			name:          "leaseUpdateError",
			logLine:       `May 19 19:10:03.753983 ci-op-6clh576g-0dd98-xz4pt-master-2 kubenswrapper[1516]: E0519 19:10:03.753942    1516 controller.go:189] failed to update lease, error: Put "https://api-int.ci-op-6clh576g-0dd98.ci2.azure.devcluster.openshift.com:6443/apis/coordination.k8s.io/v1/namespaces/kube-node-lease/leases/ci-op-6clh576g-0dd98-xz4pt-master-2?timeout=10s": net/http: request canceled (Client.Timeout exceeded while awaiting headers)`,
			generatorFunc: eventsFromKubeletLogs,
			want: monitorapi.Interval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: "node/testName",
					Message: "reason/FailedToUpdateLease https://api-int.ci-op-6clh576g-0dd98.ci2.azure.devcluster.openshift.com:6443/apis/coordination.k8s.io/v1/namespaces/kube-node-lease/leases/ci-op-6clh576g-0dd98-xz4pt-master-2?timeout=10s - net/http: request canceled (Client.Timeout exceeded while awaiting headers)",
					StructuredLocator: monitorapi.Locator{
						Type: monitorapi.LocatorTypeNode,
						Keys: map[monitorapi.LocatorKey]string{
							"node": "testName",
						},
					},
					StructuredMessage: monitorapi.Message{
						Reason:       "FailedToUpdateLease",
						HumanMessage: "https://api-int.ci-op-6clh576g-0dd98.ci2.azure.devcluster.openshift.com:6443/apis/coordination.k8s.io/v1/namespaces/kube-node-lease/leases/ci-op-6clh576g-0dd98-xz4pt-master-2?timeout=10s - net/http: request canceled (Client.Timeout exceeded while awaiting headers)",
						Annotations: map[monitorapi.AnnotationKey]string{
							monitorapi.AnnotationReason: "FailedToUpdateLease",
						},
					},
				},
				From: systemdJournalLogTime("May 19 19:10:03.753983"),
				To:   systemdJournalLogTime("May 19 19:10:04.753983"),
			},
		},
		{
			name:          "leaseUpdateErr",
			logLine:       `Jun 29 05:16:54.197389 ci-op-cyqgzj4w-ed5cd-ll5md-master-0 kubenswrapper[2336]: E0629 05:16:54.195979    2336 controller.go:193] "Failed to update lease" err="Put \"https://api-int.ci-op-cyqgzj4w-ed5cd.ci2.azure.devcluster.openshift.com:6443/apis/coordination.k8s.io/v1/namespaces/kube-node-lease/leases/ci-op-cyqgzj4w-ed5cd-ll5md-master-0?timeout=10s\": http2: client connection lost"`,
			generatorFunc: eventsFromKubeletLogs,
			want: monitorapi.Interval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: "node/testName",
					Message: "reason/FailedToUpdateLease https://api-int.ci-op-cyqgzj4w-ed5cd.ci2.azure.devcluster.openshift.com:6443/apis/coordination.k8s.io/v1/namespaces/kube-node-lease/leases/ci-op-cyqgzj4w-ed5cd-ll5md-master-0?timeout=10s - http2: client connection lost",
					StructuredLocator: monitorapi.Locator{
						Type: monitorapi.LocatorTypeNode,
						Keys: map[monitorapi.LocatorKey]string{
							"node": "testName",
						},
					},
					StructuredMessage: monitorapi.Message{
						Reason:       "FailedToUpdateLease",
						HumanMessage: "https://api-int.ci-op-cyqgzj4w-ed5cd.ci2.azure.devcluster.openshift.com:6443/apis/coordination.k8s.io/v1/namespaces/kube-node-lease/leases/ci-op-cyqgzj4w-ed5cd-ll5md-master-0?timeout=10s - http2: client connection lost",
						Annotations: map[monitorapi.AnnotationKey]string{
							monitorapi.AnnotationReason: "FailedToUpdateLease",
						},
					},
				},
				From: systemdJournalLogTime("Jun 29 05:16:54.197389"),
				To:   systemdJournalLogTime("Jun 29 05:16:55.197389"),
			},
		},
		{
			name:          "simple failure",
			logLine:       `Jul 05 17:47:52.807876 ci-op-lxqqvl5x-d3bee-gl4hp-master-0 hyperkube[1495]: I0606 17:47:52.807876    1599 prober.go:121] "Probe failed" probeType="Readiness" pod="openshift-authentication/oauth-openshift-77f7b95df5-r4xf7" podUID=1af660b3-ac3a-4182-86eb-2f74725d8415 containerName="oauth-openshift" probeResult=failure output="Get \"https://10.129.0.12:6443/healthz\": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)"`,
			generatorFunc: eventsFromKubeletLogs,
			want: monitorapi.Interval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: "ns/openshift-authentication pod/oauth-openshift-77f7b95df5-r4xf7 uid/1af660b3-ac3a-4182-86eb-2f74725d8415 container/oauth-openshift",
					Message: "reason/ReadinessFailed Get \"https://10.129.0.12:6443/healthz\": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)",
					StructuredLocator: monitorapi.Locator{
						Type: monitorapi.LocatorTypeContainer,
						Keys: map[monitorapi.LocatorKey]string{
							"namespace": "openshift-authentication",
							"pod":       "oauth-openshift-77f7b95df5-r4xf7",
							"uid":       "1af660b3-ac3a-4182-86eb-2f74725d8415",
							"container": "oauth-openshift",
						},
					},
					StructuredMessage: monitorapi.Message{
						Reason:       "ReadinessFailed",
						HumanMessage: "Get \"https://10.129.0.12:6443/healthz\": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)",
						Annotations: map[monitorapi.AnnotationKey]string{
							monitorapi.AnnotationReason: "ReadinessFailed",
							monitorapi.AnnotationNode:   "testName",
						},
					},
				},
				From: systemdJournalLogTime("Jul 05 17:47:52.807876"),
				To:   systemdJournalLogTime("Jul 05 17:47:52.807876"),
			},
		},
		{
			name:          "simple error",
			logLine:       `Jul 05 17:43:12.908344 ci-op-lxqqvl5x-d3bee-gl4hp-master-0 hyperkube[1495]: E0606 17:43:12.908344    1500 prober.go:118] "Probe errored" err="rpc error: code = NotFound desc = container is not created or running: checking if PID of 645437acbb2ca429c04d5a2628924e2e10d44c681c824dddc7c82ffa30a936be is running failed: container process not found" probeType="Readiness" pod="openshift-marketplace/redhat-operators-4jpg4" podUID=0bac4741-a3bd-483c-b119-e97663d64024 containerName="registry-server"`,
			generatorFunc: eventsFromKubeletLogs,
			want: monitorapi.Interval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: "ns/openshift-marketplace pod/redhat-operators-4jpg4 uid/0bac4741-a3bd-483c-b119-e97663d64024 container/registry-server",
					Message: "reason/ReadinessErrored rpc error: code = NotFound desc = container is not created or running: checking if PID of 645437acbb2ca429c04d5a2628924e2e10d44c681c824dddc7c82ffa30a936be is running failed: container process not found",
					StructuredLocator: monitorapi.Locator{
						Type: monitorapi.LocatorTypeContainer,
						Keys: map[monitorapi.LocatorKey]string{
							"namespace": "openshift-marketplace",
							"pod":       "redhat-operators-4jpg4",
							"uid":       "0bac4741-a3bd-483c-b119-e97663d64024",
							"container": "registry-server",
						},
					},
					StructuredMessage: monitorapi.Message{
						Reason:       "ReadinessErrored",
						HumanMessage: "rpc error: code = NotFound desc = container is not created or running: checking if PID of 645437acbb2ca429c04d5a2628924e2e10d44c681c824dddc7c82ffa30a936be is running failed: container process not found",
						Annotations: map[monitorapi.AnnotationKey]string{
							monitorapi.AnnotationReason: "ReadinessErrored",
							monitorapi.AnnotationNode:   "testName",
						},
					},
				},
				From: systemdJournalLogTime("Jul 05 17:43:12.908344"),
				To:   systemdJournalLogTime("Jul 05 17:43:12.908344"),
			},
		},
		{
			name:          "signature error",
			logLine:       `Feb 01 05:37:45.731611 ci-op-vyccmv3h-4ef92-xs5k5-master-0 kubenswrapper[2213]: E0201 05:37:45.730879 2213 pod_workers.go:965] "Error syncing pod, skipping" err="failed to \"StartContainer\" for \"oauth-proxy\" with ErrImagePull: \"rpc error: code = Unknown desc = copying system image from manifest list: reading signatures: parsing signature https://registry.redhat.io/containers/sigstore/openshift4/ose-oauth-proxy@sha256=f968922564c3eea1c69d6bbe529d8970784d6cae8935afaf674d9fa7c0f72ea3/signature-9: unrecognized signature format, starting with binary 0x3c\"" pod="openshift-e2e-loki/loki-promtail-plm74" podUID=59b26cbf-3421-407c-98ee-986b5a091ef4`,
			generatorFunc: eventsFromKubeletLogs,
			want: monitorapi.Interval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: "ns/openshift-e2e-loki pod/loki-promtail-plm74 uid/59b26cbf-3421-407c-98ee-986b5a091ef4 container/oauth-proxy",
					Message: "cause/UnrecognizedSignatureFormat reason/ErrImagePull",
					StructuredLocator: monitorapi.Locator{
						Type: monitorapi.LocatorTypeContainer,
						Keys: map[monitorapi.LocatorKey]string{
							"namespace": "openshift-e2e-loki",
							"pod":       "loki-promtail-plm74",
							"uid":       "59b26cbf-3421-407c-98ee-986b5a091ef4",
							"container": "oauth-proxy",
						},
					},
					StructuredMessage: monitorapi.Message{
						Reason:       "ErrImagePull",
						Cause:        "UnrecognizedSignatureFormat",
						HumanMessage: "",
						Annotations: map[monitorapi.AnnotationKey]string{
							monitorapi.AnnotationReason: "ErrImagePull",
							monitorapi.AnnotationNode:   "testName",
							monitorapi.AnnotationCause:  "UnrecognizedSignatureFormat",
						},
					},
				},
				From: systemdJournalLogTime("Feb 01 05:37:45.731611"),
				To:   systemdJournalLogTime("Feb 01 05:37:45.731611"),
			},
		},
		{
			name:          "too many netlink events",
			logLine:       `Apr 12 11:49:49.188086 ci-op-xs3rnrtc-2d4c7-4mhm7-worker-b-dwc7w NetworkManager[1155]: <info> [1681300187.8326] platform-linux: netlink[rtnl]: read: too many netlink events. Need to resynchronize platform cache`,
			generatorFunc: intervalsFromNetworkManagerLogs,
			want: monitorapi.Interval{
				Condition: monitorapi.Condition{
					Level:   monitorapi.Warning,
					Locator: "node/ci-op-xs3rnrtc-2d4c7-4mhm7-worker-b-dwc7w",
					Message: "[1155]: <info> [1681300187.8326] platform-linux: netlink[rtnl]: read: too many netlink events. Need to resynchronize platform cache",
					StructuredLocator: monitorapi.Locator{
						Type: monitorapi.LocatorTypeNode,
						Keys: map[monitorapi.LocatorKey]string{
							"node": "testName",
						},
					},
					StructuredMessage: monitorapi.Message{
						Reason:       "",
						Cause:        "",
						HumanMessage: "NetworkManager[1155]: <info> [1681300187.8326] platform-linux: netlink[rtnl]: read: too many netlink events. Need to resynchronize platform cache",
						Annotations:  map[monitorapi.AnnotationKey]string{},
					},
				},
				From: systemdJournalLogTime("Apr 12 11:49:49.188086"),
				To:   systemdJournalLogTime("Apr 12 11:49:50.188086"),
			},
		},
	}

	for _, tc := range testcase {
		t.Run(tc.name, func(t *testing.T) {
			logString := tc.logLine + "\n"

			intervals := tc.generatorFunc("testName", []byte(logString))

			assert.NotNil(t, intervals, "Invalid intervals")
			assert.Equal(t, 1, intervals.Len())

			//assert.Equalf(t, intervals[i], testcase[i].want, "Interval compare for %s = %v, want %v", testcase[i].name, intervals[i], testcase[i])

			assert.Equal(t, tc.want.StructuredLocator, intervals[0].StructuredLocator)
			assert.Equal(t, tc.want.StructuredMessage, intervals[0].StructuredMessage)
			assert.Equal(t, tc.want.Level, intervals[0].Level)
			assert.Equal(t, tc.want.From, intervals[0].From)
			assert.Equal(t, tc.want.To, intervals[0].To)

		})
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
		want monitorapi.Locator
	}{
		{
			name: "statusManager http connection failure",
			args: args{
				logLine:                 `Sep 27 08:59:59.857303 ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s kubenswrapper[2397]: I0927 08:59:59.850662    2397 status_manager.go:667] "Failed to get status for pod" podUID=a1947638-25c2-4fd8-b3c8-4dbaa666bc61 pod="openshift-monitoring/prometheus-k8s-0" err="Get \"https://api-int.ci-op-747jjqn3-b3af3.ci2.azure.devcluster.openshift.com:6443/api/v1/namespaces/openshift-monitoring/pods/prometheus-k8s-0\": http2: client connection lost"`,
				containerReferenceMatch: statusRefRegex,
			},
			want: monitorapi.Locator{
				Type: monitorapi.LocatorTypeContainer,
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorContainerKey: "",
					monitorapi.LocatorNamespaceKey: "openshift-monitoring",
					monitorapi.LocatorPodKey:       "prometheus-k8s-0",
					monitorapi.LocatorUIDKey:       "a1947638-25c2-4fd8-b3c8-4dbaa666bc61",
				},
			},
		},
		{
			name: "reflector http connection failure",
			args: args{
				logLine:                 `Sep 27 08:59:59.853216 ci-op-747jjqn3-b3af3-f45pk-worker-centralus2-bdp5s kubenswrapper[2397]: W0927 08:59:59.849136    2397 reflector.go:347] object-"openshift-monitoring"/"prometheus-adapter-7m6srg4dfreoi": watch of *v1.Secret ended with: an error on the server ("unable to decode an event from the watch stream: http2: client connection lost") has prevented the request from succeeding`,
				containerReferenceMatch: reflectorRefRegex,
			},
			want: monitorapi.Locator{
				Type: monitorapi.LocatorTypeContainer,
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorContainerKey: "",
					monitorapi.LocatorNamespaceKey: "openshift-monitoring",
					monitorapi.LocatorPodKey:       "prometheus-adapter-7m6srg4dfreoi",
					monitorapi.LocatorUIDKey:       "",
				},
			},
		},
		{
			name: "simple failure",
			args: args{
				logLine:                 `Jul 05 17:47:52.807876 ci-op-lxqqvl5x-d3bee-gl4hp-master-0 hyperkube[1495]: I0606 17:47:52.807876    1599 prober.go:121] "Probe failed" probeType="Readiness" pod="openshift-authentication/oauth-openshift-77f7b95df5-r4xf7" podUID=1af660b3-ac3a-4182-86eb-2f74725d8415 containerName="oauth-openshift" probeResult=failure output="Get \"https://10.129.0.12:6443/healthz\": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)"`,
				containerReferenceMatch: containerRefRegex,
			},
			want: monitorapi.Locator{
				Type: monitorapi.LocatorTypeContainer,
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorContainerKey: "oauth-openshift",
					monitorapi.LocatorNamespaceKey: "openshift-authentication",
					monitorapi.LocatorPodKey:       "oauth-openshift-77f7b95df5-r4xf7",
					monitorapi.LocatorUIDKey:       "1af660b3-ac3a-4182-86eb-2f74725d8415",
				},
			},
		},
		{
			name: "simple error",
			args: args{
				logLine:                 `Jul 05 17:43:12.908344 ci-op-lxqqvl5x-d3bee-gl4hp-master-0 hyperkube[1495]: E0606 17:43:12.908344    1500 prober.go:118] "Probe errored" err="rpc error: code = NotFound desc = container is not created or running: checking if PID of 645437acbb2ca429c04d5a2628924e2e10d44c681c824dddc7c82ffa30a936be is running failed: container process not found" probeType="Readiness" pod="openshift-marketplace/redhat-operators-4jpg4" podUID=0bac4741-a3bd-483c-b119-e97663d64024 containerName="registry-server"`,
				containerReferenceMatch: containerRefRegex,
			},
			want: monitorapi.Locator{
				Type: monitorapi.LocatorTypeContainer,
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorContainerKey: "registry-server",
					monitorapi.LocatorNamespaceKey: "openshift-marketplace",
					monitorapi.LocatorPodKey:       "redhat-operators-4jpg4",
					monitorapi.LocatorUIDKey:       "0bac4741-a3bd-483c-b119-e97663d64024",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := regexToContainerReference(tt.args.logLine, tt.args.containerReferenceMatch)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_probeProblemToContainerReference(t *testing.T) {
	type args struct {
		logLine string
	}
	tests := []struct {
		name string
		args args
		want monitorapi.Locator
	}{
		{
			name: "simple failure",
			args: args{
				logLine: `Jul 05 17:47:52.807876 ci-op-lxqqvl5x-d3bee-gl4hp-master-0 hyperkube[1495]: I0606 17:47:52.807876    1599 prober.go:121] "Probe failed" probeType="Readiness" pod="openshift-authentication/oauth-openshift-77f7b95df5-r4xf7" podUID=1af660b3-ac3a-4182-86eb-2f74725d8415 containerName="oauth-openshift" probeResult=failure output="Get \"https://10.129.0.12:6443/healthz\": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)"`,
			},
			want: monitorapi.Locator{
				Type: monitorapi.LocatorTypeContainer,
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorContainerKey: "oauth-openshift",
					monitorapi.LocatorNamespaceKey: "openshift-authentication",
					monitorapi.LocatorPodKey:       "oauth-openshift-77f7b95df5-r4xf7",
					monitorapi.LocatorUIDKey:       "1af660b3-ac3a-4182-86eb-2f74725d8415",
				},
			},
		},
		{
			name: "simple error",
			args: args{
				logLine: `Jul 05 17:43:12.908344 ci-op-lxqqvl5x-d3bee-gl4hp-master-0 hyperkube[1495]: E0606 17:43:12.908344    1500 prober.go:118] "Probe errored" err="rpc error: code = NotFound desc = container is not created or running: checking if PID of 645437acbb2ca429c04d5a2628924e2e10d44c681c824dddc7c82ffa30a936be is running failed: container process not found" probeType="Readiness" pod="openshift-marketplace/redhat-operators-4jpg4" podUID=0bac4741-a3bd-483c-b119-e97663d64024 containerName="registry-server"`,
			},
			want: monitorapi.Locator{
				Type: monitorapi.LocatorTypeContainer,
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorContainerKey: "registry-server",
					monitorapi.LocatorNamespaceKey: "openshift-marketplace",
					monitorapi.LocatorPodKey:       "redhat-operators-4jpg4",
					monitorapi.LocatorUIDKey:       "0bac4741-a3bd-483c-b119-e97663d64024",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := probeProblemToContainerReference(tt.args.logLine)
			assert.Equal(t, tt.want, got)
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
						Level:   monitorapi.Info,
						Locator: `ns/openshift-authentication pod/oauth-openshift-77f7b95df5-r4xf7 uid/1af660b3-ac3a-4182-86eb-2f74725d8415 container/oauth-openshift`,
						StructuredLocator: monitorapi.Locator{
							Type: monitorapi.LocatorTypeContainer,
							Keys: map[monitorapi.LocatorKey]string{
								"namespace": "openshift-authentication",
								"pod":       "oauth-openshift-77f7b95df5-r4xf7",
								"uid":       "1af660b3-ac3a-4182-86eb-2f74725d8415",
								"container": "oauth-openshift",
							},
						},
						Message: `reason/ReadinessFailed Get "https://10.129.0.12:6443/healthz": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)`,
						StructuredMessage: monitorapi.Message{
							Reason:       "ReadinessFailed",
							Cause:        "",
							HumanMessage: "Get \"https://10.129.0.12:6443/healthz\": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)",
							Annotations: map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationReason: "ReadinessFailed",
								monitorapi.AnnotationNode:   "fakenode",
							},
						},
					},
					From: mustTime(fmt.Sprintf("05 Jul %d 17:47:52.807876 UTC", time.Now().Year())),
					To:   mustTime(fmt.Sprintf("05 Jul %d 17:47:52.807876 UTC", time.Now().Year())),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := readinessFailure("fakenode", tt.args.logLine)
			//assert.Equal(t, tt.want, got)
			// TODO: we can't deep test because now we're building things from maps of annotations with a
			// non-predictable order, on the legacy Message and Locator. Once we eliminate these we could, in meantime
			// we'll have to selectively test the other fields that we can compare.
			assert.Equal(t, tt.want[0].StructuredLocator, got[0].StructuredLocator)
			assert.Equal(t, tt.want[0].StructuredMessage, got[0].StructuredMessage)
			assert.Equal(t, tt.want[0].Level, got[0].Level)
			assert.Equal(t, tt.want[0].From, got[0].From)
			assert.Equal(t, tt.want[0].To, got[0].To)
		})
	}
}

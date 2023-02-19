package synthetictests

import (
	_ "embed"
	"regexp"
	"strings"
	"testing"
	"time"

	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/duplicateevents"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/stretchr/testify/assert"
)

func TestEventCountExtractor(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		message string
		times   int
	}{
		{
			name:    "simple",
			input:   `pod/network-check-target-5f44k node/ip-10-0-210-155.us-west-2.compute.internal - reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started? (24 times)`,
			message: `pod/network-check-target-5f44k node/ip-10-0-210-155.us-west-2.compute.internal - reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started?`,
			times:   24,
		},
		{
			name:    "new lines",
			input:   "ns/e2e-container-probe-7285 pod/liveness-f0fce2c6-6eed-4ace-bf69-2df5e5b8b1ea node/ci-op-sti304mj-2a78c-pq5zv-worker-b-sknbn reason/ProbeWarning Liveness probe warning: <a href=\"http://0.0.0.0/\">Found</a>.\n\n (22 times)",
			message: "ns/e2e-container-probe-7285 pod/liveness-f0fce2c6-6eed-4ace-bf69-2df5e5b8b1ea node/ci-op-sti304mj-2a78c-pq5zv-worker-b-sknbn reason/ProbeWarning Liveness probe warning: <a href=\"http://0.0.0.0/\">Found</a>.\n\n",
			times:   22,
		},
		{
			name:  "other message",
			input: "some node message",
			times: 0,
		},
		{
			name:    "pod eviction failure",
			input:   "reason/MalscheduledPod pod/router-default-84c89f5bf8-5rdcb pod/router-default-84c89f5bf8-bg9ql should be one per node, but all were placed on node/ip-10-0-172-166.ec2.internal; evicting pod/router-default-84c89f5bf8-5rdcb (79 times)",
			message: "reason/MalscheduledPod pod/router-default-84c89f5bf8-5rdcb pod/router-default-84c89f5bf8-bg9ql should be one per node, but all were placed on node/ip-10-0-172-166.ec2.internal; evicting pod/router-default-84c89f5bf8-5rdcb",
			times:   79,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualMessage, actualCount := getTimesAnEventHappened(test.input)
			assert.Equal(t, test.times, actualCount)
			assert.Equal(t, test.message, actualMessage)
		})
	}
}

func TestEventRegexExcluder(t *testing.T) {
	allowedRepeatedEventsRegex := combinedRegexp(duplicateevents.AllowedRepeatedEventPatterns...)

	tests := []struct {
		name    string
		message string
	}{
		{
			name:    "port-forward",
			message: `ns/e2e-port-forwarding-588 pod/pfpod node/ci-op-g1d5csj7-b08f5-fgrqd-worker-b-xj89f - reason/Unhealthy Readiness probe failed:`,
		},
		{
			name:    "container-probe",
			message: ` ns/e2e-container-probe-3794 pod/test-webserver-3faa80d6-05f2-42a7-9846-099e8a4cf28c node/ci-op-gzm3mjwm-875d2-tvchv-worker-c-w47mw - reason/Unhealthy Readiness probe failed: Get "http://10.131.0.54:81/": dial tcp 10.131.0.54:81: connect: connection refused`,
		},
		{
			name:    "failing-init-container",
			message: `ns/e2e-init-container-368 pod/pod-init-cb40ee55-e9c5-4c4b-b541-47cc018d9856 node/ci-op-ncxkp5gj-875d2-5jcfn-worker-c-pwf97 - reason/BackOff Back-off restarting failed container`,
		},
		{
			name:    "scc-test-3",
			message: `ns/e2e-test-scc-578l5 pod/test3 - reason/FailedScheduling 0/6 nodes are available: 3 node(s) didn't match Pod's node affinity/selector, 3 node(s) had taint {node-role.kubernetes.io/master: }, that the pod didn't tolerate.`,
		},
		{
			name:    "missing image",
			message: `ns/e2e-deployment-478 pod/webserver-deployment-795d758f88-fdr4d node/ci-op-h1wxg6l0-16f7c-mb4sj-worker-b-wcdcf - reason/BackOff Back-off pulling image "webserver:404"`,
		},
		{
			name:    "non-root",
			message: `ns/e2e-security-context-test-6596 pod/explicit-root-uid node/ci-op-isj7rd3k-2a78c-kk69w-worker-a-v4kdb - reason/Failed Error: container's runAsUser breaks non-root policy (pod: "explicit-root-uid_e2e-security-context-test-6596(22bf29d0-e546-4a15-8dd7-8acd9165c924)", container: explicit-root-uid)`,
		},
		{
			name:    "local-volume-failed-scheduling",
			message: `ns/e2e-persistent-local-volumes-test-7012 pod/pod-940713ce-7645-4d8c-bba0-5705350a5655 reason/FailedScheduling 0/6 nodes are available: 1 node(s) had volume node affinity conflict, 2 node(s) didn't match Pod's node affinity/selector, 3 node(s) had taint {node-role.kubernetes.io/master: }, that the pod didn't tolerate. (2 times)`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := allowedRepeatedEventsRegex.MatchString(test.message)
			if !actual {
				t.Fatal("did not match")
			}
		})
	}

}

func TestUpgradeEventRegexExcluder(t *testing.T) {
	allowedRepeatedEventsRegex := combinedRegexp(duplicateevents.AllowedUpgradeRepeatedEventPatterns...)

	tests := []struct {
		name    string
		message string
	}{
		{
			name:    "etcd-member",
			message: `ns/openshift-etcd-operator deployment/etcd-operator - reason/UnhealthyEtcdMember unhealthy members: ip-10-0-198-128.ec2.internal`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := allowedRepeatedEventsRegex.MatchString(test.message)
			if !actual {
				t.Fatal("did not match")
			}
		})
	}

}

func TestKnownBugEvents(t *testing.T) {
	evaluator := duplicateEventsEvaluator{
		allowedRepeatedEventPatterns: duplicateevents.AllowedRepeatedEventPatterns,
		knownRepeatedEventsBugs: []duplicateevents.KnownProblem{
			{
				Regexp: regexp.MustCompile(`ns/.* reason/SomeEvent1.*`),
				BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1234567",
			},
			{
				Regexp:   regexp.MustCompile("ns/.*reason/SomeEvent2.*"),
				BZ:       "https://bugzilla.redhat.com/show_bug.cgi?id=1234567",
				Topology: duplicateevents.TopologyPointer(v1.SingleReplicaTopologyMode),
			},
			{
				Regexp:   regexp.MustCompile("ns/.*reason/SomeEvent3.*"),
				BZ:       "https://bugzilla.redhat.com/show_bug.cgi?id=1234567",
				Platform: duplicateevents.PlatformPointer(v1.AWSPlatformType),
			},
			{
				Regexp:   regexp.MustCompile("ns/.*reason/SomeEvent4.*"),
				BZ:       "https://bugzilla.redhat.com/show_bug.cgi?id=1234567",
				Topology: duplicateevents.TopologyPointer(v1.HighlyAvailableTopologyMode),
			},
			{
				Regexp:   regexp.MustCompile("ns/.*reason/SomeEvent5.*"),
				BZ:       "https://bugzilla.redhat.com/show_bug.cgi?id=1234567",
				Platform: duplicateevents.PlatformPointer(v1.GCPPlatformType),
			},
			{
				Regexp:   regexp.MustCompile("ns/.*reason/SomeEvent6.*"),
				BZ:       "https://bugzilla.redhat.com/show_bug.cgi?id=1234567",
				Platform: duplicateevents.PlatformPointer(""),
			},
		},
	}

	tests := []struct {
		name     string
		message  string
		match    bool
		platform v1.PlatformType
		topology v1.TopologyMode
	}{
		{
			name:     "matches without platform or topology",
			message:  `ns/e2e - reason/SomeEvent1 foo (21 times)`,
			match:    true,
			platform: v1.AWSPlatformType,
			topology: v1.SingleReplicaTopologyMode,
		},
		{
			name:     "matches with topology",
			message:  `ns/e2e - reason/SomeEvent2 foo (21 times)`,
			match:    true,
			platform: v1.AWSPlatformType,
			topology: v1.SingleReplicaTopologyMode,
		},
		{
			name:     "matches with topology and platform",
			message:  `ns/e2e - reason/SomeEvent3 foo (21 times)`,
			match:    true,
			platform: v1.AWSPlatformType,
			topology: v1.SingleReplicaTopologyMode,
		},
		{
			name:     "does not match against different topology",
			message:  `ns/e2e - reason/SomeEvent4 foo (21 times)`,
			platform: v1.AWSPlatformType,
			topology: v1.SingleReplicaTopologyMode,
			match:    false,
		},
		{
			name:     "does not match against different platform",
			message:  `ns/e2e - reason/SomeEvent5 foo (21 times)`,
			platform: v1.AWSPlatformType,
			topology: v1.SingleReplicaTopologyMode,
			match:    false,
		},
		{
			name:     "empty platform matches empty platform",
			message:  `ns/e2e - reason/SomeEvent6 foo (21 times)`,
			platform: "",
			match:    true,
		},
		{
			name:     "empty platform doesn't match another platform",
			message:  `ns/e2e - reason/SomeEvent6 foo (21 times)`,
			platform: v1.AWSPlatformType,
			match:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			events := monitorapi.Intervals{}
			events = append(events,
				monitorapi.EventInterval{
					Condition: monitorapi.Condition{Message: test.message},
					From:      time.Unix(1, 0),
					To:        time.Unix(1, 0)},
			)
			evaluator.platform = test.platform
			evaluator.topology = test.topology

			junits := evaluator.testDuplicatedEvents("events should not repeat", false, events, nil)
			if len(junits) < 1 {
				t.Fatal("didn't get junit for duplicated event")
			}
			if test.match && !strings.Contains(junits[0].FailureOutput.Output, "1 events with known BZs") {
				t.Fatalf("expected case to match, but it didn't: %s", test.name)
			}

			if !test.match && strings.Contains(junits[0].FailureOutput.Output, "1 events with known BZs") {
				t.Fatalf("expected case to not match, but it did: %s", test.name)
			}
		})
	}
}

func TestKnownBugEventsGroup(t *testing.T) {
	evaluator := duplicateEventsEvaluator{
		allowedRepeatedEventPatterns: duplicateevents.AllowedRepeatedEventPatterns,
		knownRepeatedEventsBugs: []duplicateevents.KnownProblem{
			{
				Regexp: regexp.MustCompile(`ns/.* reason/SomeEvent1.*`),
				BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1234567",
			},
		},
	}

	tests := []struct {
		name            string
		messages        []string
		platform        v1.PlatformType
		topology        v1.TopologyMode
		expectedMessage string
	}{
		{
			name:            "matches 22 before",
			messages:        []string{`ns/e2e - reason/SomeEvent1 foo (22 times)`, `ns/e2e - reason/SomeEvent1 foo (21 times)`},
			platform:        v1.AWSPlatformType,
			topology:        v1.SingleReplicaTopologyMode,
			expectedMessage: "1 events with known BZs\n\nevent happened 22 times, something is wrong:  - ns/e2e - reason/SomeEvent1 foo From: 04:00:00Z To: 04:00:00Z - https://bugzilla.redhat.com/show_bug.cgi?id=1234567 result=allow ",
		},
		{
			name:            "matches 25 after",
			messages:        []string{`ns/e2e - reason/SomeEvent1 foo (21 times)`, `ns/e2e - reason/SomeEvent1 foo (25 times)`},
			platform:        v1.AWSPlatformType,
			topology:        v1.SingleReplicaTopologyMode,
			expectedMessage: "1 events with known BZs\n\nevent happened 25 times, something is wrong:  - ns/e2e - reason/SomeEvent1 foo From: 04:00:00Z To: 04:00:00Z - https://bugzilla.redhat.com/show_bug.cgi?id=1234567 result=allow ",
		},
		{
			name:            "matches 22 below with below threshold following",
			messages:        []string{`ns/e2e - reason/SomeEvent1 foo (22 times)`, `ns/e2e - reason/SomeEvent1 foo (5 times)`},
			platform:        v1.AWSPlatformType,
			topology:        v1.SingleReplicaTopologyMode,
			expectedMessage: "1 events with known BZs\n\nevent happened 22 times, something is wrong:  - ns/e2e - reason/SomeEvent1 foo From: 04:00:00Z To: 04:00:00Z - https://bugzilla.redhat.com/show_bug.cgi?id=1234567 result=allow ",
		},
		{
			name:            "matches 22 with multiple line message",
			messages:        []string{"ns/e2e - reason/SomeEvent1 foo \nbody:\n (22 times)"},
			platform:        v1.AWSPlatformType,
			topology:        v1.SingleReplicaTopologyMode,
			expectedMessage: "1 events with known BZs\n\nevent happened 22 times, something is wrong:  - ns/e2e - reason/SomeEvent1 foo  result=allow \nbody:\n From: 04:00:00Z To: 04:00:00Z - https://bugzilla.redhat.com/show_bug.cgi?id=1234567",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			events := monitorapi.Intervals{}
			for _, message := range test.messages {

				events = append(events,
					monitorapi.EventInterval{
						Condition: monitorapi.Condition{Message: message},
						From:      time.Unix(872827200, 0),
						To:        time.Unix(872827200, 0)},
				)
			}

			evaluator.platform = test.platform
			evaluator.topology = test.topology

			junits := evaluator.testDuplicatedEvents("events should not repeat", false, events, nil)
			if len(junits) < 1 {
				t.Fatal("didn't get junit for duplicated event")
			}

			if strings.Compare(junits[0].FailureOutput.Output, test.expectedMessage) != 0 {
				t.Fatalf("expected case to match, but it didn't: %s.  Expected:\n%s\nReceived:\n%s\n", test.name, test.expectedMessage, junits[0].FailureOutput.Output)
			}

		})
	}
}

func TestMakeProbeTestEventsGroup(t *testing.T) {

	tests := []struct {
		name            string
		messages        []string
		match           bool
		regEx           string
		operator        string
		expectedMessage string
	}{
		{
			name:            "matches 22 before",
			messages:        []string{`ns/e2e - reason/ProbeError foo Liveness probe error: Get "https://10.128.0.21:8443/healthz": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers) occurred (22 times)`, `ns/e2e - reason/ProbeError foo Liveness probe error: Get "https://10.128.0.21:8443/healthz": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers) occurred (21 times)`},
			match:           true,
			operator:        "e2e",
			regEx:           duplicateevents.ProbeErrorLivenessMessageRegExpStr,
			expectedMessage: "00:00:01 ns/e2e - reason/ProbeError foo Liveness probe error: Get \"https://10.128.0.21:8443/healthz\": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers) occurred (22 times)\n",
		},
		{
			name:            "no matches 22 before",
			messages:        []string{`ns/e2e - reason/ProbeError foo Liveness probe error: Get "https://10.128.0.21:8443/healthz": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers) occurred (22 times)`, `ns/e2e - reason/ProbeError foo Liveness probe error: Get "https://10.128.0.21:8443/healthz": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers) occurred (21 times)`},
			match:           false,
			operator:        "e2e",
			regEx:           duplicateevents.ProbeErrorConnectionRefusedRegExpStr,
			expectedMessage: "",
		},
		{
			name:            "matches 25 after",
			messages:        []string{`ns/openshift-oauth-apiserver pod/apiserver-647fc6c7bf-s8b4h node/ip-10-0-150-209.us-west-1.compute.internal - reason/ProbeError Readiness probe error: Get "https://10.128.0.38:8443/readyz": dial tcp 10.128.0.38:8443: connect: connection refused occurred (22 times)`, `ns/openshift-oauth-apiserver pod/apiserver-647fc6c7bf-s8b4h node/ip-10-0-150-209.us-west-1.compute.internal - reason/ProbeError Readiness probe error: Get "https://10.128.0.38:8443/readyz": dial tcp 10.128.0.38:8443: connect: connection refused occurred (25 times)`},
			operator:        "openshift-oauth-apiserver",
			match:           true,
			regEx:           duplicateevents.ProbeErrorConnectionRefusedRegExpStr,
			expectedMessage: "00:00:01 ns/openshift-oauth-apiserver pod/apiserver-647fc6c7bf-s8b4h node/ip-10-0-150-209.us-west-1.compute.internal - reason/ProbeError Readiness probe error: Get \"https://10.128.0.38:8443/readyz\": dial tcp 10.128.0.38:8443: connect: connection refused occurred (25 times)\n",
		},
		{
			name:            "no matches 25 after",
			messages:        []string{`ns/openshift-oauth-apiserver pod/apiserver-647fc6c7bf-s8b4h node/ip-10-0-150-209.us-west-1.compute.internal - reason/ProbeError Readiness probe error: Get "https://10.128.0.38:8443/readyz": dial tcp 10.128.0.38:8443: connect: connection refused occurred (22 times)`, `ns/openshift-oauth-apiserver pod/apiserver-647fc6c7bf-s8b4h node/ip-10-0-150-209.us-west-1.compute.internal - reason/ProbeError Readiness probe error: Get "https://10.128.0.38:8443/readyz": dial tcp 10.128.0.38:8443: connect: connection refused occurred (25 times)`},
			operator:        "openshift-oauth-apiserver",
			match:           false,
			regEx:           duplicateevents.ProbeErrorLivenessMessageRegExpStr,
			expectedMessage: "",
		},
		{
			name:            "matches 22 below with below threshold following",
			messages:        []string{`reason/ProbeError Readiness probe error: Get "https://10.130.0.15:8443/healthz": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers) occurred (22 times)`, `reason/ProbeError Readiness probe error: Get "https://10.130.0.15:8443/healthz": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers) occurred (5 times)`},
			operator:        "openshift-oauth-apiserver",
			match:           true,
			regEx:           duplicateevents.ProbeErrorReadinessMessageRegExpStr,
			expectedMessage: "00:00:01 reason/ProbeError Readiness probe error: Get \"https://10.130.0.15:8443/healthz\": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers) occurred (22 times)\n",
		},
		{
			name:            "no matches 22 below with below threshold following",
			messages:        []string{`reason/ProbeError Readiness probe error: Get "https://10.130.0.15:8443/healthz": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers) occurred (22 times)`, `reason/ProbeError Readiness probe error: Get "https://10.130.0.15:8443/healthz": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers) occurred (5 times)`},
			operator:        "openshift-oauth-apiserver",
			match:           false,
			regEx:           duplicateevents.ProbeErrorConnectionRefusedRegExpStr,
			expectedMessage: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			events := monitorapi.Intervals{}
			for _, message := range test.messages {

				events = append(events,
					monitorapi.EventInterval{
						Condition: monitorapi.Condition{Message: message},
						From:      time.Unix(1, 0),
						To:        time.Unix(1, 0)},
				)
			}

			junits := makeProbeTest("test test", events, test.operator, test.regEx, duplicateevents.DuplicateEventThreshold)

			if len(junits) < 1 {
				t.Fatal("didn't get junit for duplicated event")
			}

			if test.match && strings.Compare(junits[0].FailureOutput.Output, test.expectedMessage) != 0 {
				t.Fatalf("expected case to match, but it didn't: %s.  Expected:\n%s\nReceived:\n%s\n", test.name, test.expectedMessage, junits[0].FailureOutput.Output)
			}

			if !test.match && junits[0].FailureOutput != nil {
				t.Fatalf("expected case to not match, but it did: %s", test.name)
			}
		})
	}
}

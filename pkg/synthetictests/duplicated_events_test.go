package synthetictests

import "testing"

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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualMessage, actualCount := getTimesAnEventHappened(test.input)
			if actualCount != test.times {
				t.Error(actualCount)
			}
			if actualMessage != test.message {
				t.Error(actualMessage)
			}
		})
	}
}

func TestEventRegexExcluder(t *testing.T) {
	allowedRepeatedEventsRegex := combinedRegexp(allowedRepeatedEventPatterns...)

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
	allowedRepeatedEventsRegex := combinedRegexp(allowedUpgradeRepeatedEventPatterns...)

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
	tests := []struct {
		name    string
		message string
	}{
		{
			name:    "machine-updated",
			message: `ns/openshift-machine-api machine/ci-op-nq7ssnz1-a6744-f6gc4-worker-49lwd - reason/Update Updated Machine ci-op-nq7ssnz1-a6744-f6gc4-worker-49lwd`,
		},
		{
			name:    "ovn-cleanup",
			message: `ns/e2e-proxy-2182 service/proxy-service-jsk2b - reason/FailedToDeleteOVNLoadBalancer Error trying to delete the idling OVN LoadBalancer for Service proxy-service-jsk2b/e2e-proxy-2182: Failed to get ovnkube balancer TCP k8s-idling-lb: OVN command '/usr/bin/ovn-nbctl --timeout=15 --data=bare --no-heading --columns=_uuid find load_balancer external_ids:k8s-idling-lb-tcp=yes' failed: exit status 1`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			found := false
			for _, curr := range knownEventsBugs {
				if curr.Regexp.MatchString(test.message) {
					found = true
				}
			}
			if !found {
				t.Fatal("did not match")
			}

		})
	}

}

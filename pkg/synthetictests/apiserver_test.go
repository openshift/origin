package synthetictests

import (
	"strings"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func timeOrDie(in string) time.Time {
	startTime, err := time.Parse(time.RFC3339, in)
	if err != nil {
		panic(err)
	}
	return startTime
}
func TestAPIServerRecievedShutdownSignal(t *testing.T) {
	tests := []struct {
		name            string
		intervals       []monitorapi.EventInterval
		expectedMessage string
	}{
		{
			name: "missing three api shutdown signals",
			intervals: []monitorapi.EventInterval{
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-0",
						Message: "reason/NodeUpdate phase/Reboot roles/control-plane,master rebooted and kubelet started",
					},
					From: timeOrDie("2023-03-14T19:25:53Z"),
					To:   timeOrDie("2023-03-14T19:26:13Z"),
				},
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-1",
						Message: "reason/NodeUpdate phase/Reboot roles/control-plane,master rebooted and kubelet started",
					},
					From: timeOrDie("2023-03-14T19:29:50Z"),
					To:   timeOrDie("2023-03-14T19:31:41Z"),
				},
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-2",
						Message: "reason/NodeUpdate phase/Reboot roles/control-plane,master rebooted and kubelet started",
					},
					From: timeOrDie("2023-03-14T19:33:57Z"),
					To:   timeOrDie("2023-03-14T19:34:19Z"),
				},
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "e2e-test/\"[sig-arch][Feature:ClusterUpgrade] Cluster should remain functional during upgrade [Disruptive] [Serial]\"",
						Message: "finishedStatus/Failed",
					},
					From: timeOrDie("2023-03-14T19:51:00Z"),
					To:   timeOrDie("2023-03-14T19:51:00Z"),
				},
			},
			expectedMessage: "missing apiserver shutdown event for node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-0 from 19:25:53Z to 19:26:13Z\nmissing apiserver shutdown event for node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-1 from 19:29:50Z to 19:31:41Z\nmissing apiserver shutdown event for node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-2 from 19:33:57Z to 19:34:19Z",
		},
		{
			name: "missing two api shutdown signals",
			intervals: []monitorapi.EventInterval{
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-0",
						Message: "reason/NodeUpdate phase/Reboot roles/control-plane,master rebooted and kubelet started",
					},
					From: timeOrDie("2023-03-14T19:25:53Z"),
					To:   timeOrDie("2023-03-14T19:26:13Z"),
				},
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "ns/openshift-kube-apiserver pod/kube-apiserver-ci-op-q4mkb8xx-ed5cd-dmqtq-master-0 node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-0",
						Message: "reason/ShutdownInitiated Received signal to terminate, becoming unready, but keeping serving",
					},
					From: timeOrDie("2023-03-14T19:25:54Z"),
					To:   timeOrDie("2023-03-14T19:25:54Z"),
				},
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-1",
						Message: "reason/NodeUpdate phase/Reboot roles/control-plane,master rebooted and kubelet started",
					},
					From: timeOrDie("2023-03-14T19:29:50Z"),
					To:   timeOrDie("2023-03-14T19:31:41Z"),
				},
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-2",
						Message: "reason/NodeUpdate phase/Reboot roles/control-plane,master rebooted and kubelet started",
					},
					From: timeOrDie("2023-03-14T19:33:57Z"),
					To:   timeOrDie("2023-03-14T19:34:19Z"),
				},
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "e2e-test/\"[sig-arch][Feature:ClusterUpgrade] Cluster should remain functional during upgrade [Disruptive] [Serial]\"",
						Message: "finishedStatus/Failed",
					},
					From: timeOrDie("2023-03-14T19:51:00Z"),
					To:   timeOrDie("2023-03-14T19:51:00Z"),
				},
			},
			expectedMessage: "missing apiserver shutdown event for node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-1 from 19:29:50Z to 19:31:41Z\nmissing apiserver shutdown event for node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-2 from 19:33:57Z to 19:34:19Z",
		},
		{
			name: "missing one api shutdown signal",
			intervals: []monitorapi.EventInterval{
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-0",
						Message: "reason/NodeUpdate phase/Reboot roles/control-plane,master rebooted and kubelet started",
					},
					From: timeOrDie("2023-03-14T19:25:53Z"),
					To:   timeOrDie("2023-03-14T19:26:13Z"),
				},
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "ns/openshift-kube-apiserver pod/kube-apiserver-ci-op-q4mkb8xx-ed5cd-dmqtq-master-0 node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-0",
						Message: "reason/ShutdownInitiated Received signal to terminate, becoming unready, but keeping serving",
					},
					From: timeOrDie("2023-03-14T19:25:54Z"),
					To:   timeOrDie("2023-03-14T19:25:54Z"),
				},
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-1",
						Message: "reason/NodeUpdate phase/Reboot roles/control-plane,master rebooted and kubelet started",
					},
					From: timeOrDie("2023-03-14T19:29:50Z"),
					To:   timeOrDie("2023-03-14T19:31:41Z"),
				},
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "ns/openshift-kube-apiserver pod/kube-apiserver-ci-op-q4mkb8xx-ed5cd-dmqtq-master-1 node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-1",
						Message: "reason/ShutdownInitiated Received signal to terminate, becoming unready, but keeping serving",
					},
					From: timeOrDie("2023-03-14T19:29:54Z"),
					To:   timeOrDie("2023-03-14T19:29:54Z"),
				},
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-2",
						Message: "reason/NodeUpdate phase/Reboot roles/control-plane,master rebooted and kubelet started",
					},
					From: timeOrDie("2023-03-14T19:33:57Z"),
					To:   timeOrDie("2023-03-14T19:34:19Z"),
				},
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "e2e-test/\"[sig-arch][Feature:ClusterUpgrade] Cluster should remain functional during upgrade [Disruptive] [Serial]\"",
						Message: "finishedStatus/Failed",
					},
					From: timeOrDie("2023-03-14T19:51:00Z"),
					To:   timeOrDie("2023-03-14T19:51:00Z"),
				},
			},
			expectedMessage: "missing apiserver shutdown event for node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-2 from 19:33:57Z to 19:34:19Z",
		},
		{
			name: "received all api shutdown signals",
			intervals: []monitorapi.EventInterval{
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-0",
						Message: "reason/NodeUpdate phase/Reboot roles/control-plane,master rebooted and kubelet started",
					},
					From: timeOrDie("2023-03-14T19:25:53Z"),
					To:   timeOrDie("2023-03-14T19:26:13Z"),
				},
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "ns/openshift-kube-apiserver pod/kube-apiserver-ci-op-q4mkb8xx-ed5cd-dmqtq-master-0 node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-0",
						Message: "reason/ShutdownInitiated Received signal to terminate, becoming unready, but keeping serving",
					},
					From: timeOrDie("2023-03-14T19:25:54Z"),
					To:   timeOrDie("2023-03-14T19:25:54Z"),
				},
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-1",
						Message: "reason/NodeUpdate phase/Reboot roles/control-plane,master rebooted and kubelet started",
					},
					From: timeOrDie("2023-03-14T19:29:50Z"),
					To:   timeOrDie("2023-03-14T19:31:41Z"),
				},
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "ns/openshift-kube-apiserver pod/kube-apiserver-ci-op-q4mkb8xx-ed5cd-dmqtq-master-1 node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-1",
						Message: "reason/ShutdownInitiated Received signal to terminate, becoming unready, but keeping serving",
					},
					From: timeOrDie("2023-03-14T19:29:54Z"),
					To:   timeOrDie("2023-03-14T19:29:54Z"),
				},
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-2",
						Message: "reason/NodeUpdate phase/Reboot roles/control-plane,master rebooted and kubelet started",
					},
					From: timeOrDie("2023-03-14T19:33:57Z"),
					To:   timeOrDie("2023-03-14T19:34:19Z"),
				},
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "ns/openshift-kube-apiserver pod/kube-apiserver-ci-op-q4mkb8xx-ed5cd-dmqtq-master-2 node/ci-op-q4mkb8xx-ed5cd-dmqtq-master-2",
						Message: "reason/ShutdownInitiated Received signal to terminate, becoming unready, but keeping serving",
					},
					From: timeOrDie("2023-03-14T19:33:59Z"),
					To:   timeOrDie("2023-03-14T19:33:59Z"),
				},
				{
					Condition: monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: "e2e-test/\"[sig-arch][Feature:ClusterUpgrade] Cluster should remain functional during upgrade [Disruptive] [Serial]\"",
						Message: "finishedStatus/Failed",
					},
					From: timeOrDie("2023-03-14T19:51:00Z"),
					To:   timeOrDie("2023-03-14T19:51:00Z"),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			junits := testAPIServerRecievedShutdownSignal(test.intervals)

			if len(junits) < 1 {
				t.Fatal("didn't get junit for api shutdown signal event")
			}

			if len(test.expectedMessage) == 0 {
				if junits[0].FailureOutput != nil {
					t.Fatalf("expected output to match, but it didn't: %s.\nExpected no error output\nReceived:\n%s\n", test.name, junits[0].FailureOutput.Output)
				}
			} else if strings.Compare(junits[0].FailureOutput.Output, test.expectedMessage) != 0 {
				t.Fatalf("expected output to match, but it didn't: %s.\nExpected:\n%s\nReceived:\n%s\n", test.name, test.expectedMessage, junits[0].FailureOutput.Output)
			}
		})
	}
}

package synthetictests

import "testing"

func TestEventCountExtractor(t *testing.T) {
	const sampleDisplay = `pod/network-check-target-5f44k node/ip-10-0-210-155.us-west-2.compute.internal - reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started? (24 times)`

	tests := []struct {
		name    string
		input   string
		message string
		times   int
	}{
		{
			name:    "simple",
			input:   sampleDisplay,
			message: `pod/network-check-target-5f44k node/ip-10-0-210-155.us-west-2.compute.internal - reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started? `,
			times:   24,
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

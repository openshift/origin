package ginkgo

import (
	"reflect"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func Test_markMissedPathologicalEvents(t *testing.T) {
	type args struct {
		events        monitorapi.Intervals
		mutatedEvents monitorapi.Intervals
	}

	from := time.Now()
	to := from.Add(1 * time.Second)

	tests := []struct {
		name string
		args args
	}{
		{
			name: "two pathos, one previous event each",
			args: args{
				events: []monitorapi.Interval{
					{
						Condition: monitorapi.Condition{
							Locator: "node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0",
							Message: "reason/ErrorReconcilingNode roles/control-plane,master [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0, macAddress annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\"] (19 times)",
						},
						From: from,
						To:   to,
					},
					{
						Condition: monitorapi.Condition{
							Locator: "ns/openshift-kube-controller-manager pod/revision-pruner-6-ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0",
							Message: "reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started? (8 times)",
						},
						From: from.Add(-90 * time.Second),
						To:   to.Add(-89 * time.Second),
					},
					{
						Condition: monitorapi.Condition{
							Locator: "node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 hmsg/f33a7e39ac",
							Message: "pathological/true reason/ErrorReconcilingNode roles/control-plane,master [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0, macAddress annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\"] (21 times)",
						},
						From: from.Add(-100 * time.Second),
						To:   to.Add(-95 * time.Second),
					},
					{
						Condition: monitorapi.Condition{
							Locator: "ns/openshift-kube-controller-manager pod/revision-pruner-6-ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 hmsg/c6151e47e4",
							Message: "pathological/true reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started? (153 times)",
						},
						From: from.Add(-110 * time.Second),
						To:   to.Add(-105 * time.Second),
					},
				},
				mutatedEvents: []monitorapi.Interval{
					{
						Condition: monitorapi.Condition{
							Locator: "node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 hmsg/f33a7e39ac",
							Message: "pathological/true reason/ErrorReconcilingNode roles/control-plane,master [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0, macAddress annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\"] (19 times)",
						},
						From: from,
						To:   to,
					},
					{
						Condition: monitorapi.Condition{
							Locator: "ns/openshift-kube-controller-manager pod/revision-pruner-6-ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 hmsg/c6151e47e4",
							Message: "pathological/true reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started? (8 times)",
						},
						From: from.Add(-90 * time.Second),
						To:   to.Add(-89 * time.Second),
					},
					{
						Condition: monitorapi.Condition{
							Locator: "node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 hmsg/f33a7e39ac",
							Message: "pathological/true reason/ErrorReconcilingNode roles/control-plane,master [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0, macAddress annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\"] (21 times)",
						},
						From: from.Add(-100 * time.Second),
						To:   to.Add(-95 * time.Second),
					},
					{
						Condition: monitorapi.Condition{
							Locator: "ns/openshift-kube-controller-manager pod/revision-pruner-6-ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 hmsg/c6151e47e4",
							Message: "pathological/true reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started? (153 times)",
						},
						From: from.Add(-110 * time.Second),
						To:   to.Add(-105 * time.Second),
					},
				},
			},
		},
		{
			name: "locatorMatch, msgDifferent",
			args: args{
				events: []monitorapi.Interval{
					{
						Condition: monitorapi.Condition{
							Locator: "ns/openshift-kube-controller-manager pod/revision-pruner-6-ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0",
							Message: "reason/NetworkNotReady network is wacked: container runtime network is DIFFERENT. Has your DIFFERENT network provider started? (8 times)",
						},
						From: from,
						To:   to,
					},
					{
						Condition: monitorapi.Condition{
							Locator: "ns/openshift-kube-controller-manager pod/revision-pruner-6-ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 hmsg/c6151e47e4",
							Message: "pathological/true reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started? (153 times)",
						},
						From: from.Add(-90 * time.Second),
						To:   to.Add(-89 * time.Second),
					},
				},
				mutatedEvents: []monitorapi.Interval{
					{
						Condition: monitorapi.Condition{
							Locator: "ns/openshift-kube-controller-manager pod/revision-pruner-6-ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0",
							Message: "reason/NetworkNotReady network is wacked: container runtime network is DIFFERENT. Has your DIFFERENT network provider started? (8 times)",
						},
						From: from,
						To:   to,
					},
					{
						Condition: monitorapi.Condition{
							Locator: "ns/openshift-kube-controller-manager pod/revision-pruner-6-ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 hmsg/c6151e47e4",
							Message: "pathological/true reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started? (153 times)",
						},
						From: from.Add(-90 * time.Second),
						To:   to.Add(-89 * time.Second),
					},
				},
			},
		},
		{
			name: "locatorDifferent, msgMatch",
			args: args{
				events: []monitorapi.Interval{
					{
						Condition: monitorapi.Condition{
							Locator: "node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-DIFFERENT",
							Message: "reason/ErrorReconcilingNode roles/control-plane,master [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0, macAddress annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\"] (19 times)",
						},
						From: from.Add(-100 * time.Second),
						To:   to.Add(-95 * time.Second),
					},
					{
						Condition: monitorapi.Condition{
							Locator: "node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 hmsg/f33a7e39ac",
							Message: "pathological/true reason/ErrorReconcilingNode roles/control-plane,master [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0, macAddress annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\"] (21 times)",
						},
						From: from.Add(-110 * time.Second),
						To:   to.Add(-105 * time.Second),
					},
				},
				mutatedEvents: []monitorapi.Interval{
					{
						Condition: monitorapi.Condition{
							Locator: "node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-DIFFERENT",
							Message: "reason/ErrorReconcilingNode roles/control-plane,master [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0, macAddress annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\"] (19 times)",
						},
						From: from.Add(-100 * time.Second),
						To:   to.Add(-95 * time.Second),
					},
					{
						Condition: monitorapi.Condition{
							Locator: "node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 hmsg/f33a7e39ac",
							Message: "pathological/true reason/ErrorReconcilingNode roles/control-plane,master [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0, macAddress annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\"] (21 times)",
						},
						From: from.Add(-110 * time.Second),
						To:   to.Add(-105 * time.Second),
					},
				},
			},
		},
		{
			name: "no patho events",
			args: args{
				events: []monitorapi.Interval{
					{
						Condition: monitorapi.Condition{
							Locator: "node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0",
							Message: "reason/ErrorReconcilingNode roles/control-plane,master [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0, macAddress annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\"] (5 times)",
						},
						From: from.Add(-100 * time.Second),
						To:   to.Add(-95 * time.Second),
					},
					{
						Condition: monitorapi.Condition{
							Locator: "node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-1",
							Message: "reason/ErrorReconcilingNode roles/control-plane,master [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0, macAddress annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-1\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\"] (4 times)",
						},
						From: from.Add(-110 * time.Second),
						To:   to.Add(-105 * time.Second),
					},
				},
				mutatedEvents: []monitorapi.Interval{
					{
						Condition: monitorapi.Condition{
							Locator: "node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0",
							Message: "reason/ErrorReconcilingNode roles/control-plane,master [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0, macAddress annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\"] (5 times)",
						},
						From: from.Add(-100 * time.Second),
						To:   to.Add(-95 * time.Second),
					},
					{
						Condition: monitorapi.Condition{
							Locator: "node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-1",
							Message: "reason/ErrorReconcilingNode roles/control-plane,master [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0, macAddress annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-1\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\"] (4 times)",
						},
						From: from.Add(-110 * time.Second),
						To:   to.Add(-105 * time.Second),
					},
				},
			},
		},
		{
			name: "two pathos (with one already known), one previous event each",
			args: args{
				events: []monitorapi.Interval{
					{
						Condition: monitorapi.Condition{
							Locator: "node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0",
							Message: "reason/ErrorReconcilingNode roles/control-plane,master [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0, macAddress annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\"] (19 times)",
						},
						From: from,
						To:   to,
					},
					{
						Condition: monitorapi.Condition{
							Locator: "ns/openshift-kube-controller-manager pod/revision-pruner-6-ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0",
							Message: "reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started? (8 times)",
						},
						From: from.Add(-90 * time.Second),
						To:   to.Add(-89 * time.Second),
					},
					{
						Condition: monitorapi.Condition{
							Locator: "node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 hmsg/f33a7e39ac",
							Message: "pathological/true reason/ErrorReconcilingNode roles/control-plane,master [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0, macAddress annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\"] (21 times)",
						},
						From: from.Add(-100 * time.Second),
						To:   to.Add(-95 * time.Second),
					},
					{
						Condition: monitorapi.Condition{
							Locator: "ns/openshift-kube-controller-manager pod/revision-pruner-6-ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 hmsg/c6151e47e4",
							Message: "pathological/true interesting/true reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started? (153 times)",
						},
						From: from.Add(-110 * time.Second),
						To:   to.Add(-105 * time.Second),
					},
				},
				mutatedEvents: []monitorapi.Interval{
					{
						Condition: monitorapi.Condition{
							Locator: "node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 hmsg/f33a7e39ac",
							Message: "pathological/true reason/ErrorReconcilingNode roles/control-plane,master [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0, macAddress annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\"] (19 times)",
						},
						From: from,
						To:   to,
					},
					{
						Condition: monitorapi.Condition{
							Locator: "ns/openshift-kube-controller-manager pod/revision-pruner-6-ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0",
							Message: "reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started? (8 times)",
						},
						From: from.Add(-90 * time.Second),
						To:   to.Add(-89 * time.Second),
					},
					{
						Condition: monitorapi.Condition{
							Locator: "node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 hmsg/f33a7e39ac",
							Message: "pathological/true reason/ErrorReconcilingNode roles/control-plane,master [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0, macAddress annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\" , k8s.ovn.org/l3-gateway-config annotation not found for node \"ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0\"] (21 times)",
						},
						From: from.Add(-100 * time.Second),
						To:   to.Add(-95 * time.Second),
					},
					{
						Condition: monitorapi.Condition{
							Locator: "ns/openshift-kube-controller-manager pod/revision-pruner-6-ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 node/ci-op-i20psv8m-6a467-xftbs-master-j6mzw-0 hmsg/c6151e47e4",
							Message: "pathological/true interesting/true reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net.d/. Has your network provider started? (153 times)",
						},
						From: from.Add(-110 * time.Second),
						To:   to.Add(-105 * time.Second),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markMissedPathologicalEvents(tt.args.events)

			eventLength := len(tt.args.events)
			mutatedEventLength := len(tt.args.mutatedEvents)
			if eventLength != mutatedEventLength {
				t.Errorf("original and mutated events should be same size, eventLength = %d, mutatedEventLength = %d", eventLength, mutatedEventLength)
			}
			for i := 0; i < eventLength; i++ {
				if !reflect.DeepEqual(tt.args.events[i], tt.args.mutatedEvents[i]) {
					t.Errorf("Events not properly mutated on index = %d, name = %s", i, tt.name)
				}
			}
		})
	}
}

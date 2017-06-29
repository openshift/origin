package plugin

import (
	"testing"

	osapi "github.com/openshift/origin/pkg/sdn/apis/network"
)

func Test_clusterNetworkChanged(t *testing.T) {
	origCN := osapi.ClusterNetwork{
		Network:          "10.128.0.0/14",
		HostSubnetLength: 10,
		ServiceNetwork:   "172.30.0.0/16",
		PluginName:       "redhat/openshift-ovs-subnet",
	}

	tests := []struct {
		name        string
		changes     *osapi.ClusterNetwork
		expectError bool
	}{
		{
			name:        "no change",
			changes:     &osapi.ClusterNetwork{},
			expectError: false,
		},
		{
			name: "larger Network",
			changes: &osapi.ClusterNetwork{
				Network: "10.128.0.0/12",
			},
			expectError: false,
		},
		{
			name: "larger Network",
			changes: &osapi.ClusterNetwork{
				Network: "10.0.0.0/8",
			},
			expectError: false,
		},
		{
			name: "smaller Network",
			changes: &osapi.ClusterNetwork{
				Network: "10.128.0.0/15",
			},
			expectError: true,
		},
		{
			name: "moved Network",
			changes: &osapi.ClusterNetwork{
				Network: "10.1.0.0/16",
			},
			expectError: true,
		},
		{
			name: "larger HostSubnetLength",
			changes: &osapi.ClusterNetwork{
				HostSubnetLength: 11,
			},
			expectError: true,
		},
		{
			name: "smaller HostSubnetLength",
			changes: &osapi.ClusterNetwork{
				HostSubnetLength: 9,
			},
			expectError: true,
		},
		{
			name: "larger ServiceNetwork",
			changes: &osapi.ClusterNetwork{
				ServiceNetwork: "172.30.0.0/15",
			},
			expectError: true,
		},
		{
			name: "smaller ServiceNetwork",
			changes: &osapi.ClusterNetwork{
				ServiceNetwork: "172.30.0.0/17",
			},
			expectError: true,
		},
		{
			name: "moved ServiceNetwork",
			changes: &osapi.ClusterNetwork{
				ServiceNetwork: "192.168.0.0/16",
			},
			expectError: true,
		},
		{
			name: "changed PluginName",
			changes: &osapi.ClusterNetwork{
				PluginName: "redhat/openshift-ovs-multitenant",
			},
			expectError: false,
		},
	}

	for _, test := range tests {
		newCN := origCN
		expectChanged := false
		if test.changes.Network != "" {
			newCN.Network = test.changes.Network
			expectChanged = true
		}
		if test.changes.HostSubnetLength != 0 {
			newCN.HostSubnetLength = test.changes.HostSubnetLength
			expectChanged = true
		}
		if test.changes.ServiceNetwork != "" {
			newCN.ServiceNetwork = test.changes.ServiceNetwork
			expectChanged = true
		}
		if test.changes.PluginName != "" {
			newCN.PluginName = test.changes.PluginName
			expectChanged = true
		}

		changed, err := clusterNetworkChanged(&newCN, &origCN)
		if changed != expectChanged {
			t.Fatalf("unexpected result (%t instead of %t) on %q: %s -> %s", changed, expectChanged, test.name, clusterNetworkToString(&origCN), clusterNetworkToString(&newCN))
		}
		if (err != nil) != test.expectError {
			t.Fatalf("unexpected error on %q: %s -> %s: %v", test.name, clusterNetworkToString(&origCN), clusterNetworkToString(&newCN), err)
		}
	}
}

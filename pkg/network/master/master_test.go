package master

import (
	"testing"

	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/network/common"
)

func Test_clusterNetworkChanged(t *testing.T) {
	origCN := networkapi.ClusterNetwork{
		ClusterNetworks: []networkapi.ClusterNetworkEntry{{CIDR: "10.128.0.0/14", HostSubnetLength: 10}},
		ServiceNetwork:  "172.30.0.0/16",
		PluginName:      "redhat/openshift-ovs-subnet",
	}

	tests := []struct {
		name        string
		changes     *networkapi.ClusterNetwork
		expectError bool
	}{
		{
			name:        "no change",
			changes:     &networkapi.ClusterNetwork{},
			expectError: false,
		},
		{
			name: "larger Network",
			changes: &networkapi.ClusterNetwork{
				ClusterNetworks: []networkapi.ClusterNetworkEntry{{CIDR: "10.128.0.0/12"}},
			},
			expectError: false,
		},
		{
			name: "larger ServiceNetwork",
			changes: &networkapi.ClusterNetwork{
				ServiceNetwork: "172.30.0.0/15",
			},
			expectError: true,
		},
		{
			name: "smaller ServiceNetwork",
			changes: &networkapi.ClusterNetwork{
				ServiceNetwork: "172.30.0.0/17",
			},
			expectError: true,
		},
		{
			name: "moved ServiceNetwork",
			changes: &networkapi.ClusterNetwork{
				ServiceNetwork: "192.168.0.0/16",
			},
			expectError: true,
		},
		{
			name: "changed PluginName",
			changes: &networkapi.ClusterNetwork{
				PluginName: "redhat/openshift-ovs-multitenant",
			},
			expectError: false,
		},
	}

	for _, test := range tests {
		newCN := origCN
		expectChanged := false
		if test.changes.ClusterNetworks != nil {
			newCN.ClusterNetworks = test.changes.ClusterNetworks
			expectChanged = true
		}
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
			t.Fatalf("unexpected result (%t instead of %t) on %q: %s -> %s", changed, expectChanged, test.name, common.ClusterNetworkToString(&origCN), common.ClusterNetworkToString(&newCN))
		}
		if (err != nil) != test.expectError {
			t.Fatalf("unexpected error on %q: %s -> %s: %v", test.name, common.ClusterNetworkToString(&origCN), common.ClusterNetworkToString(&newCN), err)
		}
	}
}

package common

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networkapi "github.com/openshift/api/network/v1"
)

func TestValidateClusterNetwork(t *testing.T) {
	tests := []struct {
		name           string
		cn             *networkapi.ClusterNetwork
		expectedErrors int
	}{
		{
			name: "Good one",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:      metav1.ObjectMeta{Name: "any"},
				ClusterNetworks: []networkapi.ClusterNetworkEntry{{CIDR: "10.20.0.0/16", HostSubnetLength: 8}},
				ServiceNetwork:  "172.30.0.0/16",
			},
			expectedErrors: 0,
		},
		{
			name: "Good one old network and hostsubnetlength set",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: "any"},
				Network:          "10.20.0.0/16",
				HostSubnetLength: 8,
				ClusterNetworks:  []networkapi.ClusterNetworkEntry{{CIDR: "10.20.0.0/16", HostSubnetLength: 8}},
				ServiceNetwork:   "172.30.0.0/16",
			},
			expectedErrors: 0,
		},
		{
			name: "old network set incorrectly",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: "any"},
				Network:          "10.30.0.0/16",
				HostSubnetLength: 8,
				ClusterNetworks:  []networkapi.ClusterNetworkEntry{{CIDR: "10.20.0.0/16", HostSubnetLength: 8}},
				ServiceNetwork:   "172.30.0.0/16",
			},
			expectedErrors: 1,
		},
		{
			name: "old hostsubnetlength set incorrectly",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: "any"},
				Network:          "10.20.0.0/16",
				HostSubnetLength: 9,
				ClusterNetworks:  []networkapi.ClusterNetworkEntry{{CIDR: "10.20.0.0/16", HostSubnetLength: 8}},
				ServiceNetwork:   "172.30.0.0/16",
			},
			expectedErrors: 1,
		},
		{
			name: "only old network set",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:      metav1.ObjectMeta{Name: "any"},
				Network:         "10.20.0.0/16",
				ClusterNetworks: []networkapi.ClusterNetworkEntry{{CIDR: "10.20.0.0/16", HostSubnetLength: 8}},
				ServiceNetwork:  "172.30.0.0/16",
			},
			expectedErrors: 1,
		},
		{
			name: "only old hostsubnetlength set",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: "any"},
				HostSubnetLength: 8,
				ClusterNetworks:  []networkapi.ClusterNetworkEntry{{CIDR: "10.20.0.0/16", HostSubnetLength: 8}},
				ServiceNetwork:   "172.30.0.0/16",
			},
			expectedErrors: 1,
		},
		{
			name: "Good one multiple addresses",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:      metav1.ObjectMeta{Name: "any"},
				ClusterNetworks: []networkapi.ClusterNetworkEntry{{CIDR: "10.20.0.0/16", HostSubnetLength: 8}, {CIDR: "10.128.0.0/16", HostSubnetLength: 8}},
				ServiceNetwork:  "172.30.0.0/16",
			},
			expectedErrors: 0,
		},
		{
			name: "Bad network",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:      metav1.ObjectMeta{Name: "any"},
				ClusterNetworks: []networkapi.ClusterNetworkEntry{{CIDR: "10.20.0.0.0/16", HostSubnetLength: 8}},
				ServiceNetwork:  "172.30.0.0/16",
			},
			expectedErrors: 1,
		},
		{
			name: "Bad network CIDR",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:      metav1.ObjectMeta{Name: "any"},
				ClusterNetworks: []networkapi.ClusterNetworkEntry{{CIDR: "10.20.0.1/16", HostSubnetLength: 8}},
				ServiceNetwork:  "172.30.0.0/16",
			},
			expectedErrors: 1,
		},
		{
			name: "Empty network ClusterNetworks",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:     metav1.ObjectMeta{Name: "any"},
				ServiceNetwork: "172.30.0.0/16",
			},
			expectedErrors: 1,
		},
		{
			name: "Subnet length too large for network",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:      metav1.ObjectMeta{Name: "any"},
				ClusterNetworks: []networkapi.ClusterNetworkEntry{{CIDR: "10.20.30.0/24", HostSubnetLength: 16}},
				ServiceNetwork:  "172.30.0.0/16",
			},
			expectedErrors: 1,
		},
		{
			name: "Subnet length too small",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:      metav1.ObjectMeta{Name: "any"},
				ClusterNetworks: []networkapi.ClusterNetworkEntry{{CIDR: "10.20.0.0/16", HostSubnetLength: 1}},
				ServiceNetwork:  "172.30.0.0/16",
			},
			expectedErrors: 1,
		},
		{
			name: "Bad service network",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:      metav1.ObjectMeta{Name: "any"},
				ClusterNetworks: []networkapi.ClusterNetworkEntry{{CIDR: "10.20.0.0/16", HostSubnetLength: 8}},
				ServiceNetwork:  "1172.30.0.0/16",
			},
			expectedErrors: 1,
		},
		{
			name: "Bad service network CIDR",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:      metav1.ObjectMeta{Name: "any"},
				ClusterNetworks: []networkapi.ClusterNetworkEntry{{CIDR: "10.20.0.0/16", HostSubnetLength: 8}},
				ServiceNetwork:  "172.30.1.0/16",
			},
			expectedErrors: 1,
		},
		{
			name: "Service network overlaps with cluster network",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:      metav1.ObjectMeta{Name: "any"},
				ClusterNetworks: []networkapi.ClusterNetworkEntry{{CIDR: "10.20.0.0/16", HostSubnetLength: 8}},
				ServiceNetwork:  "10.20.1.0/24",
			},
			expectedErrors: 1,
		},
		{
			name: "Cluster network overlaps with service network",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:      metav1.ObjectMeta{Name: "any"},
				ClusterNetworks: []networkapi.ClusterNetworkEntry{{CIDR: "10.20.0.0/16", HostSubnetLength: 8}},
				ServiceNetwork:  "10.0.0.0/8",
			},
			expectedErrors: 1,
		},
		{
			name: "Cluster networks overlap with each other",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:      metav1.ObjectMeta{Name: "any"},
				ClusterNetworks: []networkapi.ClusterNetworkEntry{{CIDR: "10.128.0.0/14", HostSubnetLength: 8}, {CIDR: "10.0.0.0/8", HostSubnetLength: 8}},
				ServiceNetwork:  "172.30.0.0/16",
			},
			expectedErrors: 1,
		},
		{
			name: "IPv6 ClusterNetwork",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:      metav1.ObjectMeta{Name: "any"},
				ClusterNetworks: []networkapi.ClusterNetworkEntry{{CIDR: "fe80:1234::/64", HostSubnetLength: 8}},
				ServiceNetwork:  "172.30.0.0/16",
			},
			expectedErrors: 1,
		},
		{
			name: "IPv6 ServiceNetwork",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:      metav1.ObjectMeta{Name: "any"},
				ClusterNetworks: []networkapi.ClusterNetworkEntry{{CIDR: "10.20.0.0/16", HostSubnetLength: 8}},
				ServiceNetwork:  "fe80:1234::/64",
			},
			expectedErrors: 1,
		},
	}

	for _, tc := range tests {
		err := ValidateClusterNetwork(tc.cn)

		if err == nil && tc.expectedErrors > 0 {
			t.Errorf("Test case %s expected errors, but passed", tc.name)
		} else if err != nil && tc.expectedErrors == 0 {
			t.Errorf("Test case %s expected no error, got %v", tc.name, err)
		}
	}
}

func TestValidateHostSubnet(t *testing.T) {
	tests := []struct {
		name           string
		hs             *networkapi.HostSubnet
		expectedErrors int
	}{
		{
			name: "good",
			hs: &networkapi.HostSubnet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc.def.com",
				},
				Host:   "abc.def.com",
				HostIP: "10.20.30.40",
				Subnet: "8.8.8.0/24",
			},
			expectedErrors: 0,
		},
		{
			name: "missing subnet",
			hs: &networkapi.HostSubnet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc.def.com",
				},
				Host:   "abc.def.com",
				HostIP: "10.20.30.40",
			},
			expectedErrors: 1,
		},
		{
			name: "missing subnet plus annotation",
			hs: &networkapi.HostSubnet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc.def.com",
					Annotations: map[string]string{
						"pod.network.openshift.io/assign-subnet": "true",
					},
				},
				Host:   "abc.def.com",
				HostIP: "10.20.30.40",
			},
			expectedErrors: 0,
		},
	}

	for _, tc := range tests {
		err := ValidateHostSubnet(tc.hs)

		if err == nil && tc.expectedErrors > 0 {
			t.Errorf("Test case %s expected errors, but passed", tc.name)
		} else if err != nil && tc.expectedErrors == 0 {
			t.Errorf("Test case %s expected no error, got %v", tc.name, err)
		}
	}
}

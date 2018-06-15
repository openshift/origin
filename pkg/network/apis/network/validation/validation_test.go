package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networkapi "github.com/openshift/origin/pkg/network/apis/network"
)

// TestValidateClusterNetwork ensures not specifying a required field results in error and a fully specified
// sdn passes successfully
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
		errs := ValidateClusterNetwork(tc.cn)

		if len(errs) != tc.expectedErrors {
			t.Errorf("Test case %s expected %d error(s), got %d. %v", tc.name, tc.expectedErrors, len(errs), errs)
		}
	}
}

func TestSetDefaultClusterNetwork(t *testing.T) {
	defaultClusterNetwork := networkapi.ClusterNetwork{
		ObjectMeta:       metav1.ObjectMeta{Name: networkapi.ClusterNetworkDefault},
		ClusterNetworks:  []networkapi.ClusterNetworkEntry{{CIDR: "10.20.0.0/16", HostSubnetLength: 8}},
		ServiceNetwork:   "172.30.0.0/16",
		PluginName:       "redhat/openshift-ovs-multitenant",
		Network:          "10.20.0.0/16",
		HostSubnetLength: 8,
	}
	SetDefaultClusterNetwork(defaultClusterNetwork)

	tests := []struct {
		name           string
		cn             *networkapi.ClusterNetwork
		expectedErrors int
	}{
		{
			name:           "Good one",
			cn:             &defaultClusterNetwork,
			expectedErrors: 0,
		},
		{
			name: "Wrong Network",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: networkapi.ClusterNetworkDefault},
				ClusterNetworks:  []networkapi.ClusterNetworkEntry{{CIDR: "10.30.0.0/16", HostSubnetLength: 8}},
				ServiceNetwork:   "172.30.0.0/16",
				PluginName:       "redhat/openshift-ovs-multitenant",
				Network:          "10.30.0.0/16",
				HostSubnetLength: 8,
			},
			expectedErrors: 2,
		},
		{
			name: "Additional Network",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: networkapi.ClusterNetworkDefault},
				ClusterNetworks:  []networkapi.ClusterNetworkEntry{{CIDR: "10.20.0.0/16", HostSubnetLength: 8}, {CIDR: "10.30.0.0/16", HostSubnetLength: 8}},
				ServiceNetwork:   "172.30.0.0/16",
				PluginName:       "redhat/openshift-ovs-multitenant",
				Network:          "10.20.0.0/16",
				HostSubnetLength: 8,
			},
			expectedErrors: 1,
		},
		{
			name: "Wrong HostSubnetLength",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: networkapi.ClusterNetworkDefault},
				ClusterNetworks:  []networkapi.ClusterNetworkEntry{{CIDR: "10.20.0.0/16", HostSubnetLength: 9}},
				ServiceNetwork:   "172.30.0.0/16",
				PluginName:       "redhat/openshift-ovs-multitenant",
				Network:          "10.20.0.0/16",
				HostSubnetLength: 9,
			},
			expectedErrors: 2,
		},
		{
			name: "Wrong ServiceNetwork",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: networkapi.ClusterNetworkDefault},
				ClusterNetworks:  []networkapi.ClusterNetworkEntry{{CIDR: "10.20.0.0/16", HostSubnetLength: 8}},
				ServiceNetwork:   "172.20.0.0/16",
				PluginName:       "redhat/openshift-ovs-multitenant",
				Network:          "10.20.0.0/16",
				HostSubnetLength: 8,
			},
			expectedErrors: 1,
		},
		{
			name: "Wrong PluginName",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: networkapi.ClusterNetworkDefault},
				ClusterNetworks:  []networkapi.ClusterNetworkEntry{{CIDR: "10.20.0.0/16", HostSubnetLength: 8}},
				ServiceNetwork:   "172.30.0.0/16",
				PluginName:       "redhat/openshift-ovs-subnet",
				Network:          "10.20.0.0/16",
				HostSubnetLength: 8,
			},
			expectedErrors: 1,
		},
		{
			name: "Wrong legacy Network",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: networkapi.ClusterNetworkDefault},
				ClusterNetworks:  []networkapi.ClusterNetworkEntry{{CIDR: "10.20.0.0/16", HostSubnetLength: 8}},
				ServiceNetwork:   "172.30.0.0/16",
				PluginName:       "redhat/openshift-ovs-multitenant",
				Network:          "10.30.0.0/16",
				HostSubnetLength: 8,
			},
			expectedErrors: 2,
		},
		{
			name: "Missing legacy fields",
			cn: &networkapi.ClusterNetwork{
				ObjectMeta:      metav1.ObjectMeta{Name: networkapi.ClusterNetworkDefault},
				ClusterNetworks: []networkapi.ClusterNetworkEntry{{CIDR: "10.20.0.0/16", HostSubnetLength: 8}},
				ServiceNetwork:  "172.30.0.0/16",
				PluginName:      "redhat/openshift-ovs-multitenant",
			},
			expectedErrors: 4,
		},
	}

	for _, tc := range tests {
		errs := ValidateClusterNetwork(tc.cn)

		if len(errs) != tc.expectedErrors {
			t.Errorf("Test case %s expected %d error(s), got %d. %v", tc.name, tc.expectedErrors, len(errs), errs)
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
			name: "Good one",
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
			name: "Malformed HostIP",
			hs: &networkapi.HostSubnet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc.def.com",
				},
				Host:   "abc.def.com",
				HostIP: "10.20.300.40",
				Subnet: "8.8.0.0/24",
			},
			expectedErrors: 1,
		},
		{
			name: "Malformed subnet",
			hs: &networkapi.HostSubnet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc.def.com",
				},
				Host:   "abc.def.com",
				HostIP: "10.20.30.40",
				Subnet: "8.8.0/24",
			},
			expectedErrors: 1,
		},
		{
			name: "Malformed subnet CIDR",
			hs: &networkapi.HostSubnet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc.def.com",
				},
				Host:   "abc.def.com",
				HostIP: "10.20.30.40",
				Subnet: "8.8.0.1/24",
			},
			expectedErrors: 1,
		},
		{
			name: "Good one with EgressIPs",
			hs: &networkapi.HostSubnet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc.def.com",
				},
				Host:   "abc.def.com",
				HostIP: "10.20.30.40",
				Subnet: "8.8.8.0/24",
				EgressIPs: []string{
					"192.168.1.99",
					"192.168.2.42",
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Malformed EgressIPs",
			hs: &networkapi.HostSubnet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc.def.com",
				},
				Host:   "abc.def.com",
				HostIP: "10.20.30.40",
				Subnet: "8.8.8.0/24",
				EgressIPs: []string{
					"192.168.1.0/24",
					"bob",
				},
			},
			expectedErrors: 2,
		},
		{
			name: "IPv6 subnet",
			hs: &networkapi.HostSubnet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc.def.com",
				},
				Host:   "abc.def.com",
				HostIP: "10.20.30.40",
				Subnet: "fe80:1234::/64",
			},
			expectedErrors: 1,
		},
		{
			name: "IPv6 EgressIP",
			hs: &networkapi.HostSubnet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc.def.com",
				},
				Host:   "abc.def.com",
				HostIP: "10.20.30.40",
				Subnet: "8.8.8.0/24",
				EgressIPs: []string{
					"192.168.1.99",
					"1234::5678",
				},
			},
			expectedErrors: 1,
		},
	}

	for _, tc := range tests {
		errs := ValidateHostSubnet(tc.hs)

		if len(errs) != tc.expectedErrors {
			t.Errorf("Test case %s expected %d error(s), got %d. %v", tc.name, tc.expectedErrors, len(errs), errs)
		}
	}
}

func TestValidateNetNamespace(t *testing.T) {
	tests := []struct {
		name           string
		netns          *networkapi.NetNamespace
		expectedErrors int
	}{
		{
			name: "Good one",
			netns: &networkapi.NetNamespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
				NetName: "abc",
				NetID:   12345,
			},
			expectedErrors: 0,
		},
		{
			name: "Bad NetName",
			netns: &networkapi.NetNamespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
				NetName: "wrong",
				NetID:   12345,
			},
			expectedErrors: 1,
		},
		{
			name: "NetID too large",
			netns: &networkapi.NetNamespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
				NetName: "abc",
				NetID:   16777217,
			},
			expectedErrors: 1,
		},
		{
			name: "NetID in reserved range",
			netns: &networkapi.NetNamespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
				NetName: "abc",
				NetID:   5,
			},
			expectedErrors: 1,
		},
		{
			name: "Good with EgressIPs",
			netns: &networkapi.NetNamespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
				NetName:   "abc",
				NetID:     12345,
				EgressIPs: []string{"192.168.1.5", "192.168.1.7"},
			},
			expectedErrors: 0,
		},
		{
			name: "Bad EgressIPs",
			netns: &networkapi.NetNamespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc",
				},
				NetName:   "abc",
				NetID:     12345,
				EgressIPs: []string{"example.com", "", "1234::5678"},
			},
			expectedErrors: 3,
		},
	}

	for _, tc := range tests {
		errs := ValidateNetNamespace(tc.netns)

		if len(errs) != tc.expectedErrors {
			t.Errorf("Test case %s expected %d error(s), got %d. %v", tc.name, tc.expectedErrors, len(errs), errs)
		}
	}
}

func TestValidateEgressNetworkPolicy(t *testing.T) {
	tests := []struct {
		name           string
		fw             *networkapi.EgressNetworkPolicy
		expectedErrors int
	}{
		{
			name: "Empty",
			fw: &networkapi.EgressNetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "testing",
				},
				Spec: networkapi.EgressNetworkPolicySpec{
					Egress: []networkapi.EgressNetworkPolicyRule{},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Good one",
			fw: &networkapi.EgressNetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "testing",
				},
				Spec: networkapi.EgressNetworkPolicySpec{
					Egress: []networkapi.EgressNetworkPolicyRule{
						{
							Type: networkapi.EgressNetworkPolicyRuleAllow,
							To: networkapi.EgressNetworkPolicyPeer{
								CIDRSelector: "1.2.3.0/24",
							},
						},
						{
							Type: networkapi.EgressNetworkPolicyRuleAllow,
							To: networkapi.EgressNetworkPolicyPeer{
								DNSName: "www.example.com",
							},
						},
						{
							Type: networkapi.EgressNetworkPolicyRuleDeny,
							To: networkapi.EgressNetworkPolicyPeer{
								CIDRSelector: "1.2.3.4/32",
							},
						},
						{
							Type: networkapi.EgressNetworkPolicyRuleDeny,
							To: networkapi.EgressNetworkPolicyPeer{
								DNSName: "www.foo.com",
							},
						},
					},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Bad policy",
			fw: &networkapi.EgressNetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "testing",
				},
				Spec: networkapi.EgressNetworkPolicySpec{
					Egress: []networkapi.EgressNetworkPolicyRule{
						{
							Type: networkapi.EgressNetworkPolicyRuleType("Bob"),
							To: networkapi.EgressNetworkPolicyPeer{
								CIDRSelector: "1.2.3.0/24",
							},
						},
						{
							Type: networkapi.EgressNetworkPolicyRuleDeny,
							To: networkapi.EgressNetworkPolicyPeer{
								CIDRSelector: "1.2.3.4/32",
							},
						},
					},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Bad destination",
			fw: &networkapi.EgressNetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "testing",
				},
				Spec: networkapi.EgressNetworkPolicySpec{
					Egress: []networkapi.EgressNetworkPolicyRule{
						{
							Type: networkapi.EgressNetworkPolicyRuleAllow,
							To: networkapi.EgressNetworkPolicyPeer{
								CIDRSelector: "1.2.3.4",
							},
						},
						{
							Type: networkapi.EgressNetworkPolicyRuleDeny,
							To: networkapi.EgressNetworkPolicyPeer{
								CIDRSelector: "",
							},
						},
					},
				},
			},
			expectedErrors: 2,
		},
		{
			name: "Policy rule with both CIDR and DNS",
			fw: &networkapi.EgressNetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "testing",
				},
				Spec: networkapi.EgressNetworkPolicySpec{
					Egress: []networkapi.EgressNetworkPolicyRule{
						{
							Type: networkapi.EgressNetworkPolicyRuleAllow,
							To: networkapi.EgressNetworkPolicyPeer{
								CIDRSelector: "1.2.3.4",
								DNSName:      "www.example.com",
							},
						},
					},
				},
			},
			expectedErrors: 2,
		},
		{
			name: "Policy rule without CIDR or DNS",
			fw: &networkapi.EgressNetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "testing",
				},
				Spec: networkapi.EgressNetworkPolicySpec{
					Egress: []networkapi.EgressNetworkPolicyRule{
						{
							Type: networkapi.EgressNetworkPolicyRuleAllow,
							To:   networkapi.EgressNetworkPolicyPeer{},
						},
					},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Policy rule with invalid DNS",
			fw: &networkapi.EgressNetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "testing",
				},
				Spec: networkapi.EgressNetworkPolicySpec{
					Egress: []networkapi.EgressNetworkPolicyRule{
						{
							Type: networkapi.EgressNetworkPolicyRuleAllow,
							To: networkapi.EgressNetworkPolicyPeer{
								DNSName: "www.Example$.com",
							},
						},
					},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Policy rule with wildcard DNS",
			fw: &networkapi.EgressNetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "testing",
				},
				Spec: networkapi.EgressNetworkPolicySpec{
					Egress: []networkapi.EgressNetworkPolicyRule{
						{
							Type: networkapi.EgressNetworkPolicyRuleAllow,
							To: networkapi.EgressNetworkPolicyPeer{
								DNSName: "*.example.com",
							},
						},
					},
				},
			},
			expectedErrors: 1,
		},
		{
			// IPv6 CIDRSelectors are currently useless, but they don't break anything
			name: "IPv6 CIDR",
			fw: &networkapi.EgressNetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "testing",
				},
				Spec: networkapi.EgressNetworkPolicySpec{
					Egress: []networkapi.EgressNetworkPolicyRule{
						{
							Type: networkapi.EgressNetworkPolicyRuleAllow,
							To: networkapi.EgressNetworkPolicyPeer{
								CIDRSelector: "fe80::/64",
							},
						},
					},
				},
			},
			expectedErrors: 0,
		},
	}

	for _, tc := range tests {
		errs := ValidateEgressNetworkPolicy(tc.fw)

		if len(errs) != tc.expectedErrors {
			t.Errorf("Test case %s expected %d error(s), got %d. %v", tc.name, tc.expectedErrors, len(errs), errs)
		}
	}
}

package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sdnapi "github.com/openshift/origin/pkg/sdn/apis/network"
)

// TestValidateClusterNetwork ensures not specifying a required field results in error and a fully specified
// sdn passes successfully
func TestValidateClusterNetwork(t *testing.T) {
	tests := []struct {
		name           string
		cn             *sdnapi.ClusterNetwork
		expectedErrors int
	}{
		{
			name: "Good one",
			cn: &sdnapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: "any"},
				Network:          "10.20.0.0/16",
				HostSubnetLength: 8,
				ServiceNetwork:   "172.30.0.0/16",
			},
			expectedErrors: 0,
		},
		{
			name: "Bad network",
			cn: &sdnapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: "any"},
				Network:          "10.20.0.0.0/16",
				HostSubnetLength: 8,
				ServiceNetwork:   "172.30.0.0/16",
			},
			expectedErrors: 1,
		},
		{
			name: "Bad network CIDR",
			cn: &sdnapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: "any"},
				Network:          "10.20.0.1/16",
				HostSubnetLength: 8,
				ServiceNetwork:   "172.30.0.0/16",
			},
			expectedErrors: 1,
		},
		{
			name: "Subnet length too large for network",
			cn: &sdnapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: "any"},
				Network:          "10.20.30.0/24",
				HostSubnetLength: 16,
				ServiceNetwork:   "172.30.0.0/16",
			},
			expectedErrors: 1,
		},
		{
			name: "Subnet length too small",
			cn: &sdnapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: "any"},
				Network:          "10.20.30.0/24",
				HostSubnetLength: 1,
				ServiceNetwork:   "172.30.0.0/16",
			},
			expectedErrors: 1,
		},
		{
			name: "Bad service network",
			cn: &sdnapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: "any"},
				Network:          "10.20.0.0/16",
				HostSubnetLength: 8,
				ServiceNetwork:   "1172.30.0.0/16",
			},
			expectedErrors: 1,
		},
		{
			name: "Bad service network CIDR",
			cn: &sdnapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: "any"},
				Network:          "10.20.0.0/16",
				HostSubnetLength: 8,
				ServiceNetwork:   "172.30.1.0/16",
			},
			expectedErrors: 1,
		},
		{
			name: "Service network overlaps with cluster network",
			cn: &sdnapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: "any"},
				Network:          "10.20.0.0/16",
				HostSubnetLength: 8,
				ServiceNetwork:   "10.20.1.0/24",
			},
			expectedErrors: 1,
		},
		{
			name: "Cluster network overlaps with service network",
			cn: &sdnapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: "any"},
				Network:          "10.20.0.0/16",
				HostSubnetLength: 8,
				ServiceNetwork:   "10.0.0.0/8",
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
	defaultClusterNetwork := sdnapi.ClusterNetwork{
		ObjectMeta:       metav1.ObjectMeta{Name: sdnapi.ClusterNetworkDefault},
		Network:          "10.20.0.0/16",
		HostSubnetLength: 8,
		ServiceNetwork:   "172.30.0.0/16",
		PluginName:       "redhat/openshift-ovs-multitenant",
	}
	SetDefaultClusterNetwork(defaultClusterNetwork)

	tests := []struct {
		name           string
		cn             *sdnapi.ClusterNetwork
		expectedErrors int
	}{
		{
			name:           "Good one",
			cn:             &defaultClusterNetwork,
			expectedErrors: 0,
		},
		{
			name: "Wrong Network",
			cn: &sdnapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: sdnapi.ClusterNetworkDefault},
				Network:          "10.30.0.0/16",
				HostSubnetLength: 8,
				ServiceNetwork:   "172.30.0.0/16",
				PluginName:       "redhat/openshift-ovs-multitenant",
			},
			expectedErrors: 1,
		},
		{
			name: "Wrong HostSubnetLength",
			cn: &sdnapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: sdnapi.ClusterNetworkDefault},
				Network:          "10.20.0.0/16",
				HostSubnetLength: 9,
				ServiceNetwork:   "172.30.0.0/16",
				PluginName:       "redhat/openshift-ovs-multitenant",
			},
			expectedErrors: 1,
		},
		{
			name: "Wrong ServiceNetwork",
			cn: &sdnapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: sdnapi.ClusterNetworkDefault},
				Network:          "10.20.0.0/16",
				HostSubnetLength: 8,
				ServiceNetwork:   "172.20.0.0/16",
				PluginName:       "redhat/openshift-ovs-multitenant",
			},
			expectedErrors: 1,
		},
		{
			name: "Wrong PluginName",
			cn: &sdnapi.ClusterNetwork{
				ObjectMeta:       metav1.ObjectMeta{Name: sdnapi.ClusterNetworkDefault},
				Network:          "10.20.0.0/16",
				HostSubnetLength: 8,
				ServiceNetwork:   "172.30.0.0/16",
				PluginName:       "redhat/openshift-ovs-subnet",
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

func TestValidateHostSubnet(t *testing.T) {
	tests := []struct {
		name           string
		hs             *sdnapi.HostSubnet
		expectedErrors int
	}{
		{
			name: "Good one",
			hs: &sdnapi.HostSubnet{
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
			hs: &sdnapi.HostSubnet{
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
			hs: &sdnapi.HostSubnet{
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
			hs: &sdnapi.HostSubnet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "abc.def.com",
				},
				Host:   "abc.def.com",
				HostIP: "10.20.30.40",
				Subnet: "8.8.0.1/24",
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

func TestValidateEgressNetworkPolicy(t *testing.T) {
	tests := []struct {
		name           string
		fw             *sdnapi.EgressNetworkPolicy
		expectedErrors int
	}{
		{
			name: "Empty",
			fw: &sdnapi.EgressNetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "testing",
				},
				Spec: sdnapi.EgressNetworkPolicySpec{
					Egress: []sdnapi.EgressNetworkPolicyRule{},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Good one",
			fw: &sdnapi.EgressNetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "testing",
				},
				Spec: sdnapi.EgressNetworkPolicySpec{
					Egress: []sdnapi.EgressNetworkPolicyRule{
						{
							Type: sdnapi.EgressNetworkPolicyRuleAllow,
							To: sdnapi.EgressNetworkPolicyPeer{
								CIDRSelector: "1.2.3.0/24",
							},
						},
						{
							Type: sdnapi.EgressNetworkPolicyRuleAllow,
							To: sdnapi.EgressNetworkPolicyPeer{
								DNSName: "www.example.com",
							},
						},
						{
							Type: sdnapi.EgressNetworkPolicyRuleDeny,
							To: sdnapi.EgressNetworkPolicyPeer{
								CIDRSelector: "1.2.3.4/32",
							},
						},
						{
							Type: sdnapi.EgressNetworkPolicyRuleDeny,
							To: sdnapi.EgressNetworkPolicyPeer{
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
			fw: &sdnapi.EgressNetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "testing",
				},
				Spec: sdnapi.EgressNetworkPolicySpec{
					Egress: []sdnapi.EgressNetworkPolicyRule{
						{
							Type: sdnapi.EgressNetworkPolicyRuleType("Bob"),
							To: sdnapi.EgressNetworkPolicyPeer{
								CIDRSelector: "1.2.3.0/24",
							},
						},
						{
							Type: sdnapi.EgressNetworkPolicyRuleDeny,
							To: sdnapi.EgressNetworkPolicyPeer{
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
			fw: &sdnapi.EgressNetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "testing",
				},
				Spec: sdnapi.EgressNetworkPolicySpec{
					Egress: []sdnapi.EgressNetworkPolicyRule{
						{
							Type: sdnapi.EgressNetworkPolicyRuleAllow,
							To: sdnapi.EgressNetworkPolicyPeer{
								CIDRSelector: "1.2.3.4",
							},
						},
						{
							Type: sdnapi.EgressNetworkPolicyRuleDeny,
							To: sdnapi.EgressNetworkPolicyPeer{
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
			fw: &sdnapi.EgressNetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "testing",
				},
				Spec: sdnapi.EgressNetworkPolicySpec{
					Egress: []sdnapi.EgressNetworkPolicyRule{
						{
							Type: sdnapi.EgressNetworkPolicyRuleAllow,
							To: sdnapi.EgressNetworkPolicyPeer{
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
			fw: &sdnapi.EgressNetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "testing",
				},
				Spec: sdnapi.EgressNetworkPolicySpec{
					Egress: []sdnapi.EgressNetworkPolicyRule{
						{
							Type: sdnapi.EgressNetworkPolicyRuleAllow,
							To:   sdnapi.EgressNetworkPolicyPeer{},
						},
					},
				},
			},
			expectedErrors: 1,
		},
		{
			name: "Policy rule with invalid DNS",
			fw: &sdnapi.EgressNetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "testing",
				},
				Spec: sdnapi.EgressNetworkPolicySpec{
					Egress: []sdnapi.EgressNetworkPolicyRule{
						{
							Type: sdnapi.EgressNetworkPolicyRuleAllow,
							To: sdnapi.EgressNetworkPolicyPeer{
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
			fw: &sdnapi.EgressNetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "testing",
				},
				Spec: sdnapi.EgressNetworkPolicySpec{
					Egress: []sdnapi.EgressNetworkPolicyRule{
						{
							Type: sdnapi.EgressNetworkPolicyRuleAllow,
							To: sdnapi.EgressNetworkPolicyPeer{
								DNSName: "*.example.com",
							},
						},
					},
				},
			},
			expectedErrors: 1,
		},
	}

	for _, tc := range tests {
		errs := ValidateEgressNetworkPolicy(tc.fw)

		if len(errs) != tc.expectedErrors {
			t.Errorf("Test case %s expected %d error(s), got %d. %v", tc.name, tc.expectedErrors, len(errs), errs)
		}
	}
}

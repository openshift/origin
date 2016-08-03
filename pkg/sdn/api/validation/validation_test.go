package validation

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/sdn/api"
)

// TestValidateClusterNetwork ensures not specifying a required field results in error and a fully specified
// sdn passes successfully
func TestValidateClusterNetwork(t *testing.T) {
	tests := []struct {
		name           string
		cn             *api.ClusterNetwork
		expectedErrors int
	}{
		{
			name: "Good one",
			cn: &api.ClusterNetwork{
				ObjectMeta:       kapi.ObjectMeta{Name: "any"},
				Network:          "10.20.0.0/16",
				HostSubnetLength: 8,
				ServiceNetwork:   "172.30.0.0/16",
			},
			expectedErrors: 0,
		},
		{
			name: "Bad network",
			cn: &api.ClusterNetwork{
				ObjectMeta:       kapi.ObjectMeta{Name: "any"},
				Network:          "10.20.0.0.0/16",
				HostSubnetLength: 8,
				ServiceNetwork:   "172.30.0.0/16",
			},
			expectedErrors: 1,
		},
		{
			name: "Invalid subnet length",
			cn: &api.ClusterNetwork{
				ObjectMeta:       kapi.ObjectMeta{Name: "any"},
				Network:          "10.20.30.0/24",
				HostSubnetLength: 16,
				ServiceNetwork:   "172.30.0.0/16",
			},
			expectedErrors: 1,
		},
		{
			name: "Bad service network",
			cn: &api.ClusterNetwork{
				ObjectMeta:       kapi.ObjectMeta{Name: "any"},
				Network:          "10.20.0.0/16",
				HostSubnetLength: 8,
				ServiceNetwork:   "1172.30.0.0/16",
			},
			expectedErrors: 1,
		},
		{
			name: "Service network overlaps with cluster network",
			cn: &api.ClusterNetwork{
				ObjectMeta:       kapi.ObjectMeta{Name: "any"},
				Network:          "10.20.0.0/16",
				HostSubnetLength: 8,
				ServiceNetwork:   "10.20.1.0/24",
			},
			expectedErrors: 1,
		},
		{
			name: "Cluster network overlaps with service network",
			cn: &api.ClusterNetwork{
				ObjectMeta:       kapi.ObjectMeta{Name: "any"},
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

func TestValidateHostSubnet(t *testing.T) {
	tests := []struct {
		name           string
		hs             *api.HostSubnet
		expectedErrors int
	}{
		{
			name: "Good one",
			hs: &api.HostSubnet{
				ObjectMeta: kapi.ObjectMeta{
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
			hs: &api.HostSubnet{
				ObjectMeta: kapi.ObjectMeta{
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
			hs: &api.HostSubnet{
				ObjectMeta: kapi.ObjectMeta{
					Name: "abc.def.com",
				},
				Host:   "abc.def.com",
				HostIP: "10.20.30.40",
				Subnet: "8.8.0/24",
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
		fw             *api.EgressNetworkPolicy
		expectedErrors int
	}{
		{
			name: "Empty",
			fw: &api.EgressNetworkPolicy{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "default",
					Namespace: "testing",
				},
				Spec: api.EgressNetworkPolicySpec{
					Egress: []api.EgressNetworkPolicyRule{},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Good one",
			fw: &api.EgressNetworkPolicy{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "default",
					Namespace: "testing",
				},
				Spec: api.EgressNetworkPolicySpec{
					Egress: []api.EgressNetworkPolicyRule{
						{
							Type: api.EgressNetworkPolicyRuleAllow,
							To: api.EgressNetworkPolicyPeer{
								CIDRSelector: "1.2.3.0/24",
							},
						},
						{
							Type: api.EgressNetworkPolicyRuleDeny,
							To: api.EgressNetworkPolicyPeer{
								CIDRSelector: "1.2.3.4/32",
							},
						},
					},
				},
			},
			expectedErrors: 0,
		},
		{
			name: "Bad policy",
			fw: &api.EgressNetworkPolicy{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "default",
					Namespace: "testing",
				},
				Spec: api.EgressNetworkPolicySpec{
					Egress: []api.EgressNetworkPolicyRule{
						{
							Type: api.EgressNetworkPolicyRuleType("Bob"),
							To: api.EgressNetworkPolicyPeer{
								CIDRSelector: "1.2.3.0/24",
							},
						},
						{
							Type: api.EgressNetworkPolicyRuleDeny,
							To: api.EgressNetworkPolicyPeer{
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
			fw: &api.EgressNetworkPolicy{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "default",
					Namespace: "testing",
				},
				Spec: api.EgressNetworkPolicySpec{
					Egress: []api.EgressNetworkPolicyRule{
						{
							Type: api.EgressNetworkPolicyRuleAllow,
							To: api.EgressNetworkPolicyPeer{
								CIDRSelector: "1.2.3.4",
							},
						},
						{
							Type: api.EgressNetworkPolicyRuleDeny,
							To: api.EgressNetworkPolicyPeer{
								CIDRSelector: "",
							},
						},
					},
				},
			},
			expectedErrors: 2,
		},
	}

	for _, tc := range tests {
		errs := ValidateEgressNetworkPolicy(tc.fw)

		if len(errs) != tc.expectedErrors {
			t.Errorf("Test case %s expected %d error(s), got %d. %v", tc.name, tc.expectedErrors, len(errs), errs)
		}
	}
}

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

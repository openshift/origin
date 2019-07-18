package common

import (
	"net"
	"strings"
	"testing"

	networkapi "github.com/openshift/api/network/v1"
	kapi "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
)

func mustParseCIDR(cidr string) *net.IPNet {
	_, net, err := net.ParseCIDR(cidr)
	if err != nil {
		panic("bad CIDR string constant " + cidr)
	}
	return net
}

func TestGenerateGateway(t *testing.T) {
	_, ipNet, err := net.ParseCIDR("10.1.0.0/24")
	if err != nil {
		t.Fatal(err)
	}
	gatewayIP := GenerateDefaultGateway(ipNet)
	if gatewayIP.String() != "10.1.0.1" {
		t.Fatalf("Did not get expected gateway IP Address (gatewayIP=%s)", gatewayIP.String())
	}
}

func TestCheckHostNetworks(t *testing.T) {
	hostIPNets := []*net.IPNet{
		mustParseCIDR("10.0.0.0/9"),
		mustParseCIDR("172.20.0.0/16"),
	}

	tests := []struct {
		name        string
		networkInfo *ParsedClusterNetwork
		expectError bool
	}{
		{
			name: "valid",
			networkInfo: &ParsedClusterNetwork{
				ClusterNetworks: []ParsedClusterNetworkEntry{
					{ClusterCIDR: mustParseCIDR("10.128.0.0/14"), HostSubnetLength: 8},
				},
				ServiceNetwork: mustParseCIDR("172.30.0.0/16"),
			},
			expectError: false,
		},
		{
			name: "valid multiple networks",
			networkInfo: &ParsedClusterNetwork{
				ClusterNetworks: []ParsedClusterNetworkEntry{
					{ClusterCIDR: mustParseCIDR("10.128.0.0/14"), HostSubnetLength: 8},
					{ClusterCIDR: mustParseCIDR("15.128.0.0/14"), HostSubnetLength: 8},
				},
				ServiceNetwork: mustParseCIDR("172.30.0.0/16"),
			},
			expectError: false,
		},
		{
			name: "hostIPNet inside ClusterNetwork",
			networkInfo: &ParsedClusterNetwork{
				ClusterNetworks: []ParsedClusterNetworkEntry{
					{ClusterCIDR: mustParseCIDR("10.0.0.0/8"), HostSubnetLength: 8},
				},
				ServiceNetwork: mustParseCIDR("172.30.0.0/16"),
			},
			expectError: true,
		},
		{
			name: "ClusterNetwork inside hostIPNet",
			networkInfo: &ParsedClusterNetwork{
				ClusterNetworks: []ParsedClusterNetworkEntry{
					{ClusterCIDR: mustParseCIDR("10.1.0.0/16"), HostSubnetLength: 8},
				},
				ServiceNetwork: mustParseCIDR("172.30.0.0/16"),
			},
			expectError: true,
		},
		{
			name: "hostIPNet inside ServiceNetwork",
			networkInfo: &ParsedClusterNetwork{
				ClusterNetworks: []ParsedClusterNetworkEntry{
					{ClusterCIDR: mustParseCIDR("10.128.0.0/14"), HostSubnetLength: 8},
				},
				ServiceNetwork: mustParseCIDR("172.0.0.0/8"),
			},
			expectError: true,
		},
		{
			name: "ServiceNetwork inside hostIPNet",
			networkInfo: &ParsedClusterNetwork{
				ClusterNetworks: []ParsedClusterNetworkEntry{
					{ClusterCIDR: mustParseCIDR("10.128.0.0/14"), HostSubnetLength: 8},
				},
				ServiceNetwork: mustParseCIDR("172.20.30.0/8"),
			},
			expectError: true,
		},
	}

	for _, test := range tests {
		err := test.networkInfo.CheckHostNetworks(hostIPNets)
		if test.expectError {
			if err == nil {
				t.Fatalf("unexpected lack of error checking %q", test.name)
			}
		} else {
			if err != nil {
				t.Fatalf("unexpected error checking %q: %v", test.name, err)
			}
		}
	}
}

func dummySubnet(hostip string, subnet string) networkapi.HostSubnet {
	return networkapi.HostSubnet{HostIP: hostip, Subnet: subnet}
}

func dummyService(ip string) kapi.Service {
	return kapi.Service{Spec: kapi.ServiceSpec{ClusterIP: ip}}
}

func dummyPod(ip string) kapi.Pod {
	return kapi.Pod{Status: kapi.PodStatus{PodIP: ip}}
}

func Test_checkClusterObjects(t *testing.T) {
	subnets := []networkapi.HostSubnet{
		dummySubnet("192.168.1.2", "10.128.0.0/23"),
		dummySubnet("192.168.1.3", "10.129.0.0/23"),
		dummySubnet("192.168.1.4", "10.130.0.0/23"),
	}
	pods := []kapi.Pod{
		dummyPod("10.128.0.2"),
		dummyPod("10.128.0.4"),
		dummyPod("10.128.0.6"),
		dummyPod("10.128.0.8"),
		dummyPod("10.129.0.3"),
		dummyPod("10.129.0.5"),
		dummyPod("10.129.0.7"),
		dummyPod("10.129.0.9"),
		dummyPod("10.130.0.10"),
	}
	services := []kapi.Service{
		dummyService("172.30.0.1"),
		dummyService("172.30.0.128"),
		dummyService("172.30.99.99"),
		dummyService("None"),
	}

	tests := []struct {
		name string
		ni   *ParsedClusterNetwork
		errs []string
	}{
		{
			name: "valid",
			ni: &ParsedClusterNetwork{
				ClusterNetworks: []ParsedClusterNetworkEntry{
					{ClusterCIDR: mustParseCIDR("10.128.0.0/14"), HostSubnetLength: 8},
				},
				ServiceNetwork: mustParseCIDR("172.30.0.0/16"),
			},
			errs: []string{},
		},
		{
			name: "Subnet 10.130.0.0/23 and Pod 10.130.0.10 outside of ClusterNetwork",
			ni: &ParsedClusterNetwork{
				ClusterNetworks: []ParsedClusterNetworkEntry{
					{ClusterCIDR: mustParseCIDR("10.128.0.0/15"), HostSubnetLength: 8},
				},
				ServiceNetwork: mustParseCIDR("172.30.0.0/16"),
			},
			errs: []string{"10.130.0.0/23", "10.130.0.10"},
		},
		{
			name: "Service 172.30.99.99 outside of ServiceNetwork",
			ni: &ParsedClusterNetwork{
				ClusterNetworks: []ParsedClusterNetworkEntry{
					{ClusterCIDR: mustParseCIDR("10.128.0.0/14"), HostSubnetLength: 8},
				},
				ServiceNetwork: mustParseCIDR("172.30.0.0/24"),
			},
			errs: []string{"172.30.99.99"},
		},
		{
			name: "Too-many-error truncation",
			ni: &ParsedClusterNetwork{
				ClusterNetworks: []ParsedClusterNetworkEntry{
					{ClusterCIDR: mustParseCIDR("1.2.3.0/24"), HostSubnetLength: 8},
				},
				ServiceNetwork: mustParseCIDR("4.5.6.0/24"),
			},
			errs: []string{"10.128.0.0/23", "10.129.0.0/23", "10.130.0.0/23", "10.128.0.2", "10.128.0.4", "10.128.0.6", "10.128.0.8", "10.129.0.3", "10.129.0.5", "10.129.0.7", "172.30.0.1", "too many errors"},
		},
	}

	for _, test := range tests {
		err := test.ni.CheckClusterObjects(subnets, pods, services)
		if err == nil {
			if len(test.errs) > 0 {
				t.Fatalf("test %q unexpectedly did not get an error", test.name)
			}
			continue
		}
		errs := err.(kerrors.Aggregate).Errors()
		if len(errs) != len(test.errs) {
			t.Fatalf("test %q expected %d errors, got %v", test.name, len(test.errs), err)
		}
		for i, match := range test.errs {
			if !strings.Contains(errs[i].Error(), match) {
				t.Fatalf("test %q: error %d did not match %q: %v", test.name, i, match, errs[i])
			}
		}
	}
}

func TestParseClusterNetwork(t *testing.T) {
	tests := []struct {
		name string
		cn   networkapi.ClusterNetwork
		err  string
	}{
		{
			name: "valid single cidr",
			cn: networkapi.ClusterNetwork{
				ClusterNetworks: []networkapi.ClusterNetworkEntry{{CIDR: "10.0.0.0/16"}},
				ServiceNetwork:  "172.30.0.0/16",
			},
			err: "",
		},
		{
			name: "valid multiple cidr",
			cn: networkapi.ClusterNetwork{
				ClusterNetworks: []networkapi.ClusterNetworkEntry{{CIDR: "10.0.0.0/16"}, {CIDR: "10.4.0.0/16"}},
				ServiceNetwork:  "172.30.0.0/16",
			},
			err: "",
		},
		{
			name: "invalid CIDR address",
			cn: networkapi.ClusterNetwork{
				ClusterNetworks: []networkapi.ClusterNetworkEntry{{CIDR: "Invalid"}},
				ServiceNetwork:  "172.30.0.0/16",
			},
			err: "Invalid",
		},
		{
			name: "invalid serviceNetwork",
			cn: networkapi.ClusterNetwork{
				ClusterNetworks: []networkapi.ClusterNetworkEntry{{CIDR: "10.0.0.0/16"}},
				ServiceNetwork:  "172.30.0.0i/16",
			},
			err: "172.30.0.0i/16",
		},
	}
	for _, test := range tests {
		_, err := ParseClusterNetwork(&test.cn)
		if err == nil {
			if len(test.err) > 0 {
				t.Fatalf("test %q unexpectedly did not get an error", test.name)
			}
		} else {
			if !strings.Contains(err.Error(), test.err) {
				t.Fatalf("test %q: error did not match %q: %v", test.name, test.err, err)
			}
		}
	}
}

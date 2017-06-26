package plugin

import (
	"net"
	"strings"
	"testing"

	osapi "github.com/openshift/origin/pkg/sdn/apis/network"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kapi "k8s.io/kubernetes/pkg/api"
)

func mustParseCIDR(cidr string) *net.IPNet {
	_, net, err := net.ParseCIDR(cidr)
	if err != nil {
		panic("bad CIDR string constant " + cidr)
	}
	return net
}

func Test_checkHostNetworks(t *testing.T) {
	hostIPNets := []*net.IPNet{
		mustParseCIDR("10.0.0.0/9"),
		mustParseCIDR("172.20.0.0/16"),
	}

	tests := []struct {
		name        string
		networkInfo *NetworkInfo
		expectError bool
	}{
		{
			name: "valid",
			networkInfo: &NetworkInfo{
				ClusterNetwork: mustParseCIDR("10.128.0.0/14"),
				ServiceNetwork: mustParseCIDR("172.30.0.0/16"),
			},
			expectError: false,
		},
		{
			name: "hostIPNet inside ClusterNetwork",
			networkInfo: &NetworkInfo{
				ClusterNetwork: mustParseCIDR("10.0.0.0/8"),
				ServiceNetwork: mustParseCIDR("172.30.0.0/16"),
			},
			expectError: true,
		},
		{
			name: "ClusterNetwork inside hostIPNet",
			networkInfo: &NetworkInfo{
				ClusterNetwork: mustParseCIDR("10.1.0.0/16"),
				ServiceNetwork: mustParseCIDR("172.30.0.0/16"),
			},
			expectError: true,
		},
		{
			name: "hostIPNet inside ServiceNetwork",
			networkInfo: &NetworkInfo{
				ClusterNetwork: mustParseCIDR("10.128.0.0/14"),
				ServiceNetwork: mustParseCIDR("172.0.0.0/8"),
			},
			expectError: true,
		},
		{
			name: "ServiceNetwork inside hostIPNet",
			networkInfo: &NetworkInfo{
				ClusterNetwork: mustParseCIDR("10.128.0.0/14"),
				ServiceNetwork: mustParseCIDR("172.20.30.0/8"),
			},
			expectError: true,
		},
	}

	for _, test := range tests {
		err := test.networkInfo.checkHostNetworks(hostIPNets)
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

func dummySubnet(hostip string, subnet string) osapi.HostSubnet {
	return osapi.HostSubnet{HostIP: hostip, Subnet: subnet}
}

func dummyService(ip string) kapi.Service {
	return kapi.Service{Spec: kapi.ServiceSpec{ClusterIP: ip}}
}

func dummyPod(ip string) kapi.Pod {
	return kapi.Pod{Status: kapi.PodStatus{PodIP: ip}}
}

func Test_checkClusterObjects(t *testing.T) {
	subnets := []osapi.HostSubnet{
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
		ni   *NetworkInfo
		errs []string
	}{
		{
			name: "valid",
			ni: &NetworkInfo{
				ClusterNetwork: mustParseCIDR("10.128.0.0/14"),
				ServiceNetwork: mustParseCIDR("172.30.0.0/16"),
			},
			errs: []string{},
		},
		{
			name: "Subnet 10.130.0.0/23 and Pod 10.130.0.10 outside of ClusterNetwork",
			ni: &NetworkInfo{
				ClusterNetwork: mustParseCIDR("10.128.0.0/15"),
				ServiceNetwork: mustParseCIDR("172.30.0.0/16"),
			},
			errs: []string{"10.130.0.0/23", "10.130.0.10"},
		},
		{
			name: "Service 172.30.99.99 outside of ServiceNetwork",
			ni: &NetworkInfo{
				ClusterNetwork: mustParseCIDR("10.128.0.0/14"),
				ServiceNetwork: mustParseCIDR("172.30.0.0/24"),
			},
			errs: []string{"172.30.99.99"},
		},
		{
			name: "Too-many-error truncation",
			ni: &NetworkInfo{
				ClusterNetwork: mustParseCIDR("1.2.3.0/24"),
				ServiceNetwork: mustParseCIDR("4.5.6.0/24"),
			},
			errs: []string{"10.128.0.0/23", "10.129.0.0/23", "10.130.0.0/23", "10.128.0.2", "10.128.0.4", "10.128.0.6", "10.128.0.8", "10.129.0.3", "10.129.0.5", "10.129.0.7", "172.30.0.1", "too many errors"},
		},
	}

	for _, test := range tests {
		err := test.ni.checkClusterObjects(subnets, pods, services)
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

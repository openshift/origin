package netutils

import (
	"net"
	"strings"
	"testing"
)

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

func TestParseCIDRMask(t *testing.T) {
	tests := []struct {
		cidr       string
		fixedShort string
		fixedLong  string
	}{
		{
			cidr: "192.168.0.0/16",
		},
		{
			cidr: "192.168.1.0/24",
		},
		{
			cidr: "192.168.1.1/32",
		},
		{
			cidr:       "192.168.1.0/16",
			fixedShort: "192.168.0.0/16",
			fixedLong:  "192.168.1.0/32",
		},
		{
			cidr:       "192.168.1.1/24",
			fixedShort: "192.168.1.0/24",
			fixedLong:  "192.168.1.1/32",
		},
	}

	for _, test := range tests {
		_, err := ParseCIDRMask(test.cidr)
		if test.fixedShort == "" && test.fixedLong == "" {
			if err != nil {
				t.Fatalf("unexpected error parsing CIDR mask %q: %v", test.cidr, err)
			}
		} else {
			if err == nil {
				t.Fatalf("unexpected lack of error parsing CIDR mask %q", test.cidr)
			}
			if !strings.Contains(err.Error(), test.fixedShort) {
				t.Fatalf("error does not contain expected string %q: %v", test.fixedShort, err)
			}
			if !strings.Contains(err.Error(), test.fixedLong) {
				t.Fatalf("error does not contain expected string %q: %v", test.fixedLong, err)
			}
		}
	}
}

func TestIsPrivateAddress(t *testing.T) {
	for _, tc := range []struct {
		address string
		isLocal bool
	}{
		{"localhost", true},
		{"example.com", false},
		{"registry.localhost", false},

		{"9.255.255.255", false},
		{"10.0.0.1", true},
		{"10.1.255.255", true},
		{"10.255.255.255", true},
		{"11.0.0.1", false},

		{"127.0.0.1", true},

		{"172.15.255.253", false},
		{"172.16.0.1", true},
		{"172.30.0.1", true},
		{"172.31.255.255", true},
		{"172.32.0.1", false},

		{"192.167.122.1", false},
		{"192.168.0.1", true},
		{"192.168.122.1", true},
		{"192.168.255.255", true},
		{"192.169.1.1", false},

		{"::1", true},

		{"fe00::1", false},
		{"fd12:3456:789a:1::1", true},
		{"fe82:3456:789a:1::1", true},
		{"ff00::1", false},
	} {
		res := IsPrivateAddress(tc.address)
		if tc.isLocal && !res {
			t.Errorf("address %q considered not local", tc.address)
			continue
		}
		if !tc.isLocal && res {
			t.Errorf("address %q considered local", tc.address)
		}
	}
}

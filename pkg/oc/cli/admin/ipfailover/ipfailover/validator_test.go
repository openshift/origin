package ipfailover

import (
	"testing"
)

func TestValidateIPAddress(t *testing.T) {
	validIPs := []string{"1.1.1.1", "1.1.1.255", "255.255.255.255",
		"8.8.8.8", "0.1.2.3", "255.254.253.252",
	}

	for _, ip := range validIPs {
		if err := ValidateIPAddress(ip); err != nil {
			t.Errorf("Test valid ip=%q got error %s expected: no error.", ip, err)
		}
	}

	invalidIPs := []string{"1.1.1.256", "256.256.256.256",
		"1024.512.256.128", "a.b.c.d", "1.2.3.4.abc", "5.6.7.8def",
		"a.12.13.14", "9999.888.77.6", "1.2.3",
	}

	for _, ip := range invalidIPs {
		if err := ValidateIPAddress(ip); err == nil {
			t.Errorf("Test invalid ip=%q got no error expected: error.", ip)
		}
	}
}

func TestValidateIPAddressRange(t *testing.T) {
	validRanges := []string{"1.1.1.1-1", "1.1.1.1-7", "1.1.1.250-255",
		"255.255.255.255-255", "8.8.8.4-8", "0.1.2.3-255",
		"255.254.253.252-255", "1.1.1.1", "   1.1.1.1-2   ",
	}

	for _, iprange := range validRanges {
		if err := ValidateIPAddressRange(iprange); err != nil {
			t.Errorf("Test valid iprange=%q got error %s expected: no error.", iprange, err)
		}
	}

	invalidRanges := []string{"1.1.1.256-250", "1.1.1.1-0",
		"1.1.1.5-1", "255.255.255.255-259", "1024.512.256.128-255",
		"a.b.c.d-e", "1.2.3.4.abc-def", "5.6.7.8def-1.2.3.4abc",
		"a.12.13.14-55", "9999.888.77.6-66", "1.2.3.4-5-6", "1.2.3-4",
		"1,2.3.4.5-6", "-", "1.1.1.    1-2",
	}

	for _, iprange := range invalidRanges {
		if err := ValidateIPAddressRange(iprange); err == nil {
			t.Errorf("Test invalid iprange=%q got no error expected: error.", iprange)
		}
	}
}

func TestValidateVirtualIPs(t *testing.T) {
	validVIPs := []string{"", "1.1.1.1-1,2.2.2.2", "4.4.4.4-8",
		"1.1.1.1-7,2.2.2.2,3.3.3.3-5",
		"1.1.1.250-255,255.255.255.255-255", "4.4.4.4-8,8.8.8.4-8",
		"0.1.2.3-255,4.5.6.7,8.9.10.11,12.13.14.15-20",
		"255.254.253.252-255,1.1.1.1",
	}

	for _, vips := range validVIPs {
		if err := ValidateVirtualIPs(vips); err != nil {
			t.Errorf("Test valid vips=%q got error %s expected: no error.",
				vips, err)
		}
	}

	invalidVIPs := []string{"1.1.1.256-250,2.2.2.2", "1.1.1.1,2.2.2.2-0",
		"1.1.1.1-5,2.2.2.2,3.3.3.3-1", "255.255.255.255-259",
		"1.2.3.4-5,1024.512.256.128-255", "1.1.1.1,a.b.c.d-e",
		"a.b.c.d-e,5.4.3.2", "1.2.3.4.abc-def",
		"5.6.7.8def-1.2.3.4abc", "4.1.1.1,a.12.13.14-55",
		"8.8.8.8,9999.888.77.6-66,4.4.4.4-8", "1.2.3.4-5-6", "1.2.3-4",
	}

	for _, vips := range invalidVIPs {
		if err := ValidateVirtualIPs(vips); err == nil {
			t.Errorf("Test invalid vips=%q got no error expected: error.", vips)
		}
	}
}

func TestValidateCmdOptionsVIPs(t *testing.T) {
	validVIPs := []string{"", "1.1.1.1-1,2.2.2.2", "4.4.4.4-8",
		"1.1.1.1-7,2.2.2.2,3.3.3.3-5",
		"1.1.1.250-255,255.255.255.255-255", "4.4.4.4-8,8.8.8.4-8",
		"0.1.2.3-255,4.5.6.7,8.9.10.11,12.13.14.15-20",
		"255.254.253.252-255,1.1.1.1",
	}

	for _, vips := range validVIPs {
		options := &IPFailoverConfigCmdOptions{VirtualIPs: vips}
		if err := ValidateCmdOptions(options); err != nil {
			t.Errorf("Test command options valid vips=%q got error %s expected: no error.",
				vips, err)
		}
	}

	invalidVIPs := []string{"1.1.1.256-250,2.2.2.2", "1.1.1.1,2.2.2.2-0",
		"1.1.1.1-5,2.2.2.2,3.3.3.3-1", "255.255.255.255-259",
		"1.2.3.4-5,1024.512.256.128-255", "1.1.1.1,a.b.c.d-e",
		"a.b.c.d-e,5.4.3.2", "1.2.3.4.abc-def",
		"5.6.7.8def-1.2.3.4abc", "4.1.1.1,a.12.13.14-55",
		"8.8.8.8,9999.888.77.6-66,4.4.4.4-8", "1.2.3.4-5-6", "1.2.3-4",
	}

	for _, vips := range invalidVIPs {
		options := &IPFailoverConfigCmdOptions{VirtualIPs: vips}
		if err := ValidateCmdOptions(options); err == nil {
			t.Errorf("Test command options invalid vips=%q got no error expected: error.", vips)
		}
	}
}

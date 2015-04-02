package router

import (
	"testing"
)

func TestValidateIPAddress(t *testing.T) {
	validIPs := []string{"1.1.1.1", "1.1.1.255", "255.255.255.255",
		"8.8.8.8", "0.1.2.3", "255.254.253.252",
	}
	invalidIPs := []string{"1.1.1.256", "256.256.256.256",
		"1024.512.256.128", "a.b.c.d", "1.2.3.4.abc", "5.6.7.8def",
		"a.12.13.14", "9999.888.77.6",
	}

	for _, ip := range validIPs {
		if err := ValidateIPAddress(ip); err != nil {
			t.Errorf("Test case %s got error %s expected: no error.", ip, err)
		}
	}

	for _, ip := range invalidIPs {
		if err := ValidateIPAddress(ip); err == nil {
			t.Errorf("Test case %s got no error expected: error.", ip)
		}
	}
}

func TestValidateIPAddressRange(t *testing.T) {
	validRanges := []string{"1.1.1.1-1", "1.1.1.1-7", "1.1.1.250-255",
		"255.255.255.255-255", "8.8.8.4-8", "0.1.2.3-255",
		"255.254.253.252-255",
	}
	invalidRanges := []string{"1.1.1.256-250", "1.1.1.1-0",
		"1.1.1.5-1", "255.255.255.255-259", "1024.512.256.128-255",
		"a.b.c.d-e", "1.2.3.4.abc-def", "5.6.7.8def-1.2.3.4abc",
		"a.12.13.14-55", "9999.888.77.6-66",
	}

	for _, iprange := range validRanges {
		if err := ValidateIPAddressRange(iprange); err != nil {
			t.Errorf("Test case %s got error %s expected: no error.", iprange, err)
		}
	}

	for _, iprange := range invalidRanges {
		if err := ValidateIPAddressRange(iprange); err == nil {
			t.Errorf("Test case %s got no error expected: error.", iprange)
		}
	}
}

package router

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

//  Validate IP address.
func ValidateIPAddress(ip string) error {
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("Invalid IP address: %s", ip)
	}

	return nil
}

//  Validate range of IPs.
func ValidateIPAddressRange(iprange string) error {
	if strings.Count(iprange, "-") < 1 {
		return ValidateIPAddress(iprange)
	}

	// Its an IP range of the form: n.n.n.n-n
	rangeLimits := strings.Split(iprange, "-")
	startIP := rangeLimits[0]
	parts := strings.Split(startIP, ".")
	rangeStart := parts[3]
	rangeEnd := rangeLimits[1]
	if err := ValidateIPAddress(startIP); err != nil {
		return err
	}

	//  Manufacture ending IP address for the range.
	parts[3] = rangeEnd
	endIP := strings.Join(parts, ".")
	if ValidateIPAddress(endIP) != nil {
		return fmt.Errorf("Invalid IP range end: %s [%s]",
			rangeEnd, endIP)
	}

	// Lastly, ensure start <= end
	start, err := strconv.Atoi(rangeStart)
	if err != nil {
		return fmt.Errorf("Invalid IP range start: %s [%s]",
			rangeStart, startIP)
	}

	end, err := strconv.Atoi(rangeEnd)
	if err != nil {
		return fmt.Errorf("Invalid IP range end: %s [%s]",
			rangeEnd, endIP)
	}

	if start > end {
		return fmt.Errorf("Invalid IP range %s-%s: start=%v > end=%v",
			startIP, endIP, start, end)
	}

	return nil
}

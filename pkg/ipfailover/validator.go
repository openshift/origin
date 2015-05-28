package ipfailover

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

//  Validate IP address.
func ValidateIPAddress(ip string) error {
	ipaddr := strings.TrimSpace(ip)
	if net.ParseIP(ipaddr) == nil {
		return fmt.Errorf("Invalid IP address: %s", ip)
	}

	return nil
}

// Validate an IP address range or single IP address.
func ValidateIPAddressRange(iprange string) error {
	iprange = strings.TrimSpace(iprange)
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
		return fmt.Errorf("invalid IP range end: %s [%s]", rangeEnd, endIP)
	}

	// Lastly, ensure start <= end
	start, err := strconv.Atoi(rangeStart)
	if err != nil {
		return fmt.Errorf("invalid IP range start: %s [%s]", rangeStart, startIP)
	}

	end, err := strconv.Atoi(rangeEnd)
	if err != nil {
		return fmt.Errorf("invalid IP range end: %s [%s]", rangeEnd, endIP)
	}

	if start > end {
		return fmt.Errorf("invalid IP range %s-%s: start=%v > end=%v", startIP, endIP, start, end)
	}

	return nil
}

//  Validate virtual IP range/addresses.
func ValidateVirtualIPs(vips string) error {
	virtualIPs := strings.TrimSpace(vips)
	if len(virtualIPs) < 1 {
		return nil
	}

	for _, ip := range strings.Split(virtualIPs, ",") {
		if err := ValidateIPAddressRange(ip); err != nil {
			return err
		}
	}

	return nil
}

// Validate command line operations.
func ValidateCmdOptions(options *IPFailoverConfigCmdOptions, c *Configurator) error {
	dc, err := c.Plugin.GetDeploymentConfig()
	if err != nil {
		return err
	}

	//  If creating deployment, check deployment config doesn't exist.
	if options.Create && dc != nil {
		return fmt.Errorf("IP Failover config %q exists\n", c.Name)
	}

	return ValidateVirtualIPs(options.VirtualIPs)
}

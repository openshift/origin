package ipfailover

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	securityv1typedclient "github.com/openshift/client-go/security/clientset/versioned/typed/security/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

// ValidateIPAddress validates IP address.
func validateIPAddress(ip string) error {
	ipaddr := strings.TrimSpace(ip)
	if net.ParseIP(ipaddr) == nil {
		return fmt.Errorf("Invalid IP address: %s", ip)
	}

	return nil
}

// validateIPAddressRange validates an IP address range or single IP address.
func validateIPAddressRange(iprange string) error {
	iprange = strings.TrimSpace(iprange)
	if strings.Count(iprange, "-") < 1 {
		return validateIPAddress(iprange)
	}
	if strings.Count(iprange, "-") > 1 {
		return fmt.Errorf("invalid IP range format: %s", iprange)
	}

	// It's an IP range of the form: n.n.n.n-n
	rangeLimits := strings.Split(iprange, "-")
	startIP := rangeLimits[0]
	parts := strings.Split(startIP, ".")
	if len(parts) != 4 {
		return fmt.Errorf("invalid IP range start format: %s", startIP)
	}
	rangeStart := parts[3]
	rangeEnd := rangeLimits[1]
	if err := validateIPAddress(startIP); err != nil {
		return err
	}

	//  Manufacture ending IP address for the range.
	parts[3] = rangeEnd
	endIP := strings.Join(parts, ".")
	if validateIPAddress(endIP) != nil {
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

// validateVirtualIPs validates virtual IP range/addresses.
func validateVirtualIPs(vips string) error {
	virtualIPs := strings.TrimSpace(vips)
	if len(virtualIPs) < 1 {
		return nil
	}

	for _, ip := range strings.Split(virtualIPs, ",") {
		if err := validateIPAddressRange(ip); err != nil {
			return err
		}
	}

	return nil
}

func validateServiceAccount(client securityv1typedclient.SecurityV1Interface, serviceAccount string) error {
	sccList, err := client.SecurityContextConstraints().List(metav1.ListOptions{})
	if err != nil && !errors.IsUnauthorized(err) {
		return fmt.Errorf("could not retrieve list of security constraints to verify service account %q: %v", serviceAccount, err)
	}

	for _, scc := range sccList.Items {
		if scc.AllowPrivilegedContainer {
			for _, user := range scc.Users {
				if strings.Contains(user, serviceAccount) {
					return nil
				}
			}
		}
	}
	errMsg := "service account %q does not have sufficient privileges, grant access with: oc adm policy add-scc-to-user %s -z %s"
	return fmt.Errorf(errMsg, serviceAccount, bootstrappolicy.SecurityContextConstraintPrivileged, serviceAccount)
}

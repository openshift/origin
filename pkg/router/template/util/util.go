package util

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/golang/glog"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/origin/pkg/route/controller/routeapihelpers"
)

// GenerateRouteRegexp generates a regular expression to match route hosts (and paths if any).
func GenerateRouteRegexp(hostname, path string, wildcard bool) string {
	hostRE := regexp.QuoteMeta(hostname)
	if wildcard {
		subdomain := routeapihelpers.GetDomainForHost(hostname)
		if len(subdomain) == 0 {
			glog.Warningf("Generating subdomain wildcard regexp - invalid host name %s", hostname)
		} else {
			subdomainRE := regexp.QuoteMeta(fmt.Sprintf(".%s", subdomain))
			hostRE = fmt.Sprintf(`[^\.]*%s`, subdomainRE)
		}
	}

	portRE := "(:[0-9]+)?"

	// build the correct subpath regex, depending on whether path ends with a segment separator
	var pathRE, subpathRE string
	switch {
	case len(strings.TrimRight(path, "/")) == 0:
		// Special-case paths consisting solely of "/" to match a root request to "" as well
		pathRE = ""
		subpathRE = "(/.*)?"
	case strings.HasSuffix(path, "/"):
		pathRE = regexp.QuoteMeta(path)
		subpathRE = "(.*)?"
	default:
		pathRE = regexp.QuoteMeta(path)
		subpathRE = "(/.*)?"
	}

	return "^" + hostRE + portRE + pathRE + subpathRE + "$"
}

// GenCertificateHostName generates the host name to use for serving/certificate matching.
// If wildcard is set, a wildcard host name (*.<subdomain>) is generated.
func GenCertificateHostName(hostname string, wildcard bool) string {
	if wildcard {
		if idx := strings.IndexRune(hostname, '.'); idx > 0 {
			return fmt.Sprintf("*.%s", hostname[idx+1:])
		}
	}

	return hostname
}

// GenerateBackendNamePrefix generates the backend name prefix based on the termination.
func GenerateBackendNamePrefix(termination routev1.TLSTerminationType) string {
	prefix := "be_http"
	switch termination {
	case routev1.TLSTerminationEdge:
		prefix = "be_edge_http"
	case routev1.TLSTerminationReencrypt:
		prefix = "be_secure"
	case routev1.TLSTerminationPassthrough:
		prefix = "be_tcp"
	}

	return prefix
}

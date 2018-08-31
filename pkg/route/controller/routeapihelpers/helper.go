package routeapihelpers

import (
	"strings"

	routev1 "github.com/openshift/api/route/v1"
)

func RouteLessThan(route1, route2 *routev1.Route) bool {
	if route1.CreationTimestamp.Before(&route2.CreationTimestamp) {
		return true
	}

	if route2.CreationTimestamp.Before(&route1.CreationTimestamp) {
		return false
	}

	return route1.UID < route2.UID
}

// GetDomainForHost returns the domain for the specified host.
// Note for top level domains, this will return an empty string.
func GetDomainForHost(host string) string {
	if idx := strings.IndexRune(host, '.'); idx > -1 {
		return host[idx+1:]
	}

	return ""
}

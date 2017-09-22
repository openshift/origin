package templaterouter

import (
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/golang/glog"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
)

func isTrue(s string) bool {
	v, _ := strconv.ParseBool(s)
	return v
}

func firstMatch(pattern string, values ...string) string {
	glog.V(7).Infof("firstMatch called with %s and %v", pattern, values)
	if re, err := regexp.Compile(`\A(?:` + pattern + `)\z`); err == nil {
		for _, value := range values {
			if re.MatchString(value) {
				glog.V(7).Infof("firstMatch returning string: %s", value)
				return value
			}
		}
		glog.V(7).Infof("firstMatch returning empty string")
	} else {
		glog.Errorf("Error with regex pattern in call to firstMatch: %v", err)
	}
	return ""
}

func env(name string, defaults ...string) string {
	if envValue := os.Getenv(name); envValue != "" {
		return envValue
	}

	for _, val := range defaults {
		if val != "" {
			return val
		}
	}

	return ""
}

func isInteger(s string) bool {
	_, err := strconv.Atoi(s)
	return (err == nil)
}

func matchValues(s string, allowedValues ...string) bool {
	glog.V(7).Infof("matchValues called with %s and %v", s, allowedValues)
	for _, value := range allowedValues {
		if value == s {
			glog.V(7).Infof("matchValues finds matching string: %s", s)
			return true
		}
	}
	glog.V(7).Infof("matchValues cannot match string: %s", s)
	return false
}

func matchPattern(pattern, s string) bool {
	glog.V(7).Infof("matchPattern called with %s and %s", pattern, s)
	status, err := regexp.MatchString(`\A(?:`+pattern+`)\z`, s)
	if err == nil {
		glog.V(7).Infof("matchPattern returning status: %v", status)
		return status
	}
	glog.Errorf("Error with regex pattern in call to matchPattern: %v", err)
	return false
}

// genSubdomainWildcardRegexp is now legacy and around for backward
// compatibility and allows old templates to continue running.
// Generate a regular expression to match wildcard hosts (and paths if any)
// for a [sub]domain.
func genSubdomainWildcardRegexp(hostname, path string, exactPath bool) string {
	subdomain := routeapi.GetDomainForHost(hostname)
	if len(subdomain) == 0 {
		glog.Warningf("Generating subdomain wildcard regexp - invalid host name %s", hostname)
		return fmt.Sprintf("%s%s", hostname, path)
	}

	expr := regexp.QuoteMeta(fmt.Sprintf(".%s%s", subdomain, path))
	if exactPath {
		return fmt.Sprintf(`^[^\.]*%s$`, expr)
	}

	return fmt.Sprintf(`^[^\.]*%s(|/.*)$`, expr)
}

// Generate a regular expression to match route hosts (and paths if any).
func generateRouteRegexp(hostname, path string, wildcard bool) string {
	hostRE := regexp.QuoteMeta(hostname)
	if wildcard {
		subdomain := routeapi.GetDomainForHost(hostname)
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
	case strings.TrimRight(path, "/") == "":
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

// Generates the host name to use for serving/certificate matching.
// If wildcard is set, a wildcard host name (*.<subdomain>) is generated.
func genCertificateHostName(hostname string, wildcard bool) string {
	if wildcard {
		if idx := strings.IndexRune(hostname, '.'); idx > 0 {
			return fmt.Sprintf("*.%s", hostname[idx+1:])
		}
	}

	return hostname
}

// Returns the list of endpoints for the given route's service
// action argument further processes the list e.g. shuffle
// The default action is in-order traversal of internal data structure that stores
//   the endpoints (does not change the return order if the data structure did not mutate)
func processEndpointsForAlias(alias ServiceAliasConfig, svc ServiceUnit, action string) []Endpoint {
	endpoints := endpointsForAlias(alias, svc)
	if strings.ToLower(action) == "shuffle" {
		for i := len(endpoints) - 1; i >= 0; i-- {
			rIndex := rand.Intn(i + 1)
			endpoints[i], endpoints[rIndex] = endpoints[rIndex], endpoints[i]
		}
	}
	return endpoints
}

func endpointsForAlias(alias ServiceAliasConfig, svc ServiceUnit) []Endpoint {
	if len(alias.PreferPort) == 0 {
		return svc.EndpointTable
	}
	endpoints := make([]Endpoint, 0, len(svc.EndpointTable))
	for i := range svc.EndpointTable {
		endpoint := svc.EndpointTable[i]
		if endpoint.PortName == alias.PreferPort || endpoint.Port == alias.PreferPort {
			endpoints = append(endpoints, endpoint)
		}
	}
	return endpoints
}

var helperFunctions = template.FuncMap{
	"endpointsForAlias":        endpointsForAlias,        //returns the list of valid endpoints
	"processEndpointsForAlias": processEndpointsForAlias, //returns the list of valid endpoints after processing them
	"env":          env,          //tries to get an environment variable, returns the first non-empty default value or "" on failure
	"matchPattern": matchPattern, //anchors provided regular expression and evaluates against given string
	"isInteger":    isInteger,    //determines if a given variable is an integer
	"matchValues":  matchValues,  //compares a given string to a list of allowed strings

	"genSubdomainWildcardRegexp": genSubdomainWildcardRegexp, //generates a regular expression matching the subdomain for hosts (and paths) with a wildcard policy
	"generateRouteRegexp":        generateRouteRegexp,        //generates a regular expression matching the route hosts (and paths)
	"genCertificateHostName":     genCertificateHostName,     //generates host name to use for serving/matching certificates

	"isTrue":     isTrue,     //determines if a given variable is a true value
	"firstMatch": firstMatch, //anchors provided regular expression and evaluates against given strings, returns the first matched string or ""
}

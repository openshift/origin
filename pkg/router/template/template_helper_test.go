package templaterouter

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"regexp"
	"strings"
	"testing"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
)

func buildServiceAliasConfig(name, namespace, host, path string, termination routeapi.TLSTerminationType, policy routeapi.InsecureEdgeTerminationPolicyType, wildcard bool) ServiceAliasConfig {
	certs := make(map[string]Certificate)
	if termination != routeapi.TLSTerminationPassthrough {
		certs[host] = Certificate{
			ID:       fmt.Sprintf("id_%s", host),
			Contents: "abcdefghijklmnopqrstuvwxyz",
		}
	}

	return ServiceAliasConfig{
		Name:         name,
		Namespace:    namespace,
		Host:         host,
		Path:         path,
		IsWildcard:   wildcard,
		Certificates: certs,

		TLSTermination:                termination,
		InsecureEdgeTerminationPolicy: policy,
	}
}

func buildTestTemplateState() map[string]ServiceAliasConfig {
	state := make(map[string]ServiceAliasConfig)

	state["stg:api-route"] = buildServiceAliasConfig("api-route", "stg", "api-stg.127.0.0.1.nip.io", "", routeapi.TLSTerminationEdge, routeapi.InsecureEdgeTerminationPolicyRedirect, false)
	state["prod:api-route"] = buildServiceAliasConfig("api-route", "prod", "api-prod.127.0.0.1.nip.io", "", routeapi.TLSTerminationEdge, routeapi.InsecureEdgeTerminationPolicyRedirect, false)
	state["test:api-route"] = buildServiceAliasConfig("api-route", "test", "zzz-production.wildcard.test", "", routeapi.TLSTerminationEdge, routeapi.InsecureEdgeTerminationPolicyRedirect, false)
	state["dev:api-route"] = buildServiceAliasConfig("api-route", "dev", "3dev.127.0.0.1.nip.io", "", routeapi.TLSTerminationEdge, routeapi.InsecureEdgeTerminationPolicyAllow, false)
	state["prod:api-path-route"] = buildServiceAliasConfig("api-path-route", "prod", "api-prod.127.0.0.1.nip.io", "/x/y/z", routeapi.TLSTerminationEdge, routeapi.InsecureEdgeTerminationPolicyNone, false)

	state["prod:pt-route"] = buildServiceAliasConfig("pt-route", "prod", "passthrough-prod.127.0.0.1.nip.io", "", routeapi.TLSTerminationPassthrough, routeapi.InsecureEdgeTerminationPolicyNone, false)

	state["prod:wildcard-route"] = buildServiceAliasConfig("wildcard-route", "prod", "api-stg.127.0.0.1.nip.io", "", routeapi.TLSTerminationEdge, routeapi.InsecureEdgeTerminationPolicyNone, true)
	state["devel2:foo-wildcard-route"] = buildServiceAliasConfig("foo-wildcard-route", "devel2", "devel1.foo.127.0.0.1.nip.io", "", routeapi.TLSTerminationEdge, routeapi.InsecureEdgeTerminationPolicyAllow, true)
	state["devel2:foo-wildcard-test"] = buildServiceAliasConfig("foo-wildcard-test", "devel2", "something.foo.wildcard.test", "", routeapi.TLSTerminationEdge, routeapi.InsecureEdgeTerminationPolicyAllow, true)
	state["dev:pt-route"] = buildServiceAliasConfig("pt-route", "dev", "passthrough-dev.127.0.0.1.nip.io", "", routeapi.TLSTerminationPassthrough, routeapi.InsecureEdgeTerminationPolicyNone, false)
	state["dev:reencrypt-route"] = buildServiceAliasConfig("reencrypt-route", "dev", "reencrypt-dev.127.0.0.1.nip.io", "", routeapi.TLSTerminationReencrypt, routeapi.InsecureEdgeTerminationPolicyRedirect, false)

	state["dev:admin-route"] = buildServiceAliasConfig("admin-route", "dev", "3app-admin.127.0.0.1.nip.io", "", routeapi.TLSTerminationEdge, routeapi.InsecureEdgeTerminationPolicyNone, false)

	state["prod:backend-route"] = buildServiceAliasConfig("backend-route", "prod", "backend-app.127.0.0.1.nip.io", "", routeapi.TLSTerminationEdge, routeapi.InsecureEdgeTerminationPolicyRedirect, false)
	state["zzz:zed-route"] = buildServiceAliasConfig("zed-route", "zzz", "zed.127.0.0.1.nip.io", "", routeapi.TLSTerminationEdge, routeapi.InsecureEdgeTerminationPolicyAllow, false)

	return state
}

func checkExpectedOrderPrefixes(lines, expectedOrder []string) error {
	if len(lines) != len(expectedOrder) {
		return fmt.Errorf("sorted data length %d did not match expected length %d", len(lines), len(expectedOrder))
	}

	for idx, prefix := range expectedOrder {
		if !strings.HasPrefix(lines[idx], prefix) {
			return fmt.Errorf("sorted data %s at index %d did not match prefix expectation %s", lines[idx], idx, prefix)
		}
	}

	return nil
}

func checkExpectedOrderSuffixes(lines, expectedOrder []string) error {
	if len(lines) != len(expectedOrder) {
		return fmt.Errorf("sorted data length %d did not match expected length %d", len(lines), len(expectedOrder))
	}

	for idx, suffix := range expectedOrder {
		if !strings.HasSuffix(lines[idx], suffix) {
			return fmt.Errorf("sorted data %s at index %d did not match suffix expectation %s", lines[idx], idx, suffix)
		}
	}

	return nil
}

func TestFirstMatch(t *testing.T) {
	testCases := []struct {
		name    string
		pattern string
		inputs  []string
		match   string
	}{
		// Make sure we are anchoring the regex at the start and end
		{
			name:    "exact match no-substring",
			pattern: `asd`,
			inputs:  []string{"123asd123", "asd456", "123asd", "asd"},
			match:   "asd",
		},
		// Test that basic regex stuff works
		{
			name:    "don't match newline",
			pattern: `.*asd.*`,
			inputs:  []string{"123\nasd123", "123asd123", "asd"},
			match:   "123asd123",
		},
		{
			name:    "match newline",
			pattern: `(?s).*asd.*`,
			inputs:  []string{"123\nasd123", "123asd123"},
			match:   "123\nasd123",
		},
		{
			name:    "match multiline",
			pattern: `(?m)(^asd\d$\n?)+`,
			inputs:  []string{"asd1\nasd2\nasd3\n", "asd1"},
			match:   "asd1\nasd2\nasd3\n",
		},
		{
			name:    "don't match multiline",
			pattern: `(^asd\d$\n?)+`,
			inputs:  []string{"asd1\nasd2\nasd3\n", "asd1", "asd2"},
			match:   "asd1",
		},
		// Make sure that we group their pattern separately from the anchors
		{
			name:    "prefix alternation",
			pattern: `|asd`,
			inputs:  []string{"anything"},
			match:   "",
		},
		{
			name:    "postfix alternation",
			pattern: `asd|`,
			inputs:  []string{"anything"},
			match:   "",
		},
		// Make sure that a change in anchor behaviors doesn't break us
		{
			name:    "substring behavior",
			pattern: `(?m)asd`,
			inputs:  []string{"asd\n123"},
			match:   "",
		},
	}

	for _, tt := range testCases {
		match := firstMatch(tt.pattern, tt.inputs...)
		if match != tt.match {
			t.Errorf("%s: expected match of %v to %s is '%s', but didn't", tt.name, tt.inputs, tt.pattern, tt.match)
		}
	}
}

func TestGenerateRouteRegexp(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		path     string
		wildcard bool

		match   []string
		nomatch []string
	}{
		{
			name:     "no path",
			hostname: "example.com",
			path:     "",
			wildcard: false,
			match: []string{
				"example.com",
				"example.com:80",
				"example.com/",
				"example.com/sub",
				"example.com/sub/",
			},
			nomatch: []string{"other.com"},
		},
		{
			name:     "root path with trailing slash",
			hostname: "example.com",
			path:     "/",
			wildcard: false,
			match: []string{
				"example.com",
				"example.com:80",
				"example.com/",
				"example.com/sub",
				"example.com/sub/",
			},
			nomatch: []string{"other.com"},
		},
		{
			name:     "subpath with trailing slash",
			hostname: "example.com",
			path:     "/sub/",
			wildcard: false,
			match: []string{
				"example.com/sub/",
				"example.com/sub/subsub",
			},
			nomatch: []string{
				"other.com",
				"example.com",
				"example.com:80",
				"example.com/",
				"example.com/sub",    // path with trailing slash doesn't match URL without
				"example.com/subpar", // path segment boundary match required
			},
		},
		{
			name:     "subpath without trailing slash",
			hostname: "example.com",
			path:     "/sub",
			wildcard: false,
			match: []string{
				"example.com/sub",
				"example.com/sub/",
				"example.com/sub/subsub",
			},
			nomatch: []string{
				"other.com",
				"example.com",
				"example.com:80",
				"example.com/",
				"example.com/subpar", // path segment boundary match required
			},
		},
		{
			name:     "wildcard",
			hostname: "www.example.com",
			path:     "/",
			wildcard: true,
			match: []string{
				"www.example.com",
				"www.example.com/",
				"www.example.com/sub",
				"www.example.com/sub/",
				"www.example.com:80",
				"www.example.com:80/",
				"www.example.com:80/sub",
				"www.example.com:80/sub/",
				"foo.example.com",
				"foo.example.com/",
				"foo.example.com/sub",
				"foo.example.com/sub/",
			},
			nomatch: []string{
				"wwwexample.com",
				"foo.bar.example.com",
			},
		},
		{
			name:     "non-wildcard",
			hostname: "www.example.com",
			path:     "/",
			wildcard: false,
			match: []string{
				"www.example.com",
				"www.example.com/",
				"www.example.com/sub",
				"www.example.com/sub/",
				"www.example.com:80",
				"www.example.com:80/",
				"www.example.com:80/sub",
				"www.example.com:80/sub/",
			},
			nomatch: []string{
				"foo.example.com",
				"foo.example.com/",
				"foo.example.com/sub",
				"foo.example.com/sub/",
				"wwwexample.com",
				"foo.bar.example.com",
			},
		},
	}

	for _, tt := range tests {
		r := regexp.MustCompile(generateRouteRegexp(tt.hostname, tt.path, tt.wildcard))
		for _, s := range tt.match {
			if !r.Match([]byte(s)) {
				t.Errorf("%s: expected %s to match %s, but didn't", tt.name, r, s)
			}
		}
		for _, s := range tt.nomatch {
			if r.Match([]byte(s)) {
				t.Errorf("%s: expected %s not to match %s, but did", tt.name, r, s)
			}
		}
	}
}

func TestMatchPattern(t *testing.T) {
	testMatches := []struct {
		name    string
		pattern string
		input   string
	}{
		// Test that basic regex stuff works
		{
			name:    "exact match",
			pattern: `asd`,
			input:   "asd",
		},
		{
			name:    "basic regex",
			pattern: `.*asd.*`,
			input:   "123asd123",
		},
		{
			name:    "match newline",
			pattern: `(?s).*asd.*`,
			input:   "123\nasd123",
		},
		{
			name:    "match multiline",
			pattern: `(?m)(^asd\d$\n?)+`,
			input:   "asd1\nasd2\nasd3\n",
		},
	}

	testNoMatches := []struct {
		name    string
		pattern string
		input   string
	}{
		// Make sure we are anchoring the regex at the start and end
		{
			name:    "no-substring",
			pattern: `asd`,
			input:   "123asd123",
		},
		// Make sure that we group their pattern separately from the anchors
		{
			name:    "prefix alternation",
			pattern: `|asd`,
			input:   "anything",
		},
		{
			name:    "postfix alternation",
			pattern: `asd|`,
			input:   "anything",
		},
		// Make sure that a change in anchor behaviors doesn't break us
		{
			name:    "substring behavior",
			pattern: `(?m)asd`,
			input:   "asd\n123",
		},
		// Check some other regex things that should fail
		{
			name:    "don't match newline",
			pattern: `.*asd.*`,
			input:   "123\nasd123",
		},
		{
			name:    "don't match multiline",
			pattern: `(^asd\d$\n?)+`,
			input:   "asd1\nasd2\nasd3\n",
		},
	}

	for _, tt := range testMatches {
		match := matchPattern(tt.pattern, tt.input)
		if !match {
			t.Errorf("%s: expected %s to match %s, but didn't", tt.name, tt.input, tt.pattern)
		}
	}

	for _, tt := range testNoMatches {
		match := matchPattern(tt.pattern, tt.input)
		if match {
			t.Errorf("%s: expected %s not to match %s, but did", tt.name, tt.input, tt.pattern)
		}
	}
}

func createTempMapFile(prefix string, data []string) (string, error) {
	name := ""
	tempFile, err := ioutil.TempFile("", prefix)
	if err != nil {
		return "", fmt.Errorf("unexpected error creating temp file: %v", err)
	}

	name = tempFile.Name()
	if err = tempFile.Close(); err != nil {
		return name, fmt.Errorf("unexpected error creating temp file: %v", err)
	}

	if err := ioutil.WriteFile(name, []byte(strings.Join(data, "\n")), 0664); err != nil {
		return name, fmt.Errorf("unexpected error writing temp file %s: %v", name, err)
	}

	return name, nil
}

func TestGenerateHAProxyCertConfigMap(t *testing.T) {
	td := templateData{
		WorkingDir:   "/path/to",
		State:        buildTestTemplateState(),
		ServiceUnits: make(map[string]ServiceUnit),
	}

	expectedOrder := []string{
		"/path/to/certs/zzz:zed-route.pem",
		"/path/to/certs/test:api-route.pem",
		"/path/to/certs/stg:api-route.pem",
		"/path/to/certs/prod:wildcard-route.pem",
		"/path/to/certs/prod:backend-route.pem",
		"/path/to/certs/prod:api-route.pem",
		"/path/to/certs/prod:api-path-route.pem",
		"/path/to/certs/devel2:foo-wildcard-test.pem",
		"/path/to/certs/devel2:foo-wildcard-route.pem",
		"/path/to/certs/dev:reencrypt-route.pem",
		"/path/to/certs/dev:api-route.pem",
		"/path/to/certs/dev:admin-route.pem",
	}

	lines := generateHAProxyCertConfigMap(td)
	if err := checkExpectedOrderPrefixes(lines, expectedOrder); err != nil {
		t.Errorf("TestGenerateHAProxyCertConfigMap error: %v", err)
	}
}

func TestGenerateHAProxyMap(t *testing.T) {
	td := templateData{
		WorkingDir:   "/path/to",
		State:        buildTestTemplateState(),
		ServiceUnits: make(map[string]ServiceUnit),
	}

	wildcardDomainOrder := []string{
		`^[^\.]*\.foo\.wildcard\.test(:[0-9]+)?(/.*)?$`,
		`^[^\.]*\.foo\.127\.0\.0\.1\.nip\.io(:[0-9]+)?(/.*)?$`,
		`^[^\.]*\.127\.0\.0\.1\.nip\.io(:[0-9]+)?(/.*)?$`,
	}

	lines := generateHAProxyMap("os_wildcard_domain.map", td)
	if err := checkExpectedOrderPrefixes(lines, wildcardDomainOrder); err != nil {
		t.Errorf("TestGenerateHAProxyMap os_tcp_be.map error: %v", err)
	}

	httpBackendOrder := []string{
		"be_edge_http:zzz:zed-route",
		"be_edge_http:dev:api-route",
		"be_edge_http:devel2:foo-wildcard-test",
		"be_edge_http:devel2:foo-wildcard-route",
	}

	lines = generateHAProxyMap("os_http_be.map", td)
	if err := checkExpectedOrderSuffixes(lines, httpBackendOrder); err != nil {
		t.Errorf("TestGenerateHAProxyMap os_http_be.map error: %v", err)
	}

	edgeReencryptOrder := []string{
		"be_edge_http:test:api-route",
		"be_edge_http:zzz:zed-route",
		"be_secure:dev:reencrypt-route",
		"be_edge_http:prod:backend-route",
		"be_edge_http:stg:api-route",
		"be_edge_http:prod:api-path-route",
		"be_edge_http:prod:api-route",
		"be_edge_http:dev:api-route",
		"be_edge_http:dev:admin-route",
		"be_edge_http:devel2:foo-wildcard-test",
		"be_edge_http:devel2:foo-wildcard-route",
		"be_edge_http:prod:wildcard-route",
	}

	lines = generateHAProxyMap("os_edge_reencrypt_be.map", td)
	if err := checkExpectedOrderSuffixes(lines, edgeReencryptOrder); err != nil {
		t.Errorf("TestGenerateHAProxyMap os_edge_reencrypt_be.map error: %v", err)
	}

	httpRedirectOrder := []string{
		"test:api-route",
		"dev:reencrypt-route",
		"prod:backend-route",
		"stg:api-route",
		"prod:api-route",
	}

	lines = generateHAProxyMap("os_route_http_redirect.map", td)
	if err := checkExpectedOrderSuffixes(lines, httpRedirectOrder); err != nil {
		t.Errorf("TestGenerateHAProxyMap os_route_http_redirect.map error: %v", err)
	}

	passthroughOrder := []string{
		"dev:reencrypt-route",
		"prod:pt-route",
		"dev:pt-route",
	}

	lines = generateHAProxyMap("os_tcp_be.map", td)
	if err := checkExpectedOrderSuffixes(lines, passthroughOrder); err != nil {
		t.Errorf("TestGenerateHAProxyMap os_tcp_be.map error: %v", err)
	}

	sniPassthroughOrder := []string{
		`^passthrough-prod\.127\.0\.0\.1\.nip\.io(:[0-9]+)?(/.*)?$`,
		`^passthrough-dev\.127\.0\.0\.1\.nip\.io(:[0-9]+)?(/.*)?$`,
	}

	lines = generateHAProxyMap("os_sni_passthrough.map", td)
	if err := checkExpectedOrderPrefixes(lines, sniPassthroughOrder); err != nil {
		t.Errorf("TestGenerateHAProxyMap os_sni_passthrough.map error: %v", err)
	}

	certBackendOrder := []string{
		"/path/to/certs/zzz:zed-route.pem",
		"/path/to/certs/test:api-route.pem",
		"/path/to/certs/stg:api-route.pem",
		"/path/to/certs/prod:wildcard-route.pem",
		"/path/to/certs/prod:backend-route.pem",
		"/path/to/certs/prod:api-route.pem",
		"/path/to/certs/prod:api-path-route.pem",
		"/path/to/certs/devel2:foo-wildcard-test.pem",
		"/path/to/certs/devel2:foo-wildcard-route.pem",
		"/path/to/certs/dev:reencrypt-route.pem",
		"/path/to/certs/dev:api-route.pem",
		"/path/to/certs/dev:admin-route.pem",
	}

	lines = generateHAProxyMap("cert_config.map", td)
	if err := checkExpectedOrderPrefixes(lines, certBackendOrder); err != nil {
		t.Errorf("TestGenerateHAProxyMap cert_config.map error: %v", err)
	}
}

func TestGetHTTPAliasesGroupedByHost(t *testing.T) {
	aliases := map[string]ServiceAliasConfig{
		"project1:route1": {
			Host: "example.com",
			Path: "/",
		},
		"project2:route1": {
			Host: "example.org",
			Path: "/v1",
		},
		"project2:route2": {
			Host: "example.org",
			Path: "/v2",
		},
		"project3.route3": {
			Host:           "example.net",
			TLSTermination: routeapi.TLSTerminationPassthrough,
		},
	}

	expected := map[string]map[string]ServiceAliasConfig{
		"example.com": {
			"project1:route1": {
				Host: "example.com",
				Path: "/",
			},
		},
		"example.org": {
			"project2:route1": {
				Host: "example.org",
				Path: "/v1",
			},
			"project2:route2": {
				Host: "example.org",
				Path: "/v2",
			},
		},
	}

	result := getHTTPAliasesGroupedByHost(aliases)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("TestGroupAliasesByHost failed. Got %v expected %v", result, expected)
	}
}

func TestGetPrimaryAliasKey(t *testing.T) {
	testCases := []struct {
		name     string
		input    map[string]ServiceAliasConfig
		expected string
	}{
		{
			name:     "zero input",
			input:    make(map[string]ServiceAliasConfig),
			expected: "",
		},
		{
			name: "Single alias",
			input: map[string]ServiceAliasConfig{
				"project2:route1": {
					Host: "example.org",
					Path: "/v1",
				},
			},
			expected: "project2:route1",
		},
		{
			name: "Aliases with Edge Termination",
			input: map[string]ServiceAliasConfig{
				"project1:route-3": {
					Host:           "example.com",
					Path:           "/",
					TLSTermination: routeapi.TLSTerminationEdge,
				},
				"project1:route-1": {
					Host:           "example.com",
					Path:           "/path1",
					TLSTermination: routeapi.TLSTerminationEdge,
				},
				"project1:route-2": {
					Host:           "example.com",
					Path:           "/path2",
					TLSTermination: routeapi.TLSTerminationEdge,
				},
				"project1:route-4": {
					Host: "example.com",
					Path: "/path4",
				},
			},
			expected: "project1:route-3",
		},
		{
			name: "Aliases with Reencrypt Termination",
			input: map[string]ServiceAliasConfig{
				"project1:route-3": {
					Host:           "example.com",
					Path:           "/",
					TLSTermination: routeapi.TLSTerminationReencrypt,
				},
				"project1:route-1": {
					Host:           "example.com",
					Path:           "/path1",
					TLSTermination: routeapi.TLSTerminationReencrypt,
				},
				"project1:route-2": {
					Host:           "example.com",
					Path:           "/path2",
					TLSTermination: routeapi.TLSTerminationReencrypt,
				},
				"project1:route-4": {
					Host: "example.com",
					Path: "/path4",
				},
			},
			expected: "project1:route-3",
		},
		{
			name: "Non-TLS aliases",
			input: map[string]ServiceAliasConfig{
				"project1:route-3": {
					Host: "example.com",
					Path: "/",
				},
				"project1:route-1": {
					Host: "example.com",
					Path: "/path1",
				},
				"project1:route-2": {
					Host: "example.com",
					Path: "/path2",
				},
				"project1:route-4": {
					Host: "example.com",
					Path: "/path4",
				},
			},
			expected: "project1:route-4",
		},
	}

	for _, test := range testCases {
		result := getPrimaryAliasKey(test.input)

		if result != test.expected {
			t.Errorf("getPrimaryAliasKey failed. When testing for %v got %v expected %v", test.name, result, test.expected)
		}
	}
}

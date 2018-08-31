package haproxy

import (
	"fmt"
	"reflect"
	"testing"

	routev1 "github.com/openshift/api/route/v1"
	templateutil "github.com/openshift/origin/pkg/router/template/util"
)

func getTestTerminations() []routev1.TLSTerminationType {
	return []routev1.TLSTerminationType{
		routev1.TLSTerminationType(""),
		routev1.TLSTerminationEdge,
		routev1.TLSTerminationReencrypt,
		routev1.TLSTerminationPassthrough,
		routev1.TLSTerminationType("invalid"),
	}
}

func getTestInsecurePolicies() []routev1.InsecureEdgeTerminationPolicyType {
	return []routev1.InsecureEdgeTerminationPolicyType{
		routev1.InsecureEdgeTerminationPolicyNone,
		routev1.InsecureEdgeTerminationPolicyAllow,
		routev1.InsecureEdgeTerminationPolicyRedirect,
		routev1.InsecureEdgeTerminationPolicyType("hsts"),
		routev1.InsecureEdgeTerminationPolicyType("invalid2"),
	}
}

func testBackendConfig(name, host, path string, wildcard bool, termination routev1.TLSTerminationType, insecurePolicy routev1.InsecureEdgeTerminationPolicyType, hascert bool) *BackendConfig {
	return &BackendConfig{
		Name:           name,
		Host:           host,
		Path:           path,
		IsWildcard:     wildcard,
		Termination:    termination,
		InsecurePolicy: insecurePolicy,
		HasCertificate: hascert,
	}
}

func TestGenerateWildcardDomainMapEntry(t *testing.T) {
	mapName := "os_wildcard_domain.map"
	tests := []struct {
		name     string
		hostname string
		path     string
		wildcard bool
		expected *HAProxyMapEntry
	}{
		{
			name:     "empty host",
			hostname: "",
			path:     "",
			wildcard: false,
			expected: nil,
		},
		{
			name:     "empty host with path (ignored)",
			hostname: "",
			path:     "/ignored/path/to/resource",
			wildcard: false,
			expected: nil,
		},
		{
			name:     "host",
			hostname: "www.example.test",
			path:     "",
			wildcard: false,
			expected: nil,
		},
		{
			name:     "host with path (ignored)",
			hostname: "www.example.test",
			path:     "/x/y/z",
			wildcard: false,
			expected: nil,
		},
		{
			name:     "wildcard host",
			hostname: "www.wild.test",
			path:     "",
			wildcard: true,
			expected: &HAProxyMapEntry{
				Key:   `^[^\.]*\.wild\.test(:[0-9]+)?(/.*)?$`,
				Value: "1",
			},
		},
		{
			name:     "wildcard host with path (ignored)",
			hostname: "path.aces.wild.test",
			path:     "/ac/es/wi/ld/te/st",
			wildcard: true,
			expected: &HAProxyMapEntry{
				Key:   `^[^\.]*\.aces\.wild\.test(:[0-9]+)?(/.*)?$`,
				Value: "1",
			},
		},
	}

	for _, tc := range tests {
		configVariations := []*BackendConfig{}
		for _, termination := range getTestTerminations() {
			for _, policy := range getTestInsecurePolicies() {
				cfg := testBackendConfig(tc.name, tc.hostname, tc.path, tc.wildcard, termination, policy, false)
				configVariations = append(configVariations, cfg)
			}
		}

		for _, cfg := range configVariations {
			// directly call generator function
			entry := generateWildcardDomainMapEntry(cfg)
			if tc.expected == nil {
				if entry != nil {
					t.Errorf("direct:%s: did not expect a map entry, got %+v", tc.name, entry)
				}
			} else {
				if !reflect.DeepEqual(tc.expected, entry) {
					t.Errorf("direct:%s: expected map entry %+v, got %+v", tc.name, tc.expected, entry)

				}
			}

			// call via exported function
			entry = GenerateMapEntry(mapName, cfg)
			if tc.expected == nil {
				if entry != nil {
					t.Errorf("%s: did not expect a map entry, got %+v", tc.name, entry)
				}
			} else {
				if !reflect.DeepEqual(tc.expected, entry) {
					t.Errorf("%s: expected map entry %+v, got %+v", tc.name, tc.expected, entry)
				}
			}
		}
	}
}

func TestGenerateHttpMapEntry(t *testing.T) {
	mapName := "os_http_be.map"
	tests := []struct {
		name        string
		backendKey  string
		hostname    string
		path        string
		wildcard    bool
		expectedKey string
	}{
		{
			name:        "empty host",
			backendKey:  "test1",
			hostname:    "",
			path:        "",
			wildcard:    false,
			expectedKey: "",
		},
		{
			name:        "empty host with path",
			backendKey:  "test2",
			hostname:    "",
			path:        "/ignored/path/to/resource",
			wildcard:    false,
			expectedKey: "",
		},
		{
			name:        "host",
			backendKey:  "test_host",
			hostname:    "www.example.test",
			path:        "",
			wildcard:    false,
			expectedKey: `^www\.example\.test(:[0-9]+)?(/.*)?$`,
		},
		{
			name:        "host with path",
			backendKey:  "test_host_path",
			hostname:    "www.example.test",
			path:        "/x/y/z",
			wildcard:    false,
			expectedKey: `^www\.example\.test(:[0-9]+)?/x/y/z(/.*)?$`,
		},
		{
			name:        "wildcard host",
			backendKey:  "test_wildcard_host",
			hostname:    "www.wild.test",
			path:        "",
			wildcard:    true,
			expectedKey: `^[^\.]*\.wild\.test(:[0-9]+)?(/.*)?$`,
		},
		{
			name:        "wildcard host with path",
			backendKey:  "test_wildcard_host_path",
			hostname:    "path.aces.wild.test",
			path:        "/path/to/resource",
			wildcard:    true,
			expectedKey: `^[^\.]*\.aces\.wild\.test(:[0-9]+)?/path/to/resource(/.*)?$`,
		},
	}

	type testCase struct {
		name        string
		cfg         *BackendConfig
		expectation *HAProxyMapEntry
	}

	buildTestExpectation := func(name, key string, termination routev1.TLSTerminationType, policy routev1.InsecureEdgeTerminationPolicyType) *HAProxyMapEntry {
		if len(key) == 0 {
			return nil
		}

		if len(termination) > 0 && (policy != routev1.InsecureEdgeTerminationPolicyAllow || (termination != routev1.TLSTerminationEdge && termination != routev1.TLSTerminationReencrypt)) {
			return nil
		}

		value := fmt.Sprintf("%s:%s", templateutil.GenerateBackendNamePrefix(termination), name)
		return &HAProxyMapEntry{Key: key, Value: value}
	}

	for _, tt := range tests {
		testCases := []*testCase{}
		for _, termination := range getTestTerminations() {
			for _, policy := range getTestInsecurePolicies() {
				testCases = append(testCases, &testCase{
					name: fmt.Sprintf("%s:termination=%s:policy=%s", tt.name, termination, policy),
					cfg:  testBackendConfig(tt.backendKey, tt.hostname, tt.path, tt.wildcard, termination, policy, false),

					expectation: buildTestExpectation(tt.backendKey, tt.expectedKey, termination, policy),
				})
			}
		}

		for _, tc := range testCases {
			// directly call generator function
			entry := generateHttpMapEntry(tc.cfg)
			if tc.expectation == nil {
				if entry != nil {
					t.Errorf("direct:%s: did not expect a map entry, got %+v", tc.name, entry)
				}
			} else {
				if !reflect.DeepEqual(tc.expectation, entry) {
					t.Errorf("direct:%s: expected map entry %+v, got %+v", tc.name, tc.expectation, entry)
				}
			}

			// call via exported function
			entry = GenerateMapEntry(mapName, tc.cfg)
			if tc.expectation == nil {
				if entry != nil {
					t.Errorf("%s: did not expect a map entry, got %+v", tc.name, entry)
				}
			} else {
				if !reflect.DeepEqual(tc.expectation, entry) {
					t.Errorf("%s: expected map entry %+v, got %+v", tc.name, tc.expectation, entry)
				}
			}
		}
	}
}

func TestGenerateEdgeReencryptMapEntry(t *testing.T) {
	mapName := "os_edge_reencrypt_be.map"
	tests := []struct {
		name        string
		backendKey  string
		hostname    string
		path        string
		wildcard    bool
		expectedKey string
	}{
		{
			name:        "empty host",
			backendKey:  "test1",
			hostname:    "",
			path:        "",
			wildcard:    false,
			expectedKey: "",
		},
		{
			name:        "empty host with path",
			backendKey:  "test2",
			hostname:    "",
			path:        "/ignored/path/to/resource",
			wildcard:    false,
			expectedKey: "",
		},
		{
			name:        "host",
			backendKey:  "test_host",
			hostname:    "www.example.test",
			path:        "",
			wildcard:    false,
			expectedKey: `^www\.example\.test(:[0-9]+)?(/.*)?$`,
		},
		{
			name:        "host with path",
			backendKey:  "test_host_path",
			hostname:    "www.example.test",
			path:        "/x/y/z",
			wildcard:    false,
			expectedKey: `^www\.example\.test(:[0-9]+)?/x/y/z(/.*)?$`,
		},
		{
			name:        "wildcard host",
			backendKey:  "test_wildcard_host",
			hostname:    "www.wild.test",
			path:        "",
			wildcard:    true,
			expectedKey: `^[^\.]*\.wild\.test(:[0-9]+)?(/.*)?$`,
		},
		{
			name:        "wildcard host with path",
			backendKey:  "test_wildcard_host_path",
			hostname:    "path.aces.wild.test",
			path:        "/path/to/resource",
			wildcard:    true,
			expectedKey: `^[^\.]*\.aces\.wild\.test(:[0-9]+)?/path/to/resource(/.*)?$`,
		},
	}

	type testCase struct {
		name        string
		cfg         *BackendConfig
		expectation *HAProxyMapEntry
	}

	buildTestExpectation := func(name, key string, termination routev1.TLSTerminationType) *HAProxyMapEntry {
		if len(key) == 0 {
			return nil
		}

		if termination == routev1.TLSTerminationEdge || termination == routev1.TLSTerminationReencrypt {
			value := fmt.Sprintf("%s:%s", templateutil.GenerateBackendNamePrefix(termination), name)
			return &HAProxyMapEntry{Key: key, Value: value}
		}

		return nil
	}

	for _, tt := range tests {
		testCases := []*testCase{}
		for _, termination := range getTestTerminations() {
			for _, policy := range getTestInsecurePolicies() {
				testCases = append(testCases, &testCase{
					name: fmt.Sprintf("%s:termination=%s:policy=%s", tt.name, termination, policy),
					cfg:  testBackendConfig(tt.backendKey, tt.hostname, tt.path, tt.wildcard, termination, policy, false),

					expectation: buildTestExpectation(tt.backendKey, tt.expectedKey, termination),
				})
			}
		}

		for _, tc := range testCases {
			// directly call generator function
			entry := generateEdgeReencryptMapEntry(tc.cfg)
			if tc.expectation == nil {
				if entry != nil {
					t.Errorf("direct:%s: did not expect a map entry, got %+v", tc.name, entry)
				}
			} else {
				if !reflect.DeepEqual(tc.expectation, entry) {
					t.Errorf("direct:%s: expected map entry %+v, got %+v", tc.name, tc.expectation, entry)
				}
			}

			// call via exported function
			entry = GenerateMapEntry(mapName, tc.cfg)
			if tc.expectation == nil {
				if entry != nil {
					t.Errorf("%s: did not expect a map entry, got %+v", tc.name, entry)
				}
			} else {
				if !reflect.DeepEqual(tc.expectation, entry) {
					t.Errorf("%s: expected map entry %+v, got %+v", tc.name, tc.expectation, entry)
				}
			}
		}
	}
}

func TestGenerateHttpRedirectMapEntry(t *testing.T) {
	mapName := "os_route_http_redirect.map"
	tests := []struct {
		name        string
		backendKey  string
		hostname    string
		path        string
		wildcard    bool
		expectedKey string
	}{
		{
			name:        "empty host",
			backendKey:  "test1",
			hostname:    "",
			path:        "",
			wildcard:    false,
			expectedKey: "",
		},
		{
			name:        "empty host with path",
			backendKey:  "test2",
			hostname:    "",
			path:        "/ignored/path/to/resource",
			wildcard:    false,
			expectedKey: "",
		},
		{
			name:        "host",
			backendKey:  "test_host",
			hostname:    "www.example.test",
			path:        "",
			wildcard:    false,
			expectedKey: `^www\.example\.test(:[0-9]+)?(/.*)?$`,
		},
		{
			name:        "host with path",
			backendKey:  "test_host_path",
			hostname:    "www.example.test",
			path:        "/x/y/z",
			wildcard:    false,
			expectedKey: `^www\.example\.test(:[0-9]+)?/x/y/z(/.*)?$`,
		},
		{
			name:        "wildcard host",
			backendKey:  "test_wildcard_host",
			hostname:    "www.wild.test",
			path:        "",
			wildcard:    true,
			expectedKey: `^[^\.]*\.wild\.test(:[0-9]+)?(/.*)?$`,
		},
		{
			name:        "wildcard host with path",
			backendKey:  "test_wildcard_host_path",
			hostname:    "path.aces.wild.test",
			path:        "/path/to/resource",
			wildcard:    true,
			expectedKey: `^[^\.]*\.aces\.wild\.test(:[0-9]+)?/path/to/resource(/.*)?$`,
		},
	}

	type testCase struct {
		name        string
		cfg         *BackendConfig
		expectation *HAProxyMapEntry
	}

	buildTestExpectation := func(name, key string, policy routev1.InsecureEdgeTerminationPolicyType) *HAProxyMapEntry {
		if len(key) == 0 {
			return nil
		}

		if policy == routev1.InsecureEdgeTerminationPolicyRedirect {
			return &HAProxyMapEntry{Key: key, Value: name}
		}

		return nil
	}

	for _, tt := range tests {
		testCases := []*testCase{}
		for _, termination := range getTestTerminations() {
			for _, policy := range getTestInsecurePolicies() {
				testCases = append(testCases, &testCase{
					name: fmt.Sprintf("%s:termination=%s:policy=%s", tt.name, termination, policy),
					cfg:  testBackendConfig(tt.backendKey, tt.hostname, tt.path, tt.wildcard, termination, policy, false),

					expectation: buildTestExpectation(tt.backendKey, tt.expectedKey, policy),
				})
			}
		}

		for _, tc := range testCases {
			// directly call generator function
			entry := generateHttpRedirectMapEntry(tc.cfg)
			if tc.expectation == nil {
				if entry != nil {
					t.Errorf("direct:%s: did not expect a map entry, got %+v", tc.name, entry)
				}
			} else {
				if !reflect.DeepEqual(tc.expectation, entry) {
					t.Errorf("direct:%s: expected map entry %+v, got %+v", tc.name, tc.expectation, entry)
				}
			}

			// call via exported function
			entry = GenerateMapEntry(mapName, tc.cfg)
			if tc.expectation == nil {
				if entry != nil {
					t.Errorf("%s: did not expect a map entry, got %+v", tc.name, entry)
				}
			} else {
				if !reflect.DeepEqual(tc.expectation, entry) {
					t.Errorf("%s: expected map entry %+v, got %+v", tc.name, tc.expectation, entry)
				}
			}
		}
	}
}

func TestGenerateTCPMapEntry(t *testing.T) {
	mapName := "os_tcp_be.map"
	tests := []struct {
		name        string
		backendKey  string
		hostname    string
		path        string
		wildcard    bool
		expectedKey string
	}{
		{
			name:        "empty host",
			backendKey:  "test1",
			hostname:    "",
			path:        "",
			wildcard:    false,
			expectedKey: "",
		},
		{
			name:        "empty host with path",
			backendKey:  "test2",
			hostname:    "",
			path:        "/ignored/path/to/resource",
			wildcard:    false,
			expectedKey: "",
		},
		{
			name:        "host",
			backendKey:  "test_host",
			hostname:    "www.example.test",
			path:        "",
			wildcard:    false,
			expectedKey: `^www\.example\.test(:[0-9]+)?(/.*)?$`,
		},
		{
			name:        "host with path",
			backendKey:  "test_host_path",
			hostname:    "www.example.test",
			path:        "/x/y/z",
			wildcard:    false,
			expectedKey: "",
		},
		{
			name:        "wildcard host",
			backendKey:  "test_wildcard_host",
			hostname:    "www.wild.test",
			path:        "",
			wildcard:    true,
			expectedKey: `^[^\.]*\.wild\.test(:[0-9]+)?(/.*)?$`,
		},
		{
			name:        "wildcard host with path",
			backendKey:  "test_wildcard_host_path",
			hostname:    "path.aces.wild.test",
			path:        "/path/to/resource",
			wildcard:    true,
			expectedKey: "",
		},
	}

	type testCase struct {
		name        string
		cfg         *BackendConfig
		expectation *HAProxyMapEntry
	}

	buildTestExpectation := func(name, key string, termination routev1.TLSTerminationType) *HAProxyMapEntry {
		if len(key) == 0 {
			return nil
		}

		if termination == routev1.TLSTerminationPassthrough || termination == routev1.TLSTerminationReencrypt {
			return &HAProxyMapEntry{Key: key, Value: name}
		}

		return nil
	}

	for _, tt := range tests {
		testCases := []*testCase{}
		for _, termination := range getTestTerminations() {
			for _, policy := range getTestInsecurePolicies() {
				backendKey := fmt.Sprintf("be_secure:%s", tt.backendKey)
				if termination == routev1.TLSTerminationPassthrough {
					backendKey = fmt.Sprintf("be_tcp:%s", tt.backendKey)
				}
				testCases = append(testCases, &testCase{
					name: fmt.Sprintf("%s:termination=%s:policy=%s", tt.name, termination, policy),
					cfg:  testBackendConfig(tt.backendKey, tt.hostname, tt.path, tt.wildcard, termination, policy, false),

					expectation: buildTestExpectation(backendKey, tt.expectedKey, termination),
				})
			}
		}

		for _, tc := range testCases {
			// directly call generator function
			entry := generateTCPMapEntry(tc.cfg)
			if tc.expectation == nil {
				if entry != nil {
					t.Errorf("direct:%s: did not expect a map entry, got %+v", tc.name, entry)
				}
			} else {
				if !reflect.DeepEqual(tc.expectation, entry) {
					t.Errorf("direct:%s: expected map entry %+v, got %+v", tc.name, tc.expectation, entry)
				}
			}

			// call via exported function
			entry = GenerateMapEntry(mapName, tc.cfg)
			if tc.expectation == nil {
				if entry != nil {
					t.Errorf("%s: did not expect a map entry, got %+v", tc.name, entry)
				}
			} else {
				if !reflect.DeepEqual(tc.expectation, entry) {
					t.Errorf("%s: expected map entry %+v, got %+v", tc.name, tc.expectation, entry)
				}
			}
		}
	}
}

func TestGenerateSNIPassthroughMapEntry(t *testing.T) {
	mapName := "os_sni_passthrough.map"
	tests := []struct {
		name        string
		backendKey  string
		hostname    string
		path        string
		wildcard    bool
		expectedKey string
	}{
		{
			name:        "empty host",
			backendKey:  "test1",
			hostname:    "",
			path:        "",
			wildcard:    false,
			expectedKey: "",
		},
		{
			name:        "empty host with path",
			backendKey:  "test2",
			hostname:    "",
			path:        "/ignored/path/to/resource",
			wildcard:    false,
			expectedKey: "",
		},
		{
			name:        "host",
			backendKey:  "test_host",
			hostname:    "www.example.test",
			path:        "",
			wildcard:    false,
			expectedKey: `^www\.example\.test(:[0-9]+)?(/.*)?$`,
		},
		{
			name:        "host with path",
			backendKey:  "test_host_path",
			hostname:    "www.example.test",
			path:        "/x/y/z",
			wildcard:    false,
			expectedKey: "",
		},
		{
			name:        "wildcard host",
			backendKey:  "test_wildcard_host",
			hostname:    "www.wild.test",
			path:        "",
			wildcard:    true,
			expectedKey: `^[^\.]*\.wild\.test(:[0-9]+)?(/.*)?$`,
		},
		{
			name:        "wildcard host with path",
			backendKey:  "test_wildcard_host_path",
			hostname:    "path.aces.wild.test",
			path:        "/path/to/resource",
			wildcard:    true,
			expectedKey: "",
		},
	}

	type testCase struct {
		name        string
		cfg         *BackendConfig
		expectation *HAProxyMapEntry
	}

	buildTestExpectation := func(name, key string, termination routev1.TLSTerminationType) *HAProxyMapEntry {
		if len(key) == 0 {
			return nil
		}

		if termination == routev1.TLSTerminationPassthrough {
			return &HAProxyMapEntry{Key: key, Value: "1"}
		}

		return nil
	}

	for _, tt := range tests {
		testCases := []*testCase{}
		for _, termination := range getTestTerminations() {
			for _, policy := range getTestInsecurePolicies() {
				testCases = append(testCases, &testCase{
					name: fmt.Sprintf("%s:termination=%s:policy=%s", tt.name, termination, policy),
					cfg:  testBackendConfig(tt.backendKey, tt.hostname, tt.path, tt.wildcard, termination, policy, false),

					expectation: buildTestExpectation(tt.backendKey, tt.expectedKey, termination),
				})
			}
		}

		for _, tc := range testCases {
			// directly call generator function
			entry := generateSNIPassthroughMapEntry(tc.cfg)
			if tc.expectation == nil {
				if entry != nil {
					t.Errorf("direct:%s: did not expect a map entry, got %+v", tc.name, entry)
				}
			} else {
				if !reflect.DeepEqual(tc.expectation, entry) {
					t.Errorf("direct:%s: expected map entry %+v, got %+v", tc.name, tc.expectation, entry)
				}
			}

			// call via exported function
			entry = GenerateMapEntry(mapName, tc.cfg)
			if tc.expectation == nil {
				if entry != nil {
					t.Errorf("%s: did not expect a map entry, got %+v", tc.name, entry)
				}
			} else {
				if !reflect.DeepEqual(tc.expectation, entry) {
					t.Errorf("%s: expected map entry %+v, got %+v", tc.name, tc.expectation, entry)
				}
			}
		}
	}
}

func TestGenerateCertConfigMapEntry(t *testing.T) {
	mapName := "cert_config.map"
	tests := []struct {
		name        string
		backendKey  string
		hostname    string
		wildcard    bool
		hascert     bool
		expectedKey string
	}{
		{
			name:        "empty host without cert",
			backendKey:  "empty_host",
			hostname:    "",
			wildcard:    false,
			hascert:     false,
			expectedKey: "",
		},
		{
			name:        "empty host with cert",
			backendKey:  "empty_host_cert",
			hostname:    "",
			wildcard:    false,
			hascert:     true,
			expectedKey: "",
		},
		{
			name:        "host without cert",
			backendKey:  "test_host",
			hostname:    "www.example.test",
			wildcard:    false,
			hascert:     false,
			expectedKey: "",
		},
		{
			name:        "host with cert",
			backendKey:  "test_host_cert",
			hostname:    "www.example.test",
			wildcard:    false,
			hascert:     true,
			expectedKey: "test_host_cert.pem",
		},
		{
			name:        "wildcard host without cert",
			backendKey:  "test_wildcard_host",
			hostname:    "www.wild.test",
			wildcard:    true,
			hascert:     false,
			expectedKey: "",
		},
		{
			name:        "wildcard host with cert",
			backendKey:  "test_wildcard_host_cert",
			hostname:    "www.wild.test",
			wildcard:    true,
			hascert:     true,
			expectedKey: "test_wildcard_host_cert.pem",
		},
	}

	type testCase struct {
		name        string
		cfg         *BackendConfig
		expectation *HAProxyMapEntry
	}

	buildTestExpectation := func(host, key string, wildcard bool, termination routev1.TLSTerminationType, hascert bool) *HAProxyMapEntry {
		if len(key) == 0 || !hascert || (termination != routev1.TLSTerminationEdge && termination != routev1.TLSTerminationReencrypt) {
			return nil
		}

		certHost := templateutil.GenCertificateHostName(host, wildcard)
		return &HAProxyMapEntry{Key: key, Value: certHost}
	}

	for _, tt := range tests {
		testCases := []*testCase{}
		for _, termination := range getTestTerminations() {
			for _, policy := range getTestInsecurePolicies() {
				testCases = append(testCases, &testCase{
					name: fmt.Sprintf("%s:termination=%s:policy=%s", tt.name, termination, policy),
					cfg:  testBackendConfig(tt.backendKey, tt.hostname, "", tt.wildcard, termination, policy, tt.hascert),

					expectation: buildTestExpectation(tt.hostname, tt.expectedKey, tt.wildcard, termination, tt.hascert),
				})
			}
		}

		for _, tc := range testCases {
			// directly call generator function
			entry := generateCertConfigMapEntry(tc.cfg)
			if tc.expectation == nil {
				if entry != nil {
					t.Errorf("direct:%s: did not expect a map entry, got %+v", tc.name, entry)
				}
			} else {
				if !reflect.DeepEqual(tc.expectation, entry) {
					t.Errorf("direct:%s: expected map entry %+v, got %+v", tc.name, tc.expectation, entry)
				}
			}

			// call via exported function
			entry = GenerateMapEntry(mapName, tc.cfg)
			if tc.expectation == nil {
				if entry != nil {
					t.Errorf("%s: did not expect a map entry, got %+v", tc.name, entry)
				}
			} else {
				if !reflect.DeepEqual(tc.expectation, entry) {
					t.Errorf("%s: expected map entry %+v, got %+v", tc.name, tc.expectation, entry)
				}
			}
		}
	}
}

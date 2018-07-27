package haproxy

import (
	"bytes"
	"fmt"
	"testing"

	haproxytesting "github.com/openshift/origin/pkg/router/template/configmanager/haproxy/testing"
)

// TestNewConverter tests a new converter.
func TestNewConverter(t *testing.T) {
	testCases := []struct {
		name    string
		headers string
		fn      ByteConverterFunc
	}{
		{
			name:    "empty headers",
			headers: "",
			fn:      noopConverter,
		},
		{
			name:    "nil converter",
			headers: "#a",
		},
		{
			name:    "noop converter",
			headers: "#no o p",
			fn:      noopConverter,
		},
		{
			name:    "removing leading hash converter",
			headers: "#d e l",
			fn:      removeLeadingHashConverter,
		},
		{
			name:    "comment first line converter",
			headers: "#comment",
			fn:      commentFirstLineConverter,
		},
		{
			name:    "remove first line converter",
			headers: "#rm - r f",
			fn:      removeFirstLineConverter,
		},
		{
			name:    "error converter",
			headers: "#raise throw error",
			fn:      errorConverter,
		},
	}

	for _, tc := range testCases {
		entries := []*infoEntry{}
		if c := NewCSVConverter(tc.headers, entries, tc.fn); c == nil {
			t.Errorf("TestNewConverter test case %s failed.  Unexpected error", tc.name)
		}
	}
}

// TestShowInfoCommandConverter tests show info command output with a converter.
func TestShowInfoCommandConverter(t *testing.T) {
	infoCommandOutput := `Name: converter-test
Version: 0.0.1
Nbproc: 1
Process_num: 1
Pid: 42
`

	testCases := []struct {
		name            string
		commandOutput   string
		header          string
		converter       ByteConverterFunc
		failureExpected bool
	}{
		{
			name:            "info parser",
			commandOutput:   infoCommandOutput,
			header:          "name value",
			converter:       nil,
			failureExpected: false,
		},
		{
			name:            "info parser with noop converter",
			commandOutput:   infoCommandOutput,
			header:          "name value",
			converter:       noopConverter,
			failureExpected: false,
		},
		{
			name:            "info parser with comment header",
			commandOutput:   infoCommandOutput,
			header:          "#name value",
			converter:       noopConverter,
			failureExpected: false,
		},
		{
			name:            "output with header",
			commandOutput:   "#name value\n" + infoCommandOutput,
			header:          "",
			converter:       removeLeadingHashConverter,
			failureExpected: false,
		},
		{
			name:            "output without header",
			commandOutput:   "name value\n" + infoCommandOutput,
			header:          "",
			converter:       commentFirstLineConverter,
			failureExpected: false,
		},
		{
			name:            "output with error converter",
			commandOutput:   infoCommandOutput,
			header:          "#name value",
			converter:       errorConverter,
			failureExpected: true,
		},
		{
			name:            "output with bad header",
			commandOutput:   infoCommandOutput,
			header:          "# name value extra1 extra2",
			converter:       nil,
			failureExpected: true,
		},
		{
			name:            "output with bad header 2",
			commandOutput:   infoCommandOutput,
			header:          "# name value extra1 extra2",
			converter:       removeLeadingHashConverter,
			failureExpected: true,
		},
		{
			name:            "output with empty header",
			commandOutput:   "name value\n" + infoCommandOutput,
			header:          "",
			converter:       commentFirstLineConverter,
			failureExpected: false,
		},
		{
			name:            "bad command output with header",
			commandOutput:   "command error 404 - check params",
			header:          "field1 field2 field3",
			converter:       nil,
			failureExpected: true,
		},
	}

	for _, tc := range testCases {
		entries := []*infoEntry{}
		c := NewCSVConverter(tc.header, &entries, tc.converter)
		response, err := c.Convert([]byte(tc.commandOutput))
		if tc.failureExpected && err == nil {
			t.Errorf("TestShowInfoCommandConverter test case %s expected a failure but got none, response=%s",
				tc.name, string(response))
		}
		if !tc.failureExpected && err != nil {
			t.Errorf("TestShowInfoCommandConverter test case %s expected no failure but got one: %v", tc.name, err)
		}
	}
}

// TestShowBackendCommandConverter tests show backend command output with a converter.
func TestShowBackendCommandConverter(t *testing.T) {
	showBackendOutput := `# name
be_sni
be_no_sni
openshift_default
be_edge_http:blueprints:blueprint-redirect-to-https
be_edge_http:default:example-route
be_edge_http:default:test-http-allow
be_edge_http:default:test-https
be_edge_http:default:test-https-only
be_edge_http:ns1:example-route
be_tcp:default:test-passthrough
be_tcp:ns1:passthru-1
be_tcp:ns2:passthru-1
be_secure:default:test
be_secure:ns1:re1
be_secure:ns2:reencrypt-one
be_secure:ns2:reencrypt-two
be_secure:ns2:reencrypt-three
be_secure:ns3:re1
be_secure:ns3:re2
`
	testCases := []struct {
		name            string
		commandOutput   string
		header          string
		converter       ByteConverterFunc
		failureExpected bool
	}{
		{
			name:            "show backend command",
			commandOutput:   showBackendOutput,
			header:          "name",
			converter:       nil,
			failureExpected: false,
		},
		{
			name:            "show backend with noop converter",
			commandOutput:   showBackendOutput,
			header:          "name",
			converter:       noopConverter,
			failureExpected: false,
		},
		{
			name:            "show backend removing leading hash",
			commandOutput:   showBackendOutput,
			header:          "name",
			converter:       removeLeadingHashConverter,
			failureExpected: false,
		},
		{
			name:            "show backend removing leading hash 2",
			commandOutput:   showBackendOutput,
			header:          "#name",
			converter:       removeLeadingHashConverter,
			failureExpected: false,
		},
		{
			name:            "show backend comment first line",
			commandOutput:   showBackendOutput[1:],
			header:          "name",
			converter:       commentFirstLineConverter,
			failureExpected: false,
		},
		{
			name:            "show backend remove first line",
			commandOutput:   showBackendOutput,
			header:          "name",
			converter:       removeFirstLineConverter,
			failureExpected: false,
		},
		{
			name:            "show backend error converter",
			commandOutput:   showBackendOutput,
			header:          "name",
			converter:       errorConverter,
			failureExpected: true,
		},
		{
			name:            "empty output error converter",
			commandOutput:   "",
			header:          "name",
			converter:       errorConverter,
			failureExpected: true,
		},
		{
			name:            "show backend error output",
			commandOutput:   "connection failed, no backends",
			header:          "name",
			converter:       noopConverter,
			failureExpected: true,
		},
	}

	for _, tc := range testCases {
		entries := []*backendEntry{}
		c := NewCSVConverter(tc.header, &entries, tc.converter)
		response, err := c.Convert([]byte(tc.commandOutput))
		if tc.failureExpected && err == nil {
			t.Errorf("TestShowBackendCommandConverter test case %s expected a failure but got none, response=%s",
				tc.name, string(response))
		}
		if !tc.failureExpected && err != nil {
			t.Errorf("TestShowBackendCommandConverter test case %s expected no failure but got one: %v", tc.name, err)
		}
	}
}

// TestShowMapCommandConverter tests show map command output with a converter.
func TestShowMapCommandConverter(t *testing.T) {
	listMapOutput := `# id (file) description
1 (/var/lib/haproxy/conf/os_route_http_redirect.map) pattern loaded from file '/var/lib/haproxy/conf/os_route_http_redirect.map' used by map at file '/var/lib/haproxy/conf/haproxy.config' line 68
5 (/var/lib/haproxy/conf/os_sni_passthrough.map) pattern loaded from file '/var/lib/haproxy/conf/os_sni_passthrough.map' used by map at file '/var/lib/haproxy/conf/haproxy.config' line 87
-1 (/var/lib/haproxy/conf/os_http_be.map) pattern loaded from file '/var/lib/haproxy/conf/os_http_be.map' used by map at file '/var/lib/haproxy/conf/haproxy.config' line 71
`

	testCases := []struct {
		name            string
		commandOutput   string
		header          string
		converter       ByteConverterFunc
		failureExpected bool
	}{
		{
			name:            "show map",
			commandOutput:   listMapOutput,
			header:          "id (file) description",
			converter:       fixupMapListOutput,
			failureExpected: false,
		},
		{
			name:            "show map with no converter",
			commandOutput:   listMapOutput,
			header:          "id (file) description",
			converter:       nil,
			failureExpected: true,
		},
		{
			name:            "show map without map fixup",
			commandOutput:   listMapOutput,
			header:          "id (file) description",
			converter:       removeFirstLineConverter,
			failureExpected: true,
		},
		{
			name:            "show map with error converter",
			commandOutput:   listMapOutput,
			header:          "id (file) description",
			converter:       errorConverter,
			failureExpected: true,
		},
		{
			name:            "show map with error converter 2",
			commandOutput:   "",
			header:          "id (file) description",
			converter:       errorConverter,
			failureExpected: true,
		},
		{
			name:            "show map bad output",
			commandOutput:   "error fetching list of maps: connection failed",
			header:          "id (file) description",
			converter:       fixupMapListOutput,
			failureExpected: true,
		},
	}

	for _, tc := range testCases {
		entries := []*mapListEntry{}
		c := NewCSVConverter(tc.header, &entries, tc.converter)
		response, err := c.Convert([]byte(tc.commandOutput))
		if tc.failureExpected && err == nil {
			t.Errorf("TestShowMapCommandConverter test case %s expected a failure but got none, response=%s",
				tc.name, string(response))
		}
		if !tc.failureExpected && err != nil {
			t.Errorf("TestShowMapCommandConverter test case %s expected no failure but got one: %v", tc.name, err)
		}
	}
}

// TestShowServerStateOutputConverter tests show servers state output with a converter.
func TestShowServerStateOutputConverter(t *testing.T) {
	testCases := []struct {
		name            string
		commandOutput   string
		header          string
		converter       ByteConverterFunc
		failureExpected bool
	}{
		{
			name:            "show servers state",
			commandOutput:   haproxytesting.OnePodAndOneDynamicServerBackendTemplate,
			header:          serversStateHeader,
			converter:       stripVersionNumber,
			failureExpected: false,
		},
		{
			name:            "show servers state without a converter",
			commandOutput:   haproxytesting.OnePodAndOneDynamicServerBackendTemplate,
			header:          serversStateHeader,
			converter:       nil,
			failureExpected: true,
		},
		{
			name:            "show servers state without removing version number",
			commandOutput:   haproxytesting.OnePodAndOneDynamicServerBackendTemplate,
			header:          serversStateHeader,
			converter:       removeLeadingHashConverter,
			failureExpected: true,
		},
		{
			name:            "show servers state removing first line with version number",
			commandOutput:   haproxytesting.OnePodAndOneDynamicServerBackendTemplate,
			header:          serversStateHeader,
			converter:       removeFirstLineConverter,
			failureExpected: false,
		},
		{
			name:            "show servers state with error converter",
			commandOutput:   haproxytesting.OnePodAndOneDynamicServerBackendTemplate,
			header:          serversStateHeader,
			converter:       errorConverter,
			failureExpected: true,
		},
		{
			name:            "show servers state with error output",
			commandOutput:   "error: failed to find backend",
			header:          serversStateHeader,
			converter:       nil,
			failureExpected: true,
		},
	}

	for _, tc := range testCases {
		entries := []*serverStateInfo{}
		c := NewCSVConverter(tc.header, &entries, tc.converter)
		response, err := c.Convert([]byte(tc.commandOutput))
		if tc.failureExpected && err == nil {
			t.Errorf("TestShowServerStateOutputConverter test case %s expected a failure but got none, response=%s",
				tc.name, string(response))
		}
		if !tc.failureExpected && err != nil {
			t.Errorf("TestShowServerStateOutputConverter test case %s expected no failure but got one: %v", tc.name, err)
		}
	}
}

func noopConverter(data []byte) ([]byte, error) {
	return data, nil
}

func removeLeadingHashConverter(data []byte) ([]byte, error) {
	prefix := []byte("#")
	idx := 0
	if len(data) > 0 && !bytes.HasPrefix(data, prefix) {
		idx = 1
	}

	return data[idx:], nil
}

func commentFirstLineConverter(data []byte) ([]byte, error) {
	return bytes.Join([][]byte{[]byte("#"), data}, []byte("")), nil
}

func removeFirstLineConverter(data []byte) ([]byte, error) {
	if len(data) > 0 {
		idx := bytes.Index(data, []byte("\n"))
		if idx > -1 {
			if idx+1 < len(data) {
				return data[idx+1:], nil
			}
		}
	}
	return []byte(""), nil
}

func errorConverter(data []byte) ([]byte, error) {
	return data, fmt.Errorf("converter test error")
}

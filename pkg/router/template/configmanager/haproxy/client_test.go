package haproxy

import (
	"testing"

	haproxytesting "github.com/openshift/origin/pkg/router/template/configmanager/haproxy/testing"
)

type infoEntry struct {
	Name  string `csv:"name"`
	Value string `csv:"value"`
}

// TestNewClient tests a new client.
func TestNewClient(t *testing.T) {
	testCases := []struct {
		name     string
		sockFile string
	}{
		{
			name:     "empty sockfile",
			sockFile: "",
		},
		{
			name:     "some sockfile",
			sockFile: "/tmp/some-fake-haproxy.sock",
		},
		{
			name:     "bad socketfile",
			sockFile: "/non-existent/fake-haproxy.sock",
		},
	}
	for _, tc := range testCases {
		if client := NewClient(tc.sockFile, 0); client == nil {
			t.Errorf("TestNewClient test case %s failed.  Unexpected error", tc.name)
		}
	}
}

// TestClientRunCommand tests client command execution.
func TestClientRunCommand(t *testing.T) {
	server := haproxytesting.StartFakeServerForTest(t)
	defer server.Stop()

	testCases := []struct {
		name            string
		sockFile        string
		failureExpected bool
	}{
		{
			name:            "empty sockfile",
			sockFile:        "",
			failureExpected: true,
		},
		{
			name:            "bad socketfile",
			sockFile:        "/non-existent/fake-haproxy.sock",
			failureExpected: true,
		},
		{
			name:            "valid sockfile",
			sockFile:        server.SocketFile(),
			failureExpected: false,
		},
	}

	for _, tc := range testCases {
		client := NewClient(tc.sockFile, 1)
		response, err := client.RunCommand("show info", nil)
		if tc.failureExpected && err == nil {
			t.Errorf("TestClientRunCommand test case %s expected a failure but got none, response=%s",
				tc.name, string(response))
		}
		if !tc.failureExpected && err != nil {
			t.Errorf("TestClientRunCommand test case %s expected no failure but got one: %v", tc.name, err)
		}
	}
}

// TestClientRunInfoCommandConverter tests client show info command execution with a converter.
func TestClientRunInfoCommandConverter(t *testing.T) {
	testCases := []struct {
		name            string
		command         string
		header          string
		converter       ByteConverterFunc
		failureExpected bool
	}{
		{
			name:            "info parser",
			command:         "show info",
			header:          "name value",
			converter:       nil,
			failureExpected: false,
		},
		{
			name:            "info parser with comment header",
			command:         "show info",
			header:          "#name value",
			converter:       nil,
			failureExpected: false,
		},
		{
			name:            "info parser with bad header",
			command:         "show info",
			header:          "# name value extra1 extra2",
			converter:       nil,
			failureExpected: true,
		},
		{
			name:            "info parser with empty header",
			command:         "show info",
			header:          "",
			converter:       nil,
			failureExpected: false,
		},
		{
			name:            "bad command with header",
			command:         "bad command",
			header:          "field1 field2 field3",
			converter:       nil,
			failureExpected: true,
		},
	}

	server := haproxytesting.StartFakeServerForTest(t)
	defer server.Stop()

	for _, tc := range testCases {
		client := NewClient(server.SocketFile(), 1)
		entries := []*infoEntry{}
		csvcon := NewCSVConverter(tc.header, &entries, tc.converter)
		response, err := client.RunCommand(tc.command, csvcon)
		if tc.failureExpected && err == nil {
			t.Errorf("TestClientRunInfoCommandConverter test case %s expected a failure but got none, response=%s",
				tc.name, string(response))
		}
		if !tc.failureExpected && err != nil {
			t.Errorf("TestClientRunInfoCommandConverter test case %s expected no failure but got one: %v", tc.name, err)
		}
	}
}

// TestClientRunBackendCommandConverter tests client show backend command execution with a converter.
func TestClientRunBackendCommandConverter(t *testing.T) {
	testCases := []struct {
		name            string
		command         string
		header          string
		converter       ByteConverterFunc
		failureExpected bool
	}{
		{
			name:            "show backend command",
			command:         "show backend",
			header:          "name",
			converter:       nil,
			failureExpected: false,
		},
	}

	server := haproxytesting.StartFakeServerForTest(t)
	defer server.Stop()

	for _, tc := range testCases {
		client := NewClient(server.SocketFile(), 1)
		entries := []*backendEntry{}
		csvcon := NewCSVConverter(tc.header, &entries, tc.converter)
		response, err := client.RunCommand(tc.command, csvcon)
		if tc.failureExpected && err == nil {
			t.Errorf("TestClientRunBackendCommandConverter test case %s expected a failure but got none, response=%s",
				tc.name, string(response))
		}
		if !tc.failureExpected && err != nil {
			t.Errorf("TestClientRunBackendCommandConverter test case %s expected no failure but got one: %v", tc.name, err)
		}
	}
}

// TestClientRunMapCommandConverter tests client show map command execution with a converter.
func TestClientRunMapCommandConverter(t *testing.T) {
	testCases := []struct {
		name            string
		command         string
		header          string
		converter       ByteConverterFunc
		failureExpected bool
	}{
		{
			name:            "show map command",
			command:         "show map",
			header:          "id (file) description",
			converter:       fixupMapListOutput,
			failureExpected: false,
		},
		{
			name:            "show map command with no converter",
			command:         "show map",
			header:          "id (file) description",
			converter:       nil,
			failureExpected: true,
		},
	}

	server := haproxytesting.StartFakeServerForTest(t)
	defer server.Stop()

	for _, tc := range testCases {
		client := NewClient(server.SocketFile(), 1)
		entries := []*mapListEntry{}
		csvcon := NewCSVConverter(tc.header, &entries, tc.converter)
		response, err := client.RunCommand(tc.command, csvcon)
		if tc.failureExpected && err == nil {
			t.Errorf("TestClientRunMapCommandConverter test case %s expected a failure but got none, response=%s",
				tc.name, string(response))
		}
		if !tc.failureExpected && err != nil {
			t.Errorf("TestClientRunMapCommandConverter test case %s expected no failure but got one: %v", tc.name, err)
		}
	}
}

// TestClientRunServerCommandConverter tests client show servers state command execution with a converter.
func TestClientRunServerCommandConverter(t *testing.T) {
	testCases := []struct {
		name            string
		command         string
		header          string
		converter       ByteConverterFunc
		failureExpected bool
	}{
		{
			name:            "show servers state command",
			command:         "show servers state be_edge_http:default:example-route",
			header:          serversStateHeader,
			converter:       stripVersionNumber,
			failureExpected: false,
		},
		{
			name:            "show servers state command without a converter",
			command:         "show servers state be_edge_http:default:example-route",
			header:          serversStateHeader,
			converter:       nil,
			failureExpected: true,
		},
	}

	server := haproxytesting.StartFakeServerForTest(t)
	defer server.Stop()

	for _, tc := range testCases {
		client := NewClient(server.SocketFile(), 1)
		entries := []*serverStateInfo{}
		csvcon := NewCSVConverter(tc.header, &entries, tc.converter)
		response, err := client.RunCommand(tc.command, csvcon)
		if tc.failureExpected && err == nil {
			t.Errorf("TestClientRunServerCommandConverter test case %s expected a failure but got none, response=%s",
				tc.name, string(response))
		}
		if !tc.failureExpected && err != nil {
			t.Errorf("TestClientRunServerCommandConverter test case %s expected no failure but got one: %v", tc.name, err)
		}
	}
}

// TestClientExecute tests client command execution.
func TestClientExecute(t *testing.T) {
	testCases := []struct {
		name            string
		command         string
		failureExpected bool
	}{
		{
			name:            "info command",
			command:         "show info",
			failureExpected: false,
		},
		{
			name:            "bad command",
			command:         "bad command here",
			failureExpected: true,
		},
		{
			name:            "show backend command",
			command:         "show backend",
			failureExpected: false,
		},
		{
			name:            "show map command",
			command:         "show map",
			failureExpected: false,
		},
		{
			name:            "show servers state command",
			command:         "show servers state be_edge_http:default:example-route",
			failureExpected: false,
		},
		{
			name:            "set server ipaddr command",
			command:         "set server be_edge_http:default:example-route/_dynamic-pod-1 ipaddr 1.2.3.4",
			failureExpected: false,
		},
		{
			name:            "set server ipaddr and port command",
			command:         "set server be_edge_http:default:example-route/_dynamic-pod-1 ipaddr 1.2.3.4 port 8080",
			failureExpected: false,
		},
		{
			name:            "set server weight command",
			command:         "set server be_edge_http:default:example-route/_dynamic-pod-1 weight 256",
			failureExpected: false,
		},
		{
			name:            "set server state command",
			command:         "set server be_edge_http:default:example-route/_dynamic-pod-1 state maint",
			failureExpected: false,
		},
	}

	server := haproxytesting.StartFakeServerForTest(t)
	defer server.Stop()

	for _, tc := range testCases {
		client := NewClient(server.SocketFile(), 1)
		response, err := client.Execute(tc.command)
		if tc.failureExpected && err == nil {
			t.Errorf("TestClientExecute test case %s expected a failure but got none, response=%s",
				tc.name, string(response))
		}
		if !tc.failureExpected && err != nil {
			t.Errorf("TestClientExecute test case %s expected no failure but got one: %v", tc.name, err)
		}
	}
}

// TestClientReset tests client state reset.
func TestClientReset(t *testing.T) {
	server := haproxytesting.StartFakeServerForTest(t)
	defer server.Stop()

	client := NewClient(server.SocketFile(), 1)
	if _, err := client.Backends(); err != nil {
		t.Errorf("TestClientReset error getting backends: %v", err)
	}
	if _, err := client.Maps(); err != nil {
		t.Errorf("TestClientReset error getting maps: %v", err)
	}

	server.Reset()
	commands := server.Commands()
	if len(commands) != 0 {
		t.Errorf("TestClientReset error resetting server commands=%+v", commands)
	}

	client.Reset()
	if _, err := client.Backends(); err != nil {
		t.Errorf("TestClientReset error getting backends: %v", err)
	}
	commands = server.Commands()
	if len(commands) == 0 {
		t.Errorf("TestClientReset after reset no server command found, where one was expected")
	}

	server.Reset()
	commands = server.Commands()
	if len(commands) != 0 {
		t.Errorf("TestClientReset error resetting server commands=%+v", commands)
	}

	client.Reset()
	client.FindBackend("foo")
	commands = server.Commands()
	if len(commands) == 0 {
		t.Errorf("TestClientReset after reset no server command found, where one was expected")
	}
}

// TestClientCommit tests client state commit.
func TestClientCommit(t *testing.T) {
	server := haproxytesting.StartFakeServerForTest(t)
	defer server.Stop()

	client := NewClient(server.SocketFile(), 1)
	backends, err := client.Backends()
	if err != nil {
		t.Errorf("TestClientCommit error getting backends: %v", err)
	}
	maps, err := client.Maps()
	if err != nil {
		t.Errorf("TestClientCommit error getting maps: %v", err)
	}

	server.Reset()
	for _, m := range maps {
		m.Add("key", "value", true)
	}
	if len(server.Commands()) == 0 {
		t.Errorf("TestClientCommit no commands found after reset and adding to maps")
	}

	skipNames := map[string]bool{
		"be_sni":            true,
		"be_no_sni":         true,
		"openshift_default": true,
	}

	server.Reset()
	for _, be := range backends {
		if _, ok := skipNames[be.Name()]; ok {
			continue
		}

		serverName := "_dynamic-pod-1"
		if err := be.UpdateServerState(serverName, BackendServerStateMaint); err != nil {
			t.Errorf("TestClientCommit error setting state to maint for backend %s, server %s: %v",
				be.Name(), serverName, err)
		}
	}
	client.Commit()
	if len(server.Commands()) == 0 {
		t.Errorf("TestClientCommit no commands found after reset and commit")
	}

	server.Reset()
	for _, be := range backends {
		if _, ok := skipNames[be.Name()]; ok {
			continue
		}

		serverName := "invalid-pod-not-found-name"
		if err := be.UpdateServerState(serverName, BackendServerStateMaint); err == nil {
			t.Errorf("TestClientCommit error setting state to maint for backend %s, server %s, expected an error but got none",
				be.Name(), serverName)
		}
	}
	client.Commit()
	if len(server.Commands()) == 0 {
		t.Errorf("TestClientCommit no commands found after second reset and commit")
	}
}

// TestClientBackends tests client backends.
func TestClientBackends(t *testing.T) {
	server := haproxytesting.StartFakeServerForTest(t)
	defer server.Stop()

	client := NewClient(server.SocketFile(), 1)
	backends, err := client.Backends()
	if err != nil {
		t.Errorf("TestClientBackends error getting backends: %v", err)
	}
	if len(backends) == 0 {
		t.Errorf("TestClientBackends got no backends")
	}
}

// TestClientFindBackend tests client find a specific backend.
func TestClientFindBackend(t *testing.T) {
	testCases := []struct {
		name            string
		backendName     string
		failureExpected bool
	}{
		{
			name:            "non-existent backend",
			backendName:     "be_this_does_not_exist",
			failureExpected: true,
		},
		{
			name:            "existing backend",
			backendName:     "be_edge_http:default:example-route",
			failureExpected: false,
		},
		{
			name:            "existing http backend",
			backendName:     "be_edge_http:default:test-http-allow",
			failureExpected: false,
		},
		{
			name:            "existing edge backend",
			backendName:     "be_edge_http:default:test-https",
			failureExpected: false,
		},
		{
			name:            "existing passthrough backend",
			backendName:     "be_tcp:default:test-passthrough",
			failureExpected: false,
		},
		{
			name:            "existing reencrypt backend",
			backendName:     "be_secure:default:test-reencrypt",
			failureExpected: false,
		},
		{
			name:            "existing wildcard backend",
			backendName:     "be_edge_http:default:wildcard-redirect-to-https",
			failureExpected: false,
		},
		{
			name:            "bad backend name typo",
			backendName:     "be_secure:default:test-reencrypt-1234",
			failureExpected: true,
		},
	}

	server := haproxytesting.StartFakeServerForTest(t)
	defer server.Stop()

	for _, tc := range testCases {
		client := NewClient(server.SocketFile(), 1)
		backend, err := client.FindBackend(tc.backendName)
		if tc.failureExpected {
			if err == nil {
				t.Errorf("TestClientFindBackend test case %s expected an error and got none", tc.name)
			}
			if backend != nil {
				t.Errorf("TestClientFindBackend test case %s expected an error and got a valid backend: %+v", tc.name, backend)
			}
		} else {
			if err != nil {
				t.Errorf("TestClientFindBackend test case %s expected no error and got: %v", tc.name, err)
			}
			if backend == nil {
				t.Errorf("TestClientFindBackend test case %s expected a backend and got none", tc.name)
			}
		}
	}
}

// TestClientMaps tests client haproxy maps.
func TestClientMaps(t *testing.T) {
	server := haproxytesting.StartFakeServerForTest(t)
	defer server.Stop()

	client := NewClient(server.SocketFile(), 1)
	maps, err := client.Maps()
	if err != nil {
		t.Errorf("TestClientMaps error getting maps: %v", err)
	}
	if len(maps) == 0 {
		t.Errorf("TestClientMaps got no maps")
	}
}

// TestClientFindMap tests client find a specific haproxy map.
func TestClientFindMap(t *testing.T) {
	testCases := []struct {
		name            string
		mapName         string
		failureExpected bool
	}{
		{
			name:            "non-existent map",
			mapName:         "/a/b/c/d/e.map",
			failureExpected: true,
		},
		{
			name:            "existing redirect map",
			mapName:         "/var/lib/haproxy/conf/os_route_http_redirect.map",
			failureExpected: false,
		},
		{
			name:            "existing sni passthru map",
			mapName:         "/var/lib/haproxy/conf/os_sni_passthrough.map",
			failureExpected: false,
		},
		{
			name:            "existing http be map",
			mapName:         "/var/lib/haproxy/conf/os_http_be.map",
			failureExpected: false,
		},
		{
			name:            "existing tcp be map",
			mapName:         "/var/lib/haproxy/conf/os_tcp_be.map",
			failureExpected: false,
		},
		{
			name:            "existing edge and reencrypt map",
			mapName:         "/var/lib/haproxy/conf/os_edge_reencrypt_be.map",
			failureExpected: false,
		},
		{
			name:            "bad backend name typo",
			mapName:         "/var/lib/haproxy/conf/os_http_be.map.with.typo",
			failureExpected: true,
		},
	}

	server := haproxytesting.StartFakeServerForTest(t)
	defer server.Stop()

	for _, tc := range testCases {
		client := NewClient(server.SocketFile(), 1)
		haproxyMap, err := client.FindMap(tc.mapName)
		if tc.failureExpected {
			if err == nil {
				t.Errorf("TestClientFindMap test case %s expected an error and got none", tc.name)
			}
			if haproxyMap != nil {
				t.Errorf("TestClientFindMap test case %s expected an error and got a valid haproxy map: %+v", tc.name, haproxyMap)
			}
		} else {
			if err != nil {
				t.Errorf("TestClientFindMap test case %s expected no error and got: %v", tc.name, err)
			}
			if haproxyMap == nil {
				t.Errorf("TestClientFindMap test case %s expected a haproxy map and got none", tc.name)
			}
		}
	}
}

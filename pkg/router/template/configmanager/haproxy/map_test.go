package haproxy

import (
	"testing"

	haproxytesting "github.com/openshift/origin/pkg/router/template/configmanager/haproxy/testing"
)

// TestBuildHAProxyMaps tests haproxy maps.
func TestBuildHAProxyMaps(t *testing.T) {
	server := haproxytesting.StartFakeServerForTest(t)
	defer server.Stop()

	testCases := []struct {
		name            string
		sockFile        string
		failureExpected bool
	}{
		{
			name:            "empty socket",
			sockFile:        "",
			failureExpected: true,
		},
		{
			name:            "valid socket",
			sockFile:        server.SocketFile(),
			failureExpected: false,
		},
		{
			name:            "non-existent socket",
			sockFile:        "/non-existent/fake-haproxy.sock",
			failureExpected: true,
		},
	}

	for _, tc := range testCases {
		client := NewClient(tc.sockFile, 0)
		if client == nil {
			t.Errorf("TestBuildHAProxyMaps test case %s failed with no client.", tc.name)
		}

		haproxyMaps, err := buildHAProxyMaps(client)
		if tc.failureExpected {
			if err == nil {
				t.Errorf("TestBuildHAProxyMaps test case %s expected an error but got none.", tc.name)
			}
			continue
		}

		if err != nil {
			t.Errorf("TestBuildHAProxyMaps test case %s expected no error but got: %v", tc.name, err)
		}
		if len(haproxyMaps) == 0 {
			t.Errorf("TestBuildHAProxyMaps test case %s expected to get maps", tc.name)
		}
	}
}

// TestNewHAProxyMap tests a new haproxy map.
func TestNewHAProxyMap(t *testing.T) {
	server := haproxytesting.StartFakeServerForTest(t)
	defer server.Stop()

	testCases := []struct {
		name     string
		sockFile string
	}{
		{
			name:     "empty",
			sockFile: "",
		},
		{
			name:     "valid socket",
			sockFile: server.SocketFile(),
		},
		{
			name:     "non-existent socket",
			sockFile: "/non-existent/fake-haproxy.sock",
		},
	}

	for _, tc := range testCases {
		client := NewClient(tc.sockFile, 0)
		if client == nil {
			t.Errorf("TestNewHAProxyMap test case %s failed with no client.", tc.name)
		}

		if m := newHAProxyMap(tc.name, client); m == nil {
			t.Errorf("TestNewHAProxyMap test case %s expected a map but got none", tc.name)
		}
	}
}

// TestHAProxyMapRefresh tests haproxy map refresh.
func TestHAProxyMapRefresh(t *testing.T) {
	server := haproxytesting.StartFakeServerForTest(t)
	defer server.Stop()

	testCases := []struct {
		name            string
		sockFile        string
		mapName         string
		failureExpected bool
	}{
		{
			name:            "empty socket",
			sockFile:        "",
			mapName:         "empty.map",
			failureExpected: true,
		},
		{
			name:            "empty socket and valid map",
			sockFile:        "",
			mapName:         "/var/lib/haproxy/conf/os_sni_passthrough.map",
			failureExpected: true,
		},
		{
			name:            "valid socket and map",
			sockFile:        server.SocketFile(),
			mapName:         "/var/lib/haproxy/conf/os_http_be.map",
			failureExpected: false,
		},
		{
			name:            "valid socket but invalid map",
			sockFile:        server.SocketFile(),
			mapName:         "missing.map",
			failureExpected: true,
		},
		{
			name:            "valid socket but typo map",
			sockFile:        server.SocketFile(),
			mapName:         "/var/lib/haproxy/conf/os_http_be.map-1234",
			failureExpected: true,
		},
		{
			name:            "non-existent socket",
			sockFile:        "/non-existent/fake-haproxy.sock",
			mapName:         "non-existent.map",
			failureExpected: true,
		},
		{
			name:            "non-existent socket valid map",
			sockFile:        "/non-existent/fake-haproxy.sock",
			mapName:         "/var/lib/haproxy/conf/os_tcp_be.map",
			failureExpected: true,
		},
	}

	for _, tc := range testCases {
		client := NewClient(tc.sockFile, 0)
		if client == nil {
			t.Errorf("TestHAProxyMapRefresh test case %s failed with no client.", tc.name)
		}

		m := newHAProxyMap(tc.mapName, client)
		err := m.Refresh()
		if tc.failureExpected {
			if err == nil {
				t.Errorf("TestHAProxyMapRefresh test case %s expected an error but got none.", tc.name)
			}
			continue
		}

		if err != nil {
			t.Errorf("TestHAProxyMapRefresh test case %s expected no error but got: %v", tc.name, err)
		}
	}
}

// TestHAProxyMapCommit tests haproxy map commit.
func TestHAProxyMapCommit(t *testing.T) {
	server := haproxytesting.StartFakeServerForTest(t)
	defer server.Stop()

	testCases := []struct {
		name     string
		sockFile string
		mapName  string
	}{
		{
			name:     "empty socket",
			sockFile: "",
			mapName:  "empty.map",
		},
		{
			name:     "empty socket valid map",
			sockFile: "",
			mapName:  "/var/lib/haproxy/conf/os_sni_passthrough.map",
		},
		{
			name:     "valid socket",
			sockFile: server.SocketFile(),
			mapName:  "/var/lib/haproxy/conf/os_http_be.map",
		},
		{
			name:     "valid socket but invalid map",
			sockFile: server.SocketFile(),
			mapName:  "missing.map",
		},
		{
			name:     "valid socket but typo map",
			sockFile: server.SocketFile(),
			mapName:  "/var/lib/haproxy/conf/os_http_be.map-1234",
		},
		{
			name:     "non-existent socket",
			sockFile: "/non-existent/fake-haproxy.sock",
			mapName:  "non-existent.map",
		},
		{
			name:     "non-existent socket valid map",
			sockFile: "/non-existent/fake-haproxy.sock",
			mapName:  "/var/lib/haproxy/conf/os_tcp_be.map",
		},
	}

	for _, tc := range testCases {
		client := NewClient(tc.sockFile, 0)
		if client == nil {
			t.Errorf("TestHAProxyMapCommit test case %s failed with no client.", tc.name)
		}

		m := newHAProxyMap(tc.mapName, client)
		if err := m.Commit(); err != nil {
			t.Errorf("TestHAProxyMapCommit test case %s expected no error but got: %v", tc.name, err)
		}
	}
}

// TestHAProxyMapName tests haproxy map returns its name.
func TestHAProxyMapName(t *testing.T) {
	server := haproxytesting.StartFakeServerForTest(t)
	defer server.Stop()

	testCases := []struct {
		name            string
		sockFile        string
		mapName         string
		failureExpected bool
	}{
		{
			name:            "empty socket",
			sockFile:        "",
			mapName:         "empty.map",
			failureExpected: true,
		},
		{
			name:            "empty socket valid map",
			sockFile:        "",
			mapName:         "/var/lib/haproxy/conf/os_sni_passthrough.map",
			failureExpected: true,
		},
		{
			name:            "valid socket",
			sockFile:        server.SocketFile(),
			mapName:         "/var/lib/haproxy/conf/os_http_be.map",
			failureExpected: false,
		},
		{
			name:            "valid socket but invalid map",
			sockFile:        server.SocketFile(),
			mapName:         "missing.map",
			failureExpected: true,
		},
		{
			name:            "valid socket but typo map",
			sockFile:        server.SocketFile(),
			mapName:         "/var/lib/haproxy/conf/os_http_be.map-1234",
			failureExpected: true,
		},
		{
			name:            "non-existent socket",
			sockFile:        "/non-existent/fake-haproxy.sock",
			mapName:         "non-existent.map",
			failureExpected: true,
		},
		{
			name:            "non-existent socket valid map",
			sockFile:        "/non-existent/fake-haproxy.sock",
			mapName:         "/var/lib/haproxy/conf/os_tcp_be.map",
			failureExpected: true,
		},
	}

	for _, tc := range testCases {
		client := NewClient(tc.sockFile, 0)
		if client == nil {
			t.Errorf("TestHAProxyMapRefresh test case %s failed with no client.", tc.name)
		}

		m := newHAProxyMap(tc.mapName, client)
		err := m.Refresh()
		if tc.failureExpected {
			if err == nil {
				t.Errorf("TestHAProxyMapRefresh test case %s expected an error but got none.", tc.name)
			}
			continue
		}

		if err != nil {
			t.Errorf("TestHAProxyMapRefresh test case %s expected no error but got: %v", tc.name, err)
		}
	}
}

// TestHAProxyMapFind tests finding an entry in a haproxy map.
func TestHAProxyMapFind(t *testing.T) {
	server := haproxytesting.StartFakeServerForTest(t)
	defer server.Stop()

	testCases := []struct {
		name            string
		sockFile        string
		mapName         string
		keyName         string
		failureExpected bool
		entriesExpected bool
	}{
		{
			name:            "empty socket",
			sockFile:        "",
			mapName:         "empty.map",
			keyName:         "k1",
			failureExpected: true,
			entriesExpected: false,
		},
		{
			name:            "empty socket valid map and key",
			sockFile:        "",
			mapName:         "/var/lib/haproxy/conf/os_sni_passthrough.map",
			keyName:         `^route\.passthrough\.test(:[0-9]+)?(/.*)?$`,
			failureExpected: true,
			entriesExpected: false,
		},
		{
			name:            "empty socket valid map and invalid key",
			sockFile:        "",
			mapName:         "/var/lib/haproxy/conf/os_sni_passthrough.map",
			keyName:         "non-existent-key",
			failureExpected: true,
			entriesExpected: false,
		},
		{
			name:            "valid socket, map and key",
			sockFile:        server.SocketFile(),
			mapName:         "/var/lib/haproxy/conf/os_http_be.map",
			keyName:         `^route\.allow-http\.test(:[0-9]+)?(/.*)?$`,
			failureExpected: false,
			entriesExpected: true,
		},
		{
			name:            "valid socket but invalid map",
			sockFile:        server.SocketFile(),
			mapName:         "missing.map",
			keyName:         `^route\.allow-http\.test(:[0-9]+)?(/.*)?$`,
			failureExpected: true,
			entriesExpected: false,
		},
		{
			name:            "valid socket but invalid map and key",
			sockFile:        server.SocketFile(),
			mapName:         "missing.map",
			keyName:         "invalid-key",
			failureExpected: true,
			entriesExpected: false,
		},
		{
			name:            "valid socket but invalid key",
			sockFile:        server.SocketFile(),
			mapName:         "/var/lib/haproxy/conf/os_http_be.map",
			keyName:         "invalid-key",
			failureExpected: false,
			entriesExpected: false,
		},
		{
			name:            "valid socket but typo map",
			sockFile:        server.SocketFile(),
			mapName:         "/var/lib/haproxy/conf/os_http_be.map-1234",
			keyName:         `^route\.allow-http\.test(:[0-9]+)?(/.*)?$`,
			failureExpected: true,
			entriesExpected: false,
		},
		{
			name:            "non-existent socket",
			sockFile:        "/non-existent/fake-haproxy.sock",
			mapName:         "non-existent.map",
			keyName:         "invalid-key",
			failureExpected: true,
			entriesExpected: false,
		},
		{
			name:            "non-existent socket valid map",
			sockFile:        "/non-existent/fake-haproxy.sock",
			mapName:         "/var/lib/haproxy/conf/os_tcp_be.map",
			keyName:         "invalid-key",
			failureExpected: true,
			entriesExpected: false,
		},
		{
			name:            "non-existent socket invalid map",
			sockFile:        "/non-existent/fake-haproxy.sock",
			mapName:         "404.map",
			keyName:         `^reencrypt\.blueprints\.org(:[0-9]+)?(/.*)?$`,
			failureExpected: true,
			entriesExpected: false,
		},
		{
			name:            "non-existent socket valid map and key",
			sockFile:        "/non-existent/fake-haproxy.sock",
			mapName:         "/var/lib/haproxy/conf/os_tcp_be.map",
			keyName:         `^reencrypt\.blueprints\.org(:[0-9]+)?(/.*)?$`,
			failureExpected: true,
			entriesExpected: false,
		},
	}

	for _, tc := range testCases {
		client := NewClient(tc.sockFile, 0)
		if client == nil {
			t.Errorf("TestHAProxyMapFind test case %s failed with no client.", tc.name)
		}

		// Ensure server is in clean state for test.
		server.Reset()

		m := newHAProxyMap(tc.mapName, client)
		entries, err := m.Find(tc.keyName)
		if tc.failureExpected {
			if err == nil {
				t.Errorf("TestHAProxyMapFind test case %s expected an error but got none.", tc.name)
			}
			continue
		}

		if err != nil {
			t.Errorf("TestHAProxyMapFind test case %s expected no error but got: %v", tc.name, err)
		}
		if tc.entriesExpected && len(entries) < 1 {
			t.Errorf("TestHAProxyMapFind test case %s expected to find an entry but got: %v", tc.name, len(entries))
		}
	}
}

// TestHAProxyMapAdd tests adding an entry in a haproxy map.
func TestHAProxyMapAdd(t *testing.T) {
	server := haproxytesting.StartFakeServerForTest(t)
	defer server.Stop()

	testCases := []struct {
		name            string
		sockFile        string
		mapName         string
		keyName         string
		value           string
		replace         bool
		failureExpected bool
	}{
		{
			name:            "empty socket and map",
			sockFile:        "",
			mapName:         "empty.map",
			keyName:         "k1",
			value:           "v1",
			replace:         true,
			failureExpected: true,
		},
		{
			name:            "empty socket valid map and key",
			sockFile:        "",
			mapName:         "/var/lib/haproxy/conf/os_sni_passthrough.map",
			keyName:         `^route\.passthrough\.test(:[0-9]+)?(/.*)?$`,
			value:           "1",
			replace:         true,
			failureExpected: true,
		},
		{
			name:            "empty socket valid map and invalid key",
			sockFile:        "",
			mapName:         "/var/lib/haproxy/conf/os_sni_passthrough.map",
			keyName:         "non-existent-key",
			value:           "something",
			replace:         false,
			failureExpected: true,
		},
		{
			name:            "valid socket",
			sockFile:        server.SocketFile(),
			mapName:         "/var/lib/haproxy/conf/os_http_be.map",
			keyName:         `^route\.allow-http\.test(:[0-9]+)?(/.*)?$`,
			value:           "be_edge_http:default:test-http-allow",
			replace:         true,
			failureExpected: false,
		},
		{
			name:            "valid socket no replace",
			sockFile:        server.SocketFile(),
			mapName:         "/var/lib/haproxy/conf/os_http_be.map",
			keyName:         `^route\.allow-http\.test(:[0-9]+)?(/.*)?$`,
			value:           "be_edge_http:default:test-http-allow",
			replace:         false,
			failureExpected: false,
		},
		{
			name:            "valid socket but invalid map",
			sockFile:        server.SocketFile(),
			mapName:         "missing.map",
			keyName:         `^route\.allow-http\.test(:[0-9]+)?(/.*)?$`,
			value:           "be_edge_http:default:test-http-allow",
			replace:         true,
			failureExpected: true,
		},
		{
			name:            "valid socket but invalid map and key",
			sockFile:        server.SocketFile(),
			mapName:         "missing.map",
			keyName:         "invalid-key1",
			value:           "something",
			replace:         false,
			failureExpected: true,
		},
		{
			name:            "valid socket but invalid key",
			sockFile:        server.SocketFile(),
			mapName:         "/var/lib/haproxy/conf/os_http_be.map",
			keyName:         "invalid-key2",
			value:           "something",
			replace:         true,
			failureExpected: false,
		},
		{
			name:            "valid socket but typo map",
			sockFile:        server.SocketFile(),
			mapName:         "/var/lib/haproxy/conf/os_http_be.map-1234",
			keyName:         `^route\.allow-http\.test(:[0-9]+)?(/.*)?$`,
			value:           "be_edge_http:default:test-http-allow",
			replace:         true,
			failureExpected: true,
		},
		{
			name:            "non-existent socket",
			sockFile:        "/non-existent/fake-haproxy.sock",
			mapName:         "non-existent.map",
			keyName:         "invalid-key3",
			value:           "some-value",
			replace:         false,
			failureExpected: true,
		},
		{
			name:            "non-existent socket valid map",
			sockFile:        "/non-existent/fake-haproxy.sock",
			mapName:         "/var/lib/haproxy/conf/os_tcp_be.map",
			keyName:         "invalid-key4",
			value:           "some-value",
			replace:         true,
			failureExpected: true,
		},
		{
			name:            "non-existent socket invalid map",
			sockFile:        "/non-existent/fake-haproxy.sock",
			mapName:         "404.map",
			keyName:         `^reencrypt\.blueprints\.org(:[0-9]+)?(/.*)?$`,
			value:           "be_secure:blueprints:blueprint-reencrypt",
			replace:         true,
			failureExpected: true,
		},
		{
			name:            "non-existent socket valid map and key",
			sockFile:        "/non-existent/fake-haproxy.sock",
			mapName:         "/var/lib/haproxy/conf/os_tcp_be.map",
			keyName:         `^reencrypt\.blueprints\.org(:[0-9]+)?(/.*)?$`,
			value:           "1234",
			replace:         false,
			failureExpected: true,
		},
	}

	for _, tc := range testCases {
		client := NewClient(tc.sockFile, 0)
		if client == nil {
			t.Errorf("TestHAProxyMapAdd test case %s failed with no client.", tc.name)
		}

		// Ensure server is in clean state for test.
		server.Reset()

		m := newHAProxyMap(tc.mapName, client)
		err := m.Add(tc.keyName, tc.value, tc.replace)
		if tc.failureExpected {
			if err == nil {
				t.Errorf("TestHAProxyMapAdd test case %s expected an error but got none.", tc.name)
			}
			continue
		}

		if err != nil {
			t.Errorf("TestHAProxyMapAdd test case %s expected no error but got: %v", tc.name, err)
		}
	}
}

// TestHAProxyMapDelete tests deleting entries in a haproxy map.
func TestHAProxyMapDelete(t *testing.T) {
	server := haproxytesting.StartFakeServerForTest(t)
	defer server.Stop()

	testCases := []struct {
		name            string
		sockFile        string
		mapName         string
		keyName         string
		failureExpected bool
	}{
		{
			name:            "empty socket and map",
			sockFile:        "",
			mapName:         "empty.map",
			keyName:         "k1",
			failureExpected: true,
		},
		{
			name:            "empty socket valid map and key",
			sockFile:        "",
			mapName:         "/var/lib/haproxy/conf/os_sni_passthrough.map",
			keyName:         `^route\.passthrough\.test(:[0-9]+)?(/.*)?$`,
			failureExpected: true,
		},
		{
			name:            "empty socket valid map and invalid key",
			sockFile:        "",
			mapName:         "/var/lib/haproxy/conf/os_sni_passthrough.map",
			keyName:         "non-existent-key",
			failureExpected: true,
		},
		{
			name:            "valid socket",
			sockFile:        server.SocketFile(),
			mapName:         "/var/lib/haproxy/conf/os_http_be.map",
			keyName:         `^route\.allow-http\.test(:[0-9]+)?(/.*)?$`,
			failureExpected: false,
		},
		{
			name:            "valid socket but invalid map",
			sockFile:        server.SocketFile(),
			mapName:         "missing.map",
			keyName:         `^route\.allow-http\.test(:[0-9]+)?(/.*)?$`,
			failureExpected: true,
		},
		{
			name:            "valid socket but invalid map and key",
			sockFile:        server.SocketFile(),
			mapName:         "missing.map",
			keyName:         "invalid-key1",
			failureExpected: true,
		},
		{
			name:            "valid socket but invalid key",
			sockFile:        server.SocketFile(),
			mapName:         "/var/lib/haproxy/conf/os_http_be.map",
			keyName:         "invalid-key2",
			failureExpected: false,
		},
		{
			name:            "valid socket but typo map",
			sockFile:        server.SocketFile(),
			mapName:         "/var/lib/haproxy/conf/os_http_be.map-1234",
			keyName:         `^route\.allow-http\.test(:[0-9]+)?(/.*)?$`,
			failureExpected: true,
		},
		{
			name:            "non-existent socket",
			sockFile:        "/non-existent/fake-haproxy.sock",
			mapName:         "non-existent.map",
			keyName:         "invalid-key3",
			failureExpected: true,
		},
		{
			name:            "non-existent socket valid map",
			sockFile:        "/non-existent/fake-haproxy.sock",
			mapName:         "/var/lib/haproxy/conf/os_tcp_be.map",
			keyName:         "invalid-key4",
			failureExpected: true,
		},
		{
			name:            "non-existent socket invalid map",
			sockFile:        "/non-existent/fake-haproxy.sock",
			mapName:         "404.map",
			keyName:         `^reencrypt\.blueprints\.org(:[0-9]+)?(/.*)?$`,
			failureExpected: true,
		},
		{
			name:            "non-existent socket valid map and key",
			sockFile:        "/non-existent/fake-haproxy.sock",
			mapName:         "/var/lib/haproxy/conf/os_tcp_be.map",
			keyName:         `^reencrypt\.blueprints\.org(:[0-9]+)?(/.*)?$`,
			failureExpected: true,
		},
	}

	for _, tc := range testCases {
		client := NewClient(tc.sockFile, 0)
		if client == nil {
			t.Errorf("TestHAProxyMapDelete test case %s failed with no client.", tc.name)
		}

		// Ensure server is in clean state for test.
		server.Reset()

		m := newHAProxyMap(tc.mapName, client)
		err := m.Delete(tc.keyName)
		if tc.failureExpected {
			if err == nil {
				t.Errorf("TestHAProxyMapDelete test case %s expected an error but got none.", tc.name)
			}
			continue
		}

		if err != nil {
			t.Errorf("TestHAProxyMapDelete test case %s expected no error but got: %v", tc.name, err)
		}
	}
}

// TestHAProxyMapDeleteEntry tests deleting an entry in a haproxy map.
func TestHAProxyMapDeleteEntry(t *testing.T) {
	server := haproxytesting.StartFakeServerForTest(t)
	defer server.Stop()

	testCases := []struct {
		name            string
		sockFile        string
		mapName         string
		entryID         string
		failureExpected bool
	}{
		{
			name:            "empty socket and map",
			sockFile:        "",
			mapName:         "empty.map",
			entryID:         "id1",
			failureExpected: true,
		},
		{
			name:            "empty socket valid map and key",
			sockFile:        "",
			mapName:         "/var/lib/haproxy/conf/os_sni_passthrough.map",
			entryID:         "0x559a137bf730",
			failureExpected: true,
		},
		{
			name:            "empty socket valid map and invalid key",
			sockFile:        "",
			mapName:         "/var/lib/haproxy/conf/os_sni_passthrough.map",
			entryID:         "non-existent-id",
			failureExpected: true,
		},
		{
			name:            "valid socket",
			sockFile:        server.SocketFile(),
			mapName:         "/var/lib/haproxy/conf/os_http_be.map",
			entryID:         "0x559a137b4c10",
			failureExpected: false,
		},
		{
			name:            "valid socket but invalid map",
			sockFile:        server.SocketFile(),
			mapName:         "missing.map",
			entryID:         "0x559a137b4c10",
			failureExpected: false,
		},
		{
			name:            "valid socket but invalid map and key",
			sockFile:        server.SocketFile(),
			mapName:         "missing.map",
			entryID:         "invalid-id",
			failureExpected: false,
		},
		{
			name:            "valid socket but invalid key",
			sockFile:        server.SocketFile(),
			mapName:         "/var/lib/haproxy/conf/os_http_be.map",
			entryID:         "invalid-id",
			failureExpected: false,
		},
		{
			name:            "valid socket but typo map",
			sockFile:        server.SocketFile(),
			mapName:         "/var/lib/haproxy/conf/os_http_be.map-1234",
			entryID:         "0x559a137b4c10",
			failureExpected: false,
		},
		{
			name:            "non-existent socket",
			sockFile:        "/non-existent/fake-haproxy.sock",
			mapName:         "non-existent.map",
			entryID:         "invalid-id3",
			failureExpected: true,
		},
		{
			name:            "non-existent socket valid map",
			sockFile:        "/non-existent/fake-haproxy.sock",
			mapName:         "/var/lib/haproxy/conf/os_tcp_be.map",
			entryID:         "invalid-id",
			failureExpected: true,
		},
		{
			name:            "non-existent socket invalid map",
			sockFile:        "/non-existent/fake-haproxy.sock",
			mapName:         "404.map",
			entryID:         "0x559a1400f8a0",
			failureExpected: true,
		},
		{
			name:            "non-existent socket valid map and key",
			sockFile:        "/non-existent/fake-haproxy.sock",
			mapName:         "/var/lib/haproxy/conf/os_tcp_be.map",
			entryID:         "0x559a1400f8a0",
			failureExpected: true,
		},
	}

	for _, tc := range testCases {
		client := NewClient(tc.sockFile, 0)
		if client == nil {
			t.Errorf("TestHAProxyMapDeleteEntry test case %s failed with no client.", tc.name)
		}

		// Ensure server is in clean state for test.
		server.Reset()

		m := newHAProxyMap(tc.mapName, client)
		err := m.DeleteEntry(tc.entryID)
		if tc.failureExpected {
			if err == nil {
				t.Errorf("TestHAProxyMapDeleteEntry test case %s expected an error but got none.", tc.name)
			}
			continue
		}

		if err != nil {
			t.Errorf("TestHAProxyMapDeleteEntry test case %s expected no error but got: %v", tc.name, err)
		}
	}
}

package cluster

import (
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	api "k8s.io/kubernetes/pkg/apis/core"
)

// Used as a stub for golang's net.LookupIP to avoid actual DNS lookups in tests.
func dummyDnsResolver(hostname string) ([]string, error) {
	// This method should not end up getting called for actual IPs, so we only
	// need to map out the hostnames we'll be "resolving":
	domains := map[string][]string{
		"something.example.com":    {"192.168.1.1"},
		"multiaddress.example.com": {"192.168.1.2", "10.0.1.1"},
		"localhost":                {"127.0.0.1"},
	}
	ip, ok := domains[hostname]
	if !ok {
		return nil, errors.New("Dummy DNS lookup error")
	}

	return ip, nil
}

func ipTester(t *testing.T, serverUrl string, expectedIps ...string) {
	hostnames, err := resolveServerIP(serverUrl, dummyDnsResolver)

	// If given no expected IPs we assume the caller wanted an error to occur:
	if len(expectedIps) == 0 || expectedIps == nil {
		if err == nil {
			t.Errorf("No expected IPs given, error expected but none occurred. Got hostnames: %s", hostnames)
			return
		}
		return
	}

	if err != nil || len(hostnames) == 0 {
		t.Errorf("Unable to resolve IP from: %s, error: %s", serverUrl, err)
		return
	}
	if len(hostnames) != len(expectedIps) {
		t.Errorf("Expected %d IPs, got: %d", len(expectedIps), len(hostnames))
		return
	}
	for _, expectedIp := range expectedIps {
		found := false
		for _, hostResult := range hostnames {
			if hostResult == expectedIp {
				found = true
			}
		}

		if !found {
			t.Errorf("Missing expected IP: %s, got: %s", expectedIp, hostnames)
			return
		}
	}
}

func TestServerResolve(t *testing.T) {
	ipTester(t, "https://something.example.com:8443/api/", "192.168.1.1")
}

func TestServerResolveFails(t *testing.T) {
	// Our dummy DNS resolver function doesn't know this host:
	ipTester(t, "https://somethingelse.example.com:8443/api/")
}

func TestServerResolveMultiAddress(t *testing.T) {
	ipTester(t, "https://multiaddress.example.com:8443/api/", "192.168.1.2", "10.0.1.1")
}

func TestServerResolveNoPort(t *testing.T) {
	ipTester(t, "https://something.example.com/api/", "192.168.1.1")
}

func TestServerResolveNoPortNoPath(t *testing.T) {
	ipTester(t, "https://something.example.com", "192.168.1.1")
}

func TestServerResolveNoPath(t *testing.T) {
	ipTester(t, "https://something.example.com:8443", "192.168.1.1")
}

func TestServerResolveLocalhost(t *testing.T) {
	ipTester(t, "https://localhost:8443/api/", "127.0.0.1")
}

func TestServerResolveIP(t *testing.T) {
	ipTester(t, "https://192.168.1.1:8443/api/", "192.168.1.1")
}

func TestServerResolveIPNoPort(t *testing.T) {
	ipTester(t, "https://192.168.1.1/api/", "192.168.1.1")
}

func TestServerResolveIPNoPortNoPath(t *testing.T) {
	ipTester(t, "https://192.168.1.1", "192.168.1.1")
}

func TestServerResolveIPNoPath(t *testing.T) {
	ipTester(t, "https://192.168.1.1:8443", "192.168.1.1")
}

func TestServerResolveIPv6(t *testing.T) {
	ipTester(t, "https://[2001:4860:0:2001::6]:8443/api/", "2001:4860:0:2001::6")
}

func TestServerResolveIPv6UpperCaseFull(t *testing.T) {
	// net.IP normalizes to lowercase and shortens:
	ipTester(t, "https://[FE80:0000:0000:0000:0202:B3FF:FE1E:8329]:8443/api/",
		"fe80::202:b3ff:fe1e:8329")
}

func TestServerResolveIPv6NoPort(t *testing.T) {
	ipTester(t, "https://2001:4860:0:2001::6/api/", "2001:4860:0:2001::6")
}

func TestServerResolveIPv6BracesNoPort(t *testing.T) {
	// Technically bad syntax so expect an error:
	ipTester(t, "https://[2001:4860:0:2001::6]/api/")
}

func TestServerResolveIPv6NoPortNoPath(t *testing.T) {
	ipTester(t, "https://2001:4860:0:2001::6", "2001:4860:0:2001::6")
}

func TestServerResolveIPv6NoPath(t *testing.T) {
	ipTester(t, "https://[2001:4860:0:2001::6]:8443", "2001:4860:0:2001::6")
}

func TestServerResolveIPv6BadBraces(t *testing.T) {
	ipTester(t, "https://[[2001:4860:0:2001::6]:8443")
	ipTester(t, "https://[[2001:4860:0:2001::6]]:8443")
	ipTester(t, "https://2001:4860:0:2001::6]]:8443")
}

func TestServerResolveBadURL(t *testing.T) {
	ipTester(t, "thisdoesntlooklikeaurl")
}

// createNode creates a dummy Kubernetes Node object with the IP addresses we request.
func createNode(name string, ipAddresses []string) api.Node {

	// Create a Kube NodeAddress for each given IP address string:
	addresses := make([]api.NodeAddress, len(ipAddresses))
	for _, addr := range ipAddresses {
		// We don't really care what the type is, we check them all looking for any match:
		addresses = append(addresses, api.NodeAddress{Type: api.NodeExternalIP, Address: addr})
	}

	status := api.NodeStatus{Addresses: addresses}
	node := api.Node{ObjectMeta: metav1.ObjectMeta{Name: name}, Status: status}
	return node
}

func TestScanNodesMatchFound(t *testing.T) {
	nodes := make([]api.Node, 3)
	nodes = append(nodes, createNode("node1", []string{"192.168.1.1", "10.0.0.1", "24.222.0.1"}))
	nodes = append(nodes, createNode("node2", []string{"192.168.1.2", "10.0.0.2", "24.222.0.2"}))
	nodes = append(nodes, createNode("node3", []string{"192.168.1.3", "10.0.0.3", "24.222.0.3"}))

	r := searchNodesForIP(nodes, []string{"24.222.0.3"})
	if len(r.Errors()) > 0 {
		t.Error("Unexpected error attempting to locate node with IP")
	}
	if len(r.Warnings()) > 0 {
		t.Error("Unexpected warning attempting to locate node with IP")
	}
}

func TestScanNodesAnyIPv6MatchFound(t *testing.T) {
	nodes := make([]api.Node, 3)
	nodes = append(nodes, createNode("node1", []string{"2001:4860:0:2001::6"}))
	nodes = append(nodes, createNode("node2", []string{"3001:4860:0:2001::6"}))
	nodes = append(nodes, createNode("node3", []string{"4001:4860:0:2001::6"}))

	// First server IP won't match, second will:
	r := searchNodesForIP(nodes, []string{"10.0.55.55", "2001:4860:0:2001::6"})
	if len(r.Errors()) > 0 {
		t.Error("Unexpected error attempting to locate node with IP")
	}
	if len(r.Warnings()) > 0 {
		t.Error("Unexpected warning attempting to locate node with IP")
	}
}

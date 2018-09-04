package egress_dns_proxy_test

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

func TestHAProxyFrontendBackendConf(t *testing.T) {
	tests := []struct {
		dest      string
		frontends []string
		backends  []string
		dnsMap    map[string]int
	}{
		// Single destination IP
		{
			dest: "80 11.12.13.14",
			frontends: []string{`
frontend fe1
    bind :80
    default_backend be1`},
			backends: []string{`
backend be1
    server dest1 11.12.13.14:80 check`},
		},
		// Multiple destination IPs
		{
			dest: "80 11.12.13.14\n8080 21.22.23.24 100",
			frontends: []string{`
frontend fe1
    bind :80
    default_backend be1`, `
frontend fe2
    bind :8080
    default_backend be2`},
			backends: []string{`
backend be1
    server dest1 11.12.13.14:80 check`, `
backend be2
    server dest1 21.22.23.24:100 check`},
		},
		// Single destination domain name
		{
			dest: "80 example.com",
			frontends: []string{`
frontend fe1
    bind :80
    default_backend be1`},
			backends: []string{`
backend be1
    server dest1 example.com:80 check resolvers dns-resolver`},
			dnsMap: map[string]int{
				"example.com": 1,
			},
		},
		// Multiple destination domain names
		{
			dest: "80 example.com\n8080 foo.com 100",
			frontends: []string{`
frontend fe1
    bind :80
    default_backend be1`, `
frontend fe2
    bind :8080
    default_backend be2`},
			backends: []string{`
backend be1
    server dest1 example.com:80 check resolvers dns-resolver`, `
backend be2
    server dest1 foo.com:100 check resolvers dns-resolver`},
			dnsMap: map[string]int{
				"example.com": 1,
				"foo.com":     1,
			},
		},
		// Destination IP and destination domain name
		{
			dest: "80 11.12.13.14\n8080 example.com 100",
			frontends: []string{`
frontend fe1
    bind :80
    default_backend be1`, `
frontend fe2
    bind :8080
    default_backend be2`},
			backends: []string{`
backend be1
    server dest1 11.12.13.14:80 check`, `
backend be2
    server dest1 example.com:100 check resolvers dns-resolver`},
			dnsMap: map[string]int{
				"example.com": 1,
			},
		},
		// Destination with comments and blank lines
		{
			dest: `
# My DNS proxy egress router rules

# Port 80 forwards to 11.12.13.14
80 11.12.13.14

# Port 8080 forwards to port 100 on example.com
8080 example.com 100

# Skip this rule for now
# 9000 foo.com 200

# End
`,
			frontends: []string{`
frontend fe1
    bind :80
    default_backend be1`, `
frontend fe2
    bind :8080
    default_backend be2`},
			backends: []string{`
backend be1
    server dest1 11.12.13.14:80 check`, `
backend be2
    server dest1 example.com:100 check resolvers dns-resolver`},
			dnsMap: map[string]int{
				"example.com": 1,
			},
		},
		// Destination domain name with multiple backends
		{
			dest: `
# Port 8080 forwards to port 100 on example.com
8080 example.com 100
`,
			frontends: []string{`
frontend fe1
    bind :8080
    default_backend be1`},
			backends: []string{`
backend be1
    server-template dest 3 example.com:100 check resolvers dns-resolver`},
			dnsMap: map[string]int{
				"example.com": 3,
			},
		},
		// Destination IP and Destination domain name with single and multiple backends
		{
			dest: `
80 11.12.13.14
9000 foo.com 200
8080 example.com 100
8081 bar.com 100
`,
			frontends: []string{`
frontend fe1
    bind :80
    default_backend be1`, `
frontend fe2
    bind :9000
    default_backend be2`, `
frontend fe3
    bind :8080
    default_backend be3`, `
frontend fe4
    bind :8081
    default_backend be4`},
			backends: []string{`
backend be1
    server dest1 11.12.13.14:80 check`, `
backend be2
    server-template dest 2 foo.com:200 check resolvers dns-resolver`, `
backend be3
    server dest1 example.com:100 check resolvers dns-resolver`, `
backend be4
    server dest1 bar.com:100 check resolvers dns-resolver`},
			dnsMap: map[string]int{
				"foo.com":     2,
				"example.com": 1,
				"bar.com":     1,
			},
		},
	}

	frontendRegex := regexp.MustCompile("\nfrontend ")
	backendRegex := regexp.MustCompile("\nbackend ")

	for n, test := range tests {
		servers := ""
		for domain, numServers := range test.dnsMap {
			servers += fmt.Sprintf("%s:%d;", domain, numServers)
		}

		cmd := exec.Command("./egress-dns-proxy.sh")
		cmd.Env = []string{
			fmt.Sprintf("EGRESS_DNS_PROXY_DESTINATION=%s", test.dest),
			fmt.Sprintf("EGRESS_DNS_PROXY_MODE=unit-test"),
			fmt.Sprintf("EGRESS_DNS_SERVERS=%s", servers),
		}
		outBytes, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("test %d unexpected error %v, output: %q", n+1, err, string(outBytes))
		}
		out := string(outBytes)
		for _, frontend := range test.frontends {
			if !strings.Contains(out, frontend) {
				t.Fatalf("test %d expected frontend in output %q but got %q", n+1, frontend, out)
			}
		}
		matches := frontendRegex.FindAllStringIndex(out, -1)
		if len(matches) != len(test.frontends) {
			t.Fatalf("test %d number of frontends mismatch, expected %q but got %q", n+1, test.frontends, out)
		}

		for _, backend := range test.backends {
			if !strings.Contains(out, backend) {
				t.Fatalf("test %d expected backend in output %q but got %q", n+1, backend, out)
			}
		}
		matches = backendRegex.FindAllStringIndex(out, -1)
		if len(matches) != len(test.backends) {
			t.Fatalf("test %d number of backends mismatch, expected %q but got %q", n+1, test.backends, out)
		}
	}
}

func TestHAProxyFrontendBackendConfBad(t *testing.T) {
	tests := []struct {
		dest string
		err  string
	}{
		{
			dest: "",
			err:  "Must specify EGRESS_DNS_PROXY_DESTINATION",
		},
		{
			dest: "80 11.12.13.14\ninvalid",
			err:  "Bad destination 'invalid'",
		},
		{
			dest: "80 11.12.13.14\n8080 invalid",
			err:  "Bad destination '8080 invalid'",
		},
		{
			dest: "99999 11.12.13.14",
			err:  "Invalid port: 99999, must be in the range 1 to 65535",
		},
		{
			dest: "80 11.12.13.14 88888",
			err:  "Invalid port: 88888, must be in the range 1 to 65535",
		},
		{
			dest: "80 11.12.13.14\n80 21.22.23.24 100",
			err:  "Proxy port 80 already used, must be unique for each destination",
		},
	}

	for n, test := range tests {
		cmd := exec.Command("./egress-dns-proxy.sh")
		cmd.Env = []string{
			"EGRESS_DNS_PROXY_MODE=unit-test",
			fmt.Sprintf("EGRESS_DNS_PROXY_DESTINATION=%s", test.dest),
		}
		out, err := cmd.CombinedOutput()
		out_lines := strings.Split(string(out), "\n")
		got := out_lines[len(out_lines)-2]
		if err == nil {
			t.Fatalf("test %d expected error %q but got output %q", n+1, test.err, got)
		}
		if got != test.err {
			t.Fatalf("test %d expected output %q but got %q", n+1, test.err, got)
		}
	}
}

func TestHAProxyDefaults(t *testing.T) {
	defaults := `
global
    log         127.0.0.1 local2

    chroot      /var/lib/haproxy
    pidfile     /var/lib/haproxy/run/haproxy.pid
    maxconn     4000
    user        haproxy
    group       haproxy

defaults
    log                     global
    mode                    tcp
    option                  dontlognull
    option                  tcplog
    option                  redispatch
    retries                 3
    timeout http-request    100s
    timeout queue           1m
    timeout connect         10s
    timeout client          1m
    timeout server          1m
    timeout http-keep-alive 100s
    timeout check           10s
`
	cmd := exec.Command("./egress-dns-proxy.sh")
	cmd.Env = []string{
		"EGRESS_DNS_PROXY_MODE=unit-test",
		"EGRESS_DNS_PROXY_DESTINATION=80 11.12.13.14",
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if !strings.Contains(string(out), defaults) {
		t.Fatalf("expected defaults in output %q but got %q", defaults, string(out))
	}
}

func TestHAProxyResolver(t *testing.T) {
	resolverRegex := "resolvers dns-resolver\n *(nameserver ns.*)+\n +"

	cmd := exec.Command("./egress-dns-proxy.sh")
	cmd.Env = []string{
		"EGRESS_DNS_PROXY_MODE=unit-test",
		"EGRESS_DNS_PROXY_DESTINATION=80 11.12.13.14",
	}
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	out := string(outBytes)
	match, er := regexp.MatchString(resolverRegex, out)
	if !match || er != nil {
		t.Fatalf("dns resolver section not found in output %q", out)
	}
}

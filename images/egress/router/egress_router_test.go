package egress_router_test

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestEgressRouter(t *testing.T) {
	tests := []struct {
		source  string
		gateway string
		dest    string
		output  string
	}{
		{
			source:  "1.2.3.4",
			gateway: "1.1.1.1",
			dest:    "10.1.2.3",
			output: `
-A PREROUTING -i eth0 -j DNAT --to-destination 10.1.2.3
-A POSTROUTING -j SNAT --to-source 1.2.3.4
`,
		},
		{
			source:  "1.2.3.4/24",
			gateway: "1.1.1.1",
			dest:    "10.1.2.3",
			output: `
-A PREROUTING -i eth0 -j DNAT --to-destination 10.1.2.3
-A POSTROUTING -j SNAT --to-source 1.2.3.4
`,
		},
		{
			source:  "1.2.3.4",
			gateway: "1.1.1.1",
			dest:    "10.1.2.3",
			output: `
-A PREROUTING -i eth0 -j DNAT --to-destination 10.1.2.3
-A POSTROUTING -j SNAT --to-source 1.2.3.4
`,
		},
		{
			source:  "1.2.3.4",
			gateway: "1.1.1.1",
			dest:    "10.1.2.3\n",
			output: `
-A PREROUTING -i eth0 -j DNAT --to-destination 10.1.2.3
-A POSTROUTING -j SNAT --to-source 1.2.3.4
`,
		},
		{
			source:  "1.2.3.4",
			gateway: "1.1.1.1",
			dest:    "80 tcp 10.4.5.6",
			output: `
-A PREROUTING -i eth0 -p tcp --dport 80 -j DNAT --to-destination 10.4.5.6
-A POSTROUTING -j SNAT --to-source 1.2.3.4
`,
		},
		{
			source:  "1.2.3.4",
			gateway: "1.1.1.1",
			dest:    "8080 tcp 10.7.8.9 80",
			output: `
-A PREROUTING -i eth0 -p tcp --dport 8080 -j DNAT --to-destination 10.7.8.9:80
-A POSTROUTING -j SNAT --to-source 1.2.3.4
`,
		},
		{
			source:  "1.2.3.4",
			gateway: "1.1.1.1",
			dest:    "80 tcp 10.4.5.6\n8080 tcp 10.7.8.9 80",
			output: `
-A PREROUTING -i eth0 -p tcp --dport 80 -j DNAT --to-destination 10.4.5.6
-A PREROUTING -i eth0 -p tcp --dport 8080 -j DNAT --to-destination 10.7.8.9:80
-A POSTROUTING -j SNAT --to-source 1.2.3.4
`,
		},
		{
			source:  "1.2.3.4",
			gateway: "1.1.1.1",
			dest:    "80 tcp 10.4.5.6\n8080 tcp 10.7.8.9 80\n10.1.2.3",
			output: `
-A PREROUTING -i eth0 -p tcp --dport 80 -j DNAT --to-destination 10.4.5.6
-A PREROUTING -i eth0 -p tcp --dport 8080 -j DNAT --to-destination 10.7.8.9:80
-A PREROUTING -i eth0 -j DNAT --to-destination 10.1.2.3
-A POSTROUTING -j SNAT --to-source 1.2.3.4
`,
		},
		{
			source:  "1.2.3.4",
			gateway: "1.1.1.1",
			dest: `
# My egress-router rules

# Port 80 forwards to 10.4.5.6
80 tcp 10.4.5.6

# Port 8080 forwards to port 80 on 10.7.8.9
8080 tcp 10.7.8.9 80

# Skip this rule for now
# 8443 tcp 10.7.8.9 443

# Add new rules here

# Fallback; don't add anything after this
10.1.2.3

# No, seriously, don't add anything here
`,
			output: `
-A PREROUTING -i eth0 -p tcp --dport 80 -j DNAT --to-destination 10.4.5.6
-A PREROUTING -i eth0 -p tcp --dport 8080 -j DNAT --to-destination 10.7.8.9:80
-A PREROUTING -i eth0 -j DNAT --to-destination 10.1.2.3
-A POSTROUTING -j SNAT --to-source 1.2.3.4
`,
		},
	}

	for n, test := range tests {
		cmd := exec.Command("./egress-router.sh")
		cmd.Env = []string{
			"EGRESS_ROUTER_MODE=unit-test",
			fmt.Sprintf("EGRESS_SOURCE=%s", test.source),
			fmt.Sprintf("EGRESS_GATEWAY=%s", test.gateway),
			fmt.Sprintf("EGRESS_DESTINATION=%s", test.dest),
		}
		out, err := cmd.CombinedOutput()
		expected := test.output[1:]
		if err != nil {
			t.Fatalf("test %d expected output %q but got error %v", n+1, expected, err)
		}
		if string(out) != expected {
			t.Fatalf("test %d expected output %q but got %q", n+1, expected, string(out))
		}
	}
}

func TestEgressRouterBad(t *testing.T) {
	tests := []struct {
		source  string
		gateway string
		dest    string
		err     string
	}{
		{
			source:  "not an IP address",
			gateway: "1.1.1.1",
			dest:    "10.1.2.3",
			err:     "EGRESS_SOURCE unspecified or invalid",
		},
		{
			source:  "not\nan\nIP\naddress",
			gateway: "1.1.1.1",
			dest:    "10.1.2.3",
			err:     "EGRESS_SOURCE unspecified or invalid",
		},
		{
			source:  "1.2.3.4",
			gateway: "not an IP address",
			dest:    "10.1.2.3",
			err:     "EGRESS_GATEWAY unspecified or invalid",
		},
		{
			source:  "1.2.3.4",
			gateway: "1.1.1.1",
			dest:    "",
			err:     "EGRESS_DESTINATION unspecified",
		},
		{
			source:  "1.2.3.4",
			gateway: "1.1.1.1",
			dest:    "not an IP address",
			err:     "EGRESS_DESTINATION value 'not an IP address' is invalid",
		},
		{
			source:  "1.2.3.4",
			gateway: "1.1.1.1",
			dest:    "10.1.2.3\n80 tcp 10.4.5.6",
			err:     "EGRESS_DESTINATION fallback IP must be the last line",
		},
		{
			source:  "1.2.3.4",
			gateway: "1.1.1.1",
			dest:    "10.1.2.3\n10.4.5.6",
			err:     "EGRESS_DESTINATION fallback IP must be the last line",
		},
		{
			source:  "1.2.3.4",
			gateway: "1.1.1.1",
			dest:    "80 tcp 10.4.5.6\n10.1.2.3\n8080 tcp 10.7.8.9 80",
			err:     "EGRESS_DESTINATION fallback IP must be the last line",
		},
		{
			source:  "1.2.3.4",
			gateway: "1.1.1.1",
			dest:    "80 tcp 10.4.5.6\ninvalid\n8080 tcp 10.7.8.9 80",
			err:     "EGRESS_DESTINATION value 'invalid' is invalid",
		},
		{
			source:  "1.2.3.4",
			gateway: "1.1.1.1",
			dest:    "80 sctp 10.4.5.6",
			err:     "EGRESS_DESTINATION value '80 sctp 10.4.5.6' is invalid",
		},
		{
			source:  "1.2.3.4",
			gateway: "1.1.1.1",
			dest:    "800000 tcp 10.4.5.6",
			err:     "Invalid port: 800000, must be in the range 1 to 65535",
		},
		{
			source:  "1.2.3.4",
			gateway: "1.1.1.1",
			dest:    "80 tcp 10.4.5.6\n8080 tcp 10.7.8.9 80\n80 tcp 10.7.8.9 900",
			err:     "EGRESS_DESTINATION localport 80 is already used, must be unique for each destination",
		},
	}

	for n, test := range tests {
		cmd := exec.Command("./egress-router.sh")
		cmd.Env = []string{
			"EGRESS_ROUTER_MODE=unit-test",
			fmt.Sprintf("EGRESS_SOURCE=%s", test.source),
			fmt.Sprintf("EGRESS_GATEWAY=%s", test.gateway),
			fmt.Sprintf("EGRESS_DESTINATION=%s", test.dest),
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

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
			dest:    "not an IP address",
			err:     "EGRESS_DESTINATION unspecified or invalid",
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

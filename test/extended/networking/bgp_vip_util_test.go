package networking

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestRedactBGPVIPPasswords(t *testing.T) {
	for _, test := range []struct {
		name, in, want string
	}{
		{
			name: "frr running config",
			in:   " neighbor 192.168.111.1 password s3cr3t\n neighbor 192.168.111.1 bfd\n",
			want: " neighbor 192.168.111.1 password <redacted>\n neighbor 192.168.111.1 bfd\n",
		},
		{
			name: "json",
			in:   `{"address":"192.168.111.1","password":"s3cr3t","passwordSecret":{"name":"bgp"}}`,
			want: `{"address":"192.168.111.1","password":"<redacted>","passwordSecret":{"name":"bgp"}}`,
		},
		{
			name: "json with spaces",
			in:   `{"password" : "a b\"c"}`,
			want: `{"password" : "<redacted>"}`,
		},
		{
			name: "yaml",
			in:   "    password: s3cr3t\n    port: 179\n",
			want: "    password: <redacted>\n    port: 179\n",
		},
		{
			name: "no password",
			in:   `{"passwordSecret":{"name":"bgp"}}`,
			want: `{"passwordSecret":{"name":"bgp"}}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := redactBGPVIPPasswords(test.in); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestBGPVIPPeersFromConfig(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{
			"defaultPeers": []interface{}{
				map[string]interface{}{"peerAddress": "192.168.111.1", "peerASN": int64(64513), "failureDetection": "BFD", "holdTimeSeconds": int64(90), "keepaliveTimeSeconds": int64(30)},
				map[string]interface{}{"peerAddress": "fd2e:6f44:5dd8:c956::1", "peerASN": int64(64513)},
			},
		},
	}}
	want := []bgpVIPPeer{
		{Address: "192.168.111.1", ASN: 64513, FailureDetection: "BFD", HoldTimeSeconds: 90, KeepaliveTimeSeconds: 30},
		{Address: "fd2e:6f44:5dd8:c956::1", ASN: 64513},
	}
	got, err := bgpVIPPeersFromConfig(u)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestIntField(t *testing.T) {
	m := map[string]interface{}{
		"int64":    int64(7),
		"int":      8,
		"float":    float64(9),
		"fraction": 9.5,
		"number":   json.Number("10"),
		"badNum":   json.Number("1e400"),
		"string":   "11",
	}
	for key, want := range map[string]int64{"int64": 7, "int": 8, "float": 9, "number": 10, "absent": 0} {
		if got, err := intField(m, key); err != nil || got != want {
			t.Errorf("intField(%q) = %d, %v; want %d", key, got, err, want)
		}
	}
	for _, key := range []string{"fraction", "badNum", "string"} {
		if _, err := intField(m, key); err == nil {
			t.Errorf("intField(%q) did not fail", key)
		} else if strings.Contains(err.Error(), "11") || strings.Contains(err.Error(), "9.5") {
			t.Errorf("intField(%q) error quotes the value: %v", key, err)
		}
	}
}

func TestBGPVIPFamilies(t *testing.T) {
	peers := []bgpVIPPeer{{Address: "192.168.111.1"}}
	vips := []string{"192.168.111.5", "fd2e:6f44:5dd8:c956::5"}
	if got := vipsWithPeerFamily(vips, peers); !reflect.DeepEqual(got, []string{"192.168.111.5"}) {
		t.Errorf("v4 peer only: got %v", got)
	}
	peers = append(peers, bgpVIPPeer{Address: "fd2e:6f44:5dd8:c956::1"})
	if got := vipsWithPeerFamily(vips, peers); !reflect.DeepEqual(got, vips) {
		t.Errorf("dual-stack peers: got %v", got)
	}
	if got := peersOfFamily(peers, true); len(got) != 1 || got[0].Address != "fd2e:6f44:5dd8:c956::1" {
		t.Errorf("peersOfFamily(v6): got %v", got)
	}
}

func TestBGPVIPHostPrefix(t *testing.T) {
	if got := hostPrefix("192.168.111.5"); got != "192.168.111.5/32" {
		t.Error(got)
	}
	if got := hostPrefix("fd2e:6f44:5dd8:c956::5"); got != "fd2e:6f44:5dd8:c956::5/128" {
		t.Error(got)
	}
	if got := curlResolveAddr("fd2e::5"); got != "[fd2e::5]" {
		t.Error(got)
	}
}

func TestPeerTimersFromRenderedMachineConfig(t *testing.T) {
	peersJSON := `{"localASN":64512,"defaultPeers":[{"peerAddress":"192.168.111.1","peerASN":64513,"password":"not-a-real-secret","holdTime":"30","keepaliveTime":"10"},{"peerAddress":"fd2e::1","peerASN":64513}]}`
	for name, source := range map[string]string{
		"url-encoded": "data:," + url.PathEscape(peersJSON),
		"base64":      "data:text/plain;charset=utf-8;base64," + base64.StdEncoding.EncodeToString([]byte(peersJSON)),
	} {
		t.Run(name, func(t *testing.T) {
			mc := &unstructured.Unstructured{Object: map[string]interface{}{
				"spec": map[string]interface{}{
					"config": map[string]interface{}{
						"storage": map[string]interface{}{
							"files": []interface{}{
								map[string]interface{}{"path": "/etc/other", "contents": map[string]interface{}{"source": "data:,x"}},
								map[string]interface{}{"path": bgpVIPPeersFilePath, "contents": map[string]interface{}{"source": source}},
							},
						},
					},
				},
			}}
			got, found, err := peerTimersFromRenderedMachineConfig(mc)
			if err != nil || !found {
				t.Fatalf("found=%t err=%v", found, err)
			}
			want := map[string]bgpVIPTimers{"192.168.111.1": {Hold: "30", Keepalive: "10"}, "fd2e::1": {}}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("got %+v, want %+v", got, want)
			}
		})
	}
	t.Run("errors never carry the payload", func(t *testing.T) {
		mc := &unstructured.Unstructured{Object: map[string]interface{}{
			"spec": map[string]interface{}{"config": map[string]interface{}{"storage": map[string]interface{}{"files": []interface{}{
				map[string]interface{}{"path": bgpVIPPeersFilePath, "contents": map[string]interface{}{"source": "data:," + url.PathEscape(`{"password":"not-a-real-secret",`)}},
			}}}},
		}}
		_, _, err := peerTimersFromRenderedMachineConfig(mc)
		if err == nil {
			t.Fatal("expected an error")
		}
		if strings.Contains(err.Error(), "not-a-real-secret") {
			t.Errorf("error leaks the payload: %v", err)
		}
	})
	t.Run("absent", func(t *testing.T) {
		_, found, err := peerTimersFromRenderedMachineConfig(&unstructured.Unstructured{Object: map[string]interface{}{}})
		if err != nil || found {
			t.Errorf("found=%t err=%v", found, err)
		}
	})
}

func TestBGPVIPFailoverNftScripts(t *testing.T) {
	blackhole := bgpVIPBlackholeNftScript("52:54:00:aa:bb:cc", []string{"192.168.111.5", "fd2e::5"})
	for _, want := range []string{
		"table bridge bgp_vip_e2e",
		"type filter hook output priority -200; policy accept;",
		"ether daddr 52:54:00:aa:bb:cc ip daddr { 192.168.111.5 } tcp dport 6443 counter name v4 drop",
		"ether daddr 52:54:00:aa:bb:cc ip6 daddr { fd2e::5 } tcp dport 6443 counter name v6 drop",
	} {
		if !strings.Contains(blackhole, want) {
			t.Errorf("black hole script lacks %q:\n%s", want, blackhole)
		}
	}
	v4only := bgpVIPBlackholeNftScript("52:54:00:aa:bb:cc", []string{"192.168.111.5"})
	if strings.Contains(v4only, "ip6 daddr") || !strings.Contains(v4only, "counter v6") {
		t.Errorf("v4-only black hole must keep both counters and no ip6 rule:\n%s", v4only)
	}

	cut := bgpVIPPeeringCutNftScript([]string{"192.168.111.20", "fd2e::20"})
	for _, want := range []string{
		"table inet bgp_vip_e2e",
		"type filter hook input priority -200; policy accept;",
		"type filter hook output priority -200; policy accept;",
		"ip saddr { 192.168.111.20 } tcp dport 179 drop",
		"ip saddr { 192.168.111.20 } tcp sport 179 drop",
		"ip saddr { 192.168.111.20 } udp dport { 3784, 3785, 4784 } drop",
		"ip6 saddr { fd2e::20 } tcp dport 179 drop",
		"ip daddr { 192.168.111.20 } tcp dport 179 drop",
		"ip6 daddr { fd2e::20 } udp sport { 3784, 3785, 4784 } drop",
	} {
		if !strings.Contains(cut, want) {
			t.Errorf("peering cut script lacks %q:\n%s", want, cut)
		}
	}
}

func TestBGPVIPProbeAnalysis(t *testing.T) {
	log := strings.Join([]string{
		"100.0 192.168.111.5 200",
		"100.1 fd2e::5 200",
		"101.0 192.168.111.5 000",
		"101.5 fd2e::5 503",
		"102.0 192.168.111.5 200",
		"103.0 192.168.111.5 200",
		"103.0 fd2e::5 200",
		"garbage line",
	}, "\n")
	samples := parseBGPVIPProbeLog(log)
	if len(samples) != 7 {
		t.Fatalf("got %d samples", len(samples))
	}
	stats := summarizeBGPVIPProbe(samples, 100.5, 104)
	if got := stats["192.168.111.5"]; got.Total != 3 || got.Failed != 1 || got.LastFailure != 101.0 {
		t.Errorf("v4: %+v", got)
	}
	if got := stats["fd2e::5"]; got.Total != 2 || got.Failed != 1 || got.LastFailure != 101.5 {
		t.Errorf("v6: %+v", got)
	}
}

func TestParseHypervisorClock(t *testing.T) {
	for _, test := range []struct {
		name, in string
		want     float64
		wantErr  bool
	}{
		{name: "date only", in: "1794397490.709199123\n", want: 1794397490.709199123},
		{name: "after other output", in: "some nft warning\n1794397490.5\n", want: 1794397490.5},
		{name: "empty", in: "", wantErr: true},
		{name: "garbage", in: "nope\n", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseHypervisorClock(test.in)
			if (err != nil) != test.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, test.wantErr)
			}
			if !test.wantErr && got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestPickBGPVIPDay2Timers(t *testing.T) {
	for _, test := range []struct {
		name     string
		peers    []bgpVIPPeer
		hold, ka int64
		ok       bool
	}{
		{name: "unset timers", peers: []bgpVIPPeer{{}}, hold: 30, ka: 10, ok: true},
		{name: "first pair in use", peers: []bgpVIPPeer{{HoldTimeSeconds: 90}, {HoldTimeSeconds: 30, KeepaliveTimeSeconds: 10}}, hold: 24, ka: 8, ok: true},
		{name: "first hold in use", peers: []bgpVIPPeer{{HoldTimeSeconds: 30, KeepaliveTimeSeconds: 5}}, hold: 24, ka: 8, ok: true},
		{name: "all in use", peers: []bgpVIPPeer{{HoldTimeSeconds: 30}, {HoldTimeSeconds: 24}, {KeepaliveTimeSeconds: 12}}, ok: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			hold, ka, ok := pickBGPVIPDay2Timers(test.peers)
			if hold != test.hold || ka != test.ka || ok != test.ok {
				t.Errorf("got %d/%d %v, want %d/%d %v", hold, ka, ok, test.hold, test.ka, test.ok)
			}
		})
	}
}

func TestCheckBGPVIPNegotiatedTimers(t *testing.T) {
	session := func(cfgHold, negHold, cfgKA, negKA int64) frrNeighbor {
		return frrNeighbor{BGPState: "Established", ConfiguredHoldTimeMsecs: cfgHold, NegotiatedHoldTimeMsecs: negHold, ConfiguredKeepAliveMsecs: cfgKA, NegotiatedKeepAliveMsecs: negKA}
	}
	for _, test := range []struct {
		name      string
		n, prior  frrNeighbor
		havePrior bool
		ok        bool
	}{
		{name: "lowered below a known peer proposal", prior: session(180000, 90000, 60000, 30000), havePrior: true, n: session(30000, 30000, 10000, 10000), ok: true},
		{name: "stale after lowering", prior: session(180000, 90000, 60000, 30000), havePrior: true, n: session(30000, 90000, 10000, 10000), ok: false},
		{name: "peer proposes less than the new value", prior: session(180000, 20000, 60000, 6000), havePrior: true, n: session(30000, 20000, 10000, 6000), ok: true},
		{name: "peer proposal below new value but equality expected", prior: session(180000, 20000, 60000, 6000), havePrior: true, n: session(30000, 30000, 10000, 10000), ok: false},
		{name: "raised with unknown peer proposal", prior: session(30000, 30000, 10000, 10000), havePrior: true, n: session(180000, 30000, 60000, 10000), ok: true},
		{name: "raised beyond configured", prior: session(30000, 30000, 10000, 10000), havePrior: true, n: session(90000, 180000, 30000, 60000), ok: false},
		{name: "keepalive from a third of the hold time", prior: session(180000, 90000, 60000, 30000), havePrior: true, n: session(30000, 30000, 60000, 10000), ok: true},
		{name: "keepalive mismatch", prior: session(180000, 90000, 60000, 30000), havePrior: true, n: session(30000, 30000, 10000, 30000), ok: false},
		{name: "no prior session", n: session(30000, 20000, 10000, 6000), ok: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			problems := checkBGPVIPNegotiatedTimers("master-0", "192.168.111.1", test.n, test.prior, test.havePrior)
			if (len(problems) == 0) != test.ok {
				t.Errorf("problems %v, want ok=%v", problems, test.ok)
			}
		})
	}
}

func TestBGPVIPProbePorts(t *testing.T) {
	p := bgpVIPIngressProbePorts
	for _, test := range []struct {
		request     int
		first, last int
	}{
		{request: 0, first: 20000, last: 20003},
		{request: 1, first: 20004, last: 20007},
		{request: 2499, first: 29996, last: 29999},
		{request: 2500, first: 20000, last: 20003},
	} {
		if first, last := p.ports(test.request); first != test.first || last != test.last {
			t.Errorf("ports(%d) = %d-%d, want %d-%d", test.request, first, last, test.first, test.last)
		}
	}
	for port, want := range map[int]int{19999: -1, 20000: 0, 20003: 0, 20004: 1, 29999: 2499, 30000: -1} {
		if got := p.block(port); got != want {
			t.Errorf("block(%d) = %d, want %d", port, got, want)
		}
	}
	if _, last := p.ports(p.Blocks - 1); last >= 32768 {
		t.Errorf("the probe ports reach the ephemeral port range: %d", last)
	}
	// 2 VIPs at 1 request/s for the whole body must not reuse a block.
	if requests := 2 * int(bgpVIPIngressMoveBodyTimeout.Seconds()); requests > p.Blocks {
		t.Errorf("%d requests in the ingress VIP move body reuse the %d port blocks", requests, p.Blocks)
	}
}

func TestBGPVIPProbeLoop(t *testing.T) {
	const probeURL = "https://canary.apps.example.com/healthz"
	loop, err := bgpVIPProbeLoop(probeURL, []string{"192.168.111.5"}, 2, "/tmp/p/log", nil)
	if err != nil {
		t.Fatal(err)
	}
	untagged := "  for i in $(seq 2); do ( t=$(date +%s.%N); c=$(curl -sk -o /dev/null -w '%{http_code}' --max-time 3 --noproxy '*' --resolve " +
		shellQuote("canary.apps.example.com:443:192.168.111.5") + " " + shellQuote(probeURL) + " || true); echo \"$t 192.168.111.5 ${c:-000}\" >> /tmp/p/log ) & done\n"
	if !strings.Contains(loop, untagged) || strings.Contains(loop, "--local-port") {
		t.Errorf("untagged loop:\n%s\nwant line:\n%s", loop, untagged)
	}
	if _, err := bgpVIPProbeLoop("not a url", []string{"192.168.111.5"}, 1, "/tmp/p/log", nil); err == nil {
		t.Error("a URL without host was accepted")
	}
}

// TestBGPVIPProbeLoopRuns runs two rounds of the tagged probe loop with a
// fake curl, to check the request numbers, the ports of each request and the
// log format.
func TestBGPVIPProbeLoopRuns(t *testing.T) {
	dir := t.TempDir()
	fakeCurl := "#!/bin/bash\nwhile [ $# -gt 0 ]; do if [ \"$1\" = --local-port ]; then echo \"$2\" >> " + dir + "/ports; fi; shift; done\nprintf 200\n"
	if err := os.WriteFile(dir+"/curl", []byte(fakeCurl), 0o755); err != nil {
		t.Fatal(err)
	}
	ports := bgpVIPProbePorts{Base: 20000, Block: 4, Blocks: 3}
	loop, err := bgpVIPProbeLoop("https://canary.apps.example.com", []string{"192.168.111.5", "fd2e::5"}, 1, dir+"/log", &ports)
	if err != nil {
		t.Fatal(err)
	}
	loop = strings.Replace(loop, "while :; do", "for round in 1 2; do", 1)
	loop = strings.Replace(loop, "  sleep 1\n", "  wait\n", 1)
	if _, err := runBGPVIPBash(t, bgpVIPTestScriptTimeout, dir+":"+os.Getenv("PATH"), loop); err != nil {
		t.Fatal(err)
	}
	log, err := os.ReadFile(dir + "/log")
	if err != nil {
		t.Fatal(err)
	}
	samples := parseBGPVIPProbeLog(string(log))
	got := map[int]string{}
	for _, s := range samples {
		if !s.OK {
			t.Errorf("sample %+v failed", s)
		}
		got[s.Request] = s.VIP
	}
	want := map[int]string{0: "192.168.111.5", 1: "fd2e::5", 2: "192.168.111.5", 3: "fd2e::5"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("requests %v, want %v", got, want)
	}
	usedPorts, err := os.ReadFile(dir + "/ports")
	if err != nil {
		t.Fatal(err)
	}
	gotPorts := strings.Fields(string(usedPorts))
	sort.Strings(gotPorts)
	// request 3 wraps around to the first block.
	if wantPorts := []string{"20000-20003", "20000-20003", "20004-20007", "20008-20011"}; !reflect.DeepEqual(gotPorts, wantPorts) {
		t.Errorf("local ports %v, want %v", gotPorts, wantPorts)
	}
}

func TestParseBGPVIPProbeLogRequests(t *testing.T) {
	samples := parseBGPVIPProbeLog(strings.Join([]string{
		"102.0 192.168.111.5 200 7",
		"100.0 192.168.111.5 200",
		"101.0 fd2e::5 000 3",
		"103.0 fd2e::5 200 -1",
		"104.0 fd2e::5 200 x",
		"105.0 fd2e::5 200 1 extra",
	}, "\n"))
	want := []bgpVIPProbeSample{
		{Time: 100.0, VIP: "192.168.111.5", OK: true, Request: -1},
		{Time: 101.0, VIP: "fd2e::5", OK: false, Request: 3},
		{Time: 102.0, VIP: "192.168.111.5", OK: true, Request: 7},
	}
	if !reflect.DeepEqual(samples, want) {
		t.Errorf("got %+v, want %+v", samples, want)
	}
}

func TestBGPVIPProbePathNftScript(t *testing.T) {
	routers := []bgpVIPExposureRouter{
		{Node: "worker-0", MACs: []string{"52:54:00:00:00:01"}},
		{Node: "worker-1", MACs: []string{"52:54:00:00:00:02", "52:54:00:00:00:03"}},
	}
	dual := bgpVIPProbePathNftScript(routers, []string{"192.168.111.5", "fd2e::5"}, "443", bgpVIPIngressProbePorts)
	for _, want := range []string{
		"table bridge bgp_vip_e2e {",
		"\tset p0_ipv4 {\n\t\ttype inet_service\n\t\tsize 65535\n\t\tflags dynamic\n\t}\n",
		"\tcounter s1_ipv6 { }\n",
		"type filter hook output priority -200; policy accept;",
		"ether daddr { 52:54:00:00:00:01 } ip daddr { 192.168.111.5 } tcp sport 20000-29999 tcp dport 443 tcp flags & (syn | ack) == syn add @p0_ipv4 { tcp sport } counter name s0_ipv4\n",
		"ether daddr { 52:54:00:00:00:02, 52:54:00:00:00:03 } ip6 daddr { fd2e::5 } tcp sport 20000-29999 tcp dport 443 tcp flags & (syn | ack) == syn add @p1_ipv6 { tcp sport } counter name s1_ipv6\n",
	} {
		if !strings.Contains(dual, want) {
			t.Errorf("dual-stack script lacks %q:\n%s", want, dual)
		}
	}
	if strings.Contains(dual, "drop") {
		t.Errorf("the path records drop traffic:\n%s", dual)
	}
	v4 := bgpVIPProbePathNftScript(routers, []string{"192.168.111.5"}, "8443", bgpVIPIngressProbePorts)
	if strings.Contains(v4, "ip6") || strings.Contains(v4, "ipv6") || !strings.Contains(v4, "tcp dport 8443") {
		t.Errorf("IPv4-only script:\n%s", v4)
	}
}

const bgpVIPTestNftJSON = `{"nftables": [{"metainfo": {"version": "1.0.9", "json_schema_version": 1}}, {"table": {"family": "bridge", "name": "bgp_vip_e2e", "handle": 7}},` +
	` {"set": {"family": "bridge", "name": "p0_ipv4", "table": "bgp_vip_e2e", "type": "inet_service", "handle": 1, "size": 65535, "flags": ["dynamic"], "elem": [20000, 20008]}},` +
	` {"counter": {"family": "bridge", "name": "s0_ipv4", "table": "bgp_vip_e2e", "handle": 2, "packets": 3, "bytes": 180}},` +
	` {"set": {"family": "bridge", "name": "p0_ipv6", "table": "bgp_vip_e2e", "type": "inet_service", "handle": 3, "size": 65535, "flags": ["dynamic"], "elem": [{"elem": {"val": 20004, "expires": 1000}}]}},` +
	` {"counter": {"family": "bridge", "name": "s0_ipv6", "table": "bgp_vip_e2e", "handle": 4, "packets": 1, "bytes": 80}},` +
	` {"set": {"family": "bridge", "name": "p1_ipv4", "table": "bgp_vip_e2e", "type": "inet_service", "handle": 5, "size": 65535, "flags": ["dynamic"], "elem": [20008]}},` +
	` {"counter": {"family": "bridge", "name": "s1_ipv4", "table": "bgp_vip_e2e", "handle": 6, "packets": 1, "bytes": 60}},` +
	` {"set": {"family": "bridge", "name": "p1_ipv6", "table": "bgp_vip_e2e", "type": "inet_service", "handle": 8, "size": 65535, "flags": ["dynamic"]}},` +
	` {"counter": {"family": "bridge", "name": "s1_ipv6", "table": "bgp_vip_e2e", "handle": 9, "packets": 0, "bytes": 0}}]}`

func TestParseBGPVIPProbeObservation(t *testing.T) {
	routers := []bgpVIPExposureRouter{{Node: "worker-0"}, {Node: "worker-1"}}
	families := []string{"ipv4", "ipv6"}
	log := "100.0 192.168.111.5 200 0\n100.5 fd2e::5 200 1\n101.0 192.168.111.5 000 2\n"
	obs, err := parseBGPVIPProbeObservation(log+bgpVIPObservationMarker+"\n"+bgpVIPTestNftJSON+"\n\n1794397490.5\n", routers, families, bgpVIPIngressProbePorts)
	if err != nil {
		t.Fatal(err)
	}
	if obs.Time != 1794397490.5 || len(obs.Samples) != 3 {
		t.Errorf("time %v, samples %+v", obs.Time, obs.Samples)
	}
	wantBlocks := map[string]map[string]sets.Set[int]{
		"worker-0": {"ipv4": sets.New(0, 2), "ipv6": sets.New(1)},
		"worker-1": {"ipv4": sets.New(2), "ipv6": sets.New[int]()},
	}
	if !reflect.DeepEqual(obs.Blocks, wantBlocks) {
		t.Errorf("blocks %v, want %v", obs.Blocks, wantBlocks)
	}
	if wantSYNs := map[string]map[string]int64{"worker-0": {"ipv4": 3, "ipv6": 1}, "worker-1": {"ipv4": 1, "ipv6": 0}}; !reflect.DeepEqual(obs.SYNs, wantSYNs) {
		t.Errorf("SYNs %v, want %v", obs.SYNs, wantSYNs)
	}

	for _, test := range []struct{ name, out string }{
		{name: "no marker", out: log + bgpVIPTestNftJSON + "\n1794397490.5\n"},
		{name: "no clock", out: bgpVIPObservationMarker + "\n" + bgpVIPTestNftJSON + "\n"},
		{name: "no nft output", out: bgpVIPObservationMarker + "\n\n1794397490.5\n"},
		{name: "not JSON", out: bgpVIPObservationMarker + "\n{nftables\n1794397490.5\n"},
		{name: "missing set", out: bgpVIPObservationMarker + "\n" + strings.Replace(bgpVIPTestNftJSON, `"p1_ipv6"`, `"other"`, 1) + "\n1794397490.5\n"},
		{name: "missing counter", out: bgpVIPObservationMarker + "\n" + strings.Replace(bgpVIPTestNftJSON, `"s0_ipv6"`, `"other"`, 1) + "\n1794397490.5\n"},
		{name: "service name element", out: bgpVIPObservationMarker + "\n" + strings.Replace(bgpVIPTestNftJSON, `[20008]`, `["http"]`, 1) + "\n1794397490.5\n"},
		{name: "range element", out: bgpVIPObservationMarker + "\n" + strings.Replace(bgpVIPTestNftJSON, `[20008]`, `[{"range": [20000, 20003]}]`, 1) + "\n1794397490.5\n"},
		{name: "port outside the probe's ports", out: bgpVIPObservationMarker + "\n" + strings.Replace(bgpVIPTestNftJSON, `[20008]`, `[443]`, 1) + "\n1794397490.5\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if obs, err := parseBGPVIPProbeObservation(test.out, routers, families, bgpVIPIngressProbePorts); err == nil {
				t.Errorf("accepted: %+v", obs)
			}
		})
	}
}

func TestAttributeBGPVIPProbe(t *testing.T) {
	ports := bgpVIPProbePorts{Base: 20000, Block: 4, Blocks: 10}
	blocks := map[string]map[string]sets.Set[int]{
		"worker-0": {"ipv4": sets.New(0, 2, 5), "ipv6": sets.New[int]()},
		"worker-1": {"ipv4": sets.New(2), "ipv6": sets.New(1)},
	}
	samples := []bgpVIPProbeSample{
		{Time: 10, VIP: "192.168.111.5", OK: true, Request: 0},
		{Time: 10.1, VIP: "fd2e::5", OK: true, Request: 1},
		// retransmitted SYN that the hypervisor sent to both nodes.
		{Time: 11, VIP: "192.168.111.5", OK: false, Request: 2},
		// no SYN recorded.
		{Time: 12, VIP: "192.168.111.5", OK: true, Request: 3},
		{Time: 13, VIP: "192.168.111.5", OK: true, Request: -1},
		// requests 5 and 15 share block 5.
		{Time: 14, VIP: "192.168.111.5", OK: true, Request: 5},
		{Time: 24, VIP: "192.168.111.5", OK: true, Request: 15},
	}
	got := attributeBGPVIPProbe(samples, blocks, ports)
	want := [][]string{{"worker-0"}, {"worker-1"}, {"worker-0", "worker-1"}, nil, nil, nil, nil}
	for i, a := range got {
		if !reflect.DeepEqual(a.Routers, want[i]) || a.bgpVIPProbeSample != samples[i] {
			t.Errorf("sample %d: %+v, want routers %v", i, a, want[i])
		}
	}
	if got[1].Family != "ipv6" || got[0].Family != "ipv4" {
		t.Errorf("families: %+v", got)
	}

	exposure := bgpVIPExposure(got, 0, math.Inf(1))
	if wantExposure := map[string]map[string]int{"worker-0": {"ipv4": 1}, "worker-1": {"ipv6": 1}}; !reflect.DeepEqual(exposure, wantExposure) {
		t.Errorf("exposure %v, want %v", exposure, wantExposure)
	}
	if exposure := bgpVIPExposure(got, 10.05, 11); !reflect.DeepEqual(exposure, map[string]map[string]int{"worker-1": {"ipv6": 1}}) {
		t.Errorf("exposure in [10.05, 11): %v", exposure)
	}
}

func TestSelectBGPVIPExposedRouter(t *testing.T) {
	dual := []string{"ipv4", "ipv6"}
	for _, test := range []struct {
		name     string
		exposure map[string]map[string]int
		families []string
		want     string
	}{
		{name: "IPv4", exposure: map[string]map[string]int{"a": {"ipv4": 6}, "b": {"ipv4": 9}}, families: []string{"ipv4"}, want: "b"},
		{name: "IPv6", exposure: map[string]map[string]int{"a": {"ipv6": 5}, "b": {"ipv6": 4}}, families: []string{"ipv6"}, want: "a"},
		{name: "dual-stack prefers the most exposed least family", exposure: map[string]map[string]int{"a": {"ipv4": 20, "ipv6": 5}, "b": {"ipv4": 6, "ipv6": 6}}, families: dual, want: "b"},
		{name: "families split between nodes", exposure: map[string]map[string]int{"a": {"ipv4": 20}, "b": {"ipv6": 20}}, families: dual},
		{name: "below the threshold", exposure: map[string]map[string]int{"a": {"ipv4": 4, "ipv6": 9}}, families: dual},
		{name: "at the threshold", exposure: map[string]map[string]int{"a": {"ipv4": 5, "ipv6": 5}}, families: dual, want: "a"},
		{name: "tie goes to the first by name", exposure: map[string]map[string]int{"b": {"ipv4": 7, "ipv6": 7}, "a": {"ipv4": 8, "ipv6": 7}}, families: dual, want: "a"},
		{name: "no traffic", exposure: map[string]map[string]int{}, families: dual},
		{name: "no family", exposure: map[string]map[string]int{"a": {"ipv4": 9}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := selectBGPVIPExposedRouter(test.exposure, test.families, 5)
			if got != test.want || ok != (test.want != "") {
				t.Errorf("got %q, %t, want %q", got, ok, test.want)
			}
		})
	}
}

func TestBGPVIPProbePhases(t *testing.T) {
	phases, err := bgpVIPProbePhases(bgpVIPIngressMovePhaseNames, []float64{10, 20, 20, 40})
	if err != nil {
		t.Fatal(err)
	}
	want := []bgpVIPProbePhase{
		{Name: "exposure", From: math.Inf(-1), To: 10},
		{Name: "withdrawal", From: 10, To: 20},
		{Name: "moved", From: 20, To: 20},
		{Name: "restoration", From: 20, To: 40},
		{Name: "post-restore", From: 40, To: math.Inf(1)},
	}
	if !reflect.DeepEqual(phases, want) {
		t.Errorf("got %+v, want %+v", phases, want)
	}
	for _, test := range []struct {
		name       string
		names      []string
		boundaries []float64
	}{
		{name: "no phase"},
		{name: "too few boundaries", names: bgpVIPIngressMovePhaseNames, boundaries: []float64{10, 20, 30}},
		{name: "too many boundaries", names: []string{"a"}, boundaries: []float64{10}},
		{name: "boundaries out of order", names: bgpVIPIngressMovePhaseNames, boundaries: []float64{10, 30, 20, 40}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got, err := bgpVIPProbePhases(test.names, test.boundaries); err == nil {
				t.Errorf("accepted: %+v", got)
			}
		})
	}
}

func TestSummarizeBGPVIPPhases(t *testing.T) {
	const v4, v6 = "192.168.111.5", "fd2e::5"
	phases, err := bgpVIPProbePhases(bgpVIPIngressMovePhaseNames, []float64{10, 20, 30, 40})
	if err != nil {
		t.Fatal(err)
	}
	sample := func(at float64, vip string, ok bool, routers ...string) bgpVIPAttributedSample {
		return bgpVIPAttributedSample{bgpVIPProbeSample: bgpVIPProbeSample{Time: at, VIP: vip, OK: ok}, Family: addressFamily(vip), Routers: routers}
	}
	samples := []bgpVIPAttributedSample{
		sample(5, v4, true, "worker-0"),
		sample(10, v4, false, "worker-0"),
		sample(15, v4, false),
		sample(20, v4, true, "worker-1"),
		sample(25, v6, true, "worker-1"),
		sample(39.9, v4, false, "worker-1"),
		sample(40, v4, true, "worker-0"),
		// in flight when the probe stopped
		sample(100, v4, false, "worker-0", "worker-1"),
	}
	perPhase, overall := summarizeBGPVIPPhases(phases, samples)

	total := 0
	for _, p := range phases {
		for _, st := range perPhase[p.Name] {
			total += st.Total
		}
	}
	if total != len(samples) {
		t.Errorf("phases hold %d samples, want %d", total, len(samples))
	}
	for _, test := range []struct {
		phase, vip    string
		total, failed int
		first, last   *bgpVIPFailure
		requests      map[string]int
	}{
		{phase: "exposure", vip: v4, total: 1, requests: map[string]int{"worker-0": 1}},
		{phase: "withdrawal", vip: v4, total: 2, failed: 2, first: &bgpVIPFailure{Time: 10, Phase: "withdrawal", Routers: []string{"worker-0"}}, last: &bgpVIPFailure{Time: 15, Phase: "withdrawal"}, requests: map[string]int{"worker-0": 1, "none": 1}},
		{phase: "moved", vip: v4, total: 1, requests: map[string]int{"worker-1": 1}},
		{phase: "moved", vip: v6, total: 1, requests: map[string]int{"worker-1": 1}},
		{phase: "restoration", vip: v4, total: 1, failed: 1, first: &bgpVIPFailure{Time: 39.9, Phase: "restoration", Routers: []string{"worker-1"}}, last: &bgpVIPFailure{Time: 39.9, Phase: "restoration", Routers: []string{"worker-1"}}, requests: map[string]int{"worker-1": 1}},
		{phase: "post-restore", vip: v4, total: 2, failed: 1, first: &bgpVIPFailure{Time: 100, Phase: "post-restore", Routers: []string{"worker-0", "worker-1"}}, last: &bgpVIPFailure{Time: 100, Phase: "post-restore", Routers: []string{"worker-0", "worker-1"}}, requests: map[string]int{"worker-0": 1, "worker-0+worker-1": 1}},
	} {
		st := perPhase[test.phase][test.vip]
		if st == nil || st.Total != test.total || st.Failed != test.failed || !reflect.DeepEqual(st.First, test.first) || !reflect.DeepEqual(st.Last, test.last) || !reflect.DeepEqual(st.Requests, test.requests) {
			t.Errorf("phase %s, %s: %+v, want total %d failed %d first %+v last %+v requests %v", test.phase, test.vip, st, test.total, test.failed, test.first, test.last, test.requests)
		}
	}
	if perPhase["exposure"][v6] != nil {
		t.Errorf("no IPv6 sample in the exposure phase: %+v", perPhase["exposure"][v6])
	}

	st := overall[v4]
	if st.Total != 7 || st.Failed != 4 || st.First.Time != 10 || st.First.Phase != "withdrawal" || st.Last.Time != 100 || st.Last.Phase != "post-restore" {
		t.Errorf("overall v4: %+v first %+v last %+v", st, st.First, st.Last)
	}
	var raw []bgpVIPProbeSample
	for _, s := range samples {
		raw = append(raw, s.bgpVIPProbeSample)
	}
	for vip, want := range summarizeBGPVIPProbe(raw, 0, math.Inf(1)) {
		if got := overall[vip]; got.Total != want.Total || got.Failed != want.Failed {
			t.Errorf("overall %s: %d/%d, the loss assertion counts %d/%d", vip, got.Failed, got.Total, want.Failed, want.Total)
		}
	}

	desc := describeBGPVIPPhaseStats("whole run", v4, st, 10)
	for _, want := range []string{"via 192.168.111.5 (ipv4), whole run: 7 requests, 4 failed", "first failure +0.0s in phase withdrawal via worker-0", "last failure +90.0s in phase post-restore via worker-0+worker-1"} {
		if !strings.Contains(desc, want) {
			t.Errorf("%q lacks %q", desc, want)
		}
	}
	if desc := describeBGPVIPPhaseStats("phase withdrawal", v4, perPhase["withdrawal"][v4], 10); !strings.Contains(desc, "last failure +5.0s in phase withdrawal via no recorded router node") {
		t.Errorf("unattributed failure: %q", desc)
	}
	if desc := describeBGPVIPPhaseStats("phase exposure", v6, nil, 10); !strings.Contains(desc, "0 requests, 0 failed") || strings.Contains(desc, "first failure") {
		t.Errorf("no sample: %q", desc)
	}
}

func TestParseBGPVIPURLAndFamilies(t *testing.T) {
	for _, test := range []struct {
		in, host, port string
		wantErr        bool
	}{
		{in: "https://canary-openshift-ingress-canary.apps.ostest.test.metalkube.org", host: "canary-openshift-ingress-canary.apps.ostest.test.metalkube.org", port: "443"},
		{in: "https://h.example.com:8443/x", host: "h.example.com", port: "8443"},
		{in: "http://[fd2e::5]:80/", host: "fd2e::5", port: "80"},
		{in: "/relative", wantErr: true},
		{in: "://bad", wantErr: true},
	} {
		u, port, err := parseBGPVIPURL(test.in)
		if (err != nil) != test.wantErr {
			t.Errorf("%q: err = %v, wantErr %t", test.in, err, test.wantErr)
			continue
		}
		if !test.wantErr && (u.Hostname() != test.host || port != test.port) {
			t.Errorf("%q: host %q port %q, want %q %q", test.in, u.Hostname(), port, test.host, test.port)
		}
	}
	for _, test := range []struct {
		ips  []string
		want []string
	}{
		{ips: nil, want: []string{}},
		{ips: []string{"192.168.111.5"}, want: []string{"ipv4"}},
		{ips: []string{"fd2e::5", "192.168.111.5", "192.168.111.6"}, want: []string{"ipv4", "ipv6"}},
	} {
		if got := bgpVIPFamilies(test.ips); !reflect.DeepEqual(got, test.want) {
			t.Errorf("bgpVIPFamilies(%v) = %v, want %v", test.ips, got, test.want)
		}
	}
}

func TestCompareTorPaths(t *testing.T) {
	nodeIPs := map[string]string{"192.168.111.20": "worker-0", "fd2e::20": "worker-0", "192.168.111.21": "worker-1"}
	if !bgpVIPTorPathsVia([]string{"192.168.111.21", "fd2e::20"}, "worker-0", nodeIPs) || bgpVIPTorPathsVia([]string{"192.168.111.21"}, "worker-0", nodeIPs) {
		t.Error("bgpVIPTorPathsVia")
	}
	for _, test := range []struct {
		name    string
		peers   []string
		wantErr bool
	}{
		{name: "exactly the wanted node", peers: []string{"192.168.111.21"}},
		{name: "victim still there", peers: []string{"192.168.111.20", "192.168.111.21"}, wantErr: true},
		{name: "no path", wantErr: true},
		{name: "two paths from one node", peers: []string{"192.168.111.21", "192.168.111.21"}, wantErr: true},
	} {
		if err := compareTorPaths("192.168.111.5", test.peers, sets.New("worker-1"), nodeIPs); (err != nil) != test.wantErr {
			t.Errorf("%s: err = %v, wantErr %t", test.name, err, test.wantErr)
		}
	}
}

func TestRunBGPVIPCleanup(t *testing.T) {
	var ran []string
	errBroken := errors.New("broken")
	steps := []bgpVIPCleanupStep{
		{Name: "fails", Timeout: time.Minute, Run: func(ctx context.Context) error { ran = append(ran, "fails"); return errBroken }},
		{Name: "hangs", Timeout: 50 * time.Millisecond, Run: func(ctx context.Context) error {
			ran = append(ran, "hangs")
			<-ctx.Done()
			return ctx.Err()
		}},
		{Name: "unbounded", Run: func(ctx context.Context) error {
			if _, ok := ctx.Deadline(); ok {
				t.Error("a step without timeout got a deadline")
			}
			ran = append(ran, "unbounded")
			return nil
		}},
	}
	err := runBGPVIPCleanup(context.Background(), steps)
	if !reflect.DeepEqual(ran, []string{"fails", "hangs", "unbounded"}) {
		t.Errorf("ran %v", ran)
	}
	if !errors.Is(err, errBroken) || !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "fails: broken") {
		t.Errorf("err = %v", err)
	}
	if err := runBGPVIPCleanup(context.Background(), nil); err != nil {
		t.Errorf("no step: %v", err)
	}
}

func TestBGPVIPIngressMoveCleanupSteps(t *testing.T) {
	for _, test := range []struct {
		name  string
		state bgpVIPIngressMoveState
		want  []string
	}{
		{name: "nothing changed"},
		{name: "probe recorded but failed to start", state: bgpVIPIngressMoveState{probe: &bgpVIPProbe{}}, want: []string{"stopProbe"}},
		{name: "failed while recording the paths", state: bgpVIPIngressMoveState{pathsRecorded: true}, want: []string{"restoreHost true false"}},
		{name: "failed while setting the hash policy", state: bgpVIPIngressMoveState{probe: &bgpVIPProbe{}, pathsRecorded: true, hashPolicySet: true}, want: []string{"stopProbe", "restoreHost true true"}},
		{name: "failed after cordoning", state: bgpVIPIngressMoveState{probe: &bgpVIPProbe{}, pathsRecorded: true, victim: "worker-0", wasUnschedulable: true}, want: []string{"stopProbe", "restoreHost true false", "restoreNode worker-0 true"}},
		{name: "failed after stopping the probe", state: bgpVIPIngressMoveState{pathsRecorded: true, hashPolicySet: true, victim: "worker-0"}, want: []string{"restoreHost true true", "restoreNode worker-0 false"}},
		{name: "restored by the test", state: bgpVIPIngressMoveState{pathsRecorded: true, victim: "worker-0", restored: true}, want: []string{"restoreHost true false"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var ran []string
			// the hypervisor steps fail, which must not keep the later steps
			// from running; the node restoration succeeds.
			steps := test.state.cleanupSteps(bgpVIPIngressMoveActions{
				stopProbe: func(ctx context.Context) error {
					ran = append(ran, "stopProbe")
					return errors.New("stopProbe failed")
				},
				restoreHost: func(ctx context.Context, pathsRecorded, hashPolicySet bool) error {
					ran = append(ran, fmt.Sprintf("restoreHost %t %t", pathsRecorded, hashPolicySet))
					return errors.New("restoreHost failed")
				},
				restoreNode: func(ctx context.Context, node string, unschedulable bool) error {
					ran = append(ran, fmt.Sprintf("restoreNode %s %t", node, unschedulable))
					return nil
				},
			})
			for _, step := range steps {
				want := bgpVIPIngressMoveHostStep
				if strings.HasPrefix(step.Name, "restore node") {
					want = bgpVIPIngressMoveRestoreBudget
				}
				if step.Timeout != want {
					t.Errorf("step %q has budget %v, want %v", step.Name, step.Timeout, want)
				}
			}
			err := runBGPVIPCleanup(context.Background(), steps)
			if !reflect.DeepEqual(ran, test.want) {
				t.Errorf("ran %v, want %v", ran, test.want)
			}
			hostStepRan := slices.ContainsFunc(ran, func(s string) bool { return !strings.HasPrefix(s, "restoreNode") })
			if (err != nil) != hostStepRan {
				t.Errorf("err = %v with steps %v", err, ran)
			}
		})
	}
	if bgpVIPIngressMoveBodyTimeout != 11*time.Minute {
		t.Errorf("the ingress VIP move body has %v, want its original 11m", bgpVIPIngressMoveBodyTimeout)
	}
	cleanup := 2*bgpVIPIngressMoveHostStep + bgpVIPIngressMoveRestoreBudget
	if total := bgpVIPIngressMoveBodyTimeout + bgpVIPDumpTimeout + cleanup + time.Minute; total > 20*time.Minute {
		t.Errorf("body %v + dump %v + cleanup %v + 1m margin = %v exceed the 20m suite timeout", bgpVIPIngressMoveBodyTimeout, bgpVIPDumpTimeout, cleanup, total)
	}
}

func TestCheckBGPVIPProbePorts(t *testing.T) {
	for _, test := range []struct {
		name, in    string
		wantErr     string
		wantOverlap bool
	}{
		{name: "default range", in: "32768\t60999\n"},
		{name: "just above the probe ports", in: "30000 60999"},
		{name: "just below the probe ports", in: "1024 19999"},
		{name: "overlaps the last probe port", in: "29999 60999", wantOverlap: true},
		{name: "overlaps the first probe port", in: "1024 20000", wantOverlap: true},
		{name: "covers the probe ports", in: "1024 65535", wantOverlap: true},
		{name: "inside the probe ports", in: "21000 22000", wantOverlap: true},
		{name: "empty", in: "", wantErr: "cannot parse"},
		{name: "one port", in: "32768", wantErr: "cannot parse"},
		{name: "three fields", in: "32768 60999 1", wantErr: "cannot parse"},
		{name: "not a number", in: "a 60999", wantErr: "cannot parse"},
		{name: "high not a number", in: "32768 b", wantErr: "cannot parse"},
		{name: "reversed", in: "60999 32768", wantErr: "cannot parse"},
		{name: "port 0", in: "0 60999", wantErr: "cannot parse"},
		{name: "beyond 65535", in: "1024 65536", wantErr: "cannot parse"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := checkBGPVIPProbePorts(test.in, bgpVIPIngressProbePorts)
			switch {
			case test.wantOverlap:
				if err == nil || !strings.Contains(err.Error(), "overlap the hypervisor's ephemeral ports") {
					t.Errorf("err = %v, want an overlap", err)
				}
			case test.wantErr != "":
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Errorf("err = %v, want %q", err, test.wantErr)
				}
			case err != nil:
				t.Errorf("err = %v", err)
			}
		})
	}
}

func TestBGPVIPProbeStopScript(t *testing.T) {
	want := `if [ -f /tmp/bgp-vip-e2e-api/pid ]; then pid=$(cat /tmp/bgp-vip-e2e-api/pid); kill -TERM "$pid" 2>/dev/null || true; sleep 4; kill -TERM -- "-$pid" 2>/dev/null || true; rm -f /tmp/bgp-vip-e2e-api/pid; fi
cat /tmp/bgp-vip-e2e-api/log 2>/dev/null || true`
	if got := bgpVIPProbeStopScript("/tmp/bgp-vip-e2e-api", false); got != want {
		t.Errorf("the API probe's stop script changed:\n%s", got)
	}

	// Stop a running probe loop and remove its files, as the ingress VIP
	// move does.
	if _, err := exec.LookPath("setsid"); err != nil {
		t.Skip("no setsid")
	}
	dir := t.TempDir() + "/probe"
	setup := fmt.Sprintf("mkdir -p %[1]s; echo '100.0 192.168.111.5 200 0' > %[1]s/log; setsid bash -c 'while :; do sleep 0.1; done' > /dev/null 2>&1 < /dev/null & echo $! > %[1]s/pid; cat %[1]s/pid", dir)
	out, err := runBGPVIPTestScript(t, nil, setup)
	if err != nil {
		t.Fatal(err)
	}
	pid := strings.TrimSpace(out)
	pgid, err := strconv.Atoi(pid)
	if err != nil {
		t.Fatalf("no loop pid in %q: %v", out, err)
	}
	// The loop runs in its own session: kill it whatever happens below.
	t.Cleanup(func() { killBGPVIPTestProcess(t, -pgid, "probe loop group") })
	out, err = runBGPVIPTestScript(t, nil, bgpVIPProbeStopScript(dir, true))
	if err != nil {
		t.Fatal(err)
	}
	if samples := parseBGPVIPProbeLog(out); len(samples) != 1 || samples[0].Request != 0 {
		t.Errorf("samples %+v from %q", samples, out)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("the probe's files remain: %v", err)
	}
	if _, err := runBGPVIPTestScript(t, nil, "kill -0 "+pid); err == nil {
		t.Errorf("the probe loop %s still runs", pid)
		killBGPVIPTestProcess(t, -pgid, "probe loop group")
	}
}

func TestDescribeBGPVIPProbeSamples(t *testing.T) {
	got := describeBGPVIPProbeSamples([]bgpVIPProbeSample{
		{Time: 1.0, VIP: "192.168.111.5", OK: true},
		{Time: 1.5, VIP: "192.168.111.5"},
		{Time: 2.5, VIP: "192.168.111.5"},
		{Time: 3.5, VIP: "192.168.111.5"},
		{Time: 1.2, VIP: "fd2e::5", OK: true},
	}, 2)
	want := []string{
		"ingress probe via 192.168.111.5 (ipv4): 4 requests, 3 failed, failed requests started at 1.500, 2.500 and 1 more (hypervisor clock)",
		"ingress probe via fd2e::5 (ipv6): 1 requests, 0 failed",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := describeBGPVIPProbeSamples(nil, 2); len(got) != 0 {
		t.Errorf("no sample: %q", got)
	}
}

// runBGPVIPTestScript runs script as the hypervisor would, with the fake
// commands in bin, if any, ahead of PATH.
func runBGPVIPTestScript(t *testing.T, bin map[string]string, script string) (string, error) {
	t.Helper()
	path := os.Getenv("PATH")
	if len(bin) > 0 {
		dir := t.TempDir()
		for name, content := range bin {
			if err := os.WriteFile(dir+"/"+name, []byte("#!/bin/bash\n"+content), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		path = dir + ":" + path
	}
	return runBGPVIPBash(t, bgpVIPTestScriptTimeout, path, "set -o errexit -o nounset -o pipefail\n"+script)
}

// bgpVIPTestScriptTimeout bounds a shell script of these tests; it leaves
// ample room for the probe stop script's 4s drain.
const bgpVIPTestScriptTimeout = 30 * time.Second

// runBGPVIPBash runs script with bash in its own process group, with path as
// PATH, and kills what is left of that group once the script exits or
// outlives timeout, so that no background child keeps running. The output
// goes to files rather than pipes, so that a child holding it cannot delay
// the return. A timeout error says that the script timed out; a failure to
// kill the group is reported after the script's own error.
func runBGPVIPBash(t *testing.T, timeout time.Duration, path, script string) (string, error) {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash")
	}
	dir := t.TempDir()
	stdout, err := os.Create(dir + "/stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()
	stderr, err := os.Create(dir + "/stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bash, "-c", script)
	cmd.Env = append(os.Environ(), "PATH="+path)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return killBGPVIPProcessGroup(cmd.Process) }
	cmd.WaitDelay = time.Second
	cmd.Stdout, cmd.Stderr = stdout, stderr
	runErr := cmd.Run()
	killErr := killBGPVIPProcessGroup(cmd.Process)
	out, err := os.ReadFile(stdout.Name())
	if err != nil {
		t.Fatal(err)
	}
	errOut, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatal(err)
	}
	var scriptErr error
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		scriptErr = fmt.Errorf("the script did not finish within %v: %w: %s", timeout, ctx.Err(), errOut)
	case runErr != nil:
		scriptErr = fmt.Errorf("%v: %s", runErr, errOut)
	}
	if killErr != nil {
		scriptErr = errors.Join(scriptErr, fmt.Errorf("cannot kill what is left of the script: %w", killErr))
	}
	return string(out), scriptErr
}

// killBGPVIPProcessGroup kills the process group that p leads, if p started;
// a group that is gone already is no error.
func killBGPVIPProcessGroup(p *os.Process) error {
	if p == nil || p.Pid <= 1 {
		return nil
	}
	if err := syscall.Kill(-p.Pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

func TestRunBGPVIPBash(t *testing.T) {
	if out, err := runBGPVIPBash(t, 10*time.Second, os.Getenv("PATH"), "echo ok"); err != nil || out != "ok\n" {
		t.Errorf("got %q, %v", out, err)
	}
	if _, err := runBGPVIPBash(t, 10*time.Second, os.Getenv("PATH"), "echo broken >&2; exit 3"); err == nil || !strings.Contains(err.Error(), "exit status 3: broken") {
		t.Errorf("failing script: %v", err)
	}

	// A background child that holds stdout must neither keep the run going
	// past the timeout nor outlive it.
	start := time.Now()
	out, err := runBGPVIPBash(t, 500*time.Millisecond, os.Getenv("PATH"), "sleep 60 & echo $!; wait")
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "did not finish within 500ms") {
		t.Errorf("err = %v, want a timeout", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("the timed out script returned after %v", elapsed)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		t.Fatalf("no child pid in %q: %v", out, err)
	}
	if !bgpVIPProcessGone(pid, 2*time.Second) {
		t.Errorf("the background child %d outlived the timed out script", pid)
		killBGPVIPTestProcess(t, pid, "background child")
	}
}

// bgpVIPProcessGone reports whether pid exits, or is left a zombie for its
// new parent to reap, within the given time.
func bgpVIPProcessGone(pid int, within time.Duration) bool {
	deadline := time.Now().Add(within)
	for {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return true
		}
		if stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid)); err == nil {
			if i := strings.LastIndexByte(string(stat), ')'); i >= 0 && strings.HasPrefix(string(stat[i+1:]), " Z") {
				return true
			}
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// bgpVIPFakeSysctl keeps the sysctls in store, one file per key; a key with
// a .ro file cannot be written and a missing key does not exist.
func bgpVIPFakeSysctl(store string) string {
	return fmt.Sprintf(`store=%s
case "$1" in
-n) cat "$store/$2" 2>/dev/null || { echo "sysctl: cannot stat $2" >&2; exit 255; } ;;
-q) shift 2
  for kv in "$@"; do
    k=${kv%%%%=*}
    if [ ! -f "$store/$k" ] || [ -f "$store/$k.ro" ]; then echo "sysctl: cannot set $k" >&2; exit 255; fi
    echo "${kv#*=}" > "$store/$k"
  done ;;
*) exit 2 ;;
esac
`, store)
}

func TestBGPVIPHashPolicyScripts(t *testing.T) {
	const v4, v6 = "net.ipv4.fib_multipath_hash_policy", "net.ipv6.fib_multipath_hash_policy"
	for _, test := range []struct {
		name     string
		families []string
		// sysctls is the hypervisor's sysctls before the test, missing keys
		// do not exist; saved is a policy file left by an earlier run.
		sysctls, saved string
		readOnly       string
		setErr         bool
		afterSet       map[string]string
		restoreErr     bool
		afterRestore   map[string]string
	}{
		{
			name: "IPv4 on a host without IPv6", families: []string{"ipv4"}, sysctls: v4 + "=0",
			afterSet: map[string]string{v4: "1"}, afterRestore: map[string]string{v4: "0"},
		},
		{
			name: "dual-stack", families: []string{"ipv4", "ipv6"}, sysctls: v4 + "=0 " + v6 + "=0",
			afterSet: map[string]string{v4: "1", v6: "1"}, afterRestore: map[string]string{v4: "0", v6: "0"},
		},
		{
			name: "IPv6 only leaves IPv4 alone", families: []string{"ipv6"}, sysctls: v4 + "=0 " + v6 + "=2",
			afterSet: map[string]string{v4: "0", v6: "1"}, afterRestore: map[string]string{v4: "0", v6: "2"},
		},
		{
			name: "stops halfway and restores what it changed", families: []string{"ipv4", "ipv6"}, sysctls: v4 + "=0",
			setErr: true, afterSet: map[string]string{v4: "1"}, afterRestore: map[string]string{v4: "0"},
		},
		{
			name: "keeps the original of a run that could not restore it", families: []string{"ipv4"}, sysctls: v4 + "=1", saved: v4 + "=0\n",
			afterSet: map[string]string{v4: "1"}, afterRestore: map[string]string{v4: "0"},
		},
		{
			name: "restores every family and keeps the record when one fails", families: []string{"ipv4", "ipv6"}, sysctls: v4 + "=0 " + v6 + "=0", readOnly: v4,
			afterSet: map[string]string{v4: "1", v6: "1"}, restoreErr: true, afterRestore: map[string]string{v4: "1", v6: "0"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			store, file := dir+"/sysctl", dir+"/policy"
			if err := os.Mkdir(store, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, kv := range strings.Fields(test.sysctls) {
				k, v, _ := strings.Cut(kv, "=")
				if err := os.WriteFile(store+"/"+k, []byte(v+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if test.saved != "" {
				if err := os.WriteFile(file, []byte(test.saved), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			bin := map[string]string{"sysctl": bgpVIPFakeSysctl(store)}
			check := func(when string, want map[string]string) {
				t.Helper()
				for k, v := range want {
					got, err := os.ReadFile(store + "/" + k)
					if err != nil || strings.TrimSpace(string(got)) != v {
						t.Errorf("%s: %s = %q (%v), want %s", when, k, got, err, v)
					}
				}
			}
			_, err := runBGPVIPTestScript(t, bin, bgpVIPSetHashPolicyFamiliesScript(test.families, file))
			if (err != nil) != test.setErr {
				t.Fatalf("set: err = %v, want error %t", err, test.setErr)
			}
			check("after set", test.afterSet)
			if test.readOnly != "" {
				if err := os.WriteFile(store+"/"+test.readOnly+".ro", nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			_, err = runBGPVIPTestScript(t, bin, "rc=0\n"+bgpVIPRestoreHashPolicyFamiliesScript(file)+"exit \"$rc\"\n")
			if (err != nil) != test.restoreErr {
				t.Fatalf("restore: err = %v, want error %t", err, test.restoreErr)
			}
			check("after restore", test.afterRestore)
			if _, err := os.Stat(file); os.IsNotExist(err) == test.restoreErr {
				t.Errorf("policy file exists: %t, want %t", !os.IsNotExist(err), test.restoreErr)
			}
		})
	}
}

func TestBGPVIPIngressHostCleanupScript(t *testing.T) {
	const v4 = "net.ipv4.fib_multipath_hash_policy"
	for _, test := range []struct {
		name                         string
		pathsRecorded, hashPolicySet bool
		table, tableStuck            bool
		wantErr                      string
		wantTable                    bool
		wantV4                       string
	}{
		{name: "deletes the path records", pathsRecorded: true, table: true, wantV4: "1"},
		{name: "path records already gone", pathsRecorded: true, wantV4: "1"},
		{name: "restores the hash policy only", hashPolicySet: true, table: true, wantTable: true, wantV4: "0"},
		{name: "restores the hash policy when the path records cannot be deleted", pathsRecorded: true, hashPolicySet: true, table: true, tableStuck: true, wantErr: "cannot delete nft table", wantTable: true, wantV4: "0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			store, file := dir+"/sysctl", dir+"/policy"
			for _, f := range []struct{ path, content string }{{store + "/" + v4, "1\n"}, {file, v4 + "=0\n"}} {
				if err := os.MkdirAll(store, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(f.path, []byte(f.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if test.table {
				if err := os.WriteFile(dir+"/table", nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if test.tableStuck {
				if err := os.WriteFile(dir+"/stuck", nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			bin := map[string]string{
				"sysctl": bgpVIPFakeSysctl(store),
				"nft": fmt.Sprintf(`[ "$*" = "list table bridge bgp_vip_e2e" ] && { [ -f %[1]s/table ]; exit; }
[ "$*" = "delete table bridge bgp_vip_e2e" ] && [ ! -f %[1]s/stuck ] && rm %[1]s/table && exit 0
exit 1
`, dir),
			}
			script := strings.ReplaceAll(bgpVIPIngressHostCleanupScript(test.pathsRecorded, test.hashPolicySet), bgpVIPIngressHashPolicyFile, file)
			_, err := runBGPVIPTestScript(t, bin, script)
			if test.wantErr == "" && err != nil || test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Errorf("err = %v, want %q", err, test.wantErr)
			}
			if _, err := os.Stat(dir + "/table"); os.IsNotExist(err) == test.wantTable {
				t.Errorf("nft table exists: %t, want %t", !os.IsNotExist(err), test.wantTable)
			}
			if got, _ := os.ReadFile(store + "/" + v4); strings.TrimSpace(string(got)) != test.wantV4 {
				t.Errorf("%s = %q, want %s", v4, got, test.wantV4)
			}
		})
	}
	if script := bgpVIPIngressHostCleanupScript(true, false); strings.Contains(script, "sysctl") {
		t.Errorf("restores a hash policy it did not change:\n%s", script)
	}
}

// TestBGPVIPTorConvergence checks that a VIP whose ToR paths matched the
// router nodes of an earlier look is compared again when another router node
// becomes Ready, for either family, while its first convergence time stays.
func TestBGPVIPTorConvergence(t *testing.T) {
	const v4, v6 = "192.168.111.5", "fd2e::5"
	nodeIPs, paths := bgpVIPTestNodeIPs, bgpVIPTestTorPaths
	for _, stale := range []string{v4, v6} {
		t.Run(addressFamily(stale), func(t *testing.T) {
			other := v6
			if stale == v6 {
				other = v4
			}
			vips := []string{v4, v6}
			tor := newBGPVIPTorConvergence("worker-0", nodeIPs)

			// The stale VIP converges on worker-1 while the other still has
			// worker-0's path.
			err := tor.observe(vips, map[string][]string{stale: paths(stale, "worker-1"), other: paths(other, "worker-0", "worker-1")}, sets.New("worker-1"), 10, 11)
			if err == nil || !strings.Contains(err.Error(), other) {
				t.Errorf("look 1: err = %v, want %s pending", err, other)
			}

			// worker-2 became Ready: the other VIP has its path, the stale
			// one does not, and must not count as converged.
			want := sets.New("worker-1", "worker-2")
			err = tor.observe(vips, map[string][]string{stale: paths(stale, "worker-1"), other: paths(other, "worker-1", "worker-2")}, want, 20, 21)
			if err == nil || !strings.Contains(err.Error(), stale) || strings.Contains(err.Error(), "paths to "+other+" ") {
				t.Errorf("look 2: err = %v, want only %s pending", err, stale)
			}

			if err := tor.observe(vips, map[string][]string{stale: paths(stale, "worker-1", "worker-2"), other: paths(other, "worker-1", "worker-2")}, want, 30, 31); err != nil {
				t.Errorf("look 3: %v", err)
			}
			if got, want := tor.convergedBy, map[string]float64{stale: 11, other: 21}; !reflect.DeepEqual(got, want) {
				t.Errorf("first convergence %v, want %v", got, want)
			}
			if got, want := tor.victimSeen, map[string]float64{other: 10}; !reflect.DeepEqual(got, want) {
				t.Errorf("victim last seen %v, want %v", got, want)
			}
			if got, want := tor.victimGone, map[string]float64{stale: 11, other: 21}; !reflect.DeepEqual(got, want) {
				t.Errorf("victim gone %v, want %v", got, want)
			}
		})
	}

	tor := newBGPVIPTorConvergence("worker-0", nodeIPs)
	// The victim's path returns after it was gone: it is gone again only
	// from the next look without it.
	for i, look := range []struct {
		nodes   []string
		wantErr string
	}{
		{nodes: []string{"worker-1"}},
		{nodes: []string{"worker-0", "worker-1"}, wantErr: "the ToR has paths to " + v4 + " from [worker-0 worker-1]"},
		{nodes: []string{"worker-1"}},
	} {
		err := tor.observe([]string{v4}, map[string][]string{v4: paths(v4, look.nodes...)}, sets.New("worker-1"), float64(10*i), float64(10*i+1))
		if look.wantErr == "" && err != nil || look.wantErr != "" && (err == nil || !strings.Contains(err.Error(), look.wantErr)) {
			t.Errorf("look %d: err = %v, want %q", i+1, err, look.wantErr)
		}
	}
	if tor.victimSeen[v4] != 10 || tor.victimGone[v4] != 21 || tor.convergedBy[v4] != 1 {
		t.Errorf("seen %v gone %v converged %v", tor.victimSeen, tor.victimGone, tor.convergedBy)
	}
	if err := tor.observe([]string{v4, v6}, map[string][]string{v4: paths(v4, "worker-1")}, sets.New("worker-1"), 40, 41); err == nil || !strings.Contains(err.Error(), "no look at the ToR's paths to "+v6) {
		t.Errorf("missing look: %v", err)
	}
}

// bgpVIPTestNodeIPs are the InternalIPs of the nodes in the ToR convergence
// tests.
var bgpVIPTestNodeIPs = map[string]string{
	"192.168.111.20": "worker-0", "fd2e::20": "worker-0",
	"192.168.111.21": "worker-1", "fd2e::21": "worker-1",
	"192.168.111.22": "worker-2", "fd2e::22": "worker-2",
}

// bgpVIPTestTorPaths returns the ToR's path peers to vip from nodes, in the
// family of vip.
func bgpVIPTestTorPaths(vip string, nodes ...string) []string {
	var out []string
	for _, node := range nodes {
		for ip, name := range bgpVIPTestNodeIPs {
			if name == node && addressFamily(ip) == addressFamily(vip) {
				out = append(out, ip)
			}
		}
	}
	return out
}

// TestBGPVIPTorConvergenceVictimPathReturns keeps the expected router nodes
// the same in every look and checks that a VIP whose paths matched in an
// earlier look is pending again when the victim's path returns to it, so
// that the ToR converges only in a look where every VIP matches, for either
// family and VIP order, while the first convergence times stay.
func TestBGPVIPTorConvergenceVictimPathReturns(t *testing.T) {
	const v4, v6 = "192.168.111.5", "fd2e::5"
	const victim, survivor = "worker-0", "worker-1"
	only, both := []string{survivor}, []string{victim, survivor}
	for _, test := range []struct {
		name string
		// first matches in look 1 and gets the victim's path back in look
		// 2, when second matches.
		first, second string
		vips          []string
	}{
		{name: "IPv4 matches first", first: v4, second: v6, vips: []string{v4, v6}},
		{name: "IPv4 matches first, IPv6 listed first", first: v4, second: v6, vips: []string{v6, v4}},
		{name: "IPv6 matches first", first: v6, second: v4, vips: []string{v4, v6}},
		{name: "IPv6 matches first, IPv6 listed first", first: v6, second: v4, vips: []string{v6, v4}},
	} {
		t.Run(test.name, func(t *testing.T) {
			tor := newBGPVIPTorConvergence(victim, bgpVIPTestNodeIPs)
			look := func(firstNodes, secondNodes []string, before, after float64) error {
				return tor.observe(test.vips, map[string][]string{
					test.first:  bgpVIPTestTorPaths(test.first, firstNodes...),
					test.second: bgpVIPTestTorPaths(test.second, secondNodes...),
				}, sets.New(survivor), before, after)
			}
			pendingOnly := func(err error, pending, matched string) bool {
				return err != nil && strings.Contains(err.Error(), "paths to "+pending+" ") && !strings.Contains(err.Error(), "paths to "+matched+" ")
			}

			if err := look(only, both, 10, 11); !pendingOnly(err, test.second, test.first) {
				t.Errorf("look 1: err = %v, want only %s pending", err, test.second)
			}
			if err := look(both, only, 20, 21); !pendingOnly(err, test.first, test.second) {
				t.Errorf("look 2: err = %v, want only %s pending", err, test.first)
			}
			if err := look(only, only, 30, 31); err != nil {
				t.Errorf("look 3: %v", err)
			}
			if got, want := tor.convergedBy, map[string]float64{test.first: 11, test.second: 21}; !reflect.DeepEqual(got, want) {
				t.Errorf("first convergence %v, want %v", got, want)
			}
			if got, want := tor.victimSeen, map[string]float64{test.first: 20, test.second: 10}; !reflect.DeepEqual(got, want) {
				t.Errorf("victim last seen %v, want %v", got, want)
			}
			if got, want := tor.victimGone, map[string]float64{test.first: 31, test.second: 21}; !reflect.DeepEqual(got, want) {
				t.Errorf("victim gone %v, want %v", got, want)
			}
		})
	}
}

// TestRunBGPVIPBashKillsLeftoverChildren checks that a background child of a
// script that exits before its deadline does not outlive it, whether the
// child holds the script's output or not.
func TestRunBGPVIPBashKillsLeftoverChildren(t *testing.T) {
	for _, test := range []struct {
		name, script, wantErr string
	}{
		{name: "success with a child holding stdout", script: "sleep 60 & echo $!"},
		{name: "failure with a child holding stdout", script: "sleep 60 & echo $!; exit 3", wantErr: "exit status 3"},
		{name: "success with a redirected child", script: "sleep 60 > /dev/null 2>&1 < /dev/null & echo $!"},
		{name: "failure with a redirected child", script: "sleep 60 > /dev/null 2>&1 < /dev/null & echo $!; exit 3", wantErr: "exit status 3"},
	} {
		t.Run(test.name, func(t *testing.T) {
			start := time.Now()
			out, err := runBGPVIPBash(t, 10*time.Second, os.Getenv("PATH"), test.script)
			pid, perr := strconv.Atoi(strings.TrimSpace(out))
			if perr != nil {
				t.Fatalf("no child pid in %q (err %v): %v", out, err, perr)
			}
			t.Cleanup(func() { killBGPVIPTestProcess(t, pid, "background child") })
			if test.wantErr == "" && err != nil || test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Errorf("err = %v, want %q", err, test.wantErr)
			}
			if elapsed := time.Since(start); elapsed > 5*time.Second {
				t.Errorf("the script returned after %v", elapsed)
			}
			if !bgpVIPProcessGone(pid, 2*time.Second) {
				t.Errorf("the background child %d outlived the script", pid)
			}
		})
	}
}

// killBGPVIPTestProcess kills pid, or the process group -pid, and reports a
// failure other than the process being gone already. It refuses 0 and -1,
// which would signal the test's own process group or every process.
func killBGPVIPTestProcess(t *testing.T, pid int, what string) {
	t.Helper()
	if pid == 0 || pid == -1 {
		t.Errorf("refusing to kill %s %d", what, pid)
		return
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		t.Errorf("kill %s %d: %v", what, pid, err)
	}
}

func TestKillBGPVIPProcessGroup(t *testing.T) {
	if err := killBGPVIPProcessGroup(nil); err != nil {
		t.Errorf("a command that did not start: %v", err)
	}
	cmd := exec.Command("true")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Run(); err != nil {
		t.Skipf("cannot run true: %v", err)
	}
	if err := killBGPVIPProcessGroup(cmd.Process); err != nil {
		t.Errorf("the group of an exited command: %v", err)
	}
}

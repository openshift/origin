package networking

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

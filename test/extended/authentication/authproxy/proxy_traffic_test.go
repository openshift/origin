package authproxy

import (
	"testing"
)

func TestProxyTrafficMatches(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		logs      string
		sourceIPs []string
		destHost  string
		want      bool
	}{
		{name: "matching source", logs: "CONNECT idp.example:443 192.0.2.1 TCP_TUNNEL 200", want: true},
		{name: "matching destination", logs: "CONNECT idp.example:443 192.0.2.1 TCP_TUNNEL 200", destHost: "idp.example", want: true},
		{name: "different source", logs: "CONNECT idp.example:443 192.0.2.2 TCP_TUNNEL 200"},
		{name: "second source matches", logs: "CONNECT idp.example:443 192.0.2.2 TCP_TUNNEL 200", sourceIPs: []string{"192.0.2.1", "192.0.2.2"}, want: true},
		{name: "IPv6 source", logs: "CONNECT idp.example:443 2001:db8::1 TCP_TUNNEL 200", sourceIPs: []string{"2001:db8::1"}, want: true},
		{name: "IPv4 destination", logs: "CONNECT 192.0.2.10:443 192.0.2.1 TCP_TUNNEL 200", destHost: "192.0.2.10", want: true},
		{name: "IPv6 destination", logs: "CONNECT [2001:db8::2]:443 192.0.2.1 TCP_TUNNEL 200", destHost: "2001:db8::2", want: true},
		{name: "different IPv6 destination", logs: "CONNECT [2001:db8::3]:443 192.0.2.1 TCP_TUNNEL 200", destHost: "2001:db8::2"},
		{name: "different destination", logs: "CONNECT other.example:443 192.0.2.1 TCP_TUNNEL 200", destHost: "idp.example"},
		{name: "destination prefix is insufficient", logs: "CONNECT idp.example.other:443 192.0.2.1 TCP_TUNNEL 200", destHost: "idp.example"},
		{name: "subdomain is insufficient", logs: "CONNECT sub.idp.example:443 192.0.2.1 TCP_TUNNEL 200", destHost: "idp.example"},
		{name: "DNS case insensitive", logs: "CONNECT IDP.EXAMPLE:443 192.0.2.1 TCP_TUNNEL 200", destHost: "idp.example", want: true},
		{name: "fully qualified DNS", logs: "CONNECT idp.example.:443 192.0.2.1 TCP_TUNNEL 200", destHost: "idp.example", want: true},
		{name: "empty logs"},
		{name: "short line", logs: "CONNECT idp.example:443 192.0.2.1"},
		{name: "different method", logs: "GET idp.example:443 192.0.2.1 TCP_TUNNEL 200"},
		{name: "different status", logs: "CONNECT idp.example:443 192.0.2.1 TCP_DENIED 403"},
		{name: "invalid source", logs: "CONNECT idp.example:443 bad TCP_TUNNEL 200"},
		{name: "destination missing port", logs: "CONNECT idp.example 192.0.2.1 TCP_TUNNEL 200"},
		{name: "matching line after malformed line", logs: "noise\nCONNECT idp.example:443 192.0.2.1 TCP_TUNNEL 200\n", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sourceIPs := tc.sourceIPs
			if sourceIPs == nil {
				sourceIPs = []string{"192.0.2.1"}
			}
			q := proxyTrafficQuery{sourceIPs: sourceIPs, destHost: tc.destHost}
			if got := q.seenIn(tc.logs); got != tc.want {
				t.Errorf("seenIn() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProxyTrafficQueryMatcher(t *testing.T) {
	t.Parallel()
	const logs = "CONNECT idp.example:443 192.0.2.1 TCP_TUNNEL 200\nCONNECT other.example:443 192.0.2.9 TCP_TUNNEL 200\n"
	for _, tc := range []struct {
		name  string
		query proxyTrafficQuery
		logs  string
		want  bool
	}{
		{
			name:  "single source matches any destination",
			logs:  logs,
			query: proxyTrafficQuery{sourceIPs: []string{"192.0.2.1"}},
			want:  true,
		},
		{
			name:  "one of several sources matches",
			logs:  logs,
			query: proxyTrafficQuery{sourceIPs: []string{"192.0.2.2", "192.0.2.1"}},
			want:  true,
		},
		{
			name:  "no listed source matches",
			logs:  logs,
			query: proxyTrafficQuery{sourceIPs: []string{"203.0.113.1"}},
		},
		{
			name:  "source and destination match",
			logs:  logs,
			query: proxyTrafficQuery{sourceIPs: []string{"192.0.2.1"}, destHost: "idp.example"},
			want:  true,
		},
		{
			name:  "destination filters out the matching source",
			logs:  logs,
			query: proxyTrafficQuery{sourceIPs: []string{"192.0.2.1"}, destHost: "other.example"},
		},
		{
			name:  "empty query never matches",
			logs:  logs,
			query: proxyTrafficQuery{},
		},
		{
			name:  "match only in later logs",
			logs:  "noise\n" + logs,
			query: proxyTrafficQuery{sourceIPs: []string{"192.0.2.1"}, destHost: "idp.example"},
			want:  true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.query.seenIn(tc.logs); got != tc.want {
				t.Errorf("seenIn() = %v, want %v", got, tc.want)
			}
			// seenIn is stateless, so a repeated call must agree.
			if got := tc.query.seenIn(tc.logs); got != tc.want {
				t.Errorf("repeated seenIn() = %v, want %v", got, tc.want)
			}
		})
	}
}

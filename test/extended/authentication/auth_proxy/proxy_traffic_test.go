package auth_proxy

import (
	"context"
	"errors"
	"testing"
	"time"
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
			if got := proxyTrafficMatches(tc.logs, sourceIPs, tc.destHost); got != tc.want {
				t.Errorf("proxyTrafficMatches() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestWaitForProxyTrafficPolling(t *testing.T) {
	t.Parallel()
	readErr := errors.New("log read failed")
	for _, tc := range []struct {
		name         string
		firstError   bool
		firstMiss    bool
		alwaysError  bool
		neverMatches bool
		cancelOnRead bool
		wantErr      error
	}{
		{name: "immediate match"},
		{name: "read error followed by match", firstError: true},
		{name: "no match followed by match", firstMiss: true},
		{name: "timeout without match", neverMatches: true, wantErr: context.DeadlineExceeded},
		{name: "read error and timeout", alwaysError: true, wantErr: context.DeadlineExceeded},
		{name: "read error followed by nonmatching logs", firstError: true, neverMatches: true, wantErr: context.DeadlineExceeded},
		{name: "read error and cancellation", alwaysError: true, cancelOnRead: true, wantErr: context.Canceled},
		{name: "cancellation without read error", neverMatches: true, cancelOnRead: true, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			timeout := time.Second
			if tc.wantErr != nil {
				timeout = 50 * time.Millisecond
			}
			err := waitForProxyTraffic(ctx, timeout, time.Millisecond,
				func(ctx context.Context) (string, error) {
					calls++
					if tc.cancelOnRead {
						cancel()
					}
					if tc.alwaysError || (tc.firstError && calls == 1) {
						return "", readErr
					}
					if tc.neverMatches || (tc.firstMiss && calls == 1) {
						return "", nil
					}
					return "matching logs", nil
				}, func(logs string) bool { return logs == "matching logs" })
			if tc.wantErr == nil {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.Is(err, tc.wantErr) {
				t.Errorf("error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil && (tc.alwaysError || tc.firstError) && !errors.Is(err, readErr) {
				t.Errorf("error = %v, want original log read error", err)
			}
			if tc.neverMatches && !tc.firstError && errors.Is(err, readErr) {
				t.Error("timeout without read failures contains a log read error")
			}
			if tc.wantErr == nil && (tc.firstError || tc.firstMiss) && calls < 2 {
				t.Error("polling did not retry")
			}
		})
	}
}

func TestWaitForProxyTrafficPreservesLastReadError(t *testing.T) {
	t.Parallel()
	firstErr := errors.New("first read failure")
	lastErr := errors.New("last read failure")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	err := waitForProxyTraffic(ctx, time.Second, time.Millisecond,
		func(context.Context) (string, error) {
			calls++
			if calls == 1 {
				return "", firstErr
			}
			cancel()
			return "", lastErr
		}, func(string) bool {
			t.Error("log matching ran after a read error")
			return false
		})
	if !errors.Is(err, context.Canceled) || !errors.Is(err, lastErr) || errors.Is(err, firstErr) {
		t.Fatalf("error = %v, want cancellation and only the last read error", err)
	}
}

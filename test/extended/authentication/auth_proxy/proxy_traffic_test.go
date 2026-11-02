package auth_proxy

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"
)

func TestWaitForProxyTrafficValidation(t *testing.T) {
	t.Parallel()
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	// An unconfigured CLI must never reach client setup for invalid inputs.
	oc := &exutil.CLI{}
	for _, tc := range []struct {
		name      string
		component AuthenticationProxyComponent
		sourceIP  string
		timeout   time.Duration
		ctx       context.Context
		wantErr   error
		wantMsg   string
	}{
		{name: "nil CLI", wantErr: ErrInvalidCLI},
		{name: "missing namespace", component: AuthenticationProxyComponent{oc: oc}, wantErr: ErrMissingNamespace},
		{name: "empty source", sourceIP: "", timeout: time.Second, wantMsg: "invalid source IP"},
		{name: "source hostname", sourceIP: "source.example", timeout: time.Second, wantMsg: "invalid source IP"},
		{name: "source CIDR", sourceIP: "192.0.2.0/24", timeout: time.Second, wantMsg: "invalid source IP"},
		{name: "source whitespace", sourceIP: " 192.0.2.1 ", timeout: time.Second, wantMsg: "invalid source IP"},
		{name: "zero timeout", sourceIP: "192.0.2.1", wantMsg: "timeout must be greater than zero"},
		{name: "negative timeout", sourceIP: "192.0.2.1", timeout: -time.Second, wantMsg: "timeout must be greater than zero"},
		{name: "canceled context", sourceIP: "192.0.2.1", timeout: time.Second, ctx: canceledCtx, wantErr: context.Canceled},
	} {
		for _, method := range []string{"from", "from-to"} {
			t.Run(tc.name+"/"+method, func(t *testing.T) {
				t.Parallel()
				component := tc.component
				if tc.wantErr != ErrInvalidCLI && tc.wantErr != ErrMissingNamespace {
					component = AuthenticationProxyComponent{oc: oc, Namespace: "proxy-test"}
				}
				ctx := tc.ctx
				if ctx == nil {
					ctx = context.Background()
				}
				var err error
				if method == "from" {
					err = component.WaitForProxyTrafficFrom(ctx, tc.sourceIP, time.Time{}, tc.timeout)
				} else {
					err = component.WaitForProxyTrafficFromTo(ctx, []string{tc.sourceIP}, "idp.example", time.Time{}, tc.timeout)
				}
				if err == nil {
					t.Fatal("expected validation error")
				}
				if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
					t.Errorf("error = %v, want %v", err, tc.wantErr)
				}
				if tc.wantMsg != "" && !strings.Contains(err.Error(), tc.wantMsg) {
					t.Errorf("error = %v, want message containing %q", err, tc.wantMsg)
				}
			})
		}
	}
}

func TestWaitForProxyTrafficFromToValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		sourceIPs []string
		destHost  string
		wantMsg   string
	}{
		{name: "nil sources", destHost: "idp.example", wantMsg: "at least one source IP"},
		{name: "empty sources", sourceIPs: []string{}, destHost: "idp.example", wantMsg: "at least one source IP"},
		{name: "invalid second source", sourceIPs: []string{"192.0.2.1", "bad"}, destHost: "idp.example", wantMsg: "invalid source IP"},
		{name: "empty destination", destHost: ""},
		{name: "destination scheme", destHost: "https://idp.example"},
		{name: "destination path", destHost: "idp.example/path"},
		{name: "destination port", destHost: "idp.example:443"},
		{name: "IPv6 destination port", destHost: "[2001:db8::1]:443"},
		{name: "destination whitespace", destHost: " idp.example "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sourceIPs := tc.sourceIPs
			wantMsg := tc.wantMsg
			if wantMsg == "" {
				sourceIPs = []string{"192.0.2.1"}
				wantMsg = "destination must be a hostname or IP"
			}
			component := AuthenticationProxyComponent{oc: &exutil.CLI{}, Namespace: "proxy-test"}
			err := component.WaitForProxyTrafficFromTo(context.Background(), sourceIPs, tc.destHost, time.Time{}, time.Second)
			if err == nil || !strings.Contains(err.Error(), wantMsg) {
				t.Fatalf("error = %v, want message containing %q", err, wantMsg)
			}
		})
	}
}

func TestValidateProxyTrafficWaitValidIPs(t *testing.T) {
	t.Parallel()
	component := AuthenticationProxyComponent{oc: &exutil.CLI{}, Namespace: "proxy-test"}
	if err := component.validateProxyTrafficWait(context.Background(), []string{"192.0.2.1", "2001:db8::1"}, time.Second); err != nil {
		t.Fatal(err)
	}
}

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
		{name: "equivalent IPv6 source", logs: "CONNECT idp.example:443 2001:db8::1 TCP_TUNNEL 200", sourceIPs: []string{"2001:0db8:0:0:0:0:0:1"}, want: true},
		{name: "IPv4 destination", logs: "CONNECT 192.0.2.10:443 192.0.2.1 TCP_TUNNEL 200", destHost: "192.0.2.10", want: true},
		{name: "IPv6 destination", logs: "CONNECT [2001:db8::2]:443 192.0.2.1 TCP_TUNNEL 200", destHost: "2001:db8::2", want: true},
		{name: "equivalent IPv6 destination", logs: "CONNECT [2001:db8::2]:443 192.0.2.1 TCP_TUNNEL 200", destHost: "2001:0db8:0:0:0:0:0:2", want: true},
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
		{name: "destination missing host", logs: "CONNECT :443 192.0.2.1 TCP_TUNNEL 200"},
		{name: "invalid port", logs: "CONNECT idp.example:bad 192.0.2.1 TCP_TUNNEL 200"},
		{name: "zero port", logs: "CONNECT idp.example:0 192.0.2.1 TCP_TUNNEL 200"},
		{name: "port out of range", logs: "CONNECT idp.example:65536 192.0.2.1 TCP_TUNNEL 200"},
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

package authentication

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	configfake "github.com/openshift/client-go/config/clientset/versioned/fake"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func MustParse(url string) *url.URL {
	ret, err := ParseProxyField(url)
	if err != nil {
		panic("error parsing " + url + ": " + err.Error())
	}
	return ret
}

func TestParseProxyField(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, value string
		want        *url.URL
		wantErr     bool
	}{
		{name: "empty"},
		{name: "HTTP proxy", value: "http://a:3128", want: &url.URL{Scheme: "http", Host: "a:3128"}},
		{name: "HTTPS proxy", value: "https://b:3129", want: &url.URL{Scheme: "https", Host: "b:3129"}},
		{name: "malformed URL", value: "http://%zz", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseProxyField(tc.value)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseProxyField(%q) error = %v, want error = %v", tc.value, err, tc.wantErr)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParseProxyField(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestProxyConfigString(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		config ProxyConfig
		want   string
	}{
		{name: "empty", want: `ProxyConfig[HttpProxy="" HttpsProxy="" NoProxy=""]`},
		{
			name: "HTTP only", config: ProxyConfig{HttpProxy: MustParse("http://a")},
			want: `ProxyConfig[HttpProxy="http://a" HttpsProxy="" NoProxy=""]`,
		},
		{
			name: "HTTPS only", config: ProxyConfig{HttpsProxy: MustParse("https://b")},
			want: `ProxyConfig[HttpProxy="" HttpsProxy="https://b" NoProxy=""]`,
		},
		{
			name: "credentials removed", config: ProxyConfig{
				HttpProxy: MustParse("http://user:secret@a"), HttpsProxy: MustParse("https://other:password@b"), NoProxy: sets.New(".svc"),
			},
			want: `ProxyConfig[HttpProxy="http://a" HttpsProxy="https://b" NoProxy=".svc"]`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var beforeHTTP, beforeHTTPS string
			if tc.config.HttpProxy != nil {
				beforeHTTP = tc.config.HttpProxy.String()
			}
			if tc.config.HttpsProxy != nil {
				beforeHTTPS = tc.config.HttpsProxy.String()
			}
			if got := tc.config.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
			if tc.config.HttpProxy != nil && tc.config.HttpProxy.String() != beforeHTTP {
				t.Error("String() modified the HTTP proxy URL")
			}
			if tc.config.HttpsProxy != nil && tc.config.HttpsProxy.String() != beforeHTTPS {
				t.Error("String() modified the HTTPS proxy URL")
			}
		})
	}
}

func TestProxyEnvMatchesURL(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		value    string
		expected *url.URL
		want     bool
	}{
		{name: "empty env and nil URL", want: true},
		{name: "nonempty env and nil URL", value: "http://proxy:3128"},
		{name: "empty env and configured URL", expected: MustParse("http://proxy:3128")},
		{name: "matching URL", value: "http://proxy:3128", expected: MustParse("http://proxy:3128"), want: true},
		{name: "different URL", value: "http://other:3128", expected: MustParse("http://proxy:3128")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := proxyEnvMatchesURL(tc.value, tc.expected); got != tc.want {
				t.Errorf("proxyEnvMatchesURL(%q, %v) = %v, want %v", tc.value, tc.expected, got, tc.want)
			}
		})
	}
}

func TestProxyConfigValid(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		config ProxyConfig
		want   bool
	}{
		{
			name: "empty"},
		{
			name: "HTTP only",
			config: ProxyConfig{
				HttpProxy: MustParse("http://proxy:3128"),
			},
			want: true},
		{
			name: "HTTPS only",
			config: ProxyConfig{
				HttpsProxy: MustParse("http://proxy:3128"),
			},
			want: true},
		{
			name: "both URLs",
			config: ProxyConfig{
				HttpProxy:  MustParse("http://a"),
				HttpsProxy: MustParse("http://b"),
			},
			want: true},
		{
			name: "noProxy without URLs",
			config: ProxyConfig{
				NoProxy: sets.New(".svc"),
			},
		},
		{
			name: "CA without URLs",
			config: ProxyConfig{
				UpStreamCACertPEM: []byte("test CA"),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.config.Valid(); got != tc.want {
				t.Fatalf("Valid() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProxyConfigEqual(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		a, b ProxyConfig
		want bool
	}{
		{
			name: "empty",
			want: true,
		},
		{
			name: "nil and empty noProxy",
			b: ProxyConfig{
				NoProxy: sets.New[string](),
			},
			want: true,
		},
		{
			name: "identical URLs",
			a: ProxyConfig{
				HttpProxy:  MustParse("http://a"),
				HttpsProxy: MustParse("http://b"),
			},
			b: ProxyConfig{
				HttpProxy:  MustParse("http://a"),
				HttpsProxy: MustParse("http://b"),
			},
			want: true,
		},
		{
			name: "HTTP URL and nil",
			a:    ProxyConfig{HttpProxy: MustParse("http://a")},
		},
		{
			name: "HTTPS URL and nil",
			a:    ProxyConfig{HttpsProxy: MustParse("http://a")},
		},
		{
			name: "identical credentials",
			a:    ProxyConfig{HttpProxy: MustParse("http://user:secret@a")},
			b:    ProxyConfig{HttpProxy: MustParse("http://user:secret@a")},
			want: true,
		},
		{
			name: "different credentials",
			a:    ProxyConfig{HttpProxy: MustParse("http://user:secret@a")},
			b:    ProxyConfig{HttpProxy: MustParse("http://user:other@a")},
		},
		{
			name: "different HTTP URL",
			a: ProxyConfig{
				HttpProxy: MustParse("http://a"),
			},
			b: ProxyConfig{
				HttpProxy: MustParse("http://b"),
			},
		},
		{
			name: "different HTTPS URL",
			a: ProxyConfig{
				HttpsProxy: MustParse("http://a"),
			},
			b: ProxyConfig{
				HttpsProxy: MustParse("http://b"),
			},
		},
		{
			name: "noProxy order and duplicates",
			a: ProxyConfig{
				NoProxy: sets.New(".svc", "localhost"),
			},
			b: ProxyConfig{
				NoProxy: sets.New("localhost", ".svc", ".svc"),
			},
			want: true,
		},
		{
			name: "different noProxy",
			a: ProxyConfig{
				NoProxy: sets.New(".svc"),
			},
			b: ProxyConfig{
				NoProxy: sets.New("localhost"),
			},
		},
		{
			name: "additional noProxy entry",
			a: ProxyConfig{
				NoProxy: sets.New(".svc"),
			},
			b: ProxyConfig{
				NoProxy: sets.New(".svc", "localhost"),
			},
		},
		{
			name: "different CA ignored",
			a: ProxyConfig{
				UpStreamCACertPEM: []byte("CA A"),
			},
			b: ProxyConfig{
				UpStreamCACertPEM: []byte("CA B"),
			},
			want: true,
		},
		{
			name: "missing CA ignored",
			a: ProxyConfig{
				UpStreamCACertPEM: []byte("CA A"),
			},
			want: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.a.Equal(tc.b); got != tc.want {
				t.Errorf("a.Equal(b) = %v, want %v", got, tc.want)
			}
			if got := tc.b.Equal(tc.a); got != tc.want {
				t.Errorf("b.Equal(a) = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestUpstreamProxyConfig(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		proxy     *ProxyConfig
		wantLines []string
		wantPeers int
		wantCA    bool
		wantErr   string
	}{
		{name: "no upstream"},
		{name: "empty upstream rejected", proxy: &ProxyConfig{}, wantErr: "must be non-empty"},
		{
			name: "HTTP only", proxy: &ProxyConfig{HttpProxy: MustParse("http://a")}, wantPeers: 1,
			wantLines: []string{
				"cache_peer a parent 80 0 no-query name=upstream_http",
				"cache_peer_access upstream_http allow !upstream_connect",
				"cache_peer_access upstream_http deny all",
				"always_direct allow upstream_connect",
			},
		},
		{
			name: "HTTPS only with HTTP transport", proxy: &ProxyConfig{HttpsProxy: MustParse("http://b:3128")}, wantPeers: 1,
			wantLines: []string{
				"cache_peer b parent 3128 0 no-query name=upstream_https",
				"cache_peer_access upstream_https allow upstream_connect",
				"cache_peer_access upstream_https deny all",
				"always_direct allow !upstream_connect",
			},
		},
		{
			name: "distinct peers", proxy: &ProxyConfig{HttpProxy: MustParse("http://a:3128"), HttpsProxy: MustParse("http://b:8080")}, wantPeers: 2,
			wantLines: []string{
				"cache_peer a parent 3128 0 no-query name=upstream_http",
				"cache_peer b parent 8080 0 no-query name=upstream_https",
				"cache_peer_access upstream_http allow !upstream_connect",
				"cache_peer_access upstream_http deny all",
				"cache_peer_access upstream_https allow upstream_connect",
				"cache_peer_access upstream_https deny all",
			},
		},
		{
			name: "shared peer", proxy: &ProxyConfig{HttpProxy: MustParse("http://a"), HttpsProxy: MustParse("http://a")}, wantPeers: 1,
			wantLines: []string{"cache_peer a parent 80 0 no-query name=upstream_http", "cache_peer_access upstream_http allow all", "cache_peer_access upstream_http deny all"},
		},
		{
			name: "TLS for HTTP destinations without custom CA", proxy: &ProxyConfig{HttpProxy: MustParse("https://a")}, wantPeers: 1,
			wantLines: []string{"cache_peer a parent 443 0 no-query name=upstream_http tls"},
		},
		{
			name: "TLS for HTTPS destinations with empty CA", proxy: &ProxyConfig{HttpsProxy: MustParse("https://b:3129"), UpStreamCACertPEM: []byte{}}, wantPeers: 1,
			wantLines: []string{"cache_peer b parent 3129 0 no-query name=upstream_https tls"},
		},
		{
			name: "CA for TLS HTTP peer only", proxy: &ProxyConfig{HttpProxy: MustParse("https://a"), HttpsProxy: MustParse("http://b"), UpStreamCACertPEM: []byte("test CA")}, wantPeers: 2, wantCA: true,
			wantLines: []string{"cache_peer a parent 443 0 no-query name=upstream_http tls tls-cafile=" + UpstreamCACertPath, "cache_peer b parent 80 0 no-query name=upstream_https"},
		},
		{
			name: "CA for TLS HTTPS peer only", proxy: &ProxyConfig{HttpProxy: MustParse("http://a"), HttpsProxy: MustParse("https://b"), UpStreamCACertPEM: []byte("test CA")}, wantPeers: 2, wantCA: true,
			wantLines: []string{"cache_peer a parent 80 0 no-query name=upstream_http", "cache_peer b parent 443 0 no-query name=upstream_https tls tls-cafile=" + UpstreamCACertPath},
		},
		{
			name: "CA for both TLS peers", proxy: &ProxyConfig{HttpProxy: MustParse("https://a"), HttpsProxy: MustParse("https://b"), UpStreamCACertPEM: []byte("test CA")}, wantPeers: 2, wantCA: true,
			wantLines: []string{"cache_peer a parent 443 0 no-query name=upstream_http tls tls-cafile=" + UpstreamCACertPath, "cache_peer b parent 443 0 no-query name=upstream_https tls tls-cafile=" + UpstreamCACertPath},
		},
		{
			name: "shared TLS peer preserves CA", proxy: &ProxyConfig{HttpProxy: MustParse("https://a"), HttpsProxy: MustParse("https://a"), UpStreamCACertPEM: []byte("test CA")}, wantPeers: 1, wantCA: true,
			wantLines: []string{"cache_peer a parent 443 0 no-query name=upstream_http tls tls-cafile=" + UpstreamCACertPath, "cache_peer_access upstream_http allow all"},
		},
		{
			name: "CA unused for HTTP transport", proxy: &ProxyConfig{HttpsProxy: MustParse("http://b"), UpStreamCACertPEM: []byte("test CA")}, wantPeers: 1,
			wantLines: []string{"cache_peer b parent 80 0 no-query name=upstream_https"},
		},
		{
			name: "escaped credentials", proxy: &ProxyConfig{HttpProxy: MustParse("http://user%20name:p%40ss%20word@a:3128")}, wantPeers: 1,
			wantLines: []string{"cache_peer a parent 3128 0 no-query name=upstream_http login=user%20name:p%40ss%20word"},
		},
		{name: "missing hostname", proxy: &ProxyConfig{HttpProxy: MustParse("http:///")}, wantErr: "upstream_http"},
		{name: "unsupported scheme", proxy: &ProxyConfig{HttpsProxy: MustParse("socks5://b")}, wantErr: "upstream_https"},
		{name: "invalid second peer", proxy: &ProxyConfig{HttpProxy: MustParse("http://a"), HttpsProxy: MustParse("invalid")}, wantErr: "upstream_https"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := (AuthProxyComponent{ClusterProxy: tc.proxy}).upstreamProxyConfig()
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want error containing %q", err, tc.wantErr)
				}
				if got.squidConfig != "" || got.upstreamCACertPath != "" || len(got.upstreamCACertPEM) != 0 {
					t.Fatalf("expected empty result on error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.proxy == nil {
				if got.squidConfig != "" || got.upstreamCACertPath != "" || len(got.upstreamCACertPEM) != 0 {
					t.Fatalf("expected empty result without upstream, got %+v", got)
				}
				return
			}
			lines := strings.Split(strings.TrimSpace(got.squidConfig), "\n")
			for _, line := range append(tc.wantLines, "acl upstream_connect method CONNECT", "never_direct allow all") {
				if !slices.Contains(lines, line) {
					t.Errorf("missing directive %q in:\n%s", line, got.squidConfig)
				}
			}
			if count := strings.Count(got.squidConfig, "cache_peer "); count != tc.wantPeers {
				t.Errorf("got %d peers, want %d", count, tc.wantPeers)
			}
			if tc.proxy.HttpProxy != nil && tc.proxy.HttpsProxy != nil && strings.Contains(got.squidConfig, "always_direct ") {
				t.Error("traffic must not bypass configured peers")
			}
			wantPath := ""
			if tc.wantCA {
				wantPath = UpstreamCACertPath
			} else if strings.Contains(got.squidConfig, "tls-cafile=") {
				t.Error("unexpected custom CA reference")
			}
			if got.upstreamCACertPath != wantPath {
				t.Errorf("CA path = %q, want %q", got.upstreamCACertPath, wantPath)
			}
			if !bytes.Equal(got.upstreamCACertPEM, tc.proxy.UpStreamCACertPEM) {
				t.Error("upstream CA bytes were not preserved")
			}
		})
	}
}

func TestUpstreamProxyConfigNoProxy(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, entry, wantACL string
	}{
		{name: "bare domain", entry: "example.com", wantACL: "dstdomain -n .example.com"},
		{name: "domain suffix", entry: ".example.com", wantACL: "dstdomain -n .example.com"},
		{name: "wildcard domain", entry: "*.example.com", wantACL: "dstdomain -n .example.com"},
		{name: "whitespace", entry: " .svc ", wantACL: "dstdomain -n .svc"},
		{name: "IPv4", entry: "192.0.2.1", wantACL: "dst 192.0.2.1"},
		{name: "IPv6", entry: "2001:db8::1", wantACL: "dst 2001:db8::1"},
		{name: "CIDR", entry: "10.0.0.0/8", wantACL: "dst 10.0.0.0/8"},
		{name: "IPv6 CIDR", entry: "2001:db8::/32", wantACL: "dst 2001:db8::/32"},
		{name: "all destinations", entry: "*"},
		{name: "empty", entry: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := (AuthProxyComponent{ClusterProxy: &ProxyConfig{
				HttpProxy: MustParse("http://a"), HttpsProxy: MustParse("http://b"), NoProxy: sets.New(tc.entry),
			}}).upstreamProxyConfig()
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(got.squidConfig), "\n")
			if tc.wantACL != "" {
				if !slices.Contains(lines, "acl global_no_proxy_0 "+tc.wantACL) || !slices.Contains(lines, "always_direct allow global_no_proxy_0") {
					t.Fatalf("missing bypass ACL and rule in:\n%s", got.squidConfig)
				}
			} else {
				if strings.Contains(got.squidConfig, "global_no_proxy_") {
					t.Errorf("unexpected destination ACL in:\n%s", got.squidConfig)
				}
				if direct := slices.Contains(lines, "always_direct allow all"); direct != (tc.entry == "*") {
					t.Errorf("wildcard bypass = %v, want %v", direct, tc.entry == "*")
				}
			}
			if lines[len(lines)-1] != "never_direct allow all" {
				t.Error("expected direct-access restriction after bypass rules")
			}
		})
	}
}

func TestUpstreamProxyConfigCombinedBypass(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, httpProxy, httpsProxy, directRule string
	}{
		{name: "HTTP only", httpProxy: "http://a", directRule: "always_direct allow upstream_connect"},
		{name: "HTTPS only", httpsProxy: "http://b", directRule: "always_direct allow !upstream_connect"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := (AuthProxyComponent{ClusterProxy: &ProxyConfig{
				HttpProxy: MustParse(tc.httpProxy), HttpsProxy: MustParse(tc.httpsProxy),
				NoProxy: sets.New(".svc", "10.0.0.0/8", "192.0.2.1"),
			}}).upstreamProxyConfig()
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(got.squidConfig), "\n")
			if !slices.Contains(lines, tc.directRule) || lines[len(lines)-1] != "never_direct allow all" {
				t.Fatalf("unexpected direct-access rules:\n%s", got.squidConfig)
			}
			wantACLs := sets.New("dstdomain -n .svc", "dst 10.0.0.0/8", "dst 192.0.2.1")
			actualACLs := sets.New[string]()
			for i, line := range lines {
				if !strings.HasPrefix(line, "acl global_no_proxy_") {
					continue
				}
				fields := strings.Fields(line)
				actualACLs.Insert(strings.Join(fields[2:], " "))
				if i+1 >= len(lines) || lines[i+1] != "always_direct allow "+fields[1] {
					t.Errorf("missing bypass rule for %q", line)
				}
			}
			if !actualACLs.Equal(wantACLs) {
				t.Errorf("bypass ACLs = %v, want %v", actualACLs, wantACLs)
			}
		})
	}
}

func TestReadClusterProxyConfig(t *testing.T) {
	t.Parallel()
	readErr := errors.New("injected API failure")
	for _, tc := range []struct {
		name         string
		status       configv1.ProxyStatus
		trustedCA    string
		caData       map[string]string
		omitProxy    bool
		failResource string
		want         ProxyConfig
		wantErr      string
	}{
		{name: "no proxy"},
		{
			name:   "effective URLs from status",
			status: configv1.ProxyStatus{HTTPProxy: "http://a:3128", HTTPSProxy: "https://b:3129"},
			want:   ProxyConfig{HttpProxy: MustParse("http://a:3128"), HttpsProxy: MustParse("https://b:3129")},
		},
		{
			name:   "trim and deduplicate noProxy",
			status: configv1.ProxyStatus{NoProxy: " , .svc, localhost, .svc, , "},
			want:   ProxyConfig{NoProxy: sets.New(".svc", "localhost")},
		},
		{
			name: "read CA from openshift-config", trustedCA: "custom-proxy-ca",
			caData: map[string]string{"ca-bundle.crt": "test CA"},
			want:   ProxyConfig{UpStreamCACertPEM: []byte("test CA")},
		},
		{name: "missing proxy", omitProxy: true, wantErr: "fetching cluster proxy configuration"},
		{name: "proxy API failure", failResource: "proxies", wantErr: "fetching cluster proxy configuration"},
		{name: "missing CA configmap", trustedCA: "missing-ca", wantErr: "fetching cluster proxy CA certificate config map"},
		{name: "CA API failure", trustedCA: "custom-proxy-ca", failResource: "configmaps", wantErr: "fetching cluster proxy CA certificate config map"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			configClient := configfake.NewSimpleClientset()
			if !tc.omitProxy {
				if err := configClient.Tracker().Add(&configv1.Proxy{
					ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
					Spec: configv1.ProxySpec{
						HTTPProxy: "http://unused-spec-proxy", HTTPSProxy: "http://unused-spec-proxy", NoProxy: "unused.example",
						TrustedCA: configv1.ConfigMapNameReference{Name: tc.trustedCA},
					},
					Status: tc.status,
				}); err != nil {
					t.Fatal(err)
				}
			}
			kubeClient := kubefake.NewSimpleClientset()
			if tc.caData != nil {
				if err := kubeClient.Tracker().Add(&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: tc.trustedCA, Namespace: "openshift-config"}, Data: tc.caData,
				}); err != nil {
					t.Fatal(err)
				}
			}
			failGet := func(clienttesting.Action) (bool, runtime.Object, error) { return true, nil, readErr }
			if tc.failResource == "proxies" {
				configClient.PrependReactor("get", "proxies", failGet)
			} else if tc.failResource == "configmaps" {
				kubeClient.PrependReactor("get", "configmaps", failGet)
			}

			got, err := ClusterProxy(context.Background(), configClient.ConfigV1().Proxies(), kubeClient.CoreV1())
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want error containing %q", err, tc.wantErr)
				}
				if tc.failResource != "" && !errors.Is(err, readErr) {
					t.Errorf("API error was not preserved: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			assertReadProxyConfig(t, got, tc.want)
			if tc.trustedCA == "" && len(kubeClient.Actions()) != 0 {
				t.Error("unexpected Kubernetes API request without a trustedCA reference")
			}
		})
	}
}

func TestReadOAuthServerProxyConfig(t *testing.T) {
	t.Parallel()
	readErr := errors.New("injected API failure")
	for _, tc := range []struct {
		name          string
		env           []corev1.EnvVar
		volumes       []corev1.Volume
		caData        map[string]string
		omitContainer bool
		omitDeploy    bool
		failResource  string
		want          ProxyConfig
		wantErr       string
	}{
		{name: "no proxy"},
		{
			name: "proxy environment and normalized exclusions",
			env: []corev1.EnvVar{
				{Name: "HTTP_PROXY", Value: "http://a:3128"}, {Name: "HTTPS_PROXY", Value: "https://b:3129"},
				{Name: "NO_PROXY", Value: " .svc, localhost, , .svc, "}, {Name: "UNRELATED", Value: "ignored"},
			},
			want: ProxyConfig{HttpProxy: MustParse("http://a:3128"), HttpsProxy: MustParse("https://b:3129"), NoProxy: sets.New(".svc", "localhost")},
		},
		{name: "empty noProxy", env: []corev1.EnvVar{{Name: "NO_PROXY", Value: " , "}}},
		{
			name: "CA and environment returned together",
			env:  []corev1.EnvVar{{Name: "HTTPS_PROXY", Value: "https://b:3129"}},
			volumes: []corev1.Volume{{
				Name: componentProxyCAConfigMapName,
				VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "mounted-proxy-ca"},
				}},
			}},
			caData: map[string]string{"ca-bundle.crt": "test CA"},
			want:   ProxyConfig{HttpsProxy: MustParse("https://b:3129"), UpStreamCACertPEM: []byte("test CA")},
		},
		{
			name: "ignore unrelated CA volume",
			volumes: []corev1.Volume{{Name: "unrelated", VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "missing-ca"}},
			}}},
		},
		{
			name: "ignore non-configmap volume",
			volumes: []corev1.Volume{{Name: componentProxyCAConfigMapName, VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "unrelated"},
			}}},
		},
		{name: "missing container", omitContainer: true, wantErr: "oauth-openshift container not found"},
		{name: "missing deployment", omitDeploy: true, wantErr: "not found"},
		{name: "deployment API failure", failResource: "deployments", wantErr: "injected API failure"},
		{
			name: "missing CA configmap",
			volumes: []corev1.Volume{{Name: componentProxyCAConfigMapName, VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "missing-ca"}},
			}}},
			wantErr: "oauth-openshift proxy CA certificate",
		},
		{
			name: "CA API failure", failResource: "configmaps",
			volumes: []corev1.Volume{{Name: componentProxyCAConfigMapName, VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "mounted-proxy-ca"}},
			}}},
			wantErr: "oauth-openshift proxy CA certificate",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "oauth-openshift", Namespace: "openshift-authentication"},
			}
			deployment.Spec.Template.Spec.Volumes = tc.volumes
			deployment.Spec.Template.Spec.Containers = []corev1.Container{{
				Name: "sidecar", Env: []corev1.EnvVar{{Name: "HTTP_PROXY", Value: "http://wrong-proxy"}},
			}}
			if !tc.omitContainer {
				deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, corev1.Container{
					Name: "oauth-openshift", Env: tc.env,
				})
			}
			// A later sidecar must not override the OAuth server's configuration.
			deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, corev1.Container{
				Name: "another-sidecar", Env: []corev1.EnvVar{{Name: "HTTPS_PROXY", Value: "http://wrong-proxy"}},
			})
			client := kubefake.NewSimpleClientset()
			if !tc.omitDeploy {
				if err := client.Tracker().Add(deployment); err != nil {
					t.Fatal(err)
				}
			}
			if tc.caData != nil {
				if err := client.Tracker().Add(&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "mounted-proxy-ca", Namespace: "openshift-authentication"}, Data: tc.caData,
				}); err != nil {
					t.Fatal(err)
				}
			}
			if tc.failResource != "" {
				client.PrependReactor("get", tc.failResource, func(clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, readErr
				})
			}
			got, err := OAuthServerProxyConfig(context.Background(), client.AppsV1(), client.CoreV1())
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want error containing %q", err, tc.wantErr)
				}
				if tc.failResource != "" && !errors.Is(err, readErr) {
					t.Errorf("API error was not preserved: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			assertReadProxyConfig(t, got, tc.want)
			if tc.caData == nil {
				for _, action := range client.Actions() {
					if action.GetResource().Resource == "configmaps" {
						t.Error("unexpected configmap lookup without a proxy CA volume")
					}
				}
			}
		})
	}
}

func assertReadProxyConfig(t *testing.T, got, want ProxyConfig) {
	t.Helper()
	if !reflect.DeepEqual(got.HttpProxy, want.HttpProxy) || !reflect.DeepEqual(got.HttpsProxy, want.HttpsProxy) || !got.NoProxy.Equal(want.NoProxy) {
		t.Errorf("proxy configuration = %+v, want %+v", got, want)
	}
	if !bytes.Equal(got.UpStreamCACertPEM, want.UpStreamCACertPEM) {
		t.Errorf("CA PEM = %q, want %q", got.UpStreamCACertPEM, want.UpStreamCACertPEM)
	}
}

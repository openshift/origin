package auth_proxy

import (
	"context"
	"errors"
	"net/url"
	"reflect"
	"strings"
	"testing"

	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestDeployAuthProxyComponentValidation(t *testing.T) {
	t.Parallel()
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	// An unconfigured CLI ensures invalid inputs are rejected before client setup.
	oc := &exutil.CLI{}
	for _, tc := range []struct {
		name    string
		ctx     context.Context
		oc      *exutil.CLI
		config  AuthenticationProxyConfig
		wantErr error
		wantMsg string
	}{
		{name: "nil CLI", wantErr: ErrInvalidCLI},
		{name: "canceled context", ctx: canceledCtx, oc: oc, wantErr: context.Canceled},
		{
			name: "empty upstream", oc: oc,
			config:  AuthenticationProxyConfig{ClusterProxy: &ProxyConfig{}},
			wantMsg: "must be non-empty",
		},
		{
			name: "upstream missing hostname", oc: oc,
			config:  AuthenticationProxyConfig{ClusterProxy: &ProxyConfig{HttpProxy: &url.URL{Scheme: "http"}}},
			wantMsg: "upstream_http must be a valid HTTP or HTTPS proxy URL",
		},
		{
			name: "unsupported upstream scheme", oc: oc,
			config:  AuthenticationProxyConfig{ClusterProxy: &ProxyConfig{HttpsProxy: &url.URL{Scheme: "socks5", Host: "proxy"}}},
			wantMsg: "upstream_https must be a valid HTTP or HTTPS proxy URL",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := tc.ctx
			if ctx == nil {
				ctx = context.Background()
			}
			component, err := tc.config.DeployAuthProxyComponent(ctx, tc.oc)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantMsg != "" && !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error = %v, want message containing %q", err, tc.wantMsg)
			}
			if !reflect.DeepEqual(component, AuthenticationProxyComponent{}) {
				t.Errorf("validation failure returned a nonzero component: %v", component)
			}
		})
	}
}

func TestAuthenticationProxyComponentString(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		component AuthenticationProxyComponent
		want      string
	}{
		{
			name: "empty",
			want: `AuthenticationProxy[Namespace="" HttpProxy="" HttpsProxy="" CACertLen=0]`,
		},
		{
			name:      "namespace only",
			component: AuthenticationProxyComponent{Namespace: "proxy-test"},
			want:      `AuthenticationProxy[Namespace="proxy-test" HttpProxy="" HttpsProxy="" CACertLen=0]`,
		},
		{
			name:      "HTTP only",
			component: AuthenticationProxyComponent{HttpUrl: MustParse("http://proxy:3128")},
			want:      `AuthenticationProxy[Namespace="" HttpProxy="http://proxy:3128" HttpsProxy="" CACertLen=0]`,
		},
		{
			name:      "HTTPS only",
			component: AuthenticationProxyComponent{HttpsUrl: MustParse("https://proxy:3129")},
			want:      `AuthenticationProxy[Namespace="" HttpProxy="" HttpsProxy="https://proxy:3129" CACertLen=0]`,
		},
		{
			name: "complete component with credentials",
			component: AuthenticationProxyComponent{
				Namespace: "proxy-test", HttpUrl: MustParse("http://user:secret@proxy:3128"),
				HttpsUrl: MustParse("https://other:password@proxy:3129"), CACertPEM: []byte("test CA"),
			},
			want: `AuthenticationProxy[Namespace="proxy-test" HttpProxy="http://proxy:3128" HttpsProxy="https://proxy:3129" CACertLen=7]`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			before := tc.component
			if before.HttpUrl != nil {
				urlCopy := *before.HttpUrl
				before.HttpUrl = &urlCopy
			}
			if before.HttpsUrl != nil {
				urlCopy := *before.HttpsUrl
				before.HttpsUrl = &urlCopy
			}
			before.CACertPEM = append([]byte(nil), before.CACertPEM...)
			if got := tc.component.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
			if !reflect.DeepEqual(tc.component, before) {
				t.Error("String() modified the component")
			}
		})
	}
}

func TestRotateProxyCertificatesValidation(t *testing.T) {
	t.Parallel()
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		name      string
		component *AuthenticationProxyComponent
		ctx       context.Context
		wantErr   error
	}{
		{name: "nil receiver", wantErr: ErrInvalidCLI},
		{name: "zero component", component: &AuthenticationProxyComponent{}, wantErr: ErrInvalidCLI},
		{
			name: "missing CLI with existing CA",
			component: &AuthenticationProxyComponent{
				Namespace: "proxy-test", CACertPEM: []byte("existing CA"),
			},
			wantErr: ErrInvalidCLI,
		},
		{
			name:      "missing namespace",
			component: &AuthenticationProxyComponent{oc: &exutil.CLI{}, CACertPEM: []byte("existing CA")},
			wantErr:   ErrMissingNamespace,
		},
		{
			name:      "canceled context",
			component: &AuthenticationProxyComponent{oc: &exutil.CLI{}, Namespace: "proxy-test", CACertPEM: []byte("existing CA")},
			ctx:       canceledCtx, wantErr: context.Canceled,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var before AuthenticationProxyComponent
			if tc.component != nil {
				before = *tc.component
				before.CACertPEM = append([]byte(nil), tc.component.CACertPEM...)
			}
			ctx := tc.ctx
			if ctx == nil {
				ctx = context.Background()
			}
			ca, err := tc.component.RotateProxyCertificates(ctx)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if ca != nil {
				t.Errorf("CA = %q, want nil", ca)
			}
			if tc.component != nil && !reflect.DeepEqual(*tc.component, before) {
				t.Error("validation failure modified the component")
			}
		})
	}
}

func TestCertificateRotationResources(t *testing.T) {
	t.Parallel()
	readErr := errors.New("injected API failure")
	validData := map[string][]byte{corev1.TLSCertKey: []byte("old cert"), corev1.TLSPrivateKeyKey: []byte("old key")}
	for _, tc := range []struct {
		name         string
		data         map[string][]byte
		omitSecret   bool
		omitPod      bool
		failResource string
		wantErr      string
	}{
		{name: "valid resources", data: validData},
		{name: "missing secret", omitSecret: true, wantErr: "error getting"},
		{name: "secret API failure", failResource: "secrets", wantErr: "error getting"},
		{name: "nil secret data", wantErr: "must contain non-empty tls.crt and tls.key"},
		{name: "empty secret data", data: map[string][]byte{}, wantErr: "must contain non-empty"},
		{name: "missing certificate", data: map[string][]byte{corev1.TLSPrivateKeyKey: []byte("key")}, wantErr: "must contain non-empty"},
		{name: "missing key", data: map[string][]byte{corev1.TLSCertKey: []byte("cert")}, wantErr: "must contain non-empty"},
		{name: "empty certificate", data: map[string][]byte{corev1.TLSCertKey: {}, corev1.TLSPrivateKeyKey: []byte("key")}, wantErr: "must contain non-empty"},
		{name: "empty key", data: map[string][]byte{corev1.TLSCertKey: []byte("cert"), corev1.TLSPrivateKeyKey: {}}, wantErr: "must contain non-empty"},
		{name: "no proxy pod", data: validData, omitPod: true, wantErr: "no authentication proxy component pods found"},
		{name: "pod API failure", data: validData, failResource: "pods", wantErr: "error listing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			component := AuthenticationProxyComponent{Namespace: "proxy-test"}
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "squid-tls", Namespace: component.Namespace}, Data: tc.data}
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "proxy-0", Namespace: component.Namespace, Labels: map[string]string{"app": squidServiceName}}}
			client := kubefake.NewSimpleClientset()
			if !tc.omitSecret {
				if err := client.Tracker().Add(secret.DeepCopy()); err != nil {
					t.Fatal(err)
				}
			}
			if !tc.omitPod {
				if err := client.Tracker().Add(pod); err != nil {
					t.Fatal(err)
				}
			}
			// A pod in the namespace is insufficient unless its label matches Squid.
			if err := client.Tracker().Add(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "unrelated", Namespace: component.Namespace}}); err != nil {
				t.Fatal(err)
			}
			if tc.failResource != "" {
				client.PrependReactor("*", tc.failResource, func(clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, readErr
				})
			}
			gotSecret, gotPod, err := component.certificateRotationResources(context.Background(), client.CoreV1())
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want message containing %q", err, tc.wantErr)
				}
				if tc.failResource != "" && !errors.Is(err, readErr) {
					t.Errorf("API error was not preserved: %v", err)
				}
				if gotSecret != nil || gotPod != nil {
					t.Error("failed resource validation returned resources")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(gotSecret, secret) || !reflect.DeepEqual(gotPod, pod) {
					t.Error("unexpected rotation resources")
				}
			}
			for _, action := range client.Actions() {
				if action.GetVerb() != "get" && action.GetVerb() != "list" {
					t.Errorf("resource validation performed a write: %s", action.GetVerb())
				}
				if action.GetResource().Resource == "pods" && action.GetNamespace() != component.Namespace {
					t.Errorf("pod lookup used namespace %q", action.GetNamespace())
				}
			}
		})
	}
}

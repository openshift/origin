package auth_proxy

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

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
			component: AuthenticationProxyComponent{namespace: "proxy-test"},
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
				namespace: "proxy-test", HttpUrl: MustParse("http://user:secret@proxy:3128"),
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
			component := AuthenticationProxyComponent{namespace: "proxy-test"}
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "squid-tls", Namespace: component.namespace}, Data: tc.data}
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "proxy-0", Namespace: component.namespace, Labels: map[string]string{"app": squidServiceName}}}
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
			if err := client.Tracker().Add(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "unrelated", Namespace: component.namespace}}); err != nil {
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
				if action.GetResource().Resource == "pods" && action.GetNamespace() != component.namespace {
					t.Errorf("pod lookup used namespace %q", action.GetNamespace())
				}
			}
		})
	}
}

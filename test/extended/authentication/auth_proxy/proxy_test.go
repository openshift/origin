package auth_proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	kubefake "k8s.io/client-go/kubernetes/fake"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	clienttesting "k8s.io/client-go/testing"
)

func MustParse(url string) *url.URL {
	ret, err := parseProxyField(url)
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
		{name: "relative URL", value: "proxy", wantErr: true},
		{name: "missing scheme", value: "//proxy:3128", wantErr: true},
		{name: "unsupported scheme", value: "socks5://proxy", wantErr: true},
		{name: "missing hostname", value: "http:///", wantErr: true},
		{name: "missing hostname with port", value: "http://:3128", wantErr: true},
		{name: "nonnumeric port", value: "http://proxy:abc", wantErr: true},
		{name: "empty port", value: "http://proxy:", wantErr: true},
		{name: "zero port", value: "http://proxy:0", wantErr: true},
		{name: "negative port", value: "http://proxy:-1", wantErr: true},
		{name: "port out of range", value: "http://proxy:65536", wantErr: true},
		{name: "maximum port", value: "https://proxy:65535", want: &url.URL{Scheme: "https", Host: "proxy:65535"}},
		{name: "default port", value: "https://proxy", want: &url.URL{Scheme: "https", Host: "proxy"}},
		{name: "IPv6", value: "http://[2001:db8::1]:3128", want: &url.URL{Scheme: "http", Host: "[2001:db8::1]:3128"}},
		{name: "IPv6 without port", value: "http://[2001:db8::1]", want: &url.URL{Scheme: "http", Host: "[2001:db8::1]"}},
		{name: "IPv6 empty port", value: "http://[2001:db8::1]:", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseProxyField(tc.value)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseProxyField(%q) error = %v, want error = %v", tc.value, err, tc.wantErr)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParseProxyField(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestParseProxyFieldErrorsOmitCredentials(t *testing.T) {
	t.Parallel()
	const username, password = "private-proxy-user", "private-proxy-password"
	for _, tc := range []struct {
		name, value, wantReason string
	}{
		{
			name:       "invalid escape in username",
			value:      "http://" + username + "%zz:" + password + "@proxy:3128",
			wantReason: "invalid percent-encoding",
		},
		{
			name:       "invalid escape in password",
			value:      "https://" + username + ":" + password + "%zz@proxy:3128",
			wantReason: "invalid percent-encoding",
		},
		{
			name:       "invalid hostname",
			value:      "http://" + username + ":" + password + "@proxy host:3128",
			wantReason: "invalid hostname",
		},
		{
			name:       "invalid port",
			value:      "https://" + username + ":" + password + "@proxy:bad-port",
			wantReason: "invalid syntax",
		},
		{
			name:       "secret in malformed port",
			value:      "http://" + username + "@proxy:" + password,
			wantReason: "invalid syntax",
		},
		{
			name:       "missing IPv6 bracket",
			value:      "http://" + username + ":" + password + "@[2001:db8::1",
			wantReason: "invalid syntax",
		},
		{
			name:       "unsupported scheme",
			value:      "socks5://" + username + ":" + password + "@proxy:3128",
			wantReason: "must use HTTP or HTTPS",
		},
		{
			name:       "missing hostname",
			value:      "https://" + username + ":" + password + "@:3128",
			wantReason: "include a hostname",
		},
		{
			name:       "port out of range",
			value:      "https://" + username + ":" + password + "@proxy:65536",
			wantReason: "port must be in the range",
		},
		{
			name:       "empty port",
			value:      "https://" + username + ":" + password + "@proxy:",
			wantReason: "empty port",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseProxyField(tc.value)
			if err == nil {
				t.Fatal("expected an error")
			}
			if got != nil {
				t.Error("invalid input returned a URL")
			}
			if !strings.Contains(err.Error(), tc.wantReason) {
				t.Errorf("error = %q, want reason %q", err, tc.wantReason)
			}
			// Callers add field and container context before logging the error.
			wrapped := fmt.Errorf("reading HTTPS_PROXY for container application: %w", err)
			for _, loggedErr := range []error{err, wrapped} {
				for _, format := range []string{"%s", "%v", "%+v", "%#v"} {
					message := fmt.Sprintf(format, loggedErr)
					for _, sensitive := range []string{username, password, "%zz", tc.value} {
						if strings.Contains(message, sensitive) {
							t.Errorf("format %s exposed sensitive input: %q", format, message)
						}
					}
				}
				var rawParseErr *url.Error
				if errors.As(loggedErr, &rawParseErr) {
					t.Error("error chain exposes the original URL through url.Error")
				}
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
			name: "sorted noProxy", config: ProxyConfig{NoProxy: sets.New("localhost", ".svc", "127.0.0.1")},
			want: `ProxyConfig[HttpProxy="" HttpsProxy="" NoProxy=".svc,127.0.0.1,localhost"]`,
		},
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

func TestProxyConfigHasProxy(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		config ProxyConfig
		want   bool
	}{
		{
			name: "empty"},
		{
			name: "URL presence does not imply validity", config: ProxyConfig{HttpProxy: &url.URL{}}, want: true,
		},
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
				CACertPEM: []byte("test CA"),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.config.HasProxy(); got != tc.want {
				t.Fatalf("HasProxy() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProxyConfigMatches(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name             string
		a, b             ProxyConfig
		want, asymmetric bool
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
			asymmetric: true,
		},
		{
			name: "one noProxy has values the other is empty",
			a: ProxyConfig{
				NoProxy: sets.New("localhost"),
			},
			b: ProxyConfig{
				NoProxy: sets.New[string](),
			},
		},
		{
			name: "different CA ignored",
			a: ProxyConfig{
				CACertPEM: []byte("CA A"),
			},
			b: ProxyConfig{
				CACertPEM: []byte("CA B"),
			},
			want: true,
		},
		{
			name: "missing CA ignored",
			a: ProxyConfig{
				CACertPEM: []byte("CA A"),
			},
			want: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			AExpectedB := tc.a.Matches(tc.b)
			BExpectedA := tc.b.Matches(tc.a)

			if !tc.asymmetric {
				if AExpectedB != tc.want {
					t.Errorf("a.MatchesExpected(b) = %v, want %v", AExpectedB, tc.want)
				}
				if BExpectedA != tc.want {
					t.Errorf("b.MatchesExpected(a) = %v, want %v", BExpectedA, tc.want)
				}
			} else {
				if AExpectedB != tc.want {
					t.Errorf("a.MatchesExpected(b) = %v, want %v", AExpectedB, tc.want)
				}
				if BExpectedA == tc.want {
					t.Errorf("b.MatchesExpected(a) = %v, want %v", BExpectedA, !tc.want)
				}
			}
		})
	}
}

func TestReadDeploymentProxyConfig(t *testing.T) {
	t.Parallel()
	readErr := errors.New("injected API failure")
	optional := true
	caVolumes := []corev1.Volume{{Name: "proxy-ca-volume", VolumeSource: corev1.VolumeSource{
		ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "mounted-proxy-ca"}},
	}}}
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
		nilConfigMaps bool
		optional      *bool
	}{
		{name: "no proxy"},
		{name: "no CA volume requires no configMaps client", nilConfigMaps: true},
		{name: "CA requires configMaps client", volumes: caVolumes, nilConfigMaps: true, wantErr: "configMaps client is required"},
		{name: "invalid HTTP URL", env: []corev1.EnvVar{{Name: "HTTP_PROXY", Value: "relative"}}, wantErr: "http proxy field"},
		{name: "invalid HTTPS URL", env: []corev1.EnvVar{{Name: "HTTPS_PROXY", Value: "http://proxy:65536"}}, wantErr: "https proxy field"},
		{name: "missing CA key", volumes: caVolumes, caData: map[string]string{}, wantErr: "must contain non-empty"},
		{name: "empty CA", volumes: caVolumes, caData: map[string]string{"custom-ca.pem": ""}, wantErr: "must contain non-empty"},
		{name: "whitespace CA", volumes: caVolumes, caData: map[string]string{"custom-ca.pem": " \n"}, wantErr: "must contain non-empty"},
		{name: "Optional does not allow a missing CA ConfigMap", volumes: caVolumes, optional: &optional, wantErr: "proxy CA certificate"},
		{name: "Optional does not allow a missing CA key", volumes: caVolumes, caData: map[string]string{}, optional: &optional, wantErr: "must contain non-empty"},
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
				Name: "proxy-ca-volume",
				VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "mounted-proxy-ca"},
				}},
			}},
			caData: map[string]string{"custom-ca.pem": "test CA"},
			want:   ProxyConfig{HttpsProxy: MustParse("https://b:3129"), CACertPEM: []byte("test CA")},
		},
		{
			name: "ignore unrelated CA volume",
			volumes: []corev1.Volume{{Name: "unrelated", VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "missing-ca"}},
			}}},
		},
		{
			name: "ignore non-configmap volume",
			volumes: []corev1.Volume{{Name: "proxy-ca-volume", VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "unrelated"},
			}}},
		},
		{name: "missing container", omitContainer: true, wantErr: `container "application" not found`},
		{name: "missing deployment", omitDeploy: true, wantErr: "not found"},
		{name: "deployment API failure", failResource: "deployments", wantErr: "injected API failure"},
		{
			name: "missing CA configmap",
			volumes: []corev1.Volume{{Name: "proxy-ca-volume", VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "mounted-proxy-ca"}},
			}}},
			wantErr: "test-namespace/proxy-client proxy CA certificate",
		},
		{
			name: "CA API failure", failResource: "configmaps",
			volumes: []corev1.Volume{{Name: "proxy-ca-volume", VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "mounted-proxy-ca"}},
			}}},
			wantErr: "test-namespace/proxy-client proxy CA certificate",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "proxy-client", Namespace: "test-namespace"},
			}
			deployment.Spec.Template.Spec.Volumes = tc.volumes
			deployment.Spec.Template.Spec.Containers = []corev1.Container{{
				Name: "sidecar", Env: []corev1.EnvVar{{Name: "HTTP_PROXY", Value: "http://wrong-proxy"}},
			}}
			if !tc.omitContainer {
				deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, corev1.Container{
					Name: "application", Env: tc.env,
				})
			}
			// A later sidecar must not override the selected container's configuration.
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
					ObjectMeta: metav1.ObjectMeta{Name: "mounted-proxy-ca", Namespace: "test-namespace"}, Data: tc.caData,
				}); err != nil {
					t.Fatal(err)
				}
			}
			if tc.failResource != "" {
				client.PrependReactor("get", tc.failResource, func(clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, readErr
				})
			}
			var configMaps coreclientv1.ConfigMapsGetter = client.CoreV1()
			if tc.nilConfigMaps {
				configMaps = nil
			}
			got, err := DeploymentProxyConfig(context.Background(),
				"test-namespace", "proxy-client", "application",
				corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "mounted-proxy-ca"},
					Key:                  "custom-ca.pem",
					Optional:             tc.optional,
				}, client.AppsV1(), configMaps)
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
	if !bytes.Equal(got.CACertPEM, want.CACertPEM) {
		t.Errorf("CA PEM = %q, want %q", got.CACertPEM, want.CACertPEM)
	}
}

package disruptioninclusterapiserver

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func envMap(envVars []corev1.EnvVar) map[string]string {
	m := map[string]string{}
	for _, e := range envVars {
		m[e.Name] = e.Value
	}
	return m
}

func countEnv(envVars []corev1.EnvVar, name string) int {
	n := 0
	for _, e := range envVars {
		if e.Name == name {
			n++
		}
	}
	return n
}

func TestSetProxyEnvVars(t *testing.T) {
	baseEnv := []corev1.EnvVar{
		{Name: "KUBERNETES_SERVICE_HOST", Value: "api.example:30175"},
		{Name: "KUBERNETES_SERVICE_PORT", Value: "30175"},
	}

	tests := []struct {
		name          string
		hostedType    HostedClusterType
		httpsProxy    string
		httpProxy     string
		noProxy       string
		existingProxy []corev1.EnvVar
		wantHTTPS     string
		wantHTTP      string
		wantNoProxy   string
		wantInjected  bool
	}{
		{
			name:         "hosted bare metal with https proxy injects proxy env",
			hostedType:   HostedClusterTypeBareMetal,
			httpsProxy:   "http://proxy:8213",
			httpProxy:    "http://proxy:8213",
			noProxy:      ".svc,.cluster.local,172.31.0.0/16",
			wantHTTPS:    "http://proxy:8213",
			wantHTTP:     "http://proxy:8213",
			wantNoProxy:  ".svc,.cluster.local,172.31.0.0/16",
			wantInjected: true,
		},
		{
			name:         "hosted other with https proxy injects proxy env",
			hostedType:   HostedClusterTypeOther,
			httpsProxy:   "http://proxy:8213",
			wantHTTPS:    "http://proxy:8213",
			wantInjected: true,
		},
		{
			name:         "hosted ARO HCP with https proxy injects proxy env",
			hostedType:   HostedClusterTypeAROHCP,
			httpsProxy:   "http://proxy:8213",
			wantHTTPS:    "http://proxy:8213",
			wantInjected: true,
		},
		{
			name:         "standalone is a no-op even when proxy is set",
			hostedType:   HostedClusterTypeStandalone,
			httpsProxy:   "http://proxy:8213",
			httpProxy:    "http://proxy:8213",
			noProxy:      ".svc",
			wantInjected: false,
		},
		{
			name:         "hosted with no proxy configured is a no-op",
			hostedType:   HostedClusterTypeBareMetal,
			wantInjected: false,
		},
		{
			name:          "existing proxy entries are replaced, not duplicated",
			hostedType:    HostedClusterTypeBareMetal,
			httpsProxy:    "http://new:8213",
			existingProxy: []corev1.EnvVar{{Name: "HTTPS_PROXY", Value: "http://stale:1"}},
			wantHTTPS:     "http://new:8213",
			wantInjected:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			i := &InvariantInClusterDisruption{
				hostedClusterType:      tc.hostedType,
				clusterProxyHTTPSProxy: tc.httpsProxy,
				clusterProxyHTTPProxy:  tc.httpProxy,
				clusterProxyNoProxy:    tc.noProxy,
			}

			in := append([]corev1.EnvVar{}, baseEnv...)
			in = append(in, tc.existingProxy...)

			got := i.setProxyEnvVars(in)
			m := envMap(got)

			// Base env must always be preserved.
			if m["KUBERNETES_SERVICE_HOST"] != "api.example:30175" {
				t.Errorf("KUBERNETES_SERVICE_HOST not preserved: %q", m["KUBERNETES_SERVICE_HOST"])
			}

			if !tc.wantInjected {
				for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"} {
					if _, ok := m[name]; ok {
						t.Errorf("expected no %s injected, got %q", name, m[name])
					}
				}
				return
			}

			if tc.wantHTTPS != "" && m["HTTPS_PROXY"] != tc.wantHTTPS {
				t.Errorf("HTTPS_PROXY = %q, want %q", m["HTTPS_PROXY"], tc.wantHTTPS)
			}
			if tc.wantHTTP != "" && m["HTTP_PROXY"] != tc.wantHTTP {
				t.Errorf("HTTP_PROXY = %q, want %q", m["HTTP_PROXY"], tc.wantHTTP)
			}
			if tc.wantNoProxy != "" && m["NO_PROXY"] != tc.wantNoProxy {
				t.Errorf("NO_PROXY = %q, want %q", m["NO_PROXY"], tc.wantNoProxy)
			}
			// NO_PROXY must not be set when it was not configured.
			if tc.noProxy == "" {
				if _, ok := m["NO_PROXY"]; ok {
					t.Errorf("NO_PROXY should be absent when not configured, got %q", m["NO_PROXY"])
				}
			}
			// No duplicate proxy entries.
			if c := countEnv(got, "HTTPS_PROXY"); c > 1 {
				t.Errorf("HTTPS_PROXY appears %d times, want <=1", c)
			}
		})
	}
}

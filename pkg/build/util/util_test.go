package util

import (
	"net/url"
	"regexp"
	"testing"

	s2iapi "github.com/openshift/source-to-image/pkg/api"

	corev1 "k8s.io/api/core/v1"

	buildapiv1 "github.com/openshift/api/build/v1"
)

func TestTrustedMergeEnvWithoutDuplicates(t *testing.T) {
	input := []corev1.EnvVar{
		// stripped by whitelist
		{Name: "foo", Value: "bar"},
		// stripped by whitelist
		{Name: "input", Value: "inputVal"},
		{Name: "GIT_SSL_NO_VERIFY", Value: "source"},
		{Name: "BUILD_LOGLEVEL", Value: "source"},
	}
	output := []corev1.EnvVar{
		{Name: "foo", Value: "test"},
		{Name: "GIT_SSL_NO_VERIFY", Value: "target"},
	}
	// resolve conflicts w/ input value
	MergeTrustedEnvWithoutDuplicates(input, &output, true)

	if len(output) != 3 {
		t.Errorf("Expected output to contain input items len==3 (%d), %#v", len(output), output)
	}

	if output[0].Name != "foo" {
		t.Errorf("Expected output to have env 'foo', got %+v", output[0])
	}
	if output[0].Value != "test" {
		t.Errorf("Expected output env 'foo' to have value 'test', got %+v", output[0])
	}
	if output[1].Name != "GIT_SSL_NO_VERIFY" {
		t.Errorf("Expected output to have env 'GIT_SSL_NO_VERIFY', got %+v", output[1])
	}
	if output[1].Value != "source" {
		t.Errorf("Expected output env 'GIT_SSL_NO_VERIFY' to have value 'loglevel', got %+v", output[1])
	}
	if output[2].Name != "BUILD_LOGLEVEL" {
		t.Errorf("Expected output to have env 'BUILD_LOGLEVEL', got %+v", output[1])
	}
	if output[2].Value != "source" {
		t.Errorf("Expected output env 'BUILD_LOGLEVEL' to have value 'loglevel', got %+v", output[1])
	}

	input = []corev1.EnvVar{
		// stripped by whitelist
		{Name: "foo", Value: "bar"},
		// stripped by whitelist
		{Name: "input", Value: "inputVal"},
		{Name: "GIT_SSL_NO_VERIFY", Value: "source"},
		{Name: "BUILD_LOGLEVEL", Value: "source"},
	}
	output = []corev1.EnvVar{
		{Name: "foo", Value: "test"},
		{Name: "GIT_SSL_NO_VERIFY", Value: "target"},
	}
	// resolve conflicts w/ output value
	MergeTrustedEnvWithoutDuplicates(input, &output, false)

	if len(output) != 3 {
		t.Errorf("Expected output to contain input items len==3 (%d), %#v", len(output), output)
	}

	if output[0].Name != "foo" {
		t.Errorf("Expected output to have env 'foo', got %+v", output[0])
	}
	if output[0].Value != "test" {
		t.Errorf("Expected output env 'foo' to have value 'test', got %+v", output[0])
	}
	if output[1].Name != "GIT_SSL_NO_VERIFY" {
		t.Errorf("Expected output to have env 'GIT_SSL_NO_VERIFY', got %+v", output[1])
	}
	if output[1].Value != "target" {
		t.Errorf("Expected output env 'GIT_SSL_NO_VERIFY' to have value 'loglevel', got %+v", output[1])
	}
	if output[2].Name != "BUILD_LOGLEVEL" {
		t.Errorf("Expected output to have env 'BUILD_LOGLEVEL', got %+v", output[1])
	}
	if output[2].Value != "source" {
		t.Errorf("Expected output env 'BUILD_LOGLEVEL' to have value 'source', got %+v", output[1])
	}

}

var credsRegex = regexp.MustCompile("user:password")
var redactedRegex = regexp.MustCompile("redacted")

func TestSafeForLoggingS2IConfig(t *testing.T) {
	http, _ := url.Parse("http://user:password@proxy.com")
	https, _ := url.Parse("https://user:password@proxy.com")
	config := &s2iapi.Config{
		ScriptsURL: "https://user:password@proxy.com",
		Environment: []s2iapi.EnvironmentSpec{
			{
				Name:  "HTTP_PROXY",
				Value: "http://user:password@proxy.com",
			},
			{
				Name:  "HTTPS_PROXY",
				Value: "https://user:password@proxy.com",
			},
			{
				Name:  "other_value",
				Value: "http://user:password@proxy.com",
			},
		},
		ScriptDownloadProxyConfig: &s2iapi.ProxyConfig{
			HTTPProxy:  http,
			HTTPSProxy: https,
		},
	}
	checkEnvList(t, config.Environment, true)

	stripped := SafeForLoggingS2IConfig(config)
	if credsRegex.MatchString(stripped.ScriptsURL) {
		t.Errorf("credentials left in scripts url: %v", stripped.ScriptsURL)
	}
	if !redactedRegex.MatchString(stripped.ScriptsURL) {
		t.Errorf("redacted not present in scripts url: %v", stripped.ScriptsURL)
	}

	if credsRegex.MatchString(stripped.ScriptDownloadProxyConfig.HTTPProxy.String()) {
		t.Errorf("credentials left in scripts proxy: %v", stripped.ScriptDownloadProxyConfig.HTTPProxy)
	}
	if !redactedRegex.MatchString(stripped.ScriptDownloadProxyConfig.HTTPProxy.String()) {
		t.Errorf("redacted not present in scripts proxy: %v", stripped.ScriptDownloadProxyConfig.HTTPProxy)
	}

	if credsRegex.MatchString(stripped.ScriptDownloadProxyConfig.HTTPSProxy.String()) {
		t.Errorf("credentials left in scripts proxy: %v", stripped.ScriptDownloadProxyConfig.HTTPSProxy)
	}
	if !redactedRegex.MatchString(stripped.ScriptDownloadProxyConfig.HTTPSProxy.String()) {
		t.Errorf("redacted not present in scripts proxy: %v", stripped.ScriptDownloadProxyConfig.HTTPSProxy)
	}

	checkEnvList(t, stripped.Environment, false)

	// make sure original object is untouched
	if !credsRegex.MatchString(config.ScriptsURL) {
		t.Errorf("credentials stripped from original scripts url: %v", config.ScriptsURL)
	}
	if redactedRegex.MatchString(config.ScriptsURL) {
		t.Errorf("credentials stripped from original scripts url: %v", config.ScriptsURL)
	}
	if !credsRegex.MatchString(config.ScriptDownloadProxyConfig.HTTPProxy.String()) {
		t.Errorf("credentials stripped from original scripts proxy: %v", config.ScriptDownloadProxyConfig.HTTPProxy)
	}
	if redactedRegex.MatchString(config.ScriptDownloadProxyConfig.HTTPProxy.String()) {
		t.Errorf("credentials stripped from original scripts proxy: %v", config.ScriptDownloadProxyConfig.HTTPProxy)
	}
	if !credsRegex.MatchString(config.ScriptDownloadProxyConfig.HTTPSProxy.String()) {
		t.Errorf("credentials stripped from original scripts proxy: %v", config.ScriptDownloadProxyConfig.HTTPSProxy)
	}
	if redactedRegex.MatchString(config.ScriptDownloadProxyConfig.HTTPSProxy.String()) {
		t.Errorf("credentials stripped from original scripts proxy: %v", config.ScriptDownloadProxyConfig.HTTPSProxy)
	}
	//checkEnvList(t, config.Environment, true)

}

func checkEnvList(t *testing.T, envs s2iapi.EnvironmentList, orig bool) {
	for _, env := range envs {
		if env.Name == "other_value" {
			if !credsRegex.MatchString(env.Value) {
				t.Errorf("credentials improperly stripped from env value %v", env)
			}
			if redactedRegex.MatchString(env.Value) {
				t.Errorf("redacted should not appear in env value %v", env)
			}
		} else {
			if orig {
				if !credsRegex.MatchString(env.Value) {
					t.Errorf("credentials improperly stripped from orig env value %v", env)
				}
				if redactedRegex.MatchString(env.Value) {
					t.Errorf("redacted should appear in orig env value %v", env)
				}
			} else {
				if credsRegex.MatchString(env.Value) {
					t.Errorf("credentials not stripped from env value %v", env)
				}
				if !redactedRegex.MatchString(env.Value) {
					t.Errorf("redacted should appear in env value %v", env)
				}
			}
		}
	}
}

func TestSafeForLoggingBuild(t *testing.T) {
	httpProxy := "http://user:password@proxy.com"
	httpsProxy := "https://user:password@proxy.com"
	proxyBuild := &buildapiv1.Build{
		Spec: buildapiv1.BuildSpec{
			CommonSpec: buildapiv1.CommonSpec{
				Source: buildapiv1.BuildSource{
					Git: &buildapiv1.GitBuildSource{
						ProxyConfig: buildapiv1.ProxyConfig{
							HTTPProxy:  &httpProxy,
							HTTPSProxy: &httpsProxy,
						},
					},
				},
				Strategy: buildapiv1.BuildStrategy{
					SourceStrategy: &buildapiv1.SourceBuildStrategy{
						Env: []corev1.EnvVar{
							{
								Name:  "HTTP_PROXY",
								Value: "http://user:password@proxy.com",
							},
							{
								Name:  "HTTPS_PROXY",
								Value: "https://user:password@proxy.com",
							},
							{
								Name:  "other_value",
								Value: "http://user:password@proxy.com",
							},
						},
					},
					DockerStrategy: &buildapiv1.DockerBuildStrategy{
						Env: []corev1.EnvVar{
							{
								Name:  "HTTP_PROXY",
								Value: "http://user:password@proxy.com",
							},
							{
								Name:  "HTTPS_PROXY",
								Value: "https://user:password@proxy.com",
							},
							{
								Name:  "other_value",
								Value: "http://user:password@proxy.com",
							},
						},
					},
					CustomStrategy: &buildapiv1.CustomBuildStrategy{
						Env: []corev1.EnvVar{
							{
								Name:  "HTTP_PROXY",
								Value: "http://user:password@proxy.com",
							},
							{
								Name:  "HTTPS_PROXY",
								Value: "https://user:password@proxy.com",
							},
							{
								Name:  "other_value",
								Value: "http://user:password@proxy.com",
							},
						},
					},
					JenkinsPipelineStrategy: &buildapiv1.JenkinsPipelineBuildStrategy{
						Env: []corev1.EnvVar{
							{
								Name:  "HTTP_PROXY",
								Value: "http://user:password@proxy.com",
							},
							{
								Name:  "HTTPS_PROXY",
								Value: "https://user:password@proxy.com",
							},
							{
								Name:  "other_value",
								Value: "http://user:password@proxy.com",
							},
						},
					},
				},
			},
		},
	}

	stripped := SafeForLoggingBuild(proxyBuild)
	if credsRegex.MatchString(*stripped.Spec.Source.Git.HTTPProxy) {
		t.Errorf("credentials left in http proxy value: %v", stripped.Spec.Source.Git.HTTPProxy)
	}
	if credsRegex.MatchString(*stripped.Spec.Source.Git.HTTPSProxy) {
		t.Errorf("credentials left in https proxy value: %v", stripped.Spec.Source.Git.HTTPSProxy)
	}
	checkEnv(t, stripped.Spec.Strategy.SourceStrategy.Env)
	checkEnv(t, stripped.Spec.Strategy.DockerStrategy.Env)
	checkEnv(t, stripped.Spec.Strategy.CustomStrategy.Env)
	checkEnv(t, stripped.Spec.Strategy.JenkinsPipelineStrategy.Env)
}

func checkEnv(t *testing.T, envs []corev1.EnvVar) {
	for _, env := range envs {
		if env.Name == "other_value" {
			if !credsRegex.MatchString(env.Value) {
				t.Errorf("credentials improperly stripped from env value %v", env)
			}
		} else {
			if credsRegex.MatchString(env.Value) {
				t.Errorf("credentials not stripped from env value %v", env)
			}
		}
	}
}

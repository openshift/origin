package configuration

import (
	"bytes"
	"os"
	"testing"
)

var configYamlV0_1 = `
version: 0.1
http:
  addr: :5000
  relativeurls: true
storage:
  inmemory: {}
openshift:
  version: 1.0
  metrics:
    enabled: true
    secret: TopSecretToken
`

func TestConfigurationParser(t *testing.T) {
	configFile := bytes.NewBufferString(configYamlV0_1)

	dockerConfig, extraConfig, err := Parse(configFile)
	if err != nil {
		t.Fatalf("unexpected error parsing configuration file: %s", err)
	}

	if !dockerConfig.HTTP.RelativeURLs {
		t.Fatalf("unexpected value: dockerConfig.HTTP.RelativeURLs != true")
	}

	if extraConfig.Version.Major() != 1 || extraConfig.Version.Minor() != 0 {
		t.Fatalf("unexpected value: extraConfig.Version: %s", extraConfig.Version)
	}

	if !extraConfig.Metrics.Enabled {
		t.Fatalf("unexpected value: extraConfig.Metrics.Enabled != true")
	}

	if extraConfig.Metrics.Secret != "TopSecretToken" {
		t.Fatalf("unexpected value: extraConfig.Metrics.Secret: %s", extraConfig.Metrics.Secret)
	}
}

func TestConfigurationOverwriteEnv(t *testing.T) {
	os.Setenv("REGISTRY_OPENSHIFT_METRICS_ENABLED", "false")
	defer os.Unsetenv("REGISTRY_OPENSHIFT_METRICS_ENABLED")

	configFile := bytes.NewBufferString(configYamlV0_1)

	_, extraConfig, err := Parse(configFile)
	if err != nil {
		t.Fatalf("unexpected error parsing configuration file: %s", err)
	}
	if extraConfig.Metrics.Enabled {
		t.Fatalf("unexpected value: extraConfig.Metrics.Enabled != false")
	}
}

func TestDockerConfigurationError(t *testing.T) {
	var badDockerConfigYamlV0_1 = `
version: 0.1
http:
  addr: :5000
  relativeurls: "true"
storage:
  inmemory: {}
`
	configFile := bytes.NewBufferString(badDockerConfigYamlV0_1)

	_, _, err := Parse(configFile)
	if err == nil {
		t.Fatalf("unexpected parser success")
	}
}

func TestExtraConfigurationError(t *testing.T) {
	var badExtraConfigYaml = `
version: 0.1
http:
  addr: :5000
storage:
  inmemory: {}
openshift:
  version: 1.0
  metrics:
    enabled: "true"
`
	configFile := bytes.NewBufferString(badExtraConfigYaml)

	_, _, err := Parse(configFile)
	if err == nil {
		t.Fatalf("unexpected parser success")
	}
}

func TestEmptyExtraConfigurationError(t *testing.T) {
	var emptyExtraConfigYaml = `
version: 0.1
http:
  addr: :5000
storage:
  inmemory: {}
`
	configFile := bytes.NewBufferString(emptyExtraConfigYaml)

	_, _, err := Parse(configFile)
	if err != nil {
		t.Fatalf("unexpected parser error: %s", err)
	}
}

func TestExtraConfigurationVersionError(t *testing.T) {
	var badExtraConfigYaml = `
version: 0.1
http:
  addr: :5000
storage:
  inmemory: {}
openshift:
  version: 2.0
`
	configFile := bytes.NewBufferString(badExtraConfigYaml)

	_, _, err := Parse(configFile)
	if err == nil {
		t.Fatalf("unexpected parser success")
	}

	if err != ErrUnsupportedVersion {
		t.Fatalf("unexpected parser error: %v", err)
	}
}

package configuration

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/docker/distribution/configuration"
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
  server:
    addr: :5000
  metrics:
    enabled: true
    secret: TopSecretToken
  auth:
    realm: myrealm
  audit:
    enabled: true
  pullthrough:
    enabled: true
    mirror: true
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

	if extraConfig.Auth == nil {
		t.Fatalf("unexpected empty section: extraConfig.Auth")
	} else if extraConfig.Auth.Realm != "myrealm" {
		t.Fatalf("unexpected value: extraConfig.Auth.Realm: %s", extraConfig.Auth.Realm)
	}

	if extraConfig.Audit == nil {
		t.Fatalf("unexpected empty section: extraConfig.Audit")
	} else if !extraConfig.Audit.Enabled {
		t.Fatalf("unexpected value: extraConfig.Audit.Enabled != true")
	}

	if extraConfig.Pullthrough == nil {
		t.Fatalf("unexpected empty section: extraConfig.Pullthrough")
	} else {
		if !extraConfig.Pullthrough.Enabled {
			t.Fatalf("unexpected value: extraConfig.Pullthrough.Enabled != true")
		}
		if !extraConfig.Pullthrough.Mirror {
			t.Fatalf("unexpected value: extraConfig.Pullthrough.Mirror != true")
		}
	}
}

func testConfigurationOverwriteEnv(t *testing.T, config string) {
	os.Setenv("REGISTRY_OPENSHIFT_SERVER_ADDR", ":5000")
	defer os.Unsetenv("REGISTRY_OPENSHIFT_SERVER_ADDR")

	os.Setenv("REGISTRY_OPENSHIFT_METRICS_ENABLED", "false")
	defer os.Unsetenv("REGISTRY_OPENSHIFT_METRICS_ENABLED")

	configFile := bytes.NewBufferString(config)

	_, extraConfig, err := Parse(configFile)
	if err != nil {
		t.Fatalf("unexpected error parsing configuration file: %s", err)
	}
	if extraConfig.Metrics.Enabled {
		t.Fatalf("unexpected value: extraConfig.Metrics.Enabled != false")
	}
	if extraConfig.Server == nil {
		t.Fatalf("unexpected empty section extraConfig.Server")
	} else if extraConfig.Server.Addr != ":5000" {
		t.Fatalf("unexpected value: extraConfig.Server.Addr: %s", extraConfig.Server.Addr)
	}
}

func TestConfigurationOverwriteEnv(t *testing.T) {
	var configYaml = `
version: 0.1
storage:
  inmemory: {}
openshift:
  version: 1.0
  server:
    addr: :5000
  metrics:
    enabled: true
    secret: TopSecretToken
`
	testConfigurationOverwriteEnv(t, configYaml)
}

func TestConfigurationWithEmptyOpenshiftOverwriteEnv(t *testing.T) {
	var configYaml = `
version: 0.1
storage:
  inmemory: {}
`
	testConfigurationOverwriteEnv(t, configYaml)
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
	os.Setenv("REGISTRY_OPENSHIFT_SERVER_ADDR", ":5000")
	defer os.Unsetenv("REGISTRY_OPENSHIFT_SERVER_ADDR")

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

func TestDefaultMiddleware(t *testing.T) {
	checks := []struct {
		title, input, expect string
	}{
		{
			title: "miss all middlewares",
			input: `
version: 0.1
storage:
  inmemory: {}
`,
			expect: `
version: 0.1
storage:
  inmemory: {}
middleware:
  registry:
    - name: openshift
  repository:
    - name: openshift
  storage:
    - name: openshift
`,
		},
		{
			title: "miss some middlewares",
			input: `
version: 0.1
storage:
  inmemory: {}
middleware:
  registry:
    - name: openshift
`,
			expect: `
version: 0.1
storage:
  inmemory: {}
middleware:
  registry:
    - name: openshift
  repository:
    - name: openshift
  storage:
    - name: openshift
`,
		},
		{
			title: "all middlewares are in place",
			input: `
version: 0.1
storage:
  inmemory: {}
middleware:
  registry:
    - name: openshift
  repository:
    - name: openshift
  storage:
    - name: openshift
`,
			expect: `
version: 0.1
storage:
  inmemory: {}
middleware:
  registry:
    - name: openshift
  repository:
    - name: openshift
  storage:
    - name: openshift
`,
		},
		{
			title: "check v1.0.8 config",
			input: `
version: 0.1
log:
  level: debug
http:
  addr: :5000
storage:
  cache:
    layerinfo: inmemory
  filesystem:
    rootdirectory: /registry
auth:
  openshift:
    realm: openshift
middleware:
  repository:
   - name: openshift
`,
			expect: `
version: 0.1
log:
  level: debug
http:
  addr: :5000
storage:
  cache:
    layerinfo: inmemory
  filesystem:
    rootdirectory: /registry
auth:
  openshift:
    realm: openshift
middleware:
  registry:
    - name: openshift
  repository:
    - name: openshift
  storage:
    - name: openshift
`,
		},
		{
			title: "check v1.2.1 config",
			input: `
version: 0.1
log:
  level: debug
http:
  addr: :5000
storage:
  cache:
    layerinfo: inmemory
  filesystem:
    rootdirectory: /registry
  delete:
    enabled: true
auth:
  openshift:
    realm: openshift
middleware:
  repository:
    - name: openshift
      options:
        pullthrough: true
`,
			expect: `
version: 0.1
log:
  level: debug
http:
  addr: :5000
storage:
  cache:
    layerinfo: inmemory
  filesystem:
    rootdirectory: /registry
  delete:
    enabled: true
auth:
  openshift:
    realm: openshift
middleware:
  registry:
    - name: openshift
  repository:
    - name: openshift
      options:
        pullthrough: true
  storage:
    - name: openshift
`,
		},
		{
			title: "check v1.3.0-alpha.3 config",
			input: `
version: 0.1
log:
  level: debug
http:
  addr: :5000
storage:
  cache:
    blobdescriptor: inmemory
  filesystem:
    rootdirectory: /registry
  delete:
    enabled: true
auth:
  openshift:
    realm: openshift
middleware:
  registry:
    - name: openshift
  repository:
    - name: openshift
      options:
        acceptschema2: false
        pullthrough: true
        enforcequota: false
        projectcachettl: 1m
        blobrepositorycachettl: 10m
  storage:
    - name: openshift
`,
			expect: `
version: 0.1
log:
  level: debug
http:
  addr: :5000
storage:
  cache:
    blobdescriptor: inmemory
  filesystem:
    rootdirectory: /registry
  delete:
    enabled: true
auth:
  openshift:
    realm: openshift
middleware:
  registry:
    - name: openshift
  repository:
    - name: openshift
      options:
        acceptschema2: false
        pullthrough: true
        enforcequota: false
        projectcachettl: 1m
        blobrepositorycachettl: 10m
  storage:
    - name: openshift
`,
		},
	}

	for _, check := range checks {
		currentConfig, err := configuration.Parse(strings.NewReader(check.input))
		if err != nil {
			t.Fatal(err)
		}
		expectConfig, err := configuration.Parse(strings.NewReader(check.expect))
		if err != nil {
			t.Fatal(err)
		}
		setDefaultMiddleware(currentConfig)

		if !reflect.DeepEqual(currentConfig, expectConfig) {
			t.Errorf("%s: expected\n\t%#v\ngot\n\t%#v", check.title, expectConfig, currentConfig)
		}
	}
}

func TestMiddlewareMigration(t *testing.T) {
	var inputConfigYaml = `
version: 0.1
log:
  level: debug
http:
  addr: :5000
storage:
  cache:
    blobdescriptor: inmemory
  filesystem:
    rootdirectory: /registry
  delete:
    enabled: true
auth:
  openshift:
    realm: openshift
middleware:
  registry:
    - name: openshift
  repository:
    - name: openshift
      options:
        acceptschema2: true
        pullthrough: true
        enforcequota: false
        projectcachettl: 1m
        blobrepositorycachettl: 10m
  storage:
    - name: openshift
openshift:
  version: 1.0
  server:
    addr: :5000
`
	var expectConfigYaml = `
version: 0.1
log:
  level: debug
http:
  addr: :5000
storage:
  cache:
    blobdescriptor: inmemory
  filesystem:
    rootdirectory: /registry
  delete:
    enabled: true
auth:
  openshift
middleware:
  registry:
    - name: openshift
  repository:
    - name: openshift
  storage:
    - name: openshift
openshift:
  version: 1.0
  server:
    addr: :5000
  auth:
    realm: openshift
  quota:
    enabled: false
  pullthrough:
    enabled: true
    mirror: true
  cache:
    blobrepositoryttl: 10m
    quotattl: 1m
  compatibility:
    acceptschema2: true
`
	_, currentConfig, err := Parse(strings.NewReader(inputConfigYaml))
	if err != nil {
		t.Fatal(err)
	}
	_, expectConfig, err := Parse(strings.NewReader(expectConfigYaml))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(currentConfig.Server, expectConfig.Server) {
		t.Fatalf("expected server section\n\t%#v\ngot\n\t%#v", expectConfig.Server, currentConfig.Server)
	}
	if !reflect.DeepEqual(currentConfig.Auth, expectConfig.Auth) {
		t.Fatalf("expected auth section\n\t%#v\ngot\n\t%#v", expectConfig.Auth, currentConfig.Auth)
	}
	if !reflect.DeepEqual(currentConfig.Audit, expectConfig.Audit) {
		t.Fatalf("expected audit section\n\t%#v\ngot\n\t%#v", expectConfig.Audit, currentConfig.Audit)
	}
	if !reflect.DeepEqual(currentConfig.Quota, expectConfig.Quota) {
		t.Fatalf("expected quota section\n\t%#v\ngot\n\t%#v", expectConfig.Quota, currentConfig.Quota)
	}
	if !reflect.DeepEqual(currentConfig.Pullthrough, expectConfig.Pullthrough) {
		t.Fatalf("expected pullthrough section\n\t%#v\ngot\n\t%#v", expectConfig.Pullthrough, currentConfig.Pullthrough)
	}
	if !reflect.DeepEqual(currentConfig.Cache, expectConfig.Cache) {
		t.Fatalf("expected cache section\n\t%#v\ngot\n\t%#v", expectConfig.Cache, currentConfig.Cache)
	}
	if !reflect.DeepEqual(currentConfig.Compatibility, expectConfig.Compatibility) {
		t.Fatalf("expected compatibility section\n\t%#v\ngot\n\t%#v", expectConfig.Compatibility, currentConfig.Compatibility)
	}
}

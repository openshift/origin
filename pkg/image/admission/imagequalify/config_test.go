package imagequalify_test

import (
	"bytes"
	"io"
	"reflect"
	"testing"

	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/image/admission/imagequalify"
	"github.com/openshift/origin/pkg/image/admission/imagequalify/api"
	"github.com/openshift/origin/pkg/image/admission/imagequalify/api/validation"

	_ "github.com/openshift/origin/pkg/api/install"
)

const (
	goodConfig = `
apiVersion: admission.config.openshift.io/v1
kind: ImageQualifyConfig
rules:
- domain: example.com
  pattern: "*/*"
- domain: example.com
  pattern: "*"
`

	missingPatternConfig = `
apiVersion: admission.config.openshift.io/v1
kind: ImageQualifyConfig
rules:
- domain: example
  pattern:
`

	missingDomainConfig = `
apiVersion: admission.config.openshift.io/v1
kind: ImageQualifyConfig
rules:
- domain:
  pattern: foo
`

	invalidDomainConfig = `
apiVersion: admission.config.openshift.io/v1
kind: ImageQualifyConfig
rules:
- domain: "!example!"
  pattern: "*"
`

	emptyConfig = `
apiVersion: admission.config.openshift.io/v1
kind: ImageQualifyConfig
`
)

var (
	deserializedYamlConfig = &api.ImageQualifyConfig{
		Rules: []api.ImageQualifyRule{{
			Pattern: "*/*",
			Domain:  "example.com",
		}, {
			Pattern: "*",
			Domain:  "example.com",
		}},
	}
)

func testReaderConfig(rules []api.ImageQualifyRule) *api.ImageQualifyConfig {
	return &api.ImageQualifyConfig{
		Rules: rules,
	}
}

func TestConfigReader(t *testing.T) {
	initialConfig := testReaderConfig([]api.ImageQualifyRule{{
		Pattern: "*/*",
		Domain:  "example.com",
	}, {
		Pattern: "*",
		Domain:  "example.com",
	}})

	serializedConfig, serializationErr := configapilatest.WriteYAML(initialConfig)
	if serializationErr != nil {
		t.Fatalf("WriteYAML: config serialize failed: %v", serializationErr)
	}

	tests := []struct {
		name           string
		config         io.Reader
		expectErr      bool
		expectNil      bool
		expectInvalid  bool
		expectedConfig *api.ImageQualifyConfig
	}{{
		name:      "process nil config",
		config:    nil,
		expectNil: true,
	}, {
		name:           "deserialize initialConfig yaml",
		config:         bytes.NewReader(serializedConfig),
		expectedConfig: initialConfig,
	}, {
		name:      "completely broken config",
		config:    bytes.NewReader([]byte("busted")),
		expectErr: true,
	}, {
		name:           "deserialize good config",
		config:         bytes.NewReader([]byte(goodConfig)),
		expectedConfig: deserializedYamlConfig,
	}, {
		name:          "choke on missing pattern",
		config:        bytes.NewReader([]byte(missingPatternConfig)),
		expectInvalid: true,
		expectErr:     true,
	}, {
		name:          "choke on missing domain",
		config:        bytes.NewReader([]byte(missingDomainConfig)),
		expectInvalid: true,
		expectErr:     true,
	}, {
		name:          "choke on invalid domain",
		config:        bytes.NewReader([]byte(invalidDomainConfig)),
		expectInvalid: true,
		expectErr:     true,
	}, {
		name:           "empty config",
		config:         bytes.NewReader([]byte(emptyConfig)),
		expectedConfig: &api.ImageQualifyConfig{},
		expectInvalid:  false,
		expectErr:      false,
	}}

	for _, test := range tests {
		config, err := imagequalify.ReadConfig(test.config)
		if test.expectErr && err == nil {
			t.Errorf("%s: expected error", test.name)
		} else if !test.expectErr && err != nil {
			t.Errorf("%s: expected no error, saw %v", test.name, err)
		}
		if err == nil {
			if test.expectNil && config != nil {
				t.Errorf("%s: expected nil config, but saw: %v", test.name, config)
			} else if !test.expectNil && config == nil {
				t.Errorf("%s: expected config, but got nil", test.name)
			}
		}
		if config == nil {
			continue
		}
		if test.expectedConfig != nil && !reflect.DeepEqual(*test.expectedConfig, *config) {
			t.Errorf("%s: expected %v from reader, but got %v", test.name, test.expectErr, config)
		}
		if err := validation.Validate(config); test.expectInvalid && len(err) == 0 {
			t.Errorf("%s: expected validation to fail, but it passed", test.name)
		} else if !test.expectInvalid && len(err) > 0 {
			t.Errorf("%s: expected validation to pass, but it failed with %v", test.name, err)
		}
	}
}

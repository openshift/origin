package validation

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/validation/field"

	defaultsapi "github.com/openshift/origin/pkg/build/admission/defaults/api"
)

func TestValidateBuildDefaultsConfig(t *testing.T) {
	tests := []struct {
		config      *defaultsapi.BuildDefaultsConfig
		errExpected bool
		errField    string
		errType     field.ErrorType
	}{
		// 0: Valid config
		{
			config: &defaultsapi.BuildDefaultsConfig{
				GitHTTPProxy:  "http://valid.url",
				GitHTTPSProxy: "https://valid.url",
				Env: []kapi.EnvVar{
					{
						Name:  "VAR1",
						Value: "VALUE1",
					},
					{
						Name:  "VAR2",
						Value: "VALUE2",
					},
				},
			},
			errExpected: false,
		},
		// 1:  invalid HTTP proxy
		{
			config: &defaultsapi.BuildDefaultsConfig{
				GitHTTPProxy:  "some!@#$%^&*()url",
				GitHTTPSProxy: "https://valid.url",
			},
			errExpected: true,
			errField:    "gitHTTPProxy",
			errType:     field.ErrorTypeInvalid,
		},
		// 2:  invalid HTTPS proxy
		{
			config: &defaultsapi.BuildDefaultsConfig{
				GitHTTPProxy:  "https://valid.url",
				GitHTTPSProxy: "some!@#$%^&*()url",
			},
			errExpected: true,
			errField:    "gitHTTPSProxy",
			errType:     field.ErrorTypeInvalid,
		},
		// 3: missing Env variable name
		{
			config: &defaultsapi.BuildDefaultsConfig{
				Env: []kapi.EnvVar{
					{
						Name:  "",
						Value: "test",
					},
				},
			},
			errExpected: true,
			errField:    "env[0].name",
			errType:     field.ErrorTypeRequired,
		},
		// 4: invalid Env variable name
		{
			config: &defaultsapi.BuildDefaultsConfig{
				Env: []kapi.EnvVar{
					{
						Name:  " invalid,name",
						Value: "test",
					},
				},
			},
			errExpected: true,
			errField:    "env[0].name",
			errType:     field.ErrorTypeInvalid,
		},
		// 5: valueFrom present in env var
		{
			config: &defaultsapi.BuildDefaultsConfig{
				Env: []kapi.EnvVar{
					{
						Name:      "name",
						Value:     "test",
						ValueFrom: &kapi.EnvVarSource{},
					},
				},
			},
			errExpected: true,
			errField:    "env[0].valueFrom",
			errType:     field.ErrorTypeInvalid,
		},
	}

	for i, tc := range tests {
		result := ValidateBuildDefaultsConfig(tc.config)
		if !tc.errExpected {
			if len(result) > 0 {
				t.Errorf("%d: unexpected error: %v", i, result.ToAggregate())
			}
			continue
		}
		if tc.errExpected && len(result) == 0 {
			t.Errorf("%d: did not get expected error", i)
			continue
		}
		err := result[0]
		if err.Type != tc.errType {
			t.Errorf("%d: unexpected error type: %v", i, err.Type)
		}
		if err.Field != tc.errField {
			t.Errorf("%d: unexpected error field: %v", i, err.Field)
		}
	}
}

package validation

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/validation/field"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	defaultsapi "github.com/openshift/origin/pkg/build/controller/build/apis/defaults"
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
		// 5: ResourceFieldRef present in env var
		{
			config: &defaultsapi.BuildDefaultsConfig{
				Env: []kapi.EnvVar{
					{
						Name: "name",
						ValueFrom: &kapi.EnvVarSource{
							ResourceFieldRef: &kapi.ResourceFieldSelector{
								ContainerName: "name",
								Resource:      "resource",
							},
						},
					},
				},
			},
			errExpected: true,
			errField:    "env[0].valueFrom.ResourceFieldRef",
			errType:     field.ErrorTypeInvalid,
		},
		// 6: label: empty name
		{
			config: &defaultsapi.BuildDefaultsConfig{
				ImageLabels: []buildapi.ImageLabel{
					{
						Name:  "",
						Value: "empty",
					},
				},
			},
			errExpected: true,
			errField:    "imageLabels[0].name",
			errType:     field.ErrorTypeRequired,
		},
		// 7: label: bad name
		{
			config: &defaultsapi.BuildDefaultsConfig{
				ImageLabels: []buildapi.ImageLabel{
					{
						Name:  "\tč;",
						Value: "????",
					},
				},
			},
			errExpected: true,
			errField:    "imageLabels[0].name",
			errType:     field.ErrorTypeInvalid,
		},
		// 8: duplicate label
		{
			config: &defaultsapi.BuildDefaultsConfig{
				ImageLabels: []buildapi.ImageLabel{
					{
						Name:  "name",
						Value: "Jan",
					},
					{
						Name:  "name",
						Value: "Elvis",
					},
				},
			},
			errExpected: true,
			errField:    "imageLabels[1].name",
			errType:     field.ErrorTypeInvalid,
		},
		// 9: valid nodeselector
		{
			config: &defaultsapi.BuildDefaultsConfig{
				NodeSelector: map[string]string{"A": "B"},
			},
			errExpected: false,
		},
		// 10: invalid nodeselector
		{
			config: &defaultsapi.BuildDefaultsConfig{
				NodeSelector: map[string]string{"A@B!": "C"},
			},
			errExpected: true,
			errField:    "nodeSelector[A@B!]",
			errType:     field.ErrorTypeInvalid,
		},
		// 11: valid annotation
		{
			config: &defaultsapi.BuildDefaultsConfig{
				Annotations: map[string]string{"A": "B"},
			},
			errExpected: false,
		},
		// 12: invalid annotation
		{
			config: &defaultsapi.BuildDefaultsConfig{
				Annotations: map[string]string{"A B": "C"},
			},
			errExpected: true,
			errField:    "annotations",
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

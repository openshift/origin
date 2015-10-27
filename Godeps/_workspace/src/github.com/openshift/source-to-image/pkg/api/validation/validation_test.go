package validation

import (
	"reflect"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
)

func TestValidation(t *testing.T) {
	testCases := []struct {
		value    *api.Config
		expected []ValidationError
	}{
		{
			&api.Config{
				Source:       "http://github.com/openshift/source",
				BuilderImage: "openshift/builder",
				DockerConfig: &api.DockerConfig{Endpoint: "/var/run/docker.socket"},
			},
			[]ValidationError{},
		},
		{
			&api.Config{
				Source:            "http://github.com/openshift/source",
				BuilderImage:      "openshift/builder",
				DockerConfig:      &api.DockerConfig{Endpoint: "/var/run/docker.socket"},
				DockerNetworkMode: "foobar",
			},
			[]ValidationError{{ValidationErrorInvalidValue, "dockerNetworkMode"}},
		},
		{
			&api.Config{
				Source:            "http://github.com/openshift/source",
				BuilderImage:      "openshift/builder",
				DockerConfig:      &api.DockerConfig{Endpoint: "/var/run/docker.socket"},
				DockerNetworkMode: api.NewDockerNetworkModeContainer("8d873e496bc3e80a1cb22e67f7de7be5b0633e27916b1144978d1419c0abfcdb"),
			},
			[]ValidationError{},
		},
	}
	for _, test := range testCases {
		result := ValidateConfig(test.value)
		if !reflect.DeepEqual(result, test.expected) {
			t.Errorf("got %+v, expected %+v", result, test.expected)
		}
	}
}

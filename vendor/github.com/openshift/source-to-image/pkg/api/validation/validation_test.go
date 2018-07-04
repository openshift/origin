package validation

import (
	"reflect"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
)

func TestValidation(t *testing.T) {
	testCases := []struct {
		value    *api.Config
		expected []Error
	}{
		{
			&api.Config{
				Source:            "http://github.com/openshift/source",
				BuilderImage:      "openshift/builder",
				DockerConfig:      &api.DockerConfig{Endpoint: "/var/run/docker.socket"},
				BuilderPullPolicy: api.DefaultBuilderPullPolicy,
			},
			[]Error{},
		},
		{
			&api.Config{
				Source:            "http://github.com/openshift/source",
				BuilderImage:      "openshift/builder",
				DockerConfig:      &api.DockerConfig{Endpoint: "/var/run/docker.socket"},
				DockerNetworkMode: "foobar",
				BuilderPullPolicy: api.DefaultBuilderPullPolicy,
			},
			[]Error{{ErrorInvalidValue, "dockerNetworkMode"}},
		},
		{
			&api.Config{
				Source:            "http://github.com/openshift/source",
				BuilderImage:      "openshift/builder",
				DockerConfig:      &api.DockerConfig{Endpoint: "/var/run/docker.socket"},
				DockerNetworkMode: api.NewDockerNetworkModeContainer("8d873e496bc3e80a1cb22e67f7de7be5b0633e27916b1144978d1419c0abfcdb"),
				BuilderPullPolicy: api.DefaultBuilderPullPolicy,
			},
			[]Error{},
		},
		{
			&api.Config{
				Source:            "",
				BuilderImage:      "openshift/builder",
				DockerConfig:      &api.DockerConfig{Endpoint: "/var/run/docker.socket"},
				DockerNetworkMode: api.NewDockerNetworkModeContainer("8d873e496bc3e80a1cb22e67f7de7be5b0633e27916b1144978d1419c0abfcdb"),
				BuilderPullPolicy: api.DefaultBuilderPullPolicy,
			},
			[]Error{},
		},
		{
			&api.Config{
				Source:            "http://github.com/openshift/source",
				BuilderImage:      "openshift/builder",
				DockerConfig:      &api.DockerConfig{Endpoint: "/var/run/docker.socket"},
				BuilderPullPolicy: api.DefaultBuilderPullPolicy,
				Labels:            nil,
			},
			[]Error{},
		},
		{
			&api.Config{
				Source:            "http://github.com/openshift/source",
				BuilderImage:      "openshift/builder",
				DockerConfig:      &api.DockerConfig{Endpoint: "/var/run/docker.socket"},
				BuilderPullPolicy: api.DefaultBuilderPullPolicy,
				Labels:            map[string]string{},
			},
			[]Error{},
		},
		{
			&api.Config{
				Source:            "http://github.com/openshift/source",
				BuilderImage:      "openshift/builder",
				DockerConfig:      &api.DockerConfig{Endpoint: "/var/run/docker.socket"},
				BuilderPullPolicy: api.DefaultBuilderPullPolicy,
				Labels:            map[string]string{"some": "thing", "other": "value"},
			},
			[]Error{},
		},
		{
			&api.Config{
				Source:            "http://github.com/openshift/source",
				BuilderImage:      "openshift/builder",
				DockerConfig:      &api.DockerConfig{Endpoint: "/var/run/docker.socket"},
				BuilderPullPolicy: api.DefaultBuilderPullPolicy,
				Labels:            map[string]string{"some": "thing", "": "emptykey"},
			},
			[]Error{{ErrorInvalidValue, "labels"}},
		},
	}
	for _, test := range testCases {
		result := ValidateConfig(test.value)
		if !reflect.DeepEqual(result, test.expected) {
			t.Errorf("got %+v, expected %+v", result, test.expected)
		}
	}
}

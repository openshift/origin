package configuration

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/docker/distribution/configuration"
)

var (
	// CurrentVersion is the most recent Version that can be parsed.
	CurrentVersion = configuration.MajorMinorVersion(1, 0)

	ErrUnsupportedVersion = errors.New("Unsupported openshift configuration version")
)

type openshiftConfig struct {
	Openshift Configuration
}

type Configuration struct {
	Version  configuration.Version `yaml:"version"`
	Metrics  Metrics               `yaml:"metrics"`
	Requests Requests              `yaml:"requests"`
}

type Metrics struct {
	Enabled bool   `yaml:"enabled"`
	Secret  string `yaml:"secret"`
}

type Requests struct {
	Read  RequestsLimits `yaml:"read"`
	Write RequestsLimits `yaml:"write"`
}

type RequestsLimits struct {
	MaxRunning     int           `yaml:"maxrunning"`
	MaxInQueue     int           `yaml:"maxinqueue"`
	MaxWaitInQueue time.Duration `yaml:"maxwaitinqueue"`
}

type versionInfo struct {
	Openshift struct {
		Version *configuration.Version
	}
}

// Parse parses an input configuration and returns docker configuration structure and
// openshift specific configuration.
// Environment variables may be used to override configuration parameters.
func Parse(rd io.Reader) (*configuration.Configuration, *Configuration, error) {
	in, err := ioutil.ReadAll(rd)
	if err != nil {
		return nil, nil, err
	}

	// We don't want to change the version from the environment variables.
	os.Unsetenv("REGISTRY_OPENSHIFT_VERSION")

	openshiftEnv, err := popEnv("REGISTRY_OPENSHIFT_")
	if err != nil {
		return nil, nil, err
	}

	dockerConfig, err := configuration.Parse(bytes.NewBuffer(in))
	if err != nil {
		return nil, nil, err
	}

	dockerEnv, err := popEnv("REGISTRY_")
	if err != nil {
		return nil, nil, err
	}
	if err := pushEnv(openshiftEnv); err != nil {
		return nil, nil, err
	}

	config := openshiftConfig{}

	vInfo := &versionInfo{}
	if err := yaml.Unmarshal(in, &vInfo); err != nil {
		return nil, nil, err
	}

	if vInfo.Openshift.Version != nil {
		if *vInfo.Openshift.Version != CurrentVersion {
			return nil, nil, ErrUnsupportedVersion
		}
	} else {
		return dockerConfig, &config.Openshift, nil
	}

	p := configuration.NewParser("registry", []configuration.VersionedParseInfo{
		{
			Version: dockerConfig.Version,
			ParseAs: reflect.TypeOf(config),
			ConversionFunc: func(c interface{}) (interface{}, error) {
				return c, nil
			},
		},
	})

	if err = p.Parse(in, &config); err != nil {
		return nil, nil, err
	}
	if err := pushEnv(dockerEnv); err != nil {
		return nil, nil, err
	}

	return dockerConfig, &config.Openshift, nil
}

type envVar struct {
	name  string
	value string
}

func popEnv(prefix string) ([]envVar, error) {
	var envVars []envVar

	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, prefix) {
			continue
		}
		envParts := strings.SplitN(env, "=", 2)
		err := os.Unsetenv(envParts[0])
		if err != nil {
			return nil, err
		}

		envVars = append(envVars, envVar{envParts[0], envParts[1]})
	}

	return envVars, nil
}

func pushEnv(environ []envVar) error {
	for _, env := range environ {
		if err := os.Setenv(env.name, env.value); err != nil {
			return err
		}
	}
	return nil
}

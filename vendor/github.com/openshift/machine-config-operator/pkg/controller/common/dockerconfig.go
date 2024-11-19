package common

import (
	"fmt"

	"github.com/clarketm/json"
)

// This file contains several functions for working with Docker image pull
// configs. Instead of maintaining our own implementation here, we should
// instead use the implementations found under:
//
// - https://github.com/containers/image/blob/main/pkg/docker/config/config.go
// - https://github.com/kubernetes/kubernetes/blob/master/pkg/credentialprovider/config.go
//
// These more official implementations are more aware of edgecases than our
// naive implementation here.

// DockerConfigJSON represents ~/.docker/config.json file info
type DockerConfigJSON struct {
	Auths DockerConfig `json:"auths"`
}

// DockerConfig represents the config file used by the docker CLI.
// This config that represents the credentials that should be used
// when pulling images from specific image repositories.
type DockerConfig map[string]DockerConfigEntry

// DockerConfigEntry wraps a docker config as a entry
type DockerConfigEntry struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Auth     string `json:"auth"`
}

// Merges kubernetes.io/dockercfg type secrets into a JSON map.
// Returns an error on failure to marshal the incoming secret.
func MergeDockerConfigstoJSONMap(secretRaw []byte, auths map[string]DockerConfigEntry) error {
	var dockerConfig DockerConfig
	// Unmarshal raw JSON
	err := json.Unmarshal(secretRaw, &dockerConfig)
	if err != nil {
		return fmt.Errorf(" unmarshal failure: %w", err)
	}
	// Step through the hosts and add them to the JSON map
	for host := range dockerConfig {
		auths[host] = dockerConfig[host]
	}
	return nil
}

// Converts a kubernetes.io/dockerconfigjson type secret to a
// kubernetes.io/dockercfg type secret. Returns an error on failure
// if the incoming secret is not formatted correctly.
func ConvertSecretTodockercfg(secretBytes []byte) ([]byte, error) {
	type newStyleAuth struct {
		Auths map[string]interface{} `json:"auths,omitempty"`
	}

	// Un-marshal the new-style secret first
	newStyleDecoded := &newStyleAuth{}
	if err := json.Unmarshal(secretBytes, newStyleDecoded); err != nil {
		return nil, fmt.Errorf("could not decode new-style pull secret: %w", err)
	}

	// Marshal with old style, which is everything inside the Auths field
	out, err := json.Marshal(newStyleDecoded.Auths)

	return out, err
}

// Converts a legacy Docker pull secret into a more modern representation.
// Essentially, it converts {"registry.hostname.com": {"username": "user"...}}
// into {"auths": {"registry.hostname.com": {"username": "user"...}}}. If it
// encounters a pull secret already in this configuration, it will return the
// input secret as-is. Returns either the supplied data or the newly-configured
// representation of said data, a boolean to indicate whether it was converted,
// and any errors resulting from the conversion process.
func ConvertSecretToDockerconfigJSON(secretBytes []byte) ([]byte, bool, error) {
	type newStyleAuth struct {
		Auths map[string]interface{} `json:"auths,omitempty"`
	}

	// Try marshaling the new-style secret first:
	newStyleDecoded := &newStyleAuth{}
	if err := json.Unmarshal(secretBytes, newStyleDecoded); err != nil {
		return nil, false, fmt.Errorf("could not decode new-style pull secret: %w", err)
	}

	// We have an new-style secret, so we can just return here.
	if len(newStyleDecoded.Auths) != 0 {
		return secretBytes, false, nil
	}

	// We need to convert the legacy-style secret to the new-style.
	oldStyleDecoded := map[string]interface{}{}
	if err := json.Unmarshal(secretBytes, &oldStyleDecoded); err != nil {
		return nil, false, fmt.Errorf("could not decode legacy-style pull secret: %w", err)
	}

	out, err := json.Marshal(&newStyleAuth{
		Auths: oldStyleDecoded,
	})

	return out, err == nil, err
}

// Converts a provided secret into a kubernetes.io/dockerconfigjson secret then
// unmarshals it into the appropriate data structure. This means it will handle
// both legacy and current-style Docker configs.
func ToDockerConfigJSON(secretBytes []byte) (*DockerConfigJSON, error) {
	outBytes, _, err := ConvertSecretToDockerconfigJSON(secretBytes)
	if err != nil {
		return nil, err
	}

	out := &DockerConfigJSON{}
	if err := json.Unmarshal(outBytes, out); err != nil {
		return nil, err
	}

	return out, nil
}

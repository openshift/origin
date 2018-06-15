package build

import (
	"errors"
	"fmt"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/docker"
	"github.com/openshift/source-to-image/pkg/scm/git"
)

// GenerateConfigFromLabels generates the S2I Config struct from the Docker
// image labels.
func GenerateConfigFromLabels(config *api.Config, metadata *docker.PullResult) error {
	if config == nil {
		return errors.New("config must be provided to GenerateConfigFromLabels")
	}
	if metadata == nil {
		return errors.New("image metadata must be provided to GenerateConfigFromLabels")
	}

	labels := metadata.Image.Config.Labels

	if builderVersion, ok := labels["io.openshift.builder-version"]; ok {
		config.BuilderImageVersion = builderVersion
		config.BuilderBaseImageVersion = labels["io.openshift.builder-base-version"]
	}

	config.ScriptsURL = labels[api.DefaultNamespace+"scripts-url"]
	if len(config.ScriptsURL) == 0 {
		// FIXME: Backward compatibility
		config.ScriptsURL = labels["io.s2i.scripts-url"]
	}

	config.Description = labels[api.KubernetesNamespace+"description"]
	config.DisplayName = labels[api.KubernetesNamespace+"display-name"]

	if builder, ok := labels[api.DefaultNamespace+"build.image"]; ok {
		config.BuilderImage = builder
	} else {
		return fmt.Errorf("required label %q not found in image", api.DefaultNamespace+"build.image")
	}

	if repo, ok := labels[api.DefaultNamespace+"build.source-location"]; ok {
		source, err := git.Parse(repo)
		if err != nil {
			return fmt.Errorf("couldn't parse label %q value %s: %v", api.DefaultNamespace+"build.source-location", repo, err)
		}
		config.Source = source
	} else {
		return fmt.Errorf("required label %q not found in image", api.DefaultNamespace+"build.source-location")
	}

	config.ContextDir = labels[api.DefaultNamespace+"build.source-context-dir"]
	config.Source.URL.Fragment = labels[api.DefaultNamespace+"build.commit.ref"]

	return nil
}

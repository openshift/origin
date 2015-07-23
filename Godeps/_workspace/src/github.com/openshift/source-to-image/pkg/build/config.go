package build

import (
	"fmt"

	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/docker"
)

// GenerateConfigFromLabels generates the S2I Config struct from the Docker
// image labels.
func GenerateConfigFromLabels(image string, config *api.Config) error {
	d, err := docker.New(config.DockerConfig, config.PullAuthentication)
	if err != nil {
		return err
	}

	var source *dockerclient.Image
	if config.ForcePull {
		source, err = d.PullImage(image)
	} else {
		source, err = d.CheckAndPullImage(image)
	}

	if err != nil {
		return err
	}

	if builderVersion, ok := source.Config.Labels["io.openshift.builder-version"]; ok {
		config.BuilderImageVersion = builderVersion
		config.BuilderBaseImageVersion = source.Config.Labels["io.openshift.builder-base-version"]
	}

	config.ScriptsURL = source.Config.Labels[api.DefaultNamespace+"scripts-url"]
	if len(config.ScriptsURL) == 0 {
		// FIXME: Backward compatibility
		config.ScriptsURL = source.Config.Labels["io.s2i.scripts-url"]
	}

	config.Description = source.Config.Labels[api.KubernetesNamespace+"description"]
	config.DisplayName = source.Config.Labels[api.KubernetesNamespace+"display-name"]

	if builder, ok := source.Config.Labels[api.DefaultNamespace+"build.image"]; ok {
		config.BuilderImage = builder
	} else {
		return fmt.Errorf("Required label %q not found in image", api.DefaultNamespace+"build.image")
	}

	if repo, ok := source.Config.Labels[api.DefaultNamespace+"build.source-location"]; ok {
		config.Source = repo
	} else {
		return fmt.Errorf("Required label %q not found in image", api.DefaultNamespace+"source-location")
	}

	config.ContextDir = source.Config.Labels[api.DefaultNamespace+"build.source-context-dir"]
	config.Ref = source.Config.Labels[api.DefaultNamespace+"build.commit.ref"]

	return nil
}

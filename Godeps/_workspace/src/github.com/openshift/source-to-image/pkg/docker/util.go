package docker

import (
	"fmt"
	"io"
	"strings"

	client "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
)

// DockerImageReference points to a Docker image.
type DockerImageReference struct {
	Registry  string
	Namespace string
	Name      string
	Tag       string
	ID        string
}

const defaultRegistry = "https://index.docker.io/v1/"

func GetImageRegistryAuth(dockerCfg io.Reader, imageName string) client.AuthConfiguration {
	spec, err := ParseDockerImageReference(imageName)
	if err != nil {
		return client.AuthConfiguration{}
	}
	if auths, err := client.NewAuthConfigurations(dockerCfg); err == nil {
		if auth, ok := auths.Configs[spec.Registry]; ok {
			glog.V(5).Infof("Using %s[%s] credentials for pulling %s", auth.Email, spec.Registry, imageName)
			return auth
		}
		if auth, ok := auths.Configs[defaultRegistry]; ok {
			glog.V(5).Infof("Using %s credentials for pulling %s", auth.Email, imageName)
			return auth
		}
	}
	return client.AuthConfiguration{}
}

// ParseDockerImageReference parses a Docker pull spec string into a
// DockerImageReference.
// FIXME: This code was copied from OpenShift repository
func ParseDockerImageReference(spec string) (DockerImageReference, error) {
	var (
		ref DockerImageReference
	)
	// TODO replace with docker version once docker/docker PR11109 is merged upstream
	stream, tag, id := parseRepositoryTag(spec)

	repoParts := strings.Split(stream, "/")
	switch len(repoParts) {
	case 2:
		if strings.Contains(repoParts[0], ":") {
			// registry/name
			ref.Registry = repoParts[0]
			ref.Namespace = "library"
			ref.Name = repoParts[1]
			ref.Tag = tag
			ref.ID = id
			return ref, nil
		}
		// namespace/name
		ref.Namespace = repoParts[0]
		ref.Name = repoParts[1]
		ref.Tag = tag
		ref.ID = id
		return ref, nil
	case 3:
		// registry/namespace/name
		ref.Registry = repoParts[0]
		ref.Namespace = repoParts[1]
		ref.Name = repoParts[2]
		ref.Tag = tag
		ref.ID = id
		return ref, nil
	case 1:
		// name
		if len(repoParts[0]) == 0 {
			return ref, fmt.Errorf("the docker pull spec %q must be two or three segments separated by slashes", spec)
		}
		ref.Name = repoParts[0]
		ref.Tag = tag
		ref.ID = id
		return ref, nil
	default:
		return ref, fmt.Errorf("the docker pull spec %q must be two or three segments separated by slashes", spec)
	}
}

// TODO remove (base, tag, id)
func parseRepositoryTag(repos string) (string, string, string) {
	n := strings.Index(repos, "@")
	if n >= 0 {
		parts := strings.Split(repos, "@")
		return parts[0], "", parts[1]
	}
	n = strings.LastIndex(repos, ":")
	if n < 0 {
		return repos, "", ""
	}
	if tag := repos[n+1:]; !strings.Contains(tag, "/") {
		return repos[:n], tag, ""
	}
	return repos, "", ""
}

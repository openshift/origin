package docker

import (
	"fmt"
	"io"
	"strings"

	"bufio"
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

const (
	// maxErrorOutput is the maximum length of the error output saved for processing
	maxErrorOutput  = 1024
	defaultRegistry = "https://index.docker.io/v1/"
)

// GetImageRegistryAuth retrieves the appropriate docker client authentication object for a given
// image name and a given set of client authentication objects.
func GetImageRegistryAuth(auths *client.AuthConfigurations, imageName string) client.AuthConfiguration {
	glog.V(5).Infof("Getting docker credentials for %s", imageName)
	spec, err := ParseDockerImageReference(imageName)
	if err != nil {
		glog.Errorf("Failed to parse docker reference %s", imageName)
		return client.AuthConfiguration{}
	}

	if auth, ok := auths.Configs[spec.Registry]; ok {
		glog.V(5).Infof("Using %s[%s] credentials for pulling %s", auth.Email, spec.Registry, imageName)
		return auth
	}
	if auth, ok := auths.Configs[defaultRegistry]; ok {
		glog.V(5).Infof("Using %s credentials for pulling %s", auth.Email, imageName)
		return auth
	}
	return client.AuthConfiguration{}
}

// LoadImageRegistryAuth loads and returns the set of client auth objects from a docker config
// json file.
func LoadImageRegistryAuth(dockerCfg io.Reader) *client.AuthConfigurations {
	auths, err := client.NewAuthConfigurations(dockerCfg)
	if err != nil {
		glog.Errorf("Unable to load docker config")
		return nil
	}
	return auths
}

// LoadAndGetImageRegistryAuth loads the set of client auth objects from a docker config file
// and returns the appropriate client auth object for a given image name.
func LoadAndGetImageRegistryAuth(dockerCfg io.Reader, imageName string) client.AuthConfiguration {
	auths, err := client.NewAuthConfigurations(dockerCfg)
	if err != nil {
		glog.Errorf("Unable to load docker config")
		return client.AuthConfiguration{}
	}
	return GetImageRegistryAuth(auths, imageName)
}

// StreamContainerIO takes data from the Reader and redirects to the log functin (typically we pass in
// glog.Error for stderr and glog.Info for stdout
func StreamContainerIO(errStream io.Reader, errOutput *string, log func(...interface{})) {
	scanner := bufio.NewReader(errStream)
	for {
		text, err := scanner.ReadString('\n')
		if err != nil {
			// we're ignoring ErrClosedPipe, as this is information
			// the docker container ended streaming logs
			if err != io.ErrClosedPipe && err != io.EOF {
				glog.Errorf("Error reading docker stderr, %v", err)
			}
			break
		}
		log(text)
		if errOutput != nil && len(*errOutput) < maxErrorOutput {
			*errOutput += text + "\n"
		}
	}
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

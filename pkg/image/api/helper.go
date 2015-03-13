package api

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DockerDefaultNamespace is the value for namespace when a single segment name is provided.
const DockerDefaultNamespace = "library"

// DockerImageReference points to a Docker image.
type DockerImageReference struct {
	Registry  string
	Namespace string
	Name      string
	Tag       string
	ID        string
}

// COPIED from upstream
// TODO remove
func parseRepositoryTag(repos string) (string, string) {
	n := strings.Index(repos, "@")
	if n >= 0 {
		parts := strings.Split(repos, "@")
		return parts[0], parts[1]
	}
	n = strings.LastIndex(repos, ":")
	if n < 0 {
		return repos, ""
	}
	if tag := repos[n+1:]; !strings.Contains(tag, "/") {
		return repos[:n], tag
	}
	return repos, ""
}

// ParseDockerImageReference parses a Docker pull spec string into a
// DockerImageReference.
func ParseDockerImageReference(spec string) (DockerImageReference, error) {
	var (
		ref     DockerImageReference
		tag, id string
	)
	// TODO replace with docker version once docker/docker PR11109 is merged upstream
	repo, tagOrID := parseRepositoryTag(spec)
	if strings.Contains(tagOrID, ":") {
		id = tagOrID
	} else {
		tag = tagOrID
	}

	repoParts := strings.Split(repo, "/")
	switch len(repoParts) {
	case 2:
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

// DockerClientDefaults sets the default values used by the Docker client.
func (r DockerImageReference) DockerClientDefaults() DockerImageReference {
	if len(r.Namespace) == 0 {
		r.Namespace = "library"
	}
	if len(r.Registry) == 0 {
		r.Registry = "index.docker.io"
	}
	if len(r.Tag) == 0 {
		r.Tag = "latest"
	}
	return r
}

// String converts a DockerImageReference to a Docker pull spec.
func (r DockerImageReference) String() string {
	registry := r.Registry
	if len(registry) > 0 {
		registry += "/"
	}

	if len(r.Namespace) == 0 {
		r.Namespace = DockerDefaultNamespace
	}
	r.Namespace += "/"

	var ref string
	if len(r.Tag) > 0 {
		ref = ":" + r.Tag
	} else if len(r.ID) > 0 {
		ref = "@" + r.ID
	}

	return fmt.Sprintf("%s%s%s%s", registry, r.Namespace, r.Name, ref)
}

// ImageWithMetadata returns a copy of image with the DockerImageMetadata filled in
// from the raw DockerImageManifest data stored in the image.
func ImageWithMetadata(image Image) (*Image, error) {
	if len(image.DockerImageManifest) == 0 {
		return &image, nil
	}

	manifestData := image.DockerImageManifest

	image.DockerImageManifest = ""

	manifest := DockerImageManifest{}
	if err := json.Unmarshal([]byte(manifestData), &manifest); err != nil {
		return nil, err
	}

	if len(manifest.History) == 0 {
		// should never have an empty history, but just in case...
		return &image, nil
	}

	v1Metadata := DockerV1CompatibilityImage{}
	if err := json.Unmarshal([]byte(manifest.History[0].DockerV1Compatibility), &v1Metadata); err != nil {
		return nil, err
	}

	image.DockerImageMetadata.ID = v1Metadata.ID
	image.DockerImageMetadata.Parent = v1Metadata.Parent
	image.DockerImageMetadata.Comment = v1Metadata.Comment
	image.DockerImageMetadata.Created = v1Metadata.Created
	image.DockerImageMetadata.Container = v1Metadata.Container
	image.DockerImageMetadata.ContainerConfig = v1Metadata.ContainerConfig
	image.DockerImageMetadata.DockerVersion = v1Metadata.DockerVersion
	image.DockerImageMetadata.Author = v1Metadata.Author
	image.DockerImageMetadata.Config = v1Metadata.Config
	image.DockerImageMetadata.Architecture = v1Metadata.Architecture
	image.DockerImageMetadata.Size = v1Metadata.Size

	return &image, nil
}

// LatestTaggedImage returns the most recent TagEvent for the specified image
// repository and tag.
func LatestTaggedImage(repo ImageRepository, tag string) (*TagEvent, error) {
	if _, ok := repo.Tags[tag]; !ok {
		return nil, fmt.Errorf("image repository %s/%s: tag %q not found", repo.Namespace, repo.Name, tag)
	}

	tagHistory, ok := repo.Status.Tags[tag]
	if !ok {
		return nil, fmt.Errorf("image repository %s/%s: tag %q not found in tag history", repo.Namespace, repo.Name, tag)
	}

	if len(tagHistory.Items) == 0 {
		return nil, fmt.Errorf("image repository %s/%s: tag %q has 0 history items", repo.Namespace, repo.Name, tag)
	}

	return &tagHistory.Items[0], nil
}

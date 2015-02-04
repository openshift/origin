package app

import (
	"fmt"

	"github.com/fsouza/go-dockerclient"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/dockerregistry"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

type DockerClientResolver struct {
	Client *docker.Client
}

func (r DockerClientResolver) Resolve(value string) (*ComponentMatch, error) {
	image, err := r.Client.InspectImage(value)
	switch {
	case err == docker.ErrNoSuchImage:
		return nil, ErrNoMatch{value: value}
	case err != nil:
		return nil, err
	}
	return &ComponentMatch{
		Value:       value,
		Argument:    fmt.Sprintf("--docker-image=%q", value),
		Name:        value,
		Description: fmt.Sprintf("Docker image %q by %s\n%s", value, image.Author, image.Comment),
		Builder:     false,
		Score:       0,
	}, nil
}

type DockerRegistryResolver struct {
	Client dockerregistry.Client
}

func (r DockerRegistryResolver) Resolve(value string) (*ComponentMatch, error) {
	registry, namespace, name, tag, err := imageapi.SplitDockerPullSpec(value)
	if err != nil {
		return nil, err
	}
	connection, err := r.Client.Connect(registry)
	if err != nil {
		if dockerregistry.IsRegistryNotFound(err) {
			return nil, ErrNoMatch{value: value}
		}
		return nil, ErrNoMatch{value: value, qualifier: fmt.Sprintf("can't connect to %q: %v", registry, err)}
	}
	image, err := connection.ImageByTag(namespace, name, tag)
	if err != nil {
		if dockerregistry.IsNotFound(err) {
			return nil, ErrNoMatch{value: value, qualifier: err.Error()}
		}
		return nil, ErrNoMatch{value: value, qualifier: fmt.Sprintf("can't connect to %q: %v", registry, err)}
	}
	if len(tag) == 0 {
		tag = "latest"
	}
	glog.V(4).Infof("found image: %#v", image)
	dockerImage := &imageapi.DockerImage{}
	if err = kapi.Scheme.Convert(image, dockerImage); err != nil {
		return nil, err
	}
	return &ComponentMatch{
		Value:       value,
		Argument:    fmt.Sprintf("--docker-image=%q", value),
		Name:        value,
		Description: fmt.Sprintf("Docker image %q (%q)", value, image.ID),
		Builder:     IsBuilderImage(dockerImage),
		Score:       0,
		Image:       dockerImage,
		ImageTag:    tag,
	}, nil
}

type ImageStreamResolver struct {
	Client     client.ImageRepositoriesNamespacer
	Images     client.ImagesNamespacer
	Namespaces []string
}

func (r ImageStreamResolver) Resolve(value string) (*ComponentMatch, error) {
	registry, namespace, name, tag, err := imageapi.SplitOpenShiftPullSpec(value)
	if err != nil || len(registry) != 0 {
		return nil, fmt.Errorf("image repositories must be of the form [<namespace>/]<name>[:<tag>]")
	}
	namespaces := r.Namespaces
	if len(namespace) != 0 {
		namespaces = []string{namespace}
	}
	for _, namespace := range namespaces {
		glog.V(4).Infof("checking image stream %s/%s with tag %q", namespace, name, tag)
		repo, err := r.Client.ImageRepositories(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		searchTag := tag
		// TODO: move to a lookup function on repo, or better yet, have the repo.Status.Tags field automatically infer latest
		if len(searchTag) == 0 {
			searchTag = "latest"
		}
		id, ok := repo.Tags[searchTag]
		if !ok {
			if len(tag) == 0 {
				return nil, ErrNoMatch{value: value, qualifier: fmt.Sprintf("the default tag %q has not been set", searchTag)}
			}
			return nil, ErrNoMatch{value: value, qualifier: fmt.Sprintf("tag %q has not been set", tag)}
		}
		imageData, err := r.Images.Images(namespace).Get(id)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, ErrNoMatch{value: value, qualifier: fmt.Sprintf("tag %q is set, but image %q has been removed", tag, id)}
			}
			return nil, err
		}

		spec := imageapi.JoinDockerPullSpec("", namespace, name, tag)
		return &ComponentMatch{
			Value:       spec,
			Argument:    fmt.Sprintf("--image=%q", spec),
			Name:        name,
			Description: fmt.Sprintf("Image stream %s (tag %q) in namespace %s, tracks %q", name, searchTag, namespace, repo.Status.DockerImageRepository),
			Builder:     IsBuilderImage(&imageData.DockerImageMetadata),
			Score:       0,

			ImageStream: repo,
			Image:       &imageData.DockerImageMetadata,
			ImageTag:    searchTag,
		}, nil
	}
	return nil, ErrNoMatch{value: value}
}

type Searcher interface {
	Search(terms []string) ([]*ComponentMatch, error)
}

func InputImageFromMatch(match *ComponentMatch) (*ImageRef, error) {
	switch {
	case match.ImageStream != nil:
		input, err := ImageFromRepository(match.ImageStream, match.ImageTag)
		if err != nil {
			return nil, err
		}
		input.AsImageRepository = true
		input.Info = match.Image
		return input, nil

	case match.Image != nil:
		input, err := ImageFromName(match.Value, match.ImageTag)
		if err != nil {
			return nil, err
		}
		input.AsImageRepository = false
		input.Info = match.Image
		return input, nil

	default:
		return nil, fmt.Errorf("no image or image stream, can't setup a build")
	}
}

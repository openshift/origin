package new

import (
	"fmt"
	"strings"

	"github.com/fsouza/go-dockerclient"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/dockerregistry"
	"github.com/openshift/origin/pkg/generate/app"
	image "github.com/openshift/origin/pkg/image/api"
)

type dockerClientResolver struct {
	client *docker.Client
}

func (r dockerClientResolver) Resolve(value string) (*ComponentMatch, error) {
	image, err := r.client.InspectImage(value)
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

type dockerRegistryResolver struct {
	client dockerregistry.Client
}

func (r dockerRegistryResolver) Resolve(value string) (*ComponentMatch, error) {
	registry, namespace, name, tag, err := image.SplitDockerPullSpec(value)
	if err != nil {
		return nil, err
	}
	connection, err := r.client.Connect(registry)
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
	return &ComponentMatch{
		Value:       value,
		Argument:    fmt.Sprintf("--docker-image=%q", value),
		Name:        value,
		Description: fmt.Sprintf("Docker image %q (%q)", value, image.ID),
		Builder:     app.IsBuilderImage(image),
		Score:       0,

		Image:    image,
		ImageTag: tag,
	}, nil
}

type imageStreamResolver struct {
	client     client.ImageRepositoriesNamespacer
	images     client.ImagesNamespacer
	namespaces []string
}

func (r imageStreamResolver) Resolve(value string) (*ComponentMatch, error) {
	registry, namespace, name, tag, err := image.SplitOpenShiftPullSpec(value)
	if err != nil || len(registry) != 0 {
		return nil, fmt.Errorf("image repositories must be of the form [<namespace>/]<name>[:<tag>]")
	}
	namespaces := r.namespaces
	if len(namespace) != 0 {
		namespaces = []string{namespace}
	}
	for _, namespace := range namespaces {
		glog.V(4).Infof("checking image stream %s/%s with tag %q", namespace, name, tag)
		repo, err := r.client.ImageRepositories(namespace).Get(name)
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
		imageData, err := r.images.Images(namespace).Get(id)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, ErrNoMatch{value: value, qualifier: fmt.Sprintf("tag %q is set, but image %q has been removed", tag, id)}
			}
			return nil, err
		}

		spec := image.JoinDockerPullSpec("", namespace, name, tag)
		return &ComponentMatch{
			Value:       spec,
			Argument:    fmt.Sprintf("--image=%q", spec),
			Name:        name,
			Description: fmt.Sprintf("Image stream %s (tag %q) in namespace %s, tracks %q", name, searchTag, namespace, repo.Status.DockerImageRepository),
			Builder:     app.IsBuilderImage(&imageData.DockerImageMetadata),
			Score:       0,

			ImageStream: repo,
			Image:       &imageData.DockerImageMetadata,
			ImageTag:    searchTag,
		}, nil
	}
	return nil, ErrNoMatch{value: value}
}

type mockResolver struct{}

func (mockResolver) Resolve(value string) (*ComponentMatch, error) {
	matches, err := mockSearcher{}.Search([]string{value})
	switch {
	case err != nil:
		return nil, err
	case len(matches) > 1:
		return nil, ErrMultipleMatches{value, matches}
	case len(matches) == 0:
		return nil, ErrNoMatch{value: value}
	default:
		return matches[0], nil
	}
}

type Searcher interface {
	Search(terms []string) ([]*ComponentMatch, error)
}

type mockSearcher struct{}

func (mockSearcher) Search(terms []string) ([]*ComponentMatch, error) {
	for _, term := range terms {
		term = strings.ToLower(term)
		switch term {
		case "redhat/mysql:5.6":
			return []*ComponentMatch{
				{
					Value:       term,
					Argument:    "redhat/mysql:5.6",
					Name:        "MySQL 5.6",
					Description: "The Open Source SQL database",
				},
			}, nil
		case "mysql", "mysql5", "mysql-5", "mysql-5.x":
			return []*ComponentMatch{
				{
					Value:       term,
					Argument:    "redhat/mysql:5.6",
					Name:        "MySQL 5.6",
					Description: "The Open Source SQL database",
				},
				{
					Value:       term,
					Argument:    "mysql",
					Name:        "MySQL 5.X",
					Description: "Something out there on the Docker Hub.",
				},
			}, nil
		case "php", "php-5", "php5", "redhat/php:5", "redhat/php-5":
			return []*ComponentMatch{
				{
					Value:       term,
					Argument:    "redhat/php:5",
					Name:        "PHP 5.5",
					Description: "A fast and easy to use scripting language for building websites.",
					Builder:     true,
				},
			}, nil
		case "ruby":
			return []*ComponentMatch{
				{
					Value:       term,
					Argument:    "redhat/ruby:2",
					Name:        "Ruby 2.0",
					Description: "A fast and easy to use scripting language for building websites.",
					Builder:     true,
				},
			}, nil
		}
	}
	return []*ComponentMatch{}, nil
}

func InputImageFromMatch(match *ComponentMatch) (*app.ImageRef, error) {
	switch {
	case match.ImageStream != nil:
		input, err := app.ImageFromRepository(match.ImageStream, match.ImageTag)
		if err != nil {
			return nil, err
		}
		input.AsImageRepository = true
		input.Info = match.Image
		return input, nil

	case match.Image != nil:
		input, err := app.ImageFromName(match.Value, match.ImageTag)
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

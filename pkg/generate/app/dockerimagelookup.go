package app

import (
	"fmt"
	"sort"
	"strings"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	utilerrors "k8s.io/kubernetes/pkg/util/errors"

	"github.com/openshift/origin/pkg/dockerregistry"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// DockerClient is the local interface for the docker client
type DockerClient interface {
	ListImages(opts docker.ListImagesOptions) ([]docker.APIImages, error)
	InspectImage(name string) (*docker.Image, error)
}

// DockerClientSearcher finds local docker images locally that match a search value
type DockerClientSearcher struct {
	Client DockerClient

	// Optional, will delegate resolution to the registry if no local
	// exact matches are found.
	RegistrySearcher Searcher

	// Insecure, if true will add an annotation to generated ImageStream
	// so that the image can be pulled properly
	Insecure bool

	// AllowingMissingImages will allow images that could not be found in the local or
	// remote registry to be used anyway.
	AllowMissingImages bool
}

// Search searches all images in local docker server for images that match terms
func (r DockerClientSearcher) Search(terms ...string) (ComponentMatches, error) {
	componentMatches := ComponentMatches{}
	errs := []error{}
	for _, term := range terms {
		ref, err := imageapi.ParseDockerImageReference(term)
		if err != nil {
			return nil, err
		}

		termMatches := ScoredComponentMatches{}

		// first look for the image in the remote docker registry
		if r.RegistrySearcher != nil {
			glog.V(4).Infof("checking remote registry for %q", ref.String())
			matches, err := r.RegistrySearcher.Search(term)
			switch err.(type) {
			case nil:
				for i := range matches {
					matches[i].LocalOnly = false
					glog.V(5).Infof("Found remote match %v", matches[i].Value)
				}
				termMatches = append(termMatches, matches...)
			case ErrNoMatch:
			default:
				return nil, err
			}
		}

		// if we didn't find it exactly in a remote registry,
		// try to find it as a local-only image.
		if len(termMatches.Exact()) == 0 {
			glog.V(4).Infof("checking local Docker daemon for %q", ref.String())
			images, err := r.Client.ListImages(docker.ListImagesOptions{})
			if err != nil {
				return nil, err
			}

			if len(ref.Registry) == 0 {
				ref.Registry = "local Docker"
			}
			if len(ref.Tag) == 0 {
				ref.Tag = imageapi.DefaultImageTag
			}
			for _, image := range images {
				if tags := matchTag(image, term, ref.Registry, ref.Namespace, ref.Name, ref.Tag); len(tags) > 0 {
					for i := range tags {
						tags[i].LocalOnly = true
						glog.V(5).Infof("Found local match %v", tags[i].Value)
					}
					termMatches = append(termMatches, tags...)
				}
			}
		}
		sort.Sort(termMatches)

		for i, match := range termMatches {
			if match.Image != nil {
				continue
			}

			image, err := r.Client.InspectImage(match.Value)
			if err != nil {
				if err != docker.ErrNoSuchImage {
					errs = append(errs, err)
				}
				continue
			}
			dockerImage := &imageapi.DockerImage{}
			if err := kapi.Scheme.Convert(image, dockerImage); err != nil {
				return nil, err
			}
			updated := &ComponentMatch{
				Value:       match.Value,
				Argument:    fmt.Sprintf("--docker-image=%q", match.Value),
				Name:        match.Value,
				Description: descriptionFor(dockerImage, match.Value, ref.Registry, ""),
				Builder:     IsBuilderImage(dockerImage),
				Score:       match.Score,
				Image:       dockerImage,
				ImageTag:    ref.Tag,
				Insecure:    r.Insecure,
				Meta:        map[string]string{"registry": ref.Registry},
				LocalOnly:   match.LocalOnly,
			}
			termMatches[i] = updated
		}

		componentMatches = append(componentMatches, termMatches...)

		// if we didn't find it remotely or locally, but the user chose to
		// allow missing images, create an exact match for the value they
		// provided.
		if len(componentMatches) == 0 && r.AllowMissingImages {
			componentMatches = append(componentMatches, &ComponentMatch{
				Value:     term,
				Score:     0.0,
				Builder:   true,
				LocalOnly: true,
			})
			glog.V(4).Infof("Appended missing match %v", term)

		}
	}

	if len(errs) != 0 {
		return nil, utilerrors.NewAggregate(errs)
	}

	return componentMatches, nil
}

// DockerRegistrySearcher searches for images in a given docker registry.
// Notice that it only matches exact searches - so a search for "rub" will
// not return images with the name "ruby".
// TODO: replace ImageByTag to allow partial matches
type DockerRegistrySearcher struct {
	Client        dockerregistry.Client
	AllowInsecure bool
}

// Search searches in the Docker registry for images that match terms
func (r DockerRegistrySearcher) Search(terms ...string) (ComponentMatches, error) {
	componentMatches := ComponentMatches{}
	for _, term := range terms {
		ref, err := imageapi.ParseDockerImageReference(term)
		if err != nil {
			return nil, err
		}

		glog.V(4).Infof("checking Docker registry for %q, allow-insecure=%v", ref.String(), r.AllowInsecure)
		connection, err := r.Client.Connect(ref.Registry, r.AllowInsecure)
		if err != nil {
			if dockerregistry.IsRegistryNotFound(err) {
				return nil, ErrNoMatch{value: term}
			}
			return nil, fmt.Errorf("can't connect to %q: %v", ref.Registry, err)
		}

		image, err := connection.ImageByTag(ref.Namespace, ref.Name, ref.Tag)
		if err != nil {
			if dockerregistry.IsNotFound(err) {
				return nil, ErrNoMatch{value: term, qualifier: err.Error()}
			}
			return nil, fmt.Errorf("can't connect to %q: %v", ref.Registry, err)
		}

		if len(ref.Tag) == 0 {
			ref.Tag = imageapi.DefaultImageTag
		}
		if len(ref.Registry) == 0 {
			ref.Registry = "Docker Hub"
		}
		glog.V(4).Infof("found image: %#v", image)

		dockerImage := &imageapi.DockerImage{}
		if err = kapi.Scheme.Convert(&image.Image, dockerImage); err != nil {
			return nil, err
		}

		componentMatches = append(componentMatches, &ComponentMatch{
			Value:       term,
			Argument:    fmt.Sprintf("--docker-image=%q", term),
			Name:        term,
			Description: descriptionFor(dockerImage, term, ref.Registry, ref.Tag),
			Builder:     IsBuilderImage(dockerImage),
			Score:       0,
			Image:       dockerImage,
			ImageTag:    ref.Tag,
			Meta:        map[string]string{"registry": ref.Registry},
		})
	}

	return componentMatches, nil
}

func descriptionFor(image *imageapi.DockerImage, value, from string, tag string) string {
	shortID := image.ID
	if len(shortID) > 7 {
		shortID = shortID[:7]
	}
	tagPart := ""
	if len(tag) > 0 {
		tagPart = fmt.Sprintf(" (tag %q)", tag)
	}
	parts := []string{fmt.Sprintf("Docker image %q%v", value, tagPart), shortID, fmt.Sprintf("from %s", from)}
	if image.Size > 0 {
		mb := float64(image.Size) / float64(1024*1024)
		parts = append(parts, fmt.Sprintf("%f", mb))
	}
	if len(image.Author) > 0 {
		parts = append(parts, fmt.Sprintf("author %s", image.Author))
	}
	if len(image.Comment) > 0 {
		parts = append(parts, image.Comment)
	}
	return strings.Join(parts, ", ")
}

func matchTag(image docker.APIImages, value, registry, namespace, name, tag string) []*ComponentMatch {
	if len(tag) == 0 {
		tag = imageapi.DefaultImageTag
	}
	matches := []*ComponentMatch{}
	for _, s := range image.RepoTags {
		if value == s {
			matches = append(matches, &ComponentMatch{
				Value: s,
				Score: 0.0,
			})
			continue
		}
		iRef, err := imageapi.ParseDockerImageReference(s)
		if err != nil {
			continue
		}
		if len(iRef.Tag) == 0 {
			iRef.Tag = imageapi.DefaultImageTag
		}
		match := &ComponentMatch{}
		ok, score := partialScorer(name, iRef.Name, true, 0.5, 1.0)
		if !ok {
			continue
		}
		match.Score += score
		_, score = partialScorer(namespace, iRef.Namespace, false, 0.5, 1.0)
		match.Score += score
		_, score = partialScorer(registry, iRef.Registry, false, 0.5, 1.0)
		match.Score += score
		_, score = partialScorer(tag, iRef.Tag, false, 0.5, 1.0)
		match.Score += score

		if match.Score >= 4.0 {
			continue
		}
		match.Score = match.Score / 4.0
		glog.V(4).Infof("partial match on %q with %f", s, match.Score)
		match.Value = s
		match.Meta = map[string]string{"registry": registry}
		matches = append(matches, match)
	}
	return matches
}

// PassThroughDockerSearcher returns a match with the value that was passed in
type PassThroughDockerSearcher struct{}

// Search always returns a match for every term passed in
func (r *PassThroughDockerSearcher) Search(terms ...string) (ComponentMatches, error) {
	matches := ComponentMatches{}
	for _, value := range terms {
		matches = append(matches, &ComponentMatch{
			Value:       value,
			Name:        value,
			Argument:    fmt.Sprintf("--docker-image=%q", value),
			Description: fmt.Sprintf("Docker image %q", value),
			Score:       1.0,
		})
	}
	return matches, nil
}

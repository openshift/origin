package app

import (
	"fmt"
	"sort"
	"strings"

	docker "github.com/fsouza/go-dockerclient"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	utilerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/dockerregistry"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// DockerClient is the local interface for the docker client
type DockerClient interface {
	ListImages(opts docker.ListImagesOptions) ([]docker.APIImages, error)
	InspectImage(name string) (*docker.Image, error)
}

// DockerClientResolver finds local docker images locally that match a search value
type DockerClientResolver struct {
	Client DockerClient

	// Optional, will delegate resolution to the registry if no local
	// exact matches are found.
	RegistryResolver Resolver

	// Insecure, if true will add an annotation to generated ImageStream
	// so that the image can be pulled properly
	Insecure bool
}

// Resolve searches all images in local docker server for an image that matches the passed in value
func (r DockerClientResolver) Resolve(value string) (*ComponentMatch, error) {
	ref, err := imageapi.ParseDockerImageReference(value)
	if err != nil {
		return nil, err
	}

	glog.V(4).Infof("checking local Docker daemon for %q", ref.String())
	images, err := r.Client.ListImages(docker.ListImagesOptions{})
	if err != nil {
		return nil, err
	}
	matches := ScoredComponentMatches{}
	for _, image := range images {
		if tags := matchTag(image, value, ref.Registry, ref.Namespace, ref.Name, ref.Tag); len(tags) > 0 {
			matches = append(matches, tags...)
		}
	}
	sort.Sort(matches)
	if exact := matches.Exact(); len(exact) > 0 {
		matches = exact
	} else {
		if r.RegistryResolver != nil {
			match, err := r.RegistryResolver.Resolve(value)
			switch err.(type) {
			case nil:
				return match, nil
			case ErrNoMatch:
				// show our partial matches
			case ErrMultipleMatches:
				return nil, err
			default:
				return nil, err
			}
		}
	}

	errs := []error{}
	for i, match := range matches {
		if match.Image != nil {
			continue
		}
		updated, err := r.lookup(match.Value)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		updated.Score = match.Score
		updated.ImageTag = ref.Tag
		updated.Insecure = r.Insecure
		matches[i] = updated
	}

	if len(errs) != 0 {
		if len(errs) == 1 {
			err := errs[0]
			if err == docker.ErrNoSuchImage {
				return nil, ErrNoMatch{value: value}
			}
			return nil, err
		}
		return nil, utilerrors.NewAggregate(errs)
	}

	switch len(matches) {
	case 0:
		return nil, ErrNoMatch{value: value}
	case 1:
		return matches[0], nil
	default:
		return nil, ErrMultipleMatches{Image: value, Matches: matches}
	}
}

func (r DockerClientResolver) lookup(value string) (*ComponentMatch, error) {
	image, err := r.Client.InspectImage(value)
	if err != nil {
		return nil, err
	}
	dockerImage := &imageapi.DockerImage{}
	if err := kapi.Scheme.Convert(image, dockerImage); err != nil {
		return nil, err
	}
	return &ComponentMatch{
		Value:       value,
		Argument:    fmt.Sprintf("--docker-image=%q", value),
		Name:        value,
		Description: descriptionFor(dockerImage, value, "local Docker"),
		Builder:     IsBuilderImage(dockerImage),
		Score:       0.0,
		Image:       dockerImage,
		Insecure:    r.Insecure,
	}, nil
}

// DockerRegistryResolver searches for images in a given docker registry
type DockerRegistryResolver struct {
	Client dockerregistry.Client

	AllowInsecure bool
}

// Resolve searches the docker registry for repositories matching the passed in value
func (r DockerRegistryResolver) Resolve(value string) (*ComponentMatch, error) {
	ref, err := imageapi.ParseDockerImageReference(value)
	if err != nil {
		return nil, err
	}
	glog.V(4).Infof("checking Docker registry for %q", ref.String())
	connection, err := r.Client.Connect(ref.Registry, r.AllowInsecure)
	if err != nil {
		if dockerregistry.IsRegistryNotFound(err) {
			return nil, ErrNoMatch{value: value}
		}
		return nil, ErrNoMatch{value: value, qualifier: fmt.Sprintf("can't connect to %q: %v", ref.Registry, err)}
	}
	image, err := connection.ImageByTag(ref.Namespace, ref.Name, ref.Tag)
	if err != nil {
		if dockerregistry.IsNotFound(err) {
			return nil, ErrNoMatch{value: value, qualifier: err.Error()}
		}
		return nil, ErrNoMatch{value: value, qualifier: fmt.Sprintf("can't connect to %q: %v", ref.Registry, err)}
	}
	if len(ref.Tag) == 0 {
		ref.Tag = imageapi.DefaultImageTag
	}
	glog.V(4).Infof("found image: %#v", image)
	dockerImage := &imageapi.DockerImage{}
	if err = kapi.Scheme.Convert(image, dockerImage); err != nil {
		return nil, err
	}

	if len(ref.Registry) == 0 {
		ref.Registry = "Docker Hub"
	}
	return &ComponentMatch{
		Value:       value,
		Argument:    fmt.Sprintf("--docker-image=%q", value),
		Name:        value,
		Description: descriptionFor(dockerImage, value, ref.Registry),
		Builder:     IsBuilderImage(dockerImage),
		Score:       0,
		Image:       dockerImage,
		ImageTag:    ref.Tag,
	}, nil
}

func descriptionFor(image *imageapi.DockerImage, value, from string) string {
	shortID := image.ID
	if len(shortID) > 7 {
		shortID = shortID[:7]
	}
	parts := []string{fmt.Sprintf("Docker image %q", value), shortID, fmt.Sprintf("from %s", from)}
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

func partialScorer(a, b string, prefix bool, partial, none float32) (bool, float32) {
	switch {
	case len(a) == 0 && len(b) != 0, len(a) != 0 && len(b) == 0:
		return true, partial
	case a != b:
		if prefix {
			if strings.HasPrefix(a, b) || strings.HasPrefix(b, a) {
				return true, partial
			}
		}
		return false, none
	default:
		return true, 0.0
	}
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
		matches = append(matches, match)
	}
	return matches
}

// ImageStreamResolver searches the openshift server image streams for images matching a particular name
type ImageStreamResolver struct {
	Client            client.ImageStreamsNamespacer
	ImageStreamImages client.ImageStreamImagesNamespacer
	Namespaces        []string
}

// Resolve will attempt to find an imagestream with a name that matches the passed in value
func (r ImageStreamResolver) Resolve(value string) (*ComponentMatch, error) {
	ref, err := imageapi.ParseDockerImageReference(value)
	if err != nil || len(ref.Registry) != 0 {
		return nil, fmt.Errorf("image streams must be of the form [<namespace>/]<name>[:<tag>|@<digest>]")
	}
	namespaces := r.Namespaces
	if len(ref.Namespace) != 0 {
		namespaces = []string{ref.Namespace}
	}
	searchTag := ref.Tag
	if len(searchTag) == 0 {
		searchTag = imageapi.DefaultImageTag
	}
	matches := ScoredComponentMatches{}
	for _, namespace := range namespaces {
		glog.V(4).Infof("checking ImageStream %s/%s with ref %q", namespace, ref.Name, searchTag)
		stream, err := r.Client.ImageStreams(namespace).Get(ref.Name)
		if err != nil {
			if errors.IsNotFound(err) || errors.IsForbidden(err) {
				continue
			}
			return nil, err
		}
		ref.Namespace = namespace
		latest := imageapi.LatestTaggedImage(stream, searchTag)
		if latest == nil {
			// continue searching in the next namespace
			glog.V(2).Infof("no image recorded for %s/%s:%s", stream.Namespace, stream.Name, searchTag)
			matches = append(matches, &ComponentMatch{
				Value:       ref.String(),
				Argument:    fmt.Sprintf("--image-stream=%q", ref.String()),
				Name:        ref.Name,
				Description: fmt.Sprintf("Image stream %s in project %s, tracks %q", stream.Name, stream.Namespace, stream.Status.DockerImageRepository),
				Score:       0.5,

				ImageStream: stream,
				ImageTag:    searchTag,
			})
			continue
		}
		imageStreamImage, err := r.ImageStreamImages.ImageStreamImages(namespace).Get(ref.Name, latest.Image)
		if err != nil {
			if errors.IsNotFound(err) {
				// continue searching in the next namespace
				glog.V(2).Infof("tag %q is set, but image %q has been removed", searchTag, latest.Image)
				continue
			}
			return nil, err
		}
		imageData := imageStreamImage.Image

		ref.Registry = ""
		return &ComponentMatch{
			Value:       ref.String(),
			Argument:    fmt.Sprintf("--image-stream=%q", ref.String()),
			Name:        ref.Name,
			Description: fmt.Sprintf("Image stream %s (tag %q) in project %s, tracks %q", stream.Name, searchTag, stream.Namespace, stream.Status.DockerImageRepository),
			Builder:     IsBuilderImage(&imageData.DockerImageMetadata),
			Score:       0,

			ImageStream: stream,
			Image:       &imageData.DockerImageMetadata,
			ImageTag:    searchTag,
		}, nil
	}
	switch len(matches) {
	case 0:
		return nil, ErrNoMatch{value: value}
	case 1:
		return matches[0], nil
	default:
		return nil, ErrMultipleMatches{Image: value, Matches: matches}
	}
}

// Searcher will return potentially multiple matches for a set of search strings
type Searcher interface {
	Search(terms []string) ([]*ComponentMatch, error)
}

// InputImageFromMatch returns an image reference from a component match.
// The component match will either be an image stream or an image.
func InputImageFromMatch(match *ComponentMatch) (*ImageRef, error) {
	g := NewImageRefGenerator()

	switch {
	case match.ImageStream != nil:
		input, err := g.FromStream(match.ImageStream, match.ImageTag)
		if err != nil {
			return nil, err
		}
		input.AsImageStream = true
		input.Info = match.Image
		return input, nil

	case match.Image != nil:
		input, err := g.FromName(match.Value)
		if err != nil {
			return nil, err
		}
		input.AsImageStream = true
		input.Info = match.Image
		input.Insecure = match.Insecure
		return input, nil

	default:
		input, err := g.FromName(match.Value)
		if err != nil {
			return nil, err
		}
		return input, nil
	}
}

// ImageStreamByAnnotationResolver resolves image streams based on 'supports' annotations
// found in tagged images belonging to the stream
type ImageStreamByAnnotationResolver struct {
	Client            client.ImageStreamsNamespacer
	ImageStreamImages client.ImageStreamImagesNamespacer
	Namespaces        []string

	imageStreams map[string]*imageapi.ImageStreamList
}

const supportsAnnotationKey = "supports"

// NewImageStreamByAnnotationResolver creates a new ImageStreamByAnnotationResolver
func NewImageStreamByAnnotationResolver(streamClient client.ImageStreamsNamespacer, imageClient client.ImageStreamImagesNamespacer, namespaces []string) Resolver {
	return &ImageStreamByAnnotationResolver{
		Client:            streamClient,
		ImageStreamImages: imageClient,
		Namespaces:        namespaces,
		imageStreams:      make(map[string]*imageapi.ImageStreamList),
	}
}

func (r *ImageStreamByAnnotationResolver) getImageStreams(namespace string) ([]imageapi.ImageStream, error) {
	imageStreamList, ok := r.imageStreams[namespace]
	if !ok {
		var err error
		imageStreamList, err = r.Client.ImageStreams(namespace).List(labels.Everything(), fields.Everything())
		if err != nil {
			return nil, err
		}
		r.imageStreams[namespace] = imageStreamList
	}
	return imageStreamList.Items, nil
}

func matchSupportsAnnotation(value, annotation string) (float32, bool) {
	valueBase := strings.Split(value, ":")[0]
	parts := strings.Split(annotation, ",")

	// attempt an exact match first
	for _, p := range parts {
		if value == p {
			return 0.0, true
		}
	}

	// attempt a partial match
	for _, p := range parts {
		partBase := strings.Split(p, ":")[0]
		if valueBase == partBase {
			return 0.5, true
		}
	}

	return 0, false
}

func (r *ImageStreamByAnnotationResolver) annotationMatches(stream *imageapi.ImageStream, value string) []*ComponentMatch {
	if stream.Spec.Tags == nil {
		glog.Infof("No tags found on image, returning nil")
		return nil
	}
	matches := []*ComponentMatch{}
	for tag, tagref := range stream.Spec.Tags {
		if tagref.Annotations == nil {
			continue
		}
		supports, ok := tagref.Annotations[supportsAnnotationKey]
		if !ok {
			continue
		}
		score, ok := matchSupportsAnnotation(value, supports)
		if !ok {
			continue
		}
		latest := imageapi.LatestTaggedImage(stream, tag)
		if latest == nil {
			continue
		}
		imageStream, err := r.ImageStreamImages.ImageStreamImages(stream.Namespace).Get(stream.Name, latest.Image)
		if err != nil {
			glog.V(2).Infof("Could not retrieve image stream image for stream %q, tag %q: %v", stream.Name, tag, err)
			continue
		}
		if imageStream == nil {
			continue
		}
		imageData := imageStream.Image
		match := &ComponentMatch{
			Value:       value,
			Name:        stream.Name,
			Argument:    fmt.Sprintf("--image-stream=%q", value),
			Description: fmt.Sprintf("Image stream %s in project %s, tracks %q", stream.Name, stream.Namespace, stream.Status.DockerImageRepository),
			Builder:     IsBuilderImage(&imageData.DockerImageMetadata),
			Score:       score,

			ImageStream: stream,
			Image:       &imageData.DockerImageMetadata,
			ImageTag:    imageapi.DefaultImageTag,
		}
		matches = append(matches, match)
	}
	return matches
}

// Resolve finds image stream images using their 'supports' annotation
func (r *ImageStreamByAnnotationResolver) Resolve(value string) (*ComponentMatch, error) {
	matches := ScoredComponentMatches{}
	for _, namespace := range r.Namespaces {
		streams, err := r.getImageStreams(namespace)
		if err != nil {
			return nil, err
		}
		for i := range streams {
			matches = append(matches, r.annotationMatches(&streams[i], value)...)
		}
	}
	switch len(matches) {
	case 0:
		return nil, ErrNoMatch{value: value}
	case 1:
		return matches[0], nil
	default:
		exact := matches.Exact()
		if len(exact) == 1 {
			return exact[0], nil
		}
		sort.Sort(matches)
		return nil, ErrMultipleMatches{Image: value, Matches: matches}
	}
}

// PassThroughDockerResolver returns a match with the value that was passed in
type PassThroughDockerResolver struct{}

// Resolve always returns a match
func (r *PassThroughDockerResolver) Resolve(value string) (*ComponentMatch, error) {
	return &ComponentMatch{
		Value:       value,
		Name:        value,
		Argument:    fmt.Sprintf("--docker-image=%q", value),
		Description: fmt.Sprintf("Docker image %q", value),
		Score:       1.0,
	}, nil
}

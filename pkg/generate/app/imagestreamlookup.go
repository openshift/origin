package app

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// ImageStreamSearcher searches the openshift server image streams for images matching a particular name
type ImageStreamSearcher struct {
	Client            client.ImageStreamsNamespacer
	ImageStreamImages client.ImageStreamImagesNamespacer
	Namespaces        []string
}

// Search will attempt to find imagestreams with names that matches the passed in value
func (r ImageStreamSearcher) Search(terms ...string) (ComponentMatches, error) {
	componentMatches := ComponentMatches{}
	for _, term := range terms {
		ref, err := imageapi.ParseDockerImageReference(term)
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
		for _, namespace := range namespaces {
			glog.V(4).Infof("checking ImageStreams %s/%s with ref %q", namespace, ref.Name, searchTag)
			streams, err := r.Client.ImageStreams(namespace).List(labels.Everything(), fields.Everything())
			if err != nil {
				if errors.IsNotFound(err) || errors.IsForbidden(err) {
					continue
				}
				return nil, err
			}
			ref.Namespace = namespace
			for i := 0; i < len(streams.Items); i++ {
				stream := streams.Items[i]
				score, scored := imageStreamScorer(stream, ref.Name)
				if scored {
					imageref, _ := imageapi.ParseDockerImageReference(term)
					imageref.Name = stream.Name

					latest := imageapi.LatestTaggedImage(&stream, searchTag)
					if latest == nil {
						glog.V(2).Infof("no image recorded for %s/%s:%s", stream.Namespace, stream.Name, searchTag)
						componentMatches = append(componentMatches, &ComponentMatch{
							Value:       imageref.String(),
							Argument:    fmt.Sprintf("--image-stream=%q", imageref.String()),
							Name:        imageref.Name,
							Description: fmt.Sprintf("Image stream %s in project %s, tracks %q", stream.Name, stream.Namespace, stream.Status.DockerImageRepository),
							Score:       0.5 + score,
							ImageStream: &stream,
							ImageTag:    searchTag,
						})
						continue
					}

					imageStreamImage, err := r.ImageStreamImages.ImageStreamImages(namespace).Get(stream.Name, latest.Image)
					if err != nil {
						if errors.IsNotFound(err) {
							// continue searching
							glog.V(2).Infof("tag %q is set, but image %q has been removed", searchTag, latest.Image)
							continue
						}
						return nil, err
					}
					imageData := imageStreamImage.Image

					imageref.Registry = ""
					componentMatches = append(componentMatches, &ComponentMatch{
						Value:       imageref.String(),
						Argument:    fmt.Sprintf("--image-stream=%q", imageref.String()),
						Name:        imageref.Name,
						Description: fmt.Sprintf("Image stream %q (tag %q) in project %q, tracks %q", stream.Name, searchTag, stream.Namespace, stream.Status.DockerImageRepository),
						Builder:     IsBuilderImage(&imageData.DockerImageMetadata),
						Score:       score,
						ImageStream: &stream,
						Image:       &imageData.DockerImageMetadata,
						ImageTag:    searchTag,
					})
				}
			}
		}
	}
	return componentMatches, nil
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

// ImageStreamByAnnotationSearcher searches for image streams based on 'supports' annotations
// found in tagged images belonging to the stream
type ImageStreamByAnnotationSearcher struct {
	Client            client.ImageStreamsNamespacer
	ImageStreamImages client.ImageStreamImagesNamespacer
	Namespaces        []string

	imageStreams map[string]*imageapi.ImageStreamList
}

const supportsAnnotationKey = "supports"

// NewImageStreamByAnnotationSearcher creates a new ImageStreamByAnnotationSearcher
func NewImageStreamByAnnotationSearcher(streamClient client.ImageStreamsNamespacer, imageClient client.ImageStreamImagesNamespacer, namespaces []string) Searcher {
	return &ImageStreamByAnnotationSearcher{
		Client:            streamClient,
		ImageStreamImages: imageClient,
		Namespaces:        namespaces,
		imageStreams:      make(map[string]*imageapi.ImageStreamList),
	}
}

func (r *ImageStreamByAnnotationSearcher) getImageStreams(namespace string) ([]imageapi.ImageStream, error) {
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

func (r *ImageStreamByAnnotationSearcher) annotationMatches(stream *imageapi.ImageStream, value string) []*ComponentMatch {
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

// Search finds image stream images using their 'supports' annotation
func (r *ImageStreamByAnnotationSearcher) Search(terms ...string) (ComponentMatches, error) {
	matches := ComponentMatches{}
	for _, namespace := range r.Namespaces {
		streams, err := r.getImageStreams(namespace)
		if err != nil {
			return nil, err
		}
		for i := range streams {
			for _, term := range terms {
				matches = append(matches, r.annotationMatches(&streams[i], term)...)
			}
		}
	}
	return matches, nil
}

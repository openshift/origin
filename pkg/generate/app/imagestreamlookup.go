package app

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"

	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// ImageStreamSearcher searches the openshift server image streams for images matching a particular name
type ImageStreamSearcher struct {
	Client            client.ImageStreamsNamespacer
	ImageStreamImages client.ImageStreamImagesNamespacer
	Namespaces        []string
	AllowMissingTags  bool
}

// Search will attempt to find imagestreams with names that match the passed in value
func (r ImageStreamSearcher) Search(precise bool, terms ...string) (ComponentMatches, []error) {
	componentMatches := ComponentMatches{}
	var errs []error
	for _, term := range terms {
		var (
			ref imageapi.DockerImageReference
			err error
		)
		switch term {
		case "__imagestream_fail":
			errs = append(errs, fmt.Errorf("unable to find the specified image: %s", term))
			continue
		case "*":
			ref = imageapi.DockerImageReference{Name: term}
		default:
			ref, err = imageapi.ParseDockerImageReference(term)
			if err != nil || len(ref.Registry) != 0 {
				glog.V(2).Infof("image streams must be of the form [<namespace>/]<name>[:<tag>|@<digest>], term %q did not qualify", term)
				continue
			}
		}

		namespaces := r.Namespaces
		if len(ref.Namespace) != 0 {
			namespaces = []string{ref.Namespace}
		}
		followTag := false
		searchTag := ref.Tag
		if len(searchTag) == 0 {
			searchTag = imageapi.DefaultImageTag
			followTag = true
		}
		for _, namespace := range namespaces {
			glog.V(4).Infof("checking ImageStreams %s/%s with ref %q", namespace, ref.Name, searchTag)
			exact := false
			streams, err := r.Client.ImageStreams(namespace).List(kapi.ListOptions{})
			if err != nil {
				if errors.IsNotFound(err) || errors.IsForbidden(err) {
					continue
				}
				errs = append(errs, err)
				continue
			}
			original := ref
			ref.Namespace = namespace
			for i := range streams.Items {
				stream := &streams.Items[i]
				score, scored := imageStreamScorer(*stream, ref.Name)
				if !scored {
					glog.V(2).Infof("unscored %s: %v", stream.Name, score)
					continue
				}

				// indicate the server knows how to directly import image stream tags
				var meta map[string]string
				if stream.Generation > 0 {
					meta = map[string]string{"direct-tag": "1"}
				}

				imageref := original
				imageref.Name = stream.Name
				imageref.Registry = ""
				matchName := fmt.Sprintf("%s/%s", stream.Namespace, stream.Name)

				addMatch := func(tag string, matchScore float32, image *imageapi.DockerImage, notFound bool) {
					name := matchName
					var description, argument string
					if len(tag) > 0 {
						name = fmt.Sprintf("%s:%s", name, tag)
						argument = fmt.Sprintf("--image-stream=%q", name)
						description = fmt.Sprintf("Image stream %q (tag %q) in project %q", stream.Name, tag, stream.Namespace)
					} else {
						argument = fmt.Sprintf("--image-stream=%q --allow-missing-imagestream-tags", name)
						description = fmt.Sprintf("Image stream %q in project %q", stream.Name, stream.Namespace)
					}

					match := &ComponentMatch{
						Value:       term,
						Argument:    argument,
						Name:        name,
						Description: description,
						Score:       matchScore,
						ImageStream: stream,
						Image:       image,
						ImageTag:    tag,
						Meta:        meta,
						NoTagsFound: notFound,
					}
					glog.V(2).Infof("Adding %s as component match for %q with score %v", match.Description, term, matchScore)
					componentMatches = append(componentMatches, match)
				}

				// When an image stream contains a tag that references another local tag, and the user has not
				// provided a tag themselves (i.e. they asked for mysql and we defaulted to mysql:latest), walk
				// the chain of references to the end. This ensures that applications can default to using a "stable"
				// branch by giving the control over version to the image stream author.
				finalTag := searchTag
				if specTag, ok := stream.Spec.Tags[searchTag]; ok && followTag {
					if specTag.From != nil && specTag.From.Kind == "ImageStreamTag" && !strings.Contains(specTag.From.Name, ":") {
						if imageapi.LatestTaggedImage(stream, specTag.From.Name) != nil {
							finalTag = specTag.From.Name
						}
					}
				}

				latest := imageapi.LatestTaggedImage(stream, finalTag)
				if latest == nil || len(latest.Image) == 0 {

					glog.V(2).Infof("no image recorded for %s/%s:%s", stream.Namespace, stream.Name, finalTag)
					if r.AllowMissingTags {
						addMatch(finalTag, score, nil, false)
						continue
					}
					// Find tags that do exist and return those as partial matches
					foundOtherTags := false
					for tag := range stream.Status.Tags {
						latest := imageapi.LatestTaggedImage(stream, tag)
						if latest == nil || len(latest.Image) == 0 {
							continue
						}
						foundOtherTags = true

						// If the user specified a tag in their search string (followTag == false),
						// then score this match lower.
						tagScore := score
						if !followTag {
							tagScore += 0.5
						}
						addMatch(tag, tagScore, nil, false)
					}
					if !foundOtherTags {
						addMatch("", 0.5+score, nil, true)
					}
					continue
				}

				imageStreamImage, err := r.ImageStreamImages.ImageStreamImages(namespace).Get(stream.Name, latest.Image)
				if err != nil {
					if errors.IsNotFound(err) {
						// continue searching
						glog.V(2).Infof("tag %q is set, but image %q has been removed", finalTag, latest.Image)
						continue
					}
					errs = append(errs, err)
					continue
				}

				addMatch(finalTag, score, &imageStreamImage.Image.DockerImageMetadata, false)
				if score == 0.0 {
					exact = true
				}
			}

			// If we found one or more exact matches in this namespace, do not continue looking at
			// other namespaces
			if exact && precise {
				break
			}
		}
	}
	return componentMatches, errs
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
		if match.Meta["direct-tag"] == "1" {
			input.TagDirectly = true
		}
		input.AsImageStream = true
		input.Info = match.Image
		return input, nil

	case match.Image != nil:
		input, err := g.FromName(match.Value)
		if err != nil {
			return nil, err
		}
		if match.Meta["direct-tag"] == "1" {
			input.TagDirectly = true
			input.AsResolvedImage = true
		}
		input.AsImageStream = !match.LocalOnly
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
		imageStreamList, err = r.Client.ImageStreams(namespace).List(kapi.ListOptions{})
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

		// indicate the server knows how to directly tag images
		var meta map[string]string
		if imageStream.Generation > 0 {
			meta = map[string]string{"direct-tag": "1"}
		}

		imageData := imageStream.Image
		matchName := fmt.Sprintf("%s/%s", stream.Namespace, stream.Name)
		description := fmt.Sprintf("Image stream %q in project %q", stream.Name, stream.Namespace)
		if len(tag) > 0 {
			matchName = fmt.Sprintf("%s:%s", matchName, tag)
			description = fmt.Sprintf("Image stream %q (tag %q) in project %q", stream.Name, tag, stream.Namespace)
		}
		glog.V(5).Infof("ImageStreamAnnotationSearcher match found: %s for %s with score %f", matchName, value, score)
		match := &ComponentMatch{
			Value:       value,
			Name:        fmt.Sprintf("%s", matchName),
			Argument:    fmt.Sprintf("--image-stream=%q", matchName),
			Description: description,
			Score:       score,

			ImageStream: stream,
			Image:       &imageData.DockerImageMetadata,
			ImageTag:    tag,
			Meta:        meta,
		}
		matches = append(matches, match)
	}
	return matches
}

// Search finds image stream images using their 'supports' annotation
func (r *ImageStreamByAnnotationSearcher) Search(precise bool, terms ...string) (ComponentMatches, []error) {
	matches := ComponentMatches{}
	var errs []error
	for _, namespace := range r.Namespaces {
		streams, err := r.getImageStreams(namespace)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for i := range streams {
			for _, term := range terms {
				if term == "__imagestreamannotation_fail" {
					errs = append(errs, fmt.Errorf("unable to find the specified image: %s", term))
					continue
				}
				glog.V(5).Infof("Checking imagestream %s/%s for supports annotation %q", namespace, streams[i].Name, term)
				matches = append(matches, r.annotationMatches(&streams[i], term)...)
			}
		}
		if precise {
			for _, m := range matches {
				if m.Score == 0.0 {
					return matches, errs
				}
			}
		}
	}
	return matches, errs
}

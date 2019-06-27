package app

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	docker "github.com/fsouza/go-dockerclient"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	dockerv10 "github.com/openshift/api/image/docker10"
	imagev1 "github.com/openshift/api/image/v1"
	imagev1client "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	dockerregistry "github.com/openshift/library-go/pkg/image/dockerv1client"
	"github.com/openshift/library-go/pkg/image/imageutil"
	"github.com/openshift/library-go/pkg/image/reference"
	imagehelpers "github.com/openshift/oc/pkg/helpers/image"
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

func (r DockerClientSearcher) Type() string {
	return "local docker images"
}

// Search searches all images in local docker server for images that match terms
func (r DockerClientSearcher) Search(precise bool, terms ...string) (ComponentMatches, []error) {
	componentMatches := ComponentMatches{}
	errs := []error{}
	for _, term := range terms {
		var (
			ref reference.DockerImageReference
			err error
		)
		switch term {
		case "__dockerimage_fail":
			errs = append(errs, fmt.Errorf("unable to find the specified docker image: %s", term))
			continue
		case "scratch":
			componentMatches = append(componentMatches, &ComponentMatch{
				Value: term,
				Score: 0.0,
				// we don't want to create an imagestream for "scratch", so treat
				// it as a local only image.
				LocalOnly: true,
				Virtual:   true,
			})
			return componentMatches, errs
		case "*":
			ref = reference.DockerImageReference{Name: term}
		default:
			ref, err = reference.Parse(term)
			if err != nil {
				continue
			}
		}

		termMatches := ScoredComponentMatches{}

		// first look for the image in the remote container image registry
		if r.RegistrySearcher != nil {
			klog.V(4).Infof("checking remote registry for %q", ref.String())
			matches, err := r.RegistrySearcher.Search(precise, term)
			errs = append(errs, err...)

			for i := range matches {
				matches[i].LocalOnly = false
				klog.V(5).Infof("Found remote match %v", matches[i].Value)
			}
			termMatches = append(termMatches, matches...)
		}

		if r.Client == nil || reflect.ValueOf(r.Client).IsNil() {
			componentMatches = append(componentMatches, termMatches...)
			continue
		}

		// if we didn't find it exactly in a remote registry,
		// try to find it as a local-only image.
		if len(termMatches.Exact()) == 0 {
			klog.V(4).Infof("checking local Docker daemon for %q", ref.String())
			images, err := r.Client.ListImages(docker.ListImagesOptions{})
			if err != nil {
				errs = append(errs, err)
				continue
			}

			if len(ref.Tag) == 0 {
				ref.Tag = imagev1.DefaultImageTag
				term = fmt.Sprintf("%s:%s", term, imagev1.DefaultImageTag)
			}
			for _, image := range images {
				if tags := matchTag(image, term, ref.Registry, ref.Namespace, ref.Name, ref.Tag); len(tags) > 0 {
					for i := range tags {
						tags[i].LocalOnly = true
						klog.V(5).Infof("Found local docker image match %q with score %f", tags[i].Value, tags[i].Score)
					}
					termMatches = append(termMatches, tags...)
				}
			}
		}
		sort.Sort(termMatches)

		for i, match := range termMatches {
			if match.DockerImage != nil {
				continue
			}

			image, err := r.Client.InspectImage(match.Value)
			if err != nil {
				if err != docker.ErrNoSuchImage {
					errs = append(errs, err)
				}
				continue
			}
			dockerImage := &dockerv10.DockerImage{}
			if err := dockerregistry.ImageScheme.Convert(image, dockerImage, nil); err != nil {
				errs = append(errs, err)
				continue
			}
			updated := &ComponentMatch{
				Value:       match.Value,
				Argument:    fmt.Sprintf("--docker-image=%q", match.Value),
				Name:        match.Value,
				Description: descriptionFor(dockerImage, match.Value, ref.Registry, ""),
				Score:       match.Score,
				DockerImage: dockerImage,
				ImageTag:    ref.Tag,
				Insecure:    r.Insecure,
				Meta:        map[string]string{"registry": ref.Registry},
				LocalOnly:   match.LocalOnly,
			}
			termMatches[i] = updated
		}

		componentMatches = append(componentMatches, termMatches...)
	}

	return componentMatches, errs
}

// MissingImageSearcher always returns an exact match for the item being searched for.
// It should be used with very high weight(weak priority) as a result of last resort when the
// user has indicated they want to allow missing images(not found in the container image registry
// or locally) to be used anyway.
type MissingImageSearcher struct {
}

func (r MissingImageSearcher) Type() string {
	return "images not found in container image registry nor locally"
}

// Search always returns an exact match for the search terms.
func (r MissingImageSearcher) Search(precise bool, terms ...string) (ComponentMatches, []error) {
	componentMatches := ComponentMatches{}
	for _, term := range terms {
		componentMatches = append(componentMatches, &ComponentMatch{
			Value:     term,
			Score:     0.0,
			LocalOnly: true,
		})
		klog.V(4).Infof("Added missing image match for %v", term)
	}
	return componentMatches, nil
}

type ImageImportSearcher struct {
	Client        imagev1client.ImageStreamImportInterface
	AllowInsecure bool
	Fallback      Searcher
}

func (s ImageImportSearcher) Type() string {
	return "images via the image stream import API"
}

// Search invokes the new ImageStreamImport API to have the server look up container images for the user,
// using secrets stored on the server.
func (s ImageImportSearcher) Search(precise bool, terms ...string) (ComponentMatches, []error) {
	var errs []error
	isi := &imagev1.ImageStreamImport{}
	for _, term := range terms {
		if term == "__imageimport_fail" {
			errs = append(errs, fmt.Errorf("unable to find the specified docker import: %s", term))
			continue
		}
		isi.Spec.Images = append(isi.Spec.Images, imagev1.ImageImportSpec{
			From:         corev1.ObjectReference{Kind: "DockerImage", Name: term},
			ImportPolicy: imagev1.TagImportPolicy{Insecure: s.AllowInsecure},
		})
	}
	isi.Name = "newapp"
	result, err := s.Client.Create(isi)
	if err != nil {
		if err == imagehelpers.ErrImageStreamImportUnsupported && s.Fallback != nil {
			return s.Fallback.Search(precise, terms...)
		}
		return nil, []error{fmt.Errorf("can't lookup images: %v", err)}
	}

	componentMatches := ComponentMatches{}
	for i, image := range result.Status.Images {
		term := result.Spec.Images[i].From.Name
		if image.Status.Status != metav1.StatusSuccess {
			klog.V(4).Infof("image import failed: %#v", image)
			switch image.Status.Reason {
			case metav1.StatusReasonInternalError:
				// try to find the cause of the internal error
				if image.Status.Details != nil && len(image.Status.Details.Causes) > 0 {
					for _, c := range image.Status.Details.Causes {
						klog.Warningf("container image registry lookup failed: %s", c.Message)
					}
				} else {
					klog.Warningf("container image registry lookup failed: %s", image.Status.Message)
				}
			case metav1.StatusReasonInvalid, metav1.StatusReasonUnauthorized, metav1.StatusReasonNotFound:
			default:
				errs = append(errs, fmt.Errorf("can't look up container image %q: %s", term, image.Status.Message))
			}
			continue
		}
		ref, err := reference.Parse(term)
		if err != nil {
			klog.V(4).Infof("image import failed, can't parse ref %q: %v", term, err)
			continue
		}
		if len(ref.Tag) == 0 {
			ref.Tag = imagev1.DefaultImageTag
		}
		if len(ref.Registry) == 0 {
			ref.Registry = "Docker Hub"
		}

		if err := imageutil.ImageWithMetadata(image.Image); err != nil {
			errs = append(errs, err)
			continue
		}

		dockerImage, ok := image.Image.DockerImageMetadata.Object.(*dockerv10.DockerImage)
		if !ok {
			continue
		}

		match := &ComponentMatch{
			Value:       term,
			Argument:    fmt.Sprintf("--docker-image=%q", term),
			Name:        term,
			Description: descriptionFor(dockerImage, term, ref.Registry, ref.Tag),
			Score:       0,
			DockerImage: dockerImage,
			ImageTag:    ref.Tag,
			Insecure:    s.AllowInsecure,
			Meta:        map[string]string{"registry": ref.Registry, "direct-tag": "1"},
		}
		klog.V(2).Infof("Adding %s as component match for %q with score %v", match.Description, term, match.Score)
		componentMatches = append(componentMatches, match)
	}
	return componentMatches, errs
}

// DockerRegistrySearcher searches for images in a given container image registry.
// Notice that it only matches exact searches - so a search for "rub" will
// not return images with the name "ruby".
// TODO: replace ImageByTag to allow partial matches
type DockerRegistrySearcher struct {
	Client        dockerregistry.Client
	AllowInsecure bool
}

func (r DockerRegistrySearcher) Type() string {
	return "images in the container image registry"
}

// Search searches in the container image registry for images that match terms
func (r DockerRegistrySearcher) Search(precise bool, terms ...string) (ComponentMatches, []error) {
	componentMatches := ComponentMatches{}
	var errs []error
	for _, term := range terms {
		var (
			ref reference.DockerImageReference
			err error
		)
		if term != "*" {
			ref, err = reference.Parse(term)
			if err != nil {
				continue
			}
		} else {
			ref = reference.DockerImageReference{Name: term}
		}

		klog.V(4).Infof("checking container image registry for %q, allow-insecure=%v", ref.String(), r.AllowInsecure)
		connection, err := r.Client.Connect(ref.Registry, r.AllowInsecure)
		if err != nil {
			if dockerregistry.IsRegistryNotFound(err) {
				errs = append(errs, err)
				continue
			}
			errs = append(errs, fmt.Errorf("can't connect to %q: %v", ref.Registry, err))
			continue
		}

		image, err := connection.ImageByTag(ref.Namespace, ref.Name, ref.Tag)
		if err != nil {
			if dockerregistry.IsNotFound(err) {
				if dockerregistry.IsTagNotFound(err) {
					klog.V(4).Infof("tag not found: %v", err)
				}
				continue
			}
			errs = append(errs, fmt.Errorf("can't connect to %q: %v", ref.Registry, err))
			continue
		}

		if len(ref.Tag) == 0 {
			ref.Tag = imagev1.DefaultImageTag
		}
		if len(ref.Registry) == 0 {
			ref.Registry = "Docker Hub"
		}
		klog.V(4).Infof("found image: %#v", image)

		dockerImage := &dockerv10.DockerImage{}
		if err = dockerregistry.ImageScheme.Convert(&image.Image, dockerImage, nil); err != nil {
			errs = append(errs, err)
			continue
		}

		match := &ComponentMatch{
			Value:       term,
			Argument:    fmt.Sprintf("--docker-image=%q", term),
			Name:        term,
			Description: descriptionFor(dockerImage, term, ref.Registry, ref.Tag),
			Score:       0,
			DockerImage: dockerImage,
			ImageTag:    ref.Tag,
			Insecure:    r.AllowInsecure,
			Meta:        map[string]string{"registry": ref.Registry},
		}
		klog.V(2).Infof("Adding %s as component match for %q with score %v", match.Description, term, match.Score)
		componentMatches = append(componentMatches, match)
	}

	return componentMatches, errs
}

func descriptionFor(image *dockerv10.DockerImage, value, from string, tag string) string {
	if len(from) == 0 {
		from = "local"
	}
	shortID := imagehelpers.ShortDockerImageID(image, 7)
	tagPart := ""
	if len(tag) > 0 {
		tagPart = fmt.Sprintf(" (tag %q)", tag)
	}
	parts := []string{fmt.Sprintf("container image %q%v", value, tagPart), shortID, fmt.Sprintf("from %s", from)}
	if image.Size > 0 {
		mb := float64(image.Size) / float64(1024*1024)
		parts = append(parts, fmt.Sprintf("%.3fmb", mb))
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
	matches := []*ComponentMatch{}
	for _, s := range image.RepoTags {
		if value == s {
			klog.V(4).Infof("exact match on %q", s)
			matches = append(matches, &ComponentMatch{
				Value: s,
				Score: 0.0,
			})
			continue
		}
		iRef, err := reference.Parse(s)
		if err != nil {
			continue
		}
		if len(iRef.Tag) == 0 {
			iRef.Tag = imagev1.DefaultImageTag
		}
		match := &ComponentMatch{}
		ok, score := partialScorer(name, iRef.Name, true, 0.5, 1.0)
		// If the name doesn't match, don't consider this image as a match
		if !ok {
			continue
		}

		// Add up the score, then get the average
		match.Score += score
		_, score = partialScorer(namespace, iRef.Namespace, false, 0.5, 1.0)
		match.Score += score
		_, score = partialScorer(registry, iRef.Registry, false, 0.5, 1.0)
		match.Score += score
		_, score = partialScorer(tag, iRef.Tag, true, 0.5, 1.0)
		match.Score += score

		if match.Score >= 4.0 {
			continue
		}
		match.Score = match.Score / 4.0
		klog.V(4).Infof("partial match on %q with %f", s, match.Score)
		match.Value = s
		match.Meta = map[string]string{"registry": registry}
		matches = append(matches, match)
	}
	return matches
}

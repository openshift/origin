package imagestreammapping

import (
	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/errors"
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/wait"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapihelper "k8s.io/kubernetes/pkg/api/helper"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/registry/image"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
)

// maxRetriesOnConflict is the maximum retry count for Create calls which
// result in resource conflicts.
const maxRetriesOnConflict = 10

// REST implements the RESTStorage interface in terms of an image registry and
// image stream registry. It only supports the Create method and is used
// to simplify adding a new Image and tag to an ImageStream.
type REST struct {
	imageRegistry       image.Registry
	imageStreamRegistry imagestream.Registry
	strategy            Strategy
}

// NewREST returns a new REST.
func NewREST(imageRegistry image.Registry, imageStreamRegistry imagestream.Registry, defaultRegistry imageapi.DefaultRegistry) *REST {
	return &REST{
		imageRegistry:       imageRegistry,
		imageStreamRegistry: imageStreamRegistry,
		strategy:            NewStrategy(defaultRegistry),
	}
}

// New returns a new ImageStreamMapping for use with Create.
func (r *REST) New() runtime.Object {
	return &imageapi.ImageStreamMapping{}
}

// Create registers a new image (if it doesn't exist) and updates the
// specified ImageStream's tags. If attempts to update the ImageStream fail
// with a resource conflict, the update will be retried if the newer
// ImageStream has no tag diffs from the previous state. If tag diffs are
// detected, the conflict error is returned.
func (s *REST) Create(ctx apirequest.Context, obj runtime.Object, _ bool) (runtime.Object, error) {
	if err := rest.BeforeCreate(s.strategy, ctx, obj); err != nil {
		return nil, err
	}

	mapping := obj.(*imageapi.ImageStreamMapping)

	stream, err := s.findStreamForMapping(ctx, mapping)
	if err != nil {
		return nil, err
	}

	image := mapping.Image
	tag := mapping.Tag
	if len(tag) == 0 {
		tag = imageapi.DefaultImageTag
	}

	imageCreateErr := s.imageRegistry.CreateImage(ctx, &image)
	if imageCreateErr != nil && !errors.IsAlreadyExists(imageCreateErr) {
		return nil, imageCreateErr
	}

	// prefer dockerImageReference set on image for the tagEvent if the image is new
	ref := image.DockerImageReference
	if errors.IsAlreadyExists(imageCreateErr) && image.Annotations[imageapi.ManagedByOpenShiftAnnotation] == "true" {
		// the image is managed by us and, most probably, tagged in some other image stream
		// let's make the reference local to this stream
		if streamRef, err := imageapi.DockerImageReferenceForStream(stream); err == nil {
			streamRef.ID = image.Name
			ref = streamRef.Exact()
		} else {
			glog.V(4).Infof("Failed to get dockerImageReference for stream %s/%s: %v", stream.Namespace, stream.Name, err)
		}
	}

	next := imageapi.TagEvent{
		Created:              metav1.Now(),
		DockerImageReference: ref,
		Image:                image.Name,
	}

	err = wait.ExponentialBackoff(wait.Backoff{Steps: maxRetriesOnConflict}, func() (bool, error) {
		lastEvent := imageapi.LatestTaggedImage(stream, tag)

		next.Generation = stream.Generation

		if !imageapi.AddTagEventToImageStream(stream, tag, next) {
			// nothing actually changed
			return true, nil
		}
		imageapi.UpdateTrackingTags(stream, tag, next)
		_, err := s.imageStreamRegistry.UpdateImageStreamStatus(ctx, stream)
		if err == nil {
			return true, nil
		}
		if !errors.IsConflict(err) {
			return false, err
		}
		// If the update conflicts, get the latest stream and check for tag
		// updates. If the latest tag hasn't changed, retry.
		latestStream, findLatestErr := s.findStreamForMapping(ctx, mapping)
		if findLatestErr != nil {
			return false, findLatestErr
		}

		// no previous tag
		if lastEvent == nil {
			// The tag hasn't changed, so try again with the updated stream.
			stream = latestStream
			return false, nil
		}

		// check for tag change
		newerEvent := imageapi.LatestTaggedImage(latestStream, tag)
		// generation and creation time differences are ignored
		lastEvent.Generation = newerEvent.Generation
		lastEvent.Created = newerEvent.Created
		if kapihelper.Semantic.DeepEqual(lastEvent, newerEvent) {
			// The tag hasn't changed, so try again with the updated stream.
			stream = latestStream
			return false, nil
		}

		// The tag changed, so return the conflict error back to the client.
		return false, err
	})
	if err != nil {
		return nil, err
	}
	return &metav1.Status{Status: metav1.StatusSuccess}, nil
}

// findStreamForMapping retrieves an ImageStream whose DockerImageRepository matches dockerRepo.
func (s *REST) findStreamForMapping(ctx apirequest.Context, mapping *imageapi.ImageStreamMapping) (*imageapi.ImageStream, error) {
	if len(mapping.Name) > 0 {
		return s.imageStreamRegistry.GetImageStream(ctx, mapping.Name, &metav1.GetOptions{})
	}
	if len(mapping.DockerImageRepository) != 0 {
		list, err := s.imageStreamRegistry.ListImageStreams(ctx, &metainternal.ListOptions{})
		if err != nil {
			return nil, err
		}
		for i := range list.Items {
			if mapping.DockerImageRepository == list.Items[i].Spec.DockerImageRepository {
				return &list.Items[i], nil
			}
		}
		return nil, errors.NewInvalid(imageapi.Kind("ImageStreamMapping"), "", field.ErrorList{
			field.NotFound(field.NewPath("dockerImageStream"), mapping.DockerImageRepository),
		})
	}
	return nil, errors.NewNotFound(imageapi.Resource("imagestream"), "")
}

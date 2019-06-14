package imagestreammapping

import (
	"context"
	"fmt"

	"k8s.io/klog"

	"k8s.io/apimachinery/pkg/api/errors"
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/registry/rest"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"

	imagegroup "github.com/openshift/api/image"
	imagev1 "github.com/openshift/api/image/v1"
	imagereference "github.com/openshift/library-go/pkg/image/reference"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/apiserver/internalimageutil"
	"github.com/openshift/origin/pkg/image/apiserver/registry/image"
	"github.com/openshift/origin/pkg/image/apiserver/registry/imagestream"
	"github.com/openshift/origin/pkg/image/apiserver/registryhostname"
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

var _ rest.Creater = &REST{}
var _ rest.Scoper = &REST{}

// NewREST returns a new REST.
func NewREST(imageRegistry image.Registry, imageStreamRegistry imagestream.Registry, registry registryhostname.RegistryHostnameRetriever) *REST {
	return &REST{
		imageRegistry:       imageRegistry,
		imageStreamRegistry: imageStreamRegistry,
		strategy:            NewStrategy(registry),
	}
}

// New returns a new ImageStreamMapping for use with Create.
func (r *REST) New() runtime.Object {
	return &imageapi.ImageStreamMapping{}
}

func (s *REST) NamespaceScoped() bool {
	return true
}

// Create registers a new image (if it doesn't exist) and updates the
// specified ImageStream's tags. If attempts to update the ImageStream fail
// with a resource conflict, the update will be retried if the newer
// ImageStream has no tag diffs from the previous state. If tag diffs are
// detected, the conflict error is returned.
func (s *REST) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	if err := rest.BeforeCreate(s.strategy, ctx, obj); err != nil {
		return nil, err
	}
	if err := createValidation(obj.DeepCopyObject()); err != nil {
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
		tag = imagev1.DefaultImageTag
	}

	imageCreateErr := s.imageRegistry.CreateImage(ctx, &image)
	if imageCreateErr != nil && !errors.IsAlreadyExists(imageCreateErr) {
		return nil, imageCreateErr
	}

	// prefer dockerImageReference set on image for the tagEvent if the image is new
	ref := image.DockerImageReference
	if errors.IsAlreadyExists(imageCreateErr) && image.Annotations[imagev1.ManagedByOpenShiftAnnotation] == "true" {
		// the image is managed by us and, most probably, tagged in some other image stream
		// let's make the reference local to this stream
		if streamRef, err := dockerImageReferenceForStream(stream); err == nil {
			streamRef.ID = image.Name
			ref = streamRef.Exact()
		} else {
			klog.V(4).Infof("Failed to get dockerImageReference for stream %s/%s: %v", stream.Namespace, stream.Name, err)
		}
	}

	next := imageapi.TagEvent{
		Created:              metav1.Now(),
		DockerImageReference: ref,
		Image:                image.Name,
	}

	err = wait.ExponentialBackoff(wait.Backoff{Steps: maxRetriesOnConflict}, func() (bool, error) {
		lastEvent := internalimageutil.LatestTaggedImage(stream, tag)

		next.Generation = stream.Generation

		if !internalimageutil.AddTagEventToImageStream(stream, tag, next) {
			// nothing actually changed
			return true, nil
		}
		internalimageutil.UpdateTrackingTags(stream, tag, next)
		_, err := s.imageStreamRegistry.UpdateImageStreamStatus(ctx, stream, false, &metav1.UpdateOptions{})
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
		newerEvent := internalimageutil.LatestTaggedImage(latestStream, tag)
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

// DockerImageReferenceForStream returns a DockerImageReference that represents
// the ImageStream or false, if no valid reference exists.
func dockerImageReferenceForStream(stream *imageapi.ImageStream) (imageapi.DockerImageReference, error) {
	spec := stream.Status.DockerImageRepository
	if len(spec) == 0 {
		spec = stream.Spec.DockerImageRepository
	}
	if len(spec) == 0 {
		return imageapi.DockerImageReference{}, fmt.Errorf("no possible pull spec for %s/%s", stream.Namespace, stream.Name)
	}
	return imagereference.Parse(spec)
}

// findStreamForMapping retrieves an ImageStream whose DockerImageRepository matches dockerRepo.
func (s *REST) findStreamForMapping(ctx context.Context, mapping *imageapi.ImageStreamMapping) (*imageapi.ImageStream, error) {
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
		return nil, errors.NewInvalid(imagegroup.Kind("ImageStreamMapping"), "", field.ErrorList{
			field.NotFound(field.NewPath("dockerImageStream"), mapping.DockerImageRepository),
		})
	}
	return nil, errors.NewNotFound(imagegroup.Resource("imagestream"), "")
}

package imagestreammapping

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/fielderrors"
	"k8s.io/kubernetes/pkg/util/wait"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/api/validation"
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
}

// NewREST returns a new REST.
func NewREST(imageRegistry image.Registry, imageStreamRegistry imagestream.Registry) *REST {
	return &REST{
		imageRegistry:       imageRegistry,
		imageStreamRegistry: imageStreamRegistry,
	}
}

// imageStreamMappingStrategy implements behavior for image stream mappings.
type imageStreamMappingStrategy struct {
	runtime.ObjectTyper
	kapi.NameGenerator
}

// Strategy is the default logic that applies when creating ImageStreamMapping
// objects via the REST API.
var Strategy = imageStreamMappingStrategy{kapi.Scheme, kapi.SimpleNameGenerator}

// New returns a new ImageStreamMapping for use with Create.
func (r *REST) New() runtime.Object {
	return &api.ImageStreamMapping{}
}

// NamespaceScoped is true for image stream mappings.
func (s imageStreamMappingStrategy) NamespaceScoped() bool {
	return true
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (s imageStreamMappingStrategy) PrepareForCreate(obj runtime.Object) {
}

// Validate validates a new ImageStreamMapping.
func (s imageStreamMappingStrategy) Validate(ctx kapi.Context, obj runtime.Object) fielderrors.ValidationErrorList {
	mapping := obj.(*api.ImageStreamMapping)
	return validation.ValidateImageStreamMapping(mapping)
}

// Create registers a new image (if it doesn't exist) and updates the
// specified ImageStream's tags. If attempts to update the ImageStream fail
// with a resource conflict, the update will be retried if the newer
// ImageStream has no tag diffs from the previous state. If tag diffs are
// detected, the conflict error is returned.
func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	if err := rest.BeforeCreate(Strategy, ctx, obj); err != nil {
		return nil, err
	}

	mapping := obj.(*api.ImageStreamMapping)

	stream, err := s.findStreamForMapping(ctx, mapping)
	if err != nil {
		return nil, err
	}

	image := mapping.Image
	tag := mapping.Tag
	if len(tag) == 0 {
		tag = api.DefaultImageTag
	}

	if err := s.imageRegistry.CreateImage(ctx, &image); err != nil && !errors.IsAlreadyExists(err) {
		return nil, err
	}

	next := api.TagEvent{
		Created:              unversioned.Now(),
		DockerImageReference: image.DockerImageReference,
		Image:                image.Name,
	}

	var result *unversioned.Status
	var updateConflictErr error
	retryErr := client.RetryOnConflict(wait.Backoff{Steps: maxRetriesOnConflict}, func() error {
		lastEvent := api.LatestTaggedImage(stream, tag)
		if !api.AddTagEventToImageStream(stream, tag, next) {
			// nothing actually changed
			result = &unversioned.Status{Status: unversioned.StatusSuccess}
			return nil
		}
		api.UpdateTrackingTags(stream, tag, next)

		_, err := s.imageStreamRegistry.UpdateImageStreamStatus(ctx, stream)
		if err != nil {
			if !errors.IsConflict(err) {
				return err
			}
			// If the update conflicts, get the latest stream and check for tag
			// updates. If the latest tag hasn't changed, retry.
			latestStream, findLatestErr := s.findStreamForMapping(ctx, mapping)
			if findLatestErr != nil {
				return findLatestErr
			}
			newerEvent := api.LatestTaggedImage(latestStream, tag)
			if lastEvent == nil || kapi.Semantic.DeepEqual(lastEvent, newerEvent) {
				// The tag hasn't changed, so try again with the updated stream.
				stream = latestStream
				return err
			}
			// The tag changed, so return the conflict error back to the client.
			// This is a little indirect but is necessary because we can't just
			// return the conflict error from the retry function (since it would
			// trigger a retry).
			updateConflictErr = err
			return nil
		}
		result = &unversioned.Status{Status: unversioned.StatusSuccess}
		return nil
	})

	if updateConflictErr != nil {
		return nil, updateConflictErr
	}
	if retryErr != nil {
		return nil, retryErr
	}
	return result, nil
}

// findStreamForMapping retrieves an ImageStream whose DockerImageRepository matches dockerRepo.
func (s *REST) findStreamForMapping(ctx kapi.Context, mapping *api.ImageStreamMapping) (*api.ImageStream, error) {
	if len(mapping.Name) > 0 {
		return s.imageStreamRegistry.GetImageStream(ctx, mapping.Name)
	}
	if len(mapping.DockerImageRepository) != 0 {
		list, err := s.imageStreamRegistry.ListImageStreams(ctx, labels.Everything())
		if err != nil {
			return nil, err
		}
		for i := range list.Items {
			if mapping.DockerImageRepository == list.Items[i].Spec.DockerImageRepository {
				return &list.Items[i], nil
			}
		}
		return nil, errors.NewInvalid("imageStreamMapping", "", fielderrors.ValidationErrorList{
			fielderrors.NewFieldNotFound("dockerImageStream", mapping.DockerImageRepository),
		})
	}
	return nil, errors.NewNotFound("ImageStream", "")
}

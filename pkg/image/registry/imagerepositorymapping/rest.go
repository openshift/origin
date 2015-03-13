package imagerepositorymapping

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/api/validation"
	"github.com/openshift/origin/pkg/image/registry/image"
	"github.com/openshift/origin/pkg/image/registry/imagerepository"
)

// REST implements the RESTStorage interface in terms of an image registry and
// image repository registry. It only supports the Create method and is used
// to simplify adding a new Image and tag to an ImageRepository.
type REST struct {
	imageRegistry           image.Registry
	imageRepositoryRegistry imagerepository.Registry
}

// NewREST returns a new REST.
func NewREST(imageRegistry image.Registry, imageRepositoryRegistry imagerepository.Registry) *REST {
	return &REST{
		imageRegistry:           imageRegistry,
		imageRepositoryRegistry: imageRepositoryRegistry,
	}
}

// imageRepositoryMappingStrategy implements behavior for image repository mappings.
type imageRepositoryMappingStrategy struct {
	runtime.ObjectTyper
	kapi.NameGenerator
}

// Strategy is the default logic that applies when creating ImageRepositoryMapping
// objects via the REST API.
var Strategy = imageRepositoryMappingStrategy{kapi.Scheme, kapi.SimpleNameGenerator}

// New returns a new ImageRepositoryMapping for use with Create.
func (r *REST) New() runtime.Object {
	return &api.ImageRepositoryMapping{}
}

// NamespaceScoped is true for image repository mappings.
func (s imageRepositoryMappingStrategy) NamespaceScoped() bool {
	return true
}

// ResetBeforeCreate clears fields that are not allowed to be set by end users on creation.
func (s imageRepositoryMappingStrategy) ResetBeforeCreate(obj runtime.Object) {
}

// Validate validates a new ImageRepositoryMapping.
func (s imageRepositoryMappingStrategy) Validate(obj runtime.Object) errors.ValidationErrorList {
	mapping := obj.(*api.ImageRepositoryMapping)
	return validation.ValidateImageRepositoryMapping(mapping)
}

// Create registers a new image (if it doesn't exist) and updates the specified ImageRepository's tags.
func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	if err := rest.BeforeCreate(Strategy, ctx, obj); err != nil {
		return nil, err
	}

	mapping := obj.(*api.ImageRepositoryMapping)

	repo, err := s.findRepositoryForMapping(ctx, mapping)
	if err != nil {
		return nil, err
	}

	image := mapping.Image

	pullTag := mapping.Tag
	// TODO: default tag value to "latest" or the tag latest points to
	if len(image.DockerImageReference) > 0 {
		if ref, err := api.ParseDockerImageReference(image.DockerImageReference); err == nil {
			// TODO: use a canonical comparison (latest -> latest)
			if ref.Name == repo.Name && ref.Tag != mapping.Tag {
				pullTag = ref.Tag
			}
		}
	}

	if repo.Tags == nil {
		repo.Tags = make(map[string]string)
	}
	repo.Tags[mapping.Tag] = pullTag

	if err := s.imageRegistry.CreateImage(ctx, &image); err != nil && !errors.IsAlreadyExists(err) {
		return nil, err
	}
	if err := s.imageRepositoryRegistry.UpdateImageRepository(ctx, repo); err != nil {
		return nil, err
	}

	return &kapi.Status{Status: kapi.StatusSuccess}, nil
}

// findRepositoryForMapping retrieves an ImageRepository whose DockerImageRepository matches dockerRepo.
func (s *REST) findRepositoryForMapping(ctx kapi.Context, mapping *api.ImageRepositoryMapping) (*api.ImageRepository, error) {
	if len(mapping.Name) > 0 {
		return s.imageRepositoryRegistry.GetImageRepository(ctx, mapping.Name)
	}
	if len(mapping.DockerImageRepository) != 0 {
		list, err := s.imageRepositoryRegistry.ListImageRepositories(ctx, labels.Everything())
		if err != nil {
			return nil, err
		}
		for i := range list.Items {
			if mapping.DockerImageRepository == list.Items[i].DockerImageRepository {
				return &list.Items[i], nil
			}
		}
		return nil, errors.NewInvalid("imageRepositoryMapping", "", errors.ValidationErrorList{
			errors.NewFieldNotFound("dockerImageRepository", mapping.DockerImageRepository),
		})
	}
	return nil, errors.NewNotFound("ImageRepository", "")
}

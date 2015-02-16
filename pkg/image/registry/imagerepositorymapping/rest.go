package imagerepositorymapping

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/api/validation"
	"github.com/openshift/origin/pkg/image/registry/image"
	"github.com/openshift/origin/pkg/image/registry/imagerepository"
)

// REST implements the RESTStorage interface in terms of an Registry and Registry.
// It Only supports the Create method and is used to simply adding a new Image and tag to an ImageRepository.
type REST struct {
	imageRegistry           image.Registry
	imageRepositoryRegistry imagerepository.Registry
}

// NewREST returns a new REST.
func NewREST(imageRegistry image.Registry, imageRepositoryRegistry imagerepository.Registry) apiserver.RESTStorage {
	return &REST{imageRegistry, imageRepositoryRegistry}
}

// New returns a new ImageRepositoryMapping for use with Create.
func (s *REST) New() runtime.Object {
	return &api.ImageRepositoryMapping{}
}

// Create registers a new image (if it doesn't exist) and updates the specified ImageRepository's tags.
func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	mapping := obj.(*api.ImageRepositoryMapping)
	if !kapi.ValidNamespace(ctx, &mapping.ObjectMeta) {
		return nil, errors.NewConflict("imageRepositoryMapping", mapping.Namespace, fmt.Errorf("ImageRepositoryMapping.Namespace does not match the provided context"))
	}
	kapi.FillObjectMetaSystemFields(ctx, &mapping.ObjectMeta)
	kapi.FillObjectMetaSystemFields(ctx, &mapping.Image.ObjectMeta)
	// TODO: allow cross namespace mappings if the user has access
	mapping.Image.Namespace = ""
	if errs := validation.ValidateImageRepositoryMapping(mapping); len(errs) > 0 {
		return nil, errors.NewInvalid("imageRepositoryMapping", mapping.Name, errs)
	}

	repo, err := s.findRepositoryForMapping(ctx, mapping)
	if err != nil {
		return nil, err
	}

	image := mapping.Image

	//TODO apply metadata overrides
	if repo.Tags == nil {
		repo.Tags = make(map[string]string)
	}
	repo.Tags[mapping.Tag] = image.Name

	if err := s.imageRegistry.CreateImage(ctx, &image); err != nil {
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
		//TODO make this more efficient
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

package imagerepositorymapping

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

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

// List is not supported.
func (s *REST) List(ctx kapi.Context, selector, fields labels.Selector) (runtime.Object, error) {
	return nil, errors.NewNotFound("imageRepositoryMapping", "list")
}

// Get is not supported.
func (s *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	return nil, errors.NewNotFound("imageRepositoryMapping", id)
}

// Create registers a new image (if it doesn't exist) and updates the specified ImageRepository's tags.
func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	mapping, ok := obj.(*api.ImageRepositoryMapping)
	if !ok {
		return nil, fmt.Errorf("not an image repository mapping: %#v", obj)
	}
	if !kapi.ValidNamespace(ctx, &mapping.ObjectMeta) {
		return nil, errors.NewConflict("imageRepositoryMapping", mapping.Namespace, fmt.Errorf("ImageRepositoryMapping.Namespace does not match the provided context"))
	}

	repo, err := s.findImageRepository(ctx, mapping.DockerImageRepository)

	if err != nil {
		return nil, err
	}
	if repo == nil {
		return nil, errors.NewInvalid("imageRepositoryMapping", mapping.Name, errors.ValidationErrorList{
			errors.NewFieldNotFound("DockerImageRepository", mapping.DockerImageRepository),
		})
	}

	// you should not do this, but we have a bug right now that prevents us from trusting the ctx passed in
	imageRepoCtx := kapi.WithNamespace(kapi.NewContext(), repo.Namespace)

	if errs := validation.ValidateImageRepositoryMapping(mapping); len(errs) > 0 {
		return nil, errors.NewInvalid("imageRepositoryMapping", mapping.Name, errs)
	}

	image := mapping.Image

	image.CreationTimestamp = util.Now()

	//TODO apply metadata overrides
	if repo.Tags == nil {
		repo.Tags = make(map[string]string)
	}
	repo.Tags[mapping.Tag] = image.Name

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		err = s.imageRegistry.CreateImage(imageRepoCtx, &image)
		if err != nil && !errors.IsAlreadyExists(err) {
			return nil, err
		}

		err = s.imageRepositoryRegistry.UpdateImageRepository(imageRepoCtx, repo)
		if err != nil {
			return nil, err
		}

		return &kapi.Status{Status: kapi.StatusSuccess}, nil
	}), nil
}

// findImageRepository retrieves an ImageRepository whose DockerImageRepository matches dockerRepo.
func (s *REST) findImageRepository(ctx kapi.Context, dockerRepo string) (*api.ImageRepository, error) {
	//TODO make this more efficient
	// you should not do this, but we have a bug right now that prevents us from trusting the ctx passed in
	allNamespaces := kapi.NewContext()
	list, err := s.imageRepositoryRegistry.ListImageRepositories(allNamespaces, labels.Everything())
	if err != nil {
		return nil, err
	}

	var repo *api.ImageRepository
	for _, r := range list.Items {
		if dockerRepo == r.DockerImageRepository {
			repo = &r
			break
		}
	}

	return repo, nil
}

// Update is not supported.
func (s *REST) Update(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	return nil, fmt.Errorf("ImageRepositoryMappings may not be changed.")
}

// Delete is not supported.
func (s *REST) Delete(ctx kapi.Context, id string) (<-chan apiserver.RESTResult, error) {
	return nil, errors.NewNotFound("imageRepositoryMapping", id)
}

package imagerepository

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/api/validation"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	registry Registry
}

// NewREST returns a new REST.
func NewREST(registry Registry) apiserver.RESTStorage {
	return &REST{
		registry: registry,
	}
}

// New returns a new ImageRepository for use with Create and Update.
func (s *REST) New() runtime.Object {
	return &api.ImageRepository{}
}

func (*REST) NewList() runtime.Object {
	return &api.ImageRepository{}
}

// List retrieves a list of ImageRepositories that match selector.
func (s *REST) List(ctx kapi.Context, selector, fields labels.Selector) (runtime.Object, error) {
	return s.registry.ListImageRepositories(ctx, selector)
}

// Get retrieves an ImageRepository by id.
func (s *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	return s.registry.GetImageRepository(ctx, id)
}

// Watch begins watching for new, changed, or deleted ImageRepositories.
func (s *REST) Watch(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	return s.registry.WatchImageRepositories(ctx, label, field, resourceVersion)
}

// Create registers the given ImageRepository.
func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	repo := obj.(*api.ImageRepository)
	if !kapi.ValidNamespace(ctx, &repo.ObjectMeta) {
		return nil, errors.NewConflict("imageRepository", repo.Namespace, fmt.Errorf("ImageRepository.Namespace does not match the provided context"))
	}

	kapi.FillObjectMetaSystemFields(ctx, &repo.ObjectMeta)
	if errs := validation.ValidateImageRepository(repo); len(errs) > 0 {
		return nil, errors.NewInvalid("imageRepository", repo.Name, errs)
	}

	repo.Status = api.ImageRepositoryStatus{}
	if err := s.registry.CreateImageRepository(ctx, repo); err != nil {
		return nil, err
	}
	return s.Get(ctx, repo.Name)
}

// Update replaces an existing ImageRepository in the registry with the given ImageRepository.
func (s *REST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	repo := obj.(*api.ImageRepository)
	if !kapi.ValidNamespace(ctx, &repo.ObjectMeta) {
		return nil, false, errors.NewConflict("imageRepository", repo.Namespace, fmt.Errorf("ImageRepository.Namespace does not match the provided context"))
	}
	if errs := validation.ValidateImageRepository(repo); len(errs) > 0 {
		return nil, false, errors.NewInvalid("imageRepository", repo.Name, errs)
	}

	repo.Status = api.ImageRepositoryStatus{}

	if err := s.registry.UpdateImageRepository(ctx, repo); err != nil {
		return nil, false, err
	}
	out, err := s.Get(ctx, repo.Name)
	return out, false, err
}

// Delete asynchronously deletes an ImageRepository specified by its id.
func (s *REST) Delete(ctx kapi.Context, id string) (runtime.Object, error) {
	return &kapi.Status{Status: kapi.StatusSuccess}, s.registry.DeleteImageRepository(ctx, id)
}

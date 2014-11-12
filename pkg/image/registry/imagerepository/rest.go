package imagerepository

import (
	"fmt"

	"code.google.com/p/go-uuid/uuid"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/image/api"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	registry Registry
}

// NewREST returns a new REST.
func NewREST(registry Registry) apiserver.RESTStorage {
	return &REST{registry}
}

// New returns a new ImageRepository for use with Create and Update.
func (s *REST) New() runtime.Object {
	return &api.ImageRepository{}
}

// List retrieves a list of ImageRepositories that match selector.
func (s *REST) List(ctx kapi.Context, selector, fields labels.Selector) (runtime.Object, error) {
	imageRepositories, err := s.registry.ListImageRepositories(ctx, selector)
	if err != nil {
		return nil, err
	}
	return imageRepositories, err
}

// Get retrieves an ImageRepository by id.
func (s *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	repo, err := s.registry.GetImageRepository(ctx, id)
	if err != nil {
		return nil, err
	}
	return repo, nil
}

// Watch begins watching for new, changed, or deleted ImageRepositories.
func (s *REST) Watch(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	return s.registry.WatchImageRepositories(ctx, resourceVersion, func(repo *api.ImageRepository) bool {
		fields := labels.Set{
			"Name":                  repo.Name,
			"DockerImageRepository": repo.DockerImageRepository,
		}
		return label.Matches(labels.Set(repo.Labels)) && field.Matches(fields)
	})
}

// Create registers the given ImageRepository.
func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	repo, ok := obj.(*api.ImageRepository)
	if !ok {
		return nil, fmt.Errorf("not an image repository: %#v", obj)
	}
	if !kapi.ValidNamespace(ctx, &repo.ObjectMeta) {
		return nil, errors.NewConflict("imageRepository", repo.Namespace, fmt.Errorf("ImageRepository.Namespace does not match the provided context"))
	}

	if len(repo.Name) == 0 {
		repo.Name = uuid.NewUUID().String()
	}

	if repo.Tags == nil {
		repo.Tags = make(map[string]string)
	}

	repo.CreationTimestamp = util.Now()

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		if err := s.registry.CreateImageRepository(ctx, repo); err != nil {
			return nil, err
		}
		return s.Get(ctx, repo.Name)
	}), nil
}

// Update replaces an existing ImageRepository in the registry with the given ImageRepository.
func (s *REST) Update(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	repo, ok := obj.(*api.ImageRepository)
	if !ok {
		return nil, fmt.Errorf("not an image repository: %#v", obj)
	}
	if len(repo.Name) == 0 {
		return nil, fmt.Errorf("id is unspecified: %#v", repo)
	}
	if !kapi.ValidNamespace(ctx, &repo.ObjectMeta) {
		return nil, errors.NewConflict("imageRepository", repo.Namespace, fmt.Errorf("ImageRepository.Namespace does not match the provided context"))
	}

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		err := s.registry.UpdateImageRepository(ctx, repo)
		if err != nil {
			return nil, err
		}
		return s.Get(ctx, repo.Name)
	}), nil
}

// Delete asynchronously deletes an ImageRepository specified by its id.
func (s *REST) Delete(ctx kapi.Context, id string) (<-chan apiserver.RESTResult, error) {
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		return &kapi.Status{Status: kapi.StatusSuccess}, s.registry.DeleteImageRepository(ctx, id)
	}), nil
}

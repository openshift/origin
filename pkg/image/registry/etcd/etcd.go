package etcd

import (
	"errors"

	etcderr "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/image/api"
)

// Etcd implements ImageRegistry and ImageRepositoryRegistry backed by etcd.
type Etcd struct {
	tools.EtcdHelper
}

// New returns a new etcd registry.
func New(helper tools.EtcdHelper) *Etcd {
	return &Etcd{
		EtcdHelper: helper,
	}
}

// ListImages retrieves a list of images that match selector.
func (r *Etcd) ListImages(selector labels.Selector) (*api.ImageList, error) {
	list := api.ImageList{}
	err := r.ExtractList("/images", &list.Items, &list.ResourceVersion)
	if err != nil {
		return nil, err
	}
	filtered := []api.Image{}
	for _, item := range list.Items {
		if selector.Matches(labels.Set(item.Labels)) {
			filtered = append(filtered, item)
		}
	}
	list.Items = filtered
	return &list, nil
}

func makeImageKey(id string) string {
	return "/images/" + id
}

// GetImage retrieves a specific image
func (r *Etcd) GetImage(id string) (*api.Image, error) {
	var image api.Image
	if err := r.ExtractObj(makeImageKey(id), &image, false); err != nil {
		return nil, etcderr.InterpretGetError(err, "image", id)
	}
	return &image, nil
}

// CreateImage creates a new image
func (r *Etcd) CreateImage(image *api.Image) error {
	err := r.CreateObj(makeImageKey(image.ID), image, 0)
	return etcderr.InterpretCreateError(err, "image", image.ID)
}

// UpdateImage updates an existing image
func (r *Etcd) UpdateImage(image *api.Image) error {
	return errors.New("not supported")
}

// DeleteImage deletes an existing image
func (r *Etcd) DeleteImage(id string) error {
	key := makeImageKey(id)
	err := r.Delete(key, false)
	return etcderr.InterpretDeleteError(err, "image", id)
}

// ListImageRepositories retrieves a list of ImageRepositories that match selector.
func (r *Etcd) ListImageRepositories(selector labels.Selector) (*api.ImageRepositoryList, error) {
	list := api.ImageRepositoryList{}
	err := r.ExtractList("/imageRepositories", &list.Items, &list.ResourceVersion)
	if err != nil {
		return nil, err
	}
	filtered := []api.ImageRepository{}
	for _, item := range list.Items {
		if selector.Matches(labels.Set(item.Labels)) {
			filtered = append(filtered, item)
		}
	}
	list.Items = filtered
	return &list, nil
}

func makeImageRepositoryKey(id string) string {
	return "/imageRepositories/" + id
}

// GetImageRepository retrieves an ImageRepository by id.
func (r *Etcd) GetImageRepository(id string) (*api.ImageRepository, error) {
	var repo api.ImageRepository
	if err := r.ExtractObj(makeImageRepositoryKey(id), &repo, false); err != nil {
		return nil, etcderr.InterpretGetError(err, "imageRepository", id)
	}
	return &repo, nil
}

// WatchImageRepositories begins watching for new, changed, or deleted ImageRepositories.
func (r *Etcd) WatchImageRepositories(resourceVersion uint64, filter func(repo *api.ImageRepository) bool) (watch.Interface, error) {
	return r.WatchList("/imageRepositories", resourceVersion, func(obj runtime.Object) bool {
		repo, ok := obj.(*api.ImageRepository)
		if !ok {
			glog.Errorf("Unexpected object during image repository watch: %#v", obj)
			return false
		}
		return filter(repo)
	})
}

// CreateImageRepository registers the given ImageRepository.
func (r *Etcd) CreateImageRepository(repo *api.ImageRepository) error {
	err := r.CreateObj(makeImageRepositoryKey(repo.ID), repo, 0)
	return etcderr.InterpretCreateError(err, "imageRepository", repo.ID)
}

// UpdateImageRepository replaces an existing ImageRepository in the registry with the given ImageRepository.
func (r *Etcd) UpdateImageRepository(repo *api.ImageRepository) error {
	err := r.SetObj(makeImageRepositoryKey(repo.ID), repo)
	return etcderr.InterpretUpdateError(err, "imageRepository", repo.ID)
}

// DeleteImageRepository deletes an ImageRepository by id.
func (r *Etcd) DeleteImageRepository(id string) error {
	imageRepositoryKey := makeImageRepositoryKey(id)
	err := r.Delete(imageRepositoryKey, false)
	return etcderr.InterpretDeleteError(err, "imageRepository", id)
}

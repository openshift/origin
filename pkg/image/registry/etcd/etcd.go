package etcd

import (
	"errors"
	"fmt"
	"reflect"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	apierrs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	etcderr "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	kubeetcd "github.com/GoogleCloudPlatform/kubernetes/pkg/registry/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	ktools "github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/image/api"
)

const (
	// ImagePath is the path to deployment image in etcd
	ImagePath string = "/images"
	// ImageRepositoriesPath is the path to imageRepository resources in etcd
	ImageRepositoriesPath string = "/imageRepositories"
)

// DefaultRegistry returns the default Docker registry (host or host:port), or false if it is not available.
type DefaultRegistry interface {
	DefaultRegistry() (string, bool)
}

// DefaultRegistryFunc implements DefaultRegistry for a simple function.
type DefaultRegistryFunc func() (string, bool)

// DefaultRegistry implements the DefaultRegistry interface for a function.
func (fn DefaultRegistryFunc) DefaultRegistry() (string, bool) {
	return fn()
}

// Etcd implements ImageRegistry and ImageRepositoryRegistry backed by etcd.
type Etcd struct {
	tools.EtcdHelper
	defaultRegistry DefaultRegistry
}

// New returns a new etcd registry. Default registry is the value that will be
// applied to the Status.DockerImageRepository field if the repository does not
// have a specified DockerImageRepository.
func New(helper tools.EtcdHelper, defaultRegistry DefaultRegistry) *Etcd {
	return &Etcd{
		EtcdHelper:      helper,
		defaultRegistry: defaultRegistry,
	}
}

// ListImages retrieves a list of images that match selector.
func (r *Etcd) ListImages(ctx kapi.Context, selector labels.Selector) (*api.ImageList, error) {
	list := api.ImageList{}
	err := r.ExtractToList(makeImageListKey(ctx), &list)
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

func makeImageListKey(ctx kapi.Context) string {
	return kubeetcd.MakeEtcdListKey(ctx, ImagePath)
}

func makeImageKey(ctx kapi.Context, id string) (string, error) {
	return kubeetcd.MakeEtcdItemKey(ctx, ImagePath, id)
}

// GetImage retrieves a specific image
func (r *Etcd) GetImage(ctx kapi.Context, id string) (*api.Image, error) {
	var image api.Image
	key, err := makeImageKey(ctx, id)
	if err != nil {
		return nil, err
	}

	if err = r.ExtractObj(key, &image, false); err != nil {
		return nil, etcderr.InterpretGetError(err, "image", id)
	}
	return &image, nil
}

// CreateImage creates a new image
func (r *Etcd) CreateImage(ctx kapi.Context, image *api.Image) error {
	key, err := makeImageKey(ctx, image.Name)
	if err != nil {
		return err
	}

	err = r.AtomicUpdate(key, &api.Image{}, true, func(obj runtime.Object) (runtime.Object, error) {
		existing := obj.(*api.Image)
		if isNewObject(existing.ResourceVersion) {
			return image, nil
		}
		if equivalentImage(existing, image) {
			return existing, nil
		}
		return nil, apierrs.NewAlreadyExists("image", image.Name)
	})
	return etcderr.InterpretCreateError(err, "image", image.Name)
}

// isNewObject returns true if the provided resource version indicates the object has not been previously persisted.
func isNewObject(resourceVersion string) bool {
	v, _ := ktools.ParseWatchResourceVersion(resourceVersion, "")
	return v == 0
}

// equivalentImage returns true if the provided images have matching image metadata and reference location
func equivalentImage(a, b *api.Image) bool {
	if !reflect.DeepEqual(a.DockerImageMetadata, b.DockerImageMetadata) {
		return false
	}
	if a.DockerImageReference != b.DockerImageReference {
		return false
	}
	return true
}

// UpdateImage updates an existing image
func (r *Etcd) UpdateImage(ctx kapi.Context, image *api.Image) error {
	return errors.New("not supported")
}

// DeleteImage deletes an existing image
func (r *Etcd) DeleteImage(ctx kapi.Context, id string) error {
	key, err := makeImageKey(ctx, id)
	if err != nil {
		return err
	}

	err = r.Delete(key, false)
	return etcderr.InterpretDeleteError(err, "image", id)
}

// WatchImages begins watching for new or deleted Images.
func (r *Etcd) WatchImages(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	version, err := ktools.ParseWatchResourceVersion(resourceVersion, "image")
	if err != nil {
		return nil, err
	}
	if !field.Empty() {
		return nil, fmt.Errorf("field selectors are not supported on images")
	}

	return r.WatchList(makeImageListKey(ctx), version, func(obj runtime.Object) bool {
		image, ok := obj.(*api.Image)
		if !ok {
			glog.Errorf("Unexpected object during image watch: %#v", obj)
			return false
		}
		return label.Matches(labels.Set(image.Labels))
	})
}

// ListImageRepositories retrieves a list of ImageRepositories that match selector.
func (r *Etcd) ListImageRepositories(ctx kapi.Context, selector labels.Selector) (*api.ImageRepositoryList, error) {
	list := api.ImageRepositoryList{}
	err := r.ExtractToList(makeImageRepositoryListKey(ctx), &list)
	if err != nil {
		return nil, err
	}
	filtered := []api.ImageRepository{}
	for _, item := range list.Items {
		if selector.Matches(labels.Set(item.Labels)) {
			r.fillRepository(&item)
			filtered = append(filtered, item)
		}
	}
	list.Items = filtered
	return &list, nil
}

func makeImageRepositoryListKey(ctx kapi.Context) string {
	return kubeetcd.MakeEtcdListKey(ctx, ImageRepositoriesPath)
}

func makeImageRepositoryKey(ctx kapi.Context, id string) (string, error) {
	return kubeetcd.MakeEtcdItemKey(ctx, ImageRepositoriesPath, id)
}

// GetImageRepository retrieves an ImageRepository by id.
func (r *Etcd) GetImageRepository(ctx kapi.Context, id string) (*api.ImageRepository, error) {
	var repo api.ImageRepository
	key, err := makeImageRepositoryKey(ctx, id)
	if err != nil {
		return nil, err
	}
	if err = r.ExtractObj(key, &repo, false); err != nil {
		return nil, etcderr.InterpretGetError(err, "imageRepository", id)
	}
	return r.fillRepository(&repo), nil
}

// WatchImageRepositories begins watching for new, changed, or deleted ImageRepositories.
func (r *Etcd) WatchImageRepositories(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	version, err := ktools.ParseWatchResourceVersion(resourceVersion, "imageRepository")
	if err != nil {
		return nil, err
	}

	return r.WatchList(makeImageRepositoryListKey(ctx), version, func(obj runtime.Object) bool {
		repo, ok := obj.(*api.ImageRepository)
		if !ok {
			glog.Errorf("Unexpected object during image repository watch: %#v", obj)
			return false
		}
		fields := labels.Set{
			"name":                  repo.Name,
			"dockerImageRepository": repo.DockerImageRepository,
		}
		if !label.Matches(labels.Set(repo.Labels)) || !field.Matches(fields) {
			return false
		}
		r.fillRepository(repo)
		return true
	})
}

// CreateImageRepository registers the given ImageRepository.
func (r *Etcd) CreateImageRepository(ctx kapi.Context, repo *api.ImageRepository) error {
	key, err := makeImageRepositoryKey(ctx, repo.Name)
	if err != nil {
		return err
	}
	err = r.CreateObj(key, repo, 0)
	return etcderr.InterpretCreateError(err, "imageRepository", repo.Name)
}

// UpdateImageRepository replaces an existing ImageRepository in the registry with the given ImageRepository.
func (r *Etcd) UpdateImageRepository(ctx kapi.Context, repo *api.ImageRepository) error {
	key, err := makeImageRepositoryKey(ctx, repo.Name)
	if err != nil {
		return err
	}
	err = r.SetObj(key, repo, 0)
	return etcderr.InterpretUpdateError(err, "imageRepository", repo.Name)
}

// DeleteImageRepository deletes an ImageRepository by id.
func (r *Etcd) DeleteImageRepository(ctx kapi.Context, id string) error {
	key, err := makeImageRepositoryKey(ctx, id)
	if err != nil {
		return err
	}
	err = r.Delete(key, false)
	return etcderr.InterpretDeleteError(err, "imageRepository", id)
}

// fillRepository sets the status information of a repository
func (r *Etcd) fillRepository(repo *api.ImageRepository) *api.ImageRepository {
	var value string
	if len(repo.DockerImageRepository) != 0 {
		value = repo.DockerImageRepository
	} else {
		registry, ok := r.defaultRegistry.DefaultRegistry()
		if ok {
			if len(repo.Namespace) == 0 {
				repo.Namespace = kapi.NamespaceDefault
			}
			value = api.JoinDockerPullSpec(registry, repo.Namespace, repo.Name, "")
		}
	}
	repo.Status.DockerImageRepository = value
	return repo
}

package test

import (
	"sync"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/openshift/origin/pkg/image/api"
)

type ImageRegistry struct {
	Err    error
	Image  *api.Image
	Images *api.ImageList
	sync.Mutex
}

func NewImageRegistry() *ImageRegistry {
	return &ImageRegistry{}
}

func (r *ImageRegistry) ListImages(ctx kapi.Context, selector labels.Selector) (*api.ImageList, error) {
	r.Lock()
	defer r.Unlock()

	return r.Images, r.Err
}

func (r *ImageRegistry) GetImage(ctx kapi.Context, id string) (*api.Image, error) {
	r.Lock()
	defer r.Unlock()

	return r.Image, r.Err
}

func (r *ImageRegistry) CreateImage(ctx kapi.Context, image *api.Image) error {
	r.Lock()
	defer r.Unlock()

	r.Image = image
	return r.Err
}

func (r *ImageRegistry) UpdateImage(ctx kapi.Context, image *api.Image) error {
	r.Lock()
	defer r.Unlock()

	r.Image = image
	return r.Err
}

func (r *ImageRegistry) DeleteImage(ctx kapi.Context, id string) error {
	r.Lock()
	defer r.Unlock()

	return r.Err
}

func (r *ImageRegistry) WatchImages(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	r.Lock()
	defer r.Unlock()

	return nil, r.Err
}

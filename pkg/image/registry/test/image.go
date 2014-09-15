package test

import (
	"sync"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
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

func (r *ImageRegistry) ListImages(selector labels.Selector) (*api.ImageList, error) {
	r.Lock()
	defer r.Unlock()

	return r.Images, r.Err
}

func (r *ImageRegistry) GetImage(id string) (*api.Image, error) {
	r.Lock()
	defer r.Unlock()

	return r.Image, r.Err
}

func (r *ImageRegistry) CreateImage(image *api.Image) error {
	r.Lock()
	defer r.Unlock()

	r.Image = image
	return r.Err
}

func (r *ImageRegistry) UpdateImage(image *api.Image) error {
	r.Lock()
	defer r.Unlock()

	r.Image = image
	return r.Err
}

func (r *ImageRegistry) DeleteImage(id string) error {
	r.Lock()
	defer r.Unlock()

	return r.Err
}

package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

// FakeImageRepositories implements ImageRepositoryInterface. Meant to be
// embedded into a struct to get a default implementation. This makes faking
// out just the methods you want to test easier.
type FakeImageRepositories struct {
	Fake      *Fake
	Namespace string
}

var _ ImageRepositoryInterface = &FakeImageRepositories{}

func (c *FakeImageRepositories) List(label labels.Selector, field fields.Selector) (*imageapi.ImageRepositoryList, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "list-imagerepositories"})
	return &imageapi.ImageRepositoryList{}, nil
}

func (c *FakeImageRepositories) Get(name string) (*imageapi.ImageRepository, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "get-imagerepository", Value: name})
	return &imageapi.ImageRepository{}, nil
}

func (c *FakeImageRepositories) Create(repo *imageapi.ImageRepository) (*imageapi.ImageRepository, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "create-imagerepository"})
	return &imageapi.ImageRepository{}, nil
}

func (c *FakeImageRepositories) Update(repo *imageapi.ImageRepository) (*imageapi.ImageRepository, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "update-imagerepository"})
	return &imageapi.ImageRepository{}, nil
}

func (c *FakeImageRepositories) Delete(name string) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-imagerepository", Value: name})
	return nil
}

func (c *FakeImageRepositories) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "watch-imagerepositories"})
	return nil, nil
}

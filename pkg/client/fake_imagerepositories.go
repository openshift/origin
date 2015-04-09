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
	obj, err := c.Fake.Invokes(FakeAction{Action: "list-imagerepositories"}, &imageapi.ImageRepositoryList{})
	return obj.(*imageapi.ImageRepositoryList), err
}

func (c *FakeImageRepositories) Get(name string) (*imageapi.ImageRepository, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "get-imagerepository", Value: name}, &imageapi.ImageRepository{})
	return obj.(*imageapi.ImageRepository), err
}

func (c *FakeImageRepositories) Create(repo *imageapi.ImageRepository) (*imageapi.ImageRepository, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "create-imagerepository"}, &imageapi.ImageRepository{})
	return obj.(*imageapi.ImageRepository), err
}

func (c *FakeImageRepositories) Update(repo *imageapi.ImageRepository) (*imageapi.ImageRepository, error) {
	obj, err := c.Fake.Invokes(FakeAction{Action: "update-imagerepository"}, &imageapi.ImageRepository{})
	return obj.(*imageapi.ImageRepository), err
}

func (c *FakeImageRepositories) Delete(name string) error {
	_, err := c.Fake.Invokes(FakeAction{Action: "delete-imagerepository", Value: name}, &imageapi.ImageRepository{})
	return err
}

func (c *FakeImageRepositories) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "watch-imagerepositories"})
	return nil, nil
}

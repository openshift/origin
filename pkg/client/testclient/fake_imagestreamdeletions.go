package testclient

import (
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// FakeImageStreamDeletions implements ImageStreamDeletionInterface. Meant to be
// embedded into a struct to get a default implementation. This makes faking
// out just the methods you want to test easier.
type FakeImageStreamDeletions struct {
	Fake *Fake
}

var _ client.ImageStreamDeletionInterface = &FakeImageStreamDeletions{}

func (c *FakeImageStreamDeletions) Get(name string) (*imageapi.ImageStreamDeletion, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootGetAction("imagestreamdeletions", name), &imageapi.ImageStreamDeletion{})
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStreamDeletion), err
}

func (c *FakeImageStreamDeletions) List(label labels.Selector, field fields.Selector) (*imageapi.ImageStreamDeletionList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootListAction("imagestreamdeletions", label, field), &imageapi.ImageStreamDeletionList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStreamDeletionList), err
}

func (c *FakeImageStreamDeletions) Create(inObj *imageapi.ImageStreamDeletion) (*imageapi.ImageStreamDeletion, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootCreateAction("imagestreamdeletions", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*imageapi.ImageStreamDeletion), err
}

func (c *FakeImageStreamDeletions) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewRootDeleteAction("imagestreamdeletions", name), &imageapi.ImageStreamDeletion{})
	return err
}

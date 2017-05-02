package fake

import (
	api "github.com/openshift/origin/pkg/image/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeImageStreams implements ImageStreamInterface
type FakeImageStreams struct {
	Fake *FakeImage
	ns   string
}

var imagestreamsResource = unversioned.GroupVersionResource{Group: "image.openshift.io", Version: "", Resource: "imagestreams"}

func (c *FakeImageStreams) Create(imageStream *api.ImageStream) (result *api.ImageStream, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(imagestreamsResource, c.ns, imageStream), &api.ImageStream{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.ImageStream), err
}

func (c *FakeImageStreams) Update(imageStream *api.ImageStream) (result *api.ImageStream, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(imagestreamsResource, c.ns, imageStream), &api.ImageStream{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.ImageStream), err
}

func (c *FakeImageStreams) Delete(name string, options *pkg_api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(imagestreamsResource, c.ns, name), &api.ImageStream{})

	return err
}

func (c *FakeImageStreams) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	action := core.NewDeleteCollectionAction(imagestreamsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.ImageStreamList{})
	return err
}

func (c *FakeImageStreams) Get(name string) (result *api.ImageStream, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(imagestreamsResource, c.ns, name), &api.ImageStream{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.ImageStream), err
}

func (c *FakeImageStreams) List(opts pkg_api.ListOptions) (result *api.ImageStreamList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(imagestreamsResource, c.ns, opts), &api.ImageStreamList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := core.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.ImageStreamList{}
	for _, item := range obj.(*api.ImageStreamList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested imageStreams.
func (c *FakeImageStreams) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(imagestreamsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched imageStream.
func (c *FakeImageStreams) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.ImageStream, err error) {
	obj, err := c.Fake.
		Invokes(core.NewPatchSubresourceAction(imagestreamsResource, c.ns, name, data, subresources...), &api.ImageStream{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.ImageStream), err
}

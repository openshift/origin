package fake

import (
	api "github.com/openshift/origin/pkg/image/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeImageStreamImages implements ImageStreamImageInterface
type FakeImageStreamImages struct {
	Fake *FakeImage
}

var imagestreamimagesResource = unversioned.GroupVersionResource{Group: "image.openshift.io", Version: "", Resource: "imagestreamimages"}

func (c *FakeImageStreamImages) Create(imageStreamImage *api.ImageStreamImage) (result *api.ImageStreamImage, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootCreateAction(imagestreamimagesResource, imageStreamImage), &api.ImageStreamImage{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ImageStreamImage), err
}

func (c *FakeImageStreamImages) Update(imageStreamImage *api.ImageStreamImage) (result *api.ImageStreamImage, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootUpdateAction(imagestreamimagesResource, imageStreamImage), &api.ImageStreamImage{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ImageStreamImage), err
}

func (c *FakeImageStreamImages) Delete(name string, options *pkg_api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewRootDeleteAction(imagestreamimagesResource, name), &api.ImageStreamImage{})
	return err
}

func (c *FakeImageStreamImages) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	action := core.NewRootDeleteCollectionAction(imagestreamimagesResource, listOptions)

	_, err := c.Fake.Invokes(action, &api.ImageStreamImageList{})
	return err
}

func (c *FakeImageStreamImages) Get(name string) (result *api.ImageStreamImage, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootGetAction(imagestreamimagesResource, name), &api.ImageStreamImage{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ImageStreamImage), err
}

func (c *FakeImageStreamImages) List(opts pkg_api.ListOptions) (result *api.ImageStreamImageList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootListAction(imagestreamimagesResource, opts), &api.ImageStreamImageList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := core.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.ImageStreamImageList{}
	for _, item := range obj.(*api.ImageStreamImageList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested imageStreamImages.
func (c *FakeImageStreamImages) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewRootWatchAction(imagestreamimagesResource, opts))
}

// Patch applies the patch and returns the patched imageStreamImage.
func (c *FakeImageStreamImages) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.ImageStreamImage, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootPatchSubresourceAction(imagestreamimagesResource, name, data, subresources...), &api.ImageStreamImage{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ImageStreamImage), err
}

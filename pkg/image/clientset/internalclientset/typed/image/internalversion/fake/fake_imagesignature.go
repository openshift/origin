package fake

import (
	api "github.com/openshift/origin/pkg/image/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeImageSignatures implements ImageSignatureInterface
type FakeImageSignatures struct {
	Fake *FakeImage
}

var imagesignaturesResource = unversioned.GroupVersionResource{Group: "image.openshift.io", Version: "", Resource: "imagesignatures"}

func (c *FakeImageSignatures) Create(imageSignature *api.ImageSignature) (result *api.ImageSignature, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootCreateAction(imagesignaturesResource, imageSignature), &api.ImageSignature{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ImageSignature), err
}

func (c *FakeImageSignatures) Update(imageSignature *api.ImageSignature) (result *api.ImageSignature, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootUpdateAction(imagesignaturesResource, imageSignature), &api.ImageSignature{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ImageSignature), err
}

func (c *FakeImageSignatures) Delete(name string, options *pkg_api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewRootDeleteAction(imagesignaturesResource, name), &api.ImageSignature{})
	return err
}

func (c *FakeImageSignatures) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	action := core.NewRootDeleteCollectionAction(imagesignaturesResource, listOptions)

	_, err := c.Fake.Invokes(action, &api.ImageSignatureList{})
	return err
}

func (c *FakeImageSignatures) Get(name string) (result *api.ImageSignature, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootGetAction(imagesignaturesResource, name), &api.ImageSignature{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ImageSignature), err
}

func (c *FakeImageSignatures) List(opts pkg_api.ListOptions) (result *api.ImageSignatureList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootListAction(imagesignaturesResource, opts), &api.ImageSignatureList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := core.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.ImageSignatureList{}
	for _, item := range obj.(*api.ImageSignatureList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested imageSignatures.
func (c *FakeImageSignatures) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewRootWatchAction(imagesignaturesResource, opts))
}

// Patch applies the patch and returns the patched imageSignature.
func (c *FakeImageSignatures) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.ImageSignature, err error) {
	obj, err := c.Fake.
		Invokes(core.NewRootPatchSubresourceAction(imagesignaturesResource, name, data, subresources...), &api.ImageSignature{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.ImageSignature), err
}

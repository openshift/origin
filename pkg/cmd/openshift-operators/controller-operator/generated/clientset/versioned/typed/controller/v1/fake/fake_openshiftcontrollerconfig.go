package fake

import (
	controller_v1 "github.com/openshift/origin/pkg/cmd/openshift-operators/controller-operator/apis/controller/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeOpenShiftControllerConfigs implements OpenShiftControllerConfigInterface
type FakeOpenShiftControllerConfigs struct {
	Fake *FakeControllerV1
}

var openshiftcontrollerconfigsResource = schema.GroupVersionResource{Group: "controller", Version: "v1", Resource: "openshiftcontrollerconfigs"}

var openshiftcontrollerconfigsKind = schema.GroupVersionKind{Group: "controller", Version: "v1", Kind: "OpenShiftControllerConfig"}

// Get takes name of the openShiftControllerConfig, and returns the corresponding openShiftControllerConfig object, and an error if there is any.
func (c *FakeOpenShiftControllerConfigs) Get(name string, options v1.GetOptions) (result *controller_v1.OpenShiftControllerConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(openshiftcontrollerconfigsResource, name), &controller_v1.OpenShiftControllerConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*controller_v1.OpenShiftControllerConfig), err
}

// List takes label and field selectors, and returns the list of OpenShiftControllerConfigs that match those selectors.
func (c *FakeOpenShiftControllerConfigs) List(opts v1.ListOptions) (result *controller_v1.OpenShiftControllerConfigList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(openshiftcontrollerconfigsResource, openshiftcontrollerconfigsKind, opts), &controller_v1.OpenShiftControllerConfigList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &controller_v1.OpenShiftControllerConfigList{}
	for _, item := range obj.(*controller_v1.OpenShiftControllerConfigList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested openShiftControllerConfigs.
func (c *FakeOpenShiftControllerConfigs) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(openshiftcontrollerconfigsResource, opts))
}

// Create takes the representation of a openShiftControllerConfig and creates it.  Returns the server's representation of the openShiftControllerConfig, and an error, if there is any.
func (c *FakeOpenShiftControllerConfigs) Create(openShiftControllerConfig *controller_v1.OpenShiftControllerConfig) (result *controller_v1.OpenShiftControllerConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(openshiftcontrollerconfigsResource, openShiftControllerConfig), &controller_v1.OpenShiftControllerConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*controller_v1.OpenShiftControllerConfig), err
}

// Update takes the representation of a openShiftControllerConfig and updates it. Returns the server's representation of the openShiftControllerConfig, and an error, if there is any.
func (c *FakeOpenShiftControllerConfigs) Update(openShiftControllerConfig *controller_v1.OpenShiftControllerConfig) (result *controller_v1.OpenShiftControllerConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(openshiftcontrollerconfigsResource, openShiftControllerConfig), &controller_v1.OpenShiftControllerConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*controller_v1.OpenShiftControllerConfig), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeOpenShiftControllerConfigs) UpdateStatus(openShiftControllerConfig *controller_v1.OpenShiftControllerConfig) (*controller_v1.OpenShiftControllerConfig, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(openshiftcontrollerconfigsResource, "status", openShiftControllerConfig), &controller_v1.OpenShiftControllerConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*controller_v1.OpenShiftControllerConfig), err
}

// Delete takes name of the openShiftControllerConfig and deletes it. Returns an error if one occurs.
func (c *FakeOpenShiftControllerConfigs) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(openshiftcontrollerconfigsResource, name), &controller_v1.OpenShiftControllerConfig{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeOpenShiftControllerConfigs) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(openshiftcontrollerconfigsResource, listOptions)

	_, err := c.Fake.Invokes(action, &controller_v1.OpenShiftControllerConfigList{})
	return err
}

// Patch applies the patch and returns the patched openShiftControllerConfig.
func (c *FakeOpenShiftControllerConfigs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *controller_v1.OpenShiftControllerConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(openshiftcontrollerconfigsResource, name, data, subresources...), &controller_v1.OpenShiftControllerConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*controller_v1.OpenShiftControllerConfig), err
}

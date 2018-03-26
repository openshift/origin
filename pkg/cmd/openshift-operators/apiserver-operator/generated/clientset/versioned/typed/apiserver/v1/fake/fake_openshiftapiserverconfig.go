package fake

import (
	apiserver_v1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apiserver-operator/apis/apiserver/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeOpenShiftAPIServerConfigs implements OpenShiftAPIServerConfigInterface
type FakeOpenShiftAPIServerConfigs struct {
	Fake *FakeApiserverV1
}

var openshiftapiserverconfigsResource = schema.GroupVersionResource{Group: "apiserver", Version: "v1", Resource: "openshiftapiserverconfigs"}

var openshiftapiserverconfigsKind = schema.GroupVersionKind{Group: "apiserver", Version: "v1", Kind: "OpenShiftAPIServerConfig"}

// Get takes name of the openShiftAPIServerConfig, and returns the corresponding openShiftAPIServerConfig object, and an error if there is any.
func (c *FakeOpenShiftAPIServerConfigs) Get(name string, options v1.GetOptions) (result *apiserver_v1.OpenShiftAPIServerConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(openshiftapiserverconfigsResource, name), &apiserver_v1.OpenShiftAPIServerConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*apiserver_v1.OpenShiftAPIServerConfig), err
}

// List takes label and field selectors, and returns the list of OpenShiftAPIServerConfigs that match those selectors.
func (c *FakeOpenShiftAPIServerConfigs) List(opts v1.ListOptions) (result *apiserver_v1.OpenShiftAPIServerConfigList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(openshiftapiserverconfigsResource, openshiftapiserverconfigsKind, opts), &apiserver_v1.OpenShiftAPIServerConfigList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &apiserver_v1.OpenShiftAPIServerConfigList{}
	for _, item := range obj.(*apiserver_v1.OpenShiftAPIServerConfigList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested openShiftAPIServerConfigs.
func (c *FakeOpenShiftAPIServerConfigs) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(openshiftapiserverconfigsResource, opts))
}

// Create takes the representation of a openShiftAPIServerConfig and creates it.  Returns the server's representation of the openShiftAPIServerConfig, and an error, if there is any.
func (c *FakeOpenShiftAPIServerConfigs) Create(openShiftAPIServerConfig *apiserver_v1.OpenShiftAPIServerConfig) (result *apiserver_v1.OpenShiftAPIServerConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(openshiftapiserverconfigsResource, openShiftAPIServerConfig), &apiserver_v1.OpenShiftAPIServerConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*apiserver_v1.OpenShiftAPIServerConfig), err
}

// Update takes the representation of a openShiftAPIServerConfig and updates it. Returns the server's representation of the openShiftAPIServerConfig, and an error, if there is any.
func (c *FakeOpenShiftAPIServerConfigs) Update(openShiftAPIServerConfig *apiserver_v1.OpenShiftAPIServerConfig) (result *apiserver_v1.OpenShiftAPIServerConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(openshiftapiserverconfigsResource, openShiftAPIServerConfig), &apiserver_v1.OpenShiftAPIServerConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*apiserver_v1.OpenShiftAPIServerConfig), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeOpenShiftAPIServerConfigs) UpdateStatus(openShiftAPIServerConfig *apiserver_v1.OpenShiftAPIServerConfig) (*apiserver_v1.OpenShiftAPIServerConfig, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(openshiftapiserverconfigsResource, "status", openShiftAPIServerConfig), &apiserver_v1.OpenShiftAPIServerConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*apiserver_v1.OpenShiftAPIServerConfig), err
}

// Delete takes name of the openShiftAPIServerConfig and deletes it. Returns an error if one occurs.
func (c *FakeOpenShiftAPIServerConfigs) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(openshiftapiserverconfigsResource, name), &apiserver_v1.OpenShiftAPIServerConfig{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeOpenShiftAPIServerConfigs) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(openshiftapiserverconfigsResource, listOptions)

	_, err := c.Fake.Invokes(action, &apiserver_v1.OpenShiftAPIServerConfigList{})
	return err
}

// Patch applies the patch and returns the patched openShiftAPIServerConfig.
func (c *FakeOpenShiftAPIServerConfigs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *apiserver_v1.OpenShiftAPIServerConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(openshiftapiserverconfigsResource, name, data, subresources...), &apiserver_v1.OpenShiftAPIServerConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*apiserver_v1.OpenShiftAPIServerConfig), err
}

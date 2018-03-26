package fake

import (
	orchestration_v1 "github.com/openshift/origin/pkg/cmd/openshift-operators/orchestration-operator/apis/orchestration/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeOpenShiftOrchestrationConfigs implements OpenShiftOrchestrationConfigInterface
type FakeOpenShiftOrchestrationConfigs struct {
	Fake *FakeOrchestrationV1
}

var openshiftorchestrationconfigsResource = schema.GroupVersionResource{Group: "orchestration", Version: "v1", Resource: "openshiftorchestrationconfigs"}

var openshiftorchestrationconfigsKind = schema.GroupVersionKind{Group: "orchestration", Version: "v1", Kind: "OpenShiftOrchestrationConfig"}

// Get takes name of the openShiftOrchestrationConfig, and returns the corresponding openShiftOrchestrationConfig object, and an error if there is any.
func (c *FakeOpenShiftOrchestrationConfigs) Get(name string, options v1.GetOptions) (result *orchestration_v1.OpenShiftOrchestrationConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(openshiftorchestrationconfigsResource, name), &orchestration_v1.OpenShiftOrchestrationConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*orchestration_v1.OpenShiftOrchestrationConfig), err
}

// List takes label and field selectors, and returns the list of OpenShiftOrchestrationConfigs that match those selectors.
func (c *FakeOpenShiftOrchestrationConfigs) List(opts v1.ListOptions) (result *orchestration_v1.OpenShiftOrchestrationConfigList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(openshiftorchestrationconfigsResource, openshiftorchestrationconfigsKind, opts), &orchestration_v1.OpenShiftOrchestrationConfigList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &orchestration_v1.OpenShiftOrchestrationConfigList{}
	for _, item := range obj.(*orchestration_v1.OpenShiftOrchestrationConfigList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested openShiftOrchestrationConfigs.
func (c *FakeOpenShiftOrchestrationConfigs) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(openshiftorchestrationconfigsResource, opts))
}

// Create takes the representation of a openShiftOrchestrationConfig and creates it.  Returns the server's representation of the openShiftOrchestrationConfig, and an error, if there is any.
func (c *FakeOpenShiftOrchestrationConfigs) Create(openShiftOrchestrationConfig *orchestration_v1.OpenShiftOrchestrationConfig) (result *orchestration_v1.OpenShiftOrchestrationConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(openshiftorchestrationconfigsResource, openShiftOrchestrationConfig), &orchestration_v1.OpenShiftOrchestrationConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*orchestration_v1.OpenShiftOrchestrationConfig), err
}

// Update takes the representation of a openShiftOrchestrationConfig and updates it. Returns the server's representation of the openShiftOrchestrationConfig, and an error, if there is any.
func (c *FakeOpenShiftOrchestrationConfigs) Update(openShiftOrchestrationConfig *orchestration_v1.OpenShiftOrchestrationConfig) (result *orchestration_v1.OpenShiftOrchestrationConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(openshiftorchestrationconfigsResource, openShiftOrchestrationConfig), &orchestration_v1.OpenShiftOrchestrationConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*orchestration_v1.OpenShiftOrchestrationConfig), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeOpenShiftOrchestrationConfigs) UpdateStatus(openShiftOrchestrationConfig *orchestration_v1.OpenShiftOrchestrationConfig) (*orchestration_v1.OpenShiftOrchestrationConfig, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(openshiftorchestrationconfigsResource, "status", openShiftOrchestrationConfig), &orchestration_v1.OpenShiftOrchestrationConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*orchestration_v1.OpenShiftOrchestrationConfig), err
}

// Delete takes name of the openShiftOrchestrationConfig and deletes it. Returns an error if one occurs.
func (c *FakeOpenShiftOrchestrationConfigs) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(openshiftorchestrationconfigsResource, name), &orchestration_v1.OpenShiftOrchestrationConfig{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeOpenShiftOrchestrationConfigs) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(openshiftorchestrationconfigsResource, listOptions)

	_, err := c.Fake.Invokes(action, &orchestration_v1.OpenShiftOrchestrationConfigList{})
	return err
}

// Patch applies the patch and returns the patched openShiftOrchestrationConfig.
func (c *FakeOpenShiftOrchestrationConfigs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *orchestration_v1.OpenShiftOrchestrationConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(openshiftorchestrationconfigsResource, name, data, subresources...), &orchestration_v1.OpenShiftOrchestrationConfig{})
	if obj == nil {
		return nil, err
	}
	return obj.(*orchestration_v1.OpenShiftOrchestrationConfig), err
}

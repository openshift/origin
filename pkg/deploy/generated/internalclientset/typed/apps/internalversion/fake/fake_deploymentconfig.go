package fake

import (
	api "github.com/openshift/origin/pkg/deploy/api"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeDeploymentConfigs implements DeploymentConfigInterface
type FakeDeploymentConfigs struct {
	Fake *FakeApps
	ns   string
}

var deploymentconfigsResource = schema.GroupVersionResource{Group: "apps.openshift.io", Version: "", Resource: "deploymentconfigs"}

func (c *FakeDeploymentConfigs) Create(deploymentConfig *api.DeploymentConfig) (result *api.DeploymentConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(deploymentconfigsResource, c.ns, deploymentConfig), &api.DeploymentConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) Update(deploymentConfig *api.DeploymentConfig) (result *api.DeploymentConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(deploymentconfigsResource, c.ns, deploymentConfig), &api.DeploymentConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) UpdateStatus(deploymentConfig *api.DeploymentConfig) (*api.DeploymentConfig, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(deploymentconfigsResource, "status", c.ns, deploymentConfig), &api.DeploymentConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(deploymentconfigsResource, c.ns, name), &api.DeploymentConfig{})

	return err
}

func (c *FakeDeploymentConfigs) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(deploymentconfigsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.DeploymentConfigList{})
	return err
}

func (c *FakeDeploymentConfigs) Get(name string, options v1.GetOptions) (result *api.DeploymentConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(deploymentconfigsResource, c.ns, name), &api.DeploymentConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) List(opts v1.ListOptions) (result *api.DeploymentConfigList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(deploymentconfigsResource, c.ns, opts), &api.DeploymentConfigList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.DeploymentConfigList{}
	for _, item := range obj.(*api.DeploymentConfigList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested deploymentConfigs.
func (c *FakeDeploymentConfigs) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(deploymentconfigsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched deploymentConfig.
func (c *FakeDeploymentConfigs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.DeploymentConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(deploymentconfigsResource, c.ns, name, data, subresources...), &api.DeploymentConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.DeploymentConfig), err
}

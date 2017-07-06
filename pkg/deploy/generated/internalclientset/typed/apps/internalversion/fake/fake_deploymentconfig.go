package fake

import (
	apps "github.com/openshift/origin/pkg/deploy/apis/apps"
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

var deploymentconfigsKind = schema.GroupVersionKind{Group: "apps.openshift.io", Version: "", Kind: "DeploymentConfig"}

func (c *FakeDeploymentConfigs) Create(deploymentConfig *apps.DeploymentConfig) (result *apps.DeploymentConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(deploymentconfigsResource, c.ns, deploymentConfig), &apps.DeploymentConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*apps.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) Update(deploymentConfig *apps.DeploymentConfig) (result *apps.DeploymentConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(deploymentconfigsResource, c.ns, deploymentConfig), &apps.DeploymentConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*apps.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) UpdateStatus(deploymentConfig *apps.DeploymentConfig) (*apps.DeploymentConfig, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(deploymentconfigsResource, "status", c.ns, deploymentConfig), &apps.DeploymentConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*apps.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(deploymentconfigsResource, c.ns, name), &apps.DeploymentConfig{})

	return err
}

func (c *FakeDeploymentConfigs) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(deploymentconfigsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &apps.DeploymentConfigList{})
	return err
}

func (c *FakeDeploymentConfigs) Get(name string, options v1.GetOptions) (result *apps.DeploymentConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(deploymentconfigsResource, c.ns, name), &apps.DeploymentConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*apps.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) List(opts v1.ListOptions) (result *apps.DeploymentConfigList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(deploymentconfigsResource, deploymentconfigsKind, c.ns, opts), &apps.DeploymentConfigList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &apps.DeploymentConfigList{}
	for _, item := range obj.(*apps.DeploymentConfigList).Items {
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
func (c *FakeDeploymentConfigs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *apps.DeploymentConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(deploymentconfigsResource, c.ns, name, data, subresources...), &apps.DeploymentConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*apps.DeploymentConfig), err
}

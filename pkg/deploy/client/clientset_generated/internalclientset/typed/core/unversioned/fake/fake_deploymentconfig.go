package fake

import (
	api "github.com/openshift/origin/pkg/deploy/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeDeploymentConfigs implements DeploymentConfigInterface
type FakeDeploymentConfigs struct {
	Fake *FakeCore
	ns   string
}

var deploymentconfigsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "deploymentconfigs"}

func (c *FakeDeploymentConfigs) Create(deploymentConfig *api.DeploymentConfig) (result *api.DeploymentConfig, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(deploymentconfigsResource, c.ns, deploymentConfig), &api.DeploymentConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) Update(deploymentConfig *api.DeploymentConfig) (result *api.DeploymentConfig, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(deploymentconfigsResource, c.ns, deploymentConfig), &api.DeploymentConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) Delete(name string, options *pkg_api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(deploymentconfigsResource, c.ns, name), &api.DeploymentConfig{})

	return err
}

func (c *FakeDeploymentConfigs) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	action := core.NewDeleteCollectionAction(deploymentconfigsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.DeploymentConfigList{})
	return err
}

func (c *FakeDeploymentConfigs) Get(name string) (result *api.DeploymentConfig, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(deploymentconfigsResource, c.ns, name), &api.DeploymentConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) List(opts pkg_api.ListOptions) (result *api.DeploymentConfigList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(deploymentconfigsResource, c.ns, opts), &api.DeploymentConfigList{})

	if obj == nil {
		return nil, err
	}

	label := opts.LabelSelector
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
func (c *FakeDeploymentConfigs) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(deploymentconfigsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched deploymentConfig.
func (c *FakeDeploymentConfigs) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.DeploymentConfig, err error) {
	obj, err := c.Fake.
		Invokes(core.NewPatchSubresourceAction(deploymentconfigsResource, c.ns, name, data, subresources...), &api.DeploymentConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.DeploymentConfig), err
}

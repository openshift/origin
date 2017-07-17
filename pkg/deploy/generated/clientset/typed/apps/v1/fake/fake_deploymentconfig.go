package fake

import (
	v1 "github.com/openshift/origin/pkg/deploy/apis/apps/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeDeploymentConfigs implements DeploymentConfigInterface
type FakeDeploymentConfigs struct {
	Fake *FakeAppsV1
	ns   string
}

var deploymentconfigsResource = schema.GroupVersionResource{Group: "apps.openshift.io", Version: "v1", Resource: "deploymentconfigs"}

var deploymentconfigsKind = schema.GroupVersionKind{Group: "apps.openshift.io", Version: "v1", Kind: "DeploymentConfig"}

func (c *FakeDeploymentConfigs) Create(deploymentConfig *v1.DeploymentConfig) (result *v1.DeploymentConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(deploymentconfigsResource, c.ns, deploymentConfig), &v1.DeploymentConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) Update(deploymentConfig *v1.DeploymentConfig) (result *v1.DeploymentConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(deploymentconfigsResource, c.ns, deploymentConfig), &v1.DeploymentConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) UpdateStatus(deploymentConfig *v1.DeploymentConfig) (*v1.DeploymentConfig, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(deploymentconfigsResource, "status", c.ns, deploymentConfig), &v1.DeploymentConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(deploymentconfigsResource, c.ns, name), &v1.DeploymentConfig{})

	return err
}

func (c *FakeDeploymentConfigs) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(deploymentconfigsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.DeploymentConfigList{})
	return err
}

func (c *FakeDeploymentConfigs) Get(name string, options meta_v1.GetOptions) (result *v1.DeploymentConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(deploymentconfigsResource, c.ns, name), &v1.DeploymentConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) List(opts meta_v1.ListOptions) (result *v1.DeploymentConfigList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(deploymentconfigsResource, deploymentconfigsKind, c.ns, opts), &v1.DeploymentConfigList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.DeploymentConfigList{}
	for _, item := range obj.(*v1.DeploymentConfigList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested deploymentConfigs.
func (c *FakeDeploymentConfigs) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(deploymentconfigsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched deploymentConfig.
func (c *FakeDeploymentConfigs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.DeploymentConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(deploymentconfigsResource, c.ns, name, data, subresources...), &v1.DeploymentConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.DeploymentConfig), err
}

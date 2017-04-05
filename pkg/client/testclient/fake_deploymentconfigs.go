package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/watch"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// FakeDeploymentConfigs implements DeploymentConfigInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeDeploymentConfigs struct {
	Fake      *Fake
	Namespace string
}

var deploymentConfigsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "deploymentconfigs"}

func (c *FakeDeploymentConfigs) Get(name string) (*deployapi.DeploymentConfig, error) {
	obj, err := c.Fake.Invokes(core.NewGetAction(deploymentConfigsResource, c.Namespace, name), &deployapi.DeploymentConfig{})
	if obj == nil {
		return nil, err
	}

	return obj.(*deployapi.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) List(opts kapi.ListOptions) (*deployapi.DeploymentConfigList, error) {
	obj, err := c.Fake.Invokes(core.NewListAction(deploymentConfigsResource, c.Namespace, opts), &deployapi.DeploymentConfigList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*deployapi.DeploymentConfigList), err
}

func (c *FakeDeploymentConfigs) Create(inObj *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	obj, err := c.Fake.Invokes(core.NewCreateAction(deploymentConfigsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*deployapi.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) Update(inObj *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	obj, err := c.Fake.Invokes(core.NewUpdateAction(deploymentConfigsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*deployapi.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) Patch(name string, pt kapi.PatchType, data []byte, subresources ...string) (*deployapi.DeploymentConfig, error) {
	return nil, nil
}

func (c *FakeDeploymentConfigs) Delete(name string) error {
	_, err := c.Fake.Invokes(core.NewDeleteAction(deploymentConfigsResource, c.Namespace, name), &deployapi.DeploymentConfig{})
	return err
}

func (c *FakeDeploymentConfigs) Watch(opts kapi.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(core.NewWatchAction(deploymentConfigsResource, c.Namespace, opts))
}

func (c *FakeDeploymentConfigs) Generate(name string) (*deployapi.DeploymentConfig, error) {
	obj, err := c.Fake.Invokes(core.NewGetAction(deployapi.LegacySchemeGroupVersion.WithResource("generatedeploymentconfigs"), c.Namespace, name), &deployapi.DeploymentConfig{})
	if obj == nil {
		return nil, err
	}

	return obj.(*deployapi.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) Rollback(inObj *deployapi.DeploymentConfigRollback) (result *deployapi.DeploymentConfig, err error) {
	obj, err := c.Fake.Invokes(core.NewCreateAction(deployapi.LegacySchemeGroupVersion.WithResource("deploymentconfigs/rollback"), c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*deployapi.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) RollbackDeprecated(inObj *deployapi.DeploymentConfigRollback) (result *deployapi.DeploymentConfig, err error) {
	obj, err := c.Fake.Invokes(core.NewCreateAction(deployapi.LegacySchemeGroupVersion.WithResource("deploymentconfigrollbacks"), c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*deployapi.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) GetScale(name string) (*extensions.Scale, error) {
	obj, err := c.Fake.Invokes(core.NewGetAction(deployapi.LegacySchemeGroupVersion.WithResource("deploymentconfigs/scale"), c.Namespace, name), &extensions.Scale{})
	if obj == nil {
		return nil, err
	}

	return obj.(*extensions.Scale), err
}

func (c *FakeDeploymentConfigs) UpdateScale(inObj *extensions.Scale) (*extensions.Scale, error) {
	obj, err := c.Fake.Invokes(core.NewUpdateAction(deployapi.LegacySchemeGroupVersion.WithResource("deploymentconfigs/scale"), c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*extensions.Scale), err
}

func (c *FakeDeploymentConfigs) UpdateStatus(inObj *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	obj, err := c.Fake.Invokes(core.NewUpdateAction(deployapi.LegacySchemeGroupVersion.WithResource("deploymentconfigs/status"), c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*deployapi.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) Instantiate(inObj *deployapi.DeploymentRequest) (*deployapi.DeploymentConfig, error) {
	deployment := &deployapi.DeploymentConfig{ObjectMeta: kapi.ObjectMeta{Name: inObj.Name}}
	obj, err := c.Fake.Invokes(core.NewUpdateAction(deployapi.LegacySchemeGroupVersion.WithResource("deploymentconfigs/instantiate"), c.Namespace, deployment), deployment)
	if obj == nil {
		return nil, err
	}

	return obj.(*deployapi.DeploymentConfig), err
}

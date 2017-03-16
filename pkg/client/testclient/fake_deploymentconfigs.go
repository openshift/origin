package testclient

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/apis/extensions"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// FakeDeploymentConfigs implements DeploymentConfigInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeDeploymentConfigs struct {
	Fake      *Fake
	Namespace string
}

var deploymentConfigsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "deploymentconfigs"}

func (c *FakeDeploymentConfigs) Get(name string, options metav1.GetOptions) (*deployapi.DeploymentConfig, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewGetAction(deploymentConfigsResource, c.Namespace, name), &deployapi.DeploymentConfig{})
	if obj == nil {
		return nil, err
	}

	return obj.(*deployapi.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) List(opts metainternal.ListOptions) (*deployapi.DeploymentConfigList, error) {
	optsv1 := metav1.ListOptions{}
	err := metainternal.Convert_internalversion_ListOptions_To_v1_ListOptions(&opts, &optsv1, nil)
	if err != nil {
		return nil, err
	}
	obj, err := c.Fake.Invokes(clientgotesting.NewListAction(deploymentConfigsResource, c.Namespace, optsv1), &deployapi.DeploymentConfigList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*deployapi.DeploymentConfigList), err
}

func (c *FakeDeploymentConfigs) Create(inObj *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewCreateAction(deploymentConfigsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*deployapi.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) Update(inObj *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewUpdateAction(deploymentConfigsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*deployapi.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewDeleteAction(deploymentConfigsResource, c.Namespace, name), &deployapi.DeploymentConfig{})
	return err
}

func (c *FakeDeploymentConfigs) Watch(opts metainternal.ListOptions) (watch.Interface, error) {
	optsv1 := metav1.ListOptions{}
	err := metainternal.Convert_internalversion_ListOptions_To_v1_ListOptions(&opts, &optsv1, nil)
	if err != nil {
		return nil, err
	}
	return c.Fake.InvokesWatch(clientgotesting.NewWatchAction(deploymentConfigsResource, c.Namespace, optsv1))
}

func (c *FakeDeploymentConfigs) Generate(name string) (*deployapi.DeploymentConfig, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewGetAction(deployapi.LegacySchemeGroupVersion.WithResource("generatedeploymentconfigs"), c.Namespace, name), &deployapi.DeploymentConfig{})
	if obj == nil {
		return nil, err
	}

	return obj.(*deployapi.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) Rollback(inObj *deployapi.DeploymentConfigRollback) (result *deployapi.DeploymentConfig, err error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewCreateAction(deployapi.LegacySchemeGroupVersion.WithResource("deploymentconfigs/rollback"), c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*deployapi.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) RollbackDeprecated(inObj *deployapi.DeploymentConfigRollback) (result *deployapi.DeploymentConfig, err error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewCreateAction(deployapi.LegacySchemeGroupVersion.WithResource("deploymentconfigrollbacks"), c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*deployapi.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) GetScale(name string) (*extensions.Scale, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewGetAction(deployapi.LegacySchemeGroupVersion.WithResource("deploymentconfigs/scale"), c.Namespace, name), &extensions.Scale{})
	if obj == nil {
		return nil, err
	}

	return obj.(*extensions.Scale), err
}

func (c *FakeDeploymentConfigs) UpdateScale(inObj *extensions.Scale) (*extensions.Scale, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewUpdateAction(deployapi.LegacySchemeGroupVersion.WithResource("deploymentconfigs/scale"), c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*extensions.Scale), err
}

func (c *FakeDeploymentConfigs) UpdateStatus(inObj *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewUpdateAction(deployapi.LegacySchemeGroupVersion.WithResource("deploymentconfigs/status"), c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*deployapi.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) Instantiate(inObj *deployapi.DeploymentRequest) (*deployapi.DeploymentConfig, error) {
	deployment := &deployapi.DeploymentConfig{ObjectMeta: metav1.ObjectMeta{Name: inObj.Name}}
	obj, err := c.Fake.Invokes(clientgotesting.NewUpdateAction(deployapi.LegacySchemeGroupVersion.WithResource("deploymentconfigs/instantiate"), c.Namespace, deployment), deployment)
	if obj == nil {
		return nil, err
	}

	return obj.(*deployapi.DeploymentConfig), err
}

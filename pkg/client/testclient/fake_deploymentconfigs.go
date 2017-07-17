package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"
	extensionsv1beta1 "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"

	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
)

// FakeDeploymentConfigs implements DeploymentConfigInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeDeploymentConfigs struct {
	Fake      *Fake
	Namespace string
}

var deploymentConfigsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "deploymentconfigs"}
var deploymentConfigsKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "DeploymentConfig"}

func (c *FakeDeploymentConfigs) Get(name string, options metav1.GetOptions) (*deployapi.DeploymentConfig, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewGetAction(deploymentConfigsResource, c.Namespace, name), &deployapi.DeploymentConfig{})
	if obj == nil {
		return nil, err
	}

	return obj.(*deployapi.DeploymentConfig), err
}

func (c *FakeDeploymentConfigs) List(opts metav1.ListOptions) (*deployapi.DeploymentConfigList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewListAction(deploymentConfigsResource, deploymentConfigsKind, c.Namespace, opts), &deployapi.DeploymentConfigList{})
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

func (c *FakeDeploymentConfigs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (*deployapi.DeploymentConfig, error) {
	return nil, nil
}

func (c *FakeDeploymentConfigs) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewDeleteAction(deploymentConfigsResource, c.Namespace, name), &deployapi.DeploymentConfig{})
	return err
}

func (c *FakeDeploymentConfigs) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(clientgotesting.NewWatchAction(deploymentConfigsResource, c.Namespace, opts))
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

func (c *FakeDeploymentConfigs) GetScale(name string) (*extensionsv1beta1.Scale, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewGetAction(deployapi.LegacySchemeGroupVersion.WithResource("deploymentconfigs/scale"), c.Namespace, name), &extensionsv1beta1.Scale{})
	if obj == nil {
		return nil, err
	}

	return obj.(*extensionsv1beta1.Scale), err
}

func (c *FakeDeploymentConfigs) UpdateScale(inObj *extensionsv1beta1.Scale) (*extensionsv1beta1.Scale, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewUpdateAction(deployapi.LegacySchemeGroupVersion.WithResource("deploymentconfigs/scale"), c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*extensionsv1beta1.Scale), err
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

package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// DeploymentsNamespacer has methods to work with Deployment resources in a namespace
type DeploymentsNamespacer interface {
	Deployments(namespace string) DeploymentInterface
}

// DeploymentInterface contains methods for working with Deployments
type DeploymentInterface interface {
	List(label labels.Selector, field fields.Selector) (*deployapi.DeploymentList, error)
	Get(name string) (*deployapi.Deployment, error)
	Create(deployment *deployapi.Deployment) (*deployapi.Deployment, error)
	Update(deployment *deployapi.Deployment) (*deployapi.Deployment, error)
	Delete(name string) error
	Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error)
}

// deployments implements DeploymentsNamespacer interface
type deployments struct {
	r  *Client
	ns string
}

// newDeployments returns a deployments
func newDeployments(c *Client, namespace string) *deployments {
	return &deployments{
		r:  c,
		ns: namespace,
	}
}

// List takes a label and field selector, and returns the list of deployments that match that selectors
func (c *deployments) List(label labels.Selector, field fields.Selector) (result *deployapi.DeploymentList, err error) {
	result = &deployapi.DeploymentList{}
	err = c.r.Get().
		Namespace(c.ns).
		Resource("deployments").
		LabelsSelectorParam(label).
		FieldsSelectorParam(field).
		Do().
		Into(result)
	return
}

// Get returns information about a particular deployment
func (c *deployments) Get(name string) (result *deployapi.Deployment, err error) {
	result = &deployapi.Deployment{}
	err = c.r.Get().Namespace(c.ns).Resource("deployments").Name(name).Do().Into(result)
	return
}

// Create creates a new deployment
func (c *deployments) Create(deployment *deployapi.Deployment) (result *deployapi.Deployment, err error) {
	result = &deployapi.Deployment{}
	err = c.r.Post().Namespace(c.ns).Resource("deployments").Body(deployment).Do().Into(result)
	return
}

// Update updates an existing deployment
func (c *deployments) Update(deployment *deployapi.Deployment) (result *deployapi.Deployment, err error) {
	result = &deployapi.Deployment{}
	err = c.r.Put().Namespace(c.ns).Resource("deployments").Name(deployment.Name).Body(deployment).Do().Into(result)
	return
}

// Delete deletes an existing replication deployment.
func (c *deployments) Delete(name string) error {
	return c.r.Delete().Namespace(c.ns).Resource("deployments").Name(name).Do().Error()
}

// Watch returns a watch.Interface that watches the requested deployments.
func (c *deployments) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return c.r.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("deployments").
		Param("resourceVersion", resourceVersion).
		LabelsSelectorParam(label).
		FieldsSelectorParam(field).
		Watch()
}

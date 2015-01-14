package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	secretapi "github.com/openshift/origin/pkg/secret/api"
)

// SecretsNamespacer has methods to work with Secret resources in a namespace
type SecretsNamespacer interface {
	Secrets(namespace string) SecretInterface
}

// SecretInterface exposes methods on Secret resources.
type SecretInterface interface {
	List(label, field labels.Selector) (*secretapi.SecretList, error)
	Get(name string) (*secretapi.Secret, error)
	Create(secret *secretapi.Secret) (*secretapi.Secret, error)
	Update(secret *secretapi.Secret) (*secretapi.Secret, error)
	Delete(name string) error
	Watch(label, field labels.Selector, resourceVersion string) (watch.Interface, error)
}

// secrets implements SecretsNamespacer interface
type secrets struct {
	r  *Client
	ns string
}

// newSecrets returns a secrets
func newSecrets(c *Client, namespace string) *secrets {
	return &secrets{
		r:  c,
		ns: namespace,
	}
}

// List returns a list of secrets that match the label and field selectors.
func (c *secrets) List(label, field labels.Selector) (result *secretapi.SecretList, err error) {
	result = &secretapi.SecretList{}
	err = c.r.Get().
		Namespace(c.ns).
		Path("secrets").
		SelectorParam("labels", label).
		SelectorParam("fields", field).
		Do().
		Into(result)
	return
}

// Get returns information about a particular secret and error if one occurs.
func (c *secrets) Get(name string) (result *secretapi.Secret, err error) {
	result = &secretapi.Secret{}
	err = c.r.Get().Namespace(c.ns).Path("secrets").Path(name).Do().Into(result)
	return
}

// Create creates new secret. Returns the server's representation of the secret and error if one occurs.
func (c *secrets) Create(secret *secretapi.Secret) (result *secretapi.Secret, err error) {
	result = &secretapi.Secret{}
	err = c.r.Post().Namespace(c.ns).Path("secrets").Body(secret).Do().Into(result)
	return
}

// Update updates the secret on server. Returns the server's representation of the secret and error if one occurs.
func (c *secrets) Update(secret *secretapi.Secret) (result *secretapi.Secret, err error) {
	result = &secretapi.Secret{}
	err = c.r.Put().Namespace(c.ns).Path("secrets").Path(secret.Name).Body(secret).Do().Into(result)
	return
}

// Delete deletes a secret, returns error if one occurs.
func (c *secrets) Delete(name string) (err error) {
	err = c.r.Delete().Namespace(c.ns).Path("secrets").Path(name).Do().Error()
	return
}

// Watch returns a watch.Interface that watches the requested secrets
func (c *secrets) Watch(label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	return c.r.Get().
		Path("watch").
		Namespace(c.ns).
		Path("secrets").
		Param("resourceVersion", resourceVersion).
		SelectorParam("labels", label).
		SelectorParam("fields", field).
		Watch()
}

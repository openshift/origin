package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	secretapi "github.com/openshift/origin/pkg/secret/api"
)

// FakeSecrets implements SecretInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeSecrets struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeSecrets) List(label, field labels.Selector) (*secretapi.SecretList, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "list-secrets"})
	return &secretapi.SecretList{}, nil
}

func (c *FakeSecrets) Get(name string) (*secretapi.Secret, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "get-secret"})
	return &secretapi.Secret{}, nil
}

func (c *FakeSecrets) Create(secret *secretapi.Secret) (*secretapi.Secret, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "create-secret", Value: secret})
	return &secretapi.Secret{}, nil
}

func (c *FakeSecrets) Update(secret *secretapi.Secret) (*secretapi.Secret, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "update-secret"})
	return &secretapi.Secret{}, nil
}

func (c *FakeSecrets) Delete(name string) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-secret", Value: name})
	return nil
}

func (c *FakeSecrets) Watch(label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "watch-secrets"})
	return nil, nil
}

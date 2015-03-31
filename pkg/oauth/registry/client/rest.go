package client

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/api/validation"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	registry Registry
}

// NewStorage returns a new REST.
func NewREST(registry Registry) *REST {
	return &REST{registry}
}

// New returns a new Client for use with Create and Update.
func (s *REST) New() runtime.Object {
	return &api.OAuthClient{}
}

func (*REST) NewList() runtime.Object {
	return &api.OAuthClient{}
}

// Get retrieves an Client by id.
func (s *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	client, err := s.registry.GetClient(id)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// List retrieves a list of Clients that match label.
func (s *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	clients, err := s.registry.ListClients(label)
	if err != nil {
		return nil, err
	}

	return clients, nil
}

// Create registers the given Client.
func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	client, ok := obj.(*api.OAuthClient)
	if !ok {
		return nil, fmt.Errorf("not an client: %#v", obj)
	}

	kapi.FillObjectMetaSystemFields(ctx, &client.ObjectMeta)

	if errs := validation.ValidateClient(client); len(errs) > 0 {
		return nil, kerrors.NewInvalid("oauthClient", client.Name, errs)
	}

	if err := s.registry.CreateClient(client); err != nil {
		return nil, err
	}
	return s.Get(ctx, client.Name)
}

// Delete asynchronously deletes an Client specified by its id.
func (s *REST) Delete(ctx kapi.Context, id string) (runtime.Object, error) {
	return &kapi.Status{Status: kapi.StatusSuccess}, s.registry.DeleteClient(id)
}

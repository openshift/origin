package client

import (
	"fmt"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/oauth/api"
	//"github.com/openshift/origin/pkg/oauth/api/validation"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	registry Registry
}

// NewStorage returns a new REST.
func NewREST(registry Registry) apiserver.RESTStorage {
	return &REST{registry}
}

// New returns a new Client for use with Create and Update.
func (s *REST) New() interface{} {
	return &api.Client{}
}

// Get retrieves an Client by id.
func (s *REST) Get(id string) (interface{}, error) {
	client, err := s.registry.GetClient(id)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// List retrieves a list of Clients that match selector.
func (s *REST) List(selector labels.Selector) (interface{}, error) {
	clients, err := s.registry.ListClients(selector)
	if err != nil {
		return nil, err
	}

	return clients, nil
}

// Create registers the given Client.
func (s *REST) Create(obj interface{}) (<-chan interface{}, error) {
	client, ok := obj.(*api.Client)
	if !ok {
		return nil, fmt.Errorf("not an client: %#v", obj)
	}

	client.CreationTimestamp = util.Now()

	// if errs := validation.ValidateClient(client); len(errs) > 0 {
	// 	return nil, errors.NewInvalid("client", client.Name, errs)
	// }

	return apiserver.MakeAsync(func() (interface{}, error) {
		if err := s.registry.CreateClient(client); err != nil {
			return nil, err
		}
		return s.Get(client.Name)
	}), nil
}

// Update is not supported for Clients, as they are immutable.
func (s *REST) Update(obj interface{}) (<-chan interface{}, error) {
	return nil, fmt.Errorf("Clients may not be changed.")
}

// Delete asynchronously deletes an Client specified by its id.
func (s *REST) Delete(id string) (<-chan interface{}, error) {
	return apiserver.MakeAsync(func() (interface{}, error) {
		return &kubeapi.Status{Status: kubeapi.StatusSuccess}, s.registry.DeleteClient(id)
	}), nil
}

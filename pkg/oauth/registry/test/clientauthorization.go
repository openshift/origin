package test

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/oauth/api"
)

type ClientAuthorizationRegistry struct {
	Err                            error
	ClientAuthorizations           *api.ClientAuthorizationList
	ClientAuthorization            *api.ClientAuthorization
	CreatedAuthorization           *api.ClientAuthorization
	UpdatedAuthorization           *api.ClientAuthorization
	DeletedClientAuthorizationName string
}

func (r *ClientAuthorizationRegistry) ClientAuthorizationName(userName, clientName string) string {
	return fmt.Sprintf("%s:%s", userName, clientName)
}

func (r *ClientAuthorizationRegistry) ListClientAuthorizations(label, field labels.Selector) (*api.ClientAuthorizationList, error) {
	return r.ClientAuthorizations, r.Err
}

func (r *ClientAuthorizationRegistry) GetClientAuthorization(name string) (*api.ClientAuthorization, error) {
	return r.ClientAuthorization, r.Err
}

func (r *ClientAuthorizationRegistry) CreateClientAuthorization(grant *api.ClientAuthorization) error {
	r.CreatedAuthorization = grant
	return r.Err
}

func (r *ClientAuthorizationRegistry) UpdateClientAuthorization(grant *api.ClientAuthorization) error {
	r.UpdatedAuthorization = grant
	return r.Err
}

func (r *ClientAuthorizationRegistry) DeleteClientAuthorization(name string) error {
	r.DeletedClientAuthorizationName = name
	return r.Err
}

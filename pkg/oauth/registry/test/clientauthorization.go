package test

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/oauth/api"
)

type ClientAuthorizationRegistry struct {
	Err                            error
	ClientAuthorizations           *api.OAuthClientAuthorizationList
	ClientAuthorization            *api.OAuthClientAuthorization
	CreatedAuthorization           *api.OAuthClientAuthorization
	UpdatedAuthorization           *api.OAuthClientAuthorization
	DeletedClientAuthorizationName string
}

func (r *ClientAuthorizationRegistry) ClientAuthorizationName(userName, clientName string) string {
	return fmt.Sprintf("%s:%s", userName, clientName)
}

func (r *ClientAuthorizationRegistry) ListClientAuthorizations(label labels.Selector, field fields.Selector) (*api.OAuthClientAuthorizationList, error) {
	return r.ClientAuthorizations, r.Err
}

func (r *ClientAuthorizationRegistry) GetClientAuthorization(name string) (*api.OAuthClientAuthorization, error) {
	return r.ClientAuthorization, r.Err
}

func (r *ClientAuthorizationRegistry) CreateClientAuthorization(grant *api.OAuthClientAuthorization) error {
	r.CreatedAuthorization = grant
	return r.Err
}

func (r *ClientAuthorizationRegistry) UpdateClientAuthorization(grant *api.OAuthClientAuthorization) error {
	r.UpdatedAuthorization = grant
	return r.Err
}

func (r *ClientAuthorizationRegistry) DeleteClientAuthorization(name string) error {
	r.DeletedClientAuthorizationName = name
	return r.Err
}

package test

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/oauth/api"
)

type ClientAuthorizationRegistry struct {
	GetErr               error
	ClientAuthorizations *api.OAuthClientAuthorizationList
	ClientAuthorization  *api.OAuthClientAuthorization

	CreateErr            error
	CreatedAuthorization *api.OAuthClientAuthorization

	UpdateErr            error
	UpdatedAuthorization *api.OAuthClientAuthorization

	DeleteErr                      error
	DeletedClientAuthorizationName string
}

func (r *ClientAuthorizationRegistry) ClientAuthorizationName(userName, clientName string) string {
	return fmt.Sprintf("%s:%s", userName, clientName)
}

func (r *ClientAuthorizationRegistry) ListClientAuthorizations(ctx kapi.Context, options *kapi.ListOptions) (*api.OAuthClientAuthorizationList, error) {
	return r.ClientAuthorizations, r.GetErr
}

func (r *ClientAuthorizationRegistry) GetClientAuthorization(ctx kapi.Context, name string) (*api.OAuthClientAuthorization, error) {
	return r.ClientAuthorization, r.GetErr
}

func (r *ClientAuthorizationRegistry) CreateClientAuthorization(ctx kapi.Context, grant *api.OAuthClientAuthorization) (*api.OAuthClientAuthorization, error) {
	r.CreatedAuthorization = grant
	return r.ClientAuthorization, r.CreateErr
}

func (r *ClientAuthorizationRegistry) UpdateClientAuthorization(ctx kapi.Context, grant *api.OAuthClientAuthorization) (*api.OAuthClientAuthorization, error) {
	r.UpdatedAuthorization = grant
	return r.ClientAuthorization, r.UpdateErr
}

func (r *ClientAuthorizationRegistry) DeleteClientAuthorization(ctx kapi.Context, name string) error {
	r.DeletedClientAuthorizationName = name
	return r.DeleteErr
}

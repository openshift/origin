package test

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/labels"

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

func (r *ClientAuthorizationRegistry) ListClientAuthorizations(ctx kapi.Context, label labels.Selector) (*api.OAuthClientAuthorizationList, error) {
	return r.ClientAuthorizations, r.Err
}

func (r *ClientAuthorizationRegistry) GetClientAuthorization(ctx kapi.Context, name string) (*api.OAuthClientAuthorization, error) {
	return r.ClientAuthorization, r.Err
}

func (r *ClientAuthorizationRegistry) CreateClientAuthorization(ctx kapi.Context, grant *api.OAuthClientAuthorization) (*api.OAuthClientAuthorization, error) {
	r.CreatedAuthorization = grant
	return r.ClientAuthorization, r.Err
}

func (r *ClientAuthorizationRegistry) UpdateClientAuthorization(ctx kapi.Context, grant *api.OAuthClientAuthorization) (*api.OAuthClientAuthorization, error) {
	r.UpdatedAuthorization = grant
	return r.ClientAuthorization, r.Err
}

func (r *ClientAuthorizationRegistry) DeleteClientAuthorization(ctx kapi.Context, name string) error {
	r.DeletedClientAuthorizationName = name
	return r.Err
}

package test

import (
	"fmt"

	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
)

type ClientAuthorizationRegistry struct {
	GetErr               error
	ClientAuthorizations *oauthapi.OAuthClientAuthorizationList
	ClientAuthorization  *oauthapi.OAuthClientAuthorization

	CreateErr            error
	CreatedAuthorization *oauthapi.OAuthClientAuthorization

	UpdateErr            error
	UpdatedAuthorization *oauthapi.OAuthClientAuthorization

	DeleteErr                      error
	DeletedClientAuthorizationName string
}

func (r *ClientAuthorizationRegistry) ClientAuthorizationName(userName, clientName string) string {
	return fmt.Sprintf("%s:%s", userName, clientName)
}

func (r *ClientAuthorizationRegistry) ListClientAuthorizations(ctx apirequest.Context, options *metainternal.ListOptions) (*oauthapi.OAuthClientAuthorizationList, error) {
	return r.ClientAuthorizations, r.GetErr
}

func (r *ClientAuthorizationRegistry) GetClientAuthorization(ctx apirequest.Context, name string, options *metav1.GetOptions) (*oauthapi.OAuthClientAuthorization, error) {
	return r.ClientAuthorization, r.GetErr
}

func (r *ClientAuthorizationRegistry) CreateClientAuthorization(ctx apirequest.Context, grant *oauthapi.OAuthClientAuthorization) (*oauthapi.OAuthClientAuthorization, error) {
	r.CreatedAuthorization = grant
	return r.ClientAuthorization, r.CreateErr
}

func (r *ClientAuthorizationRegistry) UpdateClientAuthorization(ctx apirequest.Context, grant *oauthapi.OAuthClientAuthorization) (*oauthapi.OAuthClientAuthorization, error) {
	r.UpdatedAuthorization = grant
	return r.ClientAuthorization, r.UpdateErr
}

func (r *ClientAuthorizationRegistry) DeleteClientAuthorization(ctx apirequest.Context, name string) error {
	r.DeletedClientAuthorizationName = name
	return r.DeleteErr
}

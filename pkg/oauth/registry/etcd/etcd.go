package etcd

import (
	"errors"
	"fmt"
	"path"

	etcderrs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/openshift/origin/pkg/oauth/api"
)

// Etcd implements the AccessToken, AuthorizeToken, and Client registries backed by etcd.
type Etcd struct {
	tools.EtcdHelper
}

// New returns a new Etcd.
func New(helper tools.EtcdHelper) *Etcd {
	return &Etcd{
		EtcdHelper: helper,
	}
}

const (
	OAuthAccessTokenPath         = "/registry/oauth/accessTokens"
	OAuthAuthorizeTokenPath      = "/registry/oauth/authorizeTokens"
	OAuthClientPath              = "/registry/oauth/clients"
	OAuthClientAuthorizationPath = "/registry/oauth/clientAuthorizations"

	OAuthAccessTokenType         = "oauthAccessToken"
	OAuthAuthorizeTokenType      = "oauthAuthorizeToken"
	OAuthClientType              = "oauthClientType"
	OAuthClientAuthorizationType = "oauthClientAuthorization"
)

func makeAccessTokenKey(name string) string {
	return path.Join(OAuthAccessTokenPath, name)
}

func makeAuthorizeTokenKey(name string) string {
	return path.Join(OAuthAuthorizeTokenPath, name)
}

func makeClientKey(name string) string {
	return path.Join(OAuthClientPath, name)
}

func makeClientAuthorizationKey(name string) string {
	return path.Join(OAuthClientAuthorizationPath, name)
}

func (r *Etcd) GetAccessToken(name string) (token *api.OAuthAccessToken, err error) {
	token = &api.OAuthAccessToken{}
	err = etcderrs.InterpretGetError(r.ExtractObj(makeAccessTokenKey(name), token, false), OAuthAccessTokenType, name)
	return
}

func (r *Etcd) ListAccessTokens(selector labels.Selector) (*api.OAuthAccessTokenList, error) {
	list := api.OAuthAccessTokenList{}
	err := r.ExtractToList(OAuthAccessTokenPath, &list)
	if err != nil && !tools.IsEtcdNotFound(err) {
		return nil, err
	}
	filtered := []api.OAuthAccessToken{}
	for _, item := range list.Items {
		if selector.Matches(labels.Set(item.Labels)) {
			filtered = append(filtered, item)
		}
	}
	list.Items = filtered
	return &list, nil
}

func (r *Etcd) CreateAccessToken(token *api.OAuthAccessToken) error {
	err := etcderrs.InterpretCreateError(r.CreateObj(makeAccessTokenKey(token.Name), token, nil, 0), OAuthAccessTokenType, token.Name)
	return err
}

func (r *Etcd) UpdateAccessToken(*api.OAuthAccessToken) error {
	return errors.New("not supported")
}

func (r *Etcd) DeleteAccessToken(name string) error {
	key := makeAccessTokenKey(name)
	err := etcderrs.InterpretDeleteError(r.Delete(key, false), OAuthAccessTokenType, name)
	return err
}

func (r *Etcd) GetAuthorizeToken(name string) (token *api.OAuthAuthorizeToken, err error) {
	token = &api.OAuthAuthorizeToken{}
	err = etcderrs.InterpretGetError(r.ExtractObj(makeAuthorizeTokenKey(name), token, false), OAuthAuthorizeTokenType, name)
	return
}

func (r *Etcd) ListAuthorizeTokens(selector labels.Selector) (*api.OAuthAuthorizeTokenList, error) {
	list := api.OAuthAuthorizeTokenList{}
	err := r.ExtractToList(OAuthAuthorizeTokenPath, &list)
	if err != nil && !tools.IsEtcdNotFound(err) {
		return nil, err
	}
	return &list, nil
}

func (r *Etcd) CreateAuthorizeToken(token *api.OAuthAuthorizeToken) error {
	err := etcderrs.InterpretCreateError(r.CreateObj(makeAuthorizeTokenKey(token.Name), token, nil, 0), OAuthAuthorizeTokenType, token.Name)
	return err
}

func (r *Etcd) UpdateAuthorizeToken(*api.OAuthAuthorizeToken) error {
	return errors.New("not supported")
}

func (r *Etcd) DeleteAuthorizeToken(name string) error {
	key := makeAuthorizeTokenKey(name)
	err := etcderrs.InterpretDeleteError(r.Delete(key, false), OAuthAuthorizeTokenType, name)
	return err
}

func (r *Etcd) GetClient(name string) (client *api.OAuthClient, err error) {
	client = &api.OAuthClient{}
	err = etcderrs.InterpretGetError(r.ExtractObj(makeClientKey(name), client, false), OAuthClientType, name)
	return
}

func (r *Etcd) ListClients(selector labels.Selector) (*api.OAuthClientList, error) {
	list := api.OAuthClientList{}
	err := r.ExtractToList(OAuthClientPath, &list)
	if err != nil && !tools.IsEtcdNotFound(err) {
		return nil, err
	}
	filtered := []api.OAuthClient{}
	for _, item := range list.Items {
		if selector.Matches(labels.Set(item.Labels)) {
			filtered = append(filtered, item)
		}
	}
	list.Items = filtered
	return &list, nil
}

func (r *Etcd) CreateClient(client *api.OAuthClient) error {
	err := etcderrs.InterpretCreateError(r.CreateObj(makeClientKey(client.Name), client, nil, 0), OAuthClientType, client.Name)
	return err
}

func (r *Etcd) UpdateClient(_ *api.OAuthClient) error {
	return errors.New("not supported")
}

func (r *Etcd) DeleteClient(name string) error {
	key := makeClientKey(name)
	err := etcderrs.InterpretDeleteError(r.Delete(key, false), OAuthClientType, name)
	return err
}

func (r *Etcd) ClientAuthorizationName(userName, clientName string) string {
	return fmt.Sprintf("%s:%s", userName, clientName)
}

func (r *Etcd) GetClientAuthorization(name string) (client *api.OAuthClientAuthorization, err error) {
	client = &api.OAuthClientAuthorization{}
	err = etcderrs.InterpretGetError(r.ExtractObj(makeClientAuthorizationKey(name), client, false), OAuthClientAuthorizationType, name)
	return
}

func (r *Etcd) ListClientAuthorizations(label labels.Selector, field fields.Selector) (*api.OAuthClientAuthorizationList, error) {
	list := api.OAuthClientAuthorizationList{}
	err := r.ExtractToList(OAuthClientAuthorizationPath, &list)
	if err != nil && !tools.IsEtcdNotFound(err) {
		return nil, err
	}
	return &list, nil
}

func (r *Etcd) CreateClientAuthorization(client *api.OAuthClientAuthorization) error {
	err := etcderrs.InterpretCreateError(r.CreateObj(makeClientAuthorizationKey(client.Name), client, nil, 0), OAuthClientAuthorizationType, client.Name)
	return err
}

func (r *Etcd) UpdateClientAuthorization(client *api.OAuthClientAuthorization) error {
	err := etcderrs.InterpretUpdateError(r.SetObj(makeClientAuthorizationKey(client.Name), client, nil, 0), OAuthClientAuthorizationType, client.Name)
	return err
}

func (r *Etcd) DeleteClientAuthorization(name string) error {
	key := makeClientAuthorizationKey(name)
	err := etcderrs.InterpretDeleteError(r.Delete(key, false), OAuthClientAuthorizationType, name)
	return err
}

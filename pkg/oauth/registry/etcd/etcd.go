package etcd

import (
	"errors"
	"fmt"

	etcderrs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors/etcd"
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

func makeAccessTokenKey(id string) string {
	return "/accessTokens/" + id
}

func (r *Etcd) GetAccessToken(name string) (token *api.AccessToken, err error) {
	token = &api.AccessToken{}
	err = etcderrs.InterpretGetError(r.ExtractObj(makeAccessTokenKey(name), token, false), "accessToken", name)
	return
}

func (r *Etcd) ListAccessTokens(selector labels.Selector) (*api.AccessTokenList, error) {
	list := api.AccessTokenList{}
	err := r.ExtractToList("/accessTokens", &list)
	if err != nil && !tools.IsEtcdNotFound(err) {
		return nil, err
	}
	filtered := []api.AccessToken{}
	for _, item := range list.Items {
		if selector.Matches(labels.Set(item.Labels)) {
			filtered = append(filtered, item)
		}
	}
	list.Items = filtered
	return &list, nil
}

func (r *Etcd) CreateAccessToken(token *api.AccessToken) error {
	err := etcderrs.InterpretCreateError(r.CreateObj(makeAccessTokenKey(token.Name), token, 0), "accessToken", token.Name)
	return err
}

func (r *Etcd) UpdateAccessToken(*api.AccessToken) error {
	return errors.New("not supported")
}

func (r *Etcd) DeleteAccessToken(name string) error {
	key := makeAccessTokenKey(name)
	err := etcderrs.InterpretDeleteError(r.Delete(key, false), "accessToken", name)
	return err
}

func makeAuthorizeTokenKey(id string) string {
	return "/authorizeTokens/" + id
}

func (r *Etcd) GetAuthorizeToken(name string) (token *api.AuthorizeToken, err error) {
	token = &api.AuthorizeToken{}
	err = etcderrs.InterpretGetError(r.ExtractObj(makeAuthorizeTokenKey(name), token, false), "authorizeToken", name)
	return
}

func (r *Etcd) ListAuthorizeTokens(selector labels.Selector) (*api.AuthorizeTokenList, error) {
	list := api.AuthorizeTokenList{}
	err := r.ExtractToList("/authorizeTokens", &list)
	if err != nil && !tools.IsEtcdNotFound(err) {
		return nil, err
	}
	return &list, nil
}

func (r *Etcd) CreateAuthorizeToken(token *api.AuthorizeToken) error {
	err := etcderrs.InterpretCreateError(r.CreateObj(makeAuthorizeTokenKey(token.Name), token, 0), "authorizeToken", token.Name)
	return err
}

func (r *Etcd) UpdateAuthorizeToken(*api.AuthorizeToken) error {
	return errors.New("not supported")
}

func (r *Etcd) DeleteAuthorizeToken(name string) error {
	key := makeAuthorizeTokenKey(name)
	err := etcderrs.InterpretDeleteError(r.Delete(key, false), "authorizeToken", name)
	return err
}

func makeClientKey(id string) string {
	return "/clients/" + id
}

func (r *Etcd) GetClient(name string) (client *api.Client, err error) {
	client = &api.Client{}
	err = etcderrs.InterpretGetError(r.ExtractObj(makeClientKey(name), client, false), "client", name)
	return
}

func (r *Etcd) ListClients(selector labels.Selector) (*api.ClientList, error) {
	list := api.ClientList{}
	err := r.ExtractToList("/clients", &list)
	if err != nil && !tools.IsEtcdNotFound(err) {
		return nil, err
	}
	filtered := []api.Client{}
	for _, item := range list.Items {
		if selector.Matches(labels.Set(item.Labels)) {
			filtered = append(filtered, item)
		}
	}
	list.Items = filtered
	return &list, nil
}

func (r *Etcd) CreateClient(client *api.Client) error {
	err := etcderrs.InterpretCreateError(r.CreateObj(makeClientKey(client.Name), client, 0), "client", client.Name)
	return err
}

func (r *Etcd) UpdateClient(_ *api.Client) error {
	return errors.New("not supported")
}

func (r *Etcd) DeleteClient(name string) error {
	key := makeClientKey(name)
	err := etcderrs.InterpretDeleteError(r.Delete(key, false), "client", name)
	return err
}

func makeClientAuthorizationKey(id string) string {
	return "/clientAuthorizations/" + id
}

func (r *Etcd) ClientAuthorizationID(userName, clientName string) string {
	return fmt.Sprintf("%s:%s", userName, clientName)
}

func (r *Etcd) GetClientAuthorization(name string) (client *api.ClientAuthorization, err error) {
	client = &api.ClientAuthorization{}
	err = etcderrs.InterpretGetError(r.ExtractObj(makeClientAuthorizationKey(name), client, false), "clientAuthorization", name)
	return
}

func (r *Etcd) ListClientAuthorizations(label, field labels.Selector) (*api.ClientAuthorizationList, error) {
	list := api.ClientAuthorizationList{}
	err := r.ExtractToList("/clients", &list)
	if err != nil && !tools.IsEtcdNotFound(err) {
		return nil, err
	}
	return &list, nil
}

func (r *Etcd) CreateClientAuthorization(client *api.ClientAuthorization) error {
	err := etcderrs.InterpretCreateError(r.CreateObj(makeClientAuthorizationKey(client.Name), client, 0), "clientAuthorization", client.Name)
	return err
}

func (r *Etcd) UpdateClientAuthorization(client *api.ClientAuthorization) error {
	err := etcderrs.InterpretUpdateError(r.SetObj(makeClientAuthorizationKey(client.Name), client), "clientAuthorization", client.Name)
	return err
}

func (r *Etcd) DeleteClientAuthorization(name string) error {
	key := makeClientAuthorizationKey(name)
	err := etcderrs.InterpretDeleteError(r.Delete(key, false), "clientAuthorization", name)
	return err
}

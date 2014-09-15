package etcd

import (
	"errors"
	"fmt"

	apierrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
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
	err = r.ExtractObj(makeAccessTokenKey(name), token, false)
	return
}

func (r *Etcd) ListAccessTokens(selector labels.Selector) (*api.AccessTokenList, error) {
	list := api.AccessTokenList{}
	err := r.ExtractList("/accessTokens", &list.Items, &list.ResourceVersion)
	if err != nil {
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
	err := r.CreateObj(makeAccessTokenKey(token.Name), token, 0)
	if tools.IsEtcdNodeExist(err) {
		return apierrors.NewAlreadyExists("accessToken", token.Name)
	}
	return err
}

func (r *Etcd) UpdateAccessToken(*api.AccessToken) error {
	return errors.New("not supported")
}

func (r *Etcd) DeleteAccessToken(name string) error {
	key := makeAccessTokenKey(name)
	err := r.Delete(key, false)
	if tools.IsEtcdNotFound(err) {
		return apierrors.NewNotFound("accessToken", name)
	}
	return err
}

func makeAuthorizeTokenKey(id string) string {
	return "/authorizeTokens/" + id
}

func (r *Etcd) GetAuthorizeToken(name string) (token *api.AuthorizeToken, err error) {
	token = &api.AuthorizeToken{}
	err = r.ExtractObj(makeAuthorizeTokenKey(name), token, false)
	return
}

func (r *Etcd) ListAuthorizeTokens(selector labels.Selector) (*api.AuthorizeTokenList, error) {
	list := api.AuthorizeTokenList{}
	err := r.ExtractList("/authorizeTokens", &list.Items, &list.ResourceVersion)
	return &list, err
}

func (r *Etcd) CreateAuthorizeToken(token *api.AuthorizeToken) error {
	err := r.CreateObj(makeAuthorizeTokenKey(token.Name), token, 0)
	if tools.IsEtcdNodeExist(err) {
		return apierrors.NewAlreadyExists("authorizeToken", token.Name)
	}
	return err
}

func (r *Etcd) UpdateAuthorizeToken(_ *api.AuthorizeToken) error {
	return errors.New("not supported")
}

func (r *Etcd) DeleteAuthorizeToken(name string) error {
	key := makeAuthorizeTokenKey(name)
	err := r.Delete(key, false)
	if tools.IsEtcdNotFound(err) {
		return apierrors.NewNotFound("authorizeToken", name)
	}
	return err
}

func makeClientKey(id string) string {
	return "/clients/" + id
}

func (r *Etcd) GetClient(name string) (client *api.Client, err error) {
	client = &api.Client{}
	err = r.ExtractObj(makeClientKey(name), client, false)
	return
}

func (r *Etcd) ListClients(selector labels.Selector) (*api.ClientList, error) {
	list := api.ClientList{}
	err := r.ExtractList("/clients", &list.Items, &list.ResourceVersion)
	if err != nil {
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
	err := r.CreateObj(makeClientKey(client.Name), client, 0)
	if tools.IsEtcdNodeExist(err) {
		return apierrors.NewAlreadyExists("client", client.Name)
	}
	return err
}

func (r *Etcd) UpdateClient(_ *api.Client) error {
	return errors.New("not supported")
}

func (r *Etcd) DeleteClient(name string) error {
	key := makeClientKey(name)
	err := r.Delete(key, false)
	if tools.IsEtcdNotFound(err) {
		return apierrors.NewNotFound("client", name)
	}
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
	err = r.ExtractObj(makeClientAuthorizationKey(name), client, false)
	return
}

func (r *Etcd) ListClientAuthorizations(label, field labels.Selector) (*api.ClientAuthorizationList, error) {
	list := api.ClientAuthorizationList{}
	err := r.ExtractList("/clients", &list.Items, &list.ResourceVersion)
	if err != nil {
		return nil, err
	}
	return &list, nil
}

func (r *Etcd) CreateClientAuthorization(client *api.ClientAuthorization) error {
	err := r.CreateObj(makeClientAuthorizationKey(client.ID), client, 0)
	if tools.IsEtcdNodeExist(err) {
		return apierrors.NewAlreadyExists("clientAuthorization", client.ID)
	}
	return err
}

func (r *Etcd) UpdateClientAuthorization(_ *api.ClientAuthorization) error {
	return errors.New("not supported")
}

func (r *Etcd) DeleteClientAuthorization(name string) error {
	key := makeClientAuthorizationKey(name)
	err := r.Delete(key, false)
	if tools.IsEtcdNotFound(err) {
		return apierrors.NewNotFound("clientAuthorization", name)
	}
	return err
}

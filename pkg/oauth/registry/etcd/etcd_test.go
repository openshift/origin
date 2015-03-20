package etcd

import (
	"reflect"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/coreos/go-etcd/etcd"

	"github.com/openshift/origin/pkg/api/latest"
	oapi "github.com/openshift/origin/pkg/oauth/api"
)

func NewTestEtcdRegistry(client tools.EtcdGetSet) *Etcd {
	return New(tools.NewEtcdHelper(client, latest.Codec))
}

func TestGetAccessTokenNotFound(t *testing.T) {
	key := makeAccessTokenKey("foo")
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.ExpectNotFoundGet(key)
	registry := NewTestEtcdRegistry(fakeClient)
	_, err := registry.GetAccessToken("foo")
	if err == nil {
		t.Fatalf("expected not found error")
	}
}

func TestGetAccessToken(t *testing.T) {
	key := makeAccessTokenKey("foo")
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Set(key, runtime.EncodeOrDie(latest.Codec, &oapi.OAuthAccessToken{ObjectMeta: api.ObjectMeta{Name: "foo"}}), 0)
	registry := NewTestEtcdRegistry(fakeClient)
	token, err := registry.GetAccessToken("foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
		return
	}
	if token.Name != "foo" {
		t.Fatalf("expected token named foo, got %v", token)
	}
}

func TestListAccessTokensEmpty(t *testing.T) {
	key := OAuthAccessTokenPath
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.ExpectNotFoundGet(key)
	registry := NewTestEtcdRegistry(fakeClient)
	tokens, err := registry.ListAccessTokens(labels.Everything())
	if err != nil {
		t.Fatalf("got unexpected error: %v", err)
		return
	}
	if len(tokens.Items) != 0 {
		t.Fatalf("expected empty tokens list, got %v", tokens)
	}
}

func TestListAccessTokens(t *testing.T) {
	key := OAuthAccessTokenPath
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{Value: runtime.EncodeOrDie(latest.Codec, &oapi.OAuthAccessToken{ObjectMeta: api.ObjectMeta{Name: "foo"}})},
					{Value: runtime.EncodeOrDie(latest.Codec, &oapi.OAuthAccessToken{ObjectMeta: api.ObjectMeta{Name: "bar"}})},
				},
			},
		},
		E: nil,
	}

	registry := NewTestEtcdRegistry(fakeClient)
	tokens, err := registry.ListAccessTokens(labels.Everything())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
		return
	}
	if len(tokens.Items) != 2 {
		t.Fatalf("expected a list of 2 tokens, got %v", tokens)
	}
}

func TestListAccessTokensFiltered(t *testing.T) {
	// TODO
}

func TestCreateAccessToken(t *testing.T) {
	token := &oapi.OAuthAccessToken{ObjectMeta: api.ObjectMeta{Name: "foo"}}

	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcdRegistry(fakeClient)
	err := registry.CreateAccessToken(token)
	if err != nil {
		t.Fatalf("unexpected error saving: %v", err)
		return
	}
	storedtoken, err := registry.GetAccessToken(token.Name)
	if err != nil {
		t.Fatalf("unexpected error retrieving: %v", err)
		return
	}
	if storedtoken.Name != token.Name {
		t.Fatalf("stored token didn't match original token: %v", storedtoken)
	}
}

func TestDeleteAccessToken(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcdRegistry(fakeClient)

	token := &oapi.OAuthAccessToken{ObjectMeta: api.ObjectMeta{Name: "foo"}}

	if err := registry.CreateAccessToken(token); err != nil {
		t.Fatalf("unexpected error saving: %v", err)
		return
	}
	if _, err := registry.GetAccessToken("foo"); errors.IsNotFound(err) {
		t.Fatalf("token was not saved")
		return
	}
	if err := registry.DeleteAccessToken("foo"); err != nil {
		t.Fatalf("unexpected error deleting")
		return
	}
	if stored, err := registry.GetAccessToken("foo"); !errors.IsNotFound(err) {
		t.Fatalf("token was retrieved after deleting: %v", stored)
		return
	}
	if err := registry.DeleteAccessToken("foo"); !errors.IsNotFound(err) {
		t.Fatalf("unexpected error deleting non-existent token")
		return
	}
}

func TestGetAuthorizeTokenNotFound(t *testing.T) {
	key := makeAuthorizeTokenKey("foo")
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.ExpectNotFoundGet(key)
	registry := NewTestEtcdRegistry(fakeClient)
	_, err := registry.GetAuthorizeToken("foo")
	if err == nil {
		t.Fatalf("expected not found error")
	}
}

func TestGetAuthorizeToken(t *testing.T) {
	key := makeAuthorizeTokenKey("foo")
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Set(key, runtime.EncodeOrDie(latest.Codec, &oapi.OAuthAuthorizeToken{ObjectMeta: api.ObjectMeta{Name: "foo"}}), 0)
	registry := NewTestEtcdRegistry(fakeClient)
	token, err := registry.GetAuthorizeToken("foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
		return
	}
	if token.Name != "foo" {
		t.Fatalf("expected token named foo, got %v", token)
	}
}

func TestListAuthorizeTokensEmpty(t *testing.T) {
	key := OAuthAuthorizeTokenPath
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.ExpectNotFoundGet(key)
	registry := NewTestEtcdRegistry(fakeClient)
	tokens, err := registry.ListAuthorizeTokens(labels.Everything())
	if err != nil {
		t.Fatalf("got unexpected error: %v", err)
		return
	}
	if len(tokens.Items) != 0 {
		t.Fatalf("expected empty tokens list, got %v", tokens)
	}
}

func TestListAuthorizeTokens(t *testing.T) {
	key := OAuthAuthorizeTokenPath
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{Value: runtime.EncodeOrDie(latest.Codec, &oapi.OAuthAuthorizeToken{ObjectMeta: api.ObjectMeta{Name: "foo"}})},
					{Value: runtime.EncodeOrDie(latest.Codec, &oapi.OAuthAuthorizeToken{ObjectMeta: api.ObjectMeta{Name: "bar"}})},
				},
			},
		},
		E: nil,
	}

	registry := NewTestEtcdRegistry(fakeClient)
	tokens, err := registry.ListAuthorizeTokens(labels.Everything())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
		return
	}
	if len(tokens.Items) != 2 {
		t.Fatalf("expected a list of 2 tokens, got %v", tokens)
	}
}

func TestListAuthorizeTokensFiltered(t *testing.T) {
	// TODO
}

func TestCreateAuthorizeToken(t *testing.T) {
	token := &oapi.OAuthAuthorizeToken{ObjectMeta: api.ObjectMeta{Name: "foo"}}

	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcdRegistry(fakeClient)
	err := registry.CreateAuthorizeToken(token)
	if err != nil {
		t.Fatalf("unexpected error saving: %v", err)
		return
	}
	storedtoken, err := registry.GetAuthorizeToken(token.Name)
	if err != nil {
		t.Fatalf("unexpected error retrieving: %v", err)
		return
	}
	if storedtoken.Name != token.Name {
		t.Fatalf("stored token didn't match original token: %v", storedtoken)
	}
}

func TestDeleteAuthorizeToken(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcdRegistry(fakeClient)

	token := &oapi.OAuthAuthorizeToken{ObjectMeta: api.ObjectMeta{Name: "foo"}}

	if err := registry.CreateAuthorizeToken(token); err != nil {
		t.Fatalf("unexpected error saving: %v", err)
		return
	}
	if _, err := registry.GetAuthorizeToken("foo"); errors.IsNotFound(err) {
		t.Fatalf("token was not saved")
		return
	}
	if err := registry.DeleteAuthorizeToken("foo"); err != nil {
		t.Fatalf("unexpected error deleting")
		return
	}
	if stored, err := registry.GetAuthorizeToken("foo"); !errors.IsNotFound(err) {
		t.Fatalf("token was retrieved after deleting: %v", stored)
		return
	}
	if err := registry.DeleteAuthorizeToken("foo"); !errors.IsNotFound(err) {
		t.Fatalf("unexpected error deleting non-existent token")
		return
	}
}

func TestGetClientNotFound(t *testing.T) {
	key := makeClientKey("foo")
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.ExpectNotFoundGet(key)
	registry := NewTestEtcdRegistry(fakeClient)
	_, err := registry.GetClient("foo")
	if err == nil {
		t.Fatalf("expected not found error")
	}
}

func TestGetClient(t *testing.T) {
	key := makeClientKey("foo")
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Set(key, runtime.EncodeOrDie(latest.Codec, &oapi.OAuthClient{ObjectMeta: api.ObjectMeta{Name: "foo"}}), 0)
	registry := NewTestEtcdRegistry(fakeClient)
	client, err := registry.GetClient("foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
		return
	}
	if client.Name != "foo" {
		t.Fatalf("expected client named foo, got %v", client)
	}
}

func TestListClientsEmpty(t *testing.T) {
	key := OAuthClientPath
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.ExpectNotFoundGet(key)
	registry := NewTestEtcdRegistry(fakeClient)
	clients, err := registry.ListClients(labels.Everything())
	if err != nil {
		t.Fatalf("got unexpected error: %v", err)
		return
	}
	if len(clients.Items) != 0 {
		t.Fatalf("expected empty clients list, got %v", clients)
	}
}

func TestListClients(t *testing.T) {
	key := OAuthClientPath
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{Value: runtime.EncodeOrDie(latest.Codec, &oapi.OAuthClient{ObjectMeta: api.ObjectMeta{Name: "foo"}})},
					{Value: runtime.EncodeOrDie(latest.Codec, &oapi.OAuthClient{ObjectMeta: api.ObjectMeta{Name: "bar"}})},
				},
			},
		},
		E: nil,
	}

	registry := NewTestEtcdRegistry(fakeClient)
	clients, err := registry.ListClients(labels.Everything())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
		return
	}
	if len(clients.Items) != 2 {
		t.Fatalf("expected a list of 2 clients, got %v", clients)
	}
}

func TestListClientsFiltered(t *testing.T) {
	// TODO
}

func TestCreateClient(t *testing.T) {
	client := &oapi.OAuthClient{ObjectMeta: api.ObjectMeta{Name: "foo"}}

	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcdRegistry(fakeClient)
	err := registry.CreateClient(client)
	if err != nil {
		t.Fatalf("unexpected error saving: %v", err)
		return
	}
	storedclient, err := registry.GetClient(client.Name)
	if err != nil {
		t.Fatalf("unexpected error retrieving: %v", err)
		return
	}
	if storedclient.Name != client.Name {
		t.Fatalf("stored client didn't match original client: %v", storedclient)
	}
}

func TestDeleteClient(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcdRegistry(fakeClient)

	client := &oapi.OAuthClient{ObjectMeta: api.ObjectMeta{Name: "foo"}}

	if err := registry.CreateClient(client); err != nil {
		t.Fatalf("unexpected error saving: %v", err)
		return
	}
	if _, err := registry.GetClient("foo"); errors.IsNotFound(err) {
		t.Fatalf("client was not saved")
		return
	}
	if err := registry.DeleteClient("foo"); err != nil {
		t.Fatalf("unexpected error deleting")
		return
	}
	if stored, err := registry.GetClient("foo"); !errors.IsNotFound(err) {
		t.Fatalf("client was retrieved after deleting: %v", stored)
		return
	}
	if err := registry.DeleteClient("foo"); !errors.IsNotFound(err) {
		t.Fatalf("unexpected error deleting non-existent client")
		return
	}
}

func TestGetClientAuthorizationNotFound(t *testing.T) {
	key := makeClientAuthorizationKey("foo")
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.ExpectNotFoundGet(key)
	registry := NewTestEtcdRegistry(fakeClient)
	_, err := registry.GetClientAuthorization("foo")
	if err == nil {
		t.Fatalf("expected not found error")
	}
}

func TestGetClientAuthorization(t *testing.T) {
	key := makeClientAuthorizationKey("foo")
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Set(key, runtime.EncodeOrDie(latest.Codec, &oapi.OAuthClientAuthorization{ObjectMeta: api.ObjectMeta{Name: "foo"}}), 0)
	registry := NewTestEtcdRegistry(fakeClient)
	clientAuth, err := registry.GetClientAuthorization("foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
		return
	}
	if clientAuth.Name != "foo" {
		t.Fatalf("expected clientAuth named foo, got %v", clientAuth)
	}
}

func TestListClientAuthorizationsEmpty(t *testing.T) {
	key := OAuthClientAuthorizationPath
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.ExpectNotFoundGet(key)
	registry := NewTestEtcdRegistry(fakeClient)
	clientAuths, err := registry.ListClientAuthorizations(labels.Everything(), fields.Everything())
	if err != nil {
		t.Fatalf("got unexpected error: %v", err)
		return
	}
	if len(clientAuths.Items) != 0 {
		t.Fatalf("expected empty client auths list, got %v", clientAuths)
	}
}

func TestListClientAuthorizations(t *testing.T) {
	key := OAuthClientAuthorizationPath
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{Value: runtime.EncodeOrDie(latest.Codec, &oapi.OAuthClientAuthorization{ObjectMeta: api.ObjectMeta{Name: "foo"}})},
					{Value: runtime.EncodeOrDie(latest.Codec, &oapi.OAuthClientAuthorization{ObjectMeta: api.ObjectMeta{Name: "bar"}})},
				},
			},
		},
		E: nil,
	}

	registry := NewTestEtcdRegistry(fakeClient)
	clientAuths, err := registry.ListClientAuthorizations(labels.Everything(), fields.Everything())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
		return
	}
	if len(clientAuths.Items) != 2 {
		t.Fatalf("expected a list of 2 client auths, got %v", clientAuths)
	}
}

func TestListClientAuthorizationsFiltered(t *testing.T) {
	// TODO
}

func TestCreateClientAuthorization(t *testing.T) {
	clientAuthorization := &oapi.OAuthClientAuthorization{ObjectMeta: api.ObjectMeta{Name: "foo"}}

	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcdRegistry(fakeClient)
	err := registry.CreateClientAuthorization(clientAuthorization)
	if err != nil {
		t.Fatalf("unexpected error saving: %v", err)
		return
	}
	storedclientauth, err := registry.GetClientAuthorization(clientAuthorization.Name)
	if err != nil {
		t.Fatalf("unexpected error retrieving: %v", err)
		return
	}
	if storedclientauth.Name != clientAuthorization.Name {
		t.Fatalf("stored client auth didn't match original client auth: %v", storedclientauth)
	}
}

func TestDeleteClientAuthorization(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcdRegistry(fakeClient)

	clientAuth := &oapi.OAuthClientAuthorization{ObjectMeta: api.ObjectMeta{Name: "foo"}}

	if err := registry.CreateClientAuthorization(clientAuth); err != nil {
		t.Fatalf("unexpected error saving: %v", err)
		return
	}
	if _, err := registry.GetClientAuthorization("foo"); errors.IsNotFound(err) {
		t.Fatalf("client auth was not saved")
		return
	}
	if err := registry.DeleteClientAuthorization("foo"); err != nil {
		t.Fatalf("unexpected error deleting")
		return
	}
	if stored, err := registry.GetClientAuthorization("foo"); !errors.IsNotFound(err) {
		t.Fatalf("client auth was retrieved after deleting: %v", stored)
		return
	}
	if err := registry.DeleteClientAuthorization("foo"); !errors.IsNotFound(err) {
		t.Fatalf("unexpected error deleting non-existent client auth")
		return
	}
}

func TestUpdateClientAuthorizationNotFound(t *testing.T) {
	clientAuth := &oapi.OAuthClientAuthorization{
		ObjectMeta: api.ObjectMeta{
			Name:            "foo",
			ResourceVersion: "1",
		},
	}

	key := makeClientAuthorizationKey(clientAuth.Name)
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	fakeClient.ExpectNotFoundGet(key)
	registry := NewTestEtcdRegistry(fakeClient)

	err := registry.UpdateClientAuthorization(clientAuth)
	if err == nil {
		t.Fatalf("expected error updating non-existent client authorization")
	}
}

func TestUpdateClientAuthorization(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true

	registry := NewTestEtcdRegistry(fakeClient)

	clientAuth := &oapi.OAuthClientAuthorization{
		ClientName: "myclient",
		UserName:   "myuser",
		UserUID:    "myuseruid",
		Scopes:     []string{"A"},
	}
	clientAuth.Name = registry.ClientAuthorizationName(clientAuth.UserName, clientAuth.ClientName)

	if err := registry.CreateClientAuthorization(clientAuth); err != nil {
		t.Fatalf("unexpected error creating client authorization: %v", err)
		return
	}

	savedAuth, err := registry.GetClientAuthorization(clientAuth.Name)
	if err != nil {
		t.Fatalf("unexpected error fetching client authorization: %v", err)
		return
	}

	savedAuth.Scopes = []string{"A", "B"}
	if err := registry.UpdateClientAuthorization(savedAuth); err != nil {
		t.Fatalf("unexpected error updating client authorization %#v: %#v", savedAuth, err)
		return
	}

	updatedAuth, err := registry.GetClientAuthorization(clientAuth.Name)
	if err != nil {
		t.Fatalf("unexpected error fetching updated client authorization: %v", err)
		return
	}
	if !reflect.DeepEqual(updatedAuth.Scopes, savedAuth.Scopes) {
		t.Fatalf("client authorization was not updated: %v", updatedAuth)
	}
}

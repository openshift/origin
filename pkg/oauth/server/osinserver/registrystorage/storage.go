package registrystorage

import (
	"errors"
	"strings"

	"github.com/RangelReale/osin"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken"
	"github.com/openshift/origin/pkg/oauth/registry/oauthauthorizetoken"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	"github.com/openshift/origin/pkg/oauth/scope"
)

type UserConversion interface {
	ConvertToAuthorizeToken(interface{}, *api.OAuthAuthorizeToken) error
	ConvertToAccessToken(interface{}, *api.OAuthAccessToken) error
	ConvertFromAuthorizeToken(*api.OAuthAuthorizeToken) (interface{}, error)
	ConvertFromAccessToken(*api.OAuthAccessToken) (interface{}, error)
}

type storage struct {
	accesstoken    oauthaccesstoken.Registry
	authorizetoken oauthauthorizetoken.Registry
	client         oauthclient.Registry
	user           UserConversion
}

func New(access oauthaccesstoken.Registry, authorize oauthauthorizetoken.Registry, client oauthclient.Registry, user UserConversion) osin.Storage {
	return &storage{
		accesstoken:    access,
		authorizetoken: authorize,
		client:         client,
		user:           user,
	}
}

type clientWrapper struct {
	id     string
	client *api.OAuthClient
}

func (w *clientWrapper) GetId() string {
	return w.id
}

func (w *clientWrapper) GetSecret() string {
	return w.client.Secret
}

func (w *clientWrapper) GetRedirectUri() string {
	if len(w.client.RedirectURIs) == 0 {
		return ""
	}
	return strings.Join(w.client.RedirectURIs, ",")
}

func (w *clientWrapper) GetUserData() interface{} {
	return w.client
}

// Clone the storage if needed. For example, using mgo, you can clone the session with session.Clone
// to avoid concurrent access problems.
// This is to avoid cloning the connection at each method access.
// Can return itself if not a problem.
func (s *storage) Clone() osin.Storage {
	return s
}

// Close the resources the Storage potentially holds (using Clone for example)
func (s *storage) Close() {
}

// GetClient loads the client by id (client_id)
func (s *storage) GetClient(id string) (osin.Client, error) {
	c, err := s.client.GetClient(kapi.NewContext(), id)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &clientWrapper{id, c}, nil
}

// SaveAuthorize saves authorize data.
func (s *storage) SaveAuthorize(data *osin.AuthorizeData) error {
	token, err := s.convertToAuthorizeToken(data)
	if err != nil {
		return err
	}
	_, err = s.authorizetoken.CreateAuthorizeToken(kapi.NewContext(), token)
	return err
}

// LoadAuthorize looks up AuthorizeData by a code.
// Client information MUST be loaded together.
// Optionally can return error if expired.
func (s *storage) LoadAuthorize(code string) (*osin.AuthorizeData, error) {
	authorize, err := s.authorizetoken.GetAuthorizeToken(kapi.NewContext(), code)
	if err != nil {
		return nil, err
	}
	return s.convertFromAuthorizeToken(authorize)
}

// RemoveAuthorize revokes or deletes the authorization code.
func (s *storage) RemoveAuthorize(code string) error {
	// TODO: return no error if registry returns IsNotFound
	return s.authorizetoken.DeleteAuthorizeToken(kapi.NewContext(), code)
}

// SaveAccess writes AccessData.
// If RefreshToken is not blank, it must save in a way that can be loaded using LoadRefresh.
func (s *storage) SaveAccess(data *osin.AccessData) error {
	token, err := s.convertToAccessToken(data)
	if err != nil {
		return err
	}
	_, err = s.accesstoken.CreateAccessToken(kapi.NewContext(), token)
	return err
}

// LoadAccess retrieves access data by token. Client information MUST be loaded together.
// AuthorizeData and AccessData DON'T NEED to be loaded if not easily available.
// Optionally can return error if expired.
func (s *storage) LoadAccess(token string) (*osin.AccessData, error) {
	access, err := s.accesstoken.GetAccessToken(kapi.NewContext(), token)
	if err != nil {
		return nil, err
	}
	return s.convertFromAccessToken(access)
}

// RemoveAccess revokes or deletes an AccessData.
func (s *storage) RemoveAccess(token string) error {
	// TODO: return no error if registry returns IsNotFound
	return s.accesstoken.DeleteAccessToken(kapi.NewContext(), token)
}

// LoadRefresh retrieves refresh AccessData. Client information MUST be loaded together.
// AuthorizeData and AccessData DON'T NEED to be loaded if not easily available.
// Optionally can return error if expired.
func (s *storage) LoadRefresh(token string) (*osin.AccessData, error) {
	return nil, errors.New("not implemented")
}

// RemoveRefresh revokes or deletes refresh AccessData.
func (s *storage) RemoveRefresh(token string) error {
	return errors.New("not implemented")
}

func (s *storage) convertToAuthorizeToken(data *osin.AuthorizeData) (*api.OAuthAuthorizeToken, error) {
	token := &api.OAuthAuthorizeToken{
		ObjectMeta: kapi.ObjectMeta{
			Name:              data.Code,
			CreationTimestamp: util.Time{Time: data.CreatedAt},
		},
		ClientName:  data.Client.GetId(),
		ExpiresIn:   int64(data.ExpiresIn),
		Scopes:      scope.Split(data.Scope),
		RedirectURI: data.RedirectUri,
		State:       data.State,
	}
	if err := s.user.ConvertToAuthorizeToken(data.UserData, token); err != nil {
		return nil, err
	}
	return token, nil
}

func (s *storage) convertFromAuthorizeToken(authorize *api.OAuthAuthorizeToken) (*osin.AuthorizeData, error) {
	user, err := s.user.ConvertFromAuthorizeToken(authorize)
	if err != nil {
		return nil, err
	}
	client, err := s.client.GetClient(kapi.NewContext(), authorize.ClientName)
	if err != nil {
		return nil, err
	}

	return &osin.AuthorizeData{
		Code:        authorize.Name,
		Client:      &clientWrapper{authorize.ClientName, client},
		ExpiresIn:   int32(authorize.ExpiresIn),
		Scope:       scope.Join(authorize.Scopes),
		RedirectUri: authorize.RedirectURI,
		State:       authorize.State,
		CreatedAt:   authorize.CreationTimestamp.Time,
		UserData:    user,
	}, nil
}

func (s *storage) convertToAccessToken(data *osin.AccessData) (*api.OAuthAccessToken, error) {
	token := &api.OAuthAccessToken{
		ObjectMeta: kapi.ObjectMeta{
			Name:              data.AccessToken,
			CreationTimestamp: util.Time{Time: data.CreatedAt},
		},
		ExpiresIn:    int64(data.ExpiresIn),
		RefreshToken: data.RefreshToken,
		ClientName:   data.Client.GetId(),
		Scopes:       scope.Split(data.Scope),
		RedirectURI:  data.RedirectUri,
	}
	if data.AuthorizeData != nil {
		token.AuthorizeToken = data.AuthorizeData.Code
	}
	if err := s.user.ConvertToAccessToken(data.UserData, token); err != nil {
		return nil, err
	}
	return token, nil
}

func (s *storage) convertFromAccessToken(access *api.OAuthAccessToken) (*osin.AccessData, error) {
	user, err := s.user.ConvertFromAccessToken(access)
	if err != nil {
		return nil, err
	}
	client, err := s.client.GetClient(kapi.NewContext(), access.ClientName)
	if err != nil {
		return nil, err
	}

	return &osin.AccessData{
		AccessToken:  access.Name,
		RefreshToken: access.RefreshToken,
		Client:       &clientWrapper{access.ClientName, client},
		ExpiresIn:    int32(access.ExpiresIn),
		Scope:        scope.Join(access.Scopes),
		RedirectUri:  access.RedirectURI,
		CreatedAt:    access.CreationTimestamp.Time,
		UserData:     user,
	}, nil
}

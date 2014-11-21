package registrystorage

import (
	"errors"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	apierrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/RangelReale/osin"

	"github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/accesstoken"
	"github.com/openshift/origin/pkg/oauth/registry/authorizetoken"
	"github.com/openshift/origin/pkg/oauth/registry/client"
	"github.com/openshift/origin/pkg/oauth/scope"
)

type UserConversion interface {
	ConvertToAuthorizeToken(interface{}, *api.AuthorizeToken) error
	ConvertToAccessToken(interface{}, *api.AccessToken) error
	ConvertFromAuthorizeToken(*api.AuthorizeToken) (interface{}, error)
	ConvertFromAccessToken(*api.AccessToken) (interface{}, error)
}

type storage struct {
	accesstoken    accesstoken.Registry
	authorizetoken authorizetoken.Registry
	client         client.Registry
	user           UserConversion
}

func New(access accesstoken.Registry, authorize authorizetoken.Registry, client client.Registry, user UserConversion) osin.Storage {
	return &storage{
		accesstoken:    access,
		authorizetoken: authorize,
		client:         client,
		user:           user,
	}
}

type clientWrapper struct {
	id     string
	client *api.Client
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
	return nil
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
	c, err := s.client.GetClient(id)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &clientWrapper{id, c}, nil
}

// SaveAuthorize saves authorize data.
func (s *storage) SaveAuthorize(data *osin.AuthorizeData) error {
	token := &api.AuthorizeToken{
		ObjectMeta: kapi.ObjectMeta{
			CreationTimestamp: util.Time{data.CreatedAt},
			Name:              data.Code,
		},
		ClientName:  data.Client.GetId(),
		ExpiresIn:   int64(data.ExpiresIn),
		Scopes:      scope.Split(data.Scope),
		RedirectURI: data.RedirectUri,
		State:       data.State,
	}
	if err := s.user.ConvertToAuthorizeToken(data.UserData, token); err != nil {
		return err
	}
	return s.authorizetoken.CreateAuthorizeToken(token)
}

// LoadAuthorize looks up AuthorizeData by a code.
// Client information MUST be loaded together.
// Optionally can return error if expired.
func (s *storage) LoadAuthorize(code string) (*osin.AuthorizeData, error) {
	authorize, err := s.authorizetoken.GetAuthorizeToken(code)
	if err != nil {
		return nil, err
	}
	user, err := s.user.ConvertFromAuthorizeToken(authorize)
	if err != nil {
		return nil, err
	}
	client, err := s.client.GetClient(authorize.ClientName)
	if err != nil {
		return nil, err
	}

	return &osin.AuthorizeData{
		Code:        code,
		Client:      &clientWrapper{authorize.ClientName, client},
		ExpiresIn:   int32(authorize.ExpiresIn),
		Scope:       scope.Join(authorize.Scopes),
		RedirectUri: authorize.RedirectURI,
		State:       authorize.State,
		CreatedAt:   authorize.CreationTimestamp.Time,
		UserData:    user,
	}, nil
}

// RemoveAuthorize revokes or deletes the authorization code.
func (s *storage) RemoveAuthorize(code string) error {
	// TODO: return no error if registry returns IsNotFound
	return s.authorizetoken.DeleteAuthorizeToken(code)
}

// SaveAccess writes AccessData.
// If RefreshToken is not blank, it must save in a way that can be loaded using LoadRefresh.
func (s *storage) SaveAccess(data *osin.AccessData) error {
	token := &api.AccessToken{
		ObjectMeta: kapi.ObjectMeta{
			CreationTimestamp: util.Time{data.CreatedAt},
			Name:              data.AccessToken,
		},
		RefreshToken: data.RefreshToken,
		AuthorizeToken: api.AuthorizeToken{
			ClientName:  data.Client.GetId(),
			ExpiresIn:   int64(data.ExpiresIn),
			Scopes:      scope.Split(data.Scope),
			RedirectURI: data.RedirectUri,
		},
	}
	if err := s.user.ConvertToAccessToken(data.UserData, token); err != nil {
		return err
	}
	return s.accesstoken.CreateAccessToken(token)
}

// LoadAccess retrieves access data by token. Client information MUST be loaded together.
// AuthorizeData and AccessData DON'T NEED to be loaded if not easily available.
// Optionally can return error if expired.
func (s *storage) LoadAccess(token string) (*osin.AccessData, error) {
	access, err := s.accesstoken.GetAccessToken(token)
	if err != nil {
		return nil, err
	}
	user, err := s.user.ConvertFromAccessToken(access)
	if err != nil {
		return nil, err
	}
	client, err := s.client.GetClient(access.AuthorizeToken.ClientName)
	if err != nil {
		return nil, err
	}

	return &osin.AccessData{
		AccessToken:  token,
		RefreshToken: access.RefreshToken,
		Client:       &clientWrapper{access.AuthorizeToken.ClientName, client},
		ExpiresIn:    int32(access.AuthorizeToken.ExpiresIn),
		Scope:        scope.Join(access.AuthorizeToken.Scopes),
		RedirectUri:  access.AuthorizeToken.RedirectURI,
		CreatedAt:    access.CreationTimestamp.Time,
		UserData:     user,
	}, nil
}

// RemoveAccess revokes or deletes an AccessData.
func (s *storage) RemoveAccess(token string) error {
	// TODO: return no error if registry returns IsNotFound
	return s.accesstoken.DeleteAccessToken(token)
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

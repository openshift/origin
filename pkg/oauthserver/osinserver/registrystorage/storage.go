package registrystorage

import (
	"errors"
	"strings"

	"github.com/RangelReale/osin"
	"github.com/golang/glog"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	oauthapi "github.com/openshift/api/oauth/v1"
	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	scopeauthorizer "github.com/openshift/origin/pkg/authorization/authorizer/scope"
	oauthclientregistry "github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	"github.com/openshift/origin/pkg/oauth/scope"
	"github.com/openshift/origin/pkg/oauthserver/oauth/handlers"
)

type UserConversion interface {
	ConvertToAuthorizeToken(interface{}, *oauthapi.OAuthAuthorizeToken) error
	ConvertToAccessToken(interface{}, *oauthapi.OAuthAccessToken) error
	ConvertFromAuthorizeToken(*oauthapi.OAuthAuthorizeToken) (interface{}, error)
	ConvertFromAccessToken(*oauthapi.OAuthAccessToken) (interface{}, error)
}

type storage struct {
	accesstoken    oauthclient.OAuthAccessTokenInterface
	authorizetoken oauthclient.OAuthAuthorizeTokenInterface
	client         oauthclientregistry.Getter
	user           UserConversion
	tokentimeout   int32
}

func New(access oauthclient.OAuthAccessTokenInterface, authorize oauthclient.OAuthAuthorizeTokenInterface, client oauthclientregistry.Getter, user UserConversion, tokentimeout int32) osin.Storage {
	return &storage{
		accesstoken:    access,
		authorizetoken: authorize,
		client:         client,
		user:           user,
		tokentimeout:   tokentimeout,
	}
}

type clientWrapper struct {
	id     string
	client *oauthapi.OAuthClient
}

// Ensure we implement the secret matcher method that allows us to validate multiple secrets
var _ = osin.Client(&clientWrapper{})
var _ = osin.ClientSecretMatcher(&clientWrapper{})
var _ = handlers.TokenMaxAgeSeconds(&clientWrapper{})
var _ = handlers.TokenTimeoutSeconds(&clientWrapper{})

func (w *clientWrapper) GetId() string {
	return w.id
}

func (w *clientWrapper) GetSecret() string {
	// Required to implement osin.Client, but should never be called, since we implement osin.ClientSecretMatcher
	panic("unsupported")
}

func (w *clientWrapper) ClientSecretMatches(secret string) bool {
	if w.client.Secret == secret {
		return true
	}

	for _, additionalSecret := range w.client.AdditionalSecrets {
		if additionalSecret == secret {
			return true
		}
	}

	return false
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

func (w *clientWrapper) GetTokenMaxAgeSeconds() *int32 {
	return w.client.AccessTokenMaxAgeSeconds
}

func (w *clientWrapper) GetAccessTokenInactivityTimeoutSeconds() *int32 {
	return w.client.AccessTokenInactivityTimeoutSeconds
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
	c, err := s.client.Get(id, metav1.GetOptions{})
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
	_, err = s.authorizetoken.Create(token)
	return err
}

// LoadAuthorize looks up AuthorizeData by a code.
// Client information MUST be loaded together.
// Optionally can return error if expired.
func (s *storage) LoadAuthorize(code string) (*osin.AuthorizeData, error) {
	authorize, err := s.authorizetoken.Get(code, metav1.GetOptions{})
	if kerrors.IsNotFound(err) {
		glog.V(5).Info("Authorization code not found")
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.convertFromAuthorizeToken(authorize)
}

// RemoveAuthorize revokes or deletes the authorization code.
func (s *storage) RemoveAuthorize(code string) error {
	// TODO: return no error if registry returns IsNotFound
	return s.authorizetoken.Delete(code, nil)
}

// SaveAccess writes AccessData.
// If RefreshToken is not blank, it must save in a way that can be loaded using LoadRefresh.
func (s *storage) SaveAccess(data *osin.AccessData) error {
	token, err := s.convertToAccessToken(data)
	if err != nil {
		return err
	}
	_, err = s.accesstoken.Create(token)
	return err
}

// LoadAccess retrieves access data by token. Client information MUST be loaded together.
// AuthorizeData and AccessData DON'T NEED to be loaded if not easily available.
// Optionally can return error if expired.
func (s *storage) LoadAccess(token string) (*osin.AccessData, error) {
	access, err := s.accesstoken.Get(token, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return s.convertFromAccessToken(access)
}

// RemoveAccess revokes or deletes an AccessData.
func (s *storage) RemoveAccess(token string) error {
	// TODO: return no error if registry returns IsNotFound
	return s.accesstoken.Delete(token, nil)
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

func (s *storage) convertToAuthorizeToken(data *osin.AuthorizeData) (*oauthapi.OAuthAuthorizeToken, error) {
	token := &oauthapi.OAuthAuthorizeToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:              data.Code,
			CreationTimestamp: metav1.Time{Time: data.CreatedAt},
		},
		CodeChallenge:       data.CodeChallenge,
		CodeChallengeMethod: data.CodeChallengeMethod,
		ClientName:          data.Client.GetId(),
		ExpiresIn:           int64(data.ExpiresIn),
		Scopes:              scope.Split(data.Scope),
		RedirectURI:         data.RedirectUri,
		State:               data.State,
	}
	if err := s.user.ConvertToAuthorizeToken(data.UserData, token); err != nil {
		return nil, err
	}
	return token, nil
}

func (s *storage) convertFromAuthorizeToken(authorize *oauthapi.OAuthAuthorizeToken) (*osin.AuthorizeData, error) {
	user, err := s.user.ConvertFromAuthorizeToken(authorize)
	if err != nil {
		return nil, err
	}
	client, err := s.client.Get(authorize.ClientName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if err := scopeauthorizer.ValidateScopeRestrictions(client, authorize.Scopes...); err != nil {
		return nil, err
	}

	return &osin.AuthorizeData{
		Code:                authorize.Name,
		CodeChallenge:       authorize.CodeChallenge,
		CodeChallengeMethod: authorize.CodeChallengeMethod,
		Client:              &clientWrapper{authorize.ClientName, client},
		ExpiresIn:           int32(authorize.ExpiresIn),
		Scope:               scope.Join(authorize.Scopes),
		RedirectUri:         authorize.RedirectURI,
		State:               authorize.State,
		CreatedAt:           authorize.CreationTimestamp.Time,
		UserData:            user,
	}, nil
}

func (s *storage) convertToAccessToken(data *osin.AccessData) (*oauthapi.OAuthAccessToken, error) {
	token := &oauthapi.OAuthAccessToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:              data.AccessToken,
			CreationTimestamp: metav1.Time{Time: data.CreatedAt},
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

	token.InactivityTimeoutSeconds = s.tokentimeout
	// Check if we have a client specific inactivity Timeout to set
	if w, ok := data.Client.(handlers.TokenTimeoutSeconds); ok {
		if tt := w.GetAccessTokenInactivityTimeoutSeconds(); tt != nil {
			token.InactivityTimeoutSeconds = *tt
		}
	}

	return token, nil
}

func (s *storage) convertFromAccessToken(access *oauthapi.OAuthAccessToken) (*osin.AccessData, error) {
	user, err := s.user.ConvertFromAccessToken(access)
	if err != nil {
		return nil, err
	}
	client, err := s.client.Get(access.ClientName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if err := scopeauthorizer.ValidateScopeRestrictions(client, access.Scopes...); err != nil {
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

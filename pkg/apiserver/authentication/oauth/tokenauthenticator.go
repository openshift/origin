package oauth

import (
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kauthenticator "k8s.io/apiserver/pkg/authentication/authenticator"
	kuser "k8s.io/apiserver/pkg/authentication/user"

	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	userclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

var errLookup = errors.New("token lookup failed")

type tokenAuthenticator struct {
	tokens       oauthclient.OAuthAccessTokenInterface
	users        userclient.UserInterface
	groupsMapper GroupsMapper
	validators   OAuthTokenValidator
}

func NewTokenAuthenticator(tokens oauthclient.OAuthAccessTokenInterface, users userclient.UserInterface, groupsMapper GroupsMapper, validators ...OAuthTokenValidator) kauthenticator.Token {
	return &tokenAuthenticator{
		tokens:       tokens,
		users:        users,
		groupsMapper: groupsMapper,
		validators:   OAuthTokenValidators(validators),
	}
}

func (a *tokenAuthenticator) AuthenticateToken(name string) (kuser.Info, bool, error) {
	token, err := a.tokens.Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, false, errLookup // mask the error so we do not leak token data in logs
	}

	user, err := a.users.Get(token.UserName, metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}

	if err := a.validators.Validate(token, user); err != nil {
		return nil, false, err
	}

	groups, err := a.groupsMapper.GroupsFor(token, user)
	if err != nil {
		return nil, false, err
	}

	return &kuser.DefaultInfo{
		Name:   user.Name,
		UID:    string(user.UID),
		Groups: groups,
		Extra: map[string][]string{
			authorizationapi.ScopesKey: token.Scopes,
		},
	}, true, nil
}

package registry

import (
	"errors"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuser "k8s.io/apiserver/pkg/authentication/user"

	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	oauthclient "github.com/openshift/origin/pkg/oauth/generated/internalclientset/typed/oauth/internalversion"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
)

type TokenAuthenticator struct {
	tokens      oauthclient.OAuthAccessTokenInterface
	users       userclient.UserResourceInterface
	groupMapper identitymapper.UserToGroupMapper
}

var ErrExpired = errors.New("Token is expired")

func NewTokenAuthenticator(tokens oauthclient.OAuthAccessTokenInterface, users userclient.UserResourceInterface, groupMapper identitymapper.UserToGroupMapper) *TokenAuthenticator {
	return &TokenAuthenticator{
		tokens:      tokens,
		users:       users,
		groupMapper: groupMapper,
	}
}

func (a *TokenAuthenticator) AuthenticateToken(value string) (kuser.Info, bool, error) {
	token, err := a.tokens.Get(value, metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}
	if token.CreationTimestamp.Time.Add(time.Duration(token.ExpiresIn) * time.Second).Before(time.Now()) {
		return nil, false, ErrExpired
	}
	if token.DeletionTimestamp != nil {
		return nil, false, ErrExpired
	}

	u, err := a.users.Get(token.UserName, metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}
	if string(u.UID) != token.UserUID {
		return nil, false, fmt.Errorf("user.UID (%s) does not match token.userUID (%s)", u.UID, token.UserUID)
	}

	groups, err := a.groupMapper.GroupsFor(u.Name)
	if err != nil {
		return nil, false, err
	}
	groupNames := []string{}
	for _, group := range groups {
		groupNames = append(groupNames, group.Name)
	}
	groupNames = append(groupNames, u.Groups...)

	return &kuser.DefaultInfo{
		Name:   u.Name,
		UID:    string(u.UID),
		Groups: groupNames,
		Extra: map[string][]string{
			authorizationapi.ScopesKey: token.Scopes,
		},
	}, true, nil
}

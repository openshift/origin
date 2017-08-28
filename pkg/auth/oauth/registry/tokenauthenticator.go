package registry

import (
	"errors"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kuser "k8s.io/apiserver/pkg/authentication/user"
	kapirequest "k8s.io/apiserver/pkg/endpoints/request"
)

type TokenAuthenticator struct {
	tokens      oauthaccesstoken.Registry
	users       userclient.UserResourceInterface
	groupMapper identitymapper.UserToGroupMapper
}

var ErrExpired = errors.New("Token is expired")

func NewTokenAuthenticator(tokens oauthaccesstoken.Registry, users userclient.UserResourceInterface, groupMapper identitymapper.UserToGroupMapper) *TokenAuthenticator {
	return &TokenAuthenticator{
		tokens:      tokens,
		users:       users,
		groupMapper: groupMapper,
	}
}

func (a *TokenAuthenticator) AuthenticateToken(value string) (kuser.Info, bool, error) {
	ctx := kapirequest.NewContext()

	token, err := a.tokens.GetAccessToken(ctx, value, &metav1.GetOptions{})
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

	// Build group membership for the user
	groupNames := sets.NewString()
	// 1. Groups from the identity provider associated with the token
	groupNames.Insert(token.IdentityProviderGroups...)
	// 2. Groups directly in the user object (deprecated)
	groupNames.Insert(u.Groups...)
	// 3. Group API objects the user is listed as a member in
	groups, err := a.groupMapper.GroupsFor(u.Name)
	if err != nil {
		return nil, false, err
	}
	for _, group := range groups {
		groupNames.Insert(group.Name)
	}

	return &kuser.DefaultInfo{
		Name:   u.Name,
		UID:    string(u.UID),
		Groups: groupNames.List(),
		Extra: map[string][]string{
			authorizationapi.ScopesKey: token.Scopes,
		},
	}, true, nil
}

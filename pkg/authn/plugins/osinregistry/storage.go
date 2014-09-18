package osinregistry

import (
	"errors"

	"github.com/openshift/origin/pkg/authn/api"

	oapi "github.com/openshift/origin/pkg/oauth/api"
)

type UserStorage struct {
}

func NewUserStorage() *UserStorage {
	return &UserStorage{}
}

func (s *UserStorage) ConvertToAuthorizeToken(user interface{}, token *oapi.AuthorizeToken) error {
	info, ok := user.(api.UserInfo)
	if !ok {
		return errors.New("did not receive UserInfo")
	}
	token.UserName = info.GetName()
	if token.UserName == "" {
		return errors.New("user name is empty")
	}
	token.UserUID = info.GetUID()
	return nil
}

func (s *UserStorage) ConvertToAccessToken(user interface{}, token *oapi.AccessToken) error {
	return s.ConvertToAuthorizeToken(user, &token.AuthorizeToken)
}

func (s *UserStorage) ConvertFromAuthorizeToken(token *oapi.AuthorizeToken) (interface{}, error) {
	if token.UserName == "" {
		return nil, errors.New("token has no user name stored")
	}
	return &api.DefaultUserInfo{
		Name: token.UserName,
		UID:  token.UserUID,
	}, nil
}

func (s *UserStorage) ConvertFromAccessToken(token *oapi.AccessToken) (interface{}, error) {
	return s.ConvertFromAuthorizeToken(&token.AuthorizeToken)
}

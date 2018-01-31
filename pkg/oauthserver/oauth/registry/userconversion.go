package registry

import (
	"errors"

	kuser "k8s.io/apiserver/pkg/authentication/user"

	oapi "github.com/openshift/api/oauth/v1"
)

type UserConversion struct{}

// NewUserConversion creates an object that can convert the user.Info object to and from
// an oauth access/authorize token object.
func NewUserConversion() *UserConversion {
	return &UserConversion{}
}

func (s *UserConversion) ConvertToAuthorizeToken(user interface{}, token *oapi.OAuthAuthorizeToken) error {
	info, ok := user.(kuser.Info)
	if !ok {
		return errors.New("did not receive user.Info")
	}
	token.UserName = info.GetName()
	if token.UserName == "" {
		return errors.New("user name is empty")
	}
	token.UserUID = info.GetUID()
	return nil
}

func (s *UserConversion) ConvertToAccessToken(user interface{}, token *oapi.OAuthAccessToken) error {
	info, ok := user.(kuser.Info)
	if !ok {
		return errors.New("did not receive user.Info")
	}
	token.UserName = info.GetName()
	if token.UserName == "" {
		return errors.New("user name is empty")
	}
	token.UserUID = info.GetUID()
	return nil
}

func (s *UserConversion) ConvertFromAuthorizeToken(token *oapi.OAuthAuthorizeToken) (interface{}, error) {
	if token.UserName == "" {
		return nil, errors.New("token has no user name stored")
	}
	return &kuser.DefaultInfo{
		Name: token.UserName,
		UID:  token.UserUID,
	}, nil
}

func (s *UserConversion) ConvertFromAccessToken(token *oapi.OAuthAccessToken) (interface{}, error) {
	if token.UserName == "" {
		return nil, errors.New("token has no user name stored")
	}
	return &kuser.DefaultInfo{
		Name: token.UserName,
		UID:  token.UserUID,
	}, nil
}

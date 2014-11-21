package requesthandlers

import (
	"net/http"

	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/authenticator"
)

type unionAuthRequestHandler []authenticator.Request

func NewUnionAuthentication(authRequestHandlers []authenticator.Request) authenticator.Request {
	ret := unionAuthRequestHandler(authRequestHandlers)
	return &ret
}

func (authHandler unionAuthRequestHandler) AuthenticateRequest(req *http.Request) (authapi.UserInfo, bool, error) {
	var errors kutil.ErrorList
	for _, currAuthRequestHandler := range authHandler {
		info, ok, err := currAuthRequestHandler.AuthenticateRequest(req)
		if err == nil && ok {
			return info, ok, err
		}
		if err != nil {
			errors = append(errors, err)
		}
	}

	return nil, false, errors.ToError()
}

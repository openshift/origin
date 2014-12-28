package unionrequest

import (
	"net/http"

	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/authenticator"
)

// TODO remove this in favor of kubernetes types

type unionAuthRequestHandler []authenticator.Request

// NewUnionAuthentication returns a request authenticator that validates credentials using a chain of authenticator.Request objects
func NewUnionAuthentication(authRequestHandlers []authenticator.Request) authenticator.Request {
	return unionAuthRequestHandler(authRequestHandlers)
}

// AuthenticateRequest authenticates the request using a chain of authenticator.Request objects.  The first
// success returns that identity.  Errors are only returned if no matches are found.
func (authHandler unionAuthRequestHandler) AuthenticateRequest(req *http.Request) (authapi.UserInfo, bool, error) {
	errors := []error{}
	for _, currAuthRequestHandler := range authHandler {
		info, ok, err := currAuthRequestHandler.AuthenticateRequest(req)
		if err == nil && ok {
			return info, ok, err
		}
		if err != nil {
			errors = append(errors, err)
		}
	}

	return nil, false, kutil.SliceToError(errors)
}

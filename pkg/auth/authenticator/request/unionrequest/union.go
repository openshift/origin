package unionrequest

import (
	"net/http"

	kerrors "k8s.io/kubernetes/pkg/util/errors"

	"github.com/openshift/origin/pkg/auth/authenticator"
	"k8s.io/kubernetes/pkg/auth/user"
)

// TODO remove this in favor of kubernetes types

type Authenticator struct {
	Handlers    []authenticator.Request
	FailOnError bool
}

// NewUnionAuthentication returns a request authenticator that validates credentials using a chain of authenticator.Request objects
func NewUnionAuthentication(authRequestHandlers ...authenticator.Request) authenticator.Request {
	return &Authenticator{Handlers: authRequestHandlers}
}

// AuthenticateRequest authenticates the request using a chain of authenticator.Request objects.  The first
// success returns that identity.  Errors are only returned if no matches are found.
func (authHandler *Authenticator) AuthenticateRequest(req *http.Request) (user.Info, bool, error) {
	errors := []error{}
	for _, currAuthRequestHandler := range authHandler.Handlers {
		info, ok, err := currAuthRequestHandler.AuthenticateRequest(req)
		if err == nil && ok {
			return info, ok, err
		}
		if err != nil {
			if authHandler.FailOnError {
				return nil, false, err
			}
			errors = append(errors, err)
		}
	}

	if len(errors) == 1 {
		// Avoid wrapping an error if possible
		return nil, false, errors[0]
	}
	return nil, false, kerrors.NewAggregate(errors)
}

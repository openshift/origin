package basicauthrequest

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"

	"github.com/openshift/origin/pkg/auth/authenticator"
)

type basicAuthRequestHandler struct {
	passwordAuthenticator authenticator.Password
}

func NewBasicAuthAuthentication(passwordAuthenticator authenticator.Password) authenticator.Request {
	return &basicAuthRequestHandler{passwordAuthenticator}
}

func (authHandler *basicAuthRequestHandler) AuthenticateRequest(req *http.Request) (user.Info, bool, error) {
	username, password, err := getBasicAuthInfo(req)
	if err != nil {
		return nil, false, err
	}

	return authHandler.passwordAuthenticator.AuthenticatePassword(username, password)
}

func getBasicAuthInfo(r *http.Request) (string, string, error) {
	// Retrieve the Authorization header and check whether it contains basic auth information
	const basicScheme string = "Basic "
	auth := r.Header.Get("Authorization")

	if !strings.HasPrefix(auth, basicScheme) {
		return "", "", nil
	}

	str, err := base64.StdEncoding.DecodeString(auth[len(basicScheme):])
	if err != nil {
		return "", "", errors.New("No valid base64 data in basic auth scheme found")
	}

	cred := strings.Split(string(str), ":")
	if len(cred) != 2 {
		return "", "", errors.New("Invalid Authorization header")
	}

	return cred[0], cred[1], nil
}

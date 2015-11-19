package anonymous

import (
	"net/http"

	"k8s.io/kubernetes/pkg/auth/authenticator"
	"k8s.io/kubernetes/pkg/auth/user"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

func NewAuthenticator() authenticator.Request {
	return authenticator.RequestFunc(func(req *http.Request) (user.Info, bool, error) {
		return &user.DefaultInfo{Name: bootstrappolicy.UnauthenticatedUsername, Groups: []string{bootstrappolicy.UnauthenticatedGroup}}, true, nil
	})
}

package passwordchallenger

import (
	"fmt"
	"net/http"

	oauthhandlers "github.com/openshift/origin/pkg/auth/oauth/handlers"
)

type basicPasswordAuthHandler struct {
	realm string
}

// CSRFTokenHeader must be passed when requesting a WWW-Authenticate Basic challenge to prevent CSRF attacks on browsers.
// The presence of this header indicates that a user agent intended to make a basic auth request (as opposed to a browser
// being tricked into requesting /oauth/authorize?response_type=token&client_id=openshift-challenging-client).
// Because multiple clients (oc, Java client, etc) are required to set this header, it probably should not be changed.
const CSRFTokenHeader = "X-CSRF-Token"

// NewBasicAuthChallenger returns a AuthenticationChallenger that responds with a basic auth challenge for the supplied realm
func NewBasicAuthChallenger(realm string) oauthhandlers.AuthenticationChallenger {
	return &basicPasswordAuthHandler{realm}
}

// AuthenticationChallenge returns a header that indicates a basic auth challenge for the supplied realm
func (h *basicPasswordAuthHandler) AuthenticationChallenge(req *http.Request) (http.Header, error) {
	headers := http.Header{}

	if len(req.Header.Get(CSRFTokenHeader)) == 0 {
		headers.Add("Warning",
			fmt.Sprintf(
				`%s %s "A non-empty %s header is required to receive basic-auth challenges"`,
				oauthhandlers.WarningHeaderMiscCode,
				oauthhandlers.WarningHeaderOpenShiftSource,
				CSRFTokenHeader,
			),
		)
	} else {
		headers.Add("WWW-Authenticate", fmt.Sprintf(`Basic realm="%s"`, h.realm))
	}

	return headers, nil
}

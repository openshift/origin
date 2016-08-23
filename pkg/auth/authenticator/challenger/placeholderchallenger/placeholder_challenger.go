package placeholderchallenger

import (
	"fmt"
	"net/http"
	"net/url"

	"strings"

	oauthhandlers "github.com/openshift/origin/pkg/auth/oauth/handlers"
)

// New returns an AuthenticationChallenger that responds with an interaction_required error and link to the web UI for requesting a token
// TODO better comment
func New(tokenRequestURLString string) (oauthhandlers.AuthenticationChallenger, error) {
	u, err := url.Parse(tokenRequestURLString)
	if err != nil {
		return nil, err
	}
	return &placeholderChallenger{*u}, nil
}

type placeholderChallenger struct {
	tokenRequestURL url.URL
}

func (c *placeholderChallenger) AuthenticationChallenge(req *http.Request) (http.Header, error) {
	headers := http.Header{}

	q := req.URL.Query()
	redirect := q.Get("redirect_uri")
	state := q.Get("state")
	if q.Get("display") == "page" && len(state) > 0 && strings.HasPrefix(redirect, "http://127.0.0.1:") { // TODO fix
		headers.Add("Location", fmt.Sprintf("authorize?client_id=openshift-browser-client&redirect_uri=%s&response_type=code&state=%s", redirect, state)) // TODO fix, state
	} else {
		copy := c.tokenRequestURL
		cp := &copy
		query := cp.Query()

		// TODO should we leave this as-is?
		errorDescription := fmt.Sprintf(
			`%s %s "You must obtain an API token by visiting %s"`,
			oauthhandlers.WarningHeaderMiscCode,
			oauthhandlers.WarningHeaderOpenShiftSource,
			c.tokenRequestURL.String(),
		)
		query.Set("error", "interaction_required")
		query.Set("error_description", errorDescription)
		cp.RawQuery = query.Encode()
		headers.Add("Location", cp.String())
	}

	return headers, nil
}

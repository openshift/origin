package placeholderchallenger

import (
	"fmt"
	"net/url"

	"github.com/openshift/origin/pkg/auth/authenticator/redirector"
	oauthhandlers "github.com/openshift/origin/pkg/auth/oauth/handlers"
)

// New returns an AuthenticationChallenger that responds with an interaction_required error and link to the web UI for requesting a token
func New(urlString string) (oauthhandlers.AuthenticationChallenger, error) {
	// TODO should we leave this as-is?
	errorDescription := fmt.Sprintf(
		`%s %s "You must obtain an API token by visiting %s"`,
		oauthhandlers.WarningHeaderMiscCode,
		oauthhandlers.WarningHeaderOpenShiftSource,
		urlString,
	)
	u, err := url.Parse(urlString)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("error", "interaction_required")
	q.Set("error_description", errorDescription)
	u.RawQuery = q.Encode()
	return redirector.NewChallenger(nil, u.String()), nil
}

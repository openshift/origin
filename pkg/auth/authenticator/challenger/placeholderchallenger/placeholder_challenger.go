package placeholderchallenger

import (
	"fmt"

	"github.com/openshift/origin/pkg/auth/authenticator/redirector"
	oauthhandlers "github.com/openshift/origin/pkg/auth/oauth/handlers"
)

// New returns an AuthenticationChallenger that responds with an error and link to the web UI for requesting a token
func New(url string) oauthhandlers.AuthenticationChallenger {
	errorDescription := fmt.Sprintf(
		`%s %s "You must obtain an API token by visiting %s"`,
		oauthhandlers.WarningHeaderMiscCode,
		oauthhandlers.WarningHeaderOpenShiftSource,
		url,
	)
	return redirector.NewChallenger(nil, url+"?error=interaction_required&error_description="+errorDescription)
}

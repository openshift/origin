package googlecallback

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/golang/glog"
)

type GoogleAuthenticationHandler struct {
	RedirectUri string
	ClientId    string
}

func (g *GoogleAuthenticationHandler) AuthenticationNeeded(w http.ResponseWriter, req *http.Request) {
	glog.V(1).Infof("Authentication needed for %v", g)

	// TODO: tidy this
	// build our url:
	oauthUrl, _ := url.Parse("https://accounts.google.com/o/oauth2/auth")
	query := url.Values{}
	query.Set("response_type", "code")
	query.Set("client_id", g.ClientId)
	query.Set("redirect_uri", g.RedirectUri)
	query.Set("scope", "profile email")
	query.Set("state", "go-to-somewhere")
	query.Set("include_granted_scopes", "true")
	query.Set("access_type", "offline")
	oauthUrl.RawQuery = query.Encode()

	glog.V(1).Infof("redirect to  %v", oauthUrl)

	http.Redirect(w, req, oauthUrl.String(), http.StatusFound)
}
func (g *GoogleAuthenticationHandler) AuthenticationError(err error, w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "<body>AuthenticationError - %s</body>", err)
}

func (g *GoogleAuthenticationHandler) String() string {
	return fmt.Sprintf("GoogleAuthenticationHandler{ClientId: %v}", g.ClientId)
}

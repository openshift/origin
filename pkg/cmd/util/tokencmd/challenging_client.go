package tokencmd

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	kclient "k8s.io/kubernetes/pkg/client"

	"github.com/openshift/origin/pkg/cmd/util"
)

// CSRFTokenHeader is a marker header that indicates we are not a browser that got tricked into requesting basic auth
// Corresponds to the header expected by basic-auth challenging authenticators
const CSRFTokenHeader = "X-CSRF-Token"

// challengingClient conforms the kclient.HTTPClient interface.  It introspects responses for auth challenges and
// tries to response to those challenges in order to get a token back.
type challengingClient struct {
	delegate        *http.Client
	reader          io.Reader
	defaultUsername string
	defaultPassword string
}

const basicAuthPattern = `[\s]*Basic[\s]*realm="([\w]+)"`

var basicAuthRegex = regexp.MustCompile(basicAuthPattern)

// Do watches for unauthorized challenges.  If we know to respond, we respond to the challenge
func (client *challengingClient) Do(req *http.Request) (*http.Response, error) {
	// Set custom header required by server to avoid CSRF attacks on browsers using basic auth
	if req.Header == nil {
		req.Header = http.Header{}
	}
	req.Header.Set(CSRFTokenHeader, "1")

	resp, err := client.delegate.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		if wantsBasicAuth, realm := isBasicAuthChallenge(resp); wantsBasicAuth {
			username := client.defaultUsername
			password := client.defaultPassword

			missingUsername := len(username) == 0
			missingPassword := len(password) == 0

			url := *req.URL
			url.Path, url.RawQuery, url.Fragment = "", "", ""

			if (missingUsername || missingPassword) && client.reader != nil {
				fmt.Printf("Authentication required for %s (%s)\n", &url, realm)
				if missingUsername {
					username = util.PromptForString(client.reader, "Username: ")
				}
				if missingPassword {
					password = util.PromptForPasswordString(client.reader, "Password: ")
				}
			}

			if len(username) > 0 || len(password) > 0 {
				client.delegate.Transport = kclient.NewBasicAuthRoundTripper(username, password, client.delegate.Transport)
				return client.delegate.Do(resp.Request)
			}
		}
	}
	return resp, err
}

func isBasicAuthChallenge(resp *http.Response) (bool, string) {
	for currHeader, headerValue := range resp.Header {
		if strings.EqualFold(currHeader, "WWW-Authenticate") {
			for _, currAuthorizeHeader := range headerValue {
				if matches := basicAuthRegex.FindAllStringSubmatch(currAuthorizeHeader, 1); matches != nil {
					return true, matches[0][1]
				}
			}
		}
	}

	return false, ""
}

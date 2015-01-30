package tokencmd

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"

	"github.com/openshift/origin/pkg/auth/server/tokenrequest"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	accessTokenRedirectPattern = `#access_token=([\w]+)&`
)

var (
	accessTokenRedirectRegex = regexp.MustCompile(accessTokenRedirectPattern)
)

type tokenGetterInfo struct {
	accessToken string
}

// RequestToken uses the cmd arguments to locate an openshift oauth server and attempts to authenticate
// it returns the access token if it gets one.  An error if it does not
func RequestToken(clientCfg *clientcmd.Config, reader io.Reader) (string, error) {
	tokenGetter := &tokenGetterInfo{}

	osCfg := clientCfg.OpenShiftConfig()
	osClient, err := client.New(osCfg)
	if err != nil {
		return "", err
	}

	// get the transport, so that we can use it to build our own client that wraps it
	// our client understands certain challenges and can respond to them
	clientTransport, err := kclient.TransportFor(osCfg)
	if err != nil {
		return "", err
	}
	httpClient := &http.Client{
		Transport:     clientTransport,
		CheckRedirect: tokenGetter.checkRedirect,
	}
	osClient.Client = &challengingClient{httpClient, reader}

	_ = osClient.Get().AbsPath("oauth", "authorize").Param("response_type", "token").Param("client_id", "openshift-challenging-client").Do()

	if len(tokenGetter.accessToken) == 0 {
		requestTokenURL := osCfg.Host + "/oauth" /* clean up after auth.go dies */ + tokenrequest.RequestTokenEndpoint
		return "", errors.New("Unable to get token.  Try visiting " + requestTokenURL + " for a new token.")
	}

	return tokenGetter.accessToken, nil
}

// checkRedirect watches the redirects to see if any contain the access_token anchor.  It then stores the value of the access token for later retrieval
func (tokenGetter *tokenGetterInfo) checkRedirect(req *http.Request, via []*http.Request) error {
	// if we're redirected with an access token in the anchor, use it to set our transport to a proper bearer auth
	if matches := accessTokenRedirectRegex.FindAllStringSubmatch(req.URL.String(), 1); matches != nil {
		tokenGetter.accessToken = matches[0][1]
	}

	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}

	return nil
}

// challengingClient conforms the kclient.HTTPClient interface.  It introspects responses for auth challenges and
// tries to response to those challenges in order to get a token back.
type challengingClient struct {
	delegate *http.Client
	reader   io.Reader
}

const (
	basicAuthPattern = `[\s]*Basic[\s]*realm="([\w]+)"`
)

var (
	basicAuthRegex = regexp.MustCompile(basicAuthPattern)
)

// Do watches for unauthorized challenges.  If we know to respond, we respond to the challenge
func (client *challengingClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := client.delegate.Do(req)
	if resp.StatusCode == http.StatusUnauthorized {
		if wantsBasicAuth, realm := isBasicAuthChallenge(resp); wantsBasicAuth {
			fmt.Printf("Authenticate for \"%v\"\n", realm)
			username := promptForString("username", client.reader)
			password := promptForString("password", client.reader)

			client.delegate.Transport = kclient.NewBasicAuthRoundTripper(username, password, client.delegate.Transport)
			return client.Do(resp.Request)
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

func promptForString(field string, r io.Reader) string {
	fmt.Printf("Please enter %s: ", field)
	var result string
	fmt.Fscan(r, &result)
	return result
}

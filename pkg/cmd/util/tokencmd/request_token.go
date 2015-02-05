package tokencmd

import (
	"errors"
	"io"
	"net/http"
	"regexp"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/auth/server/tokenrequest"
	"github.com/openshift/origin/pkg/client"
)

const accessTokenRedirectPattern = `#access_token=([\w]+)&`

var accessTokenRedirectRegex = regexp.MustCompile(accessTokenRedirectPattern)

type tokenGetterInfo struct {
	accessToken string
}

// RequestToken uses the cmd arguments to locate an openshift oauth server and attempts to authenticate
// it returns the access token if it gets one.  An error if it does not
func RequestToken(clientCfg *kclient.Config, reader io.Reader, defaultUsername string, defaultPassword string) (string, error) {
	tokenGetter := &tokenGetterInfo{}

	osClient, err := client.New(clientCfg)
	if err != nil {
		return "", err
	}

	// get the transport, so that we can use it to build our own client that wraps it
	// our client understands certain challenges and can respond to them
	clientTransport, err := kclient.TransportFor(clientCfg)
	if err != nil {
		return "", err
	}

	httpClient := &http.Client{
		Transport:     clientTransport,
		CheckRedirect: tokenGetter.checkRedirect,
	}

	osClient.Client = &challengingClient{httpClient, reader, defaultUsername, defaultPassword}

	result := osClient.Get().AbsPath("oauth", "authorize").Param("response_type", "token").Param("client_id", "openshift-challenging-client").Do()

	if len(tokenGetter.accessToken) == 0 {
		if result.Error() != nil {
			glog.Errorf("Error making server request: %v", result.Error())
		}

		requestTokenURL := clientCfg.Host + "/oauth" /* clean up after auth.go dies */ + tokenrequest.RequestTokenEndpoint
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

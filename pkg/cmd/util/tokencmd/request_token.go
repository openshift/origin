package tokencmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/RangelReale/osincli"
	"github.com/golang/glog"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/oauth/server/osinserver"
)

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

	result := osClient.Get().AbsPath("/oauth", osinserver.AuthorizePath).
		Param("response_type", "token").
		Param("client_id", "openshift-challenging-client").
		Do()
	if err := result.Error(); err != nil && !isRedirectError(err) {
		return "", err
	}

	if len(tokenGetter.accessToken) == 0 {
		r, _ := result.Raw()
		if description, ok := rawOAuthJSONErrorDescription(r); ok {
			return "", fmt.Errorf("cannot retrieve a token: %s", description)
		}
		glog.V(4).Infof("A request token could not be created, server returned: %s", string(r))
		return "", fmt.Errorf("the server did not return a token (possible server error)")
	}

	return tokenGetter.accessToken, nil
}

func rawOAuthJSONErrorDescription(data []byte) (string, bool) {
	output := osincli.ResponseData{}
	decoder := json.NewDecoder(bytes.NewBuffer(data))
	if err := decoder.Decode(&output); err != nil {
		return "", false
	}
	if _, ok := output["error"]; !ok {
		return "", false
	}
	desc, ok := output["error_description"]
	if !ok {
		return "", false
	}
	s, ok := desc.(string)
	if !ok || len(s) == 0 {
		return "", false
	}
	return s, true
}

const accessTokenKey = "access_token"

var errRedirectComplete = errors.New("found access token")

type tokenGetterInfo struct {
	accessToken string
}

// checkRedirect watches the redirects to see if any contain the access_token anchor.  It then stores the value of the access token for later retrieval
func (tokenGetter *tokenGetterInfo) checkRedirect(req *http.Request, via []*http.Request) error {
	fragment := req.URL.Fragment
	if values, err := url.ParseQuery(fragment); err == nil {
		if v, ok := values[accessTokenKey]; ok {
			if len(v) > 0 {
				tokenGetter.accessToken = v[0]
			}
			return errRedirectComplete
		}
	}

	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}

	return nil
}

func isRedirectError(err error) bool {
	if err == errRedirectComplete {
		return true
	}
	switch t := err.(type) {
	case *url.Error:
		return t.Err == errRedirectComplete
	}
	return false
}

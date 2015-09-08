package tokencmd

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"k8s.io/kubernetes/pkg/api"
	apierrs "k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/util"
)

// CSRFTokenHeader is a marker header that indicates we are not a browser that got tricked into requesting basic auth
// Corresponds to the header expected by basic-auth challenging authenticators
const CSRFTokenHeader = "X-CSRF-Token"

// RequestToken uses the cmd arguments to locate an openshift oauth server and attempts to authenticate
// it returns the access token if it gets one.  An error if it does not
func RequestToken(clientCfg *kclient.Config, reader io.Reader, defaultUsername string, defaultPassword string) (string, error) {
	challengeHandler := &BasicChallengeHandler{
		Host:     clientCfg.Host,
		Reader:   reader,
		Username: defaultUsername,
		Password: defaultPassword,
	}

	rt, err := kclient.TransportFor(clientCfg)
	if err != nil {
		return "", err
	}

	// requestURL holds the current URL to make requests to. This can change if the server responds with a redirect
	requestURL := clientCfg.Host + "/oauth/authorize?response_type=token&client_id=openshift-challenging-client"
	// requestHeaders holds additional headers to add to the request. This can be changed by challengeHandlers
	requestHeaders := http.Header{}
	// requestedURLSet/requestedURLList hold the URLs we have requested, to prevent redirect loops. Gets reset when a challenge is handled.
	requestedURLSet := util.NewStringSet()
	requestedURLList := []string{}

	for {
		// Make the request
		resp, err := request(rt, requestURL, requestHeaders)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized {
			if resp.Header.Get("WWW-Authenticate") != "" {
				if !challengeHandler.CanHandle(resp.Header) {
					return "", apierrs.NewUnauthorized("unhandled challenge")
				}
				// Handle a challenge
				newRequestHeaders, shouldRetry, err := challengeHandler.HandleChallenge(resp.Header)
				if err != nil {
					return "", err
				}
				if !shouldRetry {
					return "", apierrs.NewUnauthorized("challenger chose not to retry the request")
				}

				// Reset request set/list. Since we're setting different headers, it is legitimate to request the same urls
				requestedURLSet = util.NewStringSet()
				requestedURLList = []string{}
				// Use the response to the challenge as the new headers
				requestHeaders = newRequestHeaders
				continue
			}

			// Unauthorized with no challenge
			unauthorizedError := apierrs.NewUnauthorized("")
			// Attempt to read body content and include as an error detail
			if details, err := ioutil.ReadAll(resp.Body); err == nil && len(details) > 0 {
				unauthorizedError.(*apierrs.StatusError).ErrStatus.Details = &api.StatusDetails{
					Causes: []api.StatusCause{
						{Message: string(details)},
					},
				}
			}

			return "", unauthorizedError
		}

		if resp.StatusCode == http.StatusFound {
			redirectURL := resp.Header.Get("Location")

			// OAuth response case (access_token or error parameter)
			accessToken, err := oauthAuthorizeResult(redirectURL)
			if err != nil {
				return "", err
			}
			if len(accessToken) > 0 {
				return accessToken, err
			}

			// Non-OAuth response, just follow the URL
			// add to our list of redirects
			requestedURLList = append(requestedURLList, redirectURL)
			// detect loops
			if !requestedURLSet.Has(redirectURL) {
				requestedURLSet.Insert(redirectURL)
				requestURL = redirectURL
				continue
			}
			return "", apierrs.NewInternalError(fmt.Errorf("redirect loop: %s", strings.Join(requestedURLList, " -> ")))
		}

		// Unknown response
		return "", apierrs.NewInternalError(fmt.Errorf("unexpected response: %d", resp.StatusCode))
	}
}

func oauthAuthorizeResult(location string) (string, error) {
	u, err := url.Parse(location)
	if err != nil {
		return "", err
	}

	if errorCode := u.Query().Get("error"); len(errorCode) > 0 {
		errorDescription := u.Query().Get("error_description")
		return "", errors.New(errorCode + " " + errorDescription)
	}

	fragmentValues, err := url.ParseQuery(u.Fragment)
	if err != nil {
		return "", err
	}

	if accessToken := fragmentValues.Get("access_token"); len(accessToken) > 0 {
		return accessToken, nil
	}

	return "", nil
}

func request(rt http.RoundTripper, requestURL string, requestHeaders http.Header) (*http.Response, error) {
	// Build the request
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range requestHeaders {
		req.Header[k] = v
	}
	req.Header.Set(CSRFTokenHeader, "1")

	// Make the request
	return rt.RoundTrip(req)
}

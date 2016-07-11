package tokencmd

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/golang/glog"

	apierrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/util/sets"
)

// CSRFTokenHeader is a marker header that indicates we are not a browser that got tricked into requesting basic auth
// Corresponds to the header expected by basic-auth challenging authenticators
const CSRFTokenHeader = "X-CSRF-Token"

// ChallengeHandler handles responses to WWW-Authenticate challenges.
type ChallengeHandler interface {
	// CanHandle returns true if the handler recognizes a challenge it thinks it can handle.
	CanHandle(headers http.Header) bool
	// HandleChallenge lets the handler attempt to handle a challenge.
	// It is only invoked if CanHandle() returned true for the given headers.
	// Returns response headers and true if the challenge is successfully handled.
	// Returns false if the challenge was not handled, and an optional error in error cases.
	HandleChallenge(requestURL string, headers http.Header) (http.Header, bool, error)
	// CompleteChallenge is invoked with the headers from a successful server response
	// received after having handled one or more challenges.
	// Returns an error if the handler does not consider the challenge/response interaction complete.
	CompleteChallenge(requestURL string, headers http.Header) error
	// Release gives the handler a chance to release any resources held during a challenge/response sequence.
	// It is always invoked, even in cases where no challenges were received or handled.
	Release() error
}

type RequestTokenOptions struct {
	ClientConfig *restclient.Config
	Handler      ChallengeHandler
}

// RequestToken uses the cmd arguments to locate an openshift oauth server and attempts to authenticate
// it returns the access token if it gets one.  An error if it does not
func RequestToken(clientCfg *restclient.Config, reader io.Reader, defaultUsername string, defaultPassword string) (string, error) {
	handlers := []ChallengeHandler{}
	if GSSAPIEnabled() {
		handlers = append(handlers, NewNegotiateChallengeHandler(NewGSSAPINegotiator(defaultUsername)))
	}
	if BasicEnabled() {
		handlers = append(handlers, &BasicChallengeHandler{Host: clientCfg.Host, Reader: reader, Username: defaultUsername, Password: defaultPassword})
	}

	var handler ChallengeHandler
	if len(handlers) == 1 {
		handler = handlers[0]
	} else {
		handler = NewMultiHandler(handlers...)
	}

	opts := &RequestTokenOptions{
		ClientConfig: clientCfg,
		Handler:      handler,
	}

	return opts.RequestToken()
}

// RequestToken locates an openshift oauth server and attempts to authenticate.
// It returns the access token if it gets one, or an error if it does not.
// It should only be invoked once on a given RequestTokenOptions instance.
// The Handler held by the options is released as part of this call.
func (o *RequestTokenOptions) RequestToken() (string, error) {
	defer func() {
		// Always release the handler
		if err := o.Handler.Release(); err != nil {
			// Release errors shouldn't fail the token request, just log
			glog.V(4).Infof("error releasing handler: %v", err)
		}
	}()

	rt, err := restclient.TransportFor(o.ClientConfig)
	if err != nil {
		return "", err
	}

	// requestURL holds the current URL to make requests to. This can change if the server responds with a redirect
	requestURL := o.ClientConfig.Host + "/oauth/authorize?response_type=token&client_id=openshift-challenging-client"
	// requestHeaders holds additional headers to add to the request. This can be changed by o.Handlers
	requestHeaders := http.Header{}
	// requestedURLSet/requestedURLList hold the URLs we have requested, to prevent redirect loops. Gets reset when a challenge is handled.
	requestedURLSet := sets.NewString()
	requestedURLList := []string{}
	handledChallenge := false

	for {
		// Make the request
		resp, err := request(rt, requestURL, requestHeaders)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized {
			if resp.Header.Get("WWW-Authenticate") != "" {
				if !o.Handler.CanHandle(resp.Header) {
					return "", apierrs.NewUnauthorized("unhandled challenge")
				}
				// Handle the challenge
				newRequestHeaders, shouldRetry, err := o.Handler.HandleChallenge(requestURL, resp.Header)
				if err != nil {
					return "", err
				}
				if !shouldRetry {
					return "", apierrs.NewUnauthorized("challenger chose not to retry the request")
				}
				// Remember if we've ever handled a challenge
				handledChallenge = true

				// Reset request set/list. Since we're setting different headers, it is legitimate to request the same urls
				requestedURLSet = sets.NewString()
				requestedURLList = []string{}
				// Use the response to the challenge as the new headers
				requestHeaders = newRequestHeaders
				continue
			}

			// Unauthorized with no challenge
			unauthorizedError := apierrs.NewUnauthorized("")
			// Attempt to read body content and include as an error detail
			if details, err := ioutil.ReadAll(resp.Body); err == nil && len(details) > 0 {
				unauthorizedError.ErrStatus.Details = &unversioned.StatusDetails{
					Causes: []unversioned.StatusCause{
						{Message: string(details)},
					},
				}
			}

			return "", unauthorizedError
		}

		// if we've ever handled a challenge, see if the handler also considers the interaction complete.
		// this is required for negotiate flows with mutual authentication.
		if handledChallenge {
			if err := o.Handler.CompleteChallenge(requestURL, resp.Header); err != nil {
				return "", err
			}
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

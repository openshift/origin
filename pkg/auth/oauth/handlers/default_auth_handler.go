package handlers

import (
	"fmt"
	"net/http"

	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"

	authapi "github.com/openshift/origin/pkg/auth/api"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
)

// unionAuthenticationHandler is an oauth.AuthenticationHandler that muxes multiple challenge handlers and redirect handlers
type unionAuthenticationHandler struct {
	challengers  map[string]AuthenticationChallenger
	redirectors  map[string]AuthenticationRedirector
	errorHandler AuthenticationErrorHandler
}

// NewUnionAuthenticationHandler returns an oauth.AuthenticationHandler that muxes multiple challenge handlers and redirect handlers
func NewUnionAuthenticationHandler(passedChallengers map[string]AuthenticationChallenger, passedRedirectors map[string]AuthenticationRedirector, errorHandler AuthenticationErrorHandler) AuthenticationHandler {
	challengers := passedChallengers
	if challengers == nil {
		challengers = make(map[string]AuthenticationChallenger, 1)
	}

	redirectors := passedRedirectors
	if redirectors == nil {
		redirectors = make(map[string]AuthenticationRedirector, 1)
	}

	return &unionAuthenticationHandler{challengers, redirectors, errorHandler}
}

// AuthenticationNeeded looks at the oauth Client to determine whether it wants try to authenticate with challenges or using a redirect path
// If the client wants a challenge path, it muxes together all the different challenges from the challenge handlers
// If (the client wants a redirect path) and ((there is one redirect handler) or (a redirect handler was requested via the "useRedirectHandler" parameter),
// then the redirect handler is called.  Otherwise, you get an error (currently) or a redirect to a page letting you choose how you'd like to authenticate.
// It returns whether the response was written and/or an error
func (authHandler *unionAuthenticationHandler) AuthenticationNeeded(apiClient authapi.Client, w http.ResponseWriter, req *http.Request) (bool, error) {
	client, ok := apiClient.GetUserData().(*oauthapi.OAuthClient)
	if !ok {
		return false, fmt.Errorf("apiClient data was not an oauthapi.OAuthClient")
	}

	if client.RespondWithChallenges {
		errors := []error{}
		headers := http.Header(make(map[string][]string))
		for _, challengingHandler := range authHandler.challengers {
			currHeaders, err := challengingHandler.AuthenticationChallenge(req)
			if err != nil {
				errors = append(errors, err)
				continue
			}

			// merge header values
			mergeHeaders(headers, currHeaders)
		}

		if len(headers) > 0 {
			mergeHeaders(w.Header(), headers)
			w.WriteHeader(http.StatusUnauthorized)
			return true, nil

		}
		return false, kerrors.NewAggregate(errors)

	}

	redirectHandlerName := req.URL.Query().Get("useRedirectHandler")

	if len(redirectHandlerName) > 0 {
		redirectHandler := authHandler.redirectors[redirectHandlerName]
		if redirectHandler == nil {
			return false, fmt.Errorf("Unable to locate redirect handler: %v", redirectHandlerName)
		}

		err := redirectHandler.AuthenticationRedirect(w, req)
		if err != nil {
			return authHandler.errorHandler.AuthenticationError(err, w, req)
		}
		return true, nil

	}

	if (len(authHandler.redirectors)) == 1 {
		// there has to be a better way
		for _, redirectHandler := range authHandler.redirectors {
			err := redirectHandler.AuthenticationRedirect(w, req)
			if err != nil {
				return authHandler.errorHandler.AuthenticationError(err, w, req)
			}
			return true, nil
		}
	} else if len(authHandler.redirectors) > 1 {
		// TODO this clearly doesn't work right.  There should probably be a redirect to an interstitial page.
		// however, this is just as good as we have now.
		return false, fmt.Errorf("Too many potential redirect handlers: %v", authHandler.redirectors)
	}

	return false, nil
}

func mergeHeaders(dest http.Header, toAdd http.Header) {
	for key, values := range toAdd {
		for _, value := range values {
			dest.Add(key, value)
		}
	}
}

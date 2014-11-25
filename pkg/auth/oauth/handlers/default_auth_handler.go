package handlers

import (
	"fmt"
	"net/http"

	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authapi "github.com/openshift/origin/pkg/auth/api"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
)

// unionAuthenticationHandler is an oauth.AuthenticationHandler that muxes multiple challange handlers and redirect handlers
type unionAuthenticationHandler struct {
	challengeHandlers map[string]ChallengeAuthHandler
	redirectHandlers  map[string]RedirectAuthHandler
	errorHandler      AuthenticationErrorHandler
}

// NewUnionAuthenticationHandler returns an oauth.AuthenticationHandler that muxes multiple challange handlers and redirect handlers
func NewUnionAuthenticationHandler(challengeHandlers map[string]ChallengeAuthHandler, redirectHandlers map[string]RedirectAuthHandler, errorHandler AuthenticationErrorHandler) AuthenticationHandler {
	challengers := challengeHandlers
	if challengeHandlers == nil {
		challengers = make(map[string]ChallengeAuthHandler, 1)
	}

	redirectors := redirectHandlers
	if redirectHandlers == nil {
		redirectors = make(map[string]RedirectAuthHandler, 1)
	}

	return &unionAuthenticationHandler{challengers, redirectors, errorHandler}
}

// AuthenticationNeeded looks at the oauth Client to determine whether it wants try to authenticate with challenges or using a redirect path
// If the client wants a challenge path, it muxes together all the different challenges from the challenge handlers
// If (the client wants a redirect path) and ((there is one redirect handler) or (a redirect handler was requested via the "useRedirectHandler" parameter),
// then the redirect handler is called.  Otherwise, you get an error (currently) or a redirect to a page letting you choose how you'd like to authenticate.
// It returns whether the response was written and/or an error
func (authHandler *unionAuthenticationHandler) AuthenticationNeeded(apiClient authapi.Client, w http.ResponseWriter, req *http.Request) (bool, error) {
	client, ok := apiClient.GetUserData().(*oauthapi.Client)
	if !ok {
		return false, fmt.Errorf("apiClient data was not an oauthapi.Client")
	}

	if client.RespondWithChallenges {
		var errors kutil.ErrorList
		headers := http.Header(make(map[string][]string))
		for _, challengingHandler := range authHandler.challengeHandlers {
			currHeaders, err := challengingHandler.AuthenticationChallengeNeeded(req)
			if err != nil {
				errors = append(errors, err)
				continue
			}

			// merge header values
			mergeHeaders(currHeaders, headers)
		}

		if len(headers) > 0 {
			mergeHeaders(headers, w.Header())
			w.WriteHeader(http.StatusUnauthorized)
			return true, nil

		} else {
			return false, errors.ToError()
		}

	} else {
		redirectHandlerName := req.URL.Query().Get("useRedirectHandler")

		if len(redirectHandlerName) > 0 {
			redirectHandler := authHandler.redirectHandlers[redirectHandlerName]
			if redirectHandler == nil {
				return false, fmt.Errorf("Unable to locate redirect handler: %v", redirectHandlerName)
			}

			err := redirectHandler.AuthenticationRedirectNeeded(w, req)
			if err != nil {
				return authHandler.errorHandler.AuthenticationError(err, w, req)
			}
			return true, nil

		} else {
			if (len(authHandler.redirectHandlers)) == 1 {
				// there has to be a better way
				for _, redirectHandler := range authHandler.redirectHandlers {
					err := redirectHandler.AuthenticationRedirectNeeded(w, req)
					if err != nil {
						return authHandler.errorHandler.AuthenticationError(err, w, req)
					}
					return true, nil
				}
			} else if len(authHandler.redirectHandlers) > 1 {
				// TODO this clearly doesn't work right.  There should probably be a redirect to an interstitial page.
				// however, this is just as good as we have now.
				return false, fmt.Errorf("Too many potential redirect handlers: %v", authHandler.redirectHandlers)
			}

		}
	}

	return false, nil
}

func mergeHeaders(toAdd http.Header, dest http.Header) {
	for key, values := range toAdd {
		for _, value := range values {
			dest.Add(key, value)
		}
	}
}

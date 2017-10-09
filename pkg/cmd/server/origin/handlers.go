package origin

import (
	"net/http"
	"regexp"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/sets"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	serverhandlers "github.com/openshift/origin/pkg/cmd/server/handlers"
)

// cacheControlFilter sets the Cache-Control header to the specified value.
func cacheControlFilter(handler http.Handler, value string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", value)
		handler.ServeHTTP(w, req)
	})
}

// namespacingFilter adds a filter that adds the namespace of the request to the context.  Not all requests will have namespaces,
// but any that do will have the appropriate value added.
func namespacingFilter(handler http.Handler, contextMapper apirequest.RequestContextMapper) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx, ok := contextMapper.Get(req)
		if !ok {
			http.Error(w, "Unable to find request context", http.StatusInternalServerError)
			return
		}

		if _, exists := apirequest.NamespaceFrom(ctx); !exists {
			if requestInfo, ok := apirequest.RequestInfoFrom(ctx); ok && requestInfo != nil {
				// only set the namespace if the apiRequestInfo was resolved
				// keep in mind that GetAPIRequestInfo will fail on non-api requests, so don't fail the entire http request on that
				// kind of failure.

				// TODO reconsider special casing this.  Having the special case hereallow us to fully share the kube
				// APIRequestInfoResolver without any modification or customization.
				namespace := requestInfo.Namespace
				if (requestInfo.Resource == "projects") && (len(requestInfo.Name) > 0) {
					namespace = requestInfo.Name
				}

				ctx = apirequest.WithNamespace(ctx, namespace)
				contextMapper.Update(req, ctx)
			}
		}

		handler.ServeHTTP(w, req)
	})
}

type userAgentFilter struct {
	regex   *regexp.Regexp
	message string
	verbs   sets.String
}

func newUserAgentFilter(config configapi.UserAgentMatchRule) (userAgentFilter, error) {
	regex, err := regexp.Compile(config.Regex)
	if err != nil {
		return userAgentFilter{}, err
	}
	userAgentFilter := userAgentFilter{regex: regex, verbs: sets.NewString(config.HTTPVerbs...)}

	return userAgentFilter, nil
}

func (f *userAgentFilter) matches(verb, userAgent string) bool {
	if len(f.verbs) > 0 && !f.verbs.Has(verb) {
		return false
	}

	return f.regex.MatchString(userAgent)
}

// versionSkewFilter adds a filter that may deny requests from skewed
// oc clients, since we know that those clients will strip unknown fields which can lead to unexpected outcomes
func (c *MasterConfig) versionSkewFilter(handler http.Handler, contextMapper apirequest.RequestContextMapper) http.Handler {
	filterConfig := c.Options.PolicyConfig.UserAgentMatchingConfig
	if len(filterConfig.RequiredClients) == 0 && len(filterConfig.DeniedClients) == 0 {
		return handler
	}

	defaultMessage := filterConfig.DefaultRejectionMessage
	if len(defaultMessage) == 0 {
		defaultMessage = "the cluster administrator has disabled access for this client, please upgrade or consult your administrator"
	}

	// the structure of the legacyClientPolicyConfig is pretty easy to write, but its inefficient to use at runtime
	// pre-process the config elements to make a more efficicent structure.
	allowedFilters := []userAgentFilter{}
	deniedFilters := []userAgentFilter{}
	for _, config := range filterConfig.RequiredClients {
		userAgentFilter, err := newUserAgentFilter(config)
		if err != nil {
			glog.Errorf("Failure to compile User-Agent regex %v: %v", config.Regex, err)
			continue
		}

		allowedFilters = append(allowedFilters, userAgentFilter)
	}
	for _, config := range filterConfig.DeniedClients {
		userAgentFilter, err := newUserAgentFilter(config.UserAgentMatchRule)
		if err != nil {
			glog.Errorf("Failure to compile User-Agent regex %v: %v", config.Regex, err)
			continue
		}
		userAgentFilter.message = config.RejectionMessage
		if len(userAgentFilter.message) == 0 {
			userAgentFilter.message = defaultMessage
		}

		deniedFilters = append(deniedFilters, userAgentFilter)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if ctx, ok := contextMapper.Get(req); ok {
			if requestInfo, ok := apirequest.RequestInfoFrom(ctx); ok && requestInfo != nil && !requestInfo.IsResourceRequest {
				handler.ServeHTTP(w, req)
				return
			}
		}

		userAgent := req.Header.Get("User-Agent")

		if len(allowedFilters) > 0 {
			foundMatch := false
			for _, filter := range allowedFilters {
				if filter.matches(req.Method, userAgent) {
					foundMatch = true
					break
				}
			}

			if !foundMatch {
				serverhandlers.Forbidden(defaultMessage, nil, w, req)
				return
			}
		}

		for _, filter := range deniedFilters {
			if filter.matches(req.Method, userAgent) {
				serverhandlers.Forbidden(filter.message, nil, w, req)
				return
			}
		}

		handler.ServeHTTP(w, req)
	})
}

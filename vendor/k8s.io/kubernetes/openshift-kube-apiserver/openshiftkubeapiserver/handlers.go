package openshiftkubeapiserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/emicklei/go-restful"
	"k8s.io/klog"

	authenticationv1 "k8s.io/api/authentication/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	kauthorizer "k8s.io/apiserver/pkg/authorization/authorizer"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	coreapi "k8s.io/kubernetes/pkg/apis/core"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	kubecontrolplanev1 "github.com/openshift/api/kubecontrolplane/v1"
)

type userAgentFilter struct {
	regex   *regexp.Regexp
	message string
	verbs   sets.String
}

func newUserAgentFilter(config kubecontrolplanev1.UserAgentMatchRule) (userAgentFilter, error) {
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
func versionSkewFilter(handler http.Handler, userAgentMatchingConfig kubecontrolplanev1.UserAgentMatchingConfig) http.Handler {
	filterConfig := userAgentMatchingConfig
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
			klog.Errorf("Failure to compile User-Agent regex %v: %v", config.Regex, err)
			continue
		}

		allowedFilters = append(allowedFilters, userAgentFilter)
	}
	for _, config := range filterConfig.DeniedClients {
		userAgentFilter, err := newUserAgentFilter(config.UserAgentMatchRule)
		if err != nil {
			klog.Errorf("Failure to compile User-Agent regex %v: %v", config.Regex, err)
			continue
		}
		userAgentFilter.message = config.RejectionMessage
		if len(userAgentFilter.message) == 0 {
			userAgentFilter.message = defaultMessage
		}

		deniedFilters = append(deniedFilters, userAgentFilter)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		if requestInfo, ok := apirequest.RequestInfoFrom(ctx); ok && requestInfo != nil && !requestInfo.IsResourceRequest {
			handler.ServeHTTP(w, req)
			return
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
				forbidden(defaultMessage, nil, w, req)
				return
			}
		}

		for _, filter := range deniedFilters {
			if filter.matches(req.Method, userAgent) {
				forbidden(filter.message, nil, w, req)
				return
			}
		}

		handler.ServeHTTP(w, req)
	})
}

// legacyImpersonateUserScopeHeader is the header name older servers were using
// just for scopes, so we need to translate it from clients that may still be
// using it.
const legacyImpersonateUserScopeHeader = "Impersonate-User-Scope"

// translateLegacyScopeImpersonation is a filter that will translates user scope impersonation for openshift into the equivalent kube headers.
func translateLegacyScopeImpersonation(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		for _, scope := range req.Header[legacyImpersonateUserScopeHeader] {
			req.Header[authenticationv1.ImpersonateUserExtraHeaderPrefix+authorizationv1.ScopesKey] =
				append(req.Header[authenticationv1.ImpersonateUserExtraHeaderPrefix+authorizationv1.ScopesKey], scope)
		}

		handler.ServeHTTP(w, req)
	})
}

// forbidden renders a simple forbidden error to the response
func forbidden(reason string, attributes kauthorizer.Attributes, w http.ResponseWriter, req *http.Request) {
	resource := ""
	group := ""
	name := ""
	// the attributes can be empty for two basic reasons:
	// 1. malformed API request
	// 2. not an API request at all
	// In these cases, just assume default that will work better than nothing
	if attributes != nil {
		group = attributes.GetAPIGroup()
		resource = attributes.GetResource()
		name = attributes.GetName()
	}

	// Reason is an opaque string that describes why access is allowed or forbidden (forbidden by the time we reach here).
	// We don't have direct access to kind or name (not that those apply either in the general case)
	// We create a NewForbidden to stay close the API, but then we override the message to get a serialization
	// that makes sense when a human reads it.
	forbiddenError := kapierrors.NewForbidden(schema.GroupResource{Group: group, Resource: resource}, name, errors.New("") /*discarded*/)
	forbiddenError.ErrStatus.Message = reason

	formatted := &bytes.Buffer{}
	output, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(coreapi.SchemeGroupVersion), &forbiddenError.ErrStatus)
	if err != nil {
		fmt.Fprintf(formatted, "%s", forbiddenError.Error())
	} else {
		json.Indent(formatted, output, "", "  ")
	}

	w.Header().Set("Content-Type", restful.MIME_JSON)
	w.WriteHeader(http.StatusForbidden)
	w.Write(formatted.Bytes())
}

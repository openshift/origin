package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	restful "github.com/emicklei/go-restful"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kauthorizer "k8s.io/kubernetes/pkg/auth/authorizer"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/authorization/authorizer"
)

type bypassAuthorizer struct {
	paths      sets.String
	authorizer kauthorizer.Authorizer
}

// NewBypassAuthorizer creates an Authorizer that always allows the exact paths described, and delegates to the nested
// authorizer for everything else.
func NewBypassAuthorizer(auth kauthorizer.Authorizer, paths ...string) kauthorizer.Authorizer {
	return bypassAuthorizer{paths: sets.NewString(paths...), authorizer: auth}
}

func (a bypassAuthorizer) Authorize(attributes kauthorizer.Attributes) (allowed bool, reason string, err error) {
	if !attributes.IsResourceRequest() && a.paths.Has(attributes.GetPath()) {
		return true, "always allowed", nil
	}
	return a.authorizer.Authorize(attributes)
}

// AuthorizationFilter imposes normal authorization rules
func AuthorizationFilter(handler http.Handler, authorizer kauthorizer.Authorizer, authorizationAttributeBuilder authorizer.AuthorizationAttributeBuilder, contextMapper kapi.RequestContextMapper) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		attributes, err := authorizationAttributeBuilder.GetAttributes(req)
		if err != nil {
			Forbidden(err.Error(), attributes, w, req)
			return
		}
		if attributes == nil {
			Forbidden("No attributes", attributes, w, req)
			return
		}

		allowed, reason, err := authorizer.Authorize(attributes)
		if err != nil {
			Forbidden(err.Error(), attributes, w, req)
			return
		}
		if !allowed {
			Forbidden(reason, attributes, w, req)
			return
		}

		handler.ServeHTTP(w, req)
	})
}

// Forbidden renders a simple forbidden error to the response
func Forbidden(reason string, attributes kauthorizer.Attributes, w http.ResponseWriter, req *http.Request) {
	kind := ""
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
		kind = attributes.GetResource()
		if len(attributes.GetAPIGroup()) > 0 {
			kind = attributes.GetAPIGroup() + "." + kind
		}
		name = attributes.GetName()
	}

	// Reason is an opaque string that describes why access is allowed or forbidden (forbidden by the time we reach here).
	// We don't have direct access to kind or name (not that those apply either in the general case)
	// We create a NewForbidden to stay close the API, but then we override the message to get a serialization
	// that makes sense when a human reads it.
	forbiddenError := kapierrors.NewForbidden(unversioned.GroupResource{Group: group, Resource: resource}, name, errors.New("") /*discarded*/)
	forbiddenError.ErrStatus.Message = reason

	formatted := &bytes.Buffer{}
	output, err := runtime.Encode(kapi.Codecs.LegacyCodec(kapi.SchemeGroupVersion), &forbiddenError.ErrStatus)
	if err != nil {
		fmt.Fprintf(formatted, "%s", forbiddenError.Error())
	} else {
		json.Indent(formatted, output, "", "  ")
	}

	w.Header().Set("Content-Type", restful.MIME_JSON)
	w.WriteHeader(http.StatusForbidden)
	w.Write(formatted.Bytes())
}

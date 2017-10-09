package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	restful "github.com/emicklei/go-restful"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	kauthorizer "k8s.io/apiserver/pkg/authorization/authorizer"
	kapi "k8s.io/kubernetes/pkg/api"
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

// Forbidden renders a simple forbidden error to the response
func Forbidden(reason string, attributes kauthorizer.Attributes, w http.ResponseWriter, req *http.Request) {
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

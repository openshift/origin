package origin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"bitbucket.org/ww/goautoneg"

	restful "github.com/emicklei/go-restful"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	klatest "k8s.io/kubernetes/pkg/api/latest"
	"k8s.io/kubernetes/pkg/apiserver"
	"k8s.io/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/authorization/authorizer"
)

// TODO We would like to use the IndexHandler from k8s but we do not yet have a
// MuxHelper to track all registered paths
func indexAPIPaths(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/" {
			// TODO once we have a MuxHelper we will not need to hardcode this list of paths
			object := kapi.RootPaths{Paths: []string{
				"/api",
				"/api/v1beta3",
				"/api/v1",
				"/controllers",
				"/healthz",
				"/healthz/ping",
				"/logs/",
				"/metrics",
				"/ready",
				"/osapi",
				"/osapi/v1beta3",
				"/oapi",
				"/oapi/v1",
				"/swaggerapi/",
				"/userresources",
			}}
			// TODO it would be nice if apiserver.writeRawJSON was not private
			output, err := json.MarshalIndent(object, "", "  ")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", restful.MIME_JSON)
			w.WriteHeader(http.StatusOK)
			w.Write(output)
		} else {
			handler.ServeHTTP(w, req)
		}
	})
}

func (c *MasterConfig) authorizationFilter(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		attributes, err := c.AuthorizationAttributeBuilder.GetAttributes(req)
		if err != nil {
			forbidden(err.Error(), attributes, w, req)
			return
		}
		if attributes == nil {
			forbidden("No attributes", attributes, w, req)
			return
		}

		ctx, exists := c.RequestContextMapper.Get(req)
		if !exists {
			forbidden("context not found", attributes, w, req)
			return
		}

		allowed, reason, err := c.Authorizer.Authorize(ctx, attributes)
		if err != nil {
			forbidden(err.Error(), attributes, w, req)
			return
		}
		if !allowed {
			forbidden(reason, attributes, w, req)
			return
		}

		handler.ServeHTTP(w, req)
	})
}

// forbidden renders a simple forbidden error
func forbidden(reason string, attributes authorizer.AuthorizationAttributes, w http.ResponseWriter, req *http.Request) {
	kind := ""
	name := ""
	apiVersion := klatest.Version
	// the attributes can be empty for two basic reasons:
	// 1. malformed API request
	// 2. not an API request at all
	// In these cases, just assume default that will work better than nothing
	if attributes != nil {
		apiVersion = attributes.GetAPIVersion()
		kind = attributes.GetResource()
		name = attributes.GetResourceName()
	}

	// Reason is an opaque string that describes why access is allowed or forbidden (forbidden by the time we reach here).
	// We don't have direct access to kind or name (not that those apply either in the general case)
	// We create a NewForbidden to stay close the API, but then we override the message to get a serialization
	// that makes sense when a human reads it.
	forbiddenError, _ := kapierrors.NewForbidden(kind, name, errors.New("") /*discarded*/).(*kapierrors.StatusError)
	forbiddenError.ErrStatus.Message = reason

	// Not all API versions in valid API requests will have a matching codec in kubernetes.  If we can't find one,
	// just default to the latest kube codec.
	codec := klatest.Codec
	if requestedCodec, err := klatest.InterfacesFor(apiVersion); err == nil {
		codec = requestedCodec
	}

	formatted := &bytes.Buffer{}
	output, err := codec.Encode(&forbiddenError.ErrStatus)
	if err != nil {
		fmt.Fprintf(formatted, "%s", forbiddenError.Error())
	} else {
		json.Indent(formatted, output, "", "  ")
	}

	w.Header().Set("Content-Type", restful.MIME_JSON)
	w.WriteHeader(http.StatusForbidden)
	w.Write(formatted.Bytes())
}

// cacheControlFilter sets the Cache-Control header to the specified value.
func cacheControlFilter(handler http.Handler, value string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", value)
		handler.ServeHTTP(w, req)
	})
}

// namespacingFilter adds a filter that adds the namespace of the request to the context.  Not all requests will have namespaces,
// but any that do will have the appropriate value added.
func namespacingFilter(handler http.Handler, contextMapper kapi.RequestContextMapper) http.Handler {
	infoResolver := &apiserver.APIRequestInfoResolver{util.NewStringSet("api", "osapi", "oapi"), latest.RESTMapper}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx, ok := contextMapper.Get(req)
		if !ok {
			http.Error(w, "Unable to find request context", http.StatusInternalServerError)
			return
		}

		if _, exists := kapi.NamespaceFrom(ctx); !exists {
			if requestInfo, err := infoResolver.GetAPIRequestInfo(req); err == nil {
				// only set the namespace if the apiRequestInfo was resolved
				// keep in mind that GetAPIRequestInfo will fail on non-api requests, so don't fail the entire http request on that
				// kind of failure.

				// TODO reconsider special casing this.  Having the special case hereallow us to fully share the kube
				// APIRequestInfoResolver without any modification or customization.
				namespace := requestInfo.Namespace
				if (requestInfo.Resource == "projects") && (len(requestInfo.Name) > 0) {
					namespace = requestInfo.Name
				}

				ctx = kapi.WithNamespace(ctx, namespace)
				contextMapper.Update(req, ctx)
			}
		}

		handler.ServeHTTP(w, req)
	})
}

// If we know the location of the asset server, redirect to it when / is requested
// and the Accept header supports text/html
func assetServerRedirect(handler http.Handler, assetPublicURL string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/" {
			accepts := goautoneg.ParseAccept(req.Header.Get("Accept"))
			for _, accept := range accepts {
				if accept.Type == "text" && accept.SubType == "html" {
					http.Redirect(w, req, assetPublicURL, http.StatusFound)
					return
				}
			}
		}
		// Dispatch to the next handler
		handler.ServeHTTP(w, req)
	})
}

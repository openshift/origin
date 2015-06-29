package origin

import (
	"encoding/json"
	"net/http"

	"bitbucket.org/ww/goautoneg"

	"github.com/emicklei/go-restful"
	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"

	"github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/authorization/authorizer"
)

// InsecureContainer is a http.Handler that delegates requests to routes held in the container
type InsecureContainer struct {
	parent http.Handler
}

// ServeHTTP allows InsecureContainer to implement http.Handler - it delegates calls to the Container
func (i *InsecureContainer) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	i.parent.ServeHTTP(responseWriter, request)
}

// AuthorizationFilter is a http.Handler that enforces authorization rules
type AuthorizationFilter struct {
	attributeBuilder authorizer.AuthorizationAttributeBuilder
	contextMapper    kapi.RequestContextMapper
	authorizer       authorizer.Authorizer
	parent           http.Handler
}

// ServeHTTP allows the AuthorizationFilter to impement http.Handler
func (a AuthorizationFilter) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	attributes, err := a.attributeBuilder.GetAttributes(request)
	if err != nil {
		forbidden(err.Error(), "", responseWriter, request)
		return
	}
	if attributes == nil {
		forbidden("No attributes", "", responseWriter, request)
		return
	}

	ctx, exists := a.contextMapper.Get(request)
	if !exists {
		forbidden("context not found", attributes.GetAPIVersion(), responseWriter, request)
		return
	}

	allowed, reason, err := a.authorizer.Authorize(ctx, attributes)
	if err != nil {
		forbidden(err.Error(), attributes.GetAPIVersion(), responseWriter, request)
		return
	}
	if !allowed {
		forbidden(reason, attributes.GetAPIVersion(), responseWriter, request)
		return
	}

	a.parent.ServeHTTP(responseWriter, request)
}

// AuthenticationFilter is a http.Handler that enforces authentication rules
type AuthenticationFilter struct {
	authenticator authenticator.Request
	contextMapper kapi.RequestContextMapper
	parent        http.Handler
}

// ServeHTTP allows the AuthenticationFilter to impement http.Handler
func (a AuthenticationFilter) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	user, ok, err := a.authenticator.AuthenticateRequest(request)
	if err != nil || !ok {
		http.Error(responseWriter, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, ok := a.contextMapper.Get(request)
	if !ok {
		http.Error(responseWriter, "Unable to find request context", http.StatusInternalServerError)
		return
	}
	if err := a.contextMapper.Update(request, kapi.WithUser(ctx, user)); err != nil {
		glog.V(4).Infof("Error setting authenticated context: %v", err)
		http.Error(responseWriter, "Unable to set authenticated request context", http.StatusInternalServerError)
		return
	}

	a.parent.ServeHTTP(responseWriter, request)
}

// NamespacingFilter is a http.Handler that enforces namespacing rules:
// NamespacingFilter adds a filter that adds the namespace of the request to the context.
// Not all requests will have namespaces, but any that do will have the appropriate value added.
type NamespacingFilter struct {
	infoResolver  apiserver.APIRequestInfoResolver
	contextMapper kapi.RequestContextMapper
	parent        http.Handler
}

// ServeHTTP allows the NamespacingFilter to impement http.Handler
func (n NamespacingFilter) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	ctx, ok := n.contextMapper.Get(request)
	if !ok {
		http.Error(responseWriter, "Unable to find request context", http.StatusInternalServerError)
		return
	}

	if _, exists := kapi.NamespaceFrom(ctx); !exists {
		if requestInfo, err := n.infoResolver.GetAPIRequestInfo(request); err == nil {
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
			n.contextMapper.Update(request, ctx)
		}
	}

	n.parent.ServeHTTP(responseWriter, request)
}

// CacheControlFilter is a http.Handler that enforces caching rules:
// The CacheControlFilter sets the Cache-Control header to the specified value.
type CacheControlFilter struct {
	headerSetting string
	parent        http.Handler
}

// ServeHTTP allows the CacheControlFilter to impement http.Handler
func (c CacheControlFilter) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.Header().Set("Cache-Control", c.headerSetting)
	c.parent.ServeHTTP(responseWriter, request)
}

// TODO We would like to use the IndexHandler from k8s but we do not yet have a
// MuxHelper to track all registered paths
// APIPathIndexer is a http.Handler that handles requests to the root with a response containing API paths
type APIPathIndexer struct {
	parent http.Handler
}

// ServeHTTP allows the APIPathIndexer to impement http.Handler
func (a APIPathIndexer) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/" {
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
		}}
		// TODO it would be nice if apiserver.writeRawJSON was not private
		output, err := json.MarshalIndent(object, "", "  ")
		if err != nil {
			http.Error(responseWriter, err.Error(), http.StatusInternalServerError)
			return
		}
		responseWriter.Header().Set("Content-Type", restful.MIME_JSON)
		responseWriter.WriteHeader(http.StatusOK)
		responseWriter.Write(output)
	} else {
		a.parent.ServeHTTP(responseWriter, request)
	}
}

// CORSFilter is a http.Handler that wraps the CORSFilter given by the Kubernetes Apiserver in order to
// strongly type this filter for identification
type CORSFilter struct {
	handler http.Handler
}

// ServeHTTP allows the CORSFilter to implement http.Handler - it delegates to the internal handler from the CORSFilter
func (c CORSFilter) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	c.handler.ServeHTTP(responseWriter, request)
}

// AssetServerRedirecter is a http.Handler that allows for asset server redirection:
// If we know the location of the asset server, redirect to it when / is requested
// and the Accept header supports text/html
type AssetServerRedirecter struct {
	assetPublicURL string
	parent         http.Handler
}

// ServeHTTP allows the AssetServerRedirecter to impement http.Handler
func (a AssetServerRedirecter) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/" {
		accepts := goautoneg.ParseAccept(request.Header.Get("Accept"))
		for _, accept := range accepts {
			if accept.Type == "text" && accept.SubType == "html" {
				http.Redirect(responseWriter, request, a.assetPublicURL, http.StatusFound)
				return
			}
		}
	}
	// Dispatch to the next handler
	a.parent.ServeHTTP(responseWriter, request)
}

// RequestContextFilter is a http.Handler that wraps the RequestContextFilter given by the Kubernetes api in order to
// strongly type this filter for identification
type RequestContextFilter struct {
	handler http.Handler
}

// ServeHTTP allows the RequestContextFilter to implement http.Handler - it delegates to the internal handler from the RequestContextFilter
func (c RequestContextFilter) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	c.handler.ServeHTTP(responseWriter, request)
}

// MaxInFlightLimitFilter is a http.Handler that wraps the MaxInFlightLimitFilter given by the Kubernetes api in order to
// strongly type this filter for identification
type MaxInFlightLimitFilter struct {
	handler http.Handler
}

// ServeHTTP allows the MaxInFlightLimitFilter to implement http.Handler - it delegates to the internal handler from the MaxInFlightLimitFilter
func (c MaxInFlightLimitFilter) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	c.handler.ServeHTTP(responseWriter, request)
}

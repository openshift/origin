package origin

import (
	"net/http"
	"regexp"

	"github.com/emicklei/go-restful"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/authorization/authorizer"
)

func (c *MasterConfig) assembleHandlers(safe, open *restful.Container) http.Handler {
	handlerPrepender := NewHandlerPrepender()
	return c.assembleHandlersUsingPrepender(safe, open, handlerPrepender)
}

func (c *MasterConfig) assembleHandlersUsingPrepender(safe, open *restful.Container, handlerPrepender HandlerPrepender) http.Handler {
	authorizationFilterConfig := &AuthorizationFilterConfig{
		attributeBuilder: c.AuthorizationAttributeBuilder,
		contextMapper:    c.getRequestContextMapper(),
		authorizer:       c.Authorizer,
	}

	authenticationFilterConfig := &AuthenticationFilterConfig{
		authenticator: c.Authenticator,
		contextMapper: c.getRequestContextMapper(),
	}

	namespacingFilterConfig := &NamespacingFilterConfig{
		contextMapper: c.getRequestContextMapper(),
	}

	cacheControlFilterConfig := &CacheControlFilterConfig{
		headerSetting: "no-store", // protected endpoints should not be cached
	}

	apiPathIndexerConfig := &APIPathIndexerConfig{}

	insecureContainerConfig := &InsecureContainerConfig{
		insecureContainer: open,
	}

	requestContextFilterConfig := &RequestContextFilterConfig{
		contextMapper: c.getRequestContextMapper(),
	}

	// prepend handlers to the secure restful container
	handler := http.Handler(safe)

	// prepend handler chain for securing the secure container
	handler = handlerPrepender.PrependHandler(safe, authorizationFilterConfig)
	handler = handlerPrepender.PrependHandler(handler, authenticationFilterConfig)
	handler = handlerPrepender.PrependHandler(handler, namespacingFilterConfig)
	handler = handlerPrepender.PrependHandler(handler, cacheControlFilterConfig)
	handler = handlerPrepender.PrependHandler(handler, apiPathIndexerConfig)

	// prepend handlers to the insecure restful container
	handler = handlerPrepender.PrependHandler(handler, insecureContainerConfig)

	// add CORS support
	if len(c.ensureCORSAllowedOrigins()) != 0 {
		corsFilterConfig := &CORSFilterConfig{
			origins:          c.ensureCORSAllowedOrigins(),
			allowedMethods:   nil, // use default set of methods
			allowedHeaders:   nil, // use default set of headers
			allowCredentials: "true",
		}
		handler = handlerPrepender.PrependHandler(handler, corsFilterConfig)
	}

	if c.Options.AssetConfig != nil {
		assetServerRedirecterConfig := &AssetServerRedirecterConfig{
			assetPublicURL: c.Options.AssetConfig.PublicURL,
		}
		handler = handlerPrepender.PrependHandler(handler, assetServerRedirecterConfig)
	}

	// Make the outermost filter the requestContextMapper to ensure all components share the same context
	handler = handlerPrepender.PrependHandler(handler, requestContextFilterConfig)

	// TODO: MaxRequestsInFlight should be subdivided by intent, type of behavior, and speed of
	// execution - updates vs reads, long reads vs short reads, fat reads vs skinny reads.
	if c.Options.ServingInfo.MaxRequestsInFlight > 0 {
		maxInFlightLimitFilterConfig := &MaxInFlightLimitFilterConfig{
			channel:                 make(chan bool, c.Options.ServingInfo.MaxRequestsInFlight),
			longRunningRequestRegex: longRunningRE,
		}
		handler = handlerPrepender.PrependHandler(handler, maxInFlightLimitFilterConfig)
	}
	return handler
}

// HandlerPrepender prepends handlers to the end of a chain
type HandlerPrepender interface {
	PrependHandler(http.Handler, HandlerPrependSpecifier) http.Handler
}

func NewHandlerPrepender() HandlerPrepender {
	return &defaultHandlerPrepender{}
}

type defaultHandlerPrepender struct{}

func (ha *defaultHandlerPrepender) PrependHandler(parent http.Handler, prepender HandlerPrependSpecifier) http.Handler {
	return prepender.Prepend(parent)
}

// HandlerPrependSpecifier is an interface for objects that can specify how to prepend a handler
type HandlerPrependSpecifier interface {
	// Prepend prepends the child handler to the parent handler
	Prepend(parent http.Handler) (child http.Handler)
}

// InsecureContainerConfig is the HandlerPrependSpecifier that prepeds an InsecureContainer
type InsecureContainerConfig struct {
	insecureContainer *restful.Container
}

// Prepend implements HandlerPrependSpecifier for prepending an InsecureContainer
// This implementation is different from others in that we want the resulting handlers to be prepended
// to not only the parent Handler given but also any WebServices on the Container, so we add the Parent
// handler to handle paths from `/` on the Container, and then return the Handler for the Container
// which will choose when to route requests to WebServices and when to route them to the Parent
func (i *InsecureContainerConfig) Prepend(parent http.Handler) http.Handler {
	i.insecureContainer.Handle("/", parent)
	return &InsecureContainer{
		parent: http.Handler(i.insecureContainer),
	}
}

// AuthorizationFilterConfig is the HandlerPrependSpecifier that prepends an AuthorizationFilter
type AuthorizationFilterConfig struct {
	attributeBuilder authorizer.AuthorizationAttributeBuilder
	contextMapper    kapi.RequestContextMapper
	authorizer       authorizer.Authorizer
}

// Prepend implements HandlerPrependSpecifier for prepending an AuthorizationFilter
func (f *AuthorizationFilterConfig) Prepend(parent http.Handler) http.Handler {
	return &AuthorizationFilter{
		attributeBuilder: f.attributeBuilder,
		contextMapper:    f.contextMapper,
		authorizer:       f.authorizer,
		parent:           parent,
	}
}

// AuthenticationFilterConfig is the HandlerPrependSpecifier that prepends an AuthenticationFilter
type AuthenticationFilterConfig struct {
	authenticator authenticator.Request
	contextMapper kapi.RequestContextMapper
}

// Prepend implements HandlerPrependSpecifier for prepending an AuthenticationFilter
func (f *AuthenticationFilterConfig) Prepend(parent http.Handler) http.Handler {
	return &AuthenticationFilter{
		authenticator: f.authenticator,
		contextMapper: f.contextMapper,
		parent:        parent,
	}
}

// NamespacingFilterConfig is the HandlerPrependSpecifier that prepends a NamepsacingFilter
type NamespacingFilterConfig struct {
	contextMapper kapi.RequestContextMapper
}

// Prepend implements HandlerPrependSpecifier for prepending an NamespacingFilter
func (n *NamespacingFilterConfig) Prepend(parent http.Handler) http.Handler {
	return &NamespacingFilter{
		infoResolver: apiserver.APIRequestInfoResolver{
			util.NewStringSet("api", "osapi", "oapi"),
			latest.RESTMapper,
		},
		contextMapper: n.contextMapper,
		parent:        parent,
	}
}

// CacheControlFilterConfig is the HandlerPrependSpecifier that prepends the CacheControlFilter
type CacheControlFilterConfig struct {
	headerSetting string
}

// Prepend implements HandlerPrependSpecifier for prepending an CacheControlFilter
func (c *CacheControlFilterConfig) Prepend(parent http.Handler) http.Handler {
	return &CacheControlFilter{
		headerSetting: c.headerSetting,
		parent:        parent,
	}
}

// APIPathIndexerConfig is the HandlerPrependSpecifier that prepends the APIPathIndexer
type APIPathIndexerConfig struct{}

// Prepend implements HandlerPrependSpecifier for prepending an APIPathIndexer
func (a *APIPathIndexerConfig) Prepend(parent http.Handler) http.Handler {
	return &APIPathIndexer{
		parent: parent,
	}
}

// CORSFilterConfig is the HandlerPrependSpecifier that prepends the CORSFilter
type CORSFilterConfig struct {
	origins          []*regexp.Regexp
	allowedMethods   []string
	allowedHeaders   []string
	allowCredentials string
}

// Prepend implements HandlerPrependSpecifier for prepending a CORSFilter
func (c *CORSFilterConfig) Prepend(parent http.Handler) http.Handler {
	corsHandler := apiserver.CORS(parent, c.origins, c.allowedMethods, c.allowedHeaders, c.allowCredentials)
	return &CORSFilter{
		handler: corsHandler,
	}
}

// AssetServerRedirecterConfig is the HandlerPrependSpecifier that prepends the AssetServerRedirecter
type AssetServerRedirecterConfig struct {
	assetPublicURL string
}

// Prepend implements HandlerPrependSpecifier for prepending an AssetServerRedirecter
func (a *AssetServerRedirecterConfig) Prepend(parent http.Handler) http.Handler {
	return &AssetServerRedirecter{
		assetPublicURL: a.assetPublicURL,
		parent:         parent,
	}
}

// RequestContextFilterConfig is the HandlerPrependSpecifier that prepends the RequestContextFilter
type RequestContextFilterConfig struct {
	contextMapper kapi.RequestContextMapper
}

// Prepend implements HandlerPrependSpecifier for prepending a RequestContextFilter
func (r *RequestContextFilterConfig) Prepend(parent http.Handler) http.Handler {
	contextFilter, err := kapi.NewRequestContextFilter(r.contextMapper, parent)
	if err != nil {
		glog.Fatalf("Error setting up request context filter: %v", err)
		return parent
	}
	return &RequestContextFilter{
		handler: contextFilter,
	}
}

// MaxInFlightLimitFilterConfig is the HandlerPrependSpecifier that prepends the MaxInFlightLimitFilter
type MaxInFlightLimitFilterConfig struct {
	channel                 chan bool
	longRunningRequestRegex *regexp.Regexp
}

// Prepend implements HandlerPrependSpecifier for prepending a MaxInFlightLimitFilter
func (m *MaxInFlightLimitFilterConfig) Prepend(parent http.Handler) http.Handler {
	maxInFlightLimitHandler := apiserver.MaxInFlightLimit(m.channel, m.longRunningRequestRegex, parent)
	return &MaxInFlightLimitFilter{
		handler: maxInFlightLimitHandler,
	}
}

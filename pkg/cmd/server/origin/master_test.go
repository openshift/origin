package origin

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	kmaster "github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"

	"github.com/emicklei/go-restful"
)

func TestInitializeOpenshiftAPIVersionRouteHandler(t *testing.T) {
	service := new(restful.WebService)
	initAPIVersionRoute(service, "osapi", "v1beta3")

	if len(service.Routes()) != 1 {
		t.Fatalf("Exp. the OSAPI route but found none")
	}
	route := service.Routes()[0]
	if !contains(route.Produces, restful.MIME_JSON) {
		t.Fatalf("Exp. route to produce mimetype json")
	}
	if !contains(route.Consumes, restful.MIME_JSON) {
		t.Fatalf("Exp. route to consume mimetype json")
	}
}

func contains(list []string, value string) bool {
	for _, entry := range list {
		if entry == value {
			return true
		}
	}
	return false
}

func TestPrependHandlers(t *testing.T) {
	tests := []struct {
		testName        string
		config          MasterConfig
		desiredHandlers []string
	}{
		{
			testName: "allHandlers",
			config: MasterConfig{
				Options: configapi.MasterConfig{
					AssetConfig: &configapi.AssetConfig{
						PublicURL: "http://localhost", // allow asset server redirects
					},
					ServingInfo: configapi.HTTPServingInfo{
						MaxRequestsInFlight: 1, // allow max-in-flight filter
					},
					CORSAllowedOrigins: []string{
						"origin", // allow CORS support
					},
				},
			},
			desiredHandlers: []string{
				"MaxInFlightLimitFilter",
				"RequestContextFilter",
				"AssetServerRedirecter",
				"CORSFilter",
				"InsecureContainer", // contains unprotected WebServices - asset & auth servers
				"APIPathIndexer",
				"CacheControlFilter",
				"NamespacingFilter",
				"AuthenticationFilter",
				"AuthorizationFilter",
				"Container", // contains protected WebServices - API, controllers, health&readiness, etc
			},
		},
		{
			testName: "noCORS",
			config: MasterConfig{
				Options: configapi.MasterConfig{
					AssetConfig: &configapi.AssetConfig{
						PublicURL: "http://localhost", // allow asset server redirects
					},
					ServingInfo: configapi.HTTPServingInfo{
						MaxRequestsInFlight: 1, // allow max-in-flight filter
					},
				},
			},
			desiredHandlers: []string{
				"MaxInFlightLimitFilter",
				"RequestContextFilter",
				"AssetServerRedirecter",
				"InsecureContainer", // contains unprotected WebServices - asset & auth servers
				"APIPathIndexer",
				"CacheControlFilter",
				"NamespacingFilter",
				"AuthenticationFilter",
				"AuthorizationFilter",
				"Container", // contains protected WebServices - API, controllers, health&readiness, etc
			},
		},
		{
			testName: "noMaxInFlight",
			config: MasterConfig{
				Options: configapi.MasterConfig{
					AssetConfig: &configapi.AssetConfig{
						PublicURL: "http://localhost", // allow asset server redirects
					},
					CORSAllowedOrigins: []string{
						"origin", // allow CORS support
					},
				},
			},
			desiredHandlers: []string{
				"RequestContextFilter",
				"AssetServerRedirecter",
				"CORSFilter",
				"InsecureContainer", // contains unprotected WebServices - asset & auth servers
				"APIPathIndexer",
				"CacheControlFilter",
				"NamespacingFilter",
				"AuthenticationFilter",
				"AuthorizationFilter",
				"Container", // contains protected WebServices - API, controllers, health&readiness, etc
			},
		},
		{
			testName: "noAssetServerRedirect",
			config: MasterConfig{
				Options: configapi.MasterConfig{
					ServingInfo: configapi.HTTPServingInfo{
						MaxRequestsInFlight: 1, // allow max-in-flight filter
					},
					CORSAllowedOrigins: []string{
						"origin", // allow CORS support
					},
				},
			},
			desiredHandlers: []string{
				"MaxInFlightLimitFilter",
				"RequestContextFilter",
				"CORSFilter",
				"InsecureContainer", // contains unprotected WebServices - asset & auth servers
				"APIPathIndexer",
				"CacheControlFilter",
				"NamespacingFilter",
				"AuthenticationFilter",
				"AuthorizationFilter",
				"Container", // contains protected WebServices - API, controllers, health&readiness, etc
			},
		},
	}

	for _, test := range tests {
		handlerPrepender := &testHandlerPrepender{
			prependedHandlers: make(map[reflect.Type]reflect.Type),
		}
		secureContainer := kmaster.NewHandlerContainer(http.NewServeMux())
		insecureContainer := kmaster.NewHandlerContainer(http.NewServeMux())

		test.config.assembleHandlersUsingPrepender(secureContainer, insecureContainer, handlerPrepender)

		actualHandlers := handlerPrepender.GetHandlerChain()

		if !reflect.DeepEqual(actualHandlers, test.desiredHandlers) {
			t.Fatalf("test %v failed: got handlers: %v, wanted: %v", test.testName, actualHandlers, test.desiredHandlers)
		}
	}
}

type testHandlerPrepender struct {
	firstHandlerPrepended reflect.Type
	prependedHandlers     map[reflect.Type]reflect.Type
}

// PrependHandler prepends a child onto the given parent using the given HandlerPrependFunc and records
// that this prepending happened internally.
func (hi *testHandlerPrepender) PrependHandler(parent http.Handler, prependSpec HandlerPrependSpecifier) http.Handler {
	child := prependSpec.Prepend(parent)

	parentType := reflect.TypeOf(parent)
	childType := reflect.TypeOf(child)

	if parentType != childType { // if the two are the same, the prependSpec encountered an error and skipped addition
		hi.prependedHandlers[parentType] = childType
		if hi.firstHandlerPrepended == nil {
			hi.firstHandlerPrepended = parentType
		}
	}

	return child
}

// GetHandlerChain returns the longest handler chain that was built from the first prepended handler
// This implementation will not be sufficient if handler prepending is ever not one-to-one (e.g. multiple children)
func (hi *testHandlerPrepender) GetHandlerChain() []string {
	handlerChain := []string{}

	var currentHandler reflect.Type
	currentHandler = hi.firstHandlerPrepended

	for {
		fullHandlerType := fmt.Sprintf("%v", currentHandler)
		handlerType := strings.Split(fullHandlerType, ".")[1]
		handlerChain = append([]string{handlerType}, handlerChain...)
		nextHandler, ok := hi.prependedHandlers[currentHandler]
		if !ok {
			break
		}
		currentHandler = nextHandler
	}
	return handlerChain
}
